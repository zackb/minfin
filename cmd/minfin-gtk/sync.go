package main

import (
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/zackb/minfin/internal/simplefin"
	"github.com/zackb/minfin/internal/syncer"
)

// syncButton is the header refresh action. It pulls fresh data for the active
// portfolio; mirrors the server's /sync, minus auth.
func (a *App) syncButton() *gtk.Button {
	b := gtk.NewButtonFromIconName("view-refresh-symbolic")
	b.SetTooltipText("Sync now")
	b.ConnectClicked(func() { a.doSync() })
	return b
}

// doSync fetches in a goroutine so the UI stays responsive, then hops back to
// the GTK main loop to refresh. The shared store is safe for this concurrency —
// it's the same pattern the server runs (background syncer + request reads).
func (a *App) doSync() {
	p := a.portfolio()
	if p.ID == "" || p.AccessURL == "" {
		a.toast("Connect an account on the Setup tab first")
		return
	}
	a.toast("Syncing…")
	go func() {
		err := syncer.Sync(a.st, p.ID, p.AccessURL)
		glib.IdleAdd(func() {
			if err != nil {
				a.toast("Sync failed: " + err.Error())
			} else {
				a.toast("Sync complete")
			}
			a.reload()
		})
	}()
}

// connectToken claims a SimpleFIN setup token, creates a portfolio for it, and
// runs the first sync — all off the UI thread.
func (a *App) connectToken(token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		a.toast("Paste a SimpleFIN setup token")
		return
	}
	a.toast("Connecting…")
	go func() {
		accessURL, err := simplefin.Claim(token)
		var pid string
		if err == nil {
			pid, err = a.st.CreatePortfolio("", accessURL)
		}
		if err == nil {
			err = syncer.Sync(a.st, pid, accessURL)
		}
		glib.IdleAdd(func() {
			if err != nil {
				a.toast("Connect failed: " + err.Error())
				return
			}
			a.pid = pid
			a.toast("Connected")
			a.reload()
		})
	}()
}

// reload re-reads portfolios (sync status, new portfolios) and rebuilds pages.
func (a *App) reload() {
	a.loadPortfolios()
	a.refreshAll()
}
