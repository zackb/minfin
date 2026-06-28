package store

import (
	"errors"
	"strings"
	"time"
)

// ErrUnknownCategory is returned when assigning a category that does not exist.
var ErrUnknownCategory = errors.New("unknown category")

// Category is a user-assignable transaction label. Exclude leaves it out of the
// spend/income totals (e.g. Transfer, which moves money between own accounts).
type Category struct {
	Name    string
	Color   string
	Exclude bool
}

// Rule maps payees to a category: payee LIKE '%'||Pattern||'%' (case-insensitive).
type Rule struct {
	ID       int64
	Pattern  string
	Category string
}

// CategoryStat is a per-category aggregate for the pie charts.
type CategoryStat struct {
	Category string
	Color    string
	Count    int
	Amount   float64 // dollars, always positive
}

// palette matches the spending chart so colors are consistent across screens.
var palette = []string{"#26c6da", "#7e57c2", "#66bb6a", "#ffa726", "#ef5350", "#42a5f5", "#ec407a", "#26a69a"}

// defaultCategories seed a Mint/Banktivity-like starter set. Order drives the
// seeded color (cycling the palette). Transfer is excluded from totals.
var defaultCategories = []Category{
	{"Groceries", "", false},
	{"Restaurants", "", false},
	{"Shopping", "", false},
	{"Gas & Fuel", "", false},
	{"Auto & Transport", "", false},
	{"Bills & Utilities", "", false},
	{"Rent & Mortgage", "", false},
	{"Health", "", false},
	{"Entertainment", "", false},
	{"Travel", "", false},
	{"Education", "", false},
	{"Fees & Charges", "", false},
	{"Paycheck", "", false},
	{"Other", "", false},
	{"Transfer", "", true},
}

// seedCategories inserts the defaults on a fresh DB. INSERT OR IGNORE so a
// user's renames/deletes/additions survive restarts.
func (s *Store) seedCategories() error {
	for i, c := range defaultCategories {
		if _, err := s.db.Exec(
			`INSERT OR IGNORE INTO categories(name, color, exclude) VALUES(?,?,?)`,
			c.Name, palette[i%len(palette)], boolToInt(c.Exclude)); err != nil {
			return err
		}
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) Categories() ([]Category, error) {
	rows, err := s.db.Query(`SELECT name, color, exclude FROM categories ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		var excl int
		if err := rows.Scan(&c.Name, &c.Color, &excl); err != nil {
			return nil, err
		}
		c.Exclude = excl != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

// categoryExists reports whether name is a known category (or the empty,
// uncategorized value).
func (s *Store) categoryExists(name string) (bool, error) {
	if name == "" {
		return true, nil
	}
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM categories WHERE name=?`, name).Scan(&n)
	return n > 0, err
}

// AddCategory creates a category, assigning the next palette color. No-op if it
// already exists.
func (s *Store) AddCategory(name string) error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&n); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO categories(name, color) VALUES(?,?)`,
		name, palette[n%len(palette)])
	return err
}

// DeleteCategory removes a category, clearing it from any transactions and rules.
func (s *Store) DeleteCategory(name string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE transactions SET category='' WHERE category=?`, name); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM category_rules WHERE category=?`, name); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM categories WHERE name=?`, name); err != nil {
		return err
	}
	return tx.Commit()
}

// SetTxnCategory assigns a category to one transaction. category must be a known
// category or "" to clear.
func (s *Store) SetTxnCategory(id, category string) error {
	ok, err := s.categoryExists(category)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUnknownCategory
	}
	_, err = s.db.Exec(`UPDATE transactions SET category=? WHERE id=?`, category, id)
	return err
}

func (s *Store) Rules() ([]Rule, error) {
	rows, err := s.db.Query(`SELECT id, pattern, category FROM category_rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.Pattern, &r.Category); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AddRule remembers a payee→category mapping. One rule per pattern: re-adding an
// existing pattern updates its category instead of duplicating. Validates the
// category exists.
func (s *Store) AddRule(pattern, category string) error {
	ok, err := s.categoryExists(category)
	if err != nil {
		return err
	}
	if !ok || category == "" {
		return ErrUnknownCategory
	}
	_, err = s.db.Exec(
		`INSERT INTO category_rules(pattern, category) VALUES(?,?)
		 ON CONFLICT(pattern) DO UPDATE SET category=excluded.category`, pattern, category)
	return err
}

// RuleMatches reports whether any saved rule already categorizes this payee
// (case-insensitive substring), i.e. it is already "remembered".
func (s *Store) RuleMatches(payee string, rules []Rule) bool {
	p := strings.ToLower(payee)
	for _, r := range rules {
		if r.Pattern != "" && strings.Contains(p, strings.ToLower(r.Pattern)) {
			return true
		}
	}
	return false
}

func (s *Store) DeleteRule(id int64) error {
	_, err := s.db.Exec(`DELETE FROM category_rules WHERE id=?`, id)
	return err
}

// ApplyRules categorizes uncategorized transactions from the saved rules and
// returns how many rows were updated. Fill-only (category=”): never overwrites
// a manual or existing category, and idempotent. First matching rule (by id)
// wins since later rules no longer see an empty category. Backs both the
// on-sync auto-categorization and the manual "recategorize" button.
func (s *Store) ApplyRules() (int, error) {
	rules, err := s.Rules()
	if err != nil {
		return 0, err
	}
	var n int64
	for _, r := range rules {
		res, err := s.db.Exec(
			`UPDATE transactions SET category=? WHERE category='' AND payee LIKE '%'||?||'%'`,
			r.Category, r.Pattern)
		if err != nil {
			return int(n), err
		}
		c, _ := res.RowsAffected()
		n += c
	}
	return int(n), nil
}

// SpendByCategory aggregates debit spend (amount<0) by category over [start,end),
// excluding categories flagged exclude. Empty categories bucket as "Uncategorized".
func (s *Store) SpendByCategory(start, end time.Time) ([]CategoryStat, error) {
	return s.byCategory(start, end, "t.amount_cents < 0", "-SUM(t.amount_cents)")
}

// IncomeByCategory aggregates credit income (amount>0) by category over [start,end).
func (s *Store) IncomeByCategory(start, end time.Time) ([]CategoryStat, error) {
	return s.byCategory(start, end, "t.amount_cents > 0", "SUM(t.amount_cents)")
}

func (s *Store) byCategory(start, end time.Time, sign, sum string) ([]CategoryStat, error) {
	rows, err := s.db.Query(
		`SELECT COALESCE(NULLIF(t.category,''), 'Uncategorized') AS cat,
		        COALESCE(c.color, '') AS color,
		        COUNT(*) AS n, `+sum+` AS amt
		 FROM transactions t LEFT JOIN categories c ON c.name = t.category
		 WHERE t.posted >= ? AND t.posted < ? AND `+sign+`
		       AND COALESCE(c.exclude, 0) = 0
		 GROUP BY cat
		 ORDER BY amt DESC`, start.Unix(), end.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CategoryStat
	for rows.Next() {
		var st CategoryStat
		var cents int64
		if err := rows.Scan(&st.Category, &st.Color, &st.Count, &cents); err != nil {
			return nil, err
		}
		st.Amount = float64(cents) / 100
		out = append(out, st)
	}
	return out, rows.Err()
}
