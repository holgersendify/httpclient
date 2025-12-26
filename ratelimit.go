package httpclient

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
// It is safe for concurrent use across goroutines.
type RateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per nanosecond
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter that allows `requests` per `duration`.
func NewRateLimiter(requests int, duration time.Duration) *RateLimiter {
	return &RateLimiter{
		tokens:     float64(requests),
		maxTokens:  float64(requests),
		refillRate: float64(requests) / float64(duration),
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available or the context is cancelled.
func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		r.mu.Lock()
		r.refill()

		if r.tokens >= 1 {
			r.tokens--
			r.mu.Unlock()
			return nil
		}

		// Calculate time until next token
		tokensNeeded := 1 - r.tokens
		waitDuration := time.Duration(tokensNeeded / r.refillRate)
		r.mu.Unlock()

		// Wait for refill or context cancellation
		timer := time.NewTimer(waitDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// Continue loop to try again
		}
	}
}

func (r *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	r.tokens += float64(elapsed) * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastRefill = now
}
