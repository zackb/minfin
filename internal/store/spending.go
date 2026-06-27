package store

import "time"

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
// single "Total" line.
// ponytail: assumes one currency; group/convert if accounts mix.
func (s *Store) SpendingSeries(start, end time.Time, interval string, perAccount bool) (Series, error) {
	bucket := bucketExpr(interval)
	where := `posted >= ? AND posted < ? AND amount_cents < 0`
	args := []any{start.Unix(), end.Unix()}

	labels, err := s.queryStrings(
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
	rows, err := s.db.Query(
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

// TopPayees ranks debit spend by payee over [start,end), highest spend first.
func (s *Store) TopPayees(start, end time.Time, limit int) ([]PayeeStat, error) {
	rows, err := s.db.Query(
		`SELECT payee, COUNT(*) AS n, -SUM(amount_cents) AS spent
		 FROM transactions
		 WHERE posted >= ? AND posted < ? AND amount_cents < 0
		 GROUP BY payee
		 ORDER BY spent DESC, n DESC
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
