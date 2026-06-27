// Package web serves the dashboard: a collapsible-sidebar layout with the
// Spending, Accounts, and Transactions screens.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/zackb/minfin/internal/simplefin"
	"github.com/zackb/minfin/internal/store"
	"github.com/zackb/minfin/internal/syncer"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Server holds dependencies and the parsed page templates. Each page is parsed
// with the shared layout into its own set so pages can define their own
// "content" block without name collisions.
type Server struct {
	store *store.Store
	mux   *http.ServeMux
	pages map[string]*template.Template
}

func NewServer(s *store.Store) *Server {
	srv := &Server{
		store: s,
		mux:   http.NewServeMux(),
		pages: map[string]*template.Template{
			"spending":     page("spending.html"),
			"accounts":     page("accounts.html"),
			"transactions": page("transactions.html"),
		},
	}
	srv.mux.Handle("/static/", http.FileServerFS(staticFS))
	srv.mux.HandleFunc("/", srv.handleSpending)
	srv.mux.HandleFunc("/accounts", srv.handleAccounts)
	srv.mux.HandleFunc("/transactions", srv.handleTransactions)
	srv.mux.HandleFunc("/setup", srv.handleSetup)
	srv.mux.HandleFunc("/sync", srv.handleSync)
	return srv
}

func (s *Server) Handler() http.Handler { return s.mux }

// money formats signed dollars as -$12.34 / $12.34 (keeps the $ left of the sign).
var funcs = template.FuncMap{
	"money": func(d float64) string {
		if d < 0 {
			return fmt.Sprintf("-$%.2f", -d)
		}
		return fmt.Sprintf("$%.2f", d)
	},
}

func page(name string) *template.Template {
	return template.Must(template.New(name).Funcs(funcs).
		ParseFS(templatesFS, "templates/layout.html", "templates/"+name))
}

// viewBase carries fields the shared layout (sidebar) needs on every page.
type viewBase struct {
	Active    string // "spending" | "accounts" | "transactions"
	Connected bool
	Error     string
}

func (s *Server) accessURL() string {
	u, _ := s.store.AccessURL()
	return u
}

func (s *Server) render(w http.ResponseWriter, name string, v any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.pages[name].ExecuteTemplate(w, "layout", v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.FormValue("token"))
	if token == "" {
		http.Error(w, "setup token required", http.StatusBadRequest)
		return
	}
	access, err := simplefin.Claim(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := s.store.SetAccessURL(access); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := syncer.Sync(s.store, access); err != nil {
		log.Printf("initial sync: %v", err)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if access := s.accessURL(); access != "" {
		if err := syncer.Sync(s.store, access); err != nil {
			log.Printf("manual sync: %v", err)
		}
	}
	http.Redirect(w, r, orDefault(r.Header.Get("Referer"), "/"), http.StatusSeeOther)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
