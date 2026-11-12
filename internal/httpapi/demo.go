package httpapi

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/service"
)

// rateRule is a named limit on one kind of request, counted per subject
// (usually the client IP) in the database so it holds across instances.
type rateRule struct {
	name   string
	limit  int
	window time.Duration
}

var (
	ruleDemo     = rateRule{"demo", 5, time.Minute}      // minting a throwaway tenant writes a user, a token and seed data
	ruleDevice   = rateRule{"device", 10, time.Minute}   // device login starts
	ruleRegister = rateRule{"register", 20, time.Minute} // OAuth client registrations
	rulePasskey  = rateRule{"passkey", 20, time.Minute}  // public passkey login calls
	ruleToken    = rateRule{"token", 60, time.Minute}    // OAuth token endpoint, per IP
	ruleMCP      = rateRule{"mcp", 120, time.Minute}     // MCP calls, per token
)

// allow counts one request by subject against rule.
func (s *Server) allow(ctx context.Context, rule rateRule, subject string) bool {
	return s.Svc.RateAllow(ctx, rule.name, subject, rule.limit, rule.window)
}

// clientIP keys the rate limiters. X-Forwarded-For is trusted only on
// Vercel, whose edge sets it; a directly exposed server would otherwise let
// every request pick its own key.
// originCtx records where a sign-in came from for the session list.
func originCtx(r *http.Request) context.Context {
	return service.WithOrigin(r.Context(), service.Origin{IP: clientIP(r), UserAgent: r.UserAgent()})
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" && os.Getenv("VERCEL") != "" {
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
	if !s.allow(r.Context(), ruleDemo, clientIP(r)) {
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
