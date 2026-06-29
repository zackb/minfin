// JSON REST API mirroring the web screens for non-browser clients (radiator,
// iOS, GTK). Handlers are thin wrappers over the same store methods the HTML
// handlers use; auth is the shared withAuth middleware (Bearer token, see
// auth.IsAuthenticated). Tokens are issued by /api/login and /api/signup and
// returned in the body — clients send them as "Authorization: Bearer <jwt>".
package web

import (
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zackb/minfin/internal/auth"
	"github.com/zackb/minfin/internal/daterange"
	"github.com/zackb/minfin/internal/simplefin"
	"github.com/zackb/minfin/internal/store"
	"github.com/zackb/minfin/internal/syncer"
)

// registerAPI wires the /api routes onto the mux using method patterns. Auth is
// applied by the shared withAuth middleware; /api/login and /api/signup are
// listed in publicPath so they bypass it.
func (s *Server) registerAPI() {
	m := s.mux
	m.HandleFunc("POST /api/signup", s.apiSignup)
	m.HandleFunc("POST /api/login", s.apiLogin)
	m.HandleFunc("GET /api/me", s.apiMe)
	m.HandleFunc("POST /api/setup", s.apiSetup)
	m.HandleFunc("POST /api/sync", s.apiSync)

	m.HandleFunc("GET /api/accounts", s.apiAccounts)
	m.HandleFunc("POST /api/accounts/type", s.apiAccountType)
	m.HandleFunc("POST /api/accounts/nickname", s.apiAccountNickname)
	m.HandleFunc("POST /api/accounts/asset-value", s.apiAccountAssetValue)

	m.HandleFunc("GET /api/transactions", s.apiTransactions)
	m.HandleFunc("POST /api/transactions/category", s.apiTxnCategory)

	m.HandleFunc("GET /api/categories", s.apiCategories)
	m.HandleFunc("POST /api/categories", s.apiCategoryAdd)
	m.HandleFunc("DELETE /api/categories", s.apiCategoryDelete) // ?name=
	m.HandleFunc("POST /api/categories/exclude", s.apiCategoryExclude)
	m.HandleFunc("POST /api/categories/recategorize", s.apiRecategorize)
	m.HandleFunc("POST /api/categories/rules", s.apiRuleAdd)
	m.HandleFunc("DELETE /api/categories/rules/{id}", s.apiRuleDelete)

	m.HandleFunc("GET /api/spending", s.apiSpending)
}

// helpers

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func apiError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// decode reads a JSON request body into dst; reports false (and writes 400) on
// malformed input.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

// auth

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) apiSignup(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	email := strings.TrimSpace(c.Email)
	if email == "" || len(c.Password) < 8 {
		apiError(w, http.StatusBadRequest, "email and an 8+ character password are required")
		return
	}
	hash, err := auth.HashPassword(c.Password)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	u, err := s.store.CreateUser(email, hash)
	if errors.Is(err, store.ErrEmailTaken) {
		apiError(w, http.StatusConflict, "that email is already registered")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.issueToken(w, u.ID)
}

func (s *Server) apiLogin(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	u, err := s.store.UserByEmail(strings.TrimSpace(c.Email))
	if err != nil || !auth.CheckPassword(u.PasswordHash, c.Password) {
		apiError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	s.issueToken(w, u.ID)
}

func (s *Server) issueToken(w http.ResponseWriter, userID string) {
	tok, err := s.auth.CreateToken(userID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": tok})
}

// apiMe reports the signed-in user and whether they've connected SimpleFIN yet,
// so clients know whether to show the onboarding flow.
func (s *Server) apiMe(w http.ResponseWriter, r *http.Request) {
	b := s.base(r, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"email":     b.Email,
		"connected": b.Connected,
		"lastSync":  b.LastSync,
		"notices":   b.Notices,
	})
}

// onboarding / sync

func (s *Server) apiSetup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &body) {
		return
	}
	token := strings.TrimSpace(body.Token)
	if token == "" {
		apiError(w, http.StatusBadRequest, "setup token required")
		return
	}
	access, err := simplefin.Claim(token)
	if err != nil {
		apiError(w, http.StatusBadGateway, err.Error())
		return
	}
	pid, err := s.store.CreatePortfolio("", access)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.store.AddMember(pid, userID(r), "owner"); err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := syncer.Sync(s.store, pid, access); err != nil {
		// Non-fatal: the portfolio is connected; a later /api/sync can retry.
		log.Printf("api initial sync: %v", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiSync(w http.ResponseWriter, r *http.Request) {
	pid := portfolioID(r)
	if pid == "" {
		apiError(w, http.StatusBadRequest, "no connected portfolio")
		return
	}
	if p, err := s.store.PortfolioByID(pid); err == nil && p.AccessURL != "" {
		if err := syncer.Sync(s.store, pid, p.AccessURL); err != nil {
			log.Printf("api manual sync: %v", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// accounts

func (s *Server) apiAccounts(w http.ResponseWriter, r *http.Request) {
	pid := portfolioID(r)
	if pid == "" {
		writeJSON(w, http.StatusOK, map[string]any{"accounts": []any{}, "types": store.AccountTypes})
		return
	}
	accts, err := s.store.Accounts(pid, time.Now())
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	assets, liabilities, net := summarize(accts)
	writeJSON(w, http.StatusOK, map[string]any{
		"accounts":    accts,
		"types":       store.AccountTypes,
		"assets":      assets,
		"liabilities": liabilities,
		"netWorth":    net,
	})
}

func (s *Server) apiAccountType(w http.ResponseWriter, r *http.Request) {
	var body struct{ ID, Type string }
	if !decode(w, r, &body) {
		return
	}
	if body.ID == "" || !store.ValidType(body.Type) {
		apiError(w, http.StatusBadRequest, "invalid account or type")
		return
	}
	if err := s.store.SetAccountType(portfolioID(r), body.ID, body.Type); err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiAccountNickname(w http.ResponseWriter, r *http.Request) {
	var body struct{ ID, Nickname string }
	if !decode(w, r, &body) {
		return
	}
	if body.ID == "" {
		apiError(w, http.StatusBadRequest, "invalid account")
		return
	}
	if err := s.store.SetAccountNickname(portfolioID(r), body.ID, strings.TrimSpace(body.Nickname)); err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiAccountAssetValue(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID    string  `json:"id"`
		Value float64 `json:"value"` // dollars; 0 clears
	}
	if !decode(w, r, &body) {
		return
	}
	if body.ID == "" || body.Value < 0 {
		apiError(w, http.StatusBadRequest, "invalid account or value")
		return
	}
	cents := int64(math.Round(body.Value * 100))
	if err := s.store.SetAccountAssetValue(portfolioID(r), body.ID, cents); err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// transactions

func (s *Server) apiTransactions(w http.ResponseWriter, r *http.Request) {
	pid := portfolioID(r)
	q := r.URL.Query()
	now := time.Now()
	to := orDefault(q.Get("to"), now.Format(dateLayout))
	from := orDefault(q.Get("from"), now.AddDate(0, 0, -30).Format(dateLayout))
	start := parseDate(from, now.AddDate(0, 0, -30))
	end := parseDate(to, now).AddDate(0, 0, 1) // inclusive of the "to" day

	page := 1
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 1 {
		page = p
	}
	if pid == "" {
		writeJSON(w, http.StatusOK, map[string]any{"rows": []any{}, "page": page, "hasNext": false, "from": from, "to": to})
		return
	}

	rows, hasNext, err := s.store.Transactions(store.TxnFilter{
		PortfolioID: pid,
		Start:       start,
		End:         end,
		AccountID:   q.Get("account"),
		Category:    q.Get("category"),
		Direction:   orDefault(q.Get("dir"), "all"),
		Query:       q.Get("q"),
		Limit:       txnPageSize,
		Offset:      (page - 1) * txnPageSize,
	})
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rules, err := s.store.Rules(pid); err == nil {
		for i := range rows {
			rows[i].Remembered = s.store.RuleMatches(rows[i].Payee, rules)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rows": rows, "page": page, "hasNext": hasNext, "from": from, "to": to,
	})
}

func (s *Server) apiTxnCategory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID       string `json:"id"`
		Category string `json:"category"`
		Remember bool   `json:"remember"`
		Pattern  string `json:"pattern"`
		Payee    string `json:"payee"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.ID == "" {
		apiError(w, http.StatusBadRequest, "transaction id required")
		return
	}
	pid := portfolioID(r)
	if err := s.store.SetTxnCategory(pid, body.ID, body.Category); err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Remember && body.Category != "" {
		pattern := strings.TrimSpace(orDefault(body.Pattern, body.Payee))
		if pattern != "" {
			if err := s.store.AddRule(pid, pattern, body.Category); err != nil {
				apiError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// categories & rules

func (s *Server) apiCategories(w http.ResponseWriter, r *http.Request) {
	pid := portfolioID(r)
	q := r.URL.Query()
	now := time.Now()
	to := orDefault(q.Get("to"), now.Format(dateLayout))
	from := orDefault(q.Get("from"), now.AddDate(0, 0, -30).Format(dateLayout))
	start := parseDate(from, now.AddDate(0, 0, -30))
	end := parseDate(to, now).AddDate(0, 0, 1)

	out := map[string]any{"from": from, "to": to}
	if pid == "" {
		out["spend"], out["income"], out["categories"], out["rules"] = []any{}, []any{}, []any{}, []any{}
		writeJSON(w, http.StatusOK, out)
		return
	}
	spend, err := s.store.SpendByCategory(pid, start, end)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	income, err := s.store.IncomeByCategory(pid, start, end)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cats, err := s.store.Categories(pid)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rules, err := s.store.Rules(pid)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out["spend"], out["income"], out["categories"], out["rules"] = spend, income, cats, rules
	// Chart-ready pie shapes for the radiator (labels/values/colors).
	out["spendPie"], out["incomePie"] = toPie(spend), toPie(income)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiCategoryAdd(w http.ResponseWriter, r *http.Request) {
	var body struct{ Name string }
	if !decode(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		apiError(w, http.StatusBadRequest, "name required")
		return
	}
	if err := s.store.AddCategory(portfolioID(r), name); err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiCategoryDelete(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		apiError(w, http.StatusBadRequest, "name required")
		return
	}
	if err := s.store.DeleteCategory(portfolioID(r), name); err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiCategoryExclude(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		Exclude bool   `json:"exclude"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Name == "" {
		apiError(w, http.StatusBadRequest, "name required")
		return
	}
	if err := s.store.SetCategoryExclude(portfolioID(r), body.Name, body.Exclude); err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiRecategorize(w http.ResponseWriter, r *http.Request) {
	updated, err := s.store.ApplyRules(portfolioID(r), true)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"updated": updated})
}

func (s *Server) apiRuleAdd(w http.ResponseWriter, r *http.Request) {
	var body struct{ Pattern, Category string }
	if !decode(w, r, &body) {
		return
	}
	pattern := strings.TrimSpace(body.Pattern)
	if pattern == "" || body.Category == "" {
		apiError(w, http.StatusBadRequest, "pattern and category required")
		return
	}
	if err := s.store.AddRule(portfolioID(r), pattern, body.Category); err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiRuleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	if err := s.store.DeleteRule(portfolioID(r), id); err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// spending

func (s *Server) apiSpending(w http.ResponseWriter, r *http.Request) {
	pid := portfolioID(r)
	q := r.URL.Query()
	rng := orDefault(q.Get("range"), "last-30-days")
	interval := orDefault(q.Get("interval"), "daily")
	split := q.Get("split") == "1"

	out := map[string]any{
		"range":      rng,
		"rangeLabel": daterange.Label(rng),
		"interval":   interval,
		"split":      split,
	}
	if pid == "" {
		out["series"], out["payees"] = store.Series{}, []any{}
		writeJSON(w, http.StatusOK, out)
		return
	}
	start, end := daterange.Resolve(rng, time.Now())
	series, err := s.store.SpendingSeries(pid, start, end, interval, split)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	payees, err := s.store.TopPayees(pid, start, end, 15)
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out["series"], out["payees"] = series, payees
	writeJSON(w, http.StatusOK, out)
}
