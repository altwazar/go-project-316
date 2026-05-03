// Package fetcher - логика http запросов
package fetcher

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter - ограничитель скорости запросов
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter создаёт новый ограничитель на основе RPS или задержки
func NewRateLimiter(rps int, delay time.Duration) *RateLimiter {
	limit := getRateLimit(rps, delay)
	return &RateLimiter{
		limiter: rate.NewLimiter(limit, 1),
	}
}

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

// Wait блокирует выполнение, пока не будет разрешён очередной запрос
func (rl *RateLimiter) Wait(ctx context.Context) error {
	if rl.limiter.Limit() == rate.Inf {
		return nil
	}
	return rl.limiter.Wait(ctx)
}
