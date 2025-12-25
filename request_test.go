package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Get(t *testing.T) {
	t.Run("successful GET request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/users/123", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"name": "test"})
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		var result struct {
			Name string `json:"name"`
		}
		resp, err := client.Get(context.Background(), "/users/123", &result)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "test", result.Name)
	})

	t.Run("includes default headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.Header.Get("User-Agent"), "httpclient/")
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)
	})

	t.Run("returns error for non-2xx status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/missing", nil)

		require.Error(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		var httpErr *Error
		require.ErrorAs(t, err, &httpErr)
		assert.Equal(t, ErrKindHTTP, httpErr.Kind)
		assert.Equal(t, http.StatusNotFound, httpErr.StatusCode)
	})

}

func TestClient_Post(t *testing.T) {
	t.Run("successful POST with JSON body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/users", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			body, _ := io.ReadAll(r.Body)
			var received map[string]string
			_ = json.Unmarshal(body, &received)
			assert.Equal(t, "test", received["name"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "name": "test"})
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		requestBody := map[string]string{"name": "test"}
		var result struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		resp, err := client.Post(context.Background(), "/users", requestBody, &result)

		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, 1, result.ID)
		assert.Equal(t, "test", result.Name)
	})

	t.Run("POST with nil body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			assert.Empty(t, body)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		_, err = client.Post(context.Background(), "/action", nil, nil)
		require.NoError(t, err)
	})
}

func TestClient_Put(t *testing.T) {
	t.Run("successful PUT request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		_, err = client.Put(context.Background(), "/users/1", map[string]string{"name": "updated"}, nil)
		require.NoError(t, err)
	})
}

func TestClient_Patch(t *testing.T) {
	t.Run("successful PATCH request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPatch, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		_, err = client.Patch(context.Background(), "/users/1", map[string]string{"name": "patched"}, nil)
		require.NoError(t, err)
	})
}

func TestClient_Delete(t *testing.T) {
	t.Run("successful DELETE request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Equal(t, "/users/1", r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		resp, err := client.Delete(context.Background(), "/users/1", nil)

		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})
}

func TestClient_ContextCancellation(t *testing.T) {
	t.Run("respects context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = client.Get(ctx, "/test", nil)
		require.Error(t, err)
	})
}
