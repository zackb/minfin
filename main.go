package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	accessFile = "accessurl.txt"
	dbFile     = "minfin.db"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed web/*
var webFS embed.FS

var tmpl = template.Must(template.ParseFS(templatesFS, "templates/*.html"))

var db *DB

func main() {
	var err error
	db, err = Open(dbFile)
	if err != nil {
		log.Fatal(err)
	}
	if access := readAccess(); access != "" {
		go func() { // initial sync at startup, non-blocking
			if err := Sync(db, access); err != nil {
				log.Printf("startup sync: %v", err)
			}
		}()
	}
	go syncLoop(db, 6*time.Hour)

	http.Handle("/web/", http.FileServerFS(webFS))
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/setup", handleSetup)
	http.HandleFunc("/sync", handleSync)
	log.Println("listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func readAccess() string {
	b, _ := os.ReadFile(accessFile)
	return strings.TrimSpace(string(b))
}

type dashboardView struct {
	Connected       bool
	Error           string
	Range           string
	RangeLabel      string
	Interval        string
	Split           bool
	RangeOptions    []rangeOption
	IntervalOptions []string
	ChartJSON       template.JS
	Payees          []PayeeStat
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	v := dashboardView{
		Range:           orDefault(r.URL.Query().Get("range"), "last-30-days"),
		Interval:        orDefault(r.URL.Query().Get("interval"), "daily"),
		Split:           r.URL.Query().Get("split") == "1",
		RangeOptions:    rangeOptions,
		IntervalOptions: intervalOptions,
	}
	v.RangeLabel = rangeLabel(v.Range)

	if readAccess() == "" {
		render(w, v) // setup form
		return
	}
	v.Connected = true

	start, end := resolveRange(v.Range, time.Now())
	series, err := db.SpendingSeries(start, end, v.Interval, v.Split)
	if err != nil {
		v.Error = err.Error()
		render(w, v)
		return
	}
	j, _ := json.Marshal(series)
	v.ChartJSON = template.JS(j)

	if v.Payees, err = db.TopPayees(start, end, 15); err != nil {
		v.Error = err.Error()
	}
	render(w, v)
}

func handleSetup(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.FormValue("token"))
	if token == "" {
		http.Error(w, "setup token required", http.StatusBadRequest)
		return
	}
	access, err := Claim(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := os.WriteFile(accessFile, []byte(access), 0o600); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := Sync(db, access); err != nil {
		log.Printf("initial sync: %v", err)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleSync(w http.ResponseWriter, r *http.Request) {
	if access := readAccess(); access != "" {
		if err := Sync(db, access); err != nil {
			log.Printf("manual sync: %v", err)
		}
	}
	http.Redirect(w, r, orDefault(r.Header.Get("Referer"), "/"), http.StatusSeeOther)
}

func render(w http.ResponseWriter, v dashboardView) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "dashboard.html", v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
