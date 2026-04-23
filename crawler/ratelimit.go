package crawler

import (
	"context"
	"golang.org/x/time/rate"
	"time"
)

type rateLimiter struct {
	limiter *rate.Limiter
}

// newRateLimiter - создание ограничителя скорости
func newRateLimiter(rps int, delay time.Duration) *rateLimiter {
	limit := getRateLimit(rps, delay)
	return &rateLimiter{
		limiter: rate.NewLimiter(limit, 1),
	}
}

// getRateLimit - получение лимита на основе параметров
func getRateLimit(rps int, delay time.Duration) rate.Limit {
	switch {
	case rps > 0:
		return rate.Limit(rps)
	case delay > 0:
		return rate.Limit(1.0 / delay.Seconds())
	default:
		return rate.Inf
	}
}

func (rl *rateLimiter) wait(ctx context.Context) error {
	if rl.limiter.Limit() == rate.Inf {
		return nil
	}
	return rl.limiter.Wait(ctx)
}
