// Package auth handles password hashing and JWT issue/validation. Tokens work
// from the web (httpOnly cookie) and a future REST API (Authorization: Bearer).
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// cookieName is the httpOnly cookie carrying the JWT for the web flow.
const cookieName = "token"

// Service issues and validates tokens with a single HS256 secret.
type Service struct {
	secret []byte
	ttl    time.Duration
	dev    bool
}

// New builds a Service. When secret is empty and dev is true, a random ephemeral
// secret is generated (sessions won't survive a restart) with a warning. An
// empty secret in production is a fatal misconfiguration.
func New(secret string, dev bool) (*Service, error) {
	if secret == "" {
		if !dev {
			return nil, errors.New("MINFIN_JWT_SECRET is required")
		}
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		secret = hex.EncodeToString(b)
		log.Printf("auth: MINFIN_JWT_SECRET unset; using an ephemeral dev secret (logins won't survive restart)")
	}
	return &Service{secret: []byte(secret), ttl: 168 * time.Hour, dev: dev}, nil
}

// HashPassword returns a bcrypt hash suitable for storage.
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword reports whether pw matches the stored bcrypt hash.
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// CreateToken mints a signed JWT whose subject is the user id.
func (s *Service) CreateToken(userID string) (string, error) {
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
	})
	return tok.SignedString(s.secret)
}

// Validate parses and verifies a token, returning the user id (subject).
func (s *Service) Validate(tokenStr string) (string, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return "", err
	}
	if claims.Subject == "" {
		return "", errors.New("token missing subject")
	}
	return claims.Subject, nil
}

// IsAuthenticated extracts and validates the token from the request, checking the
// Authorization: Bearer header first (API) then the httpOnly cookie (web).
func (s *Service) IsAuthenticated(r *http.Request) (string, bool) {
	tokenStr := ""
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		tokenStr = strings.TrimSpace(h[7:])
	}
	if tokenStr == "" {
		if c, err := r.Cookie(cookieName); err == nil {
			tokenStr = c.Value
		}
	}
	if tokenStr == "" {
		return "", false
	}
	userID, err := s.Validate(tokenStr)
	if err != nil {
		return "", false
	}
	return userID, true
}

// SetCookie writes the JWT as an httpOnly, SameSite=Lax cookie. Lax keeps the
// cookie off cross-site POSTs, which is adequate CSRF protection for this
// form-POST app (dedicated CSRF tokens deferred).
func (s *Service) SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   !s.dev,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(s.ttl),
	})
}

// ClearCookie deletes the auth cookie.
func (s *Service) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   !s.dev,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
