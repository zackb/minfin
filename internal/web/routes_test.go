package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func get(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestRoutes(t *testing.T) {
	s := newTestServer(t)

	if rec := get(t, s, "/"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Net Worth") {
		t.Errorf("GET / = %d, want 200 with dashboard; body has Net Worth: %v",
			rec.Code, strings.Contains(rec.Body.String(), "Net Worth"))
	}
	if rec := get(t, s, "/spending"); rec.Code != 200 {
		t.Errorf("GET /spending = %d, want 200", rec.Code)
	}
	if rec := get(t, s, "/nonsense"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /nonsense = %d, want 404", rec.Code)
	}
	if rec := get(t, s, "/static/css/theme.css"); rec.Code != 200 {
		t.Errorf("GET /static/css/theme.css = %d, want 200", rec.Code)
	}
	for _, p := range []string{"/static/img/logo.png", "/static/img/favicon.png"} {
		if rec := get(t, s, p); rec.Code != 200 {
			t.Errorf("GET %s = %d, want 200", p, rec.Code)
		}
	}
}
