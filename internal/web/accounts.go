package web

import (
	"net/http"
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
// already arrive negative, so net worth is just their sum.
func summarize(accts []store.AccountInfo) (assets, liabilities, net float64) {
	for _, a := range accts {
		net += a.Balance
		if a.Liability {
			liabilities += a.Balance
		} else {
			assets += a.Balance
		}
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
