package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		limiter := NewRateLimiter(10, time.Second)

		for i := 0; i < 10; i++ {
			err := limiter.Wait(context.Background())
			require.NoError(t, err)
		}
	})

	t.Run("blocks when limit exceeded", func(t *testing.T) {
		limiter := NewRateLimiter(2, time.Second)

		// Use up the tokens
		require.NoError(t, limiter.Wait(context.Background()))
		require.NoError(t, limiter.Wait(context.Background()))

		// Third request should block
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := limiter.Wait(ctx)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		limiter := NewRateLimiter(2, 100*time.Millisecond)

		// Use up the tokens
		require.NoError(t, limiter.Wait(context.Background()))
		require.NoError(t, limiter.Wait(context.Background()))

		// Wait for refill
		time.Sleep(110 * time.Millisecond)

		// Should have tokens again
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		err := limiter.Wait(ctx)
		require.NoError(t, err)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		limiter := NewRateLimiter(1, time.Hour)

		// Use up the token
		require.NoError(t, limiter.Wait(context.Background()))

		// Cancel context immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := limiter.Wait(ctx)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("is safe for concurrent use", func(t *testing.T) {
		limiter := NewRateLimiter(100, time.Second)

		var wg sync.WaitGroup
		var success int32

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				if limiter.Wait(ctx) == nil {
					atomic.AddInt32(&success, 1)
				}
			}()
		}

		wg.Wait()
		assert.Equal(t, int32(100), success)
	})
}

func TestClient_RateLimit(t *testing.T) {
	t.Run("limits request rate", func(t *testing.T) {
		var requests int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&requests, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithRateLimit(5, 100*time.Millisecond),
		)
		require.NoError(t, err)

		start := time.Now()

		// Make 10 requests - should take at least 100ms due to rate limiting
		for i := 0; i < 10; i++ {
			_, err := client.Get(context.Background(), "/test", nil)
			require.NoError(t, err)
		}

		elapsed := time.Since(start)
		assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond)
		assert.Equal(t, int32(10), atomic.LoadInt32(&requests))
	})

	t.Run("returns error when rate limit wait times out", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithRateLimit(1, time.Hour), // Very restrictive
		)
		require.NoError(t, err)

		// First request should succeed
		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		// Second request with short timeout should fail
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err = client.Get(ctx, "/test", nil)
		require.Error(t, err)
	})
}
