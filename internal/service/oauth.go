package service

import (
	"context"
	"crypto/rand"
	"math/big"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/auth"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

// OAuth2 for `pursuit auth login`. Pursuit is its own authorization
// server: a human signed into the dashboard approves, and the CLI gets a
// normal API token. Two grants share one table (sqlstore.OAuthGrant):
// loopback authorization code + PKCE (RFC 8252/7636) and device
// authorization (RFC 8628).

const (
	OAuthCodeTTL        = 60 * time.Second
	OAuthDeviceTTL      = 10 * time.Minute
	OAuthDeviceInterval = 5
	DefaultCLIClientID  = "pursuit-cli"
	DeviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"
	oauthTokenName      = "cli"
	userCodeAlphabet    = "BCDFGHJKLMNPQRSTVWXZ" // no vowels, no look-alikes
)

// OAuthError is an RFC 6749 section 5.2 error: {"error": Code, "error_description": Description}.
type OAuthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

func (e *OAuthError) Error() string { return e.Code + ": " + e.Description }

func oauthErr(code, desc string) *OAuthError { return &OAuthError{Code: code, Description: desc} }

// OAuthToken is the token endpoint's success body.
type OAuthToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// ParseOAuthScopes accepts space- or comma-separated scopes, each exactly
// "read" or "write" (case-sensitive). Empty means read and write. OAuth
// never grants approve: approving stays a dashboard action, or an
// explicit PURSUIT_TOKEN a human minted on purpose.
func ParseOAuthScopes(raw string) ([]string, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ' ' || r == ',' })
	if len(parts) == 0 {
		parts = []string{"read", "write"}
	}
	for _, p := range parts {
		if p != "read" && p != "write" {
			return nil, oauthErr("invalid_scope", "scope is read and/or write; "+quoteScope(p)+" cannot be granted through OAuth")
		}
	}
	return auth.NormalizeScopes(parts)
}

func quoteScope(s string) string {
	if len(s) > 32 {
		s = s[:32]
	}
	return `"` + strings.Map(func(r rune) rune {
		if r < 0x20 || r == '"' {
			return '?'
		}
		return r
	}, s) + `"`
}

// invalidScope is ParseOAuthScopes' error as a dashboard API error.
func invalidScope(err error) error {
	msg := err.Error()
	if oe, ok := err.(*OAuthError); ok {
		msg = oe.Description
	}
	return &domain.Error{Code: "invalid_scope", Message: msg, Hint: "request read and/or write; approve is never granted to the CLI", Status: 400}
}

const maxClientID = 64

func normalizeClientID(c string) (string, bool) {
	if c == "" {
		return DefaultCLIClientID, true
	}
	return c, len(c) <= maxClientID
}

// ValidLoopbackRedirect accepts only http://127.0.0.1:<port>/callback or
// http://[::1]:<port>/callback (RFC 8252 section 7.3). "localhost" is
// refused: it can resolve somewhere else.
func ValidLoopbackRedirect(raw string) error {
	u, err := url.Parse(raw)
	bad := domain.Validation("redirect_uri must be http://127.0.0.1:<port>/callback or http://[::1]:<port>/callback")
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/callback" {
		return bad
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || port == "" || (host != "127.0.0.1" && host != "::1") {
		return bad
	}
	return nil
}

// humanApprover is the actor allowed to approve a CLI login: a signed-in
// dashboard session. Agent tokens never carry approve (AGENTS.md), so
// requiring it keeps an agent from approving its own login.
func (s *Service) humanApprover(ctx context.Context) (domain.Actor, error) {
	a, err := s.actor(ctx, false)
	if err != nil {
		return a, err
	}
	if !a.Can("approve") || a.Demo || a.Proxy {
		return domain.Actor{}, domain.Forbidden("approve").WithHint("approve CLI logins from the dashboard while signed in as yourself")
	}
	return a, nil
}

func checkGrantable(a domain.Actor, scopes []string) error {
	for _, sc := range scopes {
		if !a.Can(sc) {
			return domain.Forbidden(sc).WithHint("you can only grant scopes your own session holds")
		}
	}
	return nil
}

type AuthorizeInput struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	State               string `json:"state"`
	Scope               string `json:"scope"`
	Approve             bool   `json:"approve"`
}

// AuthorizeRequest is a validated loopback request, normalized for the
// consent page to show as-is.
type AuthorizeRequest struct {
	ClientID    string   `json:"client_id"`
	Scopes      []string `json:"scopes"`
	RedirectURI string   `json:"redirect_uri"`
}

func (s *Service) checkAuthorize(ctx context.Context, in AuthorizeInput) (domain.Actor, AuthorizeRequest, error) {
	a, err := s.humanApprover(ctx)
	if err != nil {
		return a, AuthorizeRequest{}, err
	}
	if err := ValidLoopbackRedirect(in.RedirectURI); err != nil {
		return a, AuthorizeRequest{}, err
	}
	if in.CodeChallengeMethod != "S256" {
		return a, AuthorizeRequest{}, domain.Validation("code_challenge_method must be S256")
	}
	if !auth.ValidPKCEValue(in.CodeChallenge) {
		return a, AuthorizeRequest{}, domain.Validation("invalid code_challenge")
	}
	if in.State == "" || len(in.State) > 512 {
		return a, AuthorizeRequest{}, domain.Validation("state is required")
	}
	clientID, ok := normalizeClientID(in.ClientID)
	if !ok {
		return a, AuthorizeRequest{}, domain.Validation("client_id is at most 64 characters")
	}
	scopes, err := ParseOAuthScopes(in.Scope)
	if err != nil {
		return a, AuthorizeRequest{}, invalidScope(err)
	}
	return a, AuthorizeRequest{ClientID: clientID, Scopes: scopes, RedirectURI: in.RedirectURI}, nil
}

// OAuthAuthorizeCheck validates a loopback request without deciding it,
// so the consent page shows the server's reading of client and scopes.
func (s *Service) OAuthAuthorizeCheck(ctx context.Context, in AuthorizeInput) (AuthorizeRequest, error) {
	_, req, err := s.checkAuthorize(ctx, in)
	return req, err
}

// OAuthAuthorize is the dashboard consent step of the loopback flow. It
// validates the request, and on approval stores a single-use code bound to
// the challenge, redirect URI, user, and scopes. It returns where the
// browser goes next: the CLI's loopback callback with code or error.
func (s *Service) OAuthAuthorize(ctx context.Context, in AuthorizeInput) (string, error) {
	a, req, err := s.checkAuthorize(ctx, in)
	if err != nil {
		return "", err
	}
	q := url.Values{"state": {in.State}}
	meta := map[string]any{"flow": "browser", "client_id": req.ClientID, "scopes": req.Scopes}
	if !in.Approve {
		if err := s.audit(ctx, a, "oauth.deny", "oauth_grant", "", "denied a CLI login (browser)", meta); err != nil {
			return "", err
		}
		q.Set("error", "access_denied")
		return in.RedirectURI + "?" + q.Encode(), nil
	}
	if err := checkGrantable(a, req.Scopes); err != nil {
		return "", err
	}
	code, err := auth.RandomURLToken(32)
	if err != nil {
		return "", err
	}
	now := s.now()
	_ = s.DB.DeleteExpiredOAuthGrants(ctx, domain.FormatTime(now.Add(-time.Hour)))
	g := sqlstore.OAuthGrant{
		ID: s.NewID(), Kind: "code", CodeHash: auth.Hash(code), ClientID: req.ClientID, Scopes: req.Scopes,
		RedirectURI: in.RedirectURI, CodeChallenge: in.CodeChallenge, UserID: a.UserID, ApproverTokenID: a.TokenID,
		Status: "approved", ExpiresAt: domain.FormatTime(now.Add(OAuthCodeTTL)), CreatedAt: domain.FormatTime(now),
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.CreateOAuthGrant(ctx, g); err != nil {
			return err
		}
		return s.audit(ctx, a, "oauth.approve", "oauth_grant", g.ID, "approved a CLI login (browser)", meta)
	})
	if err != nil {
		return "", err
	}
	q.Set("code", code)
	return in.RedirectURI + "?" + q.Encode(), nil
}

// OAuthExchangeCode is grant_type=authorization_code. The code is burned
// before anything else is checked, so a wrong verifier also spends it.
func (s *Service) OAuthExchangeCode(ctx context.Context, code, verifier, redirectURI, clientID string) (OAuthToken, error) {
	if code == "" || verifier == "" || redirectURI == "" {
		return OAuthToken{}, oauthErr("invalid_request", "code, code_verifier and redirect_uri are required")
	}
	clientID, ok := normalizeClientID(clientID)
	if !ok {
		return OAuthToken{}, oauthErr("invalid_request", "client_id is at most 64 characters")
	}
	g, err := s.DB.GetOAuthGrantByCode(ctx, "code", auth.Hash(code))
	if domain.IsCode(err, "not_found") {
		return OAuthToken{}, oauthErr("invalid_grant", "unknown authorization code")
	}
	if err != nil {
		return OAuthToken{}, err
	}
	used, err := s.DB.UseOAuthGrant(ctx, g.ID)
	if err != nil {
		return OAuthToken{}, err
	}
	if !used {
		return OAuthToken{}, oauthErr("invalid_grant", "authorization code was already used")
	}
	switch {
	case domain.FormatTime(s.now()) >= g.ExpiresAt:
		return OAuthToken{}, oauthErr("invalid_grant", "authorization code expired")
	case g.ClientID != clientID, g.RedirectURI != redirectURI:
		return OAuthToken{}, oauthErr("invalid_grant", "client_id or redirect_uri does not match the authorization")
	case !auth.ValidPKCEValue(verifier) || !auth.PKCEVerify(g.CodeChallenge, verifier):
		return OAuthToken{}, oauthErr("invalid_grant", "code_verifier does not match code_challenge")
	}
	return s.issueOAuthToken(ctx, g, "browser")
}

// DeviceAuthorization is the RFC 8628 section 3.2 response.
// There is deliberately no verification_uri_complete: the human types the
// code, so a phished link can't carry one in for them.
type DeviceAuthorization struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// OAuthDeviceStart begins a device login. baseURL is the public origin
// the dashboard is served from; the human opens baseURL/device. ip is
// the requester's address, shown to the approver.
func (s *Service) OAuthDeviceStart(ctx context.Context, clientID, scope, baseURL, ip string) (DeviceAuthorization, error) {
	clientID, ok := normalizeClientID(clientID)
	if !ok {
		return DeviceAuthorization{}, oauthErr("invalid_request", "client_id is at most 64 characters")
	}
	scopes, err := ParseOAuthScopes(scope)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	deviceCode, err := auth.RandomURLToken(32)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	userCode, err := newUserCode()
	if err != nil {
		return DeviceAuthorization{}, err
	}
	now := s.now()
	// Opportunistic cleanup instead of a sweeper; failure is harmless.
	_ = s.DB.DeleteExpiredOAuthGrants(ctx, domain.FormatTime(now.Add(-time.Hour)))
	if err := s.DB.CreateOAuthGrant(ctx, sqlstore.OAuthGrant{
		ID: s.NewID(), Kind: "device", CodeHash: auth.Hash(deviceCode), UserCodeHash: auth.Hash(normalizeUserCode(userCode)),
		ClientID: clientID, Scopes: scopes, Status: "pending", Interval: OAuthDeviceInterval, RequestedFrom: ip,
		ExpiresAt: domain.FormatTime(now.Add(OAuthDeviceTTL)), CreatedAt: domain.FormatTime(now),
	}); err != nil {
		return DeviceAuthorization{}, err
	}
	verify := strings.TrimRight(baseURL, "/") + "/device"
	return DeviceAuthorization{
		DeviceCode: deviceCode, UserCode: userCode, VerificationURI: verify,
		ExpiresIn: int(OAuthDeviceTTL.Seconds()), Interval: OAuthDeviceInterval,
	}, nil
}

// DeviceRequest is what the dashboard shows before a human approves.
type DeviceRequest struct {
	UserCode  string   `json:"user_code"`
	ClientID  string   `json:"client_id"`
	Scopes    []string `json:"scopes"`
	Status    string   `json:"status"`
	CreatedAt string   `json:"created_at"`
	ExpiresAt string   `json:"expires_at"`
	// RequestedFrom is the IP that started the login; empty for older grants.
	RequestedFrom string `json:"requested_from"`
}

func (s *Service) pendingDevice(ctx context.Context, userCode string) (sqlstore.OAuthGrant, error) {
	norm := normalizeUserCode(userCode)
	if len(norm) != 8 {
		return sqlstore.OAuthGrant{}, domain.NotFound("device code")
	}
	g, err := s.DB.GetOAuthDeviceByUserCode(ctx, auth.Hash(norm))
	if domain.IsCode(err, "not_found") || (err == nil && (!auth.EqualHash(g.UserCodeHash, norm) || domain.FormatTime(s.now()) >= g.ExpiresAt)) {
		return sqlstore.OAuthGrant{}, domain.NotFound("device code").WithHint("the code is wrong or expired; run pursuit auth login --device again")
	}
	return g, err
}

func (s *Service) OAuthDeviceLookup(ctx context.Context, userCode string) (DeviceRequest, error) {
	if _, err := s.humanApprover(ctx); err != nil {
		return DeviceRequest{}, err
	}
	g, err := s.pendingDevice(ctx, userCode)
	if err != nil {
		return DeviceRequest{}, err
	}
	return DeviceRequest{UserCode: formatUserCode(normalizeUserCode(userCode)), ClientID: g.ClientID, Scopes: g.Scopes, Status: g.Status, CreatedAt: g.CreatedAt, ExpiresAt: g.ExpiresAt, RequestedFrom: g.RequestedFrom}, nil
}

// OAuthDeviceDecide approves or denies a device login. It needs both the
// user code and a signed-in human; the code alone approves nothing.
func (s *Service) OAuthDeviceDecide(ctx context.Context, userCode string, approve bool) (string, error) {
	a, err := s.humanApprover(ctx)
	if err != nil {
		return "", err
	}
	g, err := s.pendingDevice(ctx, userCode)
	if err != nil {
		return "", err
	}
	status := "denied"
	if approve {
		if err := checkGrantable(a, g.Scopes); err != nil {
			return "", err
		}
		status = "approved"
	}
	action := map[string]string{"approved": "oauth.approve", "denied": "oauth.deny"}[status]
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		ok, err := s.DB.DecideOAuthDevice(ctx, g.ID, a.UserID, a.TokenID, status, domain.FormatTime(s.now()))
		if err != nil {
			return err
		}
		if !ok {
			return domain.Conflict("this device login was already decided", "run pursuit auth login --device again")
		}
		return s.audit(ctx, a, action, "oauth_grant", g.ID, status+" a CLI login (device)", map[string]any{"flow": "device", "client_id": g.ClientID, "scopes": g.Scopes})
	})
	if err != nil {
		return "", err
	}
	return status, nil
}

// OAuthExchangeDevice is the device_code grant, polled by the CLI.
func (s *Service) OAuthExchangeDevice(ctx context.Context, deviceCode, clientID string) (OAuthToken, error) {
	if deviceCode == "" {
		return OAuthToken{}, oauthErr("invalid_request", "device_code is required")
	}
	clientID, ok := normalizeClientID(clientID)
	if !ok {
		return OAuthToken{}, oauthErr("invalid_request", "client_id is at most 64 characters")
	}
	g, err := s.DB.GetOAuthGrantByCode(ctx, "device", auth.Hash(deviceCode))
	if domain.IsCode(err, "not_found") || (err == nil && !auth.EqualHash(g.CodeHash, deviceCode)) {
		return OAuthToken{}, oauthErr("invalid_grant", "unknown device code")
	}
	if err != nil {
		return OAuthToken{}, err
	}
	if g.ClientID != clientID {
		return OAuthToken{}, oauthErr("invalid_grant", "client_id does not match the device authorization")
	}
	now := s.now()
	if domain.FormatTime(now) >= g.ExpiresAt {
		return OAuthToken{}, oauthErr("expired_token", "the device code expired; start a new login")
	}
	// RFC 8628 section 3.5: polling faster than interval earns slow_down
	// and a 5 second longer interval. Kept in the row so every replica agrees.
	interval := g.Interval
	if g.LastPolledAt != "" {
		if last, perr := domain.ParseTime(g.LastPolledAt); perr == nil && now.Before(last.Add(time.Duration(g.Interval)*time.Second)) {
			interval += 5
		}
	}
	won, err := s.DB.TouchOAuthPoll(ctx, g.ID, g.LastPolledAt, domain.FormatTime(now), interval)
	if err != nil {
		return OAuthToken{}, err
	}
	if !won || interval != g.Interval {
		return OAuthToken{}, oauthErr("slow_down", "poll less often")
	}
	switch g.Status {
	case "pending":
		return OAuthToken{}, oauthErr("authorization_pending", "waiting for approval in the dashboard")
	case "denied":
		return OAuthToken{}, oauthErr("access_denied", "the login was denied")
	case "approved":
		used, err := s.DB.UseOAuthGrant(ctx, g.ID)
		if err != nil {
			return OAuthToken{}, err
		}
		if used {
			return s.issueOAuthToken(ctx, g, "device")
		}
	}
	return OAuthToken{}, oauthErr("invalid_grant", "the device code was already used")
}

func (s *Service) issueOAuthToken(ctx context.Context, g sqlstore.OAuthGrant, flow string) (OAuthToken, error) {
	secret, prefix, hash, err := auth.Generate()
	if err != nil {
		return OAuthToken{}, err
	}
	tok := domain.Token{ID: s.NewID(), UserID: g.UserID, Name: oauthTokenName, Hash: hash, Prefix: prefix, Scopes: g.Scopes, CreatedAt: s.now()}
	actor := domain.Actor{UserID: g.UserID, TokenID: tok.ID, Name: tok.Name, Scopes: g.Scopes}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.CreateToken(ctx, tok); err != nil {
			return err
		}
		return s.audit(ctx, actor, "token.create", "token", tok.ID, "cli login token ("+flow+")", map[string]any{"prefix": prefix, "scopes": g.Scopes, "client_id": g.ClientID, "grant_id": g.ID, "approver_token_id": g.ApproverTokenID})
	})
	if err != nil {
		return OAuthToken{}, err
	}
	return OAuthToken{AccessToken: secret, TokenType: "Bearer", Scope: strings.Join(g.Scopes, " ")}, nil
}

// OAuthRevoke is RFC 7009: holding the token is the authority to revoke
// it, and an unknown token is not an error.
func (s *Service) OAuthRevoke(ctx context.Context, secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return oauthErr("invalid_request", "token is required")
	}
	tok, err := s.DB.GetTokenByHash(ctx, auth.Hash(secret))
	if domain.IsCode(err, "not_found") || (err == nil && tok.RevokedAt != nil) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.DB.WithTx(ctx, func(ctx context.Context) error {
		changed, err := s.DB.RevokeToken(ctx, tok.UserID, tok.ID, domain.FormatTime(s.now()))
		if err != nil || !changed {
			return err
		}
		actor := domain.Actor{UserID: tok.UserID, TokenID: tok.ID, Name: tok.Name, Scopes: tok.Scopes}
		return s.audit(ctx, actor, "token.revoke", "token", tok.ID, "revoked token "+tok.Name+" (logout)", nil)
	})
}

func newUserCode() (string, error) {
	var b strings.Builder
	max := big.NewInt(int64(len(userCodeAlphabet)))
	for i := 0; i < 8; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(userCodeAlphabet[n.Int64()])
	}
	return formatUserCode(b.String()), nil
}

// normalizeUserCode uppercases and drops anything outside the alphabet
// (dashes, spaces), so "bcdf-ghjk" and "BCDFGHJK" are the same code.
func normalizeUserCode(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if strings.ContainsRune(userCodeAlphabet, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func formatUserCode(s string) string {
	if len(s) != 8 {
		return s
	}
	return s[:4] + "-" + s[4:]
}
