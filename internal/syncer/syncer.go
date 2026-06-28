// Package syncer pulls SimpleFIN data into the store, on demand and on a timer.
package syncer

import (
	"log"
	"time"

	"github.com/zackb/minfin/internal/simplefin"
	"github.com/zackb/minfin/internal/store"
)

// LookbackDays is how far back each sync pulls. The SimpleFIN Bridge hard-caps a
// request at 90 days (and recommends 45), so 89 maximizes history without
// tripping the "was capped" error. Deeper history still accumulates across
// repeated syncs since we dedup by txn id and never delete.
//
// The wide window is deliberate, not lazy: re-pulling ~89 days every sync makes
// the feed self-healing — any missed-sync gap or backdated/late-posting
// transaction within ~89 days is recovered on the next sync. SimpleFIN's
// "exceeds recommended range of 45 days" warning is expected and intentionally
// ignored; the per-sync cost (upsert-by-id of ~90 days of txns) is cheap.
//
// Do NOT shrink this to silence the warning or trim load: a smaller window
// reintroduces gap risk AND truncates a newly connected account's initial
// backfill (banks backfill up to ~90 days, often hours after first connect).
// If multi-user sync load is ever *measured* to be a problem, stagger syncs and
// move to Postgres first; only then adopt the adaptive window plan
// (gap-since-last-sync + margin, with a first_seen settling guard).
const LookbackDays = 89

// Sync pulls one portfolio's accounts+transactions from SimpleFIN into the store
// and records the reported errors so the UI can surface connection problems.
func Sync(s *store.Store, portfolioID, accessURL string) error {
	set, err := simplefin.Fetch(accessURL, LookbackDays)
	if err != nil {
		return err
	}
	for _, e := range set.Errors {
		log.Printf("simplefin: %s", e)
	}
	if err := s.SetPortfolioSyncStatus(portfolioID, store.SyncStatus{At: time.Now(), Errors: set.Errors}); err != nil {
		log.Printf("save sync status: %v", err)
	}
	if err := s.SaveAccountSet(portfolioID, set); err != nil {
		return err
	}
	// Auto-categorize newly synced transactions from the saved rules. Fill-only
	// so a sync never clobbers a manual category.
	if n, err := s.ApplyRules(portfolioID, false); err != nil {
		log.Printf("apply rules: %v", err)
	} else if n > 0 {
		log.Printf("categorized %d transactions", n)
	}
	return nil
}

// SyncAll syncs every portfolio that has a token, serially. (Stagger/parallelize
// later if multi-user sync load is ever measured to be a problem.)
func SyncAll(s *store.Store) {
	ps, err := s.Portfolios()
	if err != nil {
		log.Printf("list portfolios: %v", err)
		return
	}
	for _, p := range ps {
		if p.AccessURL == "" {
			continue
		}
		if err := Sync(s, p.ID, p.AccessURL); err != nil {
			log.Printf("sync %s: %v", p.ID, err)
		}
	}
}

// Loop resyncs all portfolios on an interval until the process exits.
func Loop(s *store.Store, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for range tick.C {
		SyncAll(s)
	}
}
