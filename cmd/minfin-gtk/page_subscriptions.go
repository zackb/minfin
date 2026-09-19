package main

import (
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/zackb/minfin/internal/store"
)

func (a *App) buildSubscriptions() gtk.Widgetter {
	if a.pid == "" {
		return emptyState()
	}
	subs, err := a.st.Subscriptions(a.pid, a.now())
	if err != nil {
		return errorState(err)
	}

	var active, lapsed []store.Subscription
	var pie []store.CategoryStat
	monthly := 0.0
	for _, s := range subs {
		if !s.Active {
			lapsed = append(lapsed, s)
			continue
		}
		active = append(active, s)
		monthly += s.Monthly
		pie = append(pie, store.CategoryStat{Category: s.Payee, Amount: s.Monthly})
	}

	body := vbox(16)
	title := gtk.NewLabel("Subscriptions")
	title.AddCSSClass("title-1")
	title.SetXAlign(0)
	body.Append(title)

	cards := hbox(12)
	cards.SetHomogeneous(true)
	cards.Append(statCard("Per month", -monthly))
	cards.Append(statCard("Per year", -monthly*12))
	body.Append(cards)

	body.Append(pieCard("Monthly cost share", 240, pie, a.showPayeeTxns))

	body.Append(a.subscriptionGroup("Active", "No recurring charges found in the last 13 months", active))
	if len(lapsed) > 0 {
		body.Append(a.subscriptionGroup("Lapsed", "", lapsed))
	}
	return pageBody(body)
}

func (a *App) subscriptionGroup(title, empty string, subs []store.Subscription) *adw.PreferencesGroup {
	pg := adw.NewPreferencesGroup()
	pg.SetTitle(title)
	if len(subs) == 0 {
		row := actionRow()
		row.SetTitle(empty)
		pg.Add(row)
	}
	for _, s := range subs {
		row := actionRow()
		row.SetTitle(s.Payee)
		row.SetSubtitle(fmt.Sprintf("%s · %d charges · last %s · next %s",
			strings.Title(s.Cadence), s.Count, s.Last.Format("Jan 2"), s.Next.Format("Jan 2")))
		row.SetActivatable(true)
		row.SetTooltipText("View transactions from this vendor")
		row.ConnectActivated(func() { a.showPayeeTxns(s.Payee) })
		row.AddSuffix(moneySuffix(-s.Amount))
		pg.Add(row)
	}
	return pg
}
