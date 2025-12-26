package httpclient

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBearerAuth(t *testing.T) {
	t.Run("adds bearer token header", func(t *testing.T) {
		var receivedAuth string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithAuth(BearerAuth("my-token")),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		assert.Equal(t, "Bearer my-token", receivedAuth)
	})
}

func TestBasicAuth(t *testing.T) {
	t.Run("adds basic auth header", func(t *testing.T) {
		var receivedAuth string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithAuth(BasicAuth("user", "pass")),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
		assert.Equal(t, expected, receivedAuth)
	})
}

func TestAPIKeyAuth(t *testing.T) {
	t.Run("adds api key header", func(t *testing.T) {
		var receivedKey string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedKey = r.Header.Get("X-API-Key")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithAuth(APIKeyAuth("X-API-Key", "secret-key")),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		assert.Equal(t, "secret-key", receivedKey)
	})

	t.Run("adds api key to query string", func(t *testing.T) {
		var receivedKey string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedKey = r.URL.Query().Get("api_key")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(
			WithBaseURL(server.URL),
			WithAuth(APIKeyQueryAuth("api_key", "secret-key")),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		assert.Equal(t, "secret-key", receivedKey)
	})
}

func TestAuthFunc(t *testing.T) {
	t.Run("calls custom auth function", func(t *testing.T) {
		var receivedAuth string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("X-Custom-Auth")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		customAuth := AuthFunc(func(req *http.Request) error {
			req.Header.Set("X-Custom-Auth", "custom-value")
			return nil
		})

		client, err := New(
			WithBaseURL(server.URL),
			WithAuth(customAuth),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		assert.Equal(t, "custom-value", receivedAuth)
	})

	t.Run("returns error from auth function", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		authErr := errors.New("auth failed")
		failingAuth := AuthFunc(func(req *http.Request) error {
			return authErr
		})

		client, err := New(
			WithBaseURL(server.URL),
			WithAuth(failingAuth),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "auth failed")
	})
}

func TestTokenSource(t *testing.T) {
	t.Run("fetches token from source", func(t *testing.T) {
		var receivedAuth string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		source := TokenSourceFunc(func(ctx context.Context) (string, error) {
			return "dynamic-token", nil
		})

		client, err := New(
			WithBaseURL(server.URL),
			WithAuth(TokenAuth(source)),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		assert.Equal(t, "Bearer dynamic-token", receivedAuth)
	})

	t.Run("calls source for each request", func(t *testing.T) {
		var callCount int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		source := TokenSourceFunc(func(ctx context.Context) (string, error) {
			atomic.AddInt32(&callCount, 1)
			return "token", nil
		})

		client, err := New(
			WithBaseURL(server.URL),
			WithAuth(TokenAuth(source)),
		)
		require.NoError(t, err)

		_, _ = client.Get(context.Background(), "/test", nil)
		_, _ = client.Get(context.Background(), "/test", nil)

		assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
	})

	t.Run("returns error from token source", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		source := TokenSourceFunc(func(ctx context.Context) (string, error) {
			return "", errors.New("token fetch failed")
		})

		client, err := New(
			WithBaseURL(server.URL),
			WithAuth(TokenAuth(source)),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token fetch failed")
	})
}
