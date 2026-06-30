package main

import (
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/zackb/minfin/internal/dashboard"
)

// appCSS is the small amount of styling on top of libadwaita's defaults: padded
// stat cards and a bit of breathing room. Everything else uses Adwaita classes.
const appCSS = `
.stat { padding: 14px 16px; }
.stat-title { font-size: 0.85em; }
.swatch { min-width: 14px; min-height: 14px; border-radius: 4px; }
.numeric { font-feature-settings: "tnum"; }
`

func (a *App) loadCSS() {
	p := gtk.NewCSSProvider()
	p.LoadFromString(appCSS)
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(), p, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}

func vbox(spacing int) *gtk.Box { return gtk.NewBox(gtk.OrientationVertical, spacing) }
func hbox(spacing int) *gtk.Box { return gtk.NewBox(gtk.OrientationHorizontal, spacing) }

// pageBody clamps content to a comfortable reading width, pads it, and makes it
// scroll — the standard frame for every page.
func pageBody(content gtk.Widgetter) gtk.Widgetter {
	clamp := adw.NewClamp()
	clamp.SetMaximumSize(900)
	clamp.SetChild(content)
	clamp.SetMarginTop(20)
	clamp.SetMarginBottom(20)
	clamp.SetMarginStart(16)
	clamp.SetMarginEnd(16)
	sc := gtk.NewScrolledWindow()
	sc.SetChild(clamp)
	sc.SetVExpand(true)
	sc.SetHExpand(true)
	return sc
}

// statCard is a titled metric box (Net worth, Assets, …) using Adwaita's .card.
func statCard(title string, v float64) *gtk.Box {
	box := vbox(2)
	box.AddCSSClass("card")
	box.AddCSSClass("stat")
	box.SetHExpand(true)
	t := gtk.NewLabel(title)
	t.AddCSSClass("dim-label")
	t.AddCSSClass("stat-title")
	t.SetXAlign(0)
	val := moneyLabel(v)
	val.AddCSSClass("title-2")
	val.SetXAlign(0)
	box.Append(t)
	box.Append(val)
	return box
}

// moneyLabel renders dollars, tinted by sign, with tabular figures.
func moneyLabel(v float64) *gtk.Label {
	l := gtk.NewLabel(dashboard.USD(v))
	l.AddCSSClass("numeric")
	if v < 0 {
		l.AddCSSClass("error")
	} else if v > 0 {
		l.AddCSSClass("success")
	}
	return l
}

// moneySuffix is a right-aligned money label sized for an ActionRow suffix.
func moneySuffix(v float64) *gtk.Label {
	l := moneyLabel(v)
	l.SetVAlign(gtk.AlignCenter)
	return l
}

// amountLabel is an untinted, right-aligned money label for table columns (the
// web category tables show amounts in a neutral column, not sign-tinted).
func amountLabel(v float64) *gtk.Label {
	l := gtk.NewLabel(dashboard.USD(v))
	l.AddCSSClass("numeric")
	l.SetVAlign(gtk.AlignCenter)
	l.SetXAlign(1)
	return l
}

// actionRow / expanderRow build list rows with Pango markup disabled, so user
// data containing &, <, > (e.g. "Gas & Fuel") renders literally instead of
// failing markup parsing. use-markup covers both the title and the subtitle.
func actionRow() *adw.ActionRow {
	r := adw.NewActionRow()
	r.SetUseMarkup(false)
	return r
}

func expanderRow() *adw.ExpanderRow {
	r := adw.NewExpanderRow()
	r.SetUseMarkup(false)
	return r
}

// dropDown builds a string dropdown with its initial selection applied BEFORE
// the change handler is connected, so restoring state on a page rebuild doesn't
// re-fire onChange (which would loop the refresh).
func dropDown(labels []string, selected int, onChange func(int)) *gtk.DropDown {
	dd := gtk.NewDropDownFromStrings(labels)
	if selected >= 0 && selected < len(labels) {
		dd.SetSelected(uint(selected))
	}
	dd.SetVAlign(gtk.AlignCenter)
	dd.Connect("notify::selected", func() { onChange(int(dd.Selected())) })
	return dd
}

// sectionLabel is a small bold heading above a group.
func sectionLabel(text string) *gtk.Label {
	l := gtk.NewLabel(text)
	l.AddCSSClass("heading")
	l.SetXAlign(0)
	return l
}
