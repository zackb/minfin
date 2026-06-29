// Command minfin-gtk is a gotk4 (GTK4) dashboard that reads the SQLite file
// directly — no server. Scaffold: a read-only net-worth + accounts window.
//
// Build needs GTK4 dev libraries (pkg-config gtk4). See `make gtk`.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/zackb/minfin/internal/dashboard"
	"github.com/zackb/minfin/internal/store"
)

func main() {
	st, err := store.Open(getenv("MINFIN_DB", "minfin.db"))
	if err != nil {
		log.Fatal(err)
	}
	d, err := dashboard.Load(st, time.Now())
	st.Close() // TODO: for now just load once, then we're done with the DB
	if err != nil {
		log.Fatal(err)
	}

	app := gtk.NewApplication("com.zackb.minfin", gio.ApplicationFlagsNone)
	app.ConnectActivate(func() { activate(app, d) })
	if code := app.Run(os.Args); code > 0 {
		os.Exit(code)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func activate(app *gtk.Application, d dashboard.Dashboard) {
	win := gtk.NewApplicationWindow(app)
	win.SetTitle("minfin")
	win.SetDefaultSize(440, 600)

	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.SetMarginTop(16)
	box.SetMarginBottom(16)
	box.SetMarginStart(16)
	box.SetMarginEnd(16)

	if d.Empty() {
		box.Append(gtk.NewLabel("No portfolio yet — sync one with the server first."))
		win.SetChild(box)
		win.SetVisible(true)
		return
	}

	title := gtk.NewLabel("")
	title.SetMarkup(`<span size="x-large" weight="bold">minfin · ` + d.Portfolio.Name + `</span>`)
	title.SetXAlign(0)
	box.Append(title)

	box.Append(summaryRow("Net worth", d.NetWorth))
	box.Append(summaryRow("Assets", d.Assets))
	box.Append(summaryRow("Liabilities", d.Liabilities))
	box.Append(gtk.NewSeparator(gtk.OrientationHorizontal))

	list := gtk.NewListBox()
	for _, a := range d.Accounts {
		list.Append(accountRow(a.Display(), a.Balance))
	}
	scroll := gtk.NewScrolledWindow()
	scroll.SetChild(list)
	scroll.SetVExpand(true)
	box.Append(scroll)

	win.SetChild(box)
	win.SetVisible(true)
}

func summaryRow(label string, amount float64) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 8)
	l := gtk.NewLabel(label)
	l.SetXAlign(0)
	l.SetHExpand(true)
	v := gtk.NewLabel("")
	v.SetMarkup(fmt.Sprintf(`<span weight="bold">%s</span>`, dashboard.USD(amount)))
	v.SetXAlign(1)
	row.Append(l)
	row.Append(v)
	return row
}

func accountRow(name string, balance float64) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 8)
	row.SetMarginTop(4)
	row.SetMarginBottom(4)
	l := gtk.NewLabel(name)
	l.SetXAlign(0)
	l.SetHExpand(true)
	v := gtk.NewLabel(dashboard.USD(balance))
	v.SetXAlign(1)
	row.Append(l)
	row.Append(v)
	return row
}
