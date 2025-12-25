package httpclient

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorKind(t *testing.T) {
	t.Run("has expected values", func(t *testing.T) {
		assert.Equal(t, ErrorKind(0), ErrKindUnknown)
		assert.Equal(t, ErrorKind(1), ErrKindTimeout)
		assert.Equal(t, ErrorKind(2), ErrKindNetwork)
		assert.Equal(t, ErrorKind(3), ErrKindHTTP)
		assert.Equal(t, ErrorKind(4), ErrKindParse)
		assert.Equal(t, ErrorKind(5), ErrKindRateLimit)
	})
}

func TestError_Error(t *testing.T) {
	t.Run("formats with underlying error", func(t *testing.T) {
		err := &Error{
			Kind:   ErrKindNetwork,
			Method: "GET",
			URL:    "https://api.example.com/users",
			Err:    errors.New("connection refused"),
		}

		assert.Equal(t, "GET https://api.example.com/users: connection refused", err.Error())
	})

	t.Run("formats with status when no underlying error", func(t *testing.T) {
		err := &Error{
			Kind:       ErrKindHTTP,
			StatusCode: 404,
			Status:     "404 Not Found",
			Method:     "GET",
			URL:        "https://api.example.com/users/123",
		}

		assert.Equal(t, "GET https://api.example.com/users/123: 404 Not Found", err.Error())
	})
}

func TestError_Unwrap(t *testing.T) {
	t.Run("returns underlying error", func(t *testing.T) {
		underlying := errors.New("connection refused")
		err := &Error{
			Kind: ErrKindNetwork,
			Err:  underlying,
		}

		assert.Equal(t, underlying, err.Unwrap())
	})

	t.Run("returns nil when no underlying error", func(t *testing.T) {
		err := &Error{Kind: ErrKindHTTP}

		assert.Nil(t, err.Unwrap())
	})
}

func TestError_IsTimeout(t *testing.T) {
	tests := []struct {
		name     string
		kind     ErrorKind
		expected bool
	}{
		{"timeout error", ErrKindTimeout, true},
		{"network error", ErrKindNetwork, false},
		{"http error", ErrKindHTTP, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{Kind: tt.kind}
			assert.Equal(t, tt.expected, err.IsTimeout())
		})
	}
}

func TestError_IsNetwork(t *testing.T) {
	tests := []struct {
		name     string
		kind     ErrorKind
		expected bool
	}{
		{"network error", ErrKindNetwork, true},
		{"timeout error", ErrKindTimeout, false},
		{"http error", ErrKindHTTP, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{Kind: tt.kind}
			assert.Equal(t, tt.expected, err.IsNetwork())
		})
	}
}

func TestError_IsRetryable(t *testing.T) {
	tests := []struct {
		name       string
		kind       ErrorKind
		statusCode int
		expected   bool
	}{
		{"timeout is retryable", ErrKindTimeout, 0, true},
		{"network is retryable", ErrKindNetwork, 0, true},
		{"408 is retryable", ErrKindHTTP, http.StatusRequestTimeout, true},
		{"429 is retryable", ErrKindHTTP, http.StatusTooManyRequests, true},
		{"502 is retryable", ErrKindHTTP, http.StatusBadGateway, true},
		{"503 is retryable", ErrKindHTTP, http.StatusServiceUnavailable, true},
		{"504 is retryable", ErrKindHTTP, http.StatusGatewayTimeout, true},
		{"400 is not retryable", ErrKindHTTP, http.StatusBadRequest, false},
		{"401 is not retryable", ErrKindHTTP, http.StatusUnauthorized, false},
		{"404 is not retryable", ErrKindHTTP, http.StatusNotFound, false},
		{"500 is not retryable", ErrKindHTTP, http.StatusInternalServerError, false},
		{"parse error is not retryable", ErrKindParse, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{Kind: tt.kind, StatusCode: tt.statusCode}
			assert.Equal(t, tt.expected, err.IsRetryable())
		})
	}
}

func TestError_IsClientError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"400 is client error", 400, true},
		{"404 is client error", 404, true},
		{"499 is client error", 499, true},
		{"399 is not client error", 399, false},
		{"500 is not client error", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{StatusCode: tt.statusCode}
			assert.Equal(t, tt.expected, err.IsClientError())
		})
	}
}

func TestError_IsServerError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"500 is server error", 500, true},
		{"503 is server error", 503, true},
		{"599 is server error", 599, true},
		{"499 is not server error", 499, false},
		{"600 is not server error", 600, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{StatusCode: tt.statusCode}
			assert.Equal(t, tt.expected, err.IsServerError())
		})
	}
}

func TestError_IsStatus(t *testing.T) {
	t.Run("matches single status code", func(t *testing.T) {
		err := &Error{StatusCode: 404}

		assert.True(t, err.IsStatus(404))
		assert.False(t, err.IsStatus(500))
	})

	t.Run("matches any of multiple status codes", func(t *testing.T) {
		err := &Error{StatusCode: 503}

		assert.True(t, err.IsStatus(502, 503, 504))
		assert.False(t, err.IsStatus(400, 401, 404))
	})

	t.Run("returns false for empty codes", func(t *testing.T) {
		err := &Error{StatusCode: 404}

		assert.False(t, err.IsStatus())
	})
}

func TestError_ImplementsErrorInterface(t *testing.T) {
	var _ error = &Error{}
}

func TestError_WorksWithErrorsIs(t *testing.T) {
	underlying := errors.New("connection refused")
	err := &Error{
		Kind: ErrKindNetwork,
		Err:  underlying,
	}

	require.True(t, errors.Is(err, underlying))
}
