package store

import (
	"testing"

	"github.com/zackb/minfin/internal/simplefin"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		typ  string
		bal  int64
		want bool
	}{
		{"credit_card", 100, true}, // type wins over positive sign
		{"mortgage", 0, true},
		{"checking", -100, false}, // type wins over negative sign
		{"", -100, true},          // untyped: sign fallback
		{"", 100, false},
	}
	for _, c := range cases {
		if got := Classify(c.typ, c.bal); got != c.want {
			t.Errorf("Classify(%q,%d) = %v, want %v", c.typ, c.bal, got, c.want)
		}
	}
}

func TestValidType(t *testing.T) {
	for _, tc := range []struct {
		key  string
		want bool
	}{{"", true}, {"mortgage", true}, {"bogus", false}} {
		if got := ValidType(tc.key); got != tc.want {
			t.Errorf("ValidType(%q) = %v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestSetAccountTypeRoundTrip(t *testing.T) {
	s := seedStore(t)
	if err := s.SetAccountType("a1", "checking"); err != nil {
		t.Fatal(err)
	}
	accts, err := s.Accounts(rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range accts {
		if a.ID == "a1" && a.Type != "checking" {
			t.Fatalf("a1 type = %q, want checking", a.Type)
		}
	}
}

// The bug guard: a re-sync must not wipe a user-set type, and must update balance.
func TestSaveAccountSetPreservesType(t *testing.T) {
	s := seedStore(t)
	if err := s.SetAccountType("a1", "checking"); err != nil {
		t.Fatal(err)
	}
	resync := simplefin.AccountSet{Accounts: []simplefin.Account{
		{ID: "a1", Name: "Checking", Balance: "1234.00"},
	}}
	if err := s.SaveAccountSet(resync); err != nil {
		t.Fatal(err)
	}
	accts, err := s.Accounts(rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, a := range accts {
		if a.ID == "a1" {
			found = true
			if a.Type != "checking" {
				t.Errorf("type wiped on re-sync: %q", a.Type)
			}
			if a.Balance != 1234 {
				t.Errorf("balance not updated: %v", a.Balance)
			}
		}
	}
	if !found {
		t.Fatal("a1 missing after re-sync")
	}
}
