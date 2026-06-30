package store

import (
	"testing"

	"github.com/zackb/minfin/internal/simplefin"
)

func TestApplyRulesFillOnly(t *testing.T) {
	s := seedStore(t) // t1/t2/t5 payee "Coffee", t3 "Grocer"
	if err := s.AddRule(testPID, "Coffee", "Restaurants"); err != nil {
		t.Fatal(err)
	}
	// A manual category that ApplyRules must not overwrite.
	if err := s.SetTxnCategory(testPID, "t2", "Travel"); err != nil {
		t.Fatal(err)
	}

	n, err := s.ApplyRules(testPID, false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 { // t1 and t5; t2 was already categorized
		t.Fatalf("applied %d, want 2", n)
	}
	// Idempotent: re-running changes nothing.
	if n, err := s.ApplyRules(testPID, false); err != nil || n != 0 {
		t.Fatalf("re-apply = %d, %v; want 0, nil", n, err)
	}

	spend, err := s.SpendByCategory(testPID, rangeStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, st := range spend {
		got[st.Category] = st.Amount
	}
	if got["Restaurants"] != 17.5 { // t1 $10 + t5 $7.50
		t.Errorf("Restaurants = %v, want 17.50", got["Restaurants"])
	}
	if got["Travel"] != 5 { // t2, kept from manual set
		t.Errorf("Travel = %v, want 5", got["Travel"])
	}
	if got["Uncategorized"] != 20 { // t3 Grocer untouched
		t.Errorf("Uncategorized = %v, want 20", got["Uncategorized"])
	}
}

func TestIncomeExcludesLiabilityAccounts(t *testing.T) {
	s := seedStore(t) // a1 Checking: Paycheck +100; a2 Savings: Interest +50
	// An auto loan whose payment posts as a positive amount (debt paydown).
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

	income, err := s.IncomeByCategory(testPID, rangeStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	var total float64
	for _, st := range income {
		total += st.Amount
	}
	if total != 100 { // Paycheck 100 in range; Interest 50 is at 06-09 (out of range); Car Payment 400 excluded
		t.Fatalf("income total = %v, want 100 (loan payment must be excluded)", total)
	}
}

func TestApplyRulesLongestMatchWins(t *testing.T) {
	s := seedStore(t)
	s.AddCategory(testPID, "PayPal")
	// Two transactions: one plain "Transfer", one "PayPal Instant Transfer".
	if err := s.SaveAccountSet(testPID, simplefin.AccountSet{Accounts: []simplefin.Account{{
		ID: "a1", Name: "Checking", Balance: "1000.00",
		Transactions: []simplefin.Transaction{
			{ID: "x1", Posted: at("2026-06-10T12:00:00Z"), Amount: "-100.00", Payee: "Transfer"},
			{ID: "x2", Posted: at("2026-06-10T13:00:00Z"), Amount: "-50.00", Payee: "PayPal Instant Transfer"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	// Add the shorter, more general rule FIRST to prove order-independence.
	if err := s.AddRule(testPID, "Transfer", "Transfer"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRule(testPID, "PayPal Instant Transfer", "PayPal"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyRules(testPID, false); err != nil {
		t.Fatal(err)
	}

	got := map[string]string{}
	rows, err := s.db.Query(`SELECT id, category FROM transactions WHERE id IN ('x1','x2')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, cat string
		if err := rows.Scan(&id, &cat); err != nil {
			t.Fatal(err)
		}
		got[id] = cat
	}
	if got["x1"] != "Transfer" {
		t.Errorf("x1 = %q, want Transfer", got["x1"])
	}
	if got["x2"] != "PayPal" { // longest pattern wins, not "Transfer"
		t.Errorf("x2 = %q, want PayPal", got["x2"])
	}
}

func TestApplyRulesOverwrite(t *testing.T) {
	s := seedStore(t) // t1/t2/t5 payee "Coffee"
	if err := s.AddRule(testPID, "Coffee", "Restaurants"); err != nil {
		t.Fatal(err)
	}
	// t2 is mis-categorized; fill-only would leave it, overwrite must fix it.
	if err := s.SetTxnCategory(testPID, "t2", "Travel"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.ApplyRules(testPID, true); err != nil {
		t.Fatal(err)
	}
	var cat string
	if err := s.db.QueryRow(`SELECT category FROM transactions WHERE id='t2'`).Scan(&cat); err != nil {
		t.Fatal(err)
	}
	if cat != "Restaurants" {
		t.Fatalf("t2 after overwrite = %q, want Restaurants", cat)
	}
	// A payee with no matching rule is left alone even in overwrite mode.
	if err := s.db.QueryRow(`SELECT category FROM transactions WHERE id='t3'`).Scan(&cat); err != nil {
		t.Fatal(err)
	}
	if cat != "" {
		t.Fatalf("t3 (Grocer, no rule) = %q, want empty", cat)
	}
}

func TestSetTxnCategoryRejectsUnknown(t *testing.T) {
	s := seedStore(t)
	if err := s.SetTxnCategory(testPID, "t1", "Nonexistent"); err != ErrUnknownCategory {
		t.Fatalf("got %v, want ErrUnknownCategory", err)
	}
}

func TestTransferExcludedFromSpend(t *testing.T) {
	s := seedStore(t)
	if err := s.SetTxnCategory(testPID, "t3", "Transfer"); err != nil { // Transfer is exclude=1
		t.Fatal(err)
	}
	spend, err := s.SpendByCategory(testPID, rangeStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range spend {
		if st.Category == "Transfer" {
			t.Fatalf("Transfer should be excluded from spend, got %+v", st)
		}
	}
}

func TestSetCategoryExcludeHidesFromSpending(t *testing.T) {
	s := seedStore(t)
	s.AddCategory(testPID, "Cards")
	// Put all the Coffee debits (t1 $10, t2 $5, t5 $7.50) into Cards, then exclude it.
	for _, id := range []string{"t1", "t2", "t5"} {
		if err := s.SetTxnCategory(testPID, id, "Cards"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetCategoryExclude(testPID, "Cards", true); err != nil {
		t.Fatal(err)
	}

	spend, err := s.SpendByCategory(testPID, rangeStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range spend {
		if st.Category == "Cards" {
			t.Fatalf("Cards should be excluded from spend pie, got %+v", st)
		}
	}

	// Spending screen: only Grocer's $20 (2026-06-11) remains.
	series, err := s.SpendingSeries(testPID, rangeStart, rangeEnd, "daily", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Labels) != 1 || series.Labels[0] != "2026-06-11" {
		t.Fatalf("labels = %v, want only 2026-06-11", series.Labels)
	}
	if series.Lines[0].Values[0] != 20 {
		t.Fatalf("total = %v, want 20", series.Lines[0].Values[0])
	}

	// Top payees: Coffee gone, only Grocer left.
	p, err := s.TopPayees(testPID, rangeStart, rangeEnd, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 1 || p[0].Payee != "Grocer" {
		t.Fatalf("top payees = %+v, want only Grocer", p)
	}

	// Toggling back restores it.
	if err := s.SetCategoryExclude(testPID, "Cards", false); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.TopPayees(testPID, rangeStart, rangeEnd, 10); len(p) != 2 {
		t.Fatalf("after include, top payees = %+v, want 2", p)
	}
}

func TestCategorySurvivesResync(t *testing.T) {
	s := seedStore(t)
	if err := s.SetTxnCategory(testPID, "t1", "Groceries"); err != nil {
		t.Fatal(err)
	}
	// Re-sync the same transaction (e.g. it flips out of pending).
	if err := s.SaveAccountSet(testPID, simplefin.AccountSet{Accounts: []simplefin.Account{{
		ID: "a1", Name: "Checking", Balance: "1000.00",
		Transactions: []simplefin.Transaction{
			{ID: "t1", Posted: at("2026-06-10T12:00:00Z"), Amount: "-10.00", Payee: "Coffee"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}
	var cat string
	if err := s.db.QueryRow(`SELECT category FROM transactions WHERE id='t1'`).Scan(&cat); err != nil {
		t.Fatal(err)
	}
	if cat != "Groceries" {
		t.Fatalf("category after re-sync = %q, want Groceries", cat)
	}
}
