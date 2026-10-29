// Package betterauth verifies JWTs minted by a Neon Auth (Better Auth)
// instance. Unlike internal/oidcauth, this is not a redirect-driven login
// flow: the React dashboard signs the user in directly against Better
// Auth's own client SDK, then sends its JWT to us as a bearer token. Our
// job is just to verify it (signature, issuer, audience) via the JWKS
// endpoint and read the email claim.
package betterauth

import (
	"context"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

type Verifier struct {
	verifier *oidc.IDTokenVerifier
}

// New builds a Verifier against baseURL's JWKS endpoint. issuer defaults
// to baseURL's origin (Better Auth's "iss" claim is the bare scheme+host,
// not the app-specific base path JWKS/sign-in live under); audience,
// when empty, skips the audience check (Better Auth's JWT plugin
// audience claim isn't publicly documented).
func New(ctx context.Context, baseURL, issuer, audience string) *Verifier {
	baseURL = strings.TrimRight(baseURL, "/")
	if issuer == "" {
		issuer = Origin(baseURL)
	}
	keySet := oidc.NewRemoteKeySet(ctx, baseURL+"/.well-known/jwks.json")
	// Better Auth's JWT plugin signs with EdDSA (Ed25519), not go-oidc's
	// RS256-only default.
	cfg := &oidc.Config{SkipClientIDCheck: audience == "", ClientID: audience, SupportedSigningAlgs: []string{"EdDSA"}}
	return &Verifier{verifier: oidc.NewVerifier(issuer, keySet, cfg)}
}

// Origin returns rawURL's bare scheme+host, falling back to rawURL
// itself if it doesn't parse. Used both as the default JWT issuer and
// as the CSP connect-src entry the browser's client SDK needs.
func Origin(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil && u.Scheme != "" && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return rawURL
}

type Identity struct {
	Email         string
	EmailVerified bool
	Name          string
}

// Verify checks the JWT's signature, issuer, and (if configured)
// audience, then extracts the identity claims.
func (v *Verifier) Verify(ctx context.Context, rawToken string) (Identity, error) {
	tok, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return Identity{}, err
	}
	var claims map[string]any
	if err := tok.Claims(&claims); err != nil {
		return Identity{}, err
	}
	email, _ := claims["email"].(string)
	verified, _ := claims["email_verified"].(bool)
	if v2, ok := claims["emailVerified"].(bool); ok {
		verified = verified || v2
	}
	name, _ := claims["name"].(string)
	return Identity{Email: strings.TrimSpace(email), EmailVerified: verified, Name: name}, nil
}
