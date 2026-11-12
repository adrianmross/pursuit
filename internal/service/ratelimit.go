package service

import (
	"context"
	"strconv"
	"time"
)

// RateAllow counts one request by subject against a named rule and reports
// whether it is within limit per window. The counters live in the database,
// so the limit holds across every instance, which an in-memory counter
// cannot do on serverless. It is a fixed window: a burst straddling a window
// edge can reach twice limit, which is fine for abuse control. A database
// error lets the request through, since refusing sign-ins because the
// limiter is down is the worse failure. RateNow, when set, is
// the clock: under a frozen Now (PURSUIT_FIXED_TIME) a window would never
// end and the sixth demo of a recording session would be refused for good.
func (s *Service) RateAllow(ctx context.Context, rule, subject string, limit int, window time.Duration) bool {
	clock := s.now
	if s.RateNow != nil {
		clock = s.RateNow
	}
	now := clock().Unix()
	secs := int64(window / time.Second)
	bucket := now / secs
	hits, err := s.DB.RateHit(ctx, rule+"|"+subject+"|"+strconv.FormatInt(bucket, 10), now, (bucket+2)*secs)
	if err != nil {
		return true
	}
	return hits <= limit
}

func (s *Service) ResetRateLimits(ctx context.Context) error { return s.DB.ResetRateLimits(ctx) }
