// Package dashboard is the view-model shared by the local thick clients (TUI,
// GTK). It composes store reads into the data one screen needs, so each client
// renders the same numbers without re-implementing the queries or the
// net-worth math. The server has its own per-request, multi-portfolio path in
// internal/web and does not use this.
package dashboard

import (
	"fmt"
	"math"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/zackb/minfin/internal/store"
)

// Dashboard is one portfolio's snapshot: its accounts and the rollup.
type Dashboard struct {
	Portfolio store.Portfolio
	Accounts  []store.AccountInfo
	store.Summary
}

// Empty reports that there was no portfolio to load (fresh DB, never synced).
func (d Dashboard) Empty() bool { return d.Portfolio.ID == "" }

// Load reads the first portfolio's dashboard straight from the file — no
// server, no auth.
//
// first portfolio only. Add selection when the desktop apps actually
// need to juggle more than one.
func Load(st *store.Store, now time.Time) (Dashboard, error) {
	ps, err := st.Portfolios()
	if err != nil {
		return Dashboard{}, err
	}
	if len(ps) == 0 {
		return Dashboard{}, nil
	}
	p := ps[0]
	accts, err := st.Accounts(p.ID, now)
	if err != nil {
		return Dashboard{}, err
	}
	return Dashboard{Portfolio: p, Accounts: accts, Summary: store.Summarize(accts)}, nil
}

// USD formats dollars for display, with thousands separators and the sign
// outside the symbol (-$1,234.50, not $-1,234.50).
func USD(f float64) string {
	sign := ""
	if f < 0 {
		sign, f = "-", -f
	}
	cents := int64(math.Round(f * 100))
	return fmt.Sprintf("%s$%s.%02d", sign, humanize.Comma(cents/100), cents%100)
}
