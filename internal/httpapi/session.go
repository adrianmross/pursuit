package httpapi

import (
	"net/http"
	"strings"

	"github.com/adrianmross/appkit/account"
	"github.com/adrianmross/appkit/session"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/service"
)

// sessionCookie holds the dashboard's token where page scripts can't read
// it (httpOnly), instead of localStorage. Requests with a bearer token
// never look at it. See session.Cookie for the __Host- prefix over HTTPS.
// appkit sets and clears it (App + "_session"); authorize reads it here.
const sessionCookie = "pursuit_session"

var cookie = session.Cookie{Name: sessionCookie, MaxAge: service.LoginMaxTTL}

// sameOrigin reports whether a browser sent r from this origin.
var sameOrigin = session.SameOrigin

// publicBase is the origin the caller reached us on.
var publicBase = account.PublicBase

func crossSiteRefused() error {
	return &domain.Error{Code: "forbidden", Message: "cross-origin request refused", Hint: "send Authorization: Bearer <token>, or make the change from the dashboard itself", Status: 403}
}

// Signing in to the dashboard is appkit's (account.Accounts): what the
// sign-in page may offer, the session cookie (a pasted token is stored as
// is; a Better Auth JWT lives about 15 minutes, so it is exchanged for a
// login token), sign-out, an OIDC provider's redirect round trip, and the
// demo sandbox.
var sessionRoutes = []string{
	"GET /v1/auth/providers",
	"POST /v1/auth/session",
	"DELETE /v1/auth/session",
	"POST /v1/demo",
}

// Who is calling, their API tokens and the audit log are appkit's too.
var accountRoutes = []string{
	"GET /v1/me",
	"POST /v1/tokens",
	"GET /v1/tokens",
	"DELETE /v1/tokens/{id}",
	"GET /v1/audit",
}

func (s *Server) registerSession(mux *http.ServeMux) {
	for _, pattern := range append(sessionRoutes, accountRoutes...) {
		mux.HandleFunc(pattern, s.accounts)
	}
	mux.HandleFunc("GET /v1/auth/{provider}/start", s.oidc)
	mux.HandleFunc("GET /v1/auth/{provider}/callback", s.oidc)
}

// oidc answers for appkit when no provider, or not this one, is configured:
// appkit only has routes for the ones that are, and calls an unknown name a 404.
func (s *Server) oidc(w http.ResponseWriter, r *http.Request) {
	if s.OIDC == nil {
		writeErr(w, domain.NotFound("provider"))
		return
	}
	name := r.PathValue("provider")
	for _, p := range s.OIDC.Providers {
		if p.Name == name {
			s.accounts(w, r)
			return
		}
	}
	if strings.HasSuffix(r.URL.Path, "/start") {
		writeErr(w, domain.Validation("unknown provider %q", name))
		return
	}
	writeErr(w, domain.Unauthorized())
}

// acct is appkit's account layer as this server is configured.
func (s *Server) acct() *account.Accounts {
	a := s.Svc.Accounts()
	a.ClientIP, a.Log = clientIP, s.log()
	a.MultiTenant, a.SignupsClosed, a.HideTokenLogin = s.multiTenant(), s.SignupsClosed, s.HideTokenLogin
	a.OIDC = s.OIDC
	if s.multiTenant() {
		// Only multi-tenant mode takes Better Auth's word for who someone is.
		a.BetterAuth = s.BetterAuth
	}
	if !s.DemoMode {
		a.Demo = nil
	}
	return a
}

// accounts serves r from appkit. appkit registers every account route at
// once, and one (/v1/events) is Pursuit's own, so its routes go on a mux of
// their own and only the patterns registered with this handler are sent
// there.
func (s *Server) accounts(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	s.acct().Routes(mux)
	if strings.HasPrefix(r.URL.Path, "/v1/auth/") {
		// Nothing these answer with may be cached, a refusal for too many requests included.
		w.Header().Set("Cache-Control", "no-store")
	}
	mux.ServeHTTP(w, r)
}
