// Package lostitems is the Go port of the Node lost-items mini-app:
// declare and browse lost & found objects, on the same "lostitems"
// collection.
package lostitems

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/javimosch/enbauges-go/plugin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//go:embed web
var webFS embed.FS

const prefix = "/perdu-trouve"

type Item struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Location    string             `bson:"location,omitempty" json:"location,omitempty"`
	DateLost    *time.Time         `bson:"dateLost,omitempty" json:"dateLost,omitempty"`
	Contact     string             `bson:"contact,omitempty" json:"contact,omitempty"`
	Status      string             `bson:"status" json:"status"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type LostItems struct{}

func New() *LostItems { return &LostItems{} }

func (p *LostItems) Meta() plugin.Meta {
	return plugin.Meta{
		ID:          "lost-items",
		Name:        "Objets Perdus & Trouvés",
		Version:     "1.0.0",
		Description: "Déclarer et consulter les objets perdus ou trouvés dans le territoire",
		RoutePrefix: prefix,
		Aliases:     []string{"/lost-objects", "/perdido"},
		Tags:        []string{"community", "tools", "lost-and-found"},
	}
}

func (p *LostItems) WebFS() fs.FS {
	sub, _ := fs.Sub(webFS, "web")
	return sub
}

func (p *LostItems) Install(ctx *plugin.Context) error {
	return ctx.UpsertServiceCard(plugin.ServiceCard{
		Type:        "solution",
		Title:       "Objets Perdus & Trouvés",
		Description: "Declarer et consulter les objets perdus ou trouves dans le territoire",
		URL:         prefix,
		Tags:        []string{"service", "communaute", "outil"},
	})
}

func (p *LostItems) Bootstrap(ctx *plugin.Context) error { return nil }

func (p *LostItems) Mount(mux *http.ServeMux, ctx *plugin.Context) error {
	mux.HandleFunc("GET "+prefix, plugin.ServeWebFile(ctx, "lost-items.html"))

	mux.HandleFunc("GET "+prefix+"/api", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		filter := bson.M{}
		if s := r.URL.Query().Get("status"); s != "" && s != "all" {
			filter["status"] = s
		}
		items := []Item{}
		cur, err := ctx.DB.Collection("lostitems").Find(dbctx, filter,
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &items); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"items": items})
	})

	mux.HandleFunc("POST "+prefix+"/api", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		var in struct {
			Title, Description, Location, DateLost, Contact, Status string
		}
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		title := strings.TrimSpace(in.Title)
		if title == "" {
			plugin.WriteErr(w, 400, "Title is required")
			return
		}
		if len([]rune(title)) > 200 {
			plugin.WriteErr(w, 400, "Title must be 200 characters or less")
			return
		}
		if len([]rune(in.Description)) > 1000 {
			plugin.WriteErr(w, 400, "Description must be 1000 characters or less")
			return
		}
		status := "lost"
		if in.Status == "found" {
			status = "found"
		}
		now := time.Now().UTC()
		item := Item{
			ID:          primitive.NewObjectID(),
			Title:       title,
			Description: strings.TrimSpace(in.Description),
			Location:    strings.TrimSpace(in.Location),
			Contact:     strings.TrimSpace(in.Contact),
			Status:      status,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if in.DateLost != "" {
			if t, err := time.Parse(time.RFC3339, in.DateLost); err == nil {
				item.DateLost = &t
			}
		}
		if _, err := ctx.DB.Collection("lostitems").InsertOne(dbctx, item); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 201, item)
	})

	mux.HandleFunc("DELETE "+prefix+"/api/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
		if err != nil {
			plugin.WriteErr(w, 404, "Item not found")
			return
		}
		res, err := ctx.DB.Collection("lostitems").DeleteOne(dbctx, bson.M{"_id": id})
		if err != nil || res.DeletedCount == 0 {
			plugin.WriteErr(w, 404, "Item not found")
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"success": true})
	})
	return nil
}
