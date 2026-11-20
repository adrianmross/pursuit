package httpapi

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// rateRule is a named limit on one kind of request, counted per subject
// (usually the client IP) in the database so it holds across instances.
type rateRule struct {
	name   string
	limit  int
	window time.Duration
}

var (
	ruleMCP = rateRule{"mcp", 120, time.Minute} // MCP calls, per token
)

// allow counts one request by subject against rule.
func (s *Server) allow(ctx context.Context, rule rateRule, subject string) bool {
	return s.Svc.RateAllow(ctx, rule.name, subject, rule.limit, rule.window)
}

// clientIP keys the rate limiters and is where a sign-in is recorded as
// coming from. X-Forwarded-For is trusted only on Vercel, whose edge sets
// it; a directly exposed server would otherwise let every request pick its
// own key.
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
