package httpclient

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("returns error when baseURL is missing", func(t *testing.T) {
		client, err := New()

		require.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "base URL is required")
	})

	t.Run("succeeds with valid baseURL", func(t *testing.T) {
		client, err := New(WithBaseURL("https://api.example.com"))

		require.NoError(t, err)
		require.NotNil(t, client)
		assert.Equal(t, "https://api.example.com", client.baseURL.String())
	})

	t.Run("sets default values", func(t *testing.T) {
		client, err := New(WithBaseURL("https://api.example.com"))

		require.NoError(t, err)
		assert.Equal(t, 30*time.Second, client.timeout)
		assert.Equal(t, "application/json", client.defaultContentType)
		assert.Equal(t, "httpclient/"+Version, client.headers.Get("User-Agent"))
		assert.Equal(t, "application/json", client.headers.Get("Accept"))
	})
}

func TestWithBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantErr string
	}{
		{
			name:    "valid URL",
			baseURL: "https://api.example.com",
			wantErr: "",
		},
		{
			name:    "valid URL with path",
			baseURL: "https://api.example.com/v1",
			wantErr: "",
		},
		{
			name:    "empty URL",
			baseURL: "",
			wantErr: "base URL cannot be empty",
		},
		{
			name:    "invalid URL",
			baseURL: "://invalid",
			wantErr: "missing protocol scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(WithBaseURL(tt.baseURL))

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				require.NotNil(t, client)
				assert.Equal(t, tt.baseURL, client.baseURL.String())
			}
		})
	}
}

func TestWithHTTPClient(t *testing.T) {
	t.Run("sets custom http client", func(t *testing.T) {
		customClient := &http.Client{Timeout: 60 * time.Second}

		client, err := New(
			WithBaseURL("https://api.example.com"),
			WithHTTPClient(customClient),
		)

		require.NoError(t, err)
		assert.Same(t, customClient, client.httpClient)
	})

	t.Run("returns error for nil client", func(t *testing.T) {
		_, err := New(
			WithBaseURL("https://api.example.com"),
			WithHTTPClient(nil),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "http client cannot be nil")
	})
}

func TestWithTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		wantErr string
	}{
		{
			name:    "valid timeout",
			timeout: 10 * time.Second,
			wantErr: "",
		},
		{
			name:    "zero timeout",
			timeout: 0,
			wantErr: "timeout must be positive",
		},
		{
			name:    "negative timeout",
			timeout: -1 * time.Second,
			wantErr: "timeout must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(
				WithBaseURL("https://api.example.com"),
				WithTimeout(tt.timeout),
			)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.timeout, client.timeout)
			}
		})
	}
}

func TestWithHeader(t *testing.T) {
	t.Run("adds single header", func(t *testing.T) {
		client, err := New(
			WithBaseURL("https://api.example.com"),
			WithHeader("X-Custom", "value"),
		)

		require.NoError(t, err)
		assert.Equal(t, "value", client.headers.Get("X-Custom"))
	})

	t.Run("returns error for empty key", func(t *testing.T) {
		_, err := New(
			WithBaseURL("https://api.example.com"),
			WithHeader("", "value"),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "header key cannot be empty")
	})
}

func TestWithHeaders(t *testing.T) {
	t.Run("adds multiple headers", func(t *testing.T) {
		client, err := New(
			WithBaseURL("https://api.example.com"),
			WithHeaders(map[string]string{
				"X-First":  "one",
				"X-Second": "two",
			}),
		)

		require.NoError(t, err)
		assert.Equal(t, "one", client.headers.Get("X-First"))
		assert.Equal(t, "two", client.headers.Get("X-Second"))
	})

	t.Run("returns error for empty key in map", func(t *testing.T) {
		_, err := New(
			WithBaseURL("https://api.example.com"),
			WithHeaders(map[string]string{
				"":         "value",
				"X-Second": "two",
			}),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "header key cannot be empty")
	})
}

func TestWithUserAgent(t *testing.T) {
	t.Run("overrides default user agent", func(t *testing.T) {
		client, err := New(
			WithBaseURL("https://api.example.com"),
			WithUserAgent("custom-agent/1.0"),
		)

		require.NoError(t, err)
		assert.Equal(t, "custom-agent/1.0", client.headers.Get("User-Agent"))
	})
}

func TestWithDefaultContentType(t *testing.T) {
	t.Run("sets content type", func(t *testing.T) {
		client, err := New(
			WithBaseURL("https://api.example.com"),
			WithDefaultContentType("application/xml"),
		)

		require.NoError(t, err)
		assert.Equal(t, "application/xml", client.defaultContentType)
	})

	t.Run("returns error for empty content type", func(t *testing.T) {
		_, err := New(
			WithBaseURL("https://api.example.com"),
			WithDefaultContentType(""),
		)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "content type cannot be empty")
	})
}
