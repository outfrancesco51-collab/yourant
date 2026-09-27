package anilist

import (
	"context"
	"sync"
	"time"
)

// TokenBucketLimiter implements a thread-safe token bucket rate limiter using standard library Go.
type TokenBucketLimiter struct {
	mu         sync.Mutex
	capacity   float64
	tokens     float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewTokenBucketLimiter creates a rate limiter with given rate per minute and burst capacity.
func NewTokenBucketLimiter(ratePerMinute float64, burst int) *TokenBucketLimiter {
	if ratePerMinute <= 0 {
		ratePerMinute = 80.0 // Default AniList safe limit: 80 req/min
	}
	if burst <= 0 {
		burst = 10
	}
	return &TokenBucketLimiter{
		capacity:   float64(burst),
		tokens:     float64(burst),
		refillRate: ratePerMinute / 60.0,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available or the context is cancelled.
func (l *TokenBucketLimiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(l.lastRefill).Seconds()
		l.tokens += elapsed * l.refillRate
		if l.tokens > l.capacity {
			l.tokens = l.capacity
		}
		l.lastRefill = now

		if l.tokens >= 1.0 {
			l.tokens -= 1.0
			l.mu.Unlock()
			return nil
		}

		needed := 1.0 - l.tokens
		waitDuration := time.Duration((needed / l.refillRate) * float64(time.Second))
		l.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
		}
	}
}
