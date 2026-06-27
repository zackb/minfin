package store

import "time"

type AccountInfo struct {
	Org      string
	Name     string
	Currency string
	Balance  float64 // dollars
	TxnCount int
	LastTxn  string  // YYYY-MM-DD, "" if none
	Spent30  float64 // debit spend in last 30 days, dollars
}

// Accounts lists every connected account with balance and activity stats.
func (s *Store) Accounts(now time.Time) ([]AccountInfo, error) {
	cut := now.AddDate(0, 0, -30).Unix()
	rows, err := s.db.Query(`
		SELECT a.org_name, a.name, a.currency, a.balance_cents,
		       COUNT(t.id),
		       COALESCE(MAX(t.posted), 0),
		       -COALESCE(SUM(CASE WHEN t.amount_cents < 0 AND t.posted >= ? THEN t.amount_cents ELSE 0 END), 0)
		FROM accounts a LEFT JOIN transactions t ON t.account_id = a.id
		GROUP BY a.id
		ORDER BY a.balance_cents DESC`, cut)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccountInfo
	for rows.Next() {
		var a AccountInfo
		var balCents, lastPosted, spentCents int64
		if err := rows.Scan(&a.Org, &a.Name, &a.Currency, &balCents, &a.TxnCount, &lastPosted, &spentCents); err != nil {
			return nil, err
		}
		a.Balance = float64(balCents) / 100
		a.Spent30 = float64(spentCents) / 100
		if lastPosted > 0 {
			a.LastTxn = time.Unix(lastPosted, 0).Format("2006-01-02")
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AccountRef is the minimal account identity used to populate filter dropdowns.
type AccountRef struct {
	ID   string
	Name string
}

func (s *Store) AccountList() ([]AccountRef, error) {
	rows, err := s.db.Query(`SELECT id, name FROM accounts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccountRef
	for rows.Next() {
		var a AccountRef
		if err := rows.Scan(&a.ID, &a.Name); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
