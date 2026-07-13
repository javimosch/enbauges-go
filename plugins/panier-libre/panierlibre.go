// Package panierlibre is the Go port of the Node panier-libre mini-app:
// producers (providers) publish baskets of items that residents book, on
// the same "plproviders", "plbaskets", "plbookings" collections.
//
// Compat notes, deliberately preserved from the Node implementation:
//   - adminPassword is stored in plain text and compared with equality
//     (pre-existing accounts must keep working; hashing is a future,
//     coordinated migration).
//   - Booking mutations use optimistic locking on the mongoose __v version
//     field; Go initializes __v to 0 on insert so both apps interoperate.
package panierlibre

import (
	"embed"
	"io/fs"
	"time"

	"github.com/javimosch/enbauges-go/plugin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

//go:embed web
var webFS embed.FS

const prefix = "/panier-libre"

// ─── models ────────────────────────────────────────────────────────────

type GeoPoint struct {
	Lat *float64 `bson:"lat,omitempty" json:"lat,omitempty"`
	Lng *float64 `bson:"lng,omitempty" json:"lng,omitempty"`
}

type Provider struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Name          string             `bson:"name" json:"name"`
	Description   string             `bson:"description" json:"description"`
	Email         string             `bson:"email,omitempty" json:"email,omitempty"`
	Phone         string             `bson:"phone,omitempty" json:"phone,omitempty"`
	Address       string             `bson:"address,omitempty" json:"address,omitempty"`
	Location      *GeoPoint          `bson:"location,omitempty" json:"location,omitempty"`
	AdminPassword string             `bson:"adminPassword" json:"-"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type BasketItem struct {
	Key      string `bson:"key" json:"key"`
	Quantity int    `bson:"quantity" json:"quantity"`
	Booked   int    `bson:"booked" json:"booked"`
}

type Basket struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	ProviderID    primitive.ObjectID `bson:"providerId"`
	Title         string             `bson:"title"`
	Description   string             `bson:"description"`
	Items         []BasketItem       `bson:"items"`
	StartDate     time.Time          `bson:"startDate"`
	EndDate       *time.Time         `bson:"endDate"`
	Status        string             `bson:"status"`
	AdminPassword string             `bson:"adminPassword"`
	Version       int                `bson:"__v"`
	CreatedAt     time.Time          `bson:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt"`
}

// View reproduces the mongoose toJSON shape: adminPassword stripped and a
// computed "available" per item.
func (b *Basket) View() map[string]any {
	items := make([]map[string]any, 0, len(b.Items))
	for _, it := range b.Items {
		items = append(items, map[string]any{
			"key": it.Key, "quantity": it.Quantity, "booked": it.Booked,
			"available": max(0, it.Quantity-it.Booked),
		})
	}
	return map[string]any{
		"_id": b.ID, "providerId": b.ProviderID, "title": b.Title,
		"description": b.Description, "items": items,
		"startDate": b.StartDate, "endDate": b.EndDate, "status": b.Status,
		"createdAt": b.CreatedAt, "updatedAt": b.UpdatedAt,
	}
}

func computeStatus(start time.Time, end *time.Time) string {
	now := time.Now()
	if start.After(now) {
		return "incoming"
	}
	if end != nil && end.Before(now) {
		return "finished"
	}
	return "ongoing"
}

type BookedItem struct {
	Key      string `bson:"key" json:"key"`
	Quantity int    `bson:"quantity" json:"quantity"`
}

type Booking struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	BasketID   primitive.ObjectID `bson:"basketId" json:"basketId"`
	Name       string             `bson:"name" json:"name"`
	Email      string             `bson:"email,omitempty" json:"email,omitempty"`
	Phone      string             `bson:"phone,omitempty" json:"phone,omitempty"`
	ExtraNotes string             `bson:"extraNotes" json:"extraNotes"`
	Items      []BookedItem       `bson:"items" json:"items"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// ─── plugin ────────────────────────────────────────────────────────────

type PanierLibre struct{}

func New() *PanierLibre { return &PanierLibre{} }

func (p *PanierLibre) Meta() plugin.Meta {
	return plugin.Meta{
		ID:          "panier-libre",
		Name:        "Panier Libre",
		Version:     "1.0.0",
		Description: "Réservez des paniers et lots auprès des producteurs du Massif des Bauges",
		RoutePrefix: prefix,
		Aliases:     []string{"/paniers", "/baskets"},
		Tags:        []string{"community", "panier", "amap", "reservation", "producteur"},
	}
}

func (p *PanierLibre) WebFS() fs.FS {
	sub, _ := fs.Sub(webFS, "web")
	return sub
}

func (p *PanierLibre) Install(ctx *plugin.Context) error {
	return p.Bootstrap(ctx)
}

// Bootstrap upserts the service card on every start, matching the Node
// plugin (the only one that re-upserts at bootstrap).
func (p *PanierLibre) Bootstrap(ctx *plugin.Context) error {
	return ctx.UpsertServiceCard(plugin.ServiceCard{
		Type:        "solution",
		Title:       "Panier Libre",
		Description: "Réservez des paniers et des lots auprès des producteurs et acteurs du Massif des Bauges",
		URL:         prefix,
		Tags:        []string{"service", "panier", "amap", "reservation", "producteur"},
	})
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
