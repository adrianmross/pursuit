package sqlstore

import (
	"context"
	"database/sql"

	"github.com/pursuit/pursuit/internal/domain"
)

// WebAuthnCredential is one passkey. Credential is the library's record as
// JSON; the service layer is the only thing that reads inside it.
type WebAuthnCredential struct {
	ID         string
	UserID     string
	Name       string
	Credential []byte
	CreatedAt  string
	LastUsedAt string
}

// SetWebAuthnHandle sets a user's handle only if it has none, so two
// concurrent registrations can't give one user two handles. Read it back
// with WebAuthnHandle.
func (s *Store) SetWebAuthnHandle(ctx context.Context, userID, handle string) error {
	_, err := s.exec(ctx, `UPDATE users SET webauthn_user_handle=? WHERE id=? AND webauthn_user_handle=''`, handle, userID)
	return err
}

func (s *Store) WebAuthnHandle(ctx context.Context, userID string) (string, error) {
	var h string
	err := s.queryRow(ctx, `SELECT webauthn_user_handle FROM users WHERE id=?`, userID).Scan(&h)
	if err == sql.ErrNoRows {
		return "", domain.NotFound("user")
	}
	return h, err
}

func (s *Store) UserIDByWebAuthnHandle(ctx context.Context, handle string) (string, error) {
	var id string
	err := s.queryRow(ctx, `SELECT id FROM users WHERE webauthn_user_handle=? AND webauthn_user_handle<>''`, handle).Scan(&id)
	if err == sql.ErrNoRows {
		return "", domain.NotFound("user")
	}
	return id, err
}

func (s *Store) CreateWebAuthnCredential(ctx context.Context, c WebAuthnCredential) error {
	_, err := s.exec(ctx, `INSERT INTO webauthn_credentials (id, user_id, name, credential, created_at, last_used_at) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.UserID, c.Name, string(c.Credential), c.CreatedAt, c.LastUsedAt)
	return mapWriteErr(err)
}

func (s *Store) ListWebAuthnCredentials(ctx context.Context, userID string) ([]WebAuthnCredential, error) {
	rows, err := s.query(ctx, `SELECT id, user_id, name, credential, created_at, last_used_at FROM webauthn_credentials WHERE user_id=? ORDER BY created_at, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WebAuthnCredential{}
	for rows.Next() {
		var c WebAuthnCredential
		var cred string
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &cred, &c.CreatedAt, &c.LastUsedAt); err != nil {
			return nil, err
		}
		c.Credential = []byte(cred)
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateWebAuthnCredential stores the record a login returned (new sign
// count and flags) and when it was used.
func (s *Store) UpdateWebAuthnCredential(ctx context.Context, userID, id string, credential []byte, lastUsed string) error {
	res, err := s.exec(ctx, `UPDATE webauthn_credentials SET credential=?, last_used_at=? WHERE user_id=? AND id=?`, string(credential), lastUsed, userID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.NotFound("passkey")
	}
	return nil
}

func (s *Store) DeleteWebAuthnCredential(ctx context.Context, userID, id string) error {
	res, err := s.exec(ctx, `DELETE FROM webauthn_credentials WHERE user_id=? AND id=?`, userID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.NotFound("passkey")
	}
	return nil
}

func (s *Store) CreateWebAuthnChallenge(ctx context.Context, id, kind, userID string, session []byte, expiresAt string) error {
	_, err := s.exec(ctx, `INSERT INTO webauthn_challenges (id, kind, user_id, session, expires_at) VALUES (?, ?, ?, ?, ?)`,
		id, kind, userID, string(session), expiresAt)
	return mapWriteErr(err)
}

// UseWebAuthnChallenge marks an unexpired, unused challenge used and
// returns it. Exactly one caller across all replicas gets ok=true.
func (s *Store) UseWebAuthnChallenge(ctx context.Context, id, kind, now string) (userID string, session []byte, ok bool, err error) {
	res, err := s.exec(ctx, `UPDATE webauthn_challenges SET used_at=? WHERE id=? AND kind=? AND used_at='' AND expires_at > ?`, now, id, kind, now)
	if err != nil {
		return "", nil, false, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return "", nil, false, nil
	}
	var raw string
	if err := s.queryRow(ctx, `SELECT user_id, session FROM webauthn_challenges WHERE id=?`, id).Scan(&userID, &raw); err != nil {
		return "", nil, false, err
	}
	return userID, []byte(raw), true, nil
}

func (s *Store) DeleteExpiredWebAuthnChallenges(ctx context.Context, before string) error {
	_, err := s.exec(ctx, `DELETE FROM webauthn_challenges WHERE expires_at < ?`, before)
	return err
}
