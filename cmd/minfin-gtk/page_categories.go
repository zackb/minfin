package main

import (
	"fmt"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

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

	body := vbox(18)
	title := gtk.NewLabel("Categories")
	title.AddCSSClass("title-1")
	title.SetXAlign(0)
	body.Append(title)

	body.Append(a.categoriesGroup(cats))
	body.Append(a.rulesGroup(cats, rules))

	recat := hbox(0)
	recat.SetHAlign(gtk.AlignStart)
	recat.Append(a.recategorizeButton())
	body.Append(recat)

	return pageBody(body)
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
