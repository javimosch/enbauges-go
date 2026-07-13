package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	agendaTmpl   *template.Template
	annuaireTmpl *template.Template
	parisTZ      *time.Location
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
	agendaTmpl = template.Must(template.ParseFS(webFS, "web/agenda.html"))
	annuaireTmpl = template.Must(template.ParseFS(webFS, "web/annuaire.html"))
}

func servePage(name string) http.HandlerFunc {
	data, err := webFS.ReadFile("web/" + name)
	if err != nil {
		log.Fatalf("missing embedded page %s: %v", name, err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	sub, _ := fs.Sub(webFS, "web")
	http.FileServerFS(sub).ServeHTTP(w, r)
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
	if err := agendaTmpl.Execute(w, map[string]any{"Months": months}); err != nil {
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
	err = annuaireTmpl.Execute(w, map[string]any{
		"Total":       len(all),
		"Sections":    sections,
		"SiteURL":     baseURL(r) + "/",
		"GeneratedAt": fmt.Sprintf("%d %s %d", now.Day(), frMonths[now.Month()-1], now.Year()),
	})
	if err != nil {
		log.Println("annuaire template:", err)
	}
}
