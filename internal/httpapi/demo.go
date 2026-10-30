package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
)

// demoLimiter caps how many new demo tenants a single client IP can mint
// per window — minting is the only expensive/abusable part of demo mode
// (it writes a user, a token, and the seed data).
//
// ponytail: an in-memory map, so the limit is per function instance, not
// global — fine for how small-scale this is; move to a shared store
// (Redis, a DB row) if that ever matters.
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

// startDemo is the "try it without an account" entry point: the sign-in
// page offers it to a signed-out visitor, and Settings offers it to
// anyone, at any time, to replay the tutorial in a brand-new sandbox
// (minting again just abandons the old one — nothing to clean up client
// side). It hands back a token exactly like any other sign-in; nothing
// about later requests is special-cased beyond the Actor.Demo flag
// (see Service.Authenticate), which withholds a couple of
// outbound-HTTP-triggering scopes.
func (s *Server) startDemo(w http.ResponseWriter, r *http.Request) {
	if !s.DemoMode {
		writeErr(w, domain.NotFound("demo"))
		return
	}
	if !s.demoLimiterFor().allow(clientIP(r)) {
		writeErr(w, domain.RateLimited("too many demo sessions from this address; wait a minute and try again"))
		return
	}
	secret, actor, err := s.Svc.CreateDemoSession(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": secret, "user_id": actor.UserID})
}
