package main

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var cardTypes = []string{"actor", "solution", "initiative"}

func reqCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 10*time.Second)
}

func objID(w http.ResponseWriter, r *http.Request) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeErr(w, 404, "Card not found")
		return primitive.NilObjectID, false
	}
	return id, true
}

func clientVoter(r *http.Request, voterID string) string {
	if voterID != "" {
		return voterID
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

func handleListCards(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	q := r.URL.Query()

	filter := bson.M{}
	if t := q.Get("type"); t != "" && t != "all" {
		filter["type"] = t
	}
	if tag := q.Get("tag"); tag != "" {
		filter["tags"] = tag
	}
	if search := q.Get("search"); search != "" {
		filter["$or"] = bson.A{
			bson.M{"title": bson.M{"$regex": regexEscape(search), "$options": "i"}},
			bson.M{"description": bson.M{"$regex": regexEscape(search), "$options": "i"}},
		}
	}

	limit, _ := strconv.ParseInt(q.Get("limit"), 10, 64)
	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	offset, _ := strconv.ParseInt(q.Get("offset"), 10, 64)
	if offset < 0 {
		offset = 0
	}
	sort := bson.D{{Key: "createdAt", Value: -1}}
	if q.Get("sort") == "votes" {
		sort = bson.D{{Key: "votes", Value: -1}, {Key: "createdAt", Value: -1}}
	}

	cur, err := cards().Find(ctx, filter, options.Find().SetSort(sort).SetSkip(offset).SetLimit(limit))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	result := []Card{}
	if err := cur.All(ctx, &result); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	total, err := cards().CountDocuments(ctx, filter)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"cards": result, "total": total, "limit": limit, "offset": offset})
}

func handleGetCard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	id, ok := objID(w, r)
	if !ok {
		return
	}
	var card Card
	if err := cards().FindOne(ctx, bson.M{"_id": id}).Decode(&card); err != nil {
		writeErr(w, 404, "Card not found")
		return
	}
	linkList, err := populatedLinks(ctx, bson.M{"$or": bson.A{bson.M{"sourceCardId": id}, bson.M{"targetCardId": id}}})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	commentList := []Comment{}
	cur, err := comments().Find(ctx, bson.M{"cardId": id}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err == nil {
		_ = cur.All(ctx, &commentList)
	}
	writeJSON(w, 200, map[string]any{"card": card, "links": linkList, "comments": commentList})
}

type cardInput struct {
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	URL         *string  `json:"url"`
	Contact     *string  `json:"contact"`
	Address     *string  `json:"address"`
	Lat         *float64 `json:"lat"`
	Lng         *float64 `json:"lng"`
	VoterID     string   `json:"voterId"`
}

func handleCreateCard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	var in cardInput
	if err := decodeBody(r, &in); err != nil {
		writeErr(w, 400, "invalid JSON body")
		return
	}
	if in.Type == "" || in.Title == "" || in.Description == "" {
		writeErr(w, 400, "type, title, and description are required")
		return
	}
	if !slices.Contains(cardTypes, in.Type) {
		writeErr(w, 400, "type must be actor, solution, or initiative")
		return
	}
	if len([]rune(in.Title)) > 200 {
		writeErr(w, 400, "title must be 200 characters or less")
		return
	}
	if len([]rune(in.Description)) > 500 {
		writeErr(w, 400, "description must be 500 characters or less")
		return
	}
	now := time.Now().UTC()
	card := Card{
		ID:          primitive.NewObjectID(),
		Type:        in.Type,
		Title:       strings.TrimSpace(in.Title),
		Description: strings.TrimSpace(in.Description),
		Tags:        in.Tags,
		Voters:      []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if card.Tags == nil {
		card.Tags = []string{}
	}
	if in.URL != nil {
		card.URL = strings.TrimSpace(*in.URL)
	}
	if in.Contact != nil {
		card.Contact = strings.TrimSpace(*in.Contact)
	}
	if in.Address != nil {
		card.Address = strings.TrimSpace(*in.Address)
	}
	card.Lat, card.Lng = in.Lat, in.Lng

	if _, err := cards().InsertOne(ctx, card); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, card)
}

func handleUpdateCard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	id, ok := objID(w, r)
	if !ok {
		return
	}
	var in cardInput
	if err := decodeBody(r, &in); err != nil {
		writeErr(w, 400, "invalid JSON body")
		return
	}
	set := bson.M{"updatedAt": time.Now().UTC()}
	unset := bson.M{}
	if in.Title != "" {
		if len([]rune(in.Title)) > 200 {
			writeErr(w, 400, "title must be 200 characters or less")
			return
		}
		set["title"] = strings.TrimSpace(in.Title)
	}
	if in.Description != "" {
		if len([]rune(in.Description)) > 500 {
			writeErr(w, 400, "description must be 500 characters or less")
			return
		}
		set["description"] = strings.TrimSpace(in.Description)
	}
	if in.Tags != nil {
		set["tags"] = in.Tags
	}
	if in.URL != nil {
		if v := strings.TrimSpace(*in.URL); v != "" {
			set["url"] = v
		} else {
			unset["url"] = ""
		}
	}
	if in.Contact != nil {
		if v := strings.TrimSpace(*in.Contact); v != "" {
			set["contact"] = v
		} else {
			unset["contact"] = ""
		}
	}
	if in.Address != nil {
		if v := strings.TrimSpace(*in.Address); v != "" {
			set["address"] = v
		} else {
			unset["address"] = ""
		}
	}
	if in.Lat != nil {
		set["lat"] = *in.Lat
	}
	if in.Lng != nil {
		set["lng"] = *in.Lng
	}
	update := bson.M{"$set": set}
	if len(unset) > 0 {
		update["$unset"] = unset
	}
	var card Card
	err := cards().FindOneAndUpdate(ctx, bson.M{"_id": id}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&card)
	if err == mongo.ErrNoDocuments {
		writeErr(w, 404, "Card not found")
		return
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, card)
}

func handleDeleteCard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	id, ok := objID(w, r)
	if !ok {
		return
	}
	res, err := cards().DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if res.DeletedCount == 0 {
		writeErr(w, 404, "Card not found")
		return
	}
	_, _ = comments().DeleteMany(ctx, bson.M{"cardId": id})
	_, _ = links().DeleteMany(ctx, bson.M{"$or": bson.A{bson.M{"sourceCardId": id}, bson.M{"targetCardId": id}}})
	writeJSON(w, 200, map[string]any{"success": true})
}

func handleVote(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	id, ok := objID(w, r)
	if !ok {
		return
	}
	var in cardInput
	_ = decodeBody(r, &in)
	voter := clientVoter(r, in.VoterID)

	var card Card
	if err := cards().FindOne(ctx, bson.M{"_id": id}).Decode(&card); err != nil {
		writeErr(w, 404, "Card not found")
		return
	}
	voted := slices.Contains(card.Voters, voter)
	var update bson.M
	if voted {
		update = bson.M{"$pull": bson.M{"voters": voter}, "$inc": bson.M{"votes": -1}}
	} else {
		update = bson.M{"$addToSet": bson.M{"voters": voter}, "$inc": bson.M{"votes": 1}}
	}
	var after Card
	err := cards().FindOneAndUpdate(ctx, bson.M{"_id": id}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&after)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if after.Votes < 0 {
		_, _ = cards().UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"votes": 0}})
		after.Votes = 0
	}
	writeJSON(w, 200, map[string]any{"votes": after.Votes, "voted": slices.Contains(after.Voters, voter)})
}

func handleListComments(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	id, ok := objID(w, r)
	if !ok {
		return
	}
	list := []Comment{}
	cur, err := comments().Find(ctx, bson.M{"cardId": id}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := cur.All(ctx, &list); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"comments": list})
}

func handleCreateComment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	id, ok := objID(w, r)
	if !ok {
		return
	}
	var in struct {
		Content string `json:"content"`
	}
	if err := decodeBody(r, &in); err != nil {
		writeErr(w, 400, "invalid JSON body")
		return
	}
	content := strings.TrimSpace(in.Content)
	if content == "" {
		writeErr(w, 400, "content is required")
		return
	}
	if len([]rune(content)) > 1000 {
		writeErr(w, 400, "comment must be 1000 characters or less")
		return
	}
	if err := cards().FindOne(ctx, bson.M{"_id": id}).Err(); err != nil {
		writeErr(w, 404, "Card not found")
		return
	}
	now := time.Now().UTC()
	comment := Comment{ID: primitive.NewObjectID(), CardID: id, Content: content, CreatedAt: now, UpdatedAt: now}
	if _, err := comments().InsertOne(ctx, comment); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, comment)
}

func regexEscape(s string) string {
	specials := `\.+*?()|[]{}^$`
	var b strings.Builder
	for _, c := range s {
		if strings.ContainsRune(specials, c) {
			b.WriteRune('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}
