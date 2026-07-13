// Package calendar is the Go port of the Node calendar mini-app
// (plugins/calendar): a shared community calendar of events and recurring
// templates, on the same "calendarentries" collection.
package calendar

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"slices"
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

const prefix = "/calendrier"

type Comment struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Content   string             `bson:"content" json:"content"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

type Entry struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Type        string             `bson:"type" json:"type"`
	StartDate   time.Time          `bson:"startDate" json:"startDate"`
	EndDate     *time.Time         `bson:"endDate" json:"endDate"`
	Frequency   string             `bson:"frequency" json:"frequency"`
	Weekdays    []int              `bson:"weekdays,omitempty" json:"weekdays,omitempty"`
	MonthDays   []int              `bson:"monthDays,omitempty" json:"monthDays,omitempty"`
	Exceptions  []time.Time        `bson:"exceptions,omitempty" json:"exceptions,omitempty"`
	Location    string             `bson:"location,omitempty" json:"location,omitempty"`
	Organizer   string             `bson:"organizer,omitempty" json:"organizer,omitempty"`
	Contact     string             `bson:"contact,omitempty" json:"contact,omitempty"`
	URL         string             `bson:"url,omitempty" json:"url,omitempty"`
	Tags        []string           `bson:"tags" json:"tags"`
	Comments    []Comment          `bson:"comments" json:"comments"`
	Votes       int                `bson:"votes" json:"votes"`
	Voters      []string           `bson:"voters" json:"-"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type Calendar struct{}

func New() *Calendar { return &Calendar{} }

func (c *Calendar) Meta() plugin.Meta {
	return plugin.Meta{
		ID:          "calendar",
		Name:        "Calendrier Partage",
		Version:     "1.0.0",
		Description: "Partagez et decouvrez les evenements du territoire",
		RoutePrefix: prefix,
		Aliases:     []string{"/calendrier-partage", "/calendar", "/calendario"},
		Tags:        []string{"community", "calendar", "events"},
	}
}

func (c *Calendar) WebFS() fs.FS {
	sub, _ := fs.Sub(webFS, "web")
	return sub
}

func (c *Calendar) Install(ctx *plugin.Context) error {
	return ctx.UpsertServiceCard(plugin.ServiceCard{
		Type:        "solution",
		Title:       "Calendrier Partage",
		Description: "Partagez et decouvrez les evenements du territoire pour eviter les collisions",
		URL:         prefix,
		Tags:        []string{"service", "calendrier", "evenements", "communaute"},
	})
}

func (c *Calendar) Bootstrap(ctx *plugin.Context) error { return nil }

func (c *Calendar) col(ctx *plugin.Context) *mongo.Collection {
	return ctx.DB.Collection("calendarentries")
}

func (c *Calendar) Mount(mux *http.ServeMux, ctx *plugin.Context) error {
	mux.HandleFunc("GET "+prefix, plugin.ServeWebFile(ctx, "calendar.html"))
	mux.HandleFunc("GET "+prefix+"/api", func(w http.ResponseWriter, r *http.Request) { c.list(w, r, ctx) })
	mux.HandleFunc("POST "+prefix+"/api", func(w http.ResponseWriter, r *http.Request) { c.create(w, r, ctx) })
	mux.HandleFunc("GET "+prefix+"/api/{id}", func(w http.ResponseWriter, r *http.Request) { c.get(w, r, ctx) })
	mux.HandleFunc("PUT "+prefix+"/api/{id}", func(w http.ResponseWriter, r *http.Request) { c.update(w, r, ctx) })
	mux.HandleFunc("DELETE "+prefix+"/api/{id}", func(w http.ResponseWriter, r *http.Request) { c.delete(w, r, ctx) })
	mux.HandleFunc("POST "+prefix+"/api/{id}/vote", func(w http.ResponseWriter, r *http.Request) { c.vote(w, r, ctx) })
	mux.HandleFunc("GET "+prefix+"/api/{id}/comments", func(w http.ResponseWriter, r *http.Request) { c.listComments(w, r, ctx) })
	mux.HandleFunc("POST "+prefix+"/api/{id}/comments", func(w http.ResponseWriter, r *http.Request) { c.addComment(w, r, ctx) })
	return nil
}

func entryID(w http.ResponseWriter, r *http.Request) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		plugin.WriteErr(w, 404, "Evenement non trouve")
		return primitive.NilObjectID, false
	}
	return id, true
}

func (c *Calendar) findEntry(ctx *plugin.Context, id primitive.ObjectID) (*Entry, bool) {
	dbctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var e Entry
	if err := c.col(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&e); err != nil {
		return nil, false
	}
	return &e, true
}

func (c *Calendar) list(w http.ResponseWriter, r *http.Request, ctx *plugin.Context) {
	dbctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	q := r.URL.Query()
	filter := bson.M{}
	if t := q.Get("type"); t == "event" || t == "template" {
		filter["type"] = t
	}
	if tag := q.Get("tag"); tag != "" {
		filter["tags"] = tag
	}
	entries := []Entry{}
	cur, err := c.col(ctx).Find(dbctx, filter,
		options.Find().SetSort(bson.D{{Key: "startDate", Value: 1}, {Key: "createdAt", Value: -1}}).SetLimit(200))
	if err != nil {
		plugin.WriteErr(w, 500, err.Error())
		return
	}
	if err := cur.All(dbctx, &entries); err != nil {
		plugin.WriteErr(w, 500, err.Error())
		return
	}
	if m, y := q.Get("month"), q.Get("year"); m != "" && y != "" {
		mn, _ := strconv.Atoi(m)
		yn, _ := strconv.Atoi(y)
		filtered := entries[:0]
		for _, e := range entries {
			if int(e.StartDate.Month()) == mn && e.StartDate.Year() == yn {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	plugin.WriteJSON(w, 200, map[string]any{"entries": entries})
}

func (c *Calendar) get(w http.ResponseWriter, r *http.Request, ctx *plugin.Context) {
	id, ok := entryID(w, r)
	if !ok {
		return
	}
	e, found := c.findEntry(ctx, id)
	if !found {
		plugin.WriteErr(w, 404, "Evenement non trouve")
		return
	}
	plugin.WriteJSON(w, 200, e)
}

type entryInput struct {
	Title       *string          `json:"title"`
	Description *string          `json:"description"`
	Type        *string          `json:"type"`
	StartDate   *string          `json:"startDate"`
	EndDate     *string          `json:"endDate"`
	Frequency   *string          `json:"frequency"`
	Weekdays    []plugin.FlexInt `json:"weekdays"`
	MonthDays   []plugin.FlexInt `json:"monthDays"`
	Exceptions  []string         `json:"exceptions"`
	Location    *string          `json:"location"`
	Organizer   *string          `json:"organizer"`
	Contact     *string          `json:"contact"`
	URL         *string          `json:"url"`
	Tags        []string         `json:"tags"`
	VoterID     string           `json:"voterId"`
	Content     string           `json:"content"`
}

func parseDates(vals []string) []time.Time {
	out := []time.Time{}
	for _, v := range vals {
		if t, ok := plugin.ParseDate(v); ok {
			out = append(out, t)
		}
	}
	return out
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func (c *Calendar) create(w http.ResponseWriter, r *http.Request, ctx *plugin.Context) {
	dbctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var in entryInput
	if err := plugin.DecodeBody(r, &in); err != nil {
		plugin.WriteErr(w, 400, "invalid JSON body")
		return
	}
	title := str(in.Title)
	if title == "" {
		plugin.WriteErr(w, 400, "Le titre est requis")
		return
	}
	typ := str(in.Type)
	if typ != "event" && typ != "template" {
		plugin.WriteErr(w, 400, "Type invalide")
		return
	}
	if in.StartDate == nil || *in.StartDate == "" {
		plugin.WriteErr(w, 400, "La date de debut est requise")
		return
	}
	freq := str(in.Frequency)
	if !slices.Contains([]string{"one-time", "weekly", "monthly"}, freq) {
		plugin.WriteErr(w, 400, "Frequence invalide")
		return
	}
	if len([]rune(title)) > 200 {
		plugin.WriteErr(w, 400, "Le titre doit faire moins de 200 caracteres")
		return
	}
	startDate, ok := plugin.ParseDate(*in.StartDate)
	if !ok {
		plugin.WriteErr(w, 400, "La date de debut est requise")
		return
	}

	now := time.Now().UTC()
	e := Entry{
		ID:          primitive.NewObjectID(),
		Title:       title,
		Description: str(in.Description),
		Type:        typ,
		StartDate:   startDate,
		Frequency:   freq,
		Location:    str(in.Location),
		Organizer:   str(in.Organizer),
		Contact:     str(in.Contact),
		URL:         str(in.URL),
		Tags:        []string{},
		Comments:    []Comment{},
		Voters:      []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if in.EndDate != nil && *in.EndDate != "" {
		if t, ok := plugin.ParseDate(*in.EndDate); ok {
			e.EndDate = &t
		}
	}
	if freq == "weekly" {
		e.Weekdays = plugin.FlexInts(in.Weekdays)
	}
	if freq == "monthly" {
		e.MonthDays = plugin.FlexInts(in.MonthDays)
	}
	e.Exceptions = parseDates(in.Exceptions)
	for _, t := range in.Tags {
		if tt := strings.TrimSpace(t); tt != "" {
			e.Tags = append(e.Tags, tt)
		}
	}
	if _, err := c.col(ctx).InsertOne(dbctx, e); err != nil {
		plugin.WriteErr(w, 500, err.Error())
		return
	}
	plugin.WriteJSON(w, 201, e)
}

func (c *Calendar) update(w http.ResponseWriter, r *http.Request, ctx *plugin.Context) {
	dbctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	id, ok := entryID(w, r)
	if !ok {
		return
	}
	var in entryInput
	if err := plugin.DecodeBody(r, &in); err != nil {
		plugin.WriteErr(w, 400, "invalid JSON body")
		return
	}
	set := bson.M{"updatedAt": time.Now().UTC()}
	if in.Title != nil {
		set["title"] = str(in.Title)
	}
	if in.Description != nil {
		set["description"] = str(in.Description)
	}
	if in.Type != nil {
		set["type"] = str(in.Type)
	}
	if in.StartDate != nil {
		if t, ok := plugin.ParseDate(*in.StartDate); ok {
			set["startDate"] = t
		}
	}
	if in.EndDate != nil {
		if *in.EndDate == "" {
			set["endDate"] = nil
		} else if t, ok := plugin.ParseDate(*in.EndDate); ok {
			set["endDate"] = t
		}
	}
	if in.Frequency != nil {
		set["frequency"] = str(in.Frequency)
	}
	if in.Weekdays != nil {
		set["weekdays"] = plugin.FlexInts(in.Weekdays)
	}
	if in.MonthDays != nil {
		set["monthDays"] = plugin.FlexInts(in.MonthDays)
	}
	if in.Exceptions != nil {
		set["exceptions"] = parseDates(in.Exceptions)
	}
	if in.Location != nil {
		set["location"] = str(in.Location)
	}
	if in.Organizer != nil {
		set["organizer"] = str(in.Organizer)
	}
	if in.Contact != nil {
		set["contact"] = str(in.Contact)
	}
	if in.URL != nil {
		set["url"] = str(in.URL)
	}
	if in.Tags != nil {
		tags := []string{}
		for _, t := range in.Tags {
			if tt := strings.TrimSpace(t); tt != "" {
				tags = append(tags, tt)
			}
		}
		set["tags"] = tags
	}
	var e Entry
	err := c.col(ctx).FindOneAndUpdate(dbctx, bson.M{"_id": id}, bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&e)
	if err != nil {
		plugin.WriteErr(w, 404, "Evenement non trouve")
		return
	}
	plugin.WriteJSON(w, 200, e)
}

func (c *Calendar) delete(w http.ResponseWriter, r *http.Request, ctx *plugin.Context) {
	dbctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	id, ok := entryID(w, r)
	if !ok {
		return
	}
	res, err := c.col(ctx).DeleteOne(dbctx, bson.M{"_id": id})
	if err != nil || res.DeletedCount == 0 {
		plugin.WriteErr(w, 404, "Evenement non trouve")
		return
	}
	plugin.WriteJSON(w, 200, map[string]any{"success": true})
}

func (c *Calendar) vote(w http.ResponseWriter, r *http.Request, ctx *plugin.Context) {
	dbctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	id, ok := entryID(w, r)
	if !ok {
		return
	}
	var in entryInput
	_ = plugin.DecodeBody(r, &in)
	if in.VoterID == "" {
		plugin.WriteErr(w, 400, "Voter ID requis")
		return
	}
	e, found := c.findEntry(ctx, id)
	if !found {
		plugin.WriteErr(w, 404, "Evenement non trouve")
		return
	}
	voted := slices.Contains(e.Voters, in.VoterID)
	var update bson.M
	if voted {
		update = bson.M{"$pull": bson.M{"voters": in.VoterID}, "$inc": bson.M{"votes": -1}}
	} else {
		update = bson.M{"$addToSet": bson.M{"voters": in.VoterID}, "$inc": bson.M{"votes": 1}}
	}
	var after Entry
	if err := c.col(ctx).FindOneAndUpdate(dbctx, bson.M{"_id": id}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&after); err != nil {
		plugin.WriteErr(w, 500, err.Error())
		return
	}
	if after.Votes < 0 {
		_, _ = c.col(ctx).UpdateOne(dbctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"votes": 0}})
		after.Votes = 0
	}
	plugin.WriteJSON(w, 200, map[string]any{"votes": after.Votes, "voted": !voted})
}

func (c *Calendar) listComments(w http.ResponseWriter, r *http.Request, ctx *plugin.Context) {
	id, ok := entryID(w, r)
	if !ok {
		return
	}
	e, found := c.findEntry(ctx, id)
	if !found {
		plugin.WriteErr(w, 404, "Evenement non trouve")
		return
	}
	plugin.WriteJSON(w, 200, map[string]any{"comments": e.Comments})
}

func (c *Calendar) addComment(w http.ResponseWriter, r *http.Request, ctx *plugin.Context) {
	dbctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	id, ok := entryID(w, r)
	if !ok {
		return
	}
	var in entryInput
	if err := plugin.DecodeBody(r, &in); err != nil {
		plugin.WriteErr(w, 400, "invalid JSON body")
		return
	}
	content := strings.TrimSpace(in.Content)
	if content == "" {
		plugin.WriteErr(w, 400, "Comment content required")
		return
	}
	comment := Comment{ID: primitive.NewObjectID(), Content: content, CreatedAt: time.Now().UTC()}
	res, err := c.col(ctx).UpdateOne(dbctx, bson.M{"_id": id},
		bson.M{"$push": bson.M{"comments": comment}, "$set": bson.M{"updatedAt": time.Now().UTC()}})
	if err != nil || res.MatchedCount == 0 {
		plugin.WriteErr(w, 404, "Evenement non trouve")
		return
	}
	plugin.WriteJSON(w, 201, map[string]any{"comment": comment})
}
