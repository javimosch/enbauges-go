package panierlibre

import (
	"net/http"
	"strings"
	"time"

	"github.com/javimosch/enbauges-go/plugin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (p *PanierLibre) providers(ctx *plugin.Context) *mongo.Collection {
	return ctx.DB.Collection("plproviders")
}
func (p *PanierLibre) baskets(ctx *plugin.Context) *mongo.Collection {
	return ctx.DB.Collection("plbaskets")
}
func (p *PanierLibre) bookings(ctx *plugin.Context) *mongo.Collection {
	return ctx.DB.Collection("plbookings")
}

type reqInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	Address     *string `json:"address"`
	Location    *struct {
		Lat *float64 `json:"lat"`
		Lng *float64 `json:"lng"`
	} `json:"location"`
	AdminPassword string  `json:"adminPassword"`
	ProviderID    string  `json:"providerId"`
	Title         *string `json:"title"`
	StartDate     *string `json:"startDate"`
	EndDate       *string `json:"endDate"`
	Items         []struct {
		Key      string  `json:"key"`
		Quantity float64 `json:"quantity"`
	} `json:"items"`
	ExtraNotes *string `json:"extraNotes"`
	OwnerEmail string  `json:"ownerEmail"`
	OwnerPhone string  `json:"ownerPhone"`
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func oid(w http.ResponseWriter, r *http.Request, key, msg string) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(r.PathValue(key))
	if err != nil {
		plugin.WriteErr(w, 404, msg)
		return primitive.NilObjectID, false
	}
	return id, true
}

// refreshStatus persists a recomputed basket status if it changed, like the
// lazy refresh the Node routes do on every read.
func (p *PanierLibre) refreshStatus(r *http.Request, ctx *plugin.Context, b *Basket) {
	if ns := computeStatus(b.StartDate, b.EndDate); ns != b.Status {
		b.Status = ns
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		_, _ = p.baskets(ctx).UpdateOne(dbctx, bson.M{"_id": b.ID}, bson.M{"$set": bson.M{"status": ns}})
	}
}

func (p *PanierLibre) Mount(mux *http.ServeMux, ctx *plugin.Context) error {
	mux.HandleFunc("GET "+prefix, plugin.ServeWebFile(ctx, "panier-libre.html"))

	// ── providers ──────────────────────────────────────────────────────
	mux.HandleFunc("GET "+prefix+"/api/providers", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		list := []Provider{}
		cur, err := p.providers(ctx).Find(dbctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &list); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"providers": list})
	})

	mux.HandleFunc("GET "+prefix+"/api/providers/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := oid(w, r, "id", "Fournisseur introuvable")
		if !ok {
			return
		}
		var pr Provider
		if err := p.providers(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&pr); err != nil {
			plugin.WriteErr(w, 404, "Fournisseur introuvable")
			return
		}
		plugin.WriteJSON(w, 200, pr)
	})

	mux.HandleFunc("POST "+prefix+"/api/providers", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		var in reqInput
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		if str(in.Name) == "" {
			plugin.WriteErr(w, 400, "Le nom est requis")
			return
		}
		if len(in.AdminPassword) < 4 {
			plugin.WriteErr(w, 400, "Mot de passe admin requis (4 caractères min)")
			return
		}
		now := time.Now().UTC()
		pr := Provider{
			ID:            primitive.NewObjectID(),
			Name:          trunc(str(in.Name), 80),
			Description:   trunc(str(in.Description), 500),
			Email:         trunc(str(in.Email), 120),
			Phone:         trunc(str(in.Phone), 30),
			Address:       trunc(str(in.Address), 300),
			AdminPassword: in.AdminPassword,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if in.Location != nil && in.Location.Lat != nil && in.Location.Lng != nil {
			pr.Location = &GeoPoint{Lat: in.Location.Lat, Lng: in.Location.Lng}
		}
		if _, err := p.providers(ctx).InsertOne(dbctx, pr); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 201, pr)
	})

	mux.HandleFunc("PUT "+prefix+"/api/providers/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := oid(w, r, "id", "Fournisseur introuvable")
		if !ok {
			return
		}
		var in reqInput
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		var pr Provider
		if err := p.providers(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&pr); err != nil {
			plugin.WriteErr(w, 404, "Fournisseur introuvable")
			return
		}
		if in.AdminPassword == "" || in.AdminPassword != pr.AdminPassword {
			plugin.WriteErr(w, 403, "Mot de passe admin incorrect")
			return
		}
		set := bson.M{"updatedAt": time.Now().UTC()}
		unset := bson.M{}
		if in.Name != nil {
			set["name"] = trunc(str(in.Name), 80)
		}
		if in.Description != nil {
			set["description"] = trunc(str(in.Description), 500)
		}
		for field, v := range map[string]*string{"email": in.Email, "phone": in.Phone, "address": in.Address} {
			if v != nil {
				limits := map[string]int{"email": 120, "phone": 30, "address": 300}
				if s := trunc(strings.TrimSpace(*v), limits[field]); s != "" {
					set[field] = s
				} else {
					unset[field] = ""
				}
			}
		}
		if in.Location != nil {
			if in.Location.Lat != nil && in.Location.Lng != nil {
				set["location"] = GeoPoint{Lat: in.Location.Lat, Lng: in.Location.Lng}
			} else {
				unset["location"] = ""
			}
		}
		update := bson.M{"$set": set}
		if len(unset) > 0 {
			update["$unset"] = unset
		}
		var after Provider
		if err := p.providers(ctx).FindOneAndUpdate(dbctx, bson.M{"_id": id}, update,
			options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&after); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, after)
	})

	mux.HandleFunc("DELETE "+prefix+"/api/providers/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := oid(w, r, "id", "Fournisseur introuvable")
		if !ok {
			return
		}
		var in reqInput
		_ = plugin.DecodeBody(r, &in)
		var pr Provider
		if err := p.providers(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&pr); err != nil {
			plugin.WriteErr(w, 404, "Fournisseur introuvable")
			return
		}
		if in.AdminPassword == "" || in.AdminPassword != pr.AdminPassword {
			plugin.WriteErr(w, 403, "Mot de passe admin incorrect")
			return
		}
		// Cascade: bookings of the provider's baskets, then baskets, then provider.
		basketIDs, err := p.baskets(ctx).Distinct(dbctx, "_id", bson.M{"providerId": id})
		if err == nil && len(basketIDs) > 0 {
			_, _ = p.bookings(ctx).DeleteMany(dbctx, bson.M{"basketId": bson.M{"$in": basketIDs}})
		}
		_, _ = p.baskets(ctx).DeleteMany(dbctx, bson.M{"providerId": id})
		_, _ = p.providers(ctx).DeleteOne(dbctx, bson.M{"_id": id})
		plugin.WriteJSON(w, 200, map[string]any{"success": true})
	})

	mux.HandleFunc("POST "+prefix+"/api/providers/{id}/verify", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := oid(w, r, "id", "Fournisseur introuvable")
		if !ok {
			return
		}
		var in reqInput
		_ = plugin.DecodeBody(r, &in)
		var pr Provider
		if err := p.providers(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&pr); err != nil {
			plugin.WriteErr(w, 404, "Fournisseur introuvable")
			return
		}
		if in.AdminPassword == "" || in.AdminPassword != pr.AdminPassword {
			plugin.WriteErr(w, 403, "Mot de passe incorrect")
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"verified": true})
	})

	mux.HandleFunc("POST "+prefix+"/api/providers/{id}/map-marker", func(w http.ResponseWriter, r *http.Request) {
		p.upsertMapMarker(w, r, ctx)
	})

	// ── baskets ────────────────────────────────────────────────────────
	mux.HandleFunc("GET "+prefix+"/api/baskets", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		filter := bson.M{}
		if pid := r.URL.Query().Get("providerId"); pid != "" {
			if id, err := primitive.ObjectIDFromHex(pid); err == nil {
				filter["providerId"] = id
			}
		}
		if s := r.URL.Query().Get("status"); s != "" {
			filter["status"] = s
		}
		list := []Basket{}
		cur, err := p.baskets(ctx).Find(dbctx, filter,
			options.Find().SetSort(bson.D{{Key: "startDate", Value: -1}}).SetLimit(200))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &list); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		out := make([]map[string]any, 0, len(list))
		for i := range list {
			p.refreshStatus(r, ctx, &list[i])
			out = append(out, list[i].View())
		}
		plugin.WriteJSON(w, 200, map[string]any{"baskets": out})
	})

	mux.HandleFunc("GET "+prefix+"/api/baskets/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := oid(w, r, "id", "Panier introuvable")
		if !ok {
			return
		}
		var b Basket
		if err := p.baskets(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&b); err != nil {
			plugin.WriteErr(w, 404, "Panier introuvable")
			return
		}
		p.refreshStatus(r, ctx, &b)
		plugin.WriteJSON(w, 200, b.View())
	})

	mux.HandleFunc("POST "+prefix+"/api/baskets", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		var in reqInput
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		if in.ProviderID == "" {
			plugin.WriteErr(w, 400, "Fournisseur requis")
			return
		}
		if str(in.Title) == "" {
			plugin.WriteErr(w, 400, "Le titre est requis")
			return
		}
		if len(in.Items) == 0 {
			plugin.WriteErr(w, 400, "Au moins un article est requis")
			return
		}
		if in.StartDate == nil || *in.StartDate == "" {
			plugin.WriteErr(w, 400, "Date de début requise")
			return
		}
		if len(in.AdminPassword) < 4 {
			plugin.WriteErr(w, 400, "Mot de passe admin requis")
			return
		}
		pid, err := primitive.ObjectIDFromHex(in.ProviderID)
		if err != nil {
			plugin.WriteErr(w, 404, "Fournisseur introuvable")
			return
		}
		var pr Provider
		if err := p.providers(ctx).FindOne(dbctx, bson.M{"_id": pid}).Decode(&pr); err != nil {
			plugin.WriteErr(w, 404, "Fournisseur introuvable")
			return
		}
		if in.AdminPassword != pr.AdminPassword {
			plugin.WriteErr(w, 403, "Mot de passe admin incorrect")
			return
		}
		items := []BasketItem{}
		for _, it := range in.Items {
			key := trunc(strings.TrimSpace(it.Key), 60)
			qty := max(0, int(it.Quantity))
			if key != "" {
				items = append(items, BasketItem{Key: key, Quantity: qty})
			}
		}
		if len(items) == 0 {
			plugin.WriteErr(w, 400, "Articles invalides")
			return
		}
		startDate, ok := plugin.ParseDate(*in.StartDate)
		if !ok {
			plugin.WriteErr(w, 400, "Date de début requise")
			return
		}
		var endDate *time.Time
		if in.EndDate != nil && *in.EndDate != "" {
			if t, ok := plugin.ParseDate(*in.EndDate); ok {
				endDate = &t
			}
		}
		now := time.Now().UTC()
		b := Basket{
			ID:            primitive.NewObjectID(),
			ProviderID:    pid,
			Title:         trunc(str(in.Title), 100),
			Description:   trunc(str(in.Description), 1000),
			Items:         items,
			StartDate:     startDate,
			EndDate:       endDate,
			Status:        computeStatus(startDate, endDate),
			AdminPassword: in.AdminPassword,
			Version:       0,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if _, err := p.baskets(ctx).InsertOne(dbctx, b); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 201, b.View())
	})

	mux.HandleFunc("PUT "+prefix+"/api/baskets/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := oid(w, r, "id", "Panier introuvable")
		if !ok {
			return
		}
		var in reqInput
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		var b Basket
		if err := p.baskets(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&b); err != nil {
			plugin.WriteErr(w, 404, "Panier introuvable")
			return
		}
		if in.AdminPassword == "" || in.AdminPassword != b.AdminPassword {
			plugin.WriteErr(w, 403, "Mot de passe admin incorrect")
			return
		}
		if in.Title != nil {
			b.Title = trunc(str(in.Title), 100)
		}
		if in.Description != nil {
			b.Description = trunc(str(in.Description), 1000)
		}
		if in.StartDate != nil {
			if t, ok := plugin.ParseDate(*in.StartDate); ok {
				b.StartDate = t
			}
		}
		if in.EndDate != nil {
			if *in.EndDate == "" {
				b.EndDate = nil
			} else if t, ok := plugin.ParseDate(*in.EndDate); ok {
				b.EndDate = &t
			}
		}
		if in.Items != nil {
			booked := map[string]int{}
			for _, it := range b.Items {
				booked[it.Key] = it.Booked
			}
			items := []BasketItem{}
			for _, it := range in.Items {
				key := trunc(strings.TrimSpace(it.Key), 60)
				if key == "" {
					continue
				}
				qty := max(0, int(it.Quantity))
				bk := min(booked[key], qty) // clamp booked <= quantity
				items = append(items, BasketItem{Key: key, Quantity: qty, Booked: bk})
			}
			b.Items = items
		}
		b.Status = computeStatus(b.StartDate, b.EndDate)
		b.UpdatedAt = time.Now().UTC()
		if _, err := p.baskets(ctx).UpdateOne(dbctx, bson.M{"_id": id}, bson.M{"$set": bson.M{
			"title": b.Title, "description": b.Description, "items": b.Items,
			"startDate": b.StartDate, "endDate": b.EndDate, "status": b.Status,
			"updatedAt": b.UpdatedAt,
		}}); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, b.View())
	})

	mux.HandleFunc("DELETE "+prefix+"/api/baskets/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := oid(w, r, "id", "Panier introuvable")
		if !ok {
			return
		}
		var in reqInput
		_ = plugin.DecodeBody(r, &in)
		var b Basket
		if err := p.baskets(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&b); err != nil {
			plugin.WriteErr(w, 404, "Panier introuvable")
			return
		}
		if in.AdminPassword == "" || in.AdminPassword != b.AdminPassword {
			plugin.WriteErr(w, 403, "Mot de passe admin incorrect")
			return
		}
		_, _ = p.bookings(ctx).DeleteMany(dbctx, bson.M{"basketId": id})
		_, _ = p.baskets(ctx).DeleteOne(dbctx, bson.M{"_id": id})
		plugin.WriteJSON(w, 200, map[string]any{"success": true})
	})

	p.mountBookings(mux, ctx)
	return nil
}

// upsertMapMarker mirrors the Node cross-plugin write into the
// interactive-map collection.
func (p *PanierLibre) upsertMapMarker(w http.ResponseWriter, r *http.Request, ctx *plugin.Context) {
	dbctx, cancel := plugin.DBCtx(r)
	defer cancel()
	id, ok := oid(w, r, "id", "Fournisseur introuvable")
	if !ok {
		return
	}
	var in reqInput
	_ = plugin.DecodeBody(r, &in)
	var pr Provider
	if err := p.providers(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&pr); err != nil {
		plugin.WriteErr(w, 404, "Fournisseur introuvable")
		return
	}
	if in.AdminPassword == "" || in.AdminPassword != pr.AdminPassword {
		plugin.WriteErr(w, 403, "Mot de passe admin incorrect")
		return
	}
	hasCoords := pr.Location != nil && pr.Location.Lat != nil && pr.Location.Lng != nil
	if pr.Address == "" && !hasCoords {
		plugin.WriteErr(w, 400, "Adresse ou coordonnées requises pour créer un marqueur")
		return
	}
	markers := ctx.DB.Collection("mapmarkers")
	contact := strings.Join(nonEmpty(pr.Email, pr.Phone), " — ")
	now := time.Now().UTC()

	var existing bson.M
	err := markers.FindOne(dbctx, bson.M{"title": pr.Name, "category": "service"}).Decode(&existing)
	if err == nil {
		set := bson.M{"contact": contact, "updatedAt": now}
		if hasCoords {
			set["location.lat"] = *pr.Location.Lat
			set["location.lng"] = *pr.Location.Lng
		}
		if pr.Address != "" {
			set["location.address"] = pr.Address
		}
		if pr.Description != "" {
			set["description"] = pr.Description
		}
		_, _ = markers.UpdateOne(dbctx, bson.M{"_id": existing["_id"]}, bson.M{"$set": set})
	} else {
		lat, lng := 45.57, 6.12
		if hasCoords {
			lat, lng = *pr.Location.Lat, *pr.Location.Lng
		}
		_, _ = markers.InsertOne(dbctx, bson.M{
			"title": pr.Name, "description": pr.Description, "category": "service",
			"location": bson.M{"lat": lat, "lng": lng, "address": pr.Address},
			"contact":  contact, "tags": []string{"panier-libre", "producteur"},
			"creatorId": "panier-libre:" + pr.ID.Hex(), "confirmations": []string{},
			"status": "active", "createdAt": now, "updatedAt": now,
		})
	}
	plugin.WriteJSON(w, 200, map[string]any{"success": true, "markerUrl": "/carte-interactive"})
}

func nonEmpty(vals ...string) []string {
	out := []string{}
	for _, v := range vals {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
