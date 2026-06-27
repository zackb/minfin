package main

import (
	"log"
	"time"
)

// syncLookbackDays is how far back each sync pulls. Dedup by txn id makes a wide
// window idempotent.
// ponytail: fixed 2y lookback per sync; make incremental (track last posted) if
// the bridge caps response size.
const syncLookbackDays = 730

// Sync pulls accounts+transactions from SimpleFIN into the DB.
func Sync(db *DB, accessURL string) error {
	set, err := Fetch(accessURL, syncLookbackDays)
	if err != nil {
		return err
	}
	return db.SaveAccountSet(set)
}

// syncLoop runs an initial sync then resyncs on an interval until the process exits.
func syncLoop(db *DB, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for range tick.C {
		access := readAccess()
		if access == "" {
			continue
		}
		if err := Sync(db, access); err != nil {
			log.Printf("sync: %v", err)
		} else {
			log.Printf("sync: ok")
		}
	}
}
