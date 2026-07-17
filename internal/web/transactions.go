package web

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/zackb/minfin/internal/store"
)

const txnPageSize = 100
const dateLayout = "2006-01-02"

type transactionsView struct {
	viewBase
	From          string // yyyy-mm-dd, for the date inputs
	To            string
	SelAccounts   []string // selected account ids (multi)
	SelCategories []string // selected category values (names + "none"/"budget" sentinels)
	Direction     string
	Query         string
	Accounts      []store.AccountRef
	Categories    []store.Category
	Rows          []store.TxnRow
	Page          int
	PrevURL       string // "" if no previous page
	NextURL       string // "" if no next page
}

func (s *Server) handleTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pid := portfolioID(r)
	v := transactionsView{
		viewBase:      s.base(w, r, "transactions"),
		SelAccounts:   q["account"],
		SelCategories: q["category"],
		Direction:     orDefault(q.Get("dir"), "all"),
		Query:         q.Get("q"),
	}
	if !v.Connected {
		s.render(w, "transactions", v)
		return
	}

	// Default to the last 30 days when the date inputs are empty.
	now := time.Now()
	v.To = orDefault(q.Get("to"), now.Format(dateLayout))
	v.From = orDefault(q.Get("from"), now.AddDate(0, 0, -30).Format(dateLayout))
	start := parseDate(v.From, now.AddDate(0, 0, -30))
	end := parseDate(v.To, now).AddDate(0, 0, 1) // inclusive of the "to" day

	page := 1
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 1 {
		page = p
	}
	v.Page = page

	accts, err := s.store.AccountList(pid)
	if err != nil {
		v.failed("transactions: account list", err)
	}
	v.Accounts = accts
	if cats, err := s.store.Categories(pid); err != nil {
		v.failed("transactions: categories", err)
	} else {
		v.Categories = cats
	}

	rows, hasNext, err := s.store.Transactions(store.TxnFilter{
		PortfolioID: pid,
		Start:       start,
		End:         end,
		AccountIDs:  v.SelAccounts,
		Categories:  v.SelCategories,
		Direction:   v.Direction,
		Query:       v.Query,
		Limit:       txnPageSize,
		Offset:      (page - 1) * txnPageSize,
	})
	if err != nil {
		v.failed("transactions: list", err)
	}
	// Mark rows whose payee is already covered by a rule, so the "remember"
	// checkbox reflects real state instead of resetting on every reload.
	if rules, err := s.store.Rules(pid); err == nil {
		for i := range rows {
			rows[i].Remembered = s.store.RuleMatches(rows[i].Payee, rules)
		}
	}
	v.Rows = rows

	// Build pagination links that preserve the active filters.
	params := url.Values{}
	setIf(params, "from", v.From)
	setIf(params, "to", v.To)
	for _, a := range v.SelAccounts {
		params.Add("account", a)
	}
	for _, c := range v.SelCategories {
		params.Add("category", c)
	}
	if v.Direction != "all" {
		params.Set("dir", v.Direction)
	}
	setIf(params, "q", v.Query)
	if page > 1 {
		v.PrevURL = pageURL(params, page-1)
	}
	if hasNext {
		v.NextURL = pageURL(params, page+1)
	}

	s.render(w, "transactions", v)
}

func parseDate(s string, fallback time.Time) time.Time {
	if t, err := time.Parse(dateLayout, s); err == nil {
		return t
	}
	return fallback
}

func setIf(v url.Values, key, val string) {
	if val != "" {
		v.Set(key, val)
	}
}

func pageURL(base url.Values, page int) string {
	v := url.Values{}
	for k, vs := range base {
		v[k] = vs
	}
	v.Set("page", strconv.Itoa(page))
	return "/transactions?" + v.Encode()
}
