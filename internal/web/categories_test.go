package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/zackb/minfin/internal/simplefin"
)

func post(t *testing.T, s *Server, path string, form url.Values) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec.Code
}

func TestHandleTxnCategoryAndRemember(t *testing.T) {
	s := newTestServer(t)
	if err := s.store.SaveAccountSet(simplefin.AccountSet{Accounts: []simplefin.Account{{
		ID: "a1", Name: "Checking", Balance: "100.00",
		Transactions: []simplefin.Transaction{
			{ID: "t1", Posted: 1717000000, Amount: "-10.00", Payee: "FRED MEYER #1"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}

	if code := post(t, s, "/transactions/category", url.Values{
		"id": {"t1"}, "category": {"Groceries"}, "remember": {"1"}, "payee": {"FRED MEYER"},
	}); code != http.StatusSeeOther {
		t.Fatalf("set category: got %d, want 303", code)
	}

	rules, err := s.store.Rules()
	if err != nil || len(rules) != 1 || rules[0].Pattern != "FRED MEYER" {
		t.Fatalf("rules = %+v, %v; want one FRED MEYER rule", rules, err)
	}

	// Remembering the same payee again must not create a duplicate; a different
	// category updates the existing rule in place.
	if code := post(t, s, "/transactions/category", url.Values{
		"id": {"t1"}, "category": {"Shopping"}, "remember": {"1"}, "payee": {"FRED MEYER"},
	}); code != http.StatusSeeOther {
		t.Fatalf("re-remember: got %d, want 303", code)
	}
	rules, _ = s.store.Rules()
	if len(rules) != 1 || rules[0].Category != "Shopping" {
		t.Fatalf("rules = %+v; want one rule updated to Shopping", rules)
	}

	if code := post(t, s, "/transactions/category", url.Values{
		"id": {"t1"}, "category": {"Nope"},
	}); code != http.StatusBadRequest {
		t.Errorf("unknown category: got %d, want 400", code)
	}
}

func TestCategoriesPageRenders(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/categories", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /categories = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Groceries") {
		t.Error("categories page missing seeded category")
	}
}
