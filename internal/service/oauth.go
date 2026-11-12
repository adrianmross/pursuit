package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/adrianmross/oauthlite"

	"github.com/pursuit/pursuit/internal/auth"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

// OAuth2 for `pursuit auth login`. Pursuit is its own authorization
// server: a human signed into the dashboard approves, and the CLI gets a
// normal API token. Two grants share one table (sqlstore.OAuthGrant):
// loopback authorization code + PKCE (RFC 8252/7636) and device
// authorization (RFC 8628).
//
// The flows themselves run in oauthlite. This file supplies what is
// Pursuit's: who may approve, which scopes OAuth may carry, which clients
// exist, what a token is, and the dashboard's error shapes.

const (
	OAuthCodeTTL        = oauthlite.CodeTTL
	OAuthDeviceTTL      = oauthlite.DeviceTTL
	OAuthDeviceInterval = oauthlite.DeviceInterval
	DefaultCLIClientID  = "pursuit-cli"
	DeviceCodeGrantType = oauthlite.DeviceCodeGrantType
	oauthTokenName      = "cli"
	retryDeviceHint     = "run pursuit auth login --device again"
)

// OAuthError is an RFC 6749 section 5.2 error: {"error": Code, "error_description": Description}.
type OAuthError = oauthlite.Error

func oauthErr(code, desc string) *OAuthError { return &OAuthError{Code: code, Description: desc} }

// OfflineAccess asks for a refresh token. Registered clients always get one,
// so it is accepted and dropped.
const OfflineAccess = "offline_access"

// ParseOAuthScopes accepts space- or comma-separated scopes, each exactly
// "read" or "write" (case-sensitive). Empty means read and write. OAuth
// never grants approve: approving stays a dashboard action, or an
// explicit PURSUIT_TOKEN a human minted on purpose.
func ParseOAuthScopes(raw string) ([]string, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ' ' || r == ',' })
	parts = slices.DeleteFunc(parts, func(p string) bool { return p == OfflineAccess })
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

// invalidScope is a refused scope as a dashboard API error.
func invalidScope(msg string) error {
	return &domain.Error{Code: "invalid_scope", Message: msg, Hint: "request read and/or write; approve is never granted to the CLI", Status: 400}
}

// OAuthToken is the token endpoint's success body.
type OAuthToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	// ExpiresIn and RefreshToken are set for registered clients only; the
	// CLI's tokens do not expire.
	ExpiresIn    int    `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

const maxClientID = 64

func normalizeClientID(c string) (string, bool) {
	if c == "" {
		return DefaultCLIClientID, true
	}
	if isClientIDURL(c) {
		return c, len(c) <= maxClientIDURL
	}
	return c, len(c) <= maxClientID
}

// ValidLoopbackRedirect accepts only http://127.0.0.1:<port>/callback or
// http://[::1]:<port>/callback (RFC 8252 section 7.3). "localhost" is
// refused: it can resolve somewhere else.
var ValidLoopbackRedirect = oauthlite.ValidLoopbackRedirect

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
	if err := s.requireFresh(ctx, a); err != nil {
		return domain.Actor{}, err
	}
	return a, nil
}

type AuthorizeInput struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	State               string `json:"state"`
	Scope               string `json:"scope"`
	// Resource is the RFC 8707 target the token is for; empty or this server.
	Resource string `json:"resource,omitempty"`
	Approve  bool   `json:"approve"`
	// Issuer is set by the HTTP layer: the iss returned to the client (RFC 9207).
	Issuer string `json:"-"`
}

// AuthorizeRequest is a validated loopback request, normalized for the
// consent page to show as-is.
type AuthorizeRequest struct {
	ClientID    string   `json:"client_id"`
	ClientName  string   `json:"client_name"`
	Scopes      []string `json:"scopes"`
	RedirectURI string   `json:"redirect_uri"`
	Resource    string   `json:"resource,omitempty"`
	// Unverified is true for any client that registered itself (cimd, dcr).
	Unverified bool `json:"unverified"`
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

// oauthFlow is one request's oauthlite server plus what its hooks learned:
// the approving human and the client, which auditing and the wrappers need.
type oauthFlow struct {
	*oauthlite.Server
	approver domain.Actor
	client   OAuthClient
}

func (s *Service) oauth() *oauthFlow {
	f := &oauthFlow{}
	f.Server = &oauthlite.Server{
		Grants: grantStore{s.DB},
		Approver: func(ctx context.Context) (oauthlite.Approver, error) {
			a, err := s.humanApprover(ctx)
			f.approver = a
			return oauthlite.Approver{UserID: a.UserID, TokenID: a.TokenID, Can: a.Can}, err
		},
		Scopes: ParseOAuthScopes,
		Issue: func(ctx context.Context, g oauthlite.Grant, flow string) (oauthlite.Token, error) {
			tok, err := s.issueOAuthToken(ctx, g, flow)
			return oauthlite.Token(tok), err
		},
		Audit: func(ctx context.Context, e oauthlite.Event) error {
			return s.audit(ctx, f.approver, e.Action, "oauth_grant", e.GrantID, e.Summary, e.Meta)
		},
		Tx:    s.DB.WithTx,
		Now:   s.now,
		NewID: s.NewID,
		Client: func(ctx context.Context, id string) (oauthlite.Client, error) {
			c, err := s.resolveClient(ctx, id)
			if err != nil {
				return oauthlite.Client{}, err
			}
			f.client = c
			out := oauthlite.Client{ID: c.ID, Name: c.Name}
			// Builtin clients keep the strict loopback rule.
			if c.Source != "builtin" {
				out.Unverified = true
				out.RedirectOK = func(uri string) bool { return redirectMatches(c.RedirectURIs, uri) }
			}
			return out, nil
		},
		ClientIDOK:      func(c string) bool { _, ok := normalizeClientID(c); return ok },
		DefaultClientID: DefaultCLIClientID,
		RetryDeviceHint: retryDeviceHint,
	}
	return f
}

// oauthRefusal shows a refused dashboard request as the API's own error.
func oauthRefusal(err error) error {
	var r *oauthlite.Refusal
	if !errors.As(err, &r) {
		return err
	}
	switch r.Kind {
	case oauthlite.InvalidScope:
		return invalidScope(r.Message)
	case oauthlite.Forbidden:
		return domain.Forbidden(r.Scope).WithHint(r.Hint)
	case oauthlite.NotFound:
		e := domain.NotFound("device code")
		if r.Hint != "" {
			e = e.WithHint(r.Hint)
		}
		return e
	case oauthlite.Conflict:
		return domain.Conflict(r.Message, r.Hint)
	}
	return domain.Validation("%s", r.Message)
}

// OAuthAuthorizeCheck validates a loopback request without deciding it,
// so the consent page shows the server's reading of client and scopes.
func (s *Service) OAuthAuthorizeCheck(ctx context.Context, in AuthorizeInput) (AuthorizeRequest, error) {
	req, err := s.oauth().AuthorizeCheck(ctx, oauthlite.AuthorizeInput(in))
	return AuthorizeRequest(req), oauthRefusal(err)
}

// OAuthAuthorize is the dashboard consent step of the loopback flow. It
// validates the request, and on approval stores a single-use code bound to
// the challenge, redirect URI, user, and scopes. It returns where the
// browser goes next: the CLI's loopback callback with code or error.
func (s *Service) OAuthAuthorize(ctx context.Context, in AuthorizeInput) (string, error) {
	f := s.oauth()
	to, err := f.Authorize(ctx, oauthlite.AuthorizeInput(in))
	if err != nil {
		return "", oauthRefusal(err)
	}
	if in.Approve && f.client.Source != "builtin" {
		_ = s.DB.TouchOAuthClient(ctx, f.client.ID, domain.FormatTime(s.now()))
	}
	return to, nil
}

// OAuthExchangeCode is grant_type=authorization_code. The code is burned
// before anything else is checked, so a wrong verifier also spends it.
func (s *Service) OAuthExchangeCode(ctx context.Context, code, verifier, redirectURI, clientID, resource string) (OAuthToken, error) {
	tok, err := s.oauth().ExchangeCode(ctx, code, verifier, redirectURI, clientID, resource)
	return OAuthToken(tok), err
}

// OAuthDeviceStart begins a device login. baseURL is the public origin
// the dashboard is served from; the human opens baseURL/device. ip is
// the requester's address, shown to the approver.
func (s *Service) OAuthDeviceStart(ctx context.Context, clientID, scope, baseURL, ip string) (DeviceAuthorization, error) {
	out, err := s.oauth().DeviceStart(ctx, clientID, scope, strings.TrimRight(baseURL, "/")+"/device", ip)
	return DeviceAuthorization(out), err
}

func (s *Service) OAuthDeviceLookup(ctx context.Context, userCode string) (DeviceRequest, error) {
	r, err := s.oauth().DeviceLookup(ctx, userCode)
	if err != nil {
		return DeviceRequest{}, oauthRefusal(err)
	}
	return DeviceRequest{UserCode: r.UserCode, ClientID: r.ClientID, Scopes: r.Scopes, Status: r.Status, CreatedAt: domain.FormatTime(r.CreatedAt), ExpiresAt: domain.FormatTime(r.ExpiresAt), RequestedFrom: r.RequestedFrom}, nil
}

// OAuthDeviceDecide approves or denies a device login. It needs both the
// user code and a signed-in human; the code alone approves nothing.
func (s *Service) OAuthDeviceDecide(ctx context.Context, userCode string, approve bool) (string, error) {
	status, err := s.oauth().DeviceDecide(ctx, userCode, approve)
	return status, oauthRefusal(err)
}

// OAuthExchangeDevice is the device_code grant, polled by the CLI.
func (s *Service) OAuthExchangeDevice(ctx context.Context, deviceCode, clientID string) (OAuthToken, error) {
	tok, err := s.oauth().ExchangeDevice(ctx, deviceCode, clientID)
	return OAuthToken(tok), err
}

// grantStore keeps oauthlite's grants in the oauth_grants table, whose
// times are stored as text.
type grantStore struct{ db *sqlstore.Store }

func grantTime(s string) time.Time {
	t, _ := domain.ParseTime(s) // "" (never polled) is the zero time
	return t
}

func (g grantStore) get(row sqlstore.OAuthGrant, err error) (oauthlite.Grant, error) {
	if domain.IsCode(err, "not_found") {
		return oauthlite.Grant{}, oauthlite.ErrNotFound
	}
	return oauthlite.Grant{
		ID: row.ID, Kind: row.Kind, CodeHash: row.CodeHash, UserCodeHash: row.UserCodeHash, ClientID: row.ClientID,
		Scopes: row.Scopes, RedirectURI: row.RedirectURI, Resource: row.Resource, CodeChallenge: row.CodeChallenge,
		UserID: row.UserID, ApproverTokenID: row.ApproverTokenID, RequestedFrom: row.RequestedFrom, Status: row.Status,
		Interval: row.Interval, LastPolledAt: grantTime(row.LastPolledAt), ExpiresAt: grantTime(row.ExpiresAt), CreatedAt: grantTime(row.CreatedAt),
	}, err
}

func (g grantStore) Create(ctx context.Context, in oauthlite.Grant) error {
	return g.db.CreateOAuthGrant(ctx, sqlstore.OAuthGrant{
		ID: in.ID, Kind: in.Kind, CodeHash: in.CodeHash, UserCodeHash: in.UserCodeHash, ClientID: in.ClientID,
		Scopes: in.Scopes, RedirectURI: in.RedirectURI, Resource: in.Resource, CodeChallenge: in.CodeChallenge,
		UserID: in.UserID, ApproverTokenID: in.ApproverTokenID, RequestedFrom: in.RequestedFrom, Status: in.Status,
		Interval: in.Interval, LastPolledAt: domain.FormatTime(in.LastPolledAt), ExpiresAt: domain.FormatTime(in.ExpiresAt), CreatedAt: domain.FormatTime(in.CreatedAt),
	})
}

func (g grantStore) ByCode(ctx context.Context, kind, codeHash string) (oauthlite.Grant, error) {
	return g.get(g.db.GetOAuthGrantByCode(ctx, kind, codeHash))
}

func (g grantStore) ByUserCode(ctx context.Context, userCodeHash string) (oauthlite.Grant, error) {
	return g.get(g.db.GetOAuthDeviceByUserCode(ctx, userCodeHash))
}

func (g grantStore) Decide(ctx context.Context, id, userID, approverTokenID, status string, now time.Time) (bool, error) {
	return g.db.DecideOAuthDevice(ctx, id, userID, approverTokenID, status, domain.FormatTime(now))
}

func (g grantStore) Use(ctx context.Context, id string) (bool, error) {
	return g.db.UseOAuthGrant(ctx, id)
}

func (g grantStore) TouchPoll(ctx context.Context, id string, prev, now time.Time, interval int) (bool, error) {
	return g.db.TouchOAuthPoll(ctx, id, domain.FormatTime(prev), domain.FormatTime(now), interval)
}

func (g grantStore) DeleteExpired(ctx context.Context, before time.Time) error {
	return g.db.DeleteExpiredOAuthGrants(ctx, domain.FormatTime(before))
}

func (s *Service) issueOAuthToken(ctx context.Context, g oauthlite.Grant, flow string) (OAuthToken, error) {
	if client, err := s.DB.GetOAuthClient(ctx, g.ClientID); err == nil {
		return s.issueClientTokens(ctx, clientGrant{
			userID: g.UserID, approverTokenID: g.ApproverTokenID, resource: g.Resource, grantID: g.ID, flow: flow,
			client: client, scopes: g.Scopes, family: s.NewID(), absoluteAt: s.now().Add(RefreshMaxTTL),
		})
	} else if !domain.IsCode(err, "not_found") {
		return OAuthToken{}, err
	}
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
	if done, err := s.revokeRefresh(ctx, secret); done {
		return err
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
		if tok.FamilyID != "" {
			if err := s.DB.RevokeTokenFamily(ctx, tok.FamilyID, domain.FormatTime(s.now())); err != nil {
				return err
			}
		}
		actor := domain.Actor{UserID: tok.UserID, TokenID: tok.ID, Name: tok.Name, Scopes: tok.Scopes}
		return s.audit(ctx, actor, "token.revoke", "token", tok.ID, "revoked token "+tok.Name+" (logout)", nil)
	})
}
