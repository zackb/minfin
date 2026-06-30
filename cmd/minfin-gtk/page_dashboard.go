package main

import (
	"fmt"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/zackb/minfin/internal/store"
)

func (a *App) buildDashboard() gtk.Widgetter {
	if a.pid == "" {
		return emptyState()
	}

	accts, err := a.st.Accounts(a.pid, a.now())
	if err != nil {
		return errorState(err)
	}
	sum := store.Summarize(accts)

	body := vbox(18)

	title := gtk.NewLabel(portfolioName(a.portfolio()))
	title.AddCSSClass("title-1")
	title.SetXAlign(0)
	body.Append(title)

	cards := hbox(12)
	cards.SetHomogeneous(true)
	cards.Append(statCard("Net worth", sum.NetWorth))
	cards.Append(statCard("Assets", sum.Assets))
	cards.Append(statCard("Liabilities", sum.Liabilities))
	body.Append(cards)

	// 30-day spending visuals.
	start := a.now().AddDate(0, 0, -30)
	end := a.now().AddDate(0, 0, 1)
	if series, err := a.st.SpendingSeries(a.pid, start, end, "daily", false); err == nil {
		body.Append(chartCard("Spending — last 30 days", 200, func(cr *cairo.Context, w, h int, ink chartInk) {
			drawLineChart(cr, w, h, series, ink)
		}))
	}
	if stats, err := a.st.SpendByCategory(a.pid, start, end); err == nil && len(stats) > 0 {
		body.Append(chartCard("By category — last 30 days", 200, func(cr *cairo.Context, w, h int, ink chartInk) {
			drawPieChart(cr, w, h, stats, ink)
		}))
	}

	// Accounts overview.
	ag := adw.NewPreferencesGroup()
	ag.SetTitle("Accounts")
	for _, ac := range accts {
		row := actionRow()
		row.SetTitle(ac.Display())
		sub := ac.Org
		if t := typeLabel(ac.Type); t != "" {
			sub = sub + " · " + t
		}
		row.SetSubtitle(sub)
		row.AddSuffix(moneySuffix(ac.Balance))
		ag.Add(row)
	}
	body.Append(ag)

	// Top payees, last 30 days.
	if payees, err := a.st.TopPayees(a.pid, start, end, 8); err == nil && len(payees) > 0 {
		pg := adw.NewPreferencesGroup()
		pg.SetTitle("Top payees")
		pg.SetDescription("Last 30 days")
		for _, p := range payees {
			row := actionRow()
			row.SetTitle(p.Payee)
			row.SetSubtitle(fmt.Sprintf("%d transactions", p.Count))
			row.AddSuffix(moneySuffix(-p.Spent))
			pg.Add(row)
		}
		body.Append(pg)
	}

	return pageBody(body)
}

// errorState renders a query failure inline so a broken page doesn't blank the
// whole window.
// typeLabel maps an account type key to its human label, "" if uncategorized.
func typeLabel(key string) string {
	for _, t := range store.AccountTypes {
		if t.Key == key {
			return t.Label
		}
	}
	return ""
}

func errorState(err error) gtk.Widgetter {
	sp := adw.NewStatusPage()
	sp.SetIconName("dialog-error-symbolic")
	sp.SetTitle("Something went wrong")
	sp.SetDescription(err.Error())
	return sp
}
