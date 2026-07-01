package web

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// rateLimiter is a tiny fixed-window per-key limiter guarding the auth endpoints
// against online password guessing. In-process only.
// ponytail: fixed-window map+mutex, no dep. Ceiling: single-instance; a hosted
// multi-replica deploy would move this to a shared store (Redis) or a proxy.
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string]*window
	max    int
	per    time.Duration
	lastGC time.Time
}

type window struct {
	count int
	reset time.Time
}

func newRateLimiter(max int, per time.Duration) *rateLimiter {
	return &rateLimiter{hits: map[string]*window{}, max: max, per: per, lastGC: time.Now()}
}

// allow records a hit for key and reports whether it's within the window budget.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.gc(now)
	w := l.hits[key]
	if w == nil || now.After(w.reset) {
		l.hits[key] = &window{count: 1, reset: now.Add(l.per)}
		return true
	}
	if w.count >= l.max {
		return false
	}
	w.count++
	return true
}

// gc drops expired windows so distinct-IP churn can't grow the map unbounded.
// Cheap and infrequent; caller holds the lock.
func (l *rateLimiter) gc(now time.Time) {
	if now.Sub(l.lastGC) < l.per {
		return
	}
	for k, w := range l.hits {
		if now.After(w.reset) {
			delete(l.hits, k)
		}
	}
	l.lastGC = now
}

// clientIP is the throttle key: the peer's IP. Behind a reverse proxy every
// request shares the proxy's IP, so login throttling becomes global — a safe
// (fail-closed) coarsening. ponytail: parse a trusted X-Forwarded-For here only
// once a known proxy is in front, since XFF is client-spoofable otherwise.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
