package main

import (
	"fmt"
	"time"

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

	// Spending line chart, last 30 days.
	start := a.now().AddDate(0, 0, -30)
	end := a.now().AddDate(0, 0, 1)
	if series, err := a.st.SpendingSeries(a.pid, start, end, "daily", false); err == nil {
		body.Append(a.spendingCard(series))
	}

	// Accounts and Top Vendors, side by side (the web dashboard's two-column grid).
	columns := hbox(12)
	columns.SetHomogeneous(true)
	columns.Append(a.accountsColumn(accts))
	columns.Append(a.vendorsColumn(start, end))
	body.Append(columns)

	return pageBody(body)
}

// spendingCard is the 30-day spending line chart with a "view all →" link to the
// full Spending page in its header (matches the web dashboard).
func (a *App) spendingCard(series store.Series) gtk.Widgetter {
	box := vbox(8)
	box.AddCSSClass("card")
	box.AddCSSClass("stat")

	header := hbox(8)
	lbl := sectionLabel("Spending — last 30 days")
	lbl.SetHExpand(true)
	header.Append(lbl)
	link := gtk.NewButtonWithLabel("view all →")
	link.AddCSSClass("flat")
	link.AddCSSClass("dim-label")
	link.SetVAlign(gtk.AlignCenter)
	link.ConnectClicked(func() { a.stack.SetVisibleChildName("spending") })
	header.Append(link)
	box.Append(header)

	box.Append(chartArea(200, func(cr *cairo.Context, w, h int, ink chartInk) {
		drawLineChart(cr, w, h, series, ink)
	}))
	return box
}

func (a *App) accountsColumn(accts []store.AccountInfo) gtk.Widgetter {
	ag := adw.NewPreferencesGroup()
	ag.SetTitle("Accounts")
	if len(accts) == 0 {
		row := actionRow()
		row.SetTitle("No accounts synced yet")
		ag.Add(row)
	}
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
	return ag
}

func (a *App) vendorsColumn(start, end time.Time) gtk.Widgetter {
	pg := adw.NewPreferencesGroup()
	pg.SetTitle("Top Vendors · 30 Days")
	payees, err := a.st.TopPayees(a.pid, start, end, 8)
	if err != nil || len(payees) == 0 {
		row := actionRow()
		row.SetTitle("No spending in the last 30 days")
		pg.Add(row)
		return pg
	}
	for _, p := range payees {
		row := actionRow()
		row.SetTitle(p.Payee)
		row.SetSubtitle(fmt.Sprintf("%d transactions", p.Count))
		row.AddSuffix(moneySuffix(-p.Spent))
		pg.Add(row)
	}
	return pg
}

// typeLabel maps an account type key to its human label, "" if uncategorized.
func typeLabel(key string) string {
	for _, t := range store.AccountTypes {
		if t.Key == key {
			return t.Label
		}
	}
	return ""
}

// errorState renders a query failure inline so a broken page doesn't blank the
// whole window.
func errorState(err error) gtk.Widgetter {
	sp := adw.NewStatusPage()
	sp.SetIconName("dialog-error-symbolic")
	sp.SetTitle("Something went wrong")
	sp.SetDescription(err.Error())
	return sp
}
