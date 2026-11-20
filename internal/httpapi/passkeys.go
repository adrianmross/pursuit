package httpapi

import (
	"net/http"

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

var passkeyRoutes = []string{
	"POST /v1/passkeys/register/begin",
	"POST /v1/passkeys/register/finish",
	"GET /v1/passkeys",
	"DELETE /v1/passkeys/{id}",
	"POST /v1/auth/passkey/login/begin",
	"POST /v1/auth/passkey/login/finish",
}

func (s *Server) registerPasskeys(mux *http.ServeMux) {
	for _, pattern := range passkeyRoutes {
		mux.HandleFunc(pattern, s.passkeysOn(s.accounts))
	}
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
