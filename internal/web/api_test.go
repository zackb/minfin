package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// apiReq issues a request through the full handler, optionally with a Bearer
// token, and returns the recorder.
func apiReq(t *testing.T, e *env, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *strings.Reader
	if body != "" {
		r = strings.NewReader(body)
	} else {
		r = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, r)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestAPIAuthAndIsolation(t *testing.T) {
	e := newEnv(t) // seeds user@test.example with a portfolio holding a "Checking" account
	token := e.cookie.Value

	// Authenticated read returns the user's account.
	rec := apiReq(t, e, http.MethodGet, "/api/accounts", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/accounts = %d, body=%s; want 200", rec.Code, rec.Body.String())
	}
	if n := accountCount(t, rec.Body.Bytes()); n != 1 {
		t.Fatalf("owner sees %d accounts, want 1", n)
	}

	// No token -> 401 JSON (not a redirect).
	rec = apiReq(t, e, http.MethodGet, "/api/accounts", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth GET /api/accounts = %d, want 401", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("unauth content-type = %q, want JSON", ct)
	}
	if !strings.Contains(rec.Body.String(), "error") {
		t.Fatalf("unauth body = %s, want an error field", rec.Body.String())
	}

	// A second user signs up via the API and gets a token, but no portfolio —
	// they must not see the first user's accounts.
	rec = apiReq(t, e, http.MethodPost, "/api/signup", "",
		`{"email":"second@test.example","password":"password9"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("signup = %d, body=%s", rec.Code, rec.Body.String())
	}
	var sr struct{ Token string }
	if err := json.Unmarshal(rec.Body.Bytes(), &sr); err != nil || sr.Token == "" {
		t.Fatalf("signup token = %q, err=%v", sr.Token, err)
	}
	rec = apiReq(t, e, http.MethodGet, "/api/accounts", sr.Token, "")
	if rec.Code != http.StatusOK || accountCount(t, rec.Body.Bytes()) != 0 {
		t.Fatalf("second user GET /api/accounts = %d, body=%s; want 0 accounts",
			rec.Code, rec.Body.String())
	}
}

func accountCount(t *testing.T, body []byte) int {
	t.Helper()
	var resp struct {
		Accounts []json.RawMessage `json:"accounts"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode accounts: %v (body=%s)", err, body)
	}
	return len(resp.Accounts)
}

func TestAPILoginWrongPassword(t *testing.T) {
	e := newEnv(t) // seeds user@test.example / password123
	rec := apiReq(t, e, http.MethodPost, "/api/login", "",
		`{"email":"user@test.example","password":"wrong"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login = %d, want 401", rec.Code)
	}
}
