package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ sql *sql.DB }

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
`

func Open(path string) (*DB, error) {
	s, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := s.Exec(schema); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return &DB{s}, nil
}

// parseCents turns a SimpleFIN decimal string ("-12.34", "100", "5.6") into
// integer cents (-1234, 10000, 560). Money path: integer math, no float drift.
func parseCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	neg := false
	switch s[0] {
	case '-':
		neg, s = true, s[1:]
	case '+':
		s = s[1:]
	}
	whole, frac, _ := strings.Cut(s, ".")
	frac = (frac + "00")[:2] // pad/truncate to 2 places
	var cents int64
	if _, err := fmt.Sscanf(whole+frac, "%d", &cents); err != nil {
		return 0, fmt.Errorf("parse amount %q: %w", s, err)
	}
	if neg {
		cents = -cents
	}
	return cents, nil
}

func (db *DB) SaveAccountSet(set AccountSet) error {
	tx, err := db.sql.Begin()
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

// Series is chart-ready: shared X labels (buckets) + one or more named lines.
type Series struct {
	Labels []string    `json:"labels"`
	Lines  []SpendLine `json:"lines"`
}

type SpendLine struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"` // dollars (cents/100) per bucket
}

func bucketExpr(interval string) string {
	switch interval {
	case "weekly":
		return "strftime('%Y-W%W', posted, 'unixepoch')"
	case "monthly":
		return "strftime('%Y-%m', posted, 'unixepoch')"
	default: // daily
		return "strftime('%Y-%m-%d', posted, 'unixepoch')"
	}
}

// SpendingSeries buckets debits (amount_cents<0, negated to positive spend) by
// interval over [start,end). perAccount yields one line per account; otherwise a
// single "Total" line. ponytail: assumes one currency; group/convert if accounts mix.
func (db *DB) SpendingSeries(start, end time.Time, interval string, perAccount bool) (Series, error) {
	bucket := bucketExpr(interval)
	where := `posted >= ? AND posted < ? AND amount_cents < 0`
	args := []any{start.Unix(), end.Unix()}

	// Build the ordered set of bucket labels present in range.
	labels, err := db.queryStrings(
		`SELECT DISTINCT `+bucket+` AS b FROM transactions WHERE `+where+` ORDER BY b`, args...)
	if err != nil {
		return Series{}, err
	}
	idx := make(map[string]int, len(labels))
	for i, l := range labels {
		idx[l] = i
	}

	groupCols := bucket
	if perAccount {
		groupCols += ", account_id"
	}
	rows, err := db.sql.Query(
		`SELECT `+bucket+` AS b,
		        COALESCE(NULLIF(a.name,''), t.account_id) AS line,
		        -SUM(t.amount_cents) AS spent
		 FROM transactions t LEFT JOIN accounts a ON a.id = t.account_id
		 WHERE `+where+`
		 GROUP BY `+groupCols+`
		 ORDER BY b`, args...)
	if err != nil {
		return Series{}, err
	}
	defer rows.Close()

	lineIdx := map[string]int{}
	var lines []SpendLine
	getLine := func(name string) int {
		if i, ok := lineIdx[name]; ok {
			return i
		}
		lineIdx[name] = len(lines)
		lines = append(lines, SpendLine{Name: name, Values: make([]float64, len(labels))})
		return lineIdx[name]
	}
	for rows.Next() {
		var b, line string
		var cents int64
		if err := rows.Scan(&b, &line, &cents); err != nil {
			return Series{}, err
		}
		name := "Total"
		if perAccount {
			name = line
		}
		lines[getLine(name)].Values[idx[b]] = float64(cents) / 100
	}
	if lines == nil {
		lines = []SpendLine{{Name: "Total", Values: make([]float64, len(labels))}}
	}
	return Series{Labels: labels, Lines: lines}, rows.Err()
}

type PayeeStat struct {
	Payee string
	Count int
	Spent float64 // dollars
}

func (db *DB) TopPayees(start, end time.Time, limit int) ([]PayeeStat, error) {
	rows, err := db.sql.Query(
		`SELECT payee, COUNT(*) AS n, -SUM(amount_cents) AS spent
		 FROM transactions
		 WHERE posted >= ? AND posted < ? AND amount_cents < 0
		 GROUP BY payee
		 ORDER BY n DESC, spent DESC
		 LIMIT ?`, start.Unix(), end.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PayeeStat
	for rows.Next() {
		var p PayeeStat
		var cents int64
		if err := rows.Scan(&p.Payee, &p.Count, &cents); err != nil {
			return nil, err
		}
		p.Spent = float64(cents) / 100
		out = append(out, p)
	}
	return out, rows.Err()
}

func (db *DB) queryStrings(q string, args ...any) ([]string, error) {
	rows, err := db.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
