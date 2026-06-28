package store

import "time"

// SyncStatus is the outcome of a portfolio's most recent sync: when it ran and
// any errors SimpleFIN reported (e.g. an institution connection needing
// attention). It is persisted on the portfolios row (see portfolios.go).
type SyncStatus struct {
	At     time.Time `json:"at"`
	Errors []string  `json:"errors"`
}
