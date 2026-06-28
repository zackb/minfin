package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zackb/minfin/internal/simplefin"
	"github.com/zackb/minfin/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetAccessURL("http://example.test/simplefin"); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveAccountSet(simplefin.AccountSet{Accounts: []simplefin.Account{
		{ID: "a1", Name: "Checking", Balance: "100.00"},
	}}); err != nil {
		t.Fatal(err)
	}
	return NewServer(st)
}

func postType(t *testing.T, s *Server, id, typ string) int {
	t.Helper()
	body := url.Values{"id": {id}, "type": {typ}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/accounts/type", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec.Code
}

func TestHandleAccountType(t *testing.T) {
	s := newTestServer(t)
	if code := postType(t, s, "a1", "bogus"); code != http.StatusBadRequest {
		t.Errorf("invalid type: got %d, want 400", code)
	}
	if code := postType(t, s, "a1", "checking"); code != http.StatusSeeOther {
		t.Errorf("valid type: got %d, want 303", code)
	}
}
