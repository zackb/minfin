package web

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zackb/minfin/internal/store"
)

type accountsView struct {
	viewBase
	Accounts    []store.AccountInfo
	Types       []store.AccountType
	Assets      float64
	Liabilities float64
	NetWorth    float64
}

func (s *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	v := accountsView{viewBase: s.base(r, "accounts"), Types: store.AccountTypes}
	if v.Connected {
		accts, err := s.store.Accounts(portfolioID(r), time.Now())
		if err != nil {
			v.Error = err.Error()
		}
		v.Accounts = accts
		v.Assets, v.Liabilities, v.NetWorth = summarize(accts)
	}
	s.render(w, "accounts", v)
}

// summarize adapts store.Summarize to the (assets, liabilities, net) tuple the
// HTML/JSON handlers expect.
func summarize(accts []store.AccountInfo) (assets, liabilities, net float64) {
	s := store.Summarize(accts)
	return s.Assets, s.Liabilities, s.NetWorth
}

func (s *Server) handleAccountType(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	typ := r.FormValue("type")
	if id == "" || !store.ValidType(typ) {
		http.Error(w, "invalid account or type", http.StatusBadRequest)
		return
	}
	if err := s.store.SetAccountType(portfolioID(r), id, typ); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}

func (s *Server) handleAccountAssetValue(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	if id == "" {
		http.Error(w, "invalid account", http.StatusBadRequest)
		return
	}
	// ponytail: the input is only rendered for asset types (mortgage/auto_loan),
	// so the UI gates this; no server-side type lookup. Blank clears to 0.
	dollars, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("value")), 64)
	if r.FormValue("value") != "" && (err != nil || dollars < 0) {
		http.Error(w, "invalid value", http.StatusBadRequest)
		return
	}
	cents := int64(math.Round(dollars * 100))
	if err := s.store.SetAccountAssetValue(portfolioID(r), id, cents); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}

func (s *Server) handleAccountNickname(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	if id == "" {
		http.Error(w, "invalid account", http.StatusBadRequest)
		return
	}
	nick := strings.TrimSpace(r.FormValue("nickname"))
	if err := s.store.SetAccountNickname(portfolioID(r), id, nick); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}
