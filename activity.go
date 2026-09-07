package main

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Activity is a public log entry written by a virtual volunteer (LLM agent)
// of the enbauges-federator fleet. The feed is visible at /team/activity.
type Activity struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Agent     string             `bson:"agent" json:"agent"`         // volunteer name, e.g. "Aurélie"
	Loop      string             `bson:"loop" json:"loop"`           // fleet loop name
	Action    string             `bson:"action" json:"action"`       // short verb: "created", "scraped", "discovered"
	Detail    string             `bson:"detail" json:"detail"`       // human-readable summary
	Items     int                `bson:"items,omitempty" json:"items,omitempty"` // count of items affected
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

func activities() *mongo.Collection { return db.Collection("activities") }

func activityIndexes() []mongo.IndexModel {
	return []mongo.IndexModel{
		{Keys: bson.D{{Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "agent", Value: 1}, {Key: "createdAt", Value: -1}}},
	}
}

// POST /api/activity — write a new activity entry. Requires admin token
// (same token the fleet already uses for event creation).
func handleCreateActivity(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if adminToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
		writeErr(w, 401, "unauthorized")
		return
	}
	var act Activity
	if err := decodeBody(r, &act); err != nil {
		writeErr(w, 400, "invalid JSON body")
		return
	}
	if act.Agent == "" {
		writeErr(w, 400, "agent is required")
		return
	}
	if act.Loop == "" {
		act.Loop = "unknown"
	}
	if act.Action == "" {
		act.Action = "reported"
	}
	if act.Detail == "" {
		act.Detail = "Activity reported"
	}
	act.ID = primitive.NewObjectID()
	if act.CreatedAt.IsZero() {
		act.CreatedAt = time.Now().UTC()
	}
	_, err := activities().InsertOne(r.Context(), act)
	if err != nil {
		writeErr(w, 500, "failed to save activity")
		return
	}
	writeJSON(w, 201, act)
}

// GET /api/activity — public read of recent activity.
func handleListActivity(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	cur, err := activities().Find(ctx,
		bson.M{},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		writeJSON(w, 200, map[string]any{"activities": []any{}})
		return
	}
	var list []Activity
	_ = cur.All(ctx, &list)
	if list == nil {
		list = []Activity{}
	}
	writeJSON(w, 200, map[string]any{"activities": list, "total": len(list)})
}

// GET /team/activity — public HTML page showing the activity feed.
func handleTeamActivityPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()

	var list []Activity
	cur, err := activities().Find(ctx,
		bson.M{},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100))
	if err == nil {
		_ = cur.All(ctx, &list)
	}
	if list == nil {
		list = []Activity{}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, err := getTemplate("team-activity.html")
	if err != nil {
		http.Error(w, "template error", 500)
		return
	}
	if err := tmpl.Execute(w, map[string]any{
		"Activities": list,
	}); err != nil {
		log.Println("team-activity template:", err)
	}
}
