package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/zackb/minfin/internal/simplefin"
)

func at(s string) int64 {
	ts, _ := time.Parse(time.RFC3339, s)
	return ts.Unix()
}

// seedStore builds a temp DB with two accounts and a mix of debits/credits.
// Spending (debits) by day: 06-10 => 15.00, 06-11 => 27.50.
// Coffee debits total 22.50 (x3), Grocer 20.00 (x1).
func seedStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	set := simplefin.AccountSet{Accounts: []simplefin.Account{
		{
			ID: "a1", Name: "Checking", Balance: "1000.00",
			Transactions: []simplefin.Transaction{
				{ID: "t1", Posted: at("2026-06-10T12:00:00Z"), Amount: "-10.00", Payee: "Coffee"},
				{ID: "t2", Posted: at("2026-06-10T15:00:00Z"), Amount: "-5.00", Payee: "Coffee"},
				{ID: "t3", Posted: at("2026-06-11T09:00:00Z"), Amount: "-20.00", Payee: "Grocer", Description: "Groceries"},
				{ID: "t4", Posted: at("2026-06-10T10:00:00Z"), Amount: "100.00", Payee: "Paycheck"},
			},
		},
		{
			ID: "a2", Name: "Savings", Balance: "500.00",
			Transactions: []simplefin.Transaction{
				{ID: "t5", Posted: at("2026-06-11T08:00:00Z"), Amount: "-7.50", Payee: "Coffee"},
				{ID: "t6", Posted: at("2026-06-09T08:00:00Z"), Amount: "50.00", Payee: "Interest"},
			},
		},
	}}
	if err := s.SaveAccountSet(set); err != nil {
		t.Fatal(err)
	}
	return s
}

var (
	rangeStart = time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	rangeEnd   = time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
)

func TestSpendingSeries(t *testing.T) {
	s := seedStore(t)
	series, err := s.SpendingSeries(rangeStart, rangeEnd, "daily", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Labels) != 2 || series.Labels[0] != "2026-06-10" || series.Labels[1] != "2026-06-11" {
		t.Fatalf("labels = %v", series.Labels)
	}
	if len(series.Lines) != 1 || series.Lines[0].Values[0] != 15 || series.Lines[0].Values[1] != 27.5 {
		t.Fatalf("values = %+v, want [15 27.5]", series.Lines)
	}
}

func TestTopPayees(t *testing.T) {
	s := seedStore(t)
	p, err := s.TopPayees(rangeStart, rangeEnd, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Sorted by spend: Coffee $22.50 (3 txns) > Grocer $20.00 (1).
	if len(p) != 2 || p[0].Payee != "Coffee" || p[0].Count != 3 || p[0].Spent != 22.5 {
		t.Fatalf("top payee = %+v, want Coffee x3 $22.50", p)
	}
}

func TestAccounts(t *testing.T) {
	s := seedStore(t)
	now := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	accts, err := s.Accounts(now)
	if err != nil {
		t.Fatal(err)
	}
	// Ordered by balance desc: Checking (1000) first.
	if len(accts) != 2 || accts[0].Name != "Checking" || accts[0].TxnCount != 4 {
		t.Fatalf("accounts = %+v", accts)
	}
	if accts[0].Spent30 != 35 { // 10+5+20
		t.Fatalf("Checking Spent30 = %v, want 35", accts[0].Spent30)
	}
}
