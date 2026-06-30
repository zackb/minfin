package main

import (
	"fmt"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/zackb/minfin/internal/daterange"
	"github.com/zackb/minfin/internal/store"
)

func (a *App) buildTransactions() gtk.Widgetter {
	if a.pid == "" {
		return emptyState()
	}

	rangeKey := a.txn.rangeKey
	if rangeKey == "" {
		rangeKey = "last-30-days"
	}
	start, end := daterange.Resolve(rangeKey, a.now())

	rows, hasNext, err := a.st.Transactions(store.TxnFilter{
		PortfolioID: a.pid, Start: start, End: end,
		AccountID: a.txn.accountID, Category: a.txn.category,
		Direction: a.txn.direction, Query: a.txn.query,
		Limit: 100, Offset: a.txn.page * 100,
	})
	if err != nil {
		return errorState(err)
	}

	cats, _ := a.st.Categories(a.pid)
	rules, _ := a.st.Rules(a.pid)
	accts, _ := a.st.AccountList(a.pid)

	body := vbox(14)
	title := gtk.NewLabel("Transactions")
	title.AddCSSClass("title-1")
	title.SetXAlign(0)
	body.Append(title)
	body.Append(a.txnFilterBar(rangeKey, cats, accts))

	group := adw.NewPreferencesGroup()
	if len(rows) == 0 {
		empty := actionRow()
		empty.SetTitle("No transactions match these filters")
		group.Add(empty)
	}
	for _, r := range rows {
		group.Add(a.txnRow(r, cats, rules))
	}
	body.Append(group)
	body.Append(a.txnPager(hasNext))

	return pageBody(body)
}

func (a *App) txnFilterBar(rangeKey string, cats []store.Category, accts []store.AccountRef) gtk.Widgetter {
	bar := vbox(8)
	controls := hbox(8)

	// Date range.
	rLabels := make([]string, len(daterange.Options))
	rSel := 0
	for i, o := range daterange.Options {
		rLabels[i] = o.Label
		if o.Key == rangeKey {
			rSel = i
		}
	}
	controls.Append(dropDown(rLabels, rSel, func(i int) {
		a.txn.rangeKey = daterange.Options[i].Key
		a.txn.page = 0
		a.refreshPage("transactions")
	}))

	// Account.
	aLabels := []string{"All accounts"}
	aIDs := []string{""}
	aSel := 0
	for i, ac := range accts {
		aLabels = append(aLabels, ac.Name)
		aIDs = append(aIDs, ac.ID)
		if ac.ID == a.txn.accountID {
			aSel = i + 1
		}
	}
	controls.Append(dropDown(aLabels, aSel, func(i int) {
		a.txn.accountID = aIDs[i]
		a.txn.page = 0
		a.refreshPage("transactions")
	}))

	// Category.
	cLabels := []string{"All categories", "Uncategorized"}
	cVals := []string{"", "none"}
	cSel := 0
	for i, c := range cats {
		cLabels = append(cLabels, c.Name)
		cVals = append(cVals, c.Name)
		if c.Name == a.txn.category {
			cSel = i + 2
		}
	}
	if a.txn.category == "none" {
		cSel = 1
	}
	controls.Append(dropDown(cLabels, cSel, func(i int) {
		a.txn.category = cVals[i]
		a.txn.page = 0
		a.refreshPage("transactions")
	}))

	// Direction.
	dLabels := []string{"All", "Money out", "Money in"}
	dVals := []string{"all", "debit", "credit"}
	dSel := 0
	for i, v := range dVals {
		if v == a.txn.direction {
			dSel = i
		}
	}
	controls.Append(dropDown(dLabels, dSel, func(i int) {
		a.txn.direction = dVals[i]
		a.txn.page = 0
		a.refreshPage("transactions")
	}))

	bar.Append(controls)

	// Search (applied on Enter).
	search := gtk.NewEntry()
	search.SetPlaceholderText("Search payee or description — press Enter")
	search.SetText(a.txn.query)
	search.SetHExpand(true)
	search.ConnectActivate(func() {
		a.txn.query = search.Text()
		a.txn.page = 0
		a.refreshPage("transactions")
	})
	bar.Append(search)

	return bar
}

func (a *App) txnRow(r store.TxnRow, cats []store.Category, rules []store.Rule) *adw.ActionRow {
	row := actionRow()
	payee := r.Payee
	if payee == "" {
		payee = "(no payee)"
	}
	row.SetTitle(payee)
	sub := r.Posted.Format("Jan 2, 2006") + " · " + r.Account
	if r.Category != "" {
		sub += " · " + r.Category
	}
	if r.Pending {
		sub += " · pending"
	}
	row.SetSubtitle(sub)
	row.AddSuffix(moneySuffix(r.Amount))

	edit := gtk.NewMenuButton()
	edit.SetIconName("document-edit-symbolic")
	edit.SetVAlign(gtk.AlignCenter)
	edit.AddCSSClass("flat")
	edit.SetPopover(a.categorizePopover(r, cats, rules))
	row.AddSuffix(edit)
	return row
}

// categorizePopover edits one transaction's category, optionally remembering the
// payee as a rule (mirrors the server's categorize form).
func (a *App) categorizePopover(r store.TxnRow, cats []store.Category, rules []store.Rule) *gtk.Popover {
	pop := gtk.NewPopover()
	box := vbox(8)
	box.SetMarginTop(10)
	box.SetMarginBottom(10)
	box.SetMarginStart(10)
	box.SetMarginEnd(10)

	labels := []string{"Uncategorized"}
	vals := []string{""}
	sel := 0
	for i, c := range cats {
		labels = append(labels, c.Name)
		vals = append(vals, c.Name)
		if c.Name == r.Category {
			sel = i + 1
		}
	}
	dd := gtk.NewDropDownFromStrings(labels)
	dd.SetSelected(uint(sel))

	remember := gtk.NewCheckButtonWithLabel("Remember this payee")
	remember.SetActive(a.st.RuleMatches(r.Payee, rules))

	apply := gtk.NewButtonWithLabel("Apply")
	apply.AddCSSClass("suggested-action")
	apply.ConnectClicked(func() {
		cat := vals[int(dd.Selected())]
		if err := a.st.SetTxnCategory(a.pid, r.ID, cat); err != nil {
			a.toast("Couldn't set category: " + err.Error())
			return
		}
		if remember.Active() && cat != "" && r.Payee != "" {
			if err := a.st.AddRule(a.pid, r.Payee, cat); err != nil {
				a.toast("Couldn't save rule: " + err.Error())
			}
		}
		pop.Popdown()
		a.refreshAll() // dashboard/categories/spending reflect the change too
	})

	box.Append(sectionLabel("Category"))
	box.Append(dd)
	box.Append(remember)
	box.Append(apply)
	pop.SetChild(box)
	return pop
}

func (a *App) txnPager(hasNext bool) gtk.Widgetter {
	pager := hbox(8)
	pager.SetHAlign(gtk.AlignCenter)

	prev := gtk.NewButtonFromIconName("go-previous-symbolic")
	prev.SetSensitive(a.txn.page > 0)
	prev.ConnectClicked(func() {
		if a.txn.page > 0 {
			a.txn.page--
			a.refreshPage("transactions")
		}
	})

	lbl := gtk.NewLabel(fmt.Sprintf("Page %d", a.txn.page+1))
	lbl.SetVAlign(gtk.AlignCenter)

	next := gtk.NewButtonFromIconName("go-next-symbolic")
	next.SetSensitive(hasNext)
	next.ConnectClicked(func() {
		if hasNext {
			a.txn.page++
			a.refreshPage("transactions")
		}
	})

	pager.Append(prev)
	pager.Append(lbl)
	pager.Append(next)
	return pager
}
