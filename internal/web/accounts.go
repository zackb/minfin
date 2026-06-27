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
		for _, a := range accts {
			v.NetWorth += a.Balance
			if a.Liability {
				v.Liabilities += a.Balance
			} else {
				v.Assets += a.Balance
			}
		}
	}
	s.render(w, "accounts", v)
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
