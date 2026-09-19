package web

import (
	"html/template"
	"net/http"
	"time"

	"github.com/zackb/minfin/internal/store"
)

type subscriptionsView struct {
	viewBase
	Current, Lapsed []store.Subscription
	Monthly, Yearly float64 // active subscriptions only
	PieJSON         template.JS
	From            string // pie click-through window start, a year back
}

func (s *Server) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	v := subscriptionsView{viewBase: s.base(w, r, "subscriptions")}
	if !v.Connected {
		s.render(w, "subscriptions", v)
		return
	}
	now := time.Now()
	v.From = now.AddDate(-1, 0, 0).Format(dateLayout)
	subs, err := s.store.Subscriptions(portfolioID(r), now)
	if err != nil {
		v.failed("subscriptions", err)
		s.render(w, "subscriptions", v)
		return
	}
	var pie []store.CategoryStat
	for _, sub := range subs {
		if !sub.Active {
			v.Lapsed = append(v.Lapsed, sub)
			continue
		}
		v.Current = append(v.Current, sub)
		v.Monthly += sub.Monthly
		pie = append(pie, store.CategoryStat{Category: sub.Payee, Amount: sub.Monthly})
	}
	v.Yearly = v.Monthly * 12
	v.PieJSON = marshalPie(pie)
	s.render(w, "subscriptions", v)
}
