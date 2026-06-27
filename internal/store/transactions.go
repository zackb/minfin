package store

import (
	"strings"
	"time"
)

// TxnFilter selects and pages transactions. Zero Start/End is treated as
// unbounded on that side by the caller (the web layer defaults to last 30 days).
type TxnFilter struct {
	Start, End time.Time // [Start, End)
	AccountID  string    // "" = all accounts
	Direction  string    // "all" | "debit" | "credit"
	Query      string    // substring match on payee/description
	Limit      int       // page size (default 100)
	Offset     int
}

type TxnRow struct {
	Posted      time.Time
	Account     string
	Payee       string
	Description string
	Amount      float64 // signed dollars (negative = debit)
	Pending     bool
}

// Transactions returns rows matching the filter (newest first) plus hasNext,
// computed by fetching one extra row instead of a separate COUNT.
func (s *Store) Transactions(f TxnFilter) (rows []TxnRow, hasNext bool, err error) {
	where := []string{"t.posted >= ?", "t.posted < ?"}
	args := []any{f.Start.Unix(), f.End.Unix()}

	if f.AccountID != "" {
		where = append(where, "t.account_id = ?")
		args = append(args, f.AccountID)
	}
	switch f.Direction {
	case "debit":
		where = append(where, "t.amount_cents < 0")
	case "credit":
		where = append(where, "t.amount_cents > 0")
	}
	if f.Query != "" {
		where = append(where, "(t.payee LIKE ? OR t.description LIKE ?)")
		like := "%" + f.Query + "%"
		args = append(args, like, like)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit+1, f.Offset) // +1 to detect a next page

	q := `SELECT t.posted, COALESCE(NULLIF(a.name,''), t.account_id),
	             t.payee, t.description, t.amount_cents, t.pending
	      FROM transactions t LEFT JOIN accounts a ON a.id = t.account_id
	      WHERE ` + strings.Join(where, " AND ") + `
	      ORDER BY t.posted DESC
	      LIMIT ? OFFSET ?`

	res, err := s.db.Query(q, args...)
	if err != nil {
		return nil, false, err
	}
	defer res.Close()

	for res.Next() {
		var r TxnRow
		var posted, cents int64
		if err := res.Scan(&posted, &r.Account, &r.Payee, &r.Description, &cents, &r.Pending); err != nil {
			return nil, false, err
		}
		r.Posted = time.Unix(posted, 0)
		r.Amount = float64(cents) / 100
		rows = append(rows, r)
	}
	if err := res.Err(); err != nil {
		return nil, false, err
	}
	if len(rows) > limit {
		return rows[:limit], true, nil
	}
	return rows, false, nil
}
