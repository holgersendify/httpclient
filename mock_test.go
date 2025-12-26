package httpclient

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockTransport(t *testing.T) {
	t.Run("returns configured response", func(t *testing.T) {
		mock := NewMockTransport()
		mock.AddResponse("/users", http.StatusOK, map[string]string{"name": "Alice"})

		client, err := New(
			WithBaseURL("http://api.example.com"),
			WithHTTPClient(&http.Client{Transport: mock}),
		)
		require.NoError(t, err)

		var result map[string]string
		resp, err := client.Get(context.Background(), "/users", &result)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "Alice", result["name"])
	})

	t.Run("returns different responses for different paths", func(t *testing.T) {
		mock := NewMockTransport()
		mock.AddResponse("/users", http.StatusOK, map[string]string{"type": "users"})
		mock.AddResponse("/posts", http.StatusOK, map[string]string{"type": "posts"})

		client, err := New(
			WithBaseURL("http://api.example.com"),
			WithHTTPClient(&http.Client{Transport: mock}),
		)
		require.NoError(t, err)

		var result1 map[string]string
		_, err = client.Get(context.Background(), "/users", &result1)
		require.NoError(t, err)
		assert.Equal(t, "users", result1["type"])

		var result2 map[string]string
		_, err = client.Get(context.Background(), "/posts", &result2)
		require.NoError(t, err)
		assert.Equal(t, "posts", result2["type"])
	})

	t.Run("returns error for unregistered path", func(t *testing.T) {
		mock := NewMockTransport()

		client, err := New(
			WithBaseURL("http://api.example.com"),
			WithHTTPClient(&http.Client{Transport: mock}),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/unknown", nil)
		require.Error(t, err)
	})

	t.Run("matches method and path", func(t *testing.T) {
		mock := NewMockTransport()
		mock.AddResponseForMethod("GET", "/resource", http.StatusOK, "get response")
		mock.AddResponseForMethod("POST", "/resource", http.StatusCreated, "post response")

		client, err := New(
			WithBaseURL("http://api.example.com"),
			WithHTTPClient(&http.Client{Transport: mock}),
		)
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/resource", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = client.Post(context.Background(), "/resource", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})
}

func TestMockTransportRecording(t *testing.T) {
	t.Run("records all requests", func(t *testing.T) {
		mock := NewMockTransport()
		mock.AddResponse("/users", http.StatusOK, nil)
		mock.AddResponse("/posts", http.StatusOK, nil)

		client, err := New(
			WithBaseURL("http://api.example.com"),
			WithHTTPClient(&http.Client{Transport: mock}),
		)
		require.NoError(t, err)

		_, _ = client.Get(context.Background(), "/users", nil)
		_, _ = client.Post(context.Background(), "/posts", map[string]string{"title": "Hello"}, nil)

		requests := mock.Requests()
		require.Len(t, requests, 2)

		assert.Equal(t, "GET", requests[0].Method)
		assert.Equal(t, "/users", requests[0].URL.Path)

		assert.Equal(t, "POST", requests[1].Method)
		assert.Equal(t, "/posts", requests[1].URL.Path)
	})

	t.Run("asserts request was made", func(t *testing.T) {
		mock := NewMockTransport()
		mock.AddResponse("/users", http.StatusOK, nil)

		client, err := New(
			WithBaseURL("http://api.example.com"),
			WithHTTPClient(&http.Client{Transport: mock}),
		)
		require.NoError(t, err)

		_, _ = client.Get(context.Background(), "/users", nil)

		assert.True(t, mock.WasCalled("/users"))
		assert.False(t, mock.WasCalled("/posts"))
	})

	t.Run("asserts request count", func(t *testing.T) {
		mock := NewMockTransport()
		mock.AddResponse("/users", http.StatusOK, nil)

		client, err := New(
			WithBaseURL("http://api.example.com"),
			WithHTTPClient(&http.Client{Transport: mock}),
		)
		require.NoError(t, err)

		_, _ = client.Get(context.Background(), "/users", nil)
		_, _ = client.Get(context.Background(), "/users", nil)
		_, _ = client.Get(context.Background(), "/users", nil)

		assert.Equal(t, 3, mock.CallCount("/users"))
		assert.Equal(t, 0, mock.CallCount("/posts"))
	})

	t.Run("resets recorded requests", func(t *testing.T) {
		mock := NewMockTransport()
		mock.AddResponse("/users", http.StatusOK, nil)

		client, err := New(
			WithBaseURL("http://api.example.com"),
			WithHTTPClient(&http.Client{Transport: mock}),
		)
		require.NoError(t, err)

		_, _ = client.Get(context.Background(), "/users", nil)
		assert.Equal(t, 1, mock.CallCount("/users"))

		mock.Reset()
		assert.Equal(t, 0, mock.CallCount("/users"))
		assert.Empty(t, mock.Requests())
	})
}

func TestMockTransportHandlers(t *testing.T) {
	t.Run("uses custom handler function", func(t *testing.T) {
		mock := NewMockTransport()
		mock.AddHandler("/echo", func(req *http.Request) (*http.Response, error) {
			return MockJSONResponse(http.StatusOK, map[string]string{
				"method": req.Method,
				"path":   req.URL.Path,
			}), nil
		})

		client, err := New(
			WithBaseURL("http://api.example.com"),
			WithHTTPClient(&http.Client{Transport: mock}),
		)
		require.NoError(t, err)

		var result map[string]string
		_, err = client.Get(context.Background(), "/echo", &result)
		require.NoError(t, err)

		assert.Equal(t, "GET", result["method"])
		assert.Equal(t, "/echo", result["path"])
	})

	t.Run("handler can return error", func(t *testing.T) {
		mock := NewMockTransport()
		mock.AddHandler("/error", func(req *http.Request) (*http.Response, error) {
			return nil, MockNetworkError("connection refused")
		})

		client, err := New(
			WithBaseURL("http://api.example.com"),
			WithHTTPClient(&http.Client{Transport: mock}),
		)
		require.NoError(t, err)

		_, err = client.Get(context.Background(), "/error", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection refused")
	})

	t.Run("returns responses in sequence", func(t *testing.T) {
		mock := NewMockTransport()
		mock.AddResponseSequence("/flaky",
			MockJSONResponse(http.StatusServiceUnavailable, nil),
			MockJSONResponse(http.StatusServiceUnavailable, nil),
			MockJSONResponse(http.StatusOK, map[string]string{"status": "ok"}),
		)

		client, err := New(
			WithBaseURL("http://api.example.com"),
			WithHTTPClient(&http.Client{Transport: mock}),
		)
		require.NoError(t, err)

		resp, _ := client.Get(context.Background(), "/flaky", nil)
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		resp, _ = client.Get(context.Background(), "/flaky", nil)
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		var result map[string]string
		resp, err = client.Get(context.Background(), "/flaky", &result)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "ok", result["status"])
	})
}

func TestMockResponseHelpers(t *testing.T) {
	t.Run("creates JSON response", func(t *testing.T) {
		resp := MockJSONResponse(http.StatusOK, map[string]int{"count": 42})

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})

	t.Run("creates error response", func(t *testing.T) {
		resp := MockErrorResponse(http.StatusNotFound, "resource not found")

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
