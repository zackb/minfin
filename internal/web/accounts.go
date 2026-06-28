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
	v := accountsView{viewBase: s.base("accounts"), Types: store.AccountTypes}
	if v.Connected {
		accts, err := s.store.Accounts(time.Now())
		if err != nil {
			v.Error = err.Error()
		}
		v.Accounts = accts
		v.Assets, v.Liabilities, v.NetWorth = summarize(accts)
	}
	s.render(w, "accounts", v)
}

// summarize splits accounts into asset/liability totals. Liability balances
// already arrive negative, so net worth is just their sum. An account's
// underlying asset value (house, car) counts toward assets and net worth on
// top of its loan balance.
func summarize(accts []store.AccountInfo) (assets, liabilities, net float64) {
	for _, a := range accts {
		net += a.Balance + a.AssetValue
		if a.Liability {
			liabilities += a.Balance
		} else {
			assets += a.Balance
		}
		assets += a.AssetValue // 0 for non-asset accounts
	}
	return assets, liabilities, net
}

func (s *Server) handleAccountType(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	typ := r.FormValue("type")
	if id == "" || !store.ValidType(typ) {
		http.Error(w, "invalid account or type", http.StatusBadRequest)
		return
	}
	if err := s.store.SetAccountType(id, typ); err != nil {
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
	if err := s.store.SetAccountAssetValue(id, cents); err != nil {
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
	if err := s.store.SetAccountNickname(id, nick); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}
