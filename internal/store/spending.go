package store

import "time"

// Series is chart-ready: shared X labels (buckets) + one or more named lines.
// Ranges is parallel to Labels: each bucket's date range, for click-to-filter.
type Series struct {
	Labels []string    `json:"labels"`
	Ranges []Bucket    `json:"ranges"`
	Lines  []SpendLine `json:"lines"`
}

// Bucket is a label's date range, clamped to the chart window, both inclusive.
type Bucket struct {
	From string `json:"from"` // YYYY-MM-DD, inclusive
	To   string `json:"to"`   // YYYY-MM-DD, inclusive
}

type SpendLine struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"` // dollars (cents/100) per bucket
}

const dateLayout = "2006-01-02"

// notInvestmentSQL drops investment-account activity (trades, returns) from the
// spending and cash-flow views. Needs accounts joined as a.
const notInvestmentSQL = "COALESCE(a.type,'') <> 'investment'"

func bucketExpr(interval string) string {
	switch interval {
	case "weekly":
		return "strftime('%Y-W%W', posted, 'unixepoch')"
	case "monthly":
		return "strftime('%Y-%m', posted, 'unixepoch')"
	case "quarterly":
		return "strftime('%Y', posted, 'unixepoch') || '-Q' || ((CAST(strftime('%m', posted, 'unixepoch') AS INT)+2)/3)"
	case "yearly":
		return "strftime('%Y', posted, 'unixepoch')"
	default: // daily
		return "strftime('%Y-%m-%d', posted, 'unixepoch')"
	}
}

// bucketStartExpr is the canonical first calendar day of a bucket, so we never
// reverse-parse the display label (the weekly %W form has no clean inverse).
// Weekly uses Monday-start to match daterange.Resolve's (weekday+6)%7.
// At the year boundary a %W week split across two year-labels maps
// both halves to the same Monday start. Rare; bucketing weekly by the Monday
// date itself would fix it but changes the visible axis label. Out of scope.
func bucketStartExpr(interval string) string {
	switch interval {
	case "weekly":
		return "date(posted,'unixepoch','-' || ((strftime('%w',posted,'unixepoch')+6)%7) || ' days')"
	case "monthly":
		return "strftime('%Y-%m-01', posted,'unixepoch')"
	case "quarterly":
		return "date(posted,'unixepoch','start of month','-' || ((CAST(strftime('%m',posted,'unixepoch') AS INT)-1)%3) || ' months')"
	case "yearly":
		return "strftime('%Y-01-01', posted,'unixepoch')"
	default: // daily
		return "strftime('%Y-%m-%d', posted,'unixepoch')"
	}
}

// bucketTo returns a bucket's inclusive last day (YYYY-MM-DD), given its start
// date string and interval, clamped to the chart's exclusive end (ce).
func bucketTo(startDate, interval, ce string) string {
	sd, _ := time.Parse(dateLayout, startDate)
	var endEx time.Time
	switch interval {
	case "weekly":
		endEx = sd.AddDate(0, 0, 7)
	case "monthly":
		endEx = sd.AddDate(0, 1, 0)
	case "quarterly":
		endEx = sd.AddDate(0, 3, 0)
	case "yearly":
		endEx = sd.AddDate(1, 0, 0)
	default: // daily
		endEx = sd.AddDate(0, 0, 1)
	}
	toEx := endEx.Format(dateLayout)
	if ce < toEx { // YYYY-MM-DD sorts lexically
		toEx = ce
	}
	te, _ := time.Parse(dateLayout, toEx)
	return te.AddDate(0, 0, -1).Format(dateLayout)
}

// SpendingSeries buckets debits (amount_cents<0, negated to positive spend) by
// interval over [start,end), excluding investment accounts. perAccount yields
// one line per account; otherwise a single "Total" line.
func (s *Store) SpendingSeries(portfolioID string, start, end time.Time, interval string, perAccount bool) (Series, error) {
	bucket := bucketExpr(interval)
	// Join categories so excluded categories (e.g. Transfer, Credit Card Payment)
	// drop out of the spending totals, matching the category pie charts.
	from := `transactions t
	      LEFT JOIN accounts a ON a.portfolio_id = t.portfolio_id AND a.id = t.account_id
	      LEFT JOIN categories c ON c.portfolio_id = t.portfolio_id AND c.name = t.category`
	where := `t.portfolio_id = ? AND t.posted >= ? AND t.posted < ? AND t.amount_cents < 0 AND COALESCE(c.exclude,0)=0 AND ` + notInvestmentSQL
	args := []any{portfolioID, start.Unix(), end.Unix()}

	cs, ce := start.Format(dateLayout), end.Format(dateLayout) // chart window [cs, ce)
	lrows, err := s.db.Query(
		`SELECT DISTINCT `+bucket+` AS b, `+bucketStartExpr(interval)+` AS bs
		 FROM `+from+` WHERE `+where+` ORDER BY b`, args...)
	if err != nil {
		return Series{}, err
	}
	var labels []string
	var ranges []Bucket
	for lrows.Next() {
		var b, bs string
		if err := lrows.Scan(&b, &bs); err != nil {
			lrows.Close()
			return Series{}, err
		}
		labels = append(labels, b)
		bf := bs // bucket start clamped to chart window start
		if cs > bf {
			bf = cs
		}
		ranges = append(ranges, Bucket{From: bf, To: bucketTo(bs, interval, ce)})
	}
	lrows.Close()
	if err := lrows.Err(); err != nil {
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
		        COALESCE(NULLIF(a.nickname,''), NULLIF(a.name,''), t.account_id) AS line,
		        -SUM(t.amount_cents) AS spent
		 FROM `+from+`
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
	return Series{Labels: labels, Ranges: ranges, Lines: lines}, rows.Err()
}

// CashFlow is income vs spending per bucket. Lines are [Income, Spending].
type CashFlow struct {
	Series
	Income      float64 `json:"income"` // window totals, dollars
	Spending    float64 `json:"spending"`
	Net         float64 `json:"net"`
	SavingsRate float64 `json:"savingsRate"` // Net/Income; 0 when no income
}

// CashFlow buckets income and spending by interval over [start,end), using the
// same rules as the category totals: excluded categories drop out, and inflows
// on liability accounts (debt paydowns, refunds) don't count as income.
// Investment accounts are left out entirely: trades and returns aren't cash
// flow, and contributions are transfers (categorize them Transfer).
func (s *Store) CashFlow(portfolioID string, start, end time.Time, interval string) (CashFlow, error) {
	rows, err := s.db.Query(
		`SELECT `+bucketExpr(interval)+` AS b, `+bucketStartExpr(interval)+` AS bs,
		        COALESCE(SUM(CASE WHEN t.amount_cents > 0 AND `+liabilityExcludeSQL()+` THEN t.amount_cents END), 0),
		        COALESCE(-SUM(CASE WHEN t.amount_cents < 0 THEN t.amount_cents END), 0)
		 FROM transactions t
		      LEFT JOIN accounts a ON a.portfolio_id = t.portfolio_id AND a.id = t.account_id
		      LEFT JOIN categories c ON c.portfolio_id = t.portfolio_id AND c.name = t.category
		 WHERE t.portfolio_id = ? AND t.posted >= ? AND t.posted < ? AND COALESCE(c.exclude,0)=0
		       AND `+notInvestmentSQL+`
		 GROUP BY b
		 ORDER BY b`, portfolioID, start.Unix(), end.Unix())
	if err != nil {
		return CashFlow{}, err
	}
	defer rows.Close()

	cs, ce := start.Format(dateLayout), end.Format(dateLayout)
	cf := CashFlow{Series: Series{Lines: []SpendLine{{Name: "Income"}, {Name: "Spending"}}}}
	for rows.Next() {
		var b, bs string
		var in, out int64
		if err := rows.Scan(&b, &bs, &in, &out); err != nil {
			return CashFlow{}, err
		}
		if in == 0 && out == 0 { // only liability inflows in this bucket
			continue
		}
		bf := bs
		if cs > bf {
			bf = cs
		}
		cf.Labels = append(cf.Labels, b)
		cf.Ranges = append(cf.Ranges, Bucket{From: bf, To: bucketTo(bs, interval, ce)})
		cf.Lines[0].Values = append(cf.Lines[0].Values, float64(in)/100)
		cf.Lines[1].Values = append(cf.Lines[1].Values, float64(out)/100)
		cf.Income += float64(in) / 100
		cf.Spending += float64(out) / 100
	}
	cf.Net = cf.Income - cf.Spending
	if cf.Income > 0 {
		cf.SavingsRate = cf.Net / cf.Income
	}
	return cf, rows.Err()
}

type PayeeStat struct {
	Payee string
	Count int
	Spent float64 // dollars
}

// TopPayees ranks debit spend by payee over [start,end), highest spend first.
// Investment accounts are left out, matching SpendingSeries.
func (s *Store) TopPayees(portfolioID string, start, end time.Time, limit int) ([]PayeeStat, error) {
	rows, err := s.db.Query(
		`SELECT t.payee, COUNT(*) AS n, -SUM(t.amount_cents) AS spent
		 FROM transactions t
		      LEFT JOIN accounts a ON a.portfolio_id = t.portfolio_id AND a.id = t.account_id
		      LEFT JOIN categories c ON c.portfolio_id = t.portfolio_id AND c.name = t.category
		 WHERE t.portfolio_id = ? AND t.posted >= ? AND t.posted < ? AND t.amount_cents < 0 AND COALESCE(c.exclude,0)=0
		       AND `+notInvestmentSQL+`
		 GROUP BY t.payee
		 ORDER BY spent DESC, n DESC
		 LIMIT ?`, portfolioID, start.Unix(), end.Unix(), limit)
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
