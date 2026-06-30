package main

import (
	"fmt"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/zackb/minfin/internal/daterange"
	"github.com/zackb/minfin/internal/store"
)

func (a *App) buildCategories() gtk.Widgetter {
	if a.pid == "" {
		return emptyState()
	}
	cats, err := a.st.Categories(a.pid)
	if err != nil {
		return errorState(err)
	}
	rules, _ := a.st.Rules(a.pid)

	rangeKey := a.cat.rangeKey
	if rangeKey == "" {
		rangeKey = "last-30-days"
	}
	start, end := daterange.Resolve(rangeKey, a.now())

	body := vbox(18)
	title := gtk.NewLabel("Categories")
	title.AddCSSClass("title-1")
	title.SetXAlign(0)
	body.Append(title)

	body.Append(a.catRangeBar(rangeKey))

	spend, _ := a.st.SpendByCategory(a.pid, start, end)
	body.Append(a.categoryStatSection("Spending by Category", spend, "No spending in this range"))
	income, _ := a.st.IncomeByCategory(a.pid, start, end)
	body.Append(a.categoryStatSection("Income by Category", income, "No income in this range"))

	body.Append(a.categoriesGroup(cats))
	body.Append(a.rulesGroup(cats, rules))

	recat := hbox(0)
	recat.SetHAlign(gtk.AlignStart)
	recat.Append(a.recategorizeButton())
	body.Append(recat)

	return pageBody(body)
}

// catRangeBar is the date-range dropdown feeding the spending/income sections.
func (a *App) catRangeBar(rangeKey string) gtk.Widgetter {
	bar := hbox(8)
	bar.SetHAlign(gtk.AlignStart)
	labels := make([]string, len(daterange.Options))
	sel := 0
	for i, o := range daterange.Options {
		labels[i] = o.Label
		if o.Key == rangeKey {
			sel = i
		}
	}
	bar.Append(dropDown(labels, sel, func(i int) {
		a.cat.rangeKey = daterange.Options[i].Key
		a.refreshPage("categories")
	}))
	return bar
}

// categoryStatSection is a category pie plus a table of the same data
// (swatch · category · txns · amount), mirroring the web Categories page.
// Clicking a slice, legend entry, or table row filters Transactions by category.
func (a *App) categoryStatSection(title string, stats []store.CategoryStat, emptyMsg string) gtk.Widgetter {
	box := vbox(8)
	box.Append(a.pieCard(title, 260, stats))

	grp := adw.NewPreferencesGroup()
	if len(stats) == 0 {
		row := actionRow()
		row.SetTitle(emptyMsg)
		grp.Add(row)
		box.Append(grp)
		return box
	}
	for i, s := range stats {
		// Swatch color must match the pie slice: hex if set, else the palette by
		// index (same fallback drawPieChart's sliceColor uses).
		hex := s.Color
		if hex == "" {
			hex = chartPalette[i%len(chartPalette)]
		}
		row := actionRow()
		row.AddPrefix(colorSwatch(hex))
		row.SetTitle(s.Category)
		row.SetSubtitle(fmt.Sprintf("%d transactions", s.Count))
		row.SetActivatable(true)
		row.ConnectActivated(func() { a.showCategoryTxns(s.Category) })
		row.AddSuffix(amountLabel(s.Amount))
		grp.Add(row)
	}
	box.Append(grp)
	return box
}

// pieCard is a chart card whose slices/legend rows navigate to the Transactions
// page filtered to the clicked category (like the web app).
func (a *App) pieCard(title string, height int, stats []store.CategoryStat) gtk.Widgetter {
	da := chartArea(height, func(cr *cairo.Context, w, h int, ink chartInk) {
		drawPieChart(cr, w, h, stats, ink)
	})
	click := gtk.NewGestureClick()
	click.ConnectReleased(func(_ int, x, y float64) {
		if cat := pieSliceAt(x, y, da.Width(), da.Height(), stats); cat != "" {
			a.showCategoryTxns(cat)
		}
	})
	da.AddController(click)

	box := vbox(8)
	box.AddCSSClass("card")
	box.AddCSSClass("stat")
	box.Append(sectionLabel(title))
	box.Append(da)
	return box
}

func (a *App) categoriesGroup(cats []store.Category) gtk.Widgetter {
	grp := adw.NewPreferencesGroup()
	grp.SetTitle("Categories")
	grp.SetDescription("The switch excludes a category from spend/income totals (e.g. Transfer).")
	grp.SetHeaderSuffix(a.addCategoryButton())

	for _, c := range cats {
		row := actionRow()
		row.AddPrefix(colorSwatch(c.Color))
		row.SetTitle(c.Name)
		row.SetActivatable(true)
		row.SetTooltipText("View transactions in this category")
		row.ConnectActivated(func() { a.showCategoryTxns(c.Name) })

		sw := gtk.NewSwitch()
		sw.SetActive(c.Exclude)
		sw.SetVAlign(gtk.AlignCenter)
		sw.SetTooltipText("Exclude from totals")
		name := c.Name
		sw.Connect("notify::active", func() {
			if err := a.st.SetCategoryExclude(a.pid, name, sw.Active()); err != nil {
				a.toast("Couldn't update: " + err.Error())
				return
			}
			a.refreshAll()
		})
		row.AddSuffix(sw)

		del := gtk.NewButtonFromIconName("user-trash-symbolic")
		del.AddCSSClass("flat")
		del.SetVAlign(gtk.AlignCenter)
		del.SetTooltipText("Delete category")
		del.ConnectClicked(func() {
			a.confirmDelete("Delete \""+name+"\"?",
				"It will be removed from any transactions and rules using it.",
				func() {
					if err := a.st.DeleteCategory(a.pid, name); err != nil {
						a.toast("Couldn't delete: " + err.Error())
						return
					}
					a.refreshAll()
				})
		})
		row.AddSuffix(del)

		grp.Add(row)
	}
	return grp
}

func (a *App) addCategoryButton() *gtk.MenuButton {
	mb := gtk.NewMenuButton()
	mb.SetIconName("list-add-symbolic")
	mb.AddCSSClass("flat")
	mb.SetTooltipText("Add category")

	pop := gtk.NewPopover()
	box := popoverBox()
	entry := gtk.NewEntry()
	entry.SetPlaceholderText("New category")
	add := gtk.NewButtonWithLabel("Add")
	add.AddCSSClass("suggested-action")
	commit := func() {
		name := entry.Text()
		if name == "" {
			return
		}
		if err := a.st.AddCategory(a.pid, name); err != nil {
			a.toast("Couldn't add: " + err.Error())
			return
		}
		entry.SetText("")
		pop.Popdown()
		a.refreshAll()
	}
	entry.ConnectActivate(commit)
	add.ConnectClicked(commit)
	box.Append(entry)
	box.Append(add)
	pop.SetChild(box)
	mb.SetPopover(pop)
	return mb
}

func (a *App) rulesGroup(cats []store.Category, rules []store.Rule) gtk.Widgetter {
	grp := adw.NewPreferencesGroup()
	grp.SetTitle("Rules")
	grp.SetDescription("Payees matching a pattern are auto-categorized on sync.")
	grp.SetHeaderSuffix(a.addRuleButton(cats))

	if len(rules) == 0 {
		row := actionRow()
		row.SetTitle("No rules yet")
		grp.Add(row)
	}
	for _, r := range rules {
		row := actionRow()
		row.SetTitle(r.Pattern)
		row.SetSubtitle("→ " + r.Category)
		id := r.ID
		del := gtk.NewButtonFromIconName("user-trash-symbolic")
		del.AddCSSClass("flat")
		del.SetVAlign(gtk.AlignCenter)
		del.SetTooltipText("Delete rule")
		del.ConnectClicked(func() {
			if err := a.st.DeleteRule(a.pid, id); err != nil {
				a.toast("Couldn't delete rule: " + err.Error())
				return
			}
			a.refreshAll()
		})
		row.AddSuffix(del)
		grp.Add(row)
	}
	return grp
}

func (a *App) addRuleButton(cats []store.Category) *gtk.MenuButton {
	mb := gtk.NewMenuButton()
	mb.SetIconName("list-add-symbolic")
	mb.AddCSSClass("flat")
	mb.SetTooltipText("Add rule")

	pop := gtk.NewPopover()
	box := popoverBox()

	pattern := gtk.NewEntry()
	pattern.SetPlaceholderText("Payee contains…")

	labels := make([]string, len(cats))
	for i, c := range cats {
		labels[i] = c.Name
	}
	dd := gtk.NewDropDownFromStrings(labels)

	add := gtk.NewButtonWithLabel("Add rule")
	add.AddCSSClass("suggested-action")
	add.ConnectClicked(func() {
		i := int(dd.Selected())
		if pattern.Text() == "" || i < 0 || i >= len(cats) {
			return
		}
		if err := a.st.AddRule(a.pid, pattern.Text(), cats[i].Name); err != nil {
			a.toast("Couldn't add rule: " + err.Error())
			return
		}
		pattern.SetText("")
		pop.Popdown()
		a.refreshAll()
	})

	box.Append(sectionLabel("Payee pattern"))
	box.Append(pattern)
	box.Append(sectionLabel("Category"))
	box.Append(dd)
	box.Append(add)
	pop.SetChild(box)
	mb.SetPopover(pop)
	return mb
}

func (a *App) recategorizeButton() *gtk.Button {
	b := gtk.NewButtonWithLabel("Recategorize past transactions")
	b.ConnectClicked(func() {
		n, err := a.st.ApplyRules(a.pid, true)
		if err != nil {
			a.toast("Recategorize failed: " + err.Error())
			return
		}
		a.toast(fmt.Sprintf("Recategorized %d transactions", n))
		a.refreshAll()
	})
	return b
}
