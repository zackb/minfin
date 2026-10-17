package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/zackb/minfin/internal/daterange"
	"github.com/zackb/minfin/internal/store"
)

type cashFlowRow struct {
	Label, From, To       string
	Income, Spending, Net float64
}

type cashFlowView struct {
	viewBase
	Range           string
	Interval        string
	RangeOptions    []daterange.Option
	IntervalOptions []string
	ChartJSON       template.JS
	CashFlow        store.CashFlow
	SavingsPct      string
	Rows            []cashFlowRow // newest first
}

func (s *Server) handleCashFlow(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	v := cashFlowView{
		viewBase:        s.base(w, r, "cashflow"),
		Range:           orDefault(q.Get("range"), "last-12-months"),
		Interval:        orDefault(q.Get("interval"), "monthly"),
		RangeOptions:    daterange.Options,
		IntervalOptions: daterange.CashFlowIntervals,
	}
	if !v.Connected {
		s.render(w, "cashflow", v)
		return
	}

	start, end := daterange.Resolve(v.Range, time.Now())
	cf, err := s.store.CashFlow(portfolioID(r), start, end, v.Interval)
	if err != nil {
		v.failed("cashflow: series", err)
		s.render(w, "cashflow", v)
		return
	}
	v.CashFlow = cf
	v.SavingsPct = fmt.Sprintf("%.0f%%", cf.SavingsRate*100)
	j, _ := json.Marshal(cf.Series)
	v.ChartJSON = template.JS(j)
	for i := len(cf.Labels) - 1; i >= 0; i-- {
		in, out := cf.Lines[0].Values[i], cf.Lines[1].Values[i]
		v.Rows = append(v.Rows, cashFlowRow{
			Label: cf.Labels[i], From: cf.Ranges[i].From, To: cf.Ranges[i].To,
			Income: in, Spending: out, Net: in - out,
		})
	}
	s.render(w, "cashflow", v)
}
