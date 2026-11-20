package httpapi

import (
	"net/http"

	"github.com/adrianmross/appkit/httpx"

	"github.com/pursuit/pursuit/internal/domain"
)

// Passkeys are appkit's (account.Passkeys). Registration and management need
// a signed-in human (wrap authenticates /v1/passkeys like any /v1 route);
// login is public and rate limited per IP. Every POST is same-origin JSON,
// like /v1/auth/session.
//
//   POST   /v1/passkeys/register/begin   -> {publicKey: ...} for navigator.credentials.create
//   POST   /v1/passkeys/register/finish  {name, credential}
//   GET    /v1/passkeys
//   DELETE /v1/passkeys/{id}
//   POST   /v1/auth/passkey/login/begin  -> {publicKey: ...} for navigator.credentials.get
//   POST   /v1/auth/passkey/login/finish credential JSON; sets pursuit_session
//
// The list and the end of a sign-in call appkit from handlers here: the list
// pages like Pursuit's other short lists, and the sign-in ends in Pursuit's
// own session code, which audits the session it replaces.
// ponytail: both go to appkit's routes once its list takes ?limit and
// ?cursor and its sign-in audits the session it retires.

var passkeyRoutes = []string{
	"POST /v1/passkeys/register/begin",
	"POST /v1/passkeys/register/finish",
	"DELETE /v1/passkeys/{id}",
	"POST /v1/auth/passkey/login/begin",
}

func (s *Server) registerPasskeys(mux *http.ServeMux) {
	for _, pattern := range passkeyRoutes {
		mux.HandleFunc(pattern, s.passkeysOn(s.accounts))
	}
	mux.HandleFunc("GET /v1/passkeys", s.passkeysOn(s.listPasskeys))
	mux.HandleFunc("POST /v1/auth/passkey/login/finish", s.passkeysOn(s.passkeyLoginFinish))
}

// passkeysOn answers for h while passkeys are off.
func (s *Server) passkeysOn(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Svc.Passkeys == nil {
			writeErr(w, &domain.Error{Code: "not_found", Message: "passkeys are not enabled on this server", Hint: "the owner sets PURSUIT_WEBAUTHN_RP_ID to turn them on", Status: 404})
			return
		}
		h(w, r)
	}
}

func (s *Server) listPasskeys(w http.ResponseWriter, r *http.Request) {
	httpx.PageSlice(w, r, nil, s.Svc.Accounts().ListPasskeys)
}

// passkeyLoginFinish signs the browser in like the OIDC callback: the
// login token goes straight into the httpOnly cookie, never to the page.
func (s *Server) passkeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	// Counted with the begin calls appkit limits: same rule, same key.
	if !s.allow(r.Context(), rulePasskey, clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		writeErr(w, domain.RateLimited("too many sign-in attempts from this address; wait a minute and try again"))
		return
	}
	if err := sameOriginJSON(r); err != nil {
		writeErr(w, err)
		return
	}
	body, ok := s.read(w, r)
	if !ok {
		return
	}
	secret, err := s.Svc.Accounts().FinishPasskeyLogin(originCtx(r), body)
	if err != nil {
		writeErr(w, err)
		return
	}
	actor, err := s.Svc.Authenticate(r.Context(), secret)
	if err != nil {
		writeErr(w, err)
		return
	}
	s.retireSession(r, actor.UserID)
	setSessionCookie(w, r, secret)
	s.me(w, r.WithContext(domain.WithActor(r.Context(), actor)))
}
