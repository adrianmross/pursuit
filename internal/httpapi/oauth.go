package httpapi

import (
	"net/http"
	"strings"
)

// The OAuth server is appkit's (account.OAuth): `pursuit auth login` by
// loopback or device code, apps that register themselves, refresh tokens,
// revocation and the discovery documents. The authorize URL a client opens
// is the dashboard page /authorize, which signs the human in and calls
// POST /v1/oauth/authorize.
//
// appkit registers every account route at once, and most are still
// Pursuit's own, so its routes go on a mux of their own and only these
// patterns are sent there.
var oauthRoutes = []string{
	"POST /v1/auth/device_authorization",
	"POST /v1/auth/token",
	"POST /v1/auth/revoke",
	"POST /v1/auth/register",
	"GET /v1/oauth/authorize",
	"POST /v1/oauth/authorize",
	"GET /v1/oauth/device",
	"POST /v1/oauth/device/approve",
	"GET /.well-known/oauth-authorization-server",
	"GET /.well-known/oauth-protected-resource",
	"GET /.well-known/oauth-protected-resource/{path...}",
}

func (s *Server) registerOAuth(mux *http.ServeMux) {
	for _, pattern := range oauthRoutes {
		mux.HandleFunc(pattern, s.accounts)
	}
	s.registerMCP(mux)
}

func (s *Server) accounts(w http.ResponseWriter, r *http.Request) {
	a := s.Svc.Accounts()
	a.ClientIP, a.Log = clientIP, s.log()
	mux := http.NewServeMux()
	a.Routes(mux)
	if strings.HasPrefix(r.URL.Path, "/v1/auth/") {
		// Nothing these answer with may be cached, a refusal for too many requests included.
		w.Header().Set("Cache-Control", "no-store")
	}
	mux.ServeHTTP(w, r)
}

// oauthIssuer is the configured public URL, else the origin of the request.
func (s *Server) oauthIssuer(r *http.Request) string {
	return s.Svc.BaseURL(publicBase(r))
}
