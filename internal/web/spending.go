package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/zackb/minfin/internal/daterange"
	"github.com/zackb/minfin/internal/store"
)

type spendingView struct {
	viewBase
	Range           string
	RangeLabel      string
	Interval        string
	Split           bool
	RangeOptions    []daterange.Option
	IntervalOptions []string
	ChartJSON       template.JS
	Payees          []store.PayeeStat
}

func (s *Server) handleSpending(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	v := spendingView{
		viewBase:        s.base("spending"),
		Range:           orDefault(q.Get("range"), "last-30-days"),
		Interval:        orDefault(q.Get("interval"), "daily"),
		Split:           q.Get("split") == "1",
		RangeOptions:    daterange.Options,
		IntervalOptions: daterange.Intervals,
	}
	v.RangeLabel = daterange.Label(v.Range)

	if !v.Connected {
		s.render(w, "spending", v)
		return
	}

	start, end := daterange.Resolve(v.Range, time.Now())
	series, err := s.store.SpendingSeries(start, end, v.Interval, v.Split)
	if err != nil {
		v.Error = err.Error()
		s.render(w, "spending", v)
		return
	}
	j, _ := json.Marshal(series)
	v.ChartJSON = template.JS(j)

	if v.Payees, err = s.store.TopPayees(start, end, 15); err != nil {
		v.Error = err.Error()
	}
	s.render(w, "spending", v)
}
