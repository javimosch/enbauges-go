package panierlibre

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/javimosch/enbauges-go/plugin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// saveItemsOptimistic writes the basket's items back with the same
// __v optimistic-locking scheme mongoose uses, so concurrent bookings
// from either app conflict cleanly instead of double-booking.
func (p *PanierLibre) saveItemsOptimistic(r *http.Request, ctx *plugin.Context, b *Basket) bool {
	dbctx, cancel := plugin.DBCtx(r)
	defer cancel()
	res, err := p.baskets(ctx).UpdateOne(dbctx,
		bson.M{"_id": b.ID, "__v": b.Version},
		bson.M{"$inc": bson.M{"__v": 1}, "$set": bson.M{"items": b.Items}})
	return err == nil && res.ModifiedCount == 1
}

func (p *PanierLibre) mountBookings(mux *http.ServeMux, ctx *plugin.Context) {
	mux.HandleFunc("POST "+prefix+"/api/baskets/{basketId}/bookings/list", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := oid(w, r, "basketId", "Panier introuvable")
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
			plugin.WriteErr(w, 403, "Mot de passe admin requis")
			return
		}
		list := []Booking{}
		cur, err := p.bookings(ctx).Find(dbctx, bson.M{"basketId": id},
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &list); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, map[string]any{"bookings": list})
	})

	mux.HandleFunc("POST "+prefix+"/api/baskets/{basketId}/bookings", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := oid(w, r, "basketId", "Panier introuvable")
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
		if str(in.Email) == "" && str(in.Phone) == "" {
			plugin.WriteErr(w, 400, "Email ou téléphone requis")
			return
		}
		if str(in.Name) == "" {
			plugin.WriteErr(w, 400, "Le nom est requis")
			return
		}
		if len(in.Items) == 0 {
			plugin.WriteErr(w, 400, "Sélectionnez au moins un article")
			return
		}
		if computeStatus(b.StartDate, b.EndDate) == "finished" {
			plugin.WriteErr(w, 400, "Ce panier est terminé")
			return
		}

		bookedItems := []BookedItem{}
		for _, ri := range in.Items {
			key := strings.TrimSpace(ri.Key)
			qty := int(ri.Quantity)
			if key == "" || qty < 1 {
				continue
			}
			var bi *BasketItem
			for i := range b.Items {
				if b.Items[i].Key == key {
					bi = &b.Items[i]
					break
				}
			}
			if bi == nil {
				plugin.WriteErr(w, 400, "Article inconnu : "+key)
				return
			}
			available := bi.Quantity - bi.Booked
			if qty > available {
				plugin.WriteErr(w, 400, fmt.Sprintf("Quantité indisponible pour \"%s\" (disponible : %d)", key, available))
				return
			}
			bookedItems = append(bookedItems, BookedItem{Key: key, Quantity: qty})
		}
		if len(bookedItems) == 0 {
			plugin.WriteErr(w, 400, "Sélectionnez au moins un article")
			return
		}
		for _, bk := range bookedItems {
			for i := range b.Items {
				if b.Items[i].Key == bk.Key {
					b.Items[i].Booked += bk.Quantity
				}
			}
		}
		if !p.saveItemsOptimistic(r, ctx, &b) {
			plugin.WriteErr(w, 409, "Conflit de réservation — réessayez")
			return
		}
		now := time.Now().UTC()
		booking := Booking{
			ID:         primitive.NewObjectID(),
			BasketID:   b.ID,
			Name:       trunc(str(in.Name), 80),
			Email:      trunc(str(in.Email), 120),
			Phone:      trunc(str(in.Phone), 30),
			ExtraNotes: trunc(str(in.ExtraNotes), 500),
			Items:      bookedItems,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if _, err := p.bookings(ctx).InsertOne(dbctx, booking); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 201, booking)
	})

	mux.HandleFunc("PUT "+prefix+"/api/bookings/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := oid(w, r, "id", "Réservation introuvable")
		if !ok {
			return
		}
		var in reqInput
		if err := plugin.DecodeBody(r, &in); err != nil {
			plugin.WriteErr(w, 400, "invalid JSON body")
			return
		}
		var bk Booking
		if err := p.bookings(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&bk); err != nil {
			plugin.WriteErr(w, 404, "Réservation introuvable")
			return
		}
		var b Basket
		if err := p.baskets(ctx).FindOne(dbctx, bson.M{"_id": bk.BasketID}).Decode(&b); err != nil {
			plugin.WriteErr(w, 404, "Panier introuvable")
			return
		}
		isAdmin := in.AdminPassword != "" && in.AdminPassword == b.AdminPassword
		if !isAdmin {
			authEmail := strings.ToLower(strings.TrimSpace(firstNonEmpty(in.OwnerEmail, str(in.Email))))
			authPhone := strings.TrimSpace(firstNonEmpty(in.OwnerPhone, str(in.Phone)))
			isOwner := (bk.Email != "" && authEmail != "" && authEmail == strings.ToLower(bk.Email)) ||
				(bk.Phone != "" && authPhone != "" && authPhone == strings.TrimSpace(bk.Phone))
			if !isOwner {
				plugin.WriteErr(w, 403, "Email ou téléphone requis pour modifier cette réservation")
				return
			}
		}
		if in.Name != nil {
			bk.Name = trunc(str(in.Name), 80)
		}
		if in.Email != nil {
			bk.Email = trunc(str(in.Email), 120)
		}
		if in.Phone != nil {
			bk.Phone = trunc(str(in.Phone), 30)
		}
		if in.ExtraNotes != nil {
			bk.ExtraNotes = trunc(str(in.ExtraNotes), 500)
		}
		if in.Items != nil {
			// Release old quantities…
			for _, old := range bk.Items {
				for i := range b.Items {
					if b.Items[i].Key == old.Key {
						b.Items[i].Booked = max(0, b.Items[i].Booked-old.Quantity)
					}
				}
			}
			// …validate and reserve new ones.
			newItems := []BookedItem{}
			for _, ri := range in.Items {
				key := strings.TrimSpace(ri.Key)
				qty := int(ri.Quantity)
				if key == "" || qty < 1 {
					continue
				}
				var bi *BasketItem
				for i := range b.Items {
					if b.Items[i].Key == key {
						bi = &b.Items[i]
						break
					}
				}
				if bi == nil {
					plugin.WriteErr(w, 400, "Article inconnu : "+key)
					return
				}
				available := bi.Quantity - bi.Booked
				if qty > available {
					plugin.WriteErr(w, 400, fmt.Sprintf("Quantité indisponible pour \"%s\" (disponible : %d)", key, available))
					return
				}
				newItems = append(newItems, BookedItem{Key: key, Quantity: qty})
				bi.Booked += qty
			}
			if len(newItems) == 0 {
				plugin.WriteErr(w, 400, "Sélectionnez au moins un article")
				return
			}
			bk.Items = newItems
			if !p.saveItemsOptimistic(r, ctx, &b) {
				plugin.WriteErr(w, 409, "Conflit de réservation — réessayez")
				return
			}
		}
		bk.UpdatedAt = time.Now().UTC()
		if _, err := p.bookings(ctx).UpdateOne(dbctx, bson.M{"_id": id}, bson.M{"$set": bson.M{
			"name": bk.Name, "email": bk.Email, "phone": bk.Phone,
			"extraNotes": bk.ExtraNotes, "items": bk.Items, "updatedAt": bk.UpdatedAt,
		}}); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		plugin.WriteJSON(w, 200, bk)
	})

	mux.HandleFunc("DELETE "+prefix+"/api/bookings/{id}", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		id, ok := oid(w, r, "id", "Réservation introuvable")
		if !ok {
			return
		}
		var in reqInput
		_ = plugin.DecodeBody(r, &in)
		var bk Booking
		if err := p.bookings(ctx).FindOne(dbctx, bson.M{"_id": id}).Decode(&bk); err != nil {
			plugin.WriteErr(w, 404, "Réservation introuvable")
			return
		}
		var b Basket
		if err := p.baskets(ctx).FindOne(dbctx, bson.M{"_id": bk.BasketID}).Decode(&b); err != nil {
			plugin.WriteErr(w, 404, "Panier introuvable")
			return
		}
		isAdmin := in.AdminPassword != "" && in.AdminPassword == b.AdminPassword
		if !isAdmin {
			email := strings.ToLower(str(in.Email))
			phone := str(in.Phone)
			isOwner := (bk.Email != "" && email != "" && email == strings.ToLower(bk.Email)) ||
				(bk.Phone != "" && phone != "" && phone == strings.TrimSpace(bk.Phone))
			if !isOwner {
				plugin.WriteErr(w, 403, "Email ou téléphone requis pour annuler cette réservation")
				return
			}
		}
		for _, old := range bk.Items {
			for i := range b.Items {
				if b.Items[i].Key == old.Key {
					b.Items[i].Booked = max(0, b.Items[i].Booked-old.Quantity)
				}
			}
		}
		_, _ = p.baskets(ctx).UpdateOne(dbctx, bson.M{"_id": b.ID},
			bson.M{"$set": bson.M{"items": b.Items}, "$inc": bson.M{"__v": 1}})
		_, _ = p.bookings(ctx).DeleteOne(dbctx, bson.M{"_id": id})
		plugin.WriteJSON(w, 200, map[string]any{"success": true})
	})

	mux.HandleFunc("POST "+prefix+"/api/bookings/lookup", func(w http.ResponseWriter, r *http.Request) {
		dbctx, cancel := plugin.DBCtx(r)
		defer cancel()
		var in reqInput
		_ = plugin.DecodeBody(r, &in)
		email := strings.ToLower(str(in.Email))
		phone := str(in.Phone)
		if email == "" && phone == "" {
			plugin.WriteErr(w, 400, "Email ou téléphone requis")
			return
		}
		var filter bson.M
		switch {
		case email != "" && phone != "":
			filter = bson.M{"$or": bson.A{bson.M{"email": email}, bson.M{"phone": phone}}}
		case email != "":
			filter = bson.M{"email": email}
		default:
			filter = bson.M{"phone": phone}
		}
		list := []Booking{}
		cur, err := p.bookings(ctx).Find(dbctx, filter,
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(50))
		if err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		if err := cur.All(dbctx, &list); err != nil {
			plugin.WriteErr(w, 500, err.Error())
			return
		}
		enriched := make([]map[string]any, 0, len(list))
		for _, bk := range list {
			basketTitle, basketStatus, providerName := "Panier supprimé", "unknown", "Fournisseur inconnu"
			var b Basket
			if err := p.baskets(ctx).FindOne(dbctx, bson.M{"_id": bk.BasketID}).Decode(&b); err == nil {
				basketTitle, basketStatus = b.Title, b.Status
				var pr Provider
				if err := p.providers(ctx).FindOne(dbctx, bson.M{"_id": b.ProviderID}).Decode(&pr); err == nil {
					providerName = pr.Name
				}
			}
			enriched = append(enriched, map[string]any{
				"_id": bk.ID, "basketId": bk.BasketID, "name": bk.Name,
				"email": bk.Email, "phone": bk.Phone, "extraNotes": bk.ExtraNotes,
				"items": bk.Items, "createdAt": bk.CreatedAt, "updatedAt": bk.UpdatedAt,
				"basketTitle": basketTitle, "basketStatus": basketStatus, "providerName": providerName,
			})
		}
		plugin.WriteJSON(w, 200, map[string]any{"bookings": enriched})
	})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
