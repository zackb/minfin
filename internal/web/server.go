// Package web serves the dashboard: a collapsible-sidebar layout with the
// Spending, Accounts, Transactions, and Categories screens.
package web

import (
	"context"
	"embed"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/zackb/minfin/internal/auth"
	"github.com/zackb/minfin/internal/simplefin"
	"github.com/zackb/minfin/internal/store"
	"github.com/zackb/minfin/internal/syncer"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// Server holds dependencies and the parsed page templates. Each page is parsed
// with the shared layout into its own set so pages can define their own
// "content" block without name collisions.
type Server struct {
	store *store.Store
	auth  *auth.Service
	mux   *http.ServeMux
	pages map[string]*template.Template
	// localUID, when set, puts the server in single-user desktop mode: auth is
	// bypassed and every request runs as this user. Empty for the multi-user
	// server. See SetLocalUser.
	localUID string
}

func NewServer(s *store.Store, a *auth.Service) *Server {
	srv := &Server{
		store: s,
		auth:  a,
		mux:   http.NewServeMux(),
		pages: map[string]*template.Template{
			"home":         page("home.html"),
			"spending":     page("spending.html"),
			"accounts":     page("accounts.html"),
			"transactions": page("transactions.html"),
			"categories":   page("categories.html"),
			"login":        authPage("login.html"),
			"signup":       authPage("signup.html"),
		},
	}
	srv.mux.Handle("/static/", http.FileServerFS(staticFS))
	srv.mux.HandleFunc("/login", srv.handleLogin)
	srv.mux.HandleFunc("/signup", srv.handleSignup)
	srv.mux.HandleFunc("/logout", srv.handleLogout)
	srv.mux.HandleFunc("/", srv.handleHome)
	srv.mux.HandleFunc("/spending", srv.handleSpending)
	srv.mux.HandleFunc("/accounts", srv.handleAccounts)
	srv.mux.HandleFunc("/accounts/type", srv.handleAccountType)
	srv.mux.HandleFunc("/accounts/nickname", srv.handleAccountNickname)
	srv.mux.HandleFunc("/accounts/asset-value", srv.handleAccountAssetValue)
	srv.mux.HandleFunc("/transactions", srv.handleTransactions)
	srv.mux.HandleFunc("/transactions/category", srv.handleTxnCategory)
	srv.mux.HandleFunc("/categories", srv.handleCategories)
	srv.mux.HandleFunc("/categories/add", srv.handleCategoryAdd)
	srv.mux.HandleFunc("/categories/delete", srv.handleCategoryDelete)
	srv.mux.HandleFunc("/categories/exclude", srv.handleCategoryExclude)
	srv.mux.HandleFunc("/categories/rule/add", srv.handleRuleAdd)
	srv.mux.HandleFunc("/categories/rule/delete", srv.handleRuleDelete)
	srv.mux.HandleFunc("/categories/recategorize", srv.handleRecategorize)
	srv.mux.HandleFunc("/setup", srv.handleSetup)
	srv.mux.HandleFunc("/sync", srv.handleSync)
	srv.registerAPI()
	return srv
}

func (s *Server) Handler() http.Handler { return s.withAuth(s.mux) }

// SetLocalUser switches the server into single-user desktop mode: withAuth
// skips token checks and runs every request as uid. Used by the desktop
// launcher so there's no login screen on a local machine.
func (s *Server) SetLocalUser(uid string) { s.localUID = uid }

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxPortfolioID
)

func userID(r *http.Request) string {
	v, _ := r.Context().Value(ctxUserID).(string)
	return v
}

func portfolioID(r *http.Request) string {
	v, _ := r.Context().Value(ctxPortfolioID).(string)
	return v
}

// publicPaths bypass auth: static assets, the auth screens, and the API's own
// login/signup endpoints (which mint the token everything else requires).
func publicPath(p string) bool {
	return p == "/login" || p == "/signup" || p == "/logout" ||
		p == "/api/login" || p == "/api/signup" || strings.HasPrefix(p, "/static/")
}

// withAuth requires a valid token on every non-public route, then resolves the
// user's active portfolio (the first they belong to; multi-portfolio switching
// is deferred) and stashes both ids on the request context.
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		uid, ok := s.localUID, s.localUID != ""
		if !ok {
			uid, ok = s.auth.IsAuthenticated(r)
		}
		if !ok {
			// API clients get a 401 JSON body; browsers get redirected to login.
			if strings.HasPrefix(r.URL.Path, "/api/") {
				apiError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		pid := ""
		if ps, err := s.store.PortfoliosForUser(uid); err == nil && len(ps) > 0 {
			pid = ps[0].ID
		}
		ctx := context.WithValue(r.Context(), ctxUserID, uid)
		ctx = context.WithValue(ctx, ctxPortfolioID, pid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

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

// authPage parses a standalone auth screen (no app sidebar layout).
func authPage(name string) *template.Template {
	return template.Must(template.New(name).Funcs(funcs).
		ParseFS(templatesFS, "templates/"+name))
}

// viewBase carries fields the shared layout (sidebar) needs on every page.
type viewBase struct {
	Active    string // "spending" | "accounts" | "transactions" | "categories"
	Connected bool   // the active portfolio has a SimpleFIN token
	Email     string // signed-in user, for the header/logout affordance
	Error     string
	Notices   []string // SimpleFIN connection warnings from the last sync
	LastSync  string   // human-readable time of last sync, "" if never
}

// base builds the per-page layout fields for the signed-in user's active
// portfolio, including the last sync time and any account-health notices.
func (s *Server) base(r *http.Request, active string) viewBase {
	pid := portfolioID(r)
	b := viewBase{Active: active, Connected: pid != ""}
	if u, err := s.store.UserByID(userID(r)); err == nil {
		b.Email = u.Email
	}
	if pid == "" {
		return b
	}
	st, err := s.store.PortfolioSyncStatus(pid)
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

// authView backs the standalone login/signup pages.
type authView struct {
	Error string
	Email string
}

func (s *Server) renderAuth(w http.ResponseWriter, name string, v authView) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.pages[name].ExecuteTemplate(w, name, v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		email := strings.TrimSpace(r.FormValue("email"))
		u, err := s.store.UserByEmail(email)
		if err != nil || !auth.CheckPassword(u.PasswordHash, r.FormValue("password")) {
			s.renderAuth(w, "login", authView{Error: "Invalid email or password", Email: email})
			return
		}
		tok, err := s.auth.CreateToken(u.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.auth.SetCookie(w, tok)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.renderAuth(w, "login", authView{})
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		email := strings.TrimSpace(r.FormValue("email"))
		pw := r.FormValue("password")
		if email == "" || len(pw) < 8 {
			s.renderAuth(w, "signup", authView{Error: "Email and an 8+ character password are required", Email: email})
			return
		}
		hash, err := auth.HashPassword(pw)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		u, err := s.store.CreateUser(email, hash)
		if err == store.ErrEmailTaken {
			s.renderAuth(w, "signup", authView{Error: "That email is already registered", Email: email})
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tok, err := s.auth.CreateToken(u.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.auth.SetCookie(w, tok)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.renderAuth(w, "signup", authView{})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.ClearCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
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
	pid, err := s.store.CreatePortfolio("", access)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.store.AddMember(pid, userID(r), "owner"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := syncer.Sync(s.store, pid, access); err != nil {
		log.Printf("initial sync: %v", err)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if pid := portfolioID(r); pid != "" {
		if p, err := s.store.PortfolioByID(pid); err == nil && p.AccessURL != "" {
			if err := syncer.Sync(s.store, pid, p.AccessURL); err != nil {
				log.Printf("manual sync: %v", err)
			}
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
