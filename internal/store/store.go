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
  type TEXT NOT NULL DEFAULT '',
  nickname TEXT NOT NULL DEFAULT '',
  asset_value_cents INTEGER NOT NULL DEFAULT 0,
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
  category TEXT NOT NULL DEFAULT '',
  pending INTEGER
);
CREATE INDEX IF NOT EXISTS idx_txn_posted ON transactions(posted);
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT);
CREATE TABLE IF NOT EXISTS categories (
  name TEXT PRIMARY KEY,
  color TEXT NOT NULL DEFAULT '',
  exclude INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS category_rules (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  pattern TEXT NOT NULL,
  category TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rule_pattern ON category_rules(pattern);
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	s := &Store{db}
	if err := s.seedCategories(); err != nil {
		return nil, fmt.Errorf("seed categories: %w", err)
	}
	return s, nil
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
		// Upsert, preserving the user-set type column on existing accounts.
		if _, err := tx.Exec(
			`INSERT INTO accounts(id,org_name,name,currency,balance_cents,balance_date)
			 VALUES(?,?,?,?,?,?)
			 ON CONFLICT(id) DO UPDATE SET org_name=excluded.org_name, name=excluded.name,
			   currency=excluded.currency, balance_cents=excluded.balance_cents,
			   balance_date=excluded.balance_date`,
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
			// Upsert, preserving the user/auto-set category column on existing rows.
			if _, err := tx.Exec(
				`INSERT INTO transactions(id,account_id,posted,amount_cents,payee,description,pending)
				 VALUES(?,?,?,?,?,?,?)
				 ON CONFLICT(id) DO UPDATE SET account_id=excluded.account_id, posted=excluded.posted,
				   amount_cents=excluded.amount_cents, payee=excluded.payee,
				   description=excluded.description, pending=excluded.pending`,
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
