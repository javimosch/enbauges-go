// Package ircchat is the Go port of the Node irc-chat mini-app: an
// old-school IRC-style chat (HTTP polling, no websockets), on the same
// "ircchannels" and "ircmessages" collections. Non-pinned, non-persistent
// messages expire after 90 days via a partial TTL index.
package ircchat

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/javimosch/enbauges-go/plugin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//go:embed web
var webFS embed.FS

const prefix = "/chat"

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type Channel struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Name         string             `bson:"name" json:"name"`
	Description  string             `bson:"description" json:"description"`
	CreatorNick  string             `bson:"creatorNick" json:"creatorNick"`
	CreatorID    string             `bson:"creatorId" json:"creatorId"`
	LastActivity time.Time          `bson:"lastActivity" json:"lastActivity"`
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type Message struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	ChannelID  primitive.ObjectID `bson:"channelId" json:"channelId"`
	Nick       string             `bson:"nick" json:"nick"`
	Text       string             `bson:"text" json:"text"`
	Pinned     bool               `bson:"pinned" json:"pinned"`
	Persistent bool               `bson:"persistent" json:"persistent"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type IrcChat struct{}

func New() *IrcChat { return &IrcChat{} }

func (p *IrcChat) Meta() plugin.Meta {
	return plugin.Meta{
		ID:          "irc-chat",
		Name:        "Chat IRC",
		Version:     "1.0.0",
		Description: "Chat old-school style IRC pour la communauté du Massif des Bauges",
		RoutePrefix: prefix,
		Aliases:     []string{"/irc", "/chat-irc"},
		Tags:        []string{"community", "chat", "irc", "communaute"},
	}
}

func (p *IrcChat) WebFS() fs.FS {
	sub, _ := fs.Sub(webFS, "web")
	return sub
}

func (p *IrcChat) Install(ctx *plugin.Context) error {
	// Indexes the mongoose schema declares, incl. the partial TTL.
	ninetyDays := int32(90 * 24 * 60 * 60)
	ictx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = ctx.DB.Collection("ircchannels").Indexes().CreateMany(ictx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "name", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "lastActivity", Value: 1}}},
	})
	_, _ = ctx.DB.Collection("ircmessages").Indexes().CreateMany(ictx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "channelId", Value: 1}, {Key: "createdAt", Value: 1}}},
		{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().
			SetExpireAfterSeconds(ninetyDays).
			SetPartialFilterExpression(bson.M{"persistent": false, "pinned": false})},
	})
	return ctx.UpsertServiceCard(plugin.ServiceCard{
		Type:        "solution",
		Title:       "Chat IRC",
		Description: "Discutez en temps réel sur les canaux du Massif des Bauges — style old-school",
		URL:         prefix,
		Tags:        []string{"service", "chat", "communaute", "irc"},
	})
}

func (p *IrcChat) Bootstrap(ctx *plugin.Context) error { return nil }

func (p *IrcChat) channels(ctx *plugin.Context) *mongo.Collection {
	return ctx.DB.Collection("ircchannels")
}

func (p *IrcChat) messages(ctx *plugin.Context) *mongo.Collection {
	return ctx.DB.Collection("ircmessages")
}

type input struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	CreatorNick string  `json:"creatorNick"`
	CreatorID   string  `json:"creatorId"`
	Nick        string  `json:"nick"`
	Text        string  `json:"text"`
}

func pathID(w http.ResponseWriter, r *http.Request, key, notFound string) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(r.PathValue(key))
	if err != nil {
		plugin.WriteErr(w, 404, notFound)
		return primitive.NilObjectID, false
	}
	return id, true
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func (p *IrcChat) Mount(mux *http.ServeMux, ctx *plugin.Context) error {
	mux.HandleFunc("GET "+prefix, plugin.ServeWebFile(ctx, "irc-chat.html"))

	mux.HandleFunc("GET "+prefix+"/api/channels", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		list := []Channel{}
		cur, err := p.channels(ctx).Find(dbctx, bson.M{},
			options.Find().SetSort(bson.D{{Key: "lastActivity", Value: -1}}).SetLimit(200))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &list); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"channels": list})
	})

	mux.HandleFunc("POST "+prefix+"/api/channels", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		var in input
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		name := strings.TrimSpace(in.Name)
		switch {
		case name == "":
			plugin.WriteErr(w, 400, "Le nom du canal est requis")
			return
		case !nameRe.MatchString(name):
			plugin.WriteErr(w, 400, "Nom invalide (lettres, chiffres, _ et - uniquement)")
			return
		case len([]rune(name)) > 40:
			plugin.WriteErr(w, 400, "Nom trop long (40 caractères max)")
			return
		case strings.TrimSpace(in.CreatorNick) == "":
			plugin.WriteErr(w, 400, "Pseudo requis")
			return
		case strings.TrimSpace(in.CreatorID) == "":
			plugin.WriteErr(w, 400, "Identifiant requis")
			return
		}
		name = strings.ToLower(name)
		if err := p.channels(ctx).FindOne(dbctx, bson.M{"name": name}).Err(); err == nil {
			plugin.WriteErr(w, 409, "Ce canal existe déjà")
			return
		}
		desc := ""
		if in.Description != nil {
			desc = truncate(strings.TrimSpace(*in.Description), 200)
		}
		now := time.Now().UTC()
		ch := Channel{
			ID:           primitive.NewObjectID(),
			Name:         name,
			Description:  desc,
			CreatorNick:  strings.TrimSpace(in.CreatorNick),
			CreatorID:    strings.TrimSpace(in.CreatorID),
			LastActivity: now,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if _, err := p.channels(ctx).InsertOne(dbctx, ch); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 201, ch)
	})

	mux.HandleFunc("PUT "+prefix+"/api/channels/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := pathID(w, r, "id", "Canal introuvable")
		if !ok {
			return
		}
		var in input
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		var ch Channel
		if err := p.channels(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&ch); err != nil {
			plugin.WriteErr(w, 404, "Canal introuvable")
			return
		}
		if strings.TrimSpace(in.CreatorID) == "" || strings.TrimSpace(in.CreatorID) != ch.CreatorID {
			plugin.WriteErr(w, 403, "Seul le créateur peut modifier ce canal")
			return
		}
		if in.Description != nil {
			ch.Description = truncate(strings.TrimSpace(*in.Description), 200)
		}
		ch.UpdatedAt = time.Now().UTC()
		if _, err := p.channels(ctx).UpdateOne(dbctx, bson.M{"_id": id},
			bson.M{"$set": bson.M{"description": ch.Description, "updatedAt": ch.UpdatedAt}}); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, ch)
	})

	mux.HandleFunc("DELETE "+prefix+"/api/channels/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := pathID(w, r, "id", "Canal introuvable")
		if !ok {
			return
		}
		var in input
		_ = plugin.DecodeBody(r, &in)
		var ch Channel
		if err := p.channels(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&ch); err != nil {
			plugin.WriteErr(w, 404, "Canal introuvable")
			return
		}
		if strings.TrimSpace(in.CreatorID) == "" || strings.TrimSpace(in.CreatorID) != ch.CreatorID {
			plugin.WriteErr(w, 403, "Seul le créateur peut supprimer ce canal")
			return
		}
		_, _ = p.messages(ctx).DeleteMany(dbctx, bson.M{"channelId": id})
		_, _ = p.channels(ctx).DeleteOne(dbctx, bson.M{"_id": id})
		plugin.WriteJSON(w, 200, map[string]any{"success": true})
	})

	mux.HandleFunc("GET "+prefix+"/api/channels/{channelId}/messages", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		chID, ok := pathID(w, r, "channelId", "Canal introuvable")
		if !ok {
			return
		}
		if err := p.channels(ctx).FindOne(dbctx, bson.M{"_id": chID}).Err(); err != nil {
			plugin.WriteErr(w, 404, "Canal introuvable")
			return
		}
		limit := int64(100)
		if l, err := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64); err == nil && l > 0 {
			limit = min(l, 500)
		}
		filter := bson.M{"channelId": chID}
		if before := r.URL.Query().Get("before"); before != "" {
			if bid, err := primitive.ObjectIDFromHex(before); err == nil {
				filter["_id"] = bson.M{"$lt": bid}
			}
		}
		list := []Message{}
		cur, err := p.messages(ctx).Find(dbctx, filter,
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}).SetLimit(limit))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &list); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"messages": list})
	})

	mux.HandleFunc("POST "+prefix+"/api/channels/{channelId}/messages", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		chID, ok := pathID(w, r, "channelId", "Canal introuvable")
		if !ok {
			return
		}
		var in input
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		nick := strings.TrimSpace(in.Nick)
		text := strings.TrimSpace(in.Text)
		switch {
		case nick == "":
			plugin.WriteErr(w, 400, "Pseudo requis")
			return
		case text == "":
			plugin.WriteErr(w, 400, "Message vide")
			return
		case len([]rune(in.Text)) > 1000:
			plugin.WriteErr(w, 400, "Message trop long (1000 caractères max)")
			return
		}
		if err := p.channels(ctx).FindOne(dbctx, bson.M{"_id": chID}).Err(); err != nil {
			plugin.WriteErr(w, 404, "Canal introuvable")
			return
		}
		now := time.Now().UTC()
		msg := Message{
			ID:        primitive.NewObjectID(),
			ChannelID: chID,
			Nick:      truncate(nick, 30),
			Text:      truncate(text, 1000),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if _, err := p.messages(ctx).InsertOne(dbctx, msg); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		_, _ = p.channels(ctx).UpdateOne(dbctx, bson.M{"_id": chID},
			bson.M{"$set": bson.M{"lastActivity": now, "updatedAt": now}})
		plugin.WriteJSON(w, 201, msg)
	})

	toggle := func(field string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			dbctx, cancel := plugin.DBCtx(r)
			defer cancel()
			id, ok := pathID(w, r, "id", "Message introuvable")
			if !ok {
				return
			}
			var msg Message
			if err := p.messages(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&msg); err != nil {
				plugin.WriteErr(w, 404, "Message introuvable")
				return
			}
			val := !map[string]bool{"pinned": msg.Pinned, "persistent": msg.Persistent}[field]
			if err := p.messages(ctx).FindOneAndUpdate(dbctx, bson.M{"_id": id},
				bson.M{"$set": bson.M{field: val, "updatedAt": time.Now().UTC()}},
				options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&msg); err != nil {
				plugin.WriteErr(w, 500, err.Error())
				return
			}
			plugin.WriteJSON(w, 200, map[string]any{field: val, "message": msg})
		}
	}
	mux.HandleFunc("PATCH "+prefix+"/api/messages/{id}/pin", toggle("pinned"))
	mux.HandleFunc("PATCH "+prefix+"/api/messages/{id}/persist", toggle("persistent"))

	mux.HandleFunc("DELETE "+prefix+"/api/messages/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := pathID(w, r, "id", "Message introuvable")
		if !ok {
			return
		}
		var in input
		_ = plugin.DecodeBody(r, &in)
		var msg Message
		if err := p.messages(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&msg); err != nil {
			plugin.WriteErr(w, 404, "Message introuvable")
			return
		}
		if strings.TrimSpace(in.Nick) == "" || strings.TrimSpace(in.Nick) != msg.Nick {
			plugin.WriteErr(w, 403, "Seul l'auteur peut supprimer ce message")
			return
		}
		_, _ = p.messages(ctx).DeleteOne(dbctx, bson.M{"_id": id})
		plugin.WriteJSON(w, 200, map[string]any{"success": true})
	})
	return nil
}
