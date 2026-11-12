package sqlstore

import "context"

// RateHit counts one request against key and returns how many the key has
// now, in one statement so concurrent instances agree. A key that is new
// also deletes the windows that have expired, so there is no sweeper.
func (s *Store) RateHit(ctx context.Context, key string, now, expiresAt int64) (int, error) {
	var hits int
	err := s.queryRow(ctx, `INSERT INTO rate_limits (key, hits, expires_at) VALUES (?, 1, ?)
		ON CONFLICT(key) DO UPDATE SET hits = rate_limits.hits + 1 RETURNING hits`, key, expiresAt).Scan(&hits)
	if err != nil {
		return 0, err
	}
	if hits == 1 {
		_, _ = s.exec(ctx, `DELETE FROM rate_limits WHERE expires_at < ?`, now)
	}
	return hits, nil
}

// ResetRateLimits forgets every counter. Tests use it between phases.
func (s *Store) ResetRateLimits(ctx context.Context) error {
	_, err := s.exec(ctx, `DELETE FROM rate_limits`)
	return err
}
