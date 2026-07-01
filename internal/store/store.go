// Package store is the SQLite persistence layer: schema, sync writes, and the
// queries that back each screen. Amounts are stored as integer cents.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/zackb/minfin/internal/simplefin"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

// schema is the full multi-tenant schema. Data tables carry portfolio_id and use
// composite primary keys: the SimpleFIN ids (accounts.id, transactions.id) are
// only unique within a portfolio, so two portfolios connecting the same bank
// must not collide on the ON CONFLICT upserts.
const schema = `
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_users_email ON users(lower(email));
CREATE TABLE IF NOT EXISTS portfolios (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  access_url TEXT NOT NULL DEFAULT '',
  sync_at INTEGER NOT NULL DEFAULT 0,
  sync_errors TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS portfolio_members (
  portfolio_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'owner',
  PRIMARY KEY (portfolio_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_member_user ON portfolio_members(user_id);
CREATE TABLE IF NOT EXISTS accounts (
  portfolio_id TEXT NOT NULL,
  id TEXT NOT NULL,
  org_name TEXT,
  name TEXT,
  currency TEXT,
  type TEXT NOT NULL DEFAULT '',
  nickname TEXT NOT NULL DEFAULT '',
  asset_value_cents INTEGER NOT NULL DEFAULT 0,
  balance_cents INTEGER,
  balance_date INTEGER,
  PRIMARY KEY (portfolio_id, id)
);
CREATE TABLE IF NOT EXISTS transactions (
  portfolio_id TEXT NOT NULL,
  id TEXT NOT NULL,
  account_id TEXT,
  posted INTEGER,
  amount_cents INTEGER,
  payee TEXT,
  description TEXT,
  category TEXT NOT NULL DEFAULT '',
  pending INTEGER,
  PRIMARY KEY (portfolio_id, id)
);
CREATE INDEX IF NOT EXISTS idx_txn_posted ON transactions(portfolio_id, posted);
CREATE TABLE IF NOT EXISTS categories (
  portfolio_id TEXT NOT NULL,
  name TEXT NOT NULL,
  color TEXT NOT NULL DEFAULT '',
  exclude INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (portfolio_id, name)
);
CREATE TABLE IF NOT EXISTS category_rules (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  portfolio_id TEXT NOT NULL,
  pattern TEXT NOT NULL,
  category TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rule_pattern ON category_rules(portfolio_id, pattern);
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}

	// owner-only
	if !strings.HasPrefix(path, ":memory:") && !strings.Contains(path, "mode=memory") {
		if err := os.Chmod(path, 0o600); err != nil {
			return nil, fmt.Errorf("chmod db: %w", err)
		}
	}
	return &Store{db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// SaveAccountSet upserts a portfolio's accounts + transactions, deduping by
// (portfolio_id, id).
func (s *Store) SaveAccountSet(portfolioID string, set simplefin.AccountSet) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, a := range set.Accounts {
		bal, _ := parseCents(a.Balance)
		// Upsert, preserving the user-set type/nickname/asset_value columns.
		if _, err := tx.Exec(
			`INSERT INTO accounts(portfolio_id,id,org_name,name,currency,balance_cents,balance_date)
			 VALUES(?,?,?,?,?,?,?)
			 ON CONFLICT(portfolio_id,id) DO UPDATE SET org_name=excluded.org_name, name=excluded.name,
			   currency=excluded.currency, balance_cents=excluded.balance_cents,
			   balance_date=excluded.balance_date`,
			portfolioID, a.ID, a.Org.Name, a.Name, a.Currency, bal, a.BalanceDate); err != nil {
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
				`INSERT INTO transactions(portfolio_id,id,account_id,posted,amount_cents,payee,description,pending)
				 VALUES(?,?,?,?,?,?,?,?)
				 ON CONFLICT(portfolio_id,id) DO UPDATE SET account_id=excluded.account_id, posted=excluded.posted,
				   amount_cents=excluded.amount_cents, payee=excluded.payee,
				   description=excluded.description, pending=excluded.pending`,
				portfolioID, t.ID, a.ID, t.Posted, amt, payee, t.Description, t.Pending); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
