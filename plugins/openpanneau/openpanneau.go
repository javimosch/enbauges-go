// Package openpanneau is the Go port of the Node open-panneau mini-app:
// a free alternative to Panneau Pocket where municipalities publish
// announcements to residents, on the same "municipalities" and
// "announcements" collections.
//
// Auth model, preserved from Node:
//   - Municipality credentials: email + accessCode, stored as a sha256 hex
//     hash — existing accounts verify unchanged.
//   - Admin: HTTP Basic (ADMIN_USERNAME/ADMIN_PASSWORD env) is superadmin;
//     otherwise X-Admin-Email/X-Admin-Code headers of the FIRST active
//     municipality (by createdAt).
//   - The /auth endpoint also grants isAdmin to OPEN_PANNEAU_ADMIN_EMAIL.
package openpanneau

import (
	"embed"
	"io/fs"
	"net/http"
	"regexp"
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

const prefix = "/open-panneau"

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
var announcementTypes = []string{"info", "alert", "event", "warning"}

type Municipality struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Name           string             `bson:"name" json:"name"`
	InseeCode      string             `bson:"inseeCode,omitempty" json:"inseeCode,omitempty"`
	Email          string             `bson:"email" json:"email"`
	ContactName    string             `bson:"contactName,omitempty" json:"contactName,omitempty"`
	ContactPhone   string             `bson:"contactPhone,omitempty" json:"contactPhone,omitempty"`
	Status         string             `bson:"status" json:"status"`
	AccessCodeHash string             `bson:"accessCodeHash" json:"-"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type Announcement struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	MunicipalityID primitive.ObjectID `bson:"municipalityId" json:"-"`
	Title          string             `bson:"title" json:"title"`
	Content        string             `bson:"content" json:"content"`
	Type           string             `bson:"type" json:"type"`
	ExpiresAt      *time.Time         `bson:"expiresAt" json:"expiresAt"`
	IsActive       bool               `bson:"isActive" json:"isActive"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// view returns the announcement with municipalityId either as raw id or as
// the populated {_id, name[, email, status]} ref mongoose produces.
func (a *Announcement) view(ref any) map[string]any {
	if ref == nil {
		ref = a.MunicipalityID
	}
	return map[string]any{
		"_id": a.ID, "municipalityId": ref, "title": a.Title,
		"content": a.Content, "type": a.Type, "expiresAt": a.ExpiresAt,
		"isActive": a.IsActive, "createdAt": a.CreatedAt, "updatedAt": a.UpdatedAt,
	}
}

type OpenPanneau struct{}

func New() *OpenPanneau { return &OpenPanneau{} }

func (p *OpenPanneau) Meta() plugin.Meta {
	return plugin.Meta{
		ID:          "open-panneau",
		Name:        "Open Panneau",
		Version:     "1.0.0",
		Description: "Alternative libre à Panneau Pocket - Information municipale pour les habitants",
		RoutePrefix: prefix,
		Tags:        []string{"service", "mairie", "commune", "information", "alerts"},
	}
}

func (p *OpenPanneau) WebFS() fs.FS {
	sub, _ := fs.Sub(webFS, "web")
	return sub
}

func (p *OpenPanneau) Install(ctx *plugin.Context) error {
	return ctx.UpsertServiceCard(plugin.ServiceCard{
		Type:        "solution",
		Title:       "Open Panneau",
		Description: "Alternative libre et gratuite à Panneau Pocket - Informez les habitants de votre commune",
		URL:         prefix,
		Tags:        []string{"service", "mairie", "commune", "information", "alertes"},
	})
}

func (p *OpenPanneau) Bootstrap(ctx *plugin.Context) error { return nil }

func (p *OpenPanneau) municipalities(ctx *plugin.Context) *mongo.Collection {
	return ctx.DB.Collection("municipalities")
}
func (p *OpenPanneau) announcements(ctx *plugin.Context) *mongo.Collection {
	return ctx.DB.Collection("announcements")
}

type opInput struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	AccessCode   string `json:"accessCode"`
	ContactName  string `json:"contactName"`
	ContactPhone string `json:"contactPhone"`
	InseeCode    string `json:"inseeCode"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	Type         string `json:"type"`
	ExpiresAt    string `json:"expiresAt"`
	Status       string `json:"status"`
}

// authMunicipality validates body email+accessCode against an active
// municipality, like the Node middleware.
func (p *OpenPanneau) authMunicipality(w http.ResponseWriter, r *http.Request, ctx *plugin.Context, in *opInput) (*Municipality, bool) {
	if in.Email == "" || in.AccessCode == "" {
		plugin.WriteErr(w, 401, "Email et code d'accès requis")
		return nil, false
	}
	dbctx, cancel := plugin.DBCtx(r)
	defer cancel()
	var m Municipality
	err := p.municipalities(ctx).FindOne(dbctx, bson.M{"email": strings.ToLower(in.Email)}).Decode(&m)
	if err != nil || plugin.SHA256Hex(in.AccessCode) != m.AccessCodeHash {
		plugin.WriteErr(w, 401, "Identifiants invalides")
		return nil, false
	}
	if m.Status != "active" {
		plugin.WriteErr(w, 403, "Compte en attente d'approbation ou désactivé")
		return nil, false
	}
	return &m, true
}

// firstActive returns the oldest active municipality — the one Node treats
// as admin.
func (p *OpenPanneau) firstActive(r *http.Request, ctx *plugin.Context) *Municipality {
	dbctx, cancel := plugin.DBCtx(r)
	defer cancel()
	var m Municipality
	err := p.municipalities(ctx).FindOne(dbctx, bson.M{"status": "active"},
		options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: 1}})).Decode(&m)
	if err != nil {
		return nil
	}
	return &m
}

// authAdmin: Basic auth superadmin, or X-Admin-Email/Code of the first
// active municipality.
func (p *OpenPanneau) authAdmin(w http.ResponseWriter, r *http.Request, ctx *plugin.Context) bool {
	if strings.HasPrefix(r.Header.Get("Authorization"), "Basic ") {
		if plugin.CheckBasicAuth(r, ctx.Env("ADMIN_USERNAME", ""), ctx.Env("ADMIN_PASSWORD", "")) {
			return true
		}
		plugin.WriteErr(w, 401, "Identifiants superadmin invalides")
		return false
	}
	adminEmail := r.Header.Get("X-Admin-Email")
	adminCode := r.Header.Get("X-Admin-Code")
	if adminEmail == "" || adminCode == "" {
		plugin.WriteErr(w, 401, "Authentification requise (Basic auth ou X-Admin-Email/Code)")
		return false
	}
	dbctx, cancel := plugin.DBCtx(r)
	defer cancel()
	var m Municipality
	err := p.municipalities(ctx).FindOne(dbctx, bson.M{"email": strings.ToLower(adminEmail)}).Decode(&m)
	if err != nil || plugin.SHA256Hex(adminCode) != m.AccessCodeHash {
		plugin.WriteErr(w, 401, "Identifiants invalides")
		return false
	}
	if m.Status != "active" {
		plugin.WriteErr(w, 403, "Compte non actif")
		return false
	}
	first := p.firstActive(r, ctx)
	if first == nil || first.ID != m.ID {
		plugin.WriteErr(w, 403, "Accès admin refusé - seule la première commune active est admin")
		return false
	}
	return true
}

func pagination(r *http.Request) (int64, int64) {
	limit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// municipalityRefs loads {_id,...fields} refs for a set of announcements,
// reproducing mongoose populate.
func (p *OpenPanneau) municipalityRefs(r *http.Request, ctx *plugin.Context, list []Announcement, fields bson.M) map[primitive.ObjectID]bson.M {
	dbctx, cancel := plugin.DBCtx(r)
	defer cancel()
	idSet := map[primitive.ObjectID]bool{}
	for _, a := range list {
		idSet[a.MunicipalityID] = true
	}
	ids := make([]primitive.ObjectID, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	refs := map[primitive.ObjectID]bson.M{}
	cur, err := p.municipalities(ctx).Find(dbctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(fields))
	if err != nil {
		return refs
	}
	docs := []bson.M{}
	_ = cur.All(dbctx, &docs)
	for _, d := range docs {
		if id, ok := d["_id"].(primitive.ObjectID); ok {
			refs[id] = d
		}
	}
	return refs
}
