package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddleware(t *testing.T) {
	t.Run("executes middleware in order", func(t *testing.T) {
		var order []string

		mw1 := func(req *http.Request, next RoundTripFunc) (*http.Response, error) {
			order = append(order, "mw1-before")
			resp, err := next(req)
			order = append(order, "mw1-after")
			return resp, err
		}

		mw2 := func(req *http.Request, next RoundTripFunc) (*http.Response, error) {
			order = append(order, "mw2-before")
			resp, err := next(req)
			order = append(order, "mw2-after")
			return resp, err
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithMiddleware(mw1),
			WithMiddleware(mw2),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		assert.Equal(t, []string{"mw1-before", "mw2-before", "mw2-after", "mw1-after"}, order)
	})

	t.Run("can modify request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "middleware-value", r.Header.Get("X-Custom"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		addHeader := func(req *http.Request, next RoundTripFunc) (*http.Response, error) {
			req.Header.Set("X-Custom", "middleware-value")
			return next(req)
		}

		client, err := New(
			WithBaseURL(server.URL),
			WithMiddleware(addHeader),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)
	})

	t.Run("can short-circuit request", func(t *testing.T) {
		var serverCalled int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&serverCalled, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		shortCircuit := func(req *http.Request, next RoundTripFunc) (*http.Response, error) {
			// Return without calling next - simulate a cached/mocked response
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       http.NoBody,
			}, nil
		}

		client, err := New(
			WithBaseURL(server.URL),
			WithMiddleware(shortCircuit),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, int32(0), atomic.LoadInt32(&serverCalled))
	})
}

func TestLoggingMiddleware(t *testing.T) {
	t.Run("logs request and response", func(t *testing.T) {
		var logs []string
		logger := func(msg string) {
			logs = append(logs, msg)
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithMiddleware(LoggingMiddleware(logger)),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		require.Len(t, logs, 2)
		assert.Contains(t, logs[0], "GET")
		assert.Contains(t, logs[0], "/test")
		assert.Contains(t, logs[1], "200")
	})

	t.Run("redacts sensitive headers", func(t *testing.T) {
		var logs []string
		logger := func(msg string) {
			logs = append(logs, msg)
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithMiddleware(LoggingMiddleware(logger)),
			WithHeader("Authorization", "Bearer secret-token"),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		// Ensure the actual token is not in the logs
		for _, log := range logs {
			assert.NotContains(t, log, "secret-token")
		}
	})

	t.Run("redacts custom sensitive headers", func(t *testing.T) {
		var logs []string
		logger := func(msg string) {
			logs = append(logs, msg)
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithMiddleware(LoggingMiddlewareWithRedaction(logger, []string{"X-Api-Key"})),
			WithHeader("X-Api-Key", "my-secret-key"),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		for _, log := range logs {
			assert.NotContains(t, log, "my-secret-key")
		}
	})
}

func TestRequestIDMiddleware(t *testing.T) {
	t.Run("adds request ID header", func(t *testing.T) {
		var receivedID string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedID = r.Header.Get("X-Request-ID")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithMiddleware(RequestIDMiddleware("X-Request-ID")),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		assert.NotEmpty(t, receivedID)
	})

	t.Run("uses existing request ID from context", func(t *testing.T) {
		var receivedID string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedID = r.Header.Get("X-Request-ID")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithMiddleware(RequestIDMiddleware("X-Request-ID")),
		)
		require.NoError(t, err)

		ctx := WithRequestID(context.Background(), "my-custom-id")
		_, err = client.Get(ctx, "/test", nil)
		require.NoError(t, err)

		assert.Equal(t, "my-custom-id", receivedID)
	})

	t.Run("GetRequestID retrieves ID from context", func(t *testing.T) {
		ctx := WithRequestID(context.Background(), "test-id")
		assert.Equal(t, "test-id", GetRequestID(ctx))
	})

	t.Run("GetRequestID returns empty for missing ID", func(t *testing.T) {
		assert.Equal(t, "", GetRequestID(context.Background()))
	})
}
