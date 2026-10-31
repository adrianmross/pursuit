package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
)

// Passkey routes. Registration and management need a signed-in human
// (wrap authenticates /v1/passkeys like any /v1 route); login is public and
// rate limited per IP. Every POST is same-origin JSON, like /v1/auth/session.
//
//   POST   /v1/passkeys/register/begin   -> {publicKey: ...} for navigator.credentials.create
//   POST   /v1/passkeys/register/finish  {name, credential}
//   GET    /v1/passkeys
//   DELETE /v1/passkeys/{id}
//   POST   /v1/auth/passkey/login/begin  -> {publicKey: ...} for navigator.credentials.get
//   POST   /v1/auth/passkey/login/finish credential JSON; sets pursuit_session

func (s *Server) registerPasskeys(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/passkeys/register/begin", s.passkeyRegisterBegin)
	mux.HandleFunc("POST /v1/passkeys/register/finish", s.passkeyRegisterFinish)
	mux.HandleFunc("GET /v1/passkeys", s.listPasskeys)
	mux.HandleFunc("DELETE /v1/passkeys/{id}", s.deletePasskey)
	mux.HandleFunc("POST /v1/auth/passkey/login/begin", s.passkeyLoginBegin)
	mux.HandleFunc("POST /v1/auth/passkey/login/finish", s.passkeyLoginFinish)
}

// passkeyLimiterFor caps public passkey login calls (begin and finish) per client IP.
func (s *Server) passkeyLimiterFor() *demoLimiter {
	s.passkeyLimiterOnce.Do(func() {
		s.passkeyLimiterVal = &demoLimiter{window: time.Minute, limit: 20, seen: map[string][]time.Time{}}
	})
	return s.passkeyLimiterVal
}

// passkeyBody reads a same-origin JSON body.
func (s *Server) passkeyBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if err := sameOriginJSON(r); err != nil {
		writeErr(w, err)
		return nil, false
	}
	return s.read(w, r)
}

func (s *Server) passkeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.passkeyBody(w, r); !ok {
		return
	}
	out, err := s.Svc.BeginPasskeyRegistration(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, out)
}

func (s *Server) passkeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	body, ok := s.passkeyBody(w, r)
	if !ok {
		return
	}
	var in struct {
		Name       string          `json:"name"`
		Credential json.RawMessage `json:"credential"`
	}
	if err := decode(body, &in); err != nil {
		writeErr(w, err)
		return
	}
	out, err := s.Svc.FinishPasskeyRegistration(r.Context(), in.Name, in.Credential)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, out)
}

func (s *Server) listPasskeys(w http.ResponseWriter, r *http.Request) {
	pageSlice(w, r, nil, s.Svc.ListPasskeys)
}

func (s *Server) deletePasskey(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	id := r.PathValue("id")
	s.mutate(w, r, 200, nil, nil, func(ctx context.Context) (any, error) {
		return s.Svc.DeletePasskey(ctx, id)
	})
}

func (s *Server) passkeyLimited(w http.ResponseWriter, r *http.Request) bool {
	if s.passkeyLimiterFor().allow(clientIP(r)) {
		return false
	}
	w.Header().Set("Retry-After", "60")
	writeErr(w, domain.RateLimited("too many passkey sign-ins from this address; wait a minute and try again"))
	return true
}

func (s *Server) passkeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	if s.passkeyLimited(w, r) {
		return
	}
	if _, ok := s.passkeyBody(w, r); !ok {
		return
	}
	out, err := s.Svc.BeginPasskeyLogin(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, out)
}

// passkeyLoginFinish signs the browser in like the OIDC callback: the
// login token goes straight into the httpOnly cookie, never to the page.
func (s *Server) passkeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	if s.passkeyLimited(w, r) {
		return
	}
	body, ok := s.passkeyBody(w, r)
	if !ok {
		return
	}
	secret, err := s.Svc.FinishPasskeyLogin(r.Context(), body)
	if err != nil {
		writeErr(w, err)
		return
	}
	actor, err := s.Svc.Authenticate(r.Context(), secret)
	if err != nil {
		writeErr(w, err)
		return
	}
	setSessionCookie(w, r, secret)
	s.me(w, r.WithContext(domain.WithActor(r.Context(), actor)))
}
