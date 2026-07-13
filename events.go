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

func handlePublicEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	q := r.URL.Query()
	filter := bson.M{"status": "approved"}
	if from := q.Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filter["startAt"] = bson.M{"$gte": t}
		}
	}
	if to := q.Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filter["endAt"] = bson.M{"$lte": t}
		}
	}
	list := []Event{}
	cur, err := events().Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "startAt", Value: 1}}).SetLimit(300))
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
		out = append(out, publicEvent{e.ID, e.Title, e.Description, e.StartAt, e.EndAt, e.Location, e.Category})
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
	ev := Event{
		ID:           primitive.NewObjectID(),
		Title:        in.Title,
		Description:  strings.TrimSpace(in.Description),
		StartAt:      startAt,
		EndAt:        endAt,
		Location:     strings.TrimSpace(in.Location),
		Category:     strings.TrimSpace(in.Category),
		ContactEmail: strings.TrimSpace(in.ContactEmail),
		Status:       "pending",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := events().InsertOne(ctx, ev); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{
		"_id": ev.ID, "status": ev.Status,
		"message": "Événement proposé — il sera visible après validation.",
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
