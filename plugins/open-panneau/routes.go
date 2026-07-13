package openpanneau

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/javimosch/enbauges-go/plugin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func (p *OpenPanneau) Mount(mux *http.ServeMux, ctx *plugin.Context) error {
	mux.HandleFunc("GET "+prefix, plugin.ServeWebFile(ctx, "open-panneau.html"))

	// ── public: municipalities ─────────────────────────────────────────
	mux.HandleFunc("GET "+prefix+"/api/municipalities", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		list := []Municipality{}
		cur, err := p.municipalities(ctx).Find(dbctx, bson.M{"status": "active"},
			options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &list); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"municipalities": list})
	})

	mux.HandleFunc("POST "+prefix+"/api/municipalities/register", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		var in opInput
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		name := strings.TrimSpace(in.Name)
		email := strings.ToLower(strings.TrimSpace(in.Email))
		switch {
		case name == "":
			plugin.WriteErr(w, 400, "Le nom de la commune est requis")
			return
		case email == "":
			plugin.WriteErr(w, 400, "L'email est requis")
			return
		case len(in.AccessCode) < 6:
			plugin.WriteErr(w, 400, "Le code d'accès doit faire au moins 6 caractères")
			return
		case !emailRe.MatchString(email):
			plugin.WriteErr(w, 400, "Email invalide")
			return
		}
		if err := p.municipalities(ctx).FindOne(dbctx, bson.M{"email": email}).Err(); err == nil {
			plugin.WriteErr(w, 409, "Un compte existe déjà avec cet email")
			return
		}
		now := time.Now().UTC()
		m := Municipality{
			ID:             primitive.NewObjectID(),
			Name:           trunc(name, 200),
			Email:          trunc(email, 200),
			AccessCodeHash: plugin.SHA256Hex(in.AccessCode),
			ContactName:    trunc(strings.TrimSpace(in.ContactName), 200),
			ContactPhone:   trunc(strings.TrimSpace(in.ContactPhone), 50),
			InseeCode:      trunc(strings.TrimSpace(in.InseeCode), 10),
			Status:         "pending",
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		// First municipality auto-activates and becomes admin.
		activeCount, _ := p.municipalities(ctx).CountDocuments(dbctx, bson.M{"status": "active"})
		message := "Inscription réussie. Votre compte est en attente d'approbation."
		if activeCount == 0 {
			m.Status = "active"
			message = "Inscription réussie ! Votre commune est active."
			ctx.Log.Println("first municipality auto-activated as admin:", m.Email)
		}
		if _, err := p.municipalities(ctx).InsertOne(dbctx, m); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 201, map[string]any{"message": message, "municipality": m})
	})

	mux.HandleFunc("POST "+prefix+"/api/municipalities/auth", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		var in opInput
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		if in.Email == "" || in.AccessCode == "" {
			plugin.WriteErr(w, 400, "Email et code d'accès requis")
			return
		}
		var m Municipality
		err := p.municipalities(ctx).FindOne(dbctx, bson.M{"email": strings.ToLower(in.Email)}).Decode(&m)
		if err != nil || plugin.SHA256Hex(in.AccessCode) != m.AccessCodeHash {
			plugin.WriteErr(w, 401, "Identifiants invalides")
			return
		}
		isAdmin := m.Email == ctx.Env("OPEN_PANNEAU_ADMIN_EMAIL", "") || m.Email == "admin@open-panneau.local"
		if !isAdmin && m.Status == "active" {
			if first := p.firstActive(r, ctx); first != nil {
				isAdmin = first.ID == m.ID
			}
		}
		plugin.WriteJSON(w, 200, map[string]any{"municipality": m, "isAdmin": isAdmin})
	})

	// ── public: announcements ──────────────────────────────────────────
	mux.HandleFunc("GET "+prefix+"/api/announcements", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		q := r.URL.Query()
		filter := bson.M{"isActive": true}
		if mid := q.Get("municipalityId"); mid != "" {
			id, err := primitive.ObjectIDFromHex(mid)
			if err != nil {
				plugin.WriteJSON(w, 200, map[string]any{"announcements": []any{}})
				return
			}
			filter["municipalityId"] = id
		} else {
			activeIDs, err := p.municipalities(ctx).Distinct(dbctx, "_id", bson.M{"status": "active"})
			if err != nil {
				plugin.WriteErr(w, 500, err.Error())
				return
			}
			filter["municipalityId"] = bson.M{"$in": activeIDs}
		}
		if t := q.Get("type"); t != "" {
			filter["type"] = t
		}
		if q.Get("includeExpired") == "" {
			filter["$or"] = bson.A{
				bson.M{"expiresAt": nil},
				bson.M{"expiresAt": bson.M{"$gt": time.Now()}},
			}
		}
		limit, offset := pagination(r)
		list := []Announcement{}
		cur, err := p.announcements(ctx).Find(dbctx, filter,
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetSkip(offset).SetLimit(limit))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &list); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		refs := p.municipalityRefs(r, ctx, list, bson.M{"name": 1})
		out := make([]map[string]any, 0, len(list))
		for i := range list {
			out = append(out, list[i].view(refs[list[i].MunicipalityID]))
		}
		plugin.WriteJSON(w, 200, map[string]any{"announcements": out})
	})

	mux.HandleFunc("POST "+prefix+"/api/announcements", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		var in opInput
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		m, ok := p.authMunicipality(w, r, ctx, &in)
		if !ok {
			return
		}
		if strings.TrimSpace(in.Title) == "" {
			plugin.WriteErr(w, 400, "Le titre est requis")
			return
		}
		if strings.TrimSpace(in.Content) == "" {
			plugin.WriteErr(w, 400, "Le contenu est requis")
			return
		}
		typ := in.Type
		if typ == "" {
			typ = "info"
		}
		if !slices.Contains(announcementTypes, typ) {
			plugin.WriteErr(w, 400, "Type invalide")
			return
		}
		now := time.Now().UTC()
		a := Announcement{
			ID:             primitive.NewObjectID(),
			MunicipalityID: m.ID,
			Title:          trunc(strings.TrimSpace(in.Title), 200),
			Content:        trunc(strings.TrimSpace(in.Content), 2000),
			Type:           typ,
			IsActive:       true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if in.ExpiresAt != "" {
			if t, ok := plugin.ParseDate(in.ExpiresAt); ok {
				a.ExpiresAt = &t
			}
		}
		if _, err := p.announcements(ctx).InsertOne(dbctx, a); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 201, a.view(nil))
	})

	mux.HandleFunc("DELETE "+prefix+"/api/announcements/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		var in opInput
		_ = plugin.DecodeBody(r, &in)
		m, ok := p.authMunicipality(w, r, ctx, &in)
		if !ok {
			return
		}
		id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
		if err != nil {
			plugin.WriteErr(w, 404, "Annonce introuvable")
			return
		}
		var a Announcement
		if err := p.announcements(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&a); err != nil {
			plugin.WriteErr(w, 404, "Annonce introuvable")
			return
		}
		if a.MunicipalityID != m.ID {
			plugin.WriteErr(w, 403, "Non autorisé")
			return
		}
		_, _ = p.announcements(ctx).UpdateOne(dbctx, bson.M{"_id": id},
			bson.M{"$set": bson.M{"isActive": false, "updatedAt": time.Now().UTC()}})
		plugin.WriteJSON(w, 200, map[string]any{"success": true})
	})

	// ── admin ──────────────────────────────────────────────────────────
	mux.HandleFunc("GET "+prefix+"/api/admin/municipalities", func(w http.ResponseWriter, r *http.Request) {
		if !p.authAdmin(w, r, ctx) {
			return
		}
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		list := []Municipality{}
		cur, err := p.municipalities(ctx).Find(dbctx, bson.M{},
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &list); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"municipalities": list})
	})

	setStatus := func(w http.ResponseWriter, r *http.Request, status string) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
		if err != nil {
			plugin.WriteErr(w, 404, "Commune introuvable")
			return
		}
		var m Municipality
		err = p.municipalities(ctx).FindOneAndUpdate(dbctx, bson.M{"_id": id},
			bson.M{"$set": bson.M{"status": status, "updatedAt": time.Now().UTC()}},
			options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&m)
		if err == mongo.ErrNoDocuments {
			plugin.WriteErr(w, 404, "Commune introuvable")
			return
		}
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"municipality": m})
	}

	mux.HandleFunc("PUT "+prefix+"/api/admin/municipalities/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		if !p.authAdmin(w, r, ctx) {
			return
		}
		var in opInput
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		if !slices.Contains([]string{"pending", "active", "disabled"}, in.Status) {
			plugin.WriteErr(w, 400, "Statut invalide")
			return
		}
		setStatus(w, r, in.Status)
	})

	mux.HandleFunc("PUT "+prefix+"/api/admin/municipalities/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
		if !p.authAdmin(w, r, ctx) {
			return
		}
		setStatus(w, r, "active")
	})

	mux.HandleFunc("GET "+prefix+"/api/admin/announcements", func(w http.ResponseWriter, r *http.Request) {
		if !p.authAdmin(w, r, ctx) {
			return
		}
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		q := r.URL.Query()
		filter := bson.M{}
		if mid := q.Get("municipalityId"); mid != "" {
			if id, err := primitive.ObjectIDFromHex(mid); err == nil {
				filter["municipalityId"] = id
			}
		}
		if t := q.Get("type"); t != "" {
			filter["type"] = t
		}
		limit, offset := pagination(r)
		list := []Announcement{}
		cur, err := p.announcements(ctx).Find(dbctx, filter,
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetSkip(offset).SetLimit(limit))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &list); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		total, _ := p.announcements(ctx).CountDocuments(dbctx, filter)
		refs := p.municipalityRefs(r, ctx, list, bson.M{"name": 1, "email": 1, "status": 1})
		out := make([]map[string]any, 0, len(list))
		for i := range list {
			out = append(out, list[i].view(refs[list[i].MunicipalityID]))
		}
		plugin.WriteJSON(w, 200, map[string]any{"announcements": out, "total": total})
	})

	mux.HandleFunc("DELETE "+prefix+"/api/admin/announcements/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !p.authAdmin(w, r, ctx) {
			return
		}
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
		if err != nil {
			plugin.WriteErr(w, 404, "Annonce introuvable")
			return
		}
		res, err := p.announcements(ctx).UpdateOne(dbctx, bson.M{"_id": id},
			bson.M{"$set": bson.M{"isActive": false, "updatedAt": time.Now().UTC()}})
		if err != nil || res.MatchedCount == 0 {
			plugin.WriteErr(w, 404, "Annonce introuvable")
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"success": true})
	})

	return nil
}
