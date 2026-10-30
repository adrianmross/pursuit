package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
)

const demoCookieName = "pursuit_demo"

// demoLimiter caps how many new demo tenants a single client IP can mint
// per window — minting is the only expensive/abusable part of demo mode
// (it writes a user, a token, and the seed data).
//
// ponytail: an in-memory map, so the limit is per function instance, not
// global — fine for a preview deployment; move to a shared store (Redis,
// a DB row) if demo mode ever runs where that matters.
type demoLimiter struct {
	mu     sync.Mutex
	window time.Duration
	limit  int
	seen   map[string][]time.Time
}

func newDemoLimiter() *demoLimiter {
	return &demoLimiter{window: time.Minute, limit: 5, seen: map[string][]time.Time{}}
}

func (l *demoLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	kept := l.seen[key][:0]
	for _, t := range l.seen[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.limit {
		l.seen[key] = kept
		return false
	}
	l.seen[key] = append(kept, now)
	return true
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// demoActor authenticates an anonymous preview visitor from their session
// cookie, minting a fresh throwaway tenant (rate-limited per IP) when the
// cookie is missing or no longer valid. Nothing here is persisted beyond
// that cookie: a new tab with no cookie is a brand-new, empty tenant.
func (s *Server) demoActor(w http.ResponseWriter, r *http.Request) (domain.Actor, error) {
	ctx := r.Context()
	if c, err := r.Cookie(demoCookieName); err == nil && c.Value != "" {
		if actor, err := s.Svc.Authenticate(ctx, c.Value); err == nil {
			actor.Demo = true
			return actor, nil
		}
	}
	if !s.demoLimiterFor().allow(clientIP(r)) {
		return domain.Actor{}, domain.RateLimited("too many demo sessions from this address; wait a minute and reload")
	}
	secret, actor, err := s.Svc.CreateDemoSession(ctx)
	if err != nil {
		return domain.Actor{}, err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     demoCookieName,
		Value:    secret,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		// No MaxAge/Expires: a session cookie, gone when the browser
		// decides the session is over. Matches "no persistence beyond
		// the tab/session" — this is a demo, not an account.
	})
	return actor, nil
}
