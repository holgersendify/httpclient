package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryPolicy(t *testing.T) {
	t.Run("DefaultRetryPolicy has sensible defaults", func(t *testing.T) {
		policy := DefaultRetryPolicy()

		assert.Equal(t, 3, policy.MaxAttempts)
		assert.Equal(t, 500*time.Millisecond, policy.InitialDelay)
		assert.Equal(t, 30*time.Second, policy.MaxDelay)
		assert.Equal(t, 2.0, policy.Multiplier)
		assert.True(t, policy.Jitter > 0)
	})

	t.Run("NoRetry returns single attempt policy", func(t *testing.T) {
		policy := NoRetry()

		assert.Equal(t, 1, policy.MaxAttempts)
	})
}

func TestRetryPolicy_Backoff(t *testing.T) {
	t.Run("calculates exponential backoff", func(t *testing.T) {
		policy := &RetryPolicy{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
			Jitter:       0, // disable jitter for predictable testing
		}

		assert.Equal(t, 100*time.Millisecond, policy.Backoff(1))
		assert.Equal(t, 200*time.Millisecond, policy.Backoff(2))
		assert.Equal(t, 400*time.Millisecond, policy.Backoff(3))
		assert.Equal(t, 800*time.Millisecond, policy.Backoff(4))
	})

	t.Run("respects max delay", func(t *testing.T) {
		policy := &RetryPolicy{
			InitialDelay: 1 * time.Second,
			MaxDelay:     2 * time.Second,
			Multiplier:   10.0,
			Jitter:       0,
		}

		assert.Equal(t, 1*time.Second, policy.Backoff(1))
		assert.Equal(t, 2*time.Second, policy.Backoff(2)) // capped at max
		assert.Equal(t, 2*time.Second, policy.Backoff(3)) // still capped
	})

	t.Run("applies jitter", func(t *testing.T) {
		policy := &RetryPolicy{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
			Jitter:       0.5,
		}

		// With 50% jitter, delay should be between 50ms and 150ms for first attempt
		delays := make(map[time.Duration]bool)
		for i := 0; i < 100; i++ {
			delay := policy.Backoff(1)
			delays[delay] = true
			assert.GreaterOrEqual(t, delay, 50*time.Millisecond)
			assert.LessOrEqual(t, delay, 150*time.Millisecond)
		}
		// Should have some variation
		assert.Greater(t, len(delays), 1)
	})
}

func TestRetryPolicy_ShouldRetry(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"408 Request Timeout", http.StatusRequestTimeout, true},
		{"429 Too Many Requests", http.StatusTooManyRequests, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, true},
		{"400 Bad Request", http.StatusBadRequest, false},
		{"401 Unauthorized", http.StatusUnauthorized, false},
		{"404 Not Found", http.StatusNotFound, false},
		{"500 Internal Server Error", http.StatusInternalServerError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, policy.ShouldRetry(tt.statusCode))
		})
	}
}

func TestClient_Retry(t *testing.T) {
	t.Run("retries on retryable status codes", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&attempts, 1)
			if count < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		}))
		defer server.Close()

		policy := &RetryPolicy{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
			Jitter:       0,
		}

		client, err := New(
			WithBaseURL(server.URL),
			WithRetry(policy),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test", nil)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
	})

	t.Run("stops after max attempts", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		policy := &RetryPolicy{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
			Jitter:       0,
		}

		client, err := New(
			WithBaseURL(server.URL),
			WithRetry(policy),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test", nil)

		require.Error(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))

		var httpErr *Error
		require.ErrorAs(t, err, &httpErr)
		assert.Equal(t, 3, httpErr.Attempts)
	})

	t.Run("does not retry non-retryable status codes", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		policy := DefaultRetryPolicy()
		policy.InitialDelay = 10 * time.Millisecond

		client, err := New(
			WithBaseURL(server.URL),
			WithRetry(policy),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)

		require.Error(t, err)
		assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
	})

	t.Run("respects Retry-After header in seconds", func(t *testing.T) {
		var attempts int32
		var delays []time.Duration
		var lastRequest time.Time

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			if !lastRequest.IsZero() {
				delays = append(delays, now.Sub(lastRequest))
			}
			lastRequest = now

			count := atomic.AddInt32(&attempts, 1)
			if count < 2 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		policy := &RetryPolicy{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     5 * time.Second,
			Multiplier:   2.0,
			Jitter:       0,
		}

		client, err := New(
			WithBaseURL(server.URL),
			WithRetry(policy),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)

		require.NoError(t, err)
		require.Len(t, delays, 1)
		// Should have waited approximately 1 second (with some tolerance)
		assert.GreaterOrEqual(t, delays[0], 900*time.Millisecond)
	})

	t.Run("replays body on retry", func(t *testing.T) {
		var attempts int32
		var bodies []string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body := make([]byte, 1024)
			n, _ := r.Body.Read(body)
			bodies = append(bodies, string(body[:n]))

			count := atomic.AddInt32(&attempts, 1)
			if count < 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		policy := &RetryPolicy{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
			Jitter:       0,
		}

		client, err := New(
			WithBaseURL(server.URL),
			WithRetry(policy),
		)
		require.NoError(t, err)

		_, err = client.Post(context.Background(), "/test", map[string]string{"key": "value"}, nil)

		require.NoError(t, err)
		require.Len(t, bodies, 2)
		assert.Equal(t, bodies[0], bodies[1]) // Same body on retry
	})
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected time.Duration
	}{
		{"seconds", "5", 5 * time.Second},
		{"zero", "0", 0},
		{"invalid", "invalid", 0},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ParseRetryAfter(tt.value))
		})
	}
}
