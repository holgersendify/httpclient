package httpclient

import (
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// RetryPolicy configures retry behavior for failed requests.
type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	Jitter       float64 // 0.0 to 1.0, percentage of delay to randomize
}

// DefaultRetryPolicy returns a retry policy with sensible defaults.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
	}
}

// NoRetry returns a policy that does not retry.
func NoRetry() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts: 1,
	}
}

// Backoff calculates the delay before the given attempt (1-indexed).
func (p *RetryPolicy) Backoff(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}

	delay := float64(p.InitialDelay) * math.Pow(p.Multiplier, float64(attempt-1))

	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}

	if p.Jitter > 0 {
		jitterRange := delay * p.Jitter
		delay = delay - jitterRange + (rand.Float64() * 2 * jitterRange)
	}

	return time.Duration(delay)
}

// ShouldRetry returns true if the given status code should be retried.
func (p *RetryPolicy) ShouldRetry(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout,      // 408
		http.StatusTooManyRequests,   // 429
		http.StatusBadGateway,        // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:    // 504
		return true
	}
	return false
}

// ParseRetryAfter parses the Retry-After header value.
// Supports seconds format. Returns 0 if parsing fails.
func ParseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}

	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}

	return time.Duration(seconds) * time.Second
}
