// Package web serves the dashboard: a collapsible-sidebar layout with the
// Spending, Accounts, and Transactions screens.
package web

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"strconv"
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

var funcs = template.FuncMap{"money": money}

// money formats signed dollars US-style with thousands separators, keeping the
// $ left of the sign: 1234.5 -> "$1,234.50", -1234.56 -> "-$1,234.56".
func money(d float64) string {
	neg := d < 0
	if neg {
		d = -d
	}
	intPart, frac, _ := strings.Cut(strconv.FormatFloat(d, 'f', 2, 64), ".")
	var b strings.Builder
	for i := 0; i < len(intPart); i++ {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(intPart[i])
	}
	out := "$" + b.String() + "." + frac
	if neg {
		out = "-" + out
	}
	return out
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
	Notices   []string // SimpleFIN connection warnings from the last sync
	LastSync  string   // human-readable time of last sync, "" if never
}

func (s *Server) accessURL() string {
	u, _ := s.store.AccessURL()
	return u
}

// base builds the per-page layout fields, including the last sync time and any
// account-health notices from SimpleFIN.
func (s *Server) base(active string) viewBase {
	b := viewBase{Active: active, Connected: s.accessURL() != ""}
	st, err := s.store.SyncStatus()
	if err != nil {
		return b
	}
	if !st.At.IsZero() {
		b.LastSync = st.At.Format("Jan 2 3:04 PM")
	}
	for _, e := range st.Errors {
		// Skip our own query-window advisories; surface real account issues.
		if strings.HasPrefix(e, "Requested date range") {
			continue
		}
		b.Notices = append(b.Notices, e)
	}
	return b
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
