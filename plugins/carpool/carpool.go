// Package carpool is the Go port of the Node carpool mini-app: daily
// carpool offers and requests, on the same "carpoolentries" collection.
package carpool

import (
	"embed"
	"io/fs"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/javimosch/enbauges-go/plugin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//go:embed web
var webFS embed.FS

const prefix = "/covoiturage"

type Entry struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Title       string             `bson:"title" json:"title"`
	Type        string             `bson:"type" json:"type"`
	Origin      string             `bson:"origin" json:"origin"`
	Destination string             `bson:"destination" json:"destination"`
	Date        time.Time          `bson:"date" json:"date"`
	Time        string             `bson:"time,omitempty" json:"time,omitempty"`
	Frequency   string             `bson:"frequency" json:"frequency"`
	Weekdays    []int              `bson:"weekdays,omitempty" json:"weekdays,omitempty"`
	Exceptions  []time.Time        `bson:"exceptions,omitempty" json:"exceptions,omitempty"`
	Contact     string             `bson:"contact" json:"contact"`
	Seats       *int               `bson:"seats,omitempty" json:"seats,omitempty"`
	Luggage     string             `bson:"luggage,omitempty" json:"luggage,omitempty"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type Carpool struct{}

func New() *Carpool { return &Carpool{} }

func (p *Carpool) Meta() plugin.Meta {
	return plugin.Meta{
		ID:          "carpool",
		Name:        "Covoiturage Quotidien",
		Version:     "1.0.0",
		Description: "Proposer ou rechercher des covoiturages quotidiens dans le territoire",
		RoutePrefix: prefix,
		Aliases:     []string{"/carpool", "/blablabauges"},
		Tags:        []string{"community", "transport", "mobility"},
	}
}

func (p *Carpool) WebFS() fs.FS {
	sub, _ := fs.Sub(webFS, "web")
	return sub
}

func (p *Carpool) Install(ctx *plugin.Context) error {
	return ctx.UpsertServiceCard(plugin.ServiceCard{
		Type:        "solution",
		Title:       "Covoiturage Quotidien",
		Description: "Proposez ou recherchez des covoiturages pour vos deplacements quotidiens",
		URL:         prefix,
		Tags:        []string{"service", "transport", "mobilite", "covoiturage"},
	})
}

func (p *Carpool) Bootstrap(ctx *plugin.Context) error { return nil }

func (p *Carpool) Mount(mux *http.ServeMux, ctx *plugin.Context) error {
	mux.HandleFunc("GET "+prefix, plugin.ServeWebFile(ctx, "carpool.html"))

	mux.HandleFunc("GET "+prefix+"/api", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		entries := []Entry{}
		cur, err := ctx.DB.Collection("carpoolentries").Find(dbctx, bson.M{},
			options.Find().SetSort(bson.D{{Key: "date", Value: 1}, {Key: "createdAt", Value: -1}}).SetLimit(100))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &entries); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"entries": entries})
	})

	mux.HandleFunc("POST "+prefix+"/api", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		var in struct {
			Title       string           `json:"title"`
			Type        string           `json:"type"`
			Origin      string           `json:"origin"`
			Destination string           `json:"destination"`
			Date        string           `json:"date"`
			Time        string           `json:"time"`
			Frequency   string           `json:"frequency"`
			Weekdays    []plugin.FlexInt `json:"weekdays"`
			Exceptions  []string         `json:"exceptions"`
			Contact     string           `json:"contact"`
			Seats       *plugin.FlexInt  `json:"seats"`
			Luggage     string           `json:"luggage"`
			Description string           `json:"description"`
		}
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		title := strings.TrimSpace(in.Title)
		switch {
		case title == "":
			plugin.WriteErr(w, 400, "Le titre est requis")
			return
		case in.Type != "offer" && in.Type != "request":
			plugin.WriteErr(w, 400, "Type invalide")
			return
		case strings.TrimSpace(in.Origin) == "":
			plugin.WriteErr(w, 400, "Le lieu de depart est requis")
			return
		case strings.TrimSpace(in.Destination) == "":
			plugin.WriteErr(w, 400, "La destination est requise")
			return
		case in.Date == "":
			plugin.WriteErr(w, 400, "La date est requise")
			return
		case strings.TrimSpace(in.Contact) == "":
			plugin.WriteErr(w, 400, "Le contact est requis")
			return
		case len([]rune(title)) > 200:
			plugin.WriteErr(w, 400, "Le titre doit faire moins de 200 caracteres")
			return
		}
		date, ok := plugin.ParseDate(in.Date)
		if !ok {
			plugin.WriteErr(w, 400, "La date est requise")
			return
		}
		freq := in.Frequency
		if freq != "weekdays" {
			freq = "one-time"
		}
		now := time.Now().UTC()
		e := Entry{
			ID:          primitive.NewObjectID(),
			Title:       title,
			Type:        in.Type,
			Origin:      strings.TrimSpace(in.Origin),
			Destination: strings.TrimSpace(in.Destination),
			Date:        date,
			Time:        strings.TrimSpace(in.Time),
			Frequency:   freq,
			Contact:     strings.TrimSpace(in.Contact),
			Description: strings.TrimSpace(in.Description),
			Luggage:     "none",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if freq == "weekdays" {
			e.Weekdays = plugin.FlexInts(in.Weekdays)
			for _, x := range in.Exceptions {
				if t, ok := plugin.ParseDate(x); ok {
					e.Exceptions = append(e.Exceptions, t)
				}
			}
		}
		if in.Type == "offer" && in.Seats != nil && int(*in.Seats) >= 1 && int(*in.Seats) <= 8 {
			n := int(*in.Seats)
			e.Seats = &n
		}
		if in.Type == "request" && slices.Contains([]string{"23kg", "10kg_or_less", "small_backpacks", "none"}, in.Luggage) {
			e.Luggage = in.Luggage
		}
		if _, err := ctx.DB.Collection("carpoolentries").InsertOne(dbctx, e); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 201, e)
	})

	mux.HandleFunc("DELETE "+prefix+"/api/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
		if err != nil {
			plugin.WriteErr(w, 404, "Annonce non trouvee")
			return
		}
		res, err := ctx.DB.Collection("carpoolentries").DeleteOne(dbctx, bson.M{"_id": id})
		if err != nil || res.DeletedCount == 0 {
			plugin.WriteErr(w, 404, "Annonce non trouvee")
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"success": true})
	})
	return nil
}
