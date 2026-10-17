package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zackb/minfin/internal/simplefin"
)

// The DB file carries bank credentials + financial data, so Open must lock it to
// owner-only (0600) regardless of umask.
func TestOpenLocksFilePerms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "perms.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("db perms = %o, want 600", perm)
	}
}

// testPID is the portfolio every seeded fixture lives under.
const testPID = "p1"

func at(s string) int64 {
	ts, _ := time.Parse(time.RFC3339, s)
	return ts.Unix()
}

func openStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// seedStore builds a temp DB with one portfolio (testPID) holding two accounts
// and a mix of debits/credits.
// Spending (debits) by day: 06-10 => 15.00, 06-11 => 27.50.
// Coffee debits total 22.50 (x3), Grocer 20.00 (x1).
func seedStore(t *testing.T) *Store {
	t.Helper()
	s := openStore(t)
	if _, err := s.db.Exec(`INSERT INTO portfolios(id, created_at) VALUES(?, 0)`, testPID); err != nil {
		t.Fatal(err)
	}
	if err := s.seedCategories(testPID); err != nil {
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
	if err := s.SaveAccountSet(testPID, set); err != nil {
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
	series, err := s.SpendingSeries(testPID, rangeStart, rangeEnd, "daily", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Labels) != 2 || series.Labels[0] != "2026-06-10" || series.Labels[1] != "2026-06-11" {
		t.Fatalf("labels = %v", series.Labels)
	}
	if len(series.Lines) != 1 || series.Lines[0].Values[0] != 15 || series.Lines[0].Values[1] != 27.5 {
		t.Fatalf("values = %+v, want [15 27.5]", series.Lines)
	}
	// Daily buckets: each range is the single day, fully inside the window.
	if series.Ranges[0] != (Bucket{"2026-06-10", "2026-06-10"}) ||
		series.Ranges[1] != (Bucket{"2026-06-11", "2026-06-11"}) {
		t.Fatalf("daily ranges = %+v", series.Ranges)
	}
}

func TestSpendingRangesMonthly(t *testing.T) {
	s := seedStore(t)
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	series, err := s.SpendingSeries(testPID, start, end, "monthly", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Ranges) != 1 || series.Ranges[0] != (Bucket{"2026-06-01", "2026-06-30"}) {
		t.Fatalf("monthly ranges = %+v, want one 2026-06-01..2026-06-30", series.Ranges)
	}
}

func TestSpendingRangesWeekly(t *testing.T) {
	s := seedStore(t)
	start := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	series, err := s.SpendingSeries(testPID, start, end, "weekly", false)
	if err != nil {
		t.Fatal(err)
	}
	d := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	monday := d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
	want := Bucket{monday.Format(dateLayout), monday.AddDate(0, 0, 6).Format(dateLayout)}
	if len(series.Ranges) != 1 || series.Ranges[0] != want {
		t.Fatalf("weekly ranges = %+v, want one %+v", series.Ranges, want)
	}
}

func TestSpendingRangesClamp(t *testing.T) {
	s := seedStore(t)
	start := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC) // exclusive
	series, err := s.SpendingSeries(testPID, start, end, "monthly", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Ranges) != 1 || series.Ranges[0] != (Bucket{"2026-06-10", "2026-06-11"}) {
		t.Fatalf("clamped ranges = %+v, want 2026-06-10..2026-06-11", series.Ranges)
	}
}

func TestTopPayees(t *testing.T) {
	s := seedStore(t)
	p, err := s.TopPayees(testPID, rangeStart, rangeEnd, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Sorted by spend: Coffee $22.50 (3 txns) > Grocer $20.00 (1).
	if len(p) != 2 || p[0].Payee != "Coffee" || p[0].Count != 3 || p[0].Spent != 22.5 {
		t.Fatalf("top payee = %+v, want Coffee x3 $22.50", p)
	}
}

func TestSpendingExcludesInvestmentAccounts(t *testing.T) {
	s := seedStore(t)
	if err := s.SaveAccountSet(testPID, simplefin.AccountSet{Accounts: []simplefin.Account{{
		ID: "brk", Name: "Brokerage", Balance: "5000.00",
		Transactions: []simplefin.Transaction{
			{ID: "b1", Posted: at("2026-06-11T12:00:00Z"), Amount: "-900.00", Payee: "Buy VTI"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccountType(testPID, "brk", "investment"); err != nil {
		t.Fatal(err)
	}
	series, err := s.SpendingSeries(testPID, rangeStart, rangeEnd, "daily", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, ln := range series.Lines {
		if ln.Name == "Brokerage" {
			t.Fatalf("investment account in spending lines: %+v", series.Lines)
		}
	}
	p, err := s.TopPayees(testPID, rangeStart, rangeEnd, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 2 || p[0].Payee != "Coffee" {
		t.Fatalf("top payees = %+v, want Coffee, Grocer only", p)
	}
}

func TestAccounts(t *testing.T) {
	s := seedStore(t)
	now := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	accts, err := s.Accounts(testPID, now)
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

func TestCashFlow(t *testing.T) {
	s := seedStore(t)
	if err := s.SaveAccountSet(testPID, simplefin.AccountSet{Accounts: []simplefin.Account{{
		ID: "loan1", Name: "Auto Loan", Balance: "-20000.00",
		Transactions: []simplefin.Transaction{
			{ID: "p1", Posted: at("2026-06-10T12:00:00Z"), Amount: "400.00", Payee: "Car Payment"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccountType(testPID, "loan1", "auto_loan"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAccountSet(testPID, simplefin.AccountSet{Accounts: []simplefin.Account{{
		ID: "brk", Name: "Brokerage", Balance: "5000.00",
		Transactions: []simplefin.Transaction{
			{ID: "b1", Posted: at("2026-06-10T12:00:00Z"), Amount: "300.00", Payee: "Dividend"},
			{ID: "b2", Posted: at("2026-06-11T12:00:00Z"), Amount: "-900.00", Payee: "Buy VTI"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccountType(testPID, "brk", "investment"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTxnCategory(testPID, "t3", "Transfer"); err != nil { // drops Grocer $20
		t.Fatal(err)
	}

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	cf, err := s.CashFlow(testPID, start, end, "quarterly")
	if err != nil {
		t.Fatal(err)
	}
	if len(cf.Labels) != 1 || cf.Labels[0] != "2026-Q2" {
		t.Fatalf("labels = %v, want [2026-Q2]", cf.Labels)
	}
	if cf.Ranges[0] != (Bucket{"2026-04-01", "2026-06-30"}) {
		t.Fatalf("ranges = %+v", cf.Ranges)
	}
	// Income: Paycheck 100 + Interest 50 (loan 400, brokerage excluded). Spend: coffee 22.50.
	if cf.Income != 150 || cf.Spending != 22.5 || cf.Net != 127.5 {
		t.Fatalf("totals = %v/%v/%v, want 150/22.5/127.5", cf.Income, cf.Spending, cf.Net)
	}
	if cf.Lines[0].Values[0] != 150 || cf.Lines[1].Values[0] != 22.5 {
		t.Fatalf("lines = %+v", cf.Lines)
	}
	if cf.SavingsRate != 0.85 {
		t.Fatalf("savings rate = %v, want 0.85", cf.SavingsRate)
	}

	// Yearly bucket clamps to the window.
	cf, err = s.CashFlow(testPID, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), "yearly")
	if err != nil {
		t.Fatal(err)
	}
	if len(cf.Labels) != 1 || cf.Labels[0] != "2026" || cf.Ranges[0] != (Bucket{"2026-06-01", "2026-06-14"}) {
		t.Fatalf("yearly = %v %+v", cf.Labels, cf.Ranges)
	}
}
