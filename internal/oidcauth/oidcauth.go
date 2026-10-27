// Package oidcauth turns a verified OIDC identity into a Pursuit login.
// It only runs the OAuth2/OIDC code-exchange dance; the result is handed
// to Service.LoginIdentity, which is the one place that decides whether
// that identity is allowed to log in. There is no separate session system:
// the dashboard receives a normal API token, same as pasting one by hand.
package oidcauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Spec configures one provider. Name is the URL slug ("google", "custom").
type Spec struct {
	Name         string
	Label        string
	IssuerURL    string
	ClientID     string
	ClientSecret string
}

type provider struct {
	label    string
	verifier *oidc.IDTokenVerifier
	oauth2   oauth2.Config
}

// Manager holds the configured providers and signs the short-lived
// state cookie used for the redirect round trip.
type Manager struct {
	providers map[string]*provider
	cookieKey []byte
}

// New builds a Manager. redirectURL is a func(name) so the caller can
// compute it per-request (scheme/host of the actual deployment) instead
// of hardcoding a public base URL.
func New(ctx context.Context, cookieKey []byte, specs []Spec, redirectURL func(name string) string) (*Manager, error) {
	m := &Manager{providers: map[string]*provider{}, cookieKey: cookieKey}
	for _, sp := range specs {
		p, err := oidc.NewProvider(ctx, sp.IssuerURL)
		if err != nil {
			return nil, fmt.Errorf("oidc provider %s: %w", sp.Name, err)
		}
		label := sp.Label
		if label == "" {
			label = sp.Name
		}
		m.providers[sp.Name] = &provider{
			label:    label,
			verifier: p.Verifier(&oidc.Config{ClientID: sp.ClientID}),
			oauth2: oauth2.Config{
				ClientID:     sp.ClientID,
				ClientSecret: sp.ClientSecret,
				Endpoint:     p.Endpoint(),
				RedirectURL:  redirectURL(sp.Name),
				Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
			},
		}
	}
	return m, nil
}

// Enabled reports whether any provider is configured.
func (m *Manager) Enabled() bool { return len(m.providers) > 0 }

// Providers lists the configured login buttons for the dashboard.
func (m *Manager) Providers() []struct{ Name, Label string } {
	out := make([]struct{ Name, Label string }, 0, len(m.providers))
	for name, p := range m.providers {
		out = append(out, struct{ Name, Label string }{name, p.label})
	}
	return out
}

const stateCookie = "pursuit_oidc_state"

type statePayload struct {
	Provider string `json:"p"`
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Exp      int64  `json:"e"`
}

// Start redirects to the provider's consent screen, after setting a
// signed, short-lived cookie carrying the state and nonce it will
// check on the callback.
func (m *Manager) Start(w http.ResponseWriter, r *http.Request, name string) error {
	p, ok := m.providers[name]
	if !ok {
		return fmt.Errorf("unknown provider %q", name)
	}
	state, err := randomToken()
	if err != nil {
		return err
	}
	nonce, err := randomToken()
	if err != nil {
		return err
	}
	sp := statePayload{Provider: name, State: state, Nonce: nonce, Exp: time.Now().Add(5 * time.Minute).Unix()}
	cookie, err := m.sign(sp)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: stateCookie, Value: cookie, Path: "/v1/auth", HttpOnly: true,
		Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: 300,
	})
	http.Redirect(w, r, p.oauth2.AuthCodeURL(state, oidc.Nonce(nonce)), http.StatusFound)
	return nil
}

// Identity is the verified claims returned by a provider.
type Identity struct {
	Email         string
	EmailVerified bool
	Name          string
}

// Callback validates the state cookie, exchanges the code, and verifies
// the ID token (signature, issuer, audience, nonce, expiry).
func (m *Manager) Callback(ctx context.Context, w http.ResponseWriter, r *http.Request, name string) (Identity, error) {
	p, ok := m.providers[name]
	if !ok {
		return Identity{}, fmt.Errorf("unknown provider %q", name)
	}
	raw, err := r.Cookie(stateCookie)
	if err != nil {
		return Identity{}, fmt.Errorf("missing state cookie")
	}
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Value: "", Path: "/v1/auth", MaxAge: -1})
	sp, err := m.verify(raw.Value)
	if err != nil {
		return Identity{}, err
	}
	if sp.Provider != name || sp.Exp < time.Now().Unix() {
		return Identity{}, fmt.Errorf("state expired or mismatched")
	}
	if r.URL.Query().Get("state") != sp.State {
		return Identity{}, fmt.Errorf("state mismatch")
	}
	tok, err := p.oauth2.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		return Identity{}, err
	}
	rawIDToken, ok := tok.Extra("id_token").(string)
	if !ok {
		return Identity{}, fmt.Errorf("no id_token in response")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, err
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Nonce         string `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, err
	}
	if claims.Nonce != sp.Nonce {
		return Identity{}, fmt.Errorf("nonce mismatch")
	}
	return Identity{Email: strings.TrimSpace(claims.Email), EmailVerified: claims.EmailVerified, Name: claims.Name}, nil
}

func (m *Manager) sign(sp statePayload) (string, error) {
	b, err := json.Marshal(sp)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, m.cookieKey)
	mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return body + "." + sig, nil
}

func (m *Manager) verify(cookie string) (statePayload, error) {
	parts := strings.SplitN(cookie, ".", 2)
	if len(parts) != 2 {
		return statePayload{}, fmt.Errorf("malformed state cookie")
	}
	mac := hmac.New(sha256.New, m.cookieKey)
	mac.Write([]byte(parts[0]))
	wantSig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return statePayload{}, fmt.Errorf("malformed state cookie")
	}
	if !hmac.Equal(mac.Sum(nil), wantSig) {
		return statePayload{}, fmt.Errorf("invalid state cookie")
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return statePayload{}, fmt.Errorf("malformed state cookie")
	}
	var sp statePayload
	if err := json.Unmarshal(b, &sp); err != nil {
		return statePayload{}, fmt.Errorf("malformed state cookie")
	}
	return sp, nil
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
