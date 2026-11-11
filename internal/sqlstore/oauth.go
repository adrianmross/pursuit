package sqlstore

import (
	"context"
	"database/sql"
	"strings"

	"github.com/pursuit/pursuit/internal/domain"
)

// OAuthGrant is one authorization code (kind "code") or device
// authorization (kind "device"). Status moves pending -> approved|denied
// -> used. Code and user code are only ever stored hashed.
type OAuthGrant struct {
	ID            string
	Kind          string
	CodeHash      string
	UserCodeHash  string
	ClientID      string
	Scopes        []string
	RedirectURI   string
	CodeChallenge string
	UserID        string
	// ApproverTokenID is the dashboard session token that approved the grant.
	ApproverTokenID string
	// RequestedFrom is the client IP that started a device login.
	RequestedFrom string
	Status        string
	Interval      int
	LastPolledAt  string
	ExpiresAt     string
	CreatedAt     string
}

const oauthCols = `id, kind, code_hash, user_code_hash, client_id, scopes, redirect_uri, code_challenge, user_id, approver_token_id, status, interval_seconds, last_polled_at, expires_at, created_at, requested_from`

func (s *Store) CreateOAuthGrant(ctx context.Context, g OAuthGrant) error {
	_, err := s.exec(ctx, `INSERT INTO oauth_grants (`+oauthCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Kind, g.CodeHash, nullString(g.UserCodeHash), g.ClientID, strings.Join(g.Scopes, ","), g.RedirectURI,
		g.CodeChallenge, nullString(g.UserID), g.ApproverTokenID, g.Status, g.Interval, g.LastPolledAt, g.ExpiresAt, g.CreatedAt, g.RequestedFrom)
	return mapWriteErr(err)
}

// GetOAuthGrantByCode finds a grant by its hashed code or device code.
func (s *Store) GetOAuthGrantByCode(ctx context.Context, kind, codeHash string) (OAuthGrant, error) {
	return s.oauthGrant(ctx, `kind=? AND code_hash=?`, kind, codeHash)
}

// GetOAuthDeviceByUserCode finds a device grant by its hashed user code.
func (s *Store) GetOAuthDeviceByUserCode(ctx context.Context, userCodeHash string) (OAuthGrant, error) {
	return s.oauthGrant(ctx, `kind='device' AND user_code_hash=?`, userCodeHash)
}

func (s *Store) oauthGrant(ctx context.Context, cond string, args ...any) (OAuthGrant, error) {
	var g OAuthGrant
	var userCode, userID sql.NullString
	var scopes string
	err := s.queryRow(ctx, `SELECT `+oauthCols+` FROM oauth_grants WHERE `+cond, args...).Scan(
		&g.ID, &g.Kind, &g.CodeHash, &userCode, &g.ClientID, &scopes, &g.RedirectURI, &g.CodeChallenge,
		&userID, &g.ApproverTokenID, &g.Status, &g.Interval, &g.LastPolledAt, &g.ExpiresAt, &g.CreatedAt, &g.RequestedFrom)
	if err == sql.ErrNoRows {
		return OAuthGrant{}, domain.NotFound("authorization")
	}
	if err != nil {
		return OAuthGrant{}, err
	}
	g.UserCodeHash, g.UserID = userCode.String, userID.String
	if scopes != "" {
		g.Scopes = strings.Split(scopes, ",")
	}
	return g, nil
}

// DecideOAuthDevice records a human's approve/deny on a pending, unexpired
// device grant. It reports false when the grant was already decided.
func (s *Store) DecideOAuthDevice(ctx context.Context, id, userID, approverTokenID, status, now string) (bool, error) {
	res, err := s.exec(ctx, `UPDATE oauth_grants SET status=?, user_id=?, approver_token_id=? WHERE id=? AND kind='device' AND status='pending' AND expires_at > ?`, status, userID, approverTokenID, id, now)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// UseOAuthGrant marks an approved grant used. Exactly one caller across
// all replicas gets true; that caller may mint the token.
func (s *Store) UseOAuthGrant(ctx context.Context, id string) (bool, error) {
	res, err := s.exec(ctx, `UPDATE oauth_grants SET status='used' WHERE id=? AND status='approved'`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// TouchOAuthPoll records a device-token poll. prevPolled guards against a
// concurrent poll on another replica: only one of two racing polls wins.
func (s *Store) TouchOAuthPoll(ctx context.Context, id, prevPolled, now string, interval int) (bool, error) {
	res, err := s.exec(ctx, `UPDATE oauth_grants SET last_polled_at=?, interval_seconds=? WHERE id=? AND last_polled_at=?`, now, interval, id, prevPolled)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (s *Store) DeleteExpiredOAuthGrants(ctx context.Context, before string) error {
	_, err := s.exec(ctx, `DELETE FROM oauth_grants WHERE expires_at < ?`, before)
	return err
}

// OAuthClient is a registered or discovered client. Source is "cimd" or "dcr".
type OAuthClient struct {
	ID           string
	Name         string
	RedirectURIs []string
	Source       string
	CreatedAt    string
	LastUsedAt   string
	// ExpiresAt is when a cimd document must be fetched again; empty for dcr.
	ExpiresAt string
}

const oauthClientCols = `id, name, redirect_uris, source, created_at, last_used_at, expires_at`

// UpsertOAuthClient inserts a client, or refreshes the name, redirect URIs
// and expiry of a cimd client whose document was fetched again.
func (s *Store) UpsertOAuthClient(ctx context.Context, c OAuthClient) error {
	_, err := s.exec(ctx, `INSERT INTO oauth_clients (`+oauthClientCols+`) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, redirect_uris=excluded.redirect_uris, expires_at=excluded.expires_at`,
		c.ID, c.Name, strings.Join(c.RedirectURIs, "\n"), c.Source, c.CreatedAt, c.LastUsedAt, c.ExpiresAt)
	return mapWriteErr(err)
}

func (s *Store) GetOAuthClient(ctx context.Context, id string) (OAuthClient, error) {
	var c OAuthClient
	var uris string
	err := s.queryRow(ctx, `SELECT `+oauthClientCols+` FROM oauth_clients WHERE id=?`, id).Scan(
		&c.ID, &c.Name, &uris, &c.Source, &c.CreatedAt, &c.LastUsedAt, &c.ExpiresAt)
	if err == sql.ErrNoRows {
		return OAuthClient{}, domain.NotFound("oauth client")
	}
	if err != nil {
		return OAuthClient{}, err
	}
	if uris != "" {
		c.RedirectURIs = strings.Split(uris, "\n")
	}
	return c, nil
}

func (s *Store) TouchOAuthClient(ctx context.Context, id, at string) error {
	_, err := s.exec(ctx, `UPDATE oauth_clients SET last_used_at=? WHERE id=?`, at, id)
	return err
}

func (s *Store) CountOAuthClients(ctx context.Context, source string) (n int, err error) {
	err = s.queryRow(ctx, `SELECT COUNT(*) FROM oauth_clients WHERE source=?`, source).Scan(&n)
	return n, err
}

// DeleteUnusedOAuthClients drops clients of a source that were created
// before the cutoff and never used.
func (s *Store) DeleteUnusedOAuthClients(ctx context.Context, source, before string) error {
	_, err := s.exec(ctx, `DELETE FROM oauth_clients WHERE source=? AND last_used_at='' AND created_at < ?`, source, before)
	return err
}
