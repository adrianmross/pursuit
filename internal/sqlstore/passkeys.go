package sqlstore

import "context"

// HasPasskey reports whether the user has a passkey to step up with. The
// passkeys themselves are read and written by appkit's account package.
func (s *Store) HasPasskey(ctx context.Context, userID string) (bool, error) {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM webauthn_credentials WHERE user_id=?`, userID).Scan(&n)
	return n > 0, err
}
