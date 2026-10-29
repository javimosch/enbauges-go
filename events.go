package main

import (
	"crypto/subtle"

	"github.com/javimosch/enbauges-go/plugin"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// The shared calendar keeps the Node app's data model (orgevents collection,
// same statuses) but replaces the org-membership workflow with the vision's
// zero-friction one: anyone proposes an event (pending), an admin with the
// ADMIN_TOKEN approves or rejects it.

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if adminToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
			writeErr(w, 401, "unauthorized")
			return
		}
		next(w, r)
	}
}

// calEntries is the canonical event store — the calendar plugin's
// "calendarentries" collection. The older "orgevents" collection is retired.
func calEntries() *mongo.Collection { return db.Collection("calendarentries") }

type calEntry struct {
	ID          primitive.ObjectID `bson:"_id"`
	Title       string             `bson:"title"`
	Description string             `bson:"description"`
	StartDate   time.Time          `bson:"startDate"`
	EndDate     *time.Time         `bson:"endDate"`
	Location    string             `bson:"location"`
	Tags        []string           `bson:"tags"`
	Frequency   string             `bson:"frequency"`
}

func handlePublicEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	q := r.URL.Query()
	// calendarentries is the canonical event store (calendar plugin).
	filter := bson.M{"type": "event"}
	if from := q.Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filter["startDate"] = bson.M{"$gte": t}
		}
	}
	// Hide one-time events that ended more than 30 days ago; recurring and
	// open-ended entries stay visible.
	cutoff := bson.M{"$gte": time.Now().Add(-30 * 24 * time.Hour)}
	if to := q.Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			cutoff["$lte"] = t
		}
	}
	filter["$or"] = bson.A{
		bson.M{"frequency": bson.M{"$ne": "one-time"}},
		bson.M{"endDate": nil},
		bson.M{"endDate": cutoff},
	}
	list := []calEntry{}
	cur, err := calEntries().Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "startDate", Value: -1}}).SetLimit(300))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := cur.All(ctx, &list); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	// Public payload: no contactEmail leak.
	type publicEvent struct {
		ID          primitive.ObjectID `json:"_id"`
		Title       string             `json:"title"`
		Description string             `json:"description,omitempty"`
		StartAt     time.Time          `json:"startAt"`
		EndAt       time.Time          `json:"endAt"`
		Location    string             `json:"location,omitempty"`
		Category    string             `json:"category,omitempty"`
	}
	out := make([]publicEvent, 0, len(list))
	for _, e := range list {
		var endAt time.Time
		if e.EndDate != nil {
			endAt = *e.EndDate
		}
		cat := ""
		if len(e.Tags) > 0 {
			cat = e.Tags[0]
		}
		out = append(out, publicEvent{e.ID, e.Title, e.Description, e.StartDate, endAt, e.Location, cat})
	}
	writeJSON(w, 200, map[string]any{"events": out})
}

func handleProposeEvent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	var in struct {
		Title        string `json:"title"`
		Description  string `json:"description"`
		StartAt      string `json:"startAt"`
		EndAt        string `json:"endAt"`
		Location     string `json:"location"`
		Category     string `json:"category"`
		ContactEmail string `json:"contactEmail"`
	}
	if err := decodeBody(r, &in); err != nil {
		writeErr(w, 400, "invalid JSON body")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" || in.StartAt == "" || in.EndAt == "" {
		writeErr(w, 400, "title, startAt, and endAt are required")
		return
	}
	if len([]rune(in.Title)) > 200 || len([]rune(in.Description)) > 2000 {
		writeErr(w, 400, "title or description too long")
		return
	}
	startAt, ok1 := plugin.ParseDate(in.StartAt)
	endAt, ok2 := plugin.ParseDate(in.EndAt)
	if !ok1 || !ok2 {
		writeErr(w, 400, "startAt and endAt must be ISO 8601 dates")
		return
	}
	if !endAt.After(startAt) {
		writeErr(w, 400, "endAt must be after startAt")
		return
	}
	now := time.Now().UTC()
	ev := bson.M{
		"_id":          primitive.NewObjectID(),
		"type":         "event",
		"title":        in.Title,
		"description":  strings.TrimSpace(in.Description),
		"startDate":    startAt,
		"endDate":      endAt,
		"frequency":    "one-time",
		"location":     strings.TrimSpace(in.Location),
		"contactEmail": strings.TrimSpace(in.ContactEmail),
		"createdAt":    now,
		"updatedAt":    now,
	}
	if cat := strings.TrimSpace(in.Category); cat != "" {
		ev["tags"] = []string{cat}
	}
	if _, err := calEntries().InsertOne(ctx, ev); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{
		"_id": ev["_id"], "status": "approved",
		"message": "Événement publié — il est visible immédiatement.",
	})
}

func handleAdminListEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	filter := bson.M{"status": status}
	list := []Event{}
	cur, err := events().Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "startAt", Value: 1}}).SetLimit(500))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := cur.All(ctx, &list); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"events": list})
}

func handleModerateEvent(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := reqCtx(r)
		defer cancel()
		id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
		if err != nil {
			writeErr(w, 404, "Event not found")
			return
		}
		var ev Event
		err = events().FindOneAndUpdate(ctx, bson.M{"_id": id},
			bson.M{"$set": bson.M{"status": status, "updatedAt": time.Now().UTC()}},
			options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&ev)
		if err == mongo.ErrNoDocuments {
			writeErr(w, 404, "Event not found")
			return
		}
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, ev)
	}
}
