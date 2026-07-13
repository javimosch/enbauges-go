package plugin

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Meta mirrors the Node plugin meta contract.
type Meta struct {
	ID          string
	Name        string
	Version     string
	Description string
	RoutePrefix string   // e.g. "/calendrier"
	Aliases     []string // extra prefixes redirected to RoutePrefix
	Tags        []string
}

// ServiceCard is the canvas card a plugin upserts to announce itself,
// matching the Node SERVICE_CARD convention (keyed by url).
type ServiceCard struct {
	Type        string
	Title       string
	Description string
	URL         string
	Tags        []string
}

// Plugin is the contract every mini-app implements.
type Plugin interface {
	Meta() Meta
	// Mount registers the plugin's handlers. Patterns are plain
	// net/http patterns; the core passes a mux already scoped so the
	// plugin registers absolute paths under its RoutePrefix.
	Mount(mux *http.ServeMux, ctx *Context) error
	// Install runs once per version (tracked in the pluginstate collection).
	Install(ctx *Context) error
	// Bootstrap runs on every startup, after Install.
	Bootstrap(ctx *Context) error
}

// Context is what the core hands to plugins — the equivalent of the
// superbackend plugin ctx.
type Context struct {
	DB  *mongo.Database
	Log *log.Logger
	// Web resolves the plugin's UI files: disk override under
	// <WEB_DIR>/plugins/<id>/web first, embedded copy second.
	Web *OverlayFS
	Env func(key, fallback string) string
}

// UpsertServiceCard announces the plugin on the canvas (idempotent, keyed
// by card url), like the Node plugins do.
func (c *Context) UpsertServiceCard(card ServiceCard) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	now := time.Now().UTC()
	_, err := c.DB.Collection("cards").UpdateOne(ctx,
		bson.M{"url": card.URL},
		bson.M{
			"$set": bson.M{
				"type": card.Type, "title": card.Title, "description": card.Description,
				"tags": card.Tags, "updatedAt": now,
			},
			"$setOnInsert": bson.M{"votes": 0, "voters": []string{}, "isExample": false, "createdAt": now},
		},
		options.Update().SetUpsert(true))
	if err == nil {
		c.Log.Println("service card upserted")
	}
	return err
}

// state persisted per plugin in the pluginstate collection, mirroring the
// Node loader's {enabled, installedAt} bookkeeping plus the version so
// Install re-runs on upgrades.
type stateDoc struct {
	PluginID         string     `bson:"pluginId"`
	InstalledVersion string     `bson:"installedVersion"`
	InstalledAt      *time.Time `bson:"installedAt"`
}

// Setup installs (if needed), bootstraps, and mounts every enabled plugin.
// Disable via PLUGINS_DISABLED=comma,separated,ids.
func Setup(mux *http.ServeMux, db *mongo.Database, plugins []Plugin) {
	disabled := map[string]bool{}
	for _, id := range splitCSV(os.Getenv("PLUGINS_DISABLED")) {
		disabled[id] = true
	}
	states := db.Collection("pluginstate")

	for _, p := range plugins {
		meta := p.Meta()
		logger := log.New(os.Stderr, "["+meta.ID+"] ", log.LstdFlags)
		if disabled[meta.ID] {
			logger.Println("disabled via PLUGINS_DISABLED")
			continue
		}
		pctx := &Context{DB: db, Log: logger, Env: envFn}
		if hw, ok := p.(HasWeb); ok {
			pctx.Web = pluginOverlay(meta.ID, hw.WebFS())
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var st stateDoc
		err := states.FindOne(ctx, bson.M{"pluginId": meta.ID}).Decode(&st)
		needsInstall := err == mongo.ErrNoDocuments || st.InstalledVersion != meta.Version
		if needsInstall {
			if ierr := p.Install(pctx); ierr != nil {
				logger.Println("install failed:", ierr)
				cancel()
				continue
			}
			now := time.Now().UTC()
			_, _ = states.UpdateOne(ctx, bson.M{"pluginId": meta.ID},
				bson.M{"$set": bson.M{"installedVersion": meta.Version, "installedAt": now}},
				options.Update().SetUpsert(true))
			logger.Println("installed version", meta.Version)
		}
		cancel()

		if err := p.Bootstrap(pctx); err != nil {
			logger.Println("bootstrap failed:", err)
			continue
		}
		if err := p.Mount(mux, pctx); err != nil {
			logger.Println("mount failed:", err)
			continue
		}
		if pctx.Web != nil {
			// Same URL convention as the Node loader: plugin assets are
			// served at /plugin-static/<prefix>/<file>.
			staticBase := "/plugin-static/" + strings.TrimPrefix(meta.RoutePrefix, "/") + "/"
			fileServer := http.StripPrefix(staticBase, http.FileServerFS(pctx.Web))
			mux.HandleFunc("GET "+staticBase, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-cache")
				fileServer.ServeHTTP(w, r)
			})
		}
		for _, alias := range meta.Aliases {
			target := meta.RoutePrefix
			mux.HandleFunc("GET "+alias, func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target, http.StatusMovedPermanently)
			})
		}
		logger.Println("mounted at", meta.RoutePrefix)
	}
}

// HasWeb is implemented by plugins that ship UI files. The embedded FS is
// overlaid with <WEB_DIR_PLUGINS>/<id>/web so plugin UI files are
// live-editable on disk exactly like the core's.
type HasWeb interface {
	WebFS() fs.FS
}

func pluginOverlay(id string, embedded fs.FS) *OverlayFS {
	diskDir := filepath.Join(envFn("WEB_DIR_PLUGINS", "./plugins"), id, "web")
	if _, err := os.Stat(diskDir); err != nil {
		diskDir = ""
	}
	return NewOverlayFS(diskDir, embedded)
}

func envFn(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
