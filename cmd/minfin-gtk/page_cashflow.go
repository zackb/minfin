package main

import (
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/zackb/minfin/internal/daterange"
)

func (a *App) buildCashFlow() gtk.Widgetter {
	if a.pid == "" {
		return emptyState()
	}

	rangeKey := a.cash.rangeKey
	if rangeKey == "" {
		rangeKey = "last-12-months"
	}
	interval := a.cash.interval
	if interval == "" {
		interval = "monthly"
	}
	start, end := daterange.Resolve(rangeKey, a.now())

	cf, err := a.st.CashFlow(a.pid, start, end, interval)
	if err != nil {
		return errorState(err)
	}

	body := vbox(16)
	title := gtk.NewLabel("Cash Flow")
	title.AddCSSClass("title-1")
	title.SetXAlign(0)
	body.Append(title)
	body.Append(a.cashControls(rangeKey, interval))

	body.Append(chartCard("Income vs spending", 280, func(cr *cairo.Context, w, h int, ink chartInk) {
		drawBarChart(cr, w, h, cf.Series, ink)
	}))

	totals := adw.NewPreferencesGroup()
	totals.SetTitle("Totals")
	for _, t := range []struct {
		name string
		v    float64
	}{{"Income", cf.Income}, {"Spending", -cf.Spending}, {"Net", cf.Net}} {
		row := actionRow()
		row.SetTitle(t.name)
		row.AddSuffix(moneySuffix(t.v))
		totals.Add(row)
	}
	rate := actionRow()
	rate.SetTitle("Savings rate")
	rate.AddSuffix(gtk.NewLabel(fmt.Sprintf("%.0f%%", cf.SavingsRate*100)))
	totals.Add(rate)
	body.Append(totals)

	periods := adw.NewPreferencesGroup()
	periods.SetTitle("By period")
	if len(cf.Labels) == 0 {
		row := actionRow()
		row.SetTitle("No activity in this range")
		periods.Add(row)
	}
	for i := len(cf.Labels) - 1; i >= 0; i-- {
		in, out := cf.Lines[0].Values[i], cf.Lines[1].Values[i]
		row := actionRow()
		row.SetTitle(cf.Labels[i])
		row.SetSubtitle(fmt.Sprintf("Income %s · Spending %s", shortUSD(in), shortUSD(out)))
		row.AddSuffix(moneySuffix(in - out))
		periods.Add(row)
	}
	body.Append(periods)

	return pageBody(body)
}

func (a *App) cashControls(rangeKey, interval string) gtk.Widgetter {
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
		a.cash.rangeKey = daterange.Options[i].Key
		a.refreshPage("cashflow")
	}))

	iLabels := make([]string, len(daterange.CashFlowIntervals))
	iSel := 0
	for i, v := range daterange.CashFlowIntervals {
		iLabels[i] = strings.Title(v)
		if v == interval {
			iSel = i
		}
	}
	bar.Append(dropDown(iLabels, iSel, func(i int) {
		a.cash.interval = daterange.CashFlowIntervals[i]
		a.refreshPage("cashflow")
	}))

	return bar
}
