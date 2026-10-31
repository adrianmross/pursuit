package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/service"
)

// sessionCookie holds the dashboard's token where page scripts can't read
// it (httpOnly), instead of localStorage. Requests with a bearer token
// never look at it.
const sessionCookie = "pursuit_session"

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 30,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode})
}

// isHTTPS is true behind a TLS-terminating proxy (Vercel) too, so a local
// `pursuit serve` on plain http still gets a cookie it can send back.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func unsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func crossSiteRefused() error {
	return &domain.Error{Code: "forbidden", Message: "cross-origin request refused", Hint: "send Authorization: Bearer <token>, or make the change from the dashboard itself", Status: 403}
}

// createSession checks a token the way any request would, then stores it in
// the session cookie and answers like GET /v1/me. A Better Auth JWT lives
// about 15 minutes, so it is exchanged for a Pursuit login token (as the
// OIDC callback does) instead of being stored. Same-origin JSON only, so
// another site can't sign the browser into an account it chose.
func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	if err := sameOriginJSON(r); err != nil {
		writeErr(w, err)
		return
	}
	var in struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&in); err != nil || strings.TrimSpace(in.Token) == "" {
		writeErr(w, domain.Validation(`body must be {"token": "..."}`))
		return
	}
	token := strings.TrimSpace(in.Token)
	if s.isBetterAuthJWT(token) {
		identity, err := s.verifyBetterAuth(r.Context(), token)
		if err != nil {
			writeErr(w, err)
			return
		}
		if token, err = s.Svc.LoginIdentity(r.Context(), identity.Email, identity.Name, s.signupsOpen()); err != nil {
			writeErr(w, s.signupsPaused(err))
			return
		}
	}
	actor, err := s.authenticate(r.Context(), token)
	if err != nil {
		writeErr(w, err)
		return
	}
	setSessionCookie(w, r, token)
	s.me(w, r.WithContext(domain.WithActor(r.Context(), actor)))
}

// deleteSession signs the dashboard out. It revokes the cookie's token when
// sign-in minted it (named "login"); any other token stays valid, since the
// CLI may also use it. The cookie is cleared either way.
func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeErr(w, crossSiteRefused())
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil && strings.HasPrefix(c.Value, "pst_") {
		if actor, err := s.Svc.Authenticate(r.Context(), c.Value); err == nil && actor.Name == service.LoginTokenName {
			if _, err := s.Svc.RevokeToken(domain.WithActor(r.Context(), actor), actor.TokenID); err != nil {
				s.log().Warn("revoke login token on sign-out", "err", err.Error())
			}
		}
	}
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}
