package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/zackb/minfin/internal/simplefin"
)

func TestHandleTxnCategoryAndRemember(t *testing.T) {
	e := newEnv(t)
	if err := e.s.store.SaveAccountSet(e.pid, simplefin.AccountSet{Accounts: []simplefin.Account{{
		ID: "a1", Name: "Checking", Balance: "100.00",
		Transactions: []simplefin.Transaction{
			{ID: "t1", Posted: 1717000000, Amount: "-10.00", Payee: "FRED MEYER #1"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}

	if code := post(t, e, "/transactions/category", url.Values{
		"id": {"t1"}, "category": {"Groceries"}, "remember": {"1"}, "payee": {"FRED MEYER"},
	}); code != http.StatusSeeOther {
		t.Fatalf("set category: got %d, want 303", code)
	}

	rules, err := e.s.store.Rules(e.pid)
	if err != nil || len(rules) != 1 || rules[0].Pattern != "FRED MEYER" {
		t.Fatalf("rules = %+v, %v; want one FRED MEYER rule", rules, err)
	}

	// Remembering the same payee again must not create a duplicate; a different
	// category updates the existing rule in place.
	if code := post(t, e, "/transactions/category", url.Values{
		"id": {"t1"}, "category": {"Shopping"}, "remember": {"1"}, "payee": {"FRED MEYER"},
	}); code != http.StatusSeeOther {
		t.Fatalf("re-remember: got %d, want 303", code)
	}
	rules, _ = e.s.store.Rules(e.pid)
	if len(rules) != 1 || rules[0].Category != "Shopping" {
		t.Fatalf("rules = %+v; want one rule updated to Shopping", rules)
	}

	if rec := postRec(t, e, "/transactions/category", url.Values{
		"id": {"t1"}, "category": {"Nope"},
	}); rec.Code != http.StatusSeeOther || !flashed(rec) {
		t.Errorf("unknown category: got %d flash=%v, want 303 with flash", rec.Code, flashed(rec))
	}
}

func TestCategoriesPageRenders(t *testing.T) {
	e := newEnv(t)
	rec := get(t, e, "/categories")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /categories = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Groceries") {
		t.Error("categories page missing seeded category")
	}
}
