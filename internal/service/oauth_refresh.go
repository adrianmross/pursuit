package service

import (
	"context"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/auth"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

// Tokens for registered clients (a ChatGPT app, a Claude connector) are not
// the CLI's forever tokens. The access token lasts an hour. A refresh token
// replaces it, is used once, and is answered with the next one. Using one a
// second time means it leaked, so the whole consent is revoked. The chain
// dies after 30 idle days or 90 days in all, like a dashboard sign-in.
const (
	ClientAccessTTL = time.Hour
	RefreshIdleTTL  = 30 * 24 * time.Hour
	RefreshMaxTTL   = 90 * 24 * time.Hour
	refreshPrefix   = "psr_"
	// RefreshGrace is how long after a refresh token is used that presenting
	// it again is taken as a client retry or a parallel request, not theft.
	// It gets another valid pair; after the window the whole connection goes.
	RefreshGrace = 10 * time.Second
)

type clientGrant struct {
	userID, approverTokenID, resource, grantID, flow string
	client                                           sqlstore.OAuthClient
	scopes                                           []string
	family                                           string
	absoluteAt                                       time.Time
}

// issueClientTokens mints an access token and the next refresh token in a family.
func (s *Service) issueClientTokens(ctx context.Context, g clientGrant) (OAuthToken, error) {
	secret, prefix, hash, err := auth.Generate()
	if err != nil {
		return OAuthToken{}, err
	}
	raw, err := auth.RandomURLToken(32)
	if err != nil {
		return OAuthToken{}, err
	}
	refresh := refreshPrefix + raw
	now := s.now()
	expires := now.Add(ClientAccessTTL)
	tok := domain.Token{
		ID: s.NewID(), UserID: g.userID, Name: g.client.Name, Hash: hash, Prefix: prefix, Scopes: g.scopes, CreatedAt: now,
		ClientID: g.client.ID, Resource: g.resource, ExpiresAt: &expires, FamilyID: g.family,
	}
	idle := now.Add(RefreshIdleTTL)
	if idle.After(g.absoluteAt) {
		idle = g.absoluteAt
	}
	rt := sqlstore.RefreshToken{
		ID: s.NewID(), Hash: auth.Hash(refresh), FamilyID: g.family, ClientID: g.client.ID, UserID: g.userID, Scopes: g.scopes,
		Resource: g.resource, CreatedAt: domain.FormatTime(now), ExpiresAt: domain.FormatTime(idle), AbsoluteAt: domain.FormatTime(g.absoluteAt),
	}
	_ = s.DB.DeleteExpiredRefreshTokens(ctx, domain.FormatTime(now.Add(-time.Hour)))
	actor := domain.Actor{UserID: g.userID, TokenID: tok.ID, Name: tok.Name, Scopes: g.scopes}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.CreateToken(ctx, tok); err != nil {
			return err
		}
		if err := s.DB.CreateRefreshToken(ctx, rt); err != nil {
			return err
		}
		if g.flow == "refresh" { // not refresh-grace: see OAuthRefresh
			if err := s.DB.RevokeFamilyAccessTokens(ctx, g.family, tok.ID, domain.FormatTime(now)); err != nil {
				return err
			}
		}
		return s.audit(ctx, actor, "token.create", "token", tok.ID, g.client.Name+" token ("+g.flow+")",
			map[string]any{"prefix": prefix, "scopes": g.scopes, "client_id": g.client.ID, "family_id": g.family, "grant_id": g.grantID, "approver_token_id": g.approverTokenID, "resource": g.resource})
	})
	if err != nil {
		return OAuthToken{}, err
	}
	return OAuthToken{
		AccessToken: secret, TokenType: "Bearer", Scope: strings.Join(g.scopes, " "),
		ExpiresIn: int(ClientAccessTTL.Seconds()), RefreshToken: refresh,
	}, nil
}

// OAuthRefresh is grant_type=refresh_token. scope may only narrow what was
// granted, and resource must be the one the consent named.
func (s *Service) OAuthRefresh(ctx context.Context, refreshToken, clientID, scope, resource string) (OAuthToken, error) {
	if refreshToken == "" {
		return OAuthToken{}, oauthErr("invalid_request", "refresh_token is required")
	}
	clientID, ok := normalizeClientID(clientID)
	if !ok {
		return OAuthToken{}, oauthErr("invalid_request", "client_id is too long")
	}
	dead := oauthErr("invalid_grant", "the refresh token is invalid, expired or already used; authorize again")
	rt, err := s.DB.GetRefreshTokenByHash(ctx, auth.Hash(refreshToken))
	if domain.IsCode(err, "not_found") {
		return OAuthToken{}, dead
	}
	if err != nil {
		return OAuthToken{}, err
	}
	now := s.now()
	nowS := domain.FormatTime(now)
	switch {
	case rt.ClientID != clientID, rt.RevokedAt != "":
		return OAuthToken{}, dead
	case rt.UsedAt != "" && !s.withinGrace(rt, now):
		return OAuthToken{}, s.refreshReuse(ctx, rt, dead)
	case nowS >= rt.ExpiresAt, nowS >= rt.AbsoluteAt:
		return OAuthToken{}, dead
	case resource != "" && resource != rt.Resource:
		return OAuthToken{}, oauthErr("invalid_target", "resource does not match the one that was authorized")
	}
	scopes := rt.Scopes
	if scope != "" {
		want, err := ParseOAuthScopes(scope)
		if err != nil {
			return OAuthToken{}, err
		}
		for _, w := range want {
			if !contains(rt.Scopes, w) {
				return OAuthToken{}, oauthErr("invalid_scope", "scope is wider than the one that was granted")
			}
		}
		scopes = want
	}
	client, err := s.DB.GetOAuthClient(ctx, rt.ClientID)
	if err != nil {
		return OAuthToken{}, dead
	}
	flow := "refresh"
	used := false
	if rt.UsedAt == "" {
		if used, err = s.DB.UseRefreshToken(ctx, rt.ID, nowS); err != nil {
			return OAuthToken{}, err
		}
	}
	if !used {
		// Used already: by an earlier request inside the grace window, or by
		// one that won a race just now. Re-read to see which, and whether it
		// was revoked meanwhile.
		cur, err := s.DB.GetRefreshTokenByHash(ctx, auth.Hash(refreshToken))
		if err != nil || cur.RevokedAt != "" {
			return OAuthToken{}, dead
		}
		if !s.withinGrace(cur, now) {
			return OAuthToken{}, s.refreshReuse(ctx, cur, dead)
		}
		// A parallel request is holding the family's live access token, so
		// this one must not revoke it.
		flow = "refresh-grace"
	}
	abs, err := domain.ParseTime(rt.AbsoluteAt)
	if err != nil {
		return OAuthToken{}, err
	}
	return s.issueClientTokens(ctx, clientGrant{
		userID: rt.UserID, resource: rt.Resource, client: client, scopes: scopes, family: rt.FamilyID, absoluteAt: abs, flow: flow,
	})
}

// refreshReuse revokes the family of a replayed refresh token and returns
// the error to answer with. Either the client or a thief holds the other
// copy, and we can't tell which.
func (s *Service) refreshReuse(ctx context.Context, rt sqlstore.RefreshToken, dead error) error {
	actor := domain.Actor{UserID: rt.UserID, Name: "oauth"}
	err := s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.RevokeTokenFamily(ctx, rt.FamilyID, domain.FormatTime(s.now())); err != nil {
			return err
		}
		return s.audit(ctx, actor, "oauth.reuse", "token_family", rt.FamilyID, "refresh token reused; revoked the connection", map[string]any{"client_id": rt.ClientID})
	})
	if err != nil {
		return err
	}
	return dead
}

func (s *Service) withinGrace(rt sqlstore.RefreshToken, now time.Time) bool {
	used, err := domain.ParseTime(rt.UsedAt)
	return err == nil && now.Sub(used) <= RefreshGrace
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// revokeRefresh handles RFC 7009 for a refresh token: revoke its family.
func (s *Service) revokeRefresh(ctx context.Context, secret string) (bool, error) {
	if !strings.HasPrefix(secret, refreshPrefix) {
		return false, nil
	}
	rt, err := s.DB.GetRefreshTokenByHash(ctx, auth.Hash(secret))
	if domain.IsCode(err, "not_found") {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	return true, s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.RevokeTokenFamily(ctx, rt.FamilyID, domain.FormatTime(s.now())); err != nil {
			return err
		}
		return s.audit(ctx, domain.Actor{UserID: rt.UserID, Name: "oauth"}, "oauth.revoke", "token_family", rt.FamilyID, "revoked a connection (refresh token)", map[string]any{"client_id": rt.ClientID})
	})
}
