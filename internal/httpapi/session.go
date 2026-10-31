package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/pursuit/pursuit/internal/domain"
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
// the session cookie and answers like GET /v1/me. Same-origin JSON only, so
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
	actor, err := s.authenticate(r.Context(), token)
	if err != nil {
		writeErr(w, err)
		return
	}
	setSessionCookie(w, r, token)
	s.me(w, r.WithContext(domain.WithActor(r.Context(), actor)))
}

// deleteSession signs the dashboard out. It clears the cookie only; the
// token itself stays valid (it may be one the CLI also uses).
func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeErr(w, crossSiteRefused())
		return
	}
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}
