package store

import (
	"testing"
	"time"

	"github.com/zackb/minfin/internal/auth"
	"github.com/zackb/minfin/internal/simplefin"
)

// seedTwoPortfolios creates two portfolios that connect the SAME SimpleFIN
// account/transaction ids, to prove the composite primary key keeps them apart.
func seedTwoPortfolios(t *testing.T) (*Store, string, string) {
	t.Helper()
	s := openStore(t)
	mk := func(id, payee, amount string) string {
		pid, err := s.CreatePortfolio("", "")
		if err != nil {
			t.Fatal(err)
		}
		set := simplefin.AccountSet{Accounts: []simplefin.Account{{
			ID: "acct", Name: "Checking", Balance: "100.00",
			Transactions: []simplefin.Transaction{
				{ID: "txn", Posted: at("2026-06-10T12:00:00Z"), Amount: amount, Payee: payee},
			},
		}}}
		if err := s.SaveAccountSet(pid, set); err != nil {
			t.Fatal(err)
		}
		return pid
	}
	p1 := mk("acct", "AlphaPayee", "-10.00")
	p2 := mk("acct", "BetaPayee", "-99.00")
	return s, p1, p2
}

func TestSaveAccountSetCompositeKeyNoCollision(t *testing.T) {
	s, p1, p2 := seedTwoPortfolios(t)
	now := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)

	a1, _ := s.Accounts(p1, now)
	a2, _ := s.Accounts(p2, now)
	if len(a1) != 1 || len(a2) != 1 {
		t.Fatalf("each portfolio should have its own 'acct' row: %d / %d", len(a1), len(a2))
	}
	// Same SimpleFIN id, different balances — neither overwrote the other.
	if a1[0].Balance != 100 || a2[0].Balance != 100 {
		t.Fatalf("balances = %v / %v", a1[0].Balance, a2[0].Balance)
	}
}

func TestTransactionsIsolated(t *testing.T) {
	s, p1, _ := seedTwoPortfolios(t)
	f := TxnFilter{
		PortfolioID: p1,
		Start:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		End:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	}
	rows, _, err := s.Transactions(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Payee != "AlphaPayee" {
		t.Fatalf("p1 transactions = %+v, want only AlphaPayee", rows)
	}
}

func TestApplyRulesDoesNotCrossPortfolios(t *testing.T) {
	s, p1, p2 := seedTwoPortfolios(t)
	// A rule + category in p1 must never touch p2's identically-keyed txn.
	if err := s.AddCategory(p1, "Shopping"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRule(p1, "Payee", "Shopping"); err != nil { // matches both payees by substring
		t.Fatal(err)
	}
	if _, err := s.ApplyRules(p1, false); err != nil {
		t.Fatal(err)
	}
	var cat string
	if err := s.db.QueryRow(`SELECT category FROM transactions WHERE portfolio_id=? AND id='txn'`, p2).Scan(&cat); err != nil {
		t.Fatal(err)
	}
	if cat != "" {
		t.Fatalf("p2 txn category = %q after p1 ApplyRules, want empty", cat)
	}
}

func TestSetterCannotMutateOtherPortfolio(t *testing.T) {
	s, p1, p2 := seedTwoPortfolios(t)
	// p1 tries to retype the shared id; p2's account must be untouched.
	if err := s.SetAccountType(p1, "acct", "credit_card"); err != nil {
		t.Fatal(err)
	}
	var typ string
	if err := s.db.QueryRow(`SELECT type FROM accounts WHERE portfolio_id=? AND id='acct'`, p2).Scan(&typ); err != nil {
		t.Fatal(err)
	}
	if typ != "" {
		t.Fatalf("p2 account type = %q, want empty (p1 must not reach it)", typ)
	}
}

func TestCategoriesIsolated(t *testing.T) {
	s, p1, p2 := seedTwoPortfolios(t)
	if err := s.AddCategory(p1, "OnlyInP1"); err != nil {
		t.Fatal(err)
	}
	cats, _ := s.Categories(p2)
	for _, c := range cats {
		if c.Name == "OnlyInP1" {
			t.Fatal("p2 sees p1's category")
		}
	}
}

func TestUserEmailCaseInsensitiveUnique(t *testing.T) {
	s := openStore(t)
	h, _ := auth.HashPassword("pw")
	if _, err := s.CreateUser("Zack@Example.com", h); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser("zack@example.com", h); err != ErrEmailTaken {
		t.Fatalf("duplicate (different case) = %v, want ErrEmailTaken", err)
	}
	u, err := s.UserByEmail("ZACK@EXAMPLE.COM")
	if err != nil || u.Email != "Zack@Example.com" {
		t.Fatalf("lookup = %+v, %v", u, err)
	}
}
