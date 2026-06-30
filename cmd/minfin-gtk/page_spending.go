package main

import (
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/zackb/minfin/internal/daterange"
)

func (a *App) buildSpending() gtk.Widgetter {
	if a.pid == "" {
		return emptyState()
	}

	rangeKey := a.spend.rangeKey
	if rangeKey == "" {
		rangeKey = "last-30-days"
	}
	interval := a.spend.interval
	if interval == "" {
		interval = "daily"
	}
	start, end := daterange.Resolve(rangeKey, a.now())

	series, err := a.st.SpendingSeries(a.pid, start, end, interval, a.spend.perAccount)
	if err != nil {
		return errorState(err)
	}
	payees, _ := a.st.TopPayees(a.pid, start, end, 15)

	body := vbox(16)
	title := gtk.NewLabel("Spending")
	title.AddCSSClass("title-1")
	title.SetXAlign(0)
	body.Append(title)
	body.Append(a.spendControls(rangeKey, interval))

	body.Append(chartCard("Spending over time", 280, func(cr *cairo.Context, w, h int, ink chartInk) {
		drawLineChart(cr, w, h, series, ink)
	}))

	pg := adw.NewPreferencesGroup()
	pg.SetTitle("Top payees")
	if len(payees) == 0 {
		row := actionRow()
		row.SetTitle("No spending in this range")
		pg.Add(row)
	}
	for _, p := range payees {
		row := actionRow()
		row.SetTitle(p.Payee)
		row.SetSubtitle(fmt.Sprintf("%d transactions", p.Count))
		row.AddSuffix(moneySuffix(-p.Spent))
		pg.Add(row)
	}
	body.Append(pg)

	return pageBody(body)
}

func (a *App) spendControls(rangeKey, interval string) gtk.Widgetter {
	bar := hbox(8)

	rLabels := make([]string, len(daterange.Options))
	rSel := 0
	for i, o := range daterange.Options {
		rLabels[i] = o.Label
		if o.Key == rangeKey {
			rSel = i
		}
	}
	bar.Append(dropDown(rLabels, rSel, func(i int) {
		a.spend.rangeKey = daterange.Options[i].Key
		a.refreshPage("spending")
	}))

	iLabels := make([]string, len(daterange.Intervals))
	iSel := 0
	for i, v := range daterange.Intervals {
		iLabels[i] = strings.Title(v)
		if v == interval {
			iSel = i
		}
	}
	bar.Append(dropDown(iLabels, iSel, func(i int) {
		a.spend.interval = daterange.Intervals[i]
		a.refreshPage("spending")
	}))

	split := gtk.NewCheckButtonWithLabel("Per account")
	split.SetActive(a.spend.perAccount)
	split.SetVAlign(gtk.AlignCenter)
	split.ConnectToggled(func() {
		a.spend.perAccount = split.Active()
		a.refreshPage("spending")
	})
	bar.Append(split)

	return bar
}
