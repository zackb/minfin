package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/zackb/minfin/internal/daterange"
	"github.com/zackb/minfin/internal/store"
)

type homeView struct {
	viewBase
	Accounts    []store.AccountInfo
	Assets      float64
	Liabilities float64
	NetWorth    float64
	ChartJSON   template.JS
	Payees      []store.PayeeStat
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	v := homeView{viewBase: s.base(r, "dashboard")}
	if !v.Connected {
		s.render(w, "home", v)
		return
	}

	pid := portfolioID(r)
	now := time.Now()
	accts, err := s.store.Accounts(pid, now)
	if err != nil {
		v.Error = err.Error()
	}
	v.Accounts = accts
	v.Assets, v.Liabilities, v.NetWorth = summarize(accts)

	start, end := daterange.Resolve("last-30-days", now)
	if series, err := s.store.SpendingSeries(pid, start, end, "daily", false); err == nil {
		j, _ := json.Marshal(series)
		v.ChartJSON = template.JS(j)
	} else {
		v.Error = err.Error()
	}
	if v.Payees, err = s.store.TopPayees(pid, start, end, 8); err != nil {
		v.Error = err.Error()
	}

	s.render(w, "home", v)
}
