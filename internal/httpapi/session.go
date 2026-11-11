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
// never look at it. Over HTTPS the cookie is __Host- prefixed, which browsers
// only accept with Secure, Path=/ and no Domain, so a sibling subdomain can't
// plant one. Plain-http `pursuit serve` can't set Secure, so it keeps the bare name.
const sessionCookie = "pursuit_session"

func sessionCookieName(r *http.Request) string {
	if isHTTPS(r) {
		return "__Host-" + sessionCookie
	}
	return sessionCookie
}

// sessionCookieValue is the request's session token, "" without one.
func sessionCookieValue(r *http.Request) string {
	if c, err := r.Cookie(sessionCookieName(r)); err == nil {
		return c.Value
	}
	return ""
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName(r),
		Value:    token,
		Path:     "/",
		MaxAge:   int(service.LoginMaxTTL.Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie also clears the pre-prefix name, so a browser signed in
// before the rename doesn't keep a dead cookie.
func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{sessionCookie, "__Host-" + sessionCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode})
	}
}

// retireSession replaces the browser's current dashboard session with a new
// sign-in: a re-login (or step-up) must not leave the old token alive beside
// the new one. Only a login token of the same user is revoked.
func (s *Server) retireSession(r *http.Request, userID string) {
	old := sessionCookieValue(r)
	if !strings.HasPrefix(old, "pst_") {
		return
	}
	if actor, err := s.Svc.Authenticate(r.Context(), old); err == nil && actor.Name == service.LoginTokenName && actor.UserID == userID {
		if _, err := s.Svc.RevokeToken(domain.WithActor(r.Context(), actor), actor.TokenID); err != nil {
			s.log().Warn("retire previous session", "err", err.Error())
		}
	}
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
		if token, err = s.Svc.LoginIdentity(originCtx(r), identity.Email, identity.Name, s.signupsOpen()); err != nil {
			writeErr(w, s.signupsPaused(err))
			return
		}
	}
	actor, err := s.authenticate(r.Context(), token)
	if err != nil {
		writeErr(w, err)
		return
	}
	if token != sessionCookieValue(r) {
		s.retireSession(r, actor.UserID)
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
	if c := sessionCookieValue(r); strings.HasPrefix(c, "pst_") {
		if actor, err := s.Svc.Authenticate(r.Context(), c); err == nil && actor.Name == service.LoginTokenName {
			if _, err := s.Svc.RevokeToken(domain.WithActor(r.Context(), actor), actor.TokenID); err != nil {
				s.log().Warn("revoke login token on sign-out", "err", err.Error())
			}
		}
	}
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}
