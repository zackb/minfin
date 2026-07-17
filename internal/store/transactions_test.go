package store

import (
	"testing"
	"time"
)

func wideRange() (time.Time, time.Time) {
	return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
}

func TestTransactionsAccountFilter(t *testing.T) {
	s := seedStore(t)
	start, end := wideRange()
	rows, _, err := s.Transactions(TxnFilter{PortfolioID: testPID, Start: start, End: end, AccountIDs: []string{"a1"}, Direction: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 { // a1 has t1..t4
		t.Fatalf("account a1 = %d rows, want 4", len(rows))
	}
}

// Multi-select accounts OR together; multi-select categories OR together, including
// the "none"/"budget" sentinels. Seeded txns start uncategorized; categorize a few.
func TestTransactionsMultiFilter(t *testing.T) {
	s := seedStore(t)
	start, end := wideRange()

	// Both accounts selected -> all 6 rows.
	rows, _, _ := s.Transactions(TxnFilter{PortfolioID: testPID, Start: start, End: end, AccountIDs: []string{"a1", "a2"}})
	if len(rows) != 6 {
		t.Fatalf("accounts a1+a2 = %d rows, want 6", len(rows))
	}

	// Assign categories: t1->Travel, t3->Shopping (add it), t4->Transfer (excluded).
	// t2,t5,t6 stay uncategorized.
	if err := s.AddCategory(testPID, "Shopping"); err != nil {
		t.Fatal(err)
	}
	for id, cat := range map[string]string{"t1": "Travel", "t3": "Shopping", "t4": "Transfer"} {
		if err := s.SetTxnCategory(testPID, id, cat); err != nil {
			t.Fatal(err)
		}
	}

	// Travel + Shopping -> t1, t3.
	rows, _, _ = s.Transactions(TxnFilter{PortfolioID: testPID, Start: start, End: end, Categories: []string{"Travel", "Shopping"}})
	if len(rows) != 2 {
		t.Fatalf("Travel+Shopping = %d rows, want 2", len(rows))
	}

	// Uncategorized (none) + Travel -> t1 + the 3 blank ones (t2,t5,t6) = 4.
	rows, _, _ = s.Transactions(TxnFilter{PortfolioID: testPID, Start: start, End: end, Categories: []string{"none", "Travel"}})
	if len(rows) != 4 {
		t.Fatalf("none+Travel = %d rows, want 4", len(rows))
	}

	// Budget Only -> everything except the excluded Transfer (t4) = 5.
	rows, _, _ = s.Transactions(TxnFilter{PortfolioID: testPID, Start: start, End: end, Categories: []string{"budget"}})
	if len(rows) != 5 {
		t.Fatalf("budget = %d rows, want 5 (all but Transfer)", len(rows))
	}
}

func TestTransactionsDirectionFilter(t *testing.T) {
	s := seedStore(t)
	start, end := wideRange()
	debits, _, _ := s.Transactions(TxnFilter{PortfolioID: testPID, Start: start, End: end, Direction: "debit"})
	if len(debits) != 4 { // t1,t2,t3,t5
		t.Fatalf("debits = %d, want 4", len(debits))
	}
	for _, r := range debits {
		if r.Amount >= 0 {
			t.Fatalf("non-debit in debit filter: %+v", r)
		}
	}
	credits, _, _ := s.Transactions(TxnFilter{PortfolioID: testPID, Start: start, End: end, Direction: "credit"})
	if len(credits) != 2 { // t4,t6
		t.Fatalf("credits = %d, want 2", len(credits))
	}
}

func TestTransactionsSearch(t *testing.T) {
	s := seedStore(t)
	start, end := wideRange()
	rows, _, _ := s.Transactions(TxnFilter{PortfolioID: testPID, Start: start, End: end, Direction: "all", Query: "Coffee"})
	if len(rows) != 3 { // t1,t2,t5
		t.Fatalf("search Coffee = %d, want 3", len(rows))
	}
}

func TestTransactionsPagination(t *testing.T) {
	s := seedStore(t)
	start, end := wideRange()
	// 4 debits, page size 2 -> page 1 has next, page 2 does not.
	p1, hasNext, _ := s.Transactions(TxnFilter{PortfolioID: testPID, Start: start, End: end, Direction: "debit", Limit: 2})
	if len(p1) != 2 || !hasNext {
		t.Fatalf("page1: %d rows, hasNext=%v; want 2,true", len(p1), hasNext)
	}
	p2, hasNext, _ := s.Transactions(TxnFilter{PortfolioID: testPID, Start: start, End: end, Direction: "debit", Limit: 2, Offset: 2})
	if len(p2) != 2 || hasNext {
		t.Fatalf("page2: %d rows, hasNext=%v; want 2,false", len(p2), hasNext)
	}
}
