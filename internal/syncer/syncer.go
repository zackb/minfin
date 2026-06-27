// Package syncer pulls SimpleFIN data into the store, on demand and on a timer.
package syncer

import (
	"log"
	"time"

	"github.com/zackb/minfin/internal/simplefin"
	"github.com/zackb/minfin/internal/store"
)

// LookbackDays is how far back each sync pulls. Dedup by txn id makes a wide
// window idempotent.
// ponytail: fixed 2y lookback per sync; make incremental (track last posted) if
// the bridge caps response size.
const LookbackDays = 730

// Sync pulls accounts+transactions from SimpleFIN into the store.
func Sync(s *store.Store, accessURL string) error {
	set, err := simplefin.Fetch(accessURL, LookbackDays)
	if err != nil {
		return err
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
