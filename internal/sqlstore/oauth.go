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
