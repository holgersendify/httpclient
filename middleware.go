package httpclient

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// RoundTripFunc is the function signature for making HTTP requests.
type RoundTripFunc func(*http.Request) (*http.Response, error)

// Middleware wraps HTTP requests to add cross-cutting functionality.
type Middleware func(req *http.Request, next RoundTripFunc) (*http.Response, error)

// requestIDKey is the context key for request IDs.
type requestIDKey struct{}

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// RequestIDMiddleware adds a unique request ID to each request.
func RequestIDMiddleware(headerName string) Middleware {
	return func(req *http.Request, next RoundTripFunc) (*http.Response, error) {
		id := GetRequestID(req.Context())
		if id == "" {
			id = uuid.New().String()
		}
		req.Header.Set(headerName, id)
		return next(req)
	}
}
