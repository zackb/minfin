package store

import (
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Subscription is a recurring charge inferred from transaction history.
type Subscription struct {
	Payee   string    `json:"payee"`
	Cadence string    `json:"cadence"` // weekly | monthly | quarterly | yearly
	Amount  float64   `json:"amount"`  // latest charge, dollars, positive
	Monthly float64   `json:"monthly"` // Amount normalized to a per-month cost
	Count   int       `json:"count"`
	Last    time.Time `json:"last"`
	Next    time.Time `json:"next"` // Last + one period
	Active  bool      `json:"active"`
}

// charge is one debit fed to detection.
type charge struct {
	payee  string
	cents  int64 // positive
	posted time.Time
}

// cadence bands: a cluster matches when its median gap (days) is in [lo,hi].
var cadences = []struct {
	name     string
	lo, hi   float64
	period   int // days, for Next / active grace
	minCount int
	perMonth float64
}{
	{"weekly", 5, 9, 7, 3, 52.0 / 12},
	{"monthly", 26, 35, 30, 3, 1},
	{"quarterly", 84, 98, 91, 2, 1.0 / 3},
	{"yearly", 350, 380, 365, 2, 1.0 / 12},
}

// Subscriptions detects recurring debits over the 13 months before now,
// ignoring pending rows, excluded categories (transfers, card payments), and
// recurring bills: mortgage/loan, utility, and insurance payments. A debit
// whose amount lands as a credit on a loan-typed account within 5 days is a
// loan payment regardless of payee name.
// Active first, then by monthly cost descending.
func (s *Store) Subscriptions(portfolioID string, now time.Time) ([]Subscription, error) {
	rows, err := s.db.Query(
		`SELECT t.payee, -t.amount_cents, t.posted
		 FROM transactions t LEFT JOIN categories c ON c.portfolio_id = t.portfolio_id AND c.name = t.category
		 WHERE t.portfolio_id = ? AND t.posted >= ? AND t.amount_cents < 0
		   AND COALESCE(t.pending,0) = 0 AND COALESCE(c.exclude,0) = 0
		   AND t.category NOT IN ('Rent & Mortgage', 'Bills & Utilities', 'Insurance')
		   AND NOT EXISTS (
		     SELECT 1 FROM transactions p JOIN accounts la ON la.portfolio_id = p.portfolio_id AND la.id = p.account_id
		     WHERE p.portfolio_id = t.portfolio_id AND la.type IN ('mortgage', 'auto_loan', 'loan')
		       AND p.amount_cents = -t.amount_cents AND ABS(p.posted - t.posted) <= 5*86400)`,
		portfolioID, now.AddDate(0, -13, 0).Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cs []charge
	for rows.Next() {
		var c charge
		var posted int64
		if err := rows.Scan(&c.payee, &c.cents, &posted); err != nil {
			return nil, err
		}
		c.posted = time.Unix(posted, 0)
		cs = append(cs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return detectSubscriptions(cs, now), nil
}

// normPayee folds payee variants ("NETFLIX.COM 8839", "Netflix.com 1123") to one key.
func normPayee(p string) string {
	f := strings.FieldsFunc(strings.ToLower(p), func(r rune) bool { return !unicode.IsLetter(r) })
	return strings.Join(f, " ")
}

// billWords mark loan and insurance payments, which recur but aren't subscriptions.
var billWords = map[string]bool{
	"mortgage": true, "loan": true, "loans": true, "finance": true, "lending": true,
	"insurance": true, "assurance": true,
}

func isBill(key string) bool {
	for _, w := range strings.Fields(key) {
		if billWords[w] {
			return true
		}
	}
	return false
}

// detectSubscriptions groups charges by normalized payee, splits each group
// into amount clusters (within 10% of the cluster's smallest charge), and keeps
// clusters whose spacing matches a cadence band.
// minCount charges to reappear; merge adjacent same-cadence clusters if that bites.
func detectSubscriptions(cs []charge, now time.Time) []Subscription {
	groups := map[string][]charge{}
	for _, c := range cs {
		k := normPayee(c.payee)
		if k == "" || isBill(k) {
			continue
		}
		groups[k] = append(groups[k], c)
	}

	var out []Subscription
	for _, g := range groups {
		sort.Slice(g, func(i, j int) bool { return g[i].cents < g[j].cents })
		for start := 0; start < len(g); {
			end := start + 1
			for end < len(g) && float64(g[end].cents) <= float64(g[start].cents)*1.10 {
				end++
			}
			if sub, ok := matchCadence(slices.Clone(g[start:end]), now); ok {
				out = append(out, sub)
			}
			start = end
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		if out[i].Monthly != out[j].Monthly {
			return out[i].Monthly > out[j].Monthly
		}
		return out[i].Payee < out[j].Payee
	})
	return out
}

// matchCadence reports the subscription a single amount cluster represents, if
// its median gap falls in a cadence band and at least 2/3 of its gaps do too.
func matchCadence(cl []charge, now time.Time) (Subscription, bool) {
	if len(cl) < 2 {
		return Subscription{}, false
	}
	sort.Slice(cl, func(i, j int) bool { return cl[i].posted.Before(cl[j].posted) })
	gaps := make([]float64, len(cl)-1)
	for i := range gaps {
		gaps[i] = cl[i+1].posted.Sub(cl[i].posted).Hours() / 24
	}
	sorted := slices.Sorted(slices.Values(gaps))
	median := sorted[len(sorted)/2]

	for _, cd := range cadences {
		if len(cl) < cd.minCount || median < cd.lo || median > cd.hi {
			continue
		}
		in := 0
		for _, g := range gaps {
			if g >= cd.lo && g <= cd.hi {
				in++
			}
		}
		if in*3 < len(gaps)*2 {
			return Subscription{}, false
		}
		last := cl[len(cl)-1]
		amt := float64(last.cents) / 100
		next := last.posted.AddDate(0, 0, cd.period)
		return Subscription{
			Payee:   last.payee,
			Cadence: cd.name,
			Amount:  amt,
			Monthly: amt * cd.perMonth,
			Count:   len(cl),
			Last:    last.posted,
			Next:    next,
			Active:  !next.AddDate(0, 0, cd.period/4).Before(now),
		}, true
	}
	return Subscription{}, false
}
