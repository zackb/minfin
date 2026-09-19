package store

import (
	"fmt"
	"testing"
	"time"

	"github.com/zackb/minfin/internal/simplefin"
)

func TestDetectSubscriptions(t *testing.T) {
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	ago := func(days int) time.Time { return now.AddDate(0, 0, -days) }
	var cs []charge
	add := func(payee string, cents int64, days ...int) {
		for _, d := range days {
			cs = append(cs, charge{payee, cents, ago(d)})
		}
	}
	add("NETFLIX.COM 8839", 1599, 5, 65)
	add("Netflix.com 1123", 1599, 35, 95)
	add("Apple.com/bill", 299, 3, 33, 63)
	add("Apple.com/bill", 1099, 10, 40, 70)
	add("Spotify", 1099, 42, 72)
	add("Spotify", 1199, 12) // price hike within 10%
	add("Costco Annual", 6500, 20, 385)
	add("Whole Foods", 8000, 2, 9, 30, 31, 55, 90) // irregular
	add("Old Gym", 4000, 120, 150, 180)            // lapsed monthly
	add("Demo Auto Finance", 41200, 5, 35, 65)     // loan payment
	add("ACME MORTGAGE PMT", 182000, 1, 31, 61)    // loan payment
	add("SafeWay Insurance", 13400, 7, 37, 67)     // insurance payment

	// Keyed by payee|cadence|amount so Apple's two tiers stay distinct.
	got := map[string]Subscription{}
	for _, s := range detectSubscriptions(cs, now) {
		got[fmt.Sprintf("%s|%s|%.2f", s.Payee, s.Cadence, s.Amount)] = s
	}
	want := map[string]struct {
		count  int
		active bool
	}{
		"NETFLIX.COM 8839|monthly|15.99": {4, true},
		"Apple.com/bill|monthly|2.99":    {3, true},
		"Apple.com/bill|monthly|10.99":   {3, true},
		"Spotify|monthly|11.99":          {3, true},
		"Costco Annual|yearly|65.00":     {2, true},
		"Old Gym|monthly|40.00":          {3, false},
	}
	for k, w := range want {
		s, ok := got[k]
		if !ok {
			t.Errorf("missing %s", k)
			continue
		}
		if s.Count != w.count || s.Active != w.active {
			t.Errorf("%s = count %d active %v, want %+v", k, s.Count, s.Active, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("detected %d, want %d: %v", len(got), len(want), got)
	}
	if s := got["Costco Annual|yearly|65.00"]; s.Monthly < 5.41 || s.Monthly > 5.42 {
		t.Errorf("yearly monthly cost = %v, want 65/12", s.Monthly)
	}
}

// Loan payments are recognized by a matching credit on a loan-typed account,
// and the Insurance category is skipped, whatever the payee is called.
func TestSubscriptionsSkipsLoansAndInsurance(t *testing.T) {
	s := openStore(t)
	if _, err := s.db.Exec(`INSERT INTO portfolios(id, created_at) VALUES(?, 0)`, testPID); err != nil {
		t.Fatal(err)
	}
	if err := s.seedCategories(testPID); err != nil {
		t.Fatal(err)
	}
	var chk, loan []simplefin.Transaction
	for i, d := range []string{"2026-06-17", "2026-07-20", "2026-08-18"} {
		day := at(d + "T12:00:00Z")
		id := fmt.Sprint(i)
		chk = append(chk,
			simplefin.Transaction{ID: "ford" + id, Posted: day, Amount: "-608.00", Payee: "Ford Motor Credit"},
			simplefin.Transaction{ID: "ins" + id, Posted: day, Amount: "-167.00", Payee: "Liberty Mutual"},
			simplefin.Transaction{ID: "kin" + id, Posted: day, Amount: "-11.99", Payee: "Kindle Unlimited"})
		loan = append(loan, simplefin.Transaction{ID: "pay" + id, Posted: day - 2*86400, Amount: "608.00", Payee: "Payment Received"})
	}
	if err := s.SaveAccountSet(testPID, simplefin.AccountSet{Accounts: []simplefin.Account{
		{ID: "chk", Name: "Checking", Balance: "100.00", Transactions: chk},
		{ID: "car", Name: "Car Loan", Balance: "-9000.00", Transactions: loan},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccountType(testPID, "car", "auto_loan"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddCategory(testPID, "Insurance"); err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		if err := s.SetTxnCategory(testPID, fmt.Sprint("ins", i), "Insurance"); err != nil {
			t.Fatal(err)
		}
	}

	subs, err := s.Subscriptions(testPID, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || subs[0].Payee != "Kindle Unlimited" {
		t.Errorf("subscriptions = %+v, want only Kindle Unlimited", subs)
	}
}
