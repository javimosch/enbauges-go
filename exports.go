package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	exportSource  = "enbauges.fr"
	exportNotice  = "Données collectées depuis des sources publiques (sites de communes, Radio Alto). Accès contrôlé — la redistribution en masse est soumise à autorisation."
)

// requireAPIKey checks for a valid API key for bulk data exports.
// The ICS calendar feed remains public (it's a calendar subscription, not bulk data).
func requireAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			key = r.Header.Get("X-API-Key")
		}
		if key == "" || !validateAPIKey(r, key) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]any{
				"error":  "api_key_required",
				"detail": "L'accès aux exports en masse nécessite une clé API valide. Contactez contact@savoietech.fr pour en demander une.",
				"page":   baseURL(r) + "/acces-donnees",
			})
			return
		}
		next(w, r)
	}
}

func csvEscape(v string) string {
	if strings.ContainsAny(v, "\",\n\r;") {
		return "\"" + strings.ReplaceAll(v, "\"", "\"\"") + "\""
	}
	return v
}

func icsEscape(v string) string {
	r := strings.NewReplacer("\\", "\\\\", ";", "\\;", ",", "\\,", "\r\n", "\\n", "\n", "\\n")
	return r.Replace(v)
}

func icsDate(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func handleExportIndex(w http.ResponseWriter, r *http.Request) {
	base := baseURL(r) + "/api/export"
	writeJSON(w, 200, map[string]any{
		"source":      exportSource,
		"notice":      exportNotice,
		"description": "Accès aux données du territoire du Cœur des Bauges. Accès contrôlé — clé API requise pour les exports en masse (sauf agenda iCal).",
		"exports": []map[string]string{
			{"url": base + "/cards.json?key=YOUR_KEY", "format": "JSON", "content": "Cartes (acteurs, solutions, initiatives) et liens — clé requise"},
			{"url": base + "/cards.csv?key=YOUR_KEY", "format": "CSV", "content": "Cartes, à plat pour tableur — clé requise"},
			{"url": base + "/cards.geojson?key=YOUR_KEY", "format": "GeoJSON", "content": "Cartes géolocalisées — clé requise"},
			{"url": base + "/events.json?key=YOUR_KEY", "format": "JSON", "content": "Événements publics — clé requise"},
			{"url": base + "/events.ics", "format": "iCalendar", "content": "Agenda public, abonnable — libre d'accès"},
		},
	})
}

func handleExportCardsJSON(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	cardList := []Card{}
	cur, err := cards().Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := cur.All(ctx, &cardList); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	linkList := []Link{}
	lcur, err := links().Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err == nil {
		_ = lcur.All(ctx, &linkList)
	}
	type exportLink struct {
		ID               any       `json:"_id"`
		SourceCardID     any       `json:"sourceCardId"`
		TargetCardID     any       `json:"targetCardId"`
		RelationshipType string    `json:"relationshipType"`
		CreatedAt        time.Time `json:"createdAt"`
	}
	outLinks := make([]exportLink, 0, len(linkList))
	for _, l := range linkList {
		outLinks = append(outLinks, exportLink{l.ID, l.SourceCardID, l.TargetCardID, l.RelationshipType, l.CreatedAt})
	}
	writeJSON(w, 200, map[string]any{
		"source":     exportSource,
		"notice":     exportNotice,
		"exportedAt": time.Now().UTC().Format(time.RFC3339),
		"cards":      cardList,
		"links":      outLinks,
	})
}

func handleExportCardsCSV(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	cardList := []Card{}
	cur, err := cards().Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := cur.All(ctx, &cardList); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	var b strings.Builder
	b.WriteString("\uFEFF") // BOM so Excel opens UTF-8 accents correctly
	b.WriteString("id,type,title,description,tags,url,contact,address,lat,lng,votes,isExample,createdAt\r\n")
	for _, c := range cardList {
		lat, lng := "", ""
		if c.Lat != nil {
			lat = fmt.Sprintf("%g", *c.Lat)
		}
		if c.Lng != nil {
			lng = fmt.Sprintf("%g", *c.Lng)
		}
		fields := []string{
			c.ID.Hex(), c.Type, c.Title, c.Description, strings.Join(c.Tags, "|"),
			c.URL, c.Contact, c.Address, lat, lng,
			fmt.Sprintf("%d", c.Votes), fmt.Sprintf("%t", c.IsExample),
			c.CreatedAt.UTC().Format(time.RFC3339),
		}
		for i, f := range fields {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(csvEscape(f))
		}
		b.WriteString("\r\n")
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="enbauges-cartes.csv"`)
	w.Write([]byte(b.String()))
}

func handleExportCardsGeoJSON(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	cardList := []Card{}
	cur, err := cards().Find(ctx,
		bson.M{"lat": bson.M{"$ne": nil}, "lng": bson.M{"$ne": nil}},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if err := cur.All(ctx, &cardList); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	features := []map[string]any{}
	for _, c := range cardList {
		if c.Lat == nil || c.Lng == nil {
			continue
		}
		features = append(features, map[string]any{
			"type":     "Feature",
			"geometry": map[string]any{"type": "Point", "coordinates": []float64{*c.Lng, *c.Lat}},
			"properties": map[string]any{
				"id": c.ID.Hex(), "type": c.Type, "title": c.Title,
				"description": c.Description, "tags": c.Tags,
				"url": orNil(c.URL), "address": orNil(c.Address), "votes": c.Votes,
			},
		})
	}
	w.Header().Set("Content-Type", "application/geo+json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{
		"type": "FeatureCollection", "notice": exportNotice, "source": exportSource,
		"features": features,
	})
}

func handleExportEventsJSON(w http.ResponseWriter, r *http.Request) {
	list, err := approvedEvents(r)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	out := []map[string]any{}
	for _, e := range list {
		out = append(out, map[string]any{
			"id": e.ID.Hex(), "title": e.Title, "description": orNil(e.Description),
			"startAt": e.StartAt, "endAt": e.EndAt,
			"location": orNil(e.Location), "category": orNil(e.Category),
		})
	}
	writeJSON(w, 200, map[string]any{
		"source": exportSource, "notice": exportNotice,
		"exportedAt": time.Now().UTC().Format(time.RFC3339), "events": out,
	})
}

func handleExportEventsICS(w http.ResponseWriter, r *http.Request) {
	list, err := approvedEvents(r)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	now := icsDate(time.Now())
	lines := []string{
		"BEGIN:VCALENDAR", "VERSION:2.0",
		"PRODID:-//enbauges.fr//Agenda du Coeur des Bauges//FR",
		"CALSCALE:GREGORIAN", "METHOD:PUBLISH",
		"X-WR-CALNAME:Agenda EnBauges", "X-WR-TIMEZONE:Europe/Paris",
	}
	for _, e := range list {
		lines = append(lines,
			"BEGIN:VEVENT",
			"UID:"+e.ID.Hex()+"@enbauges.fr",
			"DTSTAMP:"+now,
			"DTSTART:"+icsDate(e.StartAt),
			"DTEND:"+icsDate(e.EndAt),
			"SUMMARY:"+icsEscape(e.Title),
		)
		if e.Description != "" {
			lines = append(lines, "DESCRIPTION:"+icsEscape(e.Description))
		}
		if e.Location != "" {
			lines = append(lines, "LOCATION:"+icsEscape(e.Location))
		}
		if e.Category != "" {
			lines = append(lines, "CATEGORIES:"+icsEscape(e.Category))
		}
		lines = append(lines, "END:VEVENT")
	}
	lines = append(lines, "END:VCALENDAR")
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="enbauges-agenda.ics"`)
	w.Write([]byte(strings.Join(lines, "\r\n") + "\r\n"))
}

func approvedEvents(r *http.Request) ([]Event, error) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	list := []Event{}
	cur, err := events().Find(ctx, bson.M{"status": "approved"},
		options.Find().SetSort(bson.D{{Key: "startAt", Value: 1}}))
	if err != nil {
		return nil, err
	}
	return list, cur.All(ctx, &list)
}

func orNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}
