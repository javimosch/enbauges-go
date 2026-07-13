package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	tmplCache = map[string]*template.Template{}
	parisTZ   *time.Location
)

var frMonths = [...]string{"janvier", "février", "mars", "avril", "mai", "juin",
	"juillet", "août", "septembre", "octobre", "novembre", "décembre"}
var frWeekdays = [...]string{"dim.", "lun.", "mar.", "mer.", "jeu.", "ven.", "sam."}

func init() {
	var err error
	parisTZ, err = time.LoadLocation("Europe/Paris")
	if err != nil {
		parisTZ = time.UTC
	}
}

// getTemplate returns the cached embedded template unless a disk override
// exists, in which case it re-parses per request so UI edits apply live.
func getTemplate(name string) (*template.Template, error) {
	if !webOverlay.OnDisk(name) {
		if t, ok := tmplCache[name]; ok {
			return t, nil
		}
	}
	data, err := webOverlay.ReadFile(name)
	if err != nil {
		return nil, err
	}
	t, err := template.New(name).Parse(string(data))
	if err != nil {
		return nil, err
	}
	if !webOverlay.OnDisk(name) {
		tmplCache[name] = t
	}
	return t, nil
}

func servePage(name string) http.HandlerFunc {
	if _, err := webOverlay.ReadFile(name); err != nil {
		log.Fatalf("missing page %s: %v", name, err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := webOverlay.ReadFile(name)
		if err != nil {
			http.Error(w, "page unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(data)
	}
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	// no-cache = always revalidate (304 when unchanged): UI edits shipped
	// via the overlay reach browsers immediately.
	w.Header().Set("Cache-Control", "no-cache")
	http.FileServerFS(webOverlay).ServeHTTP(w, r)
}

type agendaEvent struct {
	Title, Description, Location, Category, OrgName string
	Day, Weekday, Time                              string
}

type agendaMonth struct {
	Label  string
	Events []agendaEvent
}

func handleAgendaPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	list := []Event{}
	cur, err := events().Find(ctx,
		bson.M{"status": "approved", "endAt": bson.M{"$gte": time.Now()}},
		options.Find().SetSort(bson.D{{Key: "startAt", Value: 1}}).SetLimit(200))
	if err == nil {
		_ = cur.All(ctx, &list)
	}

	months := []agendaMonth{}
	var current *agendaMonth
	for _, e := range list {
		start := e.StartAt.In(parisTZ)
		end := e.EndAt.In(parisTZ)
		label := fmt.Sprintf("%s %d", frMonths[start.Month()-1], start.Year())
		if current == nil || current.Label != label {
			months = append(months, agendaMonth{Label: label})
			current = &months[len(months)-1]
		}
		current.Events = append(current.Events, agendaEvent{
			Title:       e.Title,
			Description: e.Description,
			Location:    e.Location,
			Category:    e.Category,
			Day:         fmt.Sprintf("%d", start.Day()),
			Weekday:     frWeekdays[start.Weekday()],
			Time:        start.Format("15:04") + " – " + end.Format("15:04"),
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, err := getTemplate("agenda.html")
	if err != nil {
		http.Error(w, "template error", 500)
		log.Println("agenda template:", err)
		return
	}
	if err := tmpl.Execute(w, map[string]any{"Months": months}); err != nil {
		log.Println("agenda template:", err)
	}
}

type annuaireSection struct {
	Label string
	Cards []Card
}

func handleAnnuairePage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()
	all := []Card{}
	cur, err := cards().Find(ctx, bson.M{"isExample": false},
		options.Find().
			SetSort(bson.D{{Key: "title", Value: 1}}).
			SetCollation(&options.Collation{Locale: "fr", Strength: 1}))
	if err == nil {
		_ = cur.All(ctx, &all)
	}

	sections := []*annuaireSection{
		{Label: "Acteurs"}, {Label: "Solutions"}, {Label: "Initiatives"},
	}
	byType := map[string]*annuaireSection{
		"actor": sections[0], "solution": sections[1], "initiative": sections[2],
	}
	for _, c := range all {
		if s, ok := byType[c.Type]; ok {
			s.Cards = append(s.Cards, c)
		}
	}

	now := time.Now().In(parisTZ)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, terr := getTemplate("annuaire.html")
	if terr != nil {
		http.Error(w, "template error", 500)
		log.Println("annuaire template:", terr)
		return
	}
	err = tmpl.Execute(w, map[string]any{
		"Total":       len(all),
		"Sections":    sections,
		"SiteURL":     baseURL(r) + "/",
		"GeneratedAt": fmt.Sprintf("%d %s %d", now.Day(), frMonths[now.Month()-1], now.Year()),
	})
	if err != nil {
		log.Println("annuaire template:", err)
	}
}
