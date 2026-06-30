package main

import (
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/zackb/minfin/internal/store"
)

// App holds the whole desktop app: the open store, the currently selected
// portfolio, and the pages. Pages re-query the store in their refresh() and
// rebuild their widgets, so a portfolio switch, a mutation, or a sync just calls
// refreshAll.
type App struct {
	st  *store.Store
	app *adw.Application
	win *adw.ApplicationWindow

	toasts *adw.ToastOverlay
	stack  *adw.ViewStack

	portfolios []store.Portfolio
	pid        string // selected portfolio id, "" when there are none

	pages   []*pageView
	pageMap map[string]*pageView

	txn   txnState   // transactions page filters, persisted across refreshes
	spend spendState // spending page controls, persisted across refreshes
}

// spendState is the spending page's range/interval/split controls.
type spendState struct {
	rangeKey   string // daterange preset; "" => last 30 days
	interval   string // "daily" | "weekly" | "monthly"; "" => daily
	perAccount bool   // one line per account vs a single Total
}

// txnState is the transactions page's current filter + page. It lives on App so
// it survives the page rebuilding itself on each refresh.
type txnState struct {
	rangeKey  string // daterange preset; "" => last 30 days
	accountID string // "" => all accounts
	category  string // "" => all, "none" => uncategorized, else exact name
	direction string // "all" | "debit" | "credit"
	query     string
	page      int
}

// pageView is one tab. root is an adw.Bin we swap the content of on refresh.
type pageView struct {
	root    *adw.Bin
	refresh func()
}

func newPage(build func() gtk.Widgetter) *pageView {
	bin := adw.NewBin()
	return &pageView{root: bin, refresh: func() { bin.SetChild(build()) }}
}

func (a *App) now() time.Time { return time.Now() }

func (a *App) activate() {
	a.loadCSS()

	a.win = adw.NewApplicationWindow(&a.app.Application)
	a.win.SetTitle("minfin")
	a.win.SetDefaultSize(960, 700)

	a.loadPortfolios()

	header := adw.NewHeaderBar()

	switcher := adw.NewViewSwitcher()
	switcher.SetPolicy(adw.ViewSwitcherPolicyWide)
	a.stack = adw.NewViewStack()
	switcher.SetStack(a.stack)
	header.SetTitleWidget(switcher)

	if combo := a.portfolioSwitcher(); combo != nil {
		header.PackStart(combo)
	}
	header.PackEnd(a.syncButton())

	a.addPages()

	a.toasts = adw.NewToastOverlay()
	a.toasts.SetChild(a.stack)

	toolbar := adw.NewToolbarView()
	toolbar.AddTopBar(header)
	toolbar.SetContent(a.toasts)

	a.win.SetContent(toolbar)
	a.refreshAll()
	if a.pid == "" {
		a.stack.SetVisibleChildName("setup") // fresh DB → land on onboarding
	}
	a.win.SetVisible(true)
}

// loadPortfolios reads every portfolio in the file (membership is server-only)
// and selects one: the only one, or none (onboarding) when the file is fresh.
func (a *App) loadPortfolios() {
	ps, err := a.st.Portfolios()
	if err != nil {
		a.toast("Error loading portfolios: " + err.Error())
		return
	}
	a.portfolios = ps
	if a.pid == "" && len(ps) > 0 {
		a.pid = ps[0].ID
	}
}

func (a *App) portfolio() store.Portfolio {
	for _, p := range a.portfolios {
		if p.ID == a.pid {
			return p
		}
	}
	return store.Portfolio{}
}

// portfolioSwitcher is a header dropdown shown only when more than one portfolio
// exists; switching it swaps the active portfolio and refreshes every page.
func (a *App) portfolioSwitcher() *gtk.DropDown {
	if len(a.portfolios) < 2 {
		return nil
	}
	names := make([]string, len(a.portfolios))
	for i, p := range a.portfolios {
		names[i] = portfolioName(p)
	}
	dd := gtk.NewDropDownFromStrings(names)
	dd.SetVAlign(gtk.AlignCenter)
	dd.Connect("notify::selected", func() {
		i := int(dd.Selected())
		if i >= 0 && i < len(a.portfolios) {
			a.pid = a.portfolios[i].ID
			a.refreshAll()
		}
	})
	return dd
}

func portfolioName(p store.Portfolio) string {
	if p.Name != "" {
		return p.Name
	}
	return "Portfolio"
}

func (a *App) addPages() {
	a.pageMap = map[string]*pageView{}
	add := func(name, title, icon string, build func() gtk.Widgetter) {
		pv := newPage(build)
		a.pages = append(a.pages, pv)
		a.pageMap[name] = pv
		a.stack.AddTitledWithIcon(pv.root, name, title, icon)
	}
	add("dashboard", "Dashboard", "go-home-symbolic", a.buildDashboard)
	add("accounts", "Accounts", "view-list-symbolic", a.buildAccounts)
	add("transactions", "Transactions", "view-list-bullet-symbolic", a.buildTransactions)
	add("categories", "Categories", "view-grid-symbolic", a.buildCategories)
	add("spending", "Spending", "org.gnome.Settings-time-symbolic", a.buildSpending)
	add("setup", "Setup", "emblem-system-symbolic", a.buildSetup)
}

func (a *App) refreshAll() {
	for _, p := range a.pages {
		p.refresh()
	}
}

// refreshPage rebuilds a single page, used when a filter changes only that page.
func (a *App) refreshPage(name string) {
	if p := a.pageMap[name]; p != nil {
		p.refresh()
	}
}

// showCategoryTxns jumps to the Transactions page filtered to one category,
// clearing other filters (mirrors clicking a category in the web app).
func (a *App) showCategoryTxns(category string) {
	a.txn = txnState{category: category}
	a.refreshPage("transactions")
	a.stack.SetVisibleChildName("transactions")
}

func (a *App) toast(msg string) {
	if a.toasts != nil {
		a.toasts.AddToast(adw.NewToast(msg))
	}
}

// confirmDelete shows a destructive-action confirmation before running onConfirm.
func (a *App) confirmDelete(heading, body string, onConfirm func()) {
	d := adw.NewAlertDialog(heading, body)
	d.AddResponse("cancel", "Cancel")
	d.AddResponse("delete", "Delete")
	d.SetResponseAppearance("delete", adw.ResponseDestructive)
	d.SetDefaultResponse("cancel")
	d.SetCloseResponse("cancel")
	d.ConnectResponse(func(response string) {
		if response == "delete" {
			onConfirm()
		}
	})
	d.Present(a.win)
}

// emptyState is the placeholder shown on every page when no portfolio exists yet.
func emptyState() gtk.Widgetter {
	sp := adw.NewStatusPage()
	sp.SetIconName("folder-symbolic")
	sp.SetTitle("No portfolio yet")
	sp.SetDescription("Add one on the Setup tab, or sync with the server first.")
	return sp
}
