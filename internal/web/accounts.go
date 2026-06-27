package web

import (
	"net/http"
	"time"

	"github.com/zackb/minfin/internal/store"
)

type accountsView struct {
	viewBase
	Accounts []store.AccountInfo
	NetWorth float64
}

func (s *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	v := accountsView{viewBase: s.base("accounts")}
	if v.Connected {
		accts, err := s.store.Accounts(time.Now())
		if err != nil {
			v.Error = err.Error()
		}
		v.Accounts = accts
		for _, a := range accts {
			v.NetWorth += a.Balance
		}
	}
	s.render(w, "accounts", v)
}
