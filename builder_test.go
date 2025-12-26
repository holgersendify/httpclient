package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestBuilder(t *testing.T) {
	t.Run("builds and executes request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/users", r.URL.Path)
			assert.Equal(t, "bar", r.URL.Query().Get("foo"))
			assert.Equal(t, "custom-value", r.Header.Get("X-Custom"))

			body, _ := io.ReadAll(r.Body)
			var received map[string]string
			_ = json.Unmarshal(body, &received)
			assert.Equal(t, "test", received["name"])

			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":1}`))
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		resp, err := client.Request().
			Method(http.MethodPost).
			Path("/users").
			Query("foo", "bar").
			Header("X-Custom", "custom-value").
			Body(map[string]string{"name": "test"}).
			Do(context.Background())

		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("DoInto unmarshals response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":42,"name":"test"}`))
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		var result struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		err = client.Request().
			Method(http.MethodGet).
			Path("/users/42").
			DoInto(context.Background(), &result)

		require.NoError(t, err)
		assert.Equal(t, 42, result.ID)
		assert.Equal(t, "test", result.Name)
	})

	t.Run("supports multiple query params", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "a", r.URL.Query().Get("x"))
			assert.Equal(t, "b", r.URL.Query().Get("y"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		_, err = client.Request().
			Path("/search").
			Query("x", "a").
			Query("y", "b").
			Do(context.Background())

		require.NoError(t, err)
	})

	t.Run("defaults to GET method", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		_, err = client.Request().
			Path("/test").
			Do(context.Background())

		require.NoError(t, err)
	})
}

func TestRequestOption(t *testing.T) {
	t.Run("WithRequestHeader adds header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "test-value", r.Header.Get("X-Test"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil,
			WithRequestHeader("X-Test", "test-value"))

		require.NoError(t, err)
	})

	t.Run("WithQuery adds query param", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "value", r.URL.Query().Get("key"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/test", nil,
			WithQuery("key", "value"))

		require.NoError(t, err)
	})

	t.Run("WithContentType overrides content type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		_, err = client.Post(context.Background(), "/test", "plain text", nil,
			WithContentType("text/plain"))

		require.NoError(t, err)
	})

	t.Run("WithRequestTimeout sets timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/slow", nil,
			WithRequestTimeout(10*time.Millisecond))

		require.Error(t, err)
	})
}
