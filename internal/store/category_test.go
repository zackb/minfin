package store

import (
	"testing"

	"github.com/zackb/minfin/internal/simplefin"
)

func TestApplyRulesFillOnly(t *testing.T) {
	s := seedStore(t) // t1/t2/t5 payee "Coffee", t3 "Grocer"
	if err := s.AddRule("Coffee", "Restaurants"); err != nil {
		t.Fatal(err)
	}
	// A manual category that ApplyRules must not overwrite.
	if err := s.SetTxnCategory("t2", "Travel"); err != nil {
		t.Fatal(err)
	}

	n, err := s.ApplyRules()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 { // t1 and t5; t2 was already categorized
		t.Fatalf("applied %d, want 2", n)
	}
	// Idempotent: re-running changes nothing.
	if n, err := s.ApplyRules(); err != nil || n != 0 {
		t.Fatalf("re-apply = %d, %v; want 0, nil", n, err)
	}

	spend, err := s.SpendByCategory(rangeStart, rangeEnd)
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

func TestSetTxnCategoryRejectsUnknown(t *testing.T) {
	s := seedStore(t)
	if err := s.SetTxnCategory("t1", "Nonexistent"); err != ErrUnknownCategory {
		t.Fatalf("got %v, want ErrUnknownCategory", err)
	}
}

func TestTransferExcludedFromSpend(t *testing.T) {
	s := seedStore(t)
	if err := s.SetTxnCategory("t3", "Transfer"); err != nil { // Transfer is exclude=1
		t.Fatal(err)
	}
	spend, err := s.SpendByCategory(rangeStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range spend {
		if st.Category == "Transfer" {
			t.Fatalf("Transfer should be excluded from spend, got %+v", st)
		}
	}
}

func TestCategorySurvivesResync(t *testing.T) {
	s := seedStore(t)
	if err := s.SetTxnCategory("t1", "Groceries"); err != nil {
		t.Fatal(err)
	}
	// Re-sync the same transaction (e.g. it flips out of pending).
	if err := s.SaveAccountSet(simplefin.AccountSet{Accounts: []simplefin.Account{{
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
