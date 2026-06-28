package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRoutes(t *testing.T) {
	e := newEnv(t)

	if rec := get(t, e, "/"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Net Worth") {
		t.Errorf("GET / = %d, want 200 with dashboard; body has Net Worth: %v",
			rec.Code, strings.Contains(rec.Body.String(), "Net Worth"))
	}
	if rec := get(t, e, "/spending"); rec.Code != 200 {
		t.Errorf("GET /spending = %d, want 200", rec.Code)
	}
	if rec := get(t, e, "/nonsense"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /nonsense = %d, want 404", rec.Code)
	}
	if rec := get(t, e, "/static/css/theme.css"); rec.Code != 200 {
		t.Errorf("GET /static/css/theme.css = %d, want 200", rec.Code)
	}
	for _, p := range []string{"/static/img/logo.png", "/static/img/favicon.png"} {
		if rec := get(t, e, p); rec.Code != 200 {
			t.Errorf("GET %s = %d, want 200", p, rec.Code)
		}
	}
}

// TestAuthGating covers the login/signup/logout flow and the unauthenticated
// redirect.
func TestAuthGating(t *testing.T) {
	e := newEnv(t)

	// No cookie -> redirected to /login.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Errorf("unauthenticated / = %d -> %q, want 303 -> /login", rec.Code, rec.Header().Get("Location"))
	}

	// /login renders without auth.
	req = httptest.NewRequest(http.MethodGet, "/login", nil)
	rec = httptest.NewRecorder()
	e.s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Sign in") {
		t.Errorf("GET /login = %d, want 200 with sign-in form", rec.Code)
	}

	// Signup creates an account, sets a cookie, redirects home.
	form := url.Values{"email": {"new@test.example"}, "password": {"longenough"}}
	req = httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	e.s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("signup = %d, want 303", rec.Code)
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("signup did not set an auth cookie")
	}

	// A brand-new user has no portfolio -> dashboard shows the setup card.
	cookie := rec.Result().Cookies()[0]
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	e.s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "setup token") {
		t.Errorf("new user / = %d, want 200 with setup card", rec.Code)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	e := newEnv(t) // seeds user@test.example / password123
	form := url.Values{"email": {"user@test.example"}, "password": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Invalid email or password") {
		t.Errorf("bad login = %d, want 200 with error message", rec.Code)
	}
}
