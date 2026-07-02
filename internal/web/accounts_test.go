package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zackb/minfin/internal/auth"
	"github.com/zackb/minfin/internal/simplefin"
	"github.com/zackb/minfin/internal/store"
)

// env is an authenticated test fixture: a server, a signed-in user's portfolio
// id, and the auth cookie to attach to requests.
type env struct {
	s      *Server
	pid    string
	cookie *http.Cookie
}

func newEnv(t *testing.T) *env {
	t.Helper()
	t.Setenv("MINFIN_ALLOW_SIGNUP", "1") // signup is gated off by default; tests need it on
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	authSvc, err := auth.New("test-secret", true)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := auth.HashPassword("password123")
	u, err := st.CreateUser("user@test.example", hash)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := st.CreatePortfolio("", "http://example.test/simplefin")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddMember(pid, u.ID, "owner"); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveAccountSet(pid, simplefin.AccountSet{Accounts: []simplefin.Account{
		{ID: "a1", Name: "Checking", Balance: "100.00"},
	}}); err != nil {
		t.Fatal(err)
	}
	tok, err := authSvc.CreateToken(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	authSvc.SetCookie(rec, tok)
	return &env{s: NewServer(st, authSvc), pid: pid, cookie: rec.Result().Cookies()[0]}
}

// get/post issue authenticated requests through the full handler (auth middleware
// included).
func get(t *testing.T, e *env, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(e.cookie)
	rec := httptest.NewRecorder()
	e.s.Handler().ServeHTTP(rec, req)
	return rec
}

func post(t *testing.T, e *env, path string, form url.Values) int {
	t.Helper()
	return postRec(t, e, path, form).Code
}

// postRec is like post but returns the recorder, for asserting on the flash
// cookie a failed action sets before redirecting.
func postRec(t *testing.T, e *env, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(e.cookie)
	rec := httptest.NewRecorder()
	e.s.Handler().ServeHTTP(rec, req)
	return rec
}

// flashed reports whether the response set a non-empty flash cookie, i.e. the
// handler took its friendly-error path rather than succeeding silently.
func flashed(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "minfin_flash" && c.MaxAge >= 0 && c.Value != "" {
			return true
		}
	}
	return false
}

func TestSummarizeWithAsset(t *testing.T) {
	// Mortgage: -320k loan + 500k home value => +180k equity, asset counts
	// toward assets/net, loan toward liabilities.
	accts := []store.AccountInfo{
		{Balance: 100, Liability: false},                                        // checking
		{Balance: -320000, Liability: true, HasAsset: true, AssetValue: 500000}, // mortgage
	}
	assets, liabilities, net := summarize(accts)
	if assets != 500100 {
		t.Errorf("assets: got %v, want 500100", assets)
	}
	if liabilities != -320000 {
		t.Errorf("liabilities: got %v, want -320000", liabilities)
	}
	if net != 180100 {
		t.Errorf("net: got %v, want 180100", net)
	}
	if eq := accts[1].Equity(); eq != 180000 {
		t.Errorf("equity: got %v, want 180000", eq)
	}
}

func TestHandleAccountType(t *testing.T) {
	e := newEnv(t)
	if rec := postRec(t, e, "/accounts/type", url.Values{"id": {"a1"}, "type": {"bogus"}}); rec.Code != http.StatusSeeOther || !flashed(rec) {
		t.Errorf("invalid type: got %d flash=%v, want 303 with flash", rec.Code, flashed(rec))
	}
	if code := post(t, e, "/accounts/type", url.Values{"id": {"a1"}, "type": {"checking"}}); code != http.StatusSeeOther {
		t.Errorf("valid type: got %d, want 303", code)
	}
}
