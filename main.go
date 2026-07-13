// enbauges-go — clean-room Go implementation of enbauges.fr (see docs/VISION.md
// in the original repo). Migrates the used core only: canvas (cards, links,
// comments, votes), public pages, open data exports, and the shared calendar.
// Drop-in on the same MongoDB database as the Node app.
package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//go:embed web
var webFS embed.FS

var (
	db         *mongo.Database
	adminToken string
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	port := env("PORT", "3000")
	uri := env("MONGODB_URI", "mongodb://127.0.0.1:27017/enbauges")
	dbName := env("MONGODB_DB", "enbauges")
	adminToken = os.Getenv("ADMIN_TOKEN")
	if adminToken == "" {
		log.Println("WARN: ADMIN_TOKEN not set — event moderation endpoints disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetMaxPoolSize(20))
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal("mongo unreachable: ", err)
	}
	db = client.Database(dbName)
	ensureIndexes(context.Background())

	mux := http.NewServeMux()

	// Canvas API (same shapes as the Node app)
	mux.HandleFunc("GET /api/cards", handleListCards)
	mux.HandleFunc("POST /api/cards", handleCreateCard)
	mux.HandleFunc("GET /api/cards/{id}", handleGetCard)
	mux.HandleFunc("PUT /api/cards/{id}", handleUpdateCard)
	mux.HandleFunc("DELETE /api/cards/{id}", handleDeleteCard)
	mux.HandleFunc("POST /api/cards/{id}/vote", handleVote)
	mux.HandleFunc("GET /api/cards/{id}/comments", handleListComments)
	mux.HandleFunc("POST /api/cards/{id}/comments", handleCreateComment)
	mux.HandleFunc("GET /api/links", handleListLinks)
	mux.HandleFunc("POST /api/links", handleCreateLink)
	mux.HandleFunc("DELETE /api/links/{id}", handleDeleteLink)

	// Shared calendar
	mux.HandleFunc("GET /api/events/public", handlePublicEvents)
	mux.HandleFunc("POST /api/events", handleProposeEvent)
	mux.HandleFunc("GET /api/admin/events", requireAdmin(handleAdminListEvents))
	mux.HandleFunc("POST /api/admin/events/{id}/approve", requireAdmin(handleModerateEvent("approved")))
	mux.HandleFunc("POST /api/admin/events/{id}/reject", requireAdmin(handleModerateEvent("rejected")))

	// Open data exports
	mux.HandleFunc("GET /api/export", handleExportIndex)
	mux.HandleFunc("GET /api/export/cards.json", handleExportCardsJSON)
	mux.HandleFunc("GET /api/export/cards.csv", handleExportCardsCSV)
	mux.HandleFunc("GET /api/export/cards.geojson", handleExportCardsGeoJSON)
	mux.HandleFunc("GET /api/export/events.json", handleExportEventsJSON)
	mux.HandleFunc("GET /api/export/events.ics", handleExportEventsICS)

	// Pages
	mux.HandleFunc("GET /{$}", servePage("canvas.html"))
	mux.HandleFunc("GET /canvas", servePage("canvas.html"))
	mux.HandleFunc("GET /agenda", handleAgendaPage)
	mux.HandleFunc("GET /annuaire", handleAnnuairePage)
	mux.HandleFunc("GET /donnees-ouvertes", servePage("open-data.html"))
	mux.HandleFunc("GET /legal", servePage("legal.html"))
	mux.HandleFunc("GET /privacy", servePage("privacy.html"))
	mux.HandleFunc("GET /cookies", servePage("cookies.html"))
	mux.HandleFunc("GET /css/", serveStatic)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"status": "ok"})
	})

	log.Printf("enbauges-go listening on :%s (db=%s)", port, dbName)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func ensureIndexes(ctx context.Context) {
	// Same indexes the mongoose schemas declare; no-ops if they already exist.
	c := db.Collection("cards")
	l := db.Collection("links")
	co := db.Collection("comments")
	e := db.Collection("orgevents")
	_, _ = c.Indexes().CreateMany(ctx, cardIndexes())
	_, _ = l.Indexes().CreateMany(ctx, linkIndexes())
	_, _ = co.Indexes().CreateMany(ctx, commentIndexes())
	_, _ = e.Indexes().CreateMany(ctx, eventIndexes())
}
