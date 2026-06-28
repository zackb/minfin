package auth

import (
	"net/http/httptest"
	"testing"
)

func svc(t *testing.T) *Service {
	t.Helper()
	s, err := New("test-secret", true)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPasswordHashing(t *testing.T) {
	h, err := HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "hunter2") {
		t.Error("correct password rejected")
	}
	if CheckPassword(h, "wrong") {
		t.Error("wrong password accepted")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	s := svc(t)
	tok, err := s.CreateToken("user-123")
	if err != nil {
		t.Fatal(err)
	}
	uid, err := s.Validate(tok)
	if err != nil || uid != "user-123" {
		t.Fatalf("validate = %q, %v; want user-123", uid, err)
	}
}

func TestTamperedTokenRejected(t *testing.T) {
	s := svc(t)
	tok, _ := s.CreateToken("user-123")
	if _, err := s.Validate(tok + "x"); err == nil {
		t.Error("tampered token accepted")
	}
	// A token signed with a different secret must not validate.
	other, _ := New("other-secret", true)
	otherTok, _ := other.CreateToken("user-123")
	if _, err := s.Validate(otherTok); err == nil {
		t.Error("token from a different secret accepted")
	}
}

func TestIsAuthenticatedHeaderAndCookie(t *testing.T) {
	s := svc(t)
	tok, _ := s.CreateToken("user-123")

	// Bearer header.
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	if uid, ok := s.IsAuthenticated(r); !ok || uid != "user-123" {
		t.Errorf("bearer: %q, %v", uid, ok)
	}

	// Cookie.
	rec := httptest.NewRecorder()
	s.SetCookie(rec, tok)
	r2 := httptest.NewRequest("GET", "/", nil)
	for _, c := range rec.Result().Cookies() {
		r2.AddCookie(c)
	}
	if uid, ok := s.IsAuthenticated(r2); !ok || uid != "user-123" {
		t.Errorf("cookie: %q, %v", uid, ok)
	}

	// No credentials.
	if _, ok := s.IsAuthenticated(httptest.NewRequest("GET", "/", nil)); ok {
		t.Error("unauthenticated request accepted")
	}
}

func TestProdRequiresSecret(t *testing.T) {
	if _, err := New("", false); err == nil {
		t.Error("empty secret accepted in non-dev")
	}
	if _, err := New("", true); err != nil {
		t.Errorf("dev should allow empty secret: %v", err)
	}
}
