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
const LookbackDays = 89

// Sync pulls accounts+transactions from SimpleFIN into the store and records the
// reported errors so the UI can surface connection problems.
func Sync(s *store.Store, accessURL string) error {
	set, err := simplefin.Fetch(accessURL, LookbackDays)
	if err != nil {
		return err
	}
	for _, e := range set.Errors {
		log.Printf("simplefin: %s", e)
	}
	if err := s.SetSyncStatus(store.SyncStatus{At: time.Now(), Errors: set.Errors}); err != nil {
		log.Printf("save sync status: %v", err)
	}
	return s.SaveAccountSet(set)
}

// Loop resyncs on an interval until the process exits.
func Loop(s *store.Store, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for range tick.C {
		url, err := s.AccessURL()
		if err != nil || url == "" {
			continue
		}
		if err := Sync(s, url); err != nil {
			log.Printf("sync: %v", err)
		} else {
			log.Printf("sync: ok")
		}
	}
}
