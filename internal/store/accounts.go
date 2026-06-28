package store

import "time"

type AccountInfo struct {
	ID        string
	Org       string
	Name      string
	Nickname  string
	Currency  string
	Type       string  // account type key, "" if uncategorized
	Liability  bool    // counts against net worth
	HasAsset   bool    // type carries an underlying asset value (house, car)
	Balance    float64 // dollars
	AssetValue float64 // underlying asset value in dollars, 0 if none
	TxnCount   int
	LastTxn    string  // YYYY-MM-DD, "" if none
	Spent30    float64 // debit spend in last 30 days, dollars
}

// Equity is the asset value net of the loan owed (Balance is negative for
// liabilities). Only meaningful when HasAsset.
func (a AccountInfo) Equity() float64 { return a.Balance + a.AssetValue }

// Accounts lists every connected account with balance and activity stats.
func (s *Store) Accounts(now time.Time) ([]AccountInfo, error) {
	cut := now.AddDate(0, 0, -30).Unix()
	rows, err := s.db.Query(`
		SELECT a.id, a.org_name, a.name, a.nickname, a.currency, a.type, a.balance_cents, a.asset_value_cents,
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
		var balCents, assetCents, lastPosted, spentCents int64
		if err := rows.Scan(&a.ID, &a.Org, &a.Name, &a.Nickname, &a.Currency, &a.Type, &balCents, &assetCents, &a.TxnCount, &lastPosted, &spentCents); err != nil {
			return nil, err
		}
		a.Liability = Classify(a.Type, balCents)
		a.HasAsset = HasAsset(a.Type)
		a.Balance = float64(balCents) / 100
		a.AssetValue = float64(assetCents) / 100
		a.Spent30 = float64(spentCents) / 100
		if lastPosted > 0 {
			a.LastTxn = time.Unix(lastPosted, 0).Format("2006-01-02")
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Display is the nickname if set, else the institution-given name.
func (a AccountInfo) Display() string {
	if a.Nickname != "" {
		return a.Nickname
	}
	return a.Name
}

// SetAccountType assigns a category to an account.
func (s *Store) SetAccountType(id, typ string) error {
	_, err := s.db.Exec(`UPDATE accounts SET type=? WHERE id=?`, typ, id)
	return err
}

// SetAccountAssetValue sets the underlying asset value (in cents) for a loan
// account; 0 clears it.
func (s *Store) SetAccountAssetValue(id string, cents int64) error {
	_, err := s.db.Exec(`UPDATE accounts SET asset_value_cents=? WHERE id=?`, cents, id)
	return err
}

// SetAccountNickname sets an optional user-friendly name; "" clears it.
func (s *Store) SetAccountNickname(id, nick string) error {
	_, err := s.db.Exec(`UPDATE accounts SET nickname=? WHERE id=?`, nick, id)
	return err
}

// AccountRef is the minimal account identity used to populate filter dropdowns.
type AccountRef struct {
	ID   string
	Name string
}

func (s *Store) AccountList() ([]AccountRef, error) {
	rows, err := s.db.Query(`SELECT id, COALESCE(NULLIF(nickname,''), name) FROM accounts ORDER BY name`)
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
