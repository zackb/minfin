// Package store is the SQLite persistence layer: schema, sync writes, and the
// queries that back each screen. Amounts are stored as integer cents.
package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/zackb/minfin/internal/simplefin"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS accounts (
  id TEXT PRIMARY KEY,
  org_name TEXT,
  name TEXT,
  currency TEXT,
  balance_cents INTEGER,
  balance_date INTEGER
);
CREATE TABLE IF NOT EXISTS transactions (
  id TEXT PRIMARY KEY,
  account_id TEXT,
  posted INTEGER,
  amount_cents INTEGER,
  payee TEXT,
  description TEXT,
  pending INTEGER
);
CREATE INDEX IF NOT EXISTS idx_txn_posted ON transactions(posted);
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT);
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return &Store{db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// AccessURL returns the stored SimpleFIN access URL, or "" if not connected.
func (s *Store) AccessURL() (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'access_url'`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) SetAccessURL(url string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO meta(key, value) VALUES('access_url', ?)`, url)
	return err
}

// SaveAccountSet upserts accounts + transactions, deduping by id.
func (s *Store) SaveAccountSet(set simplefin.AccountSet) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, a := range set.Accounts {
		bal, _ := parseCents(a.Balance)
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO accounts(id,org_name,name,currency,balance_cents,balance_date)
			 VALUES(?,?,?,?,?,?)`,
			a.ID, a.Org.Name, a.Name, a.Currency, bal, a.BalanceDate); err != nil {
			return err
		}
		for _, t := range a.Transactions {
			amt, err := parseCents(t.Amount)
			if err != nil {
				return err
			}
			payee := t.Payee
			if payee == "" {
				payee = t.Description
			}
			if _, err := tx.Exec(
				`INSERT OR REPLACE INTO transactions(id,account_id,posted,amount_cents,payee,description,pending)
				 VALUES(?,?,?,?,?,?,?)`,
				t.ID, a.ID, t.Posted, amt, payee, t.Description, t.Pending); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// queryStrings runs a single-column string query.
func (s *Store) queryStrings(q string, args ...any) ([]string, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
