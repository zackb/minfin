package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// SyncStatus is the outcome of the most recent sync: when it ran and any
// errors SimpleFIN reported (e.g. an institution connection needing attention).
type SyncStatus struct {
	At     time.Time `json:"at"`
	Errors []string  `json:"errors"`
}

func (s *Store) SetSyncStatus(st SyncStatus) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO meta(key, value) VALUES('sync_status', ?)`, string(b))
	return err
}

func (s *Store) SyncStatus() (SyncStatus, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'sync_status'`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return SyncStatus{}, nil
	}
	if err != nil {
		return SyncStatus{}, err
	}
	var st SyncStatus
	return st, json.Unmarshal([]byte(v), &st)
}
