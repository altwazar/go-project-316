package crawler

import (
	"context"
	"golang.org/x/time/rate"
	"time"
)

type rateLimiter struct {
	limiter *rate.Limiter
}

func newRateLimiter(rps int, delay time.Duration) *rateLimiter {
	var limit rate.Limit
	if rps > 0 {
		limit = rate.Limit(rps)
	} else if delay > 0 {
		limit = rate.Limit(1.0 / delay.Seconds())
	} else {
		limit = rate.Inf
	}
	return &rateLimiter{
		limiter: rate.NewLimiter(limit, 1),
	}
}

func (rl *rateLimiter) wait(ctx context.Context) error {
	if rl.limiter.Limit() == rate.Inf {
		return nil
	}
	return rl.limiter.Wait(ctx)
}
