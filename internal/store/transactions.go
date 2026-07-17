package store

import (
	"strings"
	"time"
)

// TxnFilter selects and pages transactions. Zero Start/End is treated as
// unbounded on that side by the caller (the web layer defaults to last 30 days).
type TxnFilter struct {
	PortfolioID string    // required: scopes the query to one portfolio
	Start, End  time.Time // [Start, End)
	AccountIDs  []string  // empty = all accounts, else OR-matched (IN)
	Categories  []string  // empty = all; OR-matched. Values are category names plus
	// the sentinels "none" (uncategorized) and "budget" (any in-budget category, i.e.
	// exclude=0, which also covers uncategorized rows — matching the app's budget math).
	Direction string // "all" | "debit" | "credit"
	Query     string // substring match on payee/description
	Limit     int    // page size (default 100)
	Offset    int
}

type TxnRow struct {
	ID          string
	Posted      time.Time
	Account     string
	Payee       string
	Description string
	Category    string
	Amount      float64 // signed dollars (negative = debit)
	Pending     bool
	Remembered  bool // a saved rule already categorizes this payee (set by the web layer)
}

// Transactions returns rows matching the filter (newest first) plus hasNext,
// computed by fetching one extra row instead of a separate COUNT.
func (s *Store) Transactions(f TxnFilter) (rows []TxnRow, hasNext bool, err error) {
	where := []string{"t.portfolio_id = ?", "t.posted >= ?", "t.posted < ?"}
	args := []any{f.PortfolioID, f.Start.Unix(), f.End.Unix()}

	if ids := nonEmpty(f.AccountIDs); len(ids) > 0 {
		where = append(where, "t.account_id IN ("+placeholders(len(ids))+")")
		for _, id := range ids {
			args = append(args, id)
		}
	}
	// Categories OR together: real names collapse into one IN clause; "none" matches
	// uncategorized; "budget" matches any non-excluded category (needs the join below).
	needCatJoin := false
	if cats := nonEmpty(f.Categories); len(cats) > 0 {
		var clauses, names []string
		for _, c := range cats {
			switch c {
			case "none":
				clauses = append(clauses, "COALESCE(t.category,'') = ''")
			case "budget":
				needCatJoin = true
				clauses = append(clauses, "COALESCE(c.exclude,0) = 0")
			default:
				names = append(names, c)
			}
		}
		if len(names) > 0 {
			clauses = append(clauses, "t.category IN ("+placeholders(len(names))+")")
			for _, n := range names {
				args = append(args, n)
			}
		}
		where = append(where, "("+strings.Join(clauses, " OR ")+")")
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

	from := "transactions t LEFT JOIN accounts a ON a.portfolio_id = t.portfolio_id AND a.id = t.account_id"
	if needCatJoin {
		from += " LEFT JOIN categories c ON c.portfolio_id = t.portfolio_id AND c.name = t.category"
	}
	q := `SELECT t.id, t.posted, COALESCE(NULLIF(a.nickname,''), NULLIF(a.name,''), t.account_id),
	             t.payee, t.description, t.category, t.amount_cents, t.pending
	      FROM ` + from + `
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
		if err := res.Scan(&r.ID, &posted, &r.Account, &r.Payee, &r.Description, &r.Category, &cents, &r.Pending); err != nil {
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

// placeholders returns "?,?,?" for n bind params.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// nonEmpty drops blank entries (e.g. a stray `?category=` from an empty select).
func nonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
