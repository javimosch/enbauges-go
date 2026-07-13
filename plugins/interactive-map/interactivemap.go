// Package interactivemap is the Go port of the Node interactive-map
// mini-app: a community Leaflet map of territory resources, on the same
// "mapmarkers" collection. Markers are owned by an anonymous creatorId;
// anyone can confirm, and repositioning is capped at 5 km to deter
// vandalism.
package interactivemap

import (
	"embed"
	"io/fs"
	"math"
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

const prefix = "/carte-interactive"

var validCategories = []string{"lieu", "service", "evenement", "autre"}

type Location struct {
	Lat     float64 `bson:"lat" json:"lat"`
	Lng     float64 `bson:"lng" json:"lng"`
	Address string  `bson:"address,omitempty" json:"address,omitempty"`
}

type Marker struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Title         string             `bson:"title" json:"title"`
	Description   string             `bson:"description,omitempty" json:"description,omitempty"`
	Category      string             `bson:"category" json:"category"`
	Location      Location           `bson:"location" json:"location"`
	Contact       string             `bson:"contact,omitempty" json:"contact,omitempty"`
	Tags          []string           `bson:"tags" json:"tags"`
	CreatorID     string             `bson:"creatorId" json:"creatorId"`
	Confirmations []string           `bson:"confirmations" json:"confirmations"`
	Status        string             `bson:"status" json:"status"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type InteractiveMap struct{}

func New() *InteractiveMap { return &InteractiveMap{} }

func (p *InteractiveMap) Meta() plugin.Meta {
	return plugin.Meta{
		ID:          "interactive-map",
		Name:        "Carte Interactive",
		Version:     "1.0.0",
		Description: "Explorez et contribuez à la carte des ressources du Massif des Bauges",
		RoutePrefix: prefix,
		Aliases:     []string{"/interactive-map", "/carte"},
		Tags:        []string{"community", "map", "territory", "resources"},
	}
}

func (p *InteractiveMap) WebFS() fs.FS {
	sub, _ := fs.Sub(webFS, "web")
	return sub
}

func (p *InteractiveMap) Install(ctx *plugin.Context) error {
	return ctx.UpsertServiceCard(plugin.ServiceCard{
		Type:        "solution",
		Title:       "Carte Interactive",
		Description: "Explorez et contribuez à la carte des ressources du Massif des Bauges",
		URL:         prefix,
		Tags:        []string{"service", "carte", "territoire", "communaute"},
	})
}

func (p *InteractiveMap) Bootstrap(ctx *plugin.Context) error { return nil }

func (p *InteractiveMap) col(ctx *plugin.Context) *mongo.Collection {
	return ctx.DB.Collection("mapmarkers")
}

type markerInput struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
	Location    *struct {
		Lat     *float64 `json:"lat"`
		Lng     *float64 `json:"lng"`
		Address *string  `json:"address"`
	} `json:"location"`
	Contact   *string  `json:"contact"`
	Tags      []string `json:"tags"`
	CreatorID string   `json:"creatorId"`
	VoterID   string   `json:"voterId"`
	Lat       *float64 `json:"lat"`
	Lng       *float64 `json:"lng"`
}

func cleanTags(tags []string) []string {
	out := []string{}
	for _, t := range tags {
		if tt := strings.TrimSpace(t); tt != "" {
			out = append(out, tt)
		}
	}
	return out
}

func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func (p *InteractiveMap) Mount(mux *http.ServeMux, ctx *plugin.Context) error {
	mux.HandleFunc("GET "+prefix, plugin.ServeWebFile(ctx, "interactive-map.html"))

	mux.HandleFunc("GET "+prefix+"/api", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		filter := bson.M{"status": "active"}
		if c := r.URL.Query().Get("category"); c != "" {
			cats := []string{}
			for _, cat := range strings.Split(c, ",") {
				if slices.Contains(validCategories, cat) {
					cats = append(cats, cat)
				}
			}
			if len(cats) > 0 {
				filter["category"] = bson.M{"$in": cats}
			}
		}
		if b := r.URL.Query().Get("bounds"); b != "" {
			parts := strings.Split(b, ",")
			if len(parts) == 4 {
				nums := make([]float64, 0, 4)
				for _, s := range parts {
					if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
						nums = append(nums, f)
					}
				}
				if len(nums) == 4 {
					filter["location.lat"] = bson.M{"$gte": nums[0], "$lte": nums[2]}
					filter["location.lng"] = bson.M{"$gte": nums[1], "$lte": nums[3]}
				}
			}
		}
		markers := []Marker{}
		cur, err := p.col(ctx).Find(dbctx, filter,
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(1000))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &markers); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"markers": markers})
	})

	mux.HandleFunc("GET "+prefix+"/api/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
		if err != nil {
			plugin.WriteErr(w, 404, "Marker not found")
			return
		}
		var m Marker
		if err := p.col(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&m); err != nil {
			plugin.WriteErr(w, 404, "Marker not found")
			return
		}
		plugin.WriteJSON(w, 200, m)
	})

	mux.HandleFunc("POST "+prefix+"/api", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		var in markerInput
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		title := ""
		if in.Title != nil {
			title = strings.TrimSpace(*in.Title)
		}
		switch {
		case title == "":
			plugin.WriteErr(w, 400, "Le titre est requis")
			return
		case len([]rune(title)) > 100:
			plugin.WriteErr(w, 400, "Le titre doit faire 100 caractères ou moins")
			return
		case in.Category == nil || !slices.Contains(validCategories, *in.Category):
			plugin.WriteErr(w, 400, "Catégorie invalide")
			return
		case in.Location == nil || in.Location.Lat == nil || in.Location.Lng == nil:
			plugin.WriteErr(w, 400, "Localisation valide requise (lat, lng)")
			return
		case strings.TrimSpace(in.CreatorID) == "":
			plugin.WriteErr(w, 400, "Identifiant créateur requis")
			return
		case in.Description != nil && len([]rune(*in.Description)) > 500:
			plugin.WriteErr(w, 400, "La description doit faire 500 caractères ou moins")
			return
		}
		now := time.Now().UTC()
		m := Marker{
			ID:            primitive.NewObjectID(),
			Title:         title,
			Category:      *in.Category,
			Location:      Location{Lat: *in.Location.Lat, Lng: *in.Location.Lng},
			Tags:          cleanTags(in.Tags),
			CreatorID:     strings.TrimSpace(in.CreatorID),
			Confirmations: []string{},
			Status:        "active",
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if in.Description != nil {
			m.Description = strings.TrimSpace(*in.Description)
		}
		if in.Location.Address != nil {
			m.Location.Address = strings.TrimSpace(*in.Location.Address)
		}
		if in.Contact != nil {
			m.Contact = strings.TrimSpace(*in.Contact)
		}
		if _, err := p.col(ctx).InsertOne(dbctx, m); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 201, m)
	})

	mux.HandleFunc("PUT "+prefix+"/api/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
		if err != nil {
			plugin.WriteErr(w, 404, "Marqueur introuvable")
			return
		}
		var in markerInput
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		var m Marker
		if err := p.col(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&m); err != nil {
			plugin.WriteErr(w, 404, "Marqueur introuvable")
			return
		}
		if m.CreatorID != in.CreatorID {
			plugin.WriteErr(w, 403, "Seul le créateur peut modifier ce marqueur")
			return
		}
		set := bson.M{"updatedAt": time.Now().UTC()}
		if in.Title != nil {
			t := strings.TrimSpace(*in.Title)
			if t == "" {
				plugin.WriteErr(w, 400, "Le titre ne peut pas être vide")
				return
			}
			if len([]rune(t)) > 100 {
				plugin.WriteErr(w, 400, "Le titre doit faire 100 caractères ou moins")
				return
			}
			set["title"] = t
		}
		if in.Description != nil {
			if len([]rune(*in.Description)) > 500 {
				plugin.WriteErr(w, 400, "La description doit faire 500 caractères ou moins")
				return
			}
			set["description"] = strings.TrimSpace(*in.Description)
		}
		if in.Category != nil {
			if !slices.Contains(validCategories, *in.Category) {
				plugin.WriteErr(w, 400, "Catégorie invalide")
				return
			}
			set["category"] = *in.Category
		}
		if in.Location != nil && in.Location.Lat != nil && in.Location.Lng != nil {
			set["location.lat"] = *in.Location.Lat
			set["location.lng"] = *in.Location.Lng
			if in.Location.Address != nil {
				set["location.address"] = strings.TrimSpace(*in.Location.Address)
			}
		}
		if in.Contact != nil {
			set["contact"] = strings.TrimSpace(*in.Contact)
		}
		if in.Tags != nil {
			set["tags"] = cleanTags(in.Tags)
		}
		var after Marker
		if err := p.col(ctx).FindOneAndUpdate(dbctx, bson.M{"_id": id}, bson.M{"$set": set},
			options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&after); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, after)
	})

	mux.HandleFunc("DELETE "+prefix+"/api/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
		if err != nil {
			plugin.WriteErr(w, 404, "Marqueur introuvable")
			return
		}
		var in markerInput
		_ = plugin.DecodeBody(r, &in)
		var m Marker
		if err := p.col(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&m); err != nil {
			plugin.WriteErr(w, 404, "Marqueur introuvable")
			return
		}
		if m.CreatorID != in.CreatorID {
			plugin.WriteErr(w, 403, "Seul le créateur peut supprimer ce marqueur")
			return
		}
		_, _ = p.col(ctx).DeleteOne(dbctx, bson.M{"_id": id})
		plugin.WriteJSON(w, 200, map[string]any{"success": true})
	})

	mux.HandleFunc("PATCH "+prefix+"/api/{id}/reposition", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
		if err != nil {
			plugin.WriteErr(w, 404, "Marqueur introuvable")
			return
		}
		var in markerInput
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		if in.Lat == nil || in.Lng == nil {
			plugin.WriteErr(w, 400, "Coordonnées invalides (lat, lng requises)")
			return
		}
		if *in.Lat < -90 || *in.Lat > 90 || *in.Lng < -180 || *in.Lng > 180 {
			plugin.WriteErr(w, 400, "Coordonnées hors limites")
			return
		}
		if in.VoterID == "" {
			plugin.WriteErr(w, 400, "Identifiant votant requis")
			return
		}
		var m Marker
		if err := p.col(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&m); err != nil {
			plugin.WriteErr(w, 404, "Marqueur introuvable")
			return
		}
		if haversineKm(m.Location.Lat, m.Location.Lng, *in.Lat, *in.Lng) > 5 {
			plugin.WriteErr(w, 400, "Déplacement trop important (max 5 km). Contactez un administrateur.")
			return
		}
		if _, err := p.col(ctx).UpdateOne(dbctx, bson.M{"_id": id},
			bson.M{"$set": bson.M{"location.lat": *in.Lat, "location.lng": *in.Lng, "updatedAt": time.Now().UTC()}}); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		ctx.Log.Printf("marker %s repositioned by %s: (%g,%g) → (%g,%g)",
			id.Hex(), in.VoterID, m.Location.Lat, m.Location.Lng, *in.Lat, *in.Lng)
		plugin.WriteJSON(w, 200, map[string]any{
			"success":  true,
			"location": map[string]any{"lat": *in.Lat, "lng": *in.Lng, "address": m.Location.Address},
		})
	})

	mux.HandleFunc("POST "+prefix+"/api/{id}/confirm", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
		if err != nil {
			plugin.WriteErr(w, 404, "Marqueur introuvable")
			return
		}
		var in markerInput
		_ = plugin.DecodeBody(r, &in)
		if in.VoterID == "" {
			plugin.WriteErr(w, 400, "Identifiant votant requis")
			return
		}
		var m Marker
		if err := p.col(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&m); err != nil {
			plugin.WriteErr(w, 404, "Marqueur introuvable")
			return
		}
		already := slices.Contains(m.Confirmations, in.VoterID)
		var update bson.M
		if already {
			update = bson.M{"$pull": bson.M{"confirmations": in.VoterID}}
		} else {
			update = bson.M{"$addToSet": bson.M{"confirmations": in.VoterID}}
		}
		var after Marker
		if err := p.col(ctx).FindOneAndUpdate(dbctx, bson.M{"_id": id}, update,
			options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&after); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{
			"confirmed":     !already,
			"confirmations": len(after.Confirmations),
		})
	})
	return nil
}
