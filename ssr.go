package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// backfillSlugs generates slugs for existing cards that don't have one.
// Runs once on startup, idempotent.
func backfillSlugs(ctx context.Context) {
	// Match cards where slug is missing, empty, or null
	filter := bson.M{"$or": []bson.M{
		{"slug": bson.M{"$exists": false}},
		{"slug": ""},
		{"slug": nil},
	}}
	cur, err := cards().Find(ctx, filter)
	if err != nil {
		log.Printf("slug backfill: find error: %v", err)
		return
	}
	var noSlug []Card
	if err := cur.All(ctx, &noSlug); err != nil {
		log.Printf("slug backfill: cursor error: %v", err)
		return
	}
	if len(noSlug) == 0 {
		log.Printf("slug backfill: no cards without slugs (all good)")
		return
	}
	log.Printf("backfilling slugs for %d cards", len(noSlug))
	done := 0
	for _, c := range noSlug {
		slug, err := generateUniqueSlug(ctx, c.Title, &c.ID)
		if err != nil {
			log.Printf("slug backfill: error generating slug for %q: %v", c.Title, err)
			continue
		}
		_, err = cards().UpdateByID(ctx, c.ID, bson.M{"$set": bson.M{"slug": slug}})
		if err != nil {
			log.Printf("slug backfill: error setting slug for %q: %v", c.Title, err)
			continue
		}
		done++
	}
	log.Printf("slug backfill complete: %d/%d cards updated", done, len(noSlug))
}

// nominatimResult is the minimal response from Nominatim geocoding.
type nominatimResult struct {
	Lat string `json:"lat"`
	Lon string `json:"lon"`
}

// geocodeAddress uses Nominatim to resolve an address to lat/lng.
// Returns nil if geocoding fails or returns no results.
// The result is cached back onto the card in MongoDB.
func geocodeAddress(ctx context.Context, cardID primitive.ObjectID, address string) (lat, lng float64, ok bool) {
	url := "https://nominatim.openstreetmap.org/search?q=" + urlEscape(address) + "&format=json&limit=1&countrycodes=fr"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, 0, false
	}
	req.Header.Set("User-Agent", "EnBauges/1.0 (https://enbauges.fr)")
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, 0, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return 0, 0, false
	}
	var results []nominatimResult
	if err := json.Unmarshal(body, &results); err != nil || len(results) == 0 {
		return 0, 0, false
	}
	lat, err = parseFloat(results[0].Lat)
	if err != nil {
		return 0, 0, false
	}
	lng, err = parseFloat(results[0].Lon)
	if err != nil {
		return 0, 0, false
	}
	// Cache result back onto the card
	_, _ = cards().UpdateByID(ctx, cardID, bson.M{"$set": bson.M{"lat": lat, "lng": lng}})
	return lat, lng, true
}

// osmMapHTML returns an iframe embedding an OpenStreetMap with a marker.
func osmMapHTML(lat, lng float64) string {
	delta := 0.008
	bbox := fmt.Sprintf("%f,%f,%f,%f", lng-delta, lat-delta, lng+delta, lat+delta)
	src := fmt.Sprintf("https://www.openstreetmap.org/export/embed.html?bbox=%s&layer=mapnik&marker=%f,%f", bbox, lat, lng)
	link := fmt.Sprintf("https://www.openstreetmap.org/?mlat=%f&mlon=%f#map=15/%f/%f", lat, lng, lat, lng)
	return fmt.Sprintf(`<div class="ssr-map"><iframe src="%s" loading="lazy" title="Carte OpenStreetMap"></iframe><a href="%s" target="_blank" rel="noopener">Voir sur OpenStreetMap ↗</a></div>`, src, link)
}

// handleCardDetailPage renders a server-side HTML page for a single card,
// with SEO meta tags, Open Graph, JSON-LD, and a share button.
func handleCardDetailPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	slug := r.PathValue("slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	var card Card
	err := cards().FindOne(ctx, bson.M{"slug": slug}).Decode(&card)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Get link count
	linkCount, _ := links().CountDocuments(ctx, bson.M{
		"$or": []bson.M{
			{"sourceCardId": card.ID},
			{"targetCardId": card.ID},
		},
	})

	// Build the page
	siteURL := baseURL(r)
	canonical := siteURL + "/carte/" + card.Slug
	typeLabel := "Acteur"
	if card.Type == "solution" {
		typeLabel = "Solution"
	} else if card.Type == "initiative" {
		typeLabel = "Initiative"
	}

	// Meta description (first 160 chars of description)
	desc := card.Description
	if len([]rune(desc)) > 160 {
		desc = string([]rune(desc)[:157]) + "…"
	}

	// Open Graph image
	var ogImage string
	if len(card.Images) > 0 {
		ogImage = card.Images[0]
	}

	// JSON-LD structured data
	schemaType := "Place"
	if card.Type == "actor" {
		schemaType = "Organization"
	}
	jsonLD := fmt.Sprintf(`{"@context":"https://schema.org","@type":"%s","name":%q,"description":%q`,
		schemaType, escapeJSON(card.Title), escapeJSON(card.Description))
	if ogImage != "" {
		jsonLD += fmt.Sprintf(`,"image":%q`, escapeJSON(ogImage))
	}
	if card.Address != "" {
		jsonLD += fmt.Sprintf(`,"address":{"@type":"PostalAddress","streetAddress":%q}`, escapeJSON(card.Address))
	}
	if card.URL != "" {
		jsonLD += fmt.Sprintf(`,"url":%q`, escapeJSON(card.URL))
	}
	jsonLD += "}"

	// Tags HTML
	tagsHTML := ""
	if len(card.Tags) > 0 {
		var tb strings.Builder
		for _, t := range card.Tags {
			tb.WriteString(fmt.Sprintf(`<span class="enbauges-tag">%s</span>`, escapeHTML(t)))
		}
		tagsHTML = tb.String()
	}

	// Images HTML
	imagesHTML := ""
	if len(card.Images) > 0 {
		var ib strings.Builder
		ib.WriteString(`<div class="ssr-gallery">`)
		for _, img := range card.Images {
			ib.WriteString(fmt.Sprintf(`<img src="%s" alt="%s" loading="lazy">`, escapeAttr(img), escapeAttr(card.Title)))
		}
		ib.WriteString(`</div>`)
		imagesHTML = ib.String()
	}

	// Contact HTML
	contactHTML := ""
	if card.Contact != "" {
		contactHTML = fmt.Sprintf(`<div class="ssr-contact"><strong>Contact :</strong> %s</div>`, escapeHTML(card.Contact))
	}

	// URL HTML
	urlHTML := ""
	if card.URL != "" {
		urlHTML = fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener" class="enbauges-link">↗ Voir le site</a>`, escapeAttr(card.URL))
	}

	// Address HTML
	addressHTML := ""
	if card.Address != "" {
		addressHTML = fmt.Sprintf(`<div class="ssr-address">📍 %s</div>`, escapeHTML(card.Address))
	}

	// OSM map: show if card has lat/lng, or if we can geocode the address
	mapHTML := ""
	var mapLat, mapLng float64
	if card.Lat != nil && card.Lng != nil {
		mapLat = *card.Lat
		mapLng = *card.Lng
		mapHTML = osmMapHTML(mapLat, mapLng)
	} else if card.Address != "" {
		// Try geocoding with a short timeout (non-blocking for the page if it fails)
		geoCtx, geoCancel := context.WithTimeout(ctx, 3*time.Second)
		defer geoCancel()
		if lat, lng, ok := geocodeAddress(geoCtx, card.ID, card.Address); ok {
			mapLat = lat
			mapLng = lng
			mapHTML = osmMapHTML(mapLat, mapLng)
		}
	}

	// Add geo to JSON-LD if we have coordinates
	if mapHTML != "" {
		jsonLD = strings.TrimSuffix(jsonLD, "}")
		jsonLD += fmt.Sprintf(`,"geo":{"@type":"GeoCoordinates","latitude":%f,"longitude":%f}}`, mapLat, mapLng)
	}

	page := fmt.Sprintf(`<!DOCTYPE html>
<html lang="fr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s — enbauges.fr</title>
<meta name="description" content="%s">
<link rel="canonical" href="%s">
<link href="/css/enbauges.css" rel="stylesheet">
<meta property="og:title" content="%s">
<meta property="og:description" content="%s">
<meta property="og:url" content="%s">
<meta property="og:type" content="website">
%s
<meta name="twitter:card" content="%s">
<meta name="twitter:title" content="%s">
<meta name="twitter:description" content="%s">
%s
<script type="application/ld+json">%s</script>
</head>
<body>
<div class="ssr-page">
  <nav class="ssr-nav">
    <a href="/">← enbauges.fr</a>
  </nav>
  <article class="ssr-card">
    <div class="ssr-header">
      <span class="enbauges-badge-%s">%s</span>
      <h1>%s</h1>
    </div>
    %s
    <p class="ssr-description">%s</p>
    <div class="ssr-tags">%s</div>
    <div class="ssr-meta">
      <span>👍 %d Utile</span>
      <span>🔗 %d %s</span>
    </div>
    %s
    %s
    %s
    %s
    <div class="ssr-actions">
      <a href="/#/card/%s" class="enbauges-btn-primary">🎯 Voir sur le canvas</a>
      <button onclick="sharePage()" class="enbauges-btn-secondary">🔗 Partager</button>
    </div>
  </article>
</div>
<script>
function sharePage() {
  const url = window.location.href;
  const title = document.title;
  if (navigator.share) {
    navigator.share({title: title, url: url}).catch(()=>{});
  } else {
    navigator.clipboard.writeText(url).then(()=>{
      const btn = event.target;
      btn.textContent = '✓ Copié !';
      setTimeout(()=>{ btn.textContent = '🔗 Partager'; }, 2000);
    });
  }
}
</script>
</body>
</html>`,
		escapeHTML(card.Title),          // <title>
		escapeAttr(desc),                // meta description
		escapeAttr(canonical),           // canonical
		escapeAttr(card.Title),          // og:title
		escapeAttr(desc),                // og:description
		escapeAttr(canonical),           // og:url
		ogImageTag(ogImage),             // og:image
		ogImageOrSummary(ogImage),       // twitter:card
		escapeAttr(card.Title),          // twitter:title
		escapeAttr(desc),                // twitter:description
		ogImageTwitterTag(ogImage),      // twitter:image
		jsonLD,                          // JSON-LD
		escapeHTML(card.Type),           // badge class
		typeLabel,                       // badge label
		escapeHTML(card.Title),          // <h1>
		imagesHTML,                      // gallery
		escapeHTML(card.Description),    // description
		tagsHTML,                        // tags
		card.Votes,                      // votes
		linkCount,                       // link count
		linkCountStr(linkCount),         // "lien" or "liens"
		addressHTML,                     // address
		contactHTML,                     // contact
		urlHTML,                         // url
		mapHTML,                         // OSM map
		card.ID.Hex(),                   // card ID for canvas link
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Write([]byte(page))
}

// handleSitemap generates a sitemap.xml listing all card detail pages.
func handleSitemap(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	cur, err := cards().Find(ctx, bson.M{"slug": bson.M{"$nin": []any{"", nil}}}, nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	var allCards []Card
	_ = cur.All(ctx, &allCards)

	siteURL := baseURL(r)
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	sb.WriteString(fmt.Sprintf(`<url><loc>%s/</loc><changefreq>daily</changefreq><priority>1.0</priority></url>`, siteURL))
	sb.WriteString(fmt.Sprintf(`<url><loc>%s/annuaire</loc><changefreq>weekly</changefreq><priority>0.8</priority></url>`, siteURL))
	for _, c := range allCards {
		lastmod := c.UpdatedAt.Format("2006-01-02")
		sb.WriteString(fmt.Sprintf(`<url><loc>%s/carte/%s</loc><lastmod>%s</lastmod><changefreq>weekly</changefreq><priority>0.6</priority></url>`,
			siteURL, escapeXML(c.Slug), lastmod))
	}
	sb.WriteString(`</urlset>`)

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(sb.String()))
}

// --- helpers ---

func ogImageTag(url string) string {
	if url == "" {
		return ""
	}
	return fmt.Sprintf(`<meta property="og:image" content="%s">`, escapeAttr(url))
}

func ogImageTwitterTag(url string) string {
	if url == "" {
		return ""
	}
	return fmt.Sprintf(`<meta name="twitter:image" content="%s">`, escapeAttr(url))
}

func ogImageOrSummary(url string) string {
	if url != "" {
		return "summary_large_image"
	}
	return "summary"
}

func linkCountStr(n int64) string {
	if n == 1 {
		return "lien"
	}
	return "liens"
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func escapeAttr(s string) string {
	s = escapeHTML(s)
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// keep primitive import used
var _ = primitive.NewObjectID

func urlEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "&", "%26")
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
