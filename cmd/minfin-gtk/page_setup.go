package main

import (
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func (a *App) buildSetup() gtk.Widgetter {
	body := vbox(18)
	title := gtk.NewLabel("Setup")
	title.AddCSSClass("title-1")
	title.SetXAlign(0)
	body.Append(title)

	if a.pid != "" {
		body.Append(a.connectionGroup())
	}
	body.Append(a.addConnectionGroup())

	return pageBody(body)
}

// connectionGroup shows the active portfolio's sync status and a quick re-sync.
func (a *App) connectionGroup() gtk.Widgetter {
	p := a.portfolio()
	grp := adw.NewPreferencesGroup()
	grp.SetTitle("Connection")

	row := actionRow()
	row.SetTitle(portfolioName(p))
	switch {
	case p.AccessURL == "":
		row.SetSubtitle("No SimpleFIN token — paste one below")
	case p.Sync.At.IsZero():
		row.SetSubtitle("Connected, not synced yet")
	default:
		row.SetSubtitle("Last sync " + p.Sync.At.Format("Jan 2, 2006 3:04 PM"))
	}
	if p.AccessURL != "" {
		b := gtk.NewButtonWithLabel("Sync now")
		b.AddCSSClass("flat")
		b.SetVAlign(gtk.AlignCenter)
		b.ConnectClicked(func() { a.doSync() })
		row.AddSuffix(b)
	}
	grp.Add(row)

	if len(p.Sync.Errors) > 0 {
		er := actionRow()
		er.SetTitle("Last sync reported errors")
		er.SetSubtitle(strings.Join(p.Sync.Errors, "; "))
		grp.Add(er)
	}
	return grp
}

// addConnectionGroup is the SimpleFIN onboarding form.
func (a *App) addConnectionGroup() gtk.Widgetter {
	grp := adw.NewPreferencesGroup()
	grp.SetTitle("Connect accounts")
	grp.SetDescription("Paste a SimpleFIN setup token to import your accounts and transactions.")

	token := adw.NewEntryRow()
	token.SetTitle("SimpleFIN setup token")
	grp.Add(token)

	connect := gtk.NewButtonWithLabel("Connect")
	connect.AddCSSClass("suggested-action")
	connect.SetVAlign(gtk.AlignCenter)
	connect.SetHAlign(gtk.AlignStart)
	connect.SetMarginTop(8)
	connect.ConnectClicked(func() { a.connectToken(token.Text()) })

	box := vbox(0)
	box.Append(grp)
	box.Append(connect)
	return box
}
