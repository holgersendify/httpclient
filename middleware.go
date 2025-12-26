package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LogFunc is a function that logs a message.
type LogFunc func(msg string)

// defaultSensitiveHeaders are headers that should be redacted by default.
var defaultSensitiveHeaders = []string{
	"Authorization",
	"Cookie",
	"Set-Cookie",
	"X-Auth-Token",
	"X-Api-Key",
}

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

// LoggingMiddleware logs HTTP requests and responses with default header redaction.
func LoggingMiddleware(log LogFunc) Middleware {
	return LoggingMiddlewareWithRedaction(log, nil)
}

// LoggingMiddlewareWithRedaction logs HTTP requests and responses with custom header redaction.
func LoggingMiddlewareWithRedaction(log LogFunc, additionalSensitive []string) Middleware {
	sensitive := make(map[string]struct{})
	for _, h := range defaultSensitiveHeaders {
		sensitive[strings.ToLower(h)] = struct{}{}
	}
	for _, h := range additionalSensitive {
		sensitive[strings.ToLower(h)] = struct{}{}
	}

	return func(req *http.Request, next RoundTripFunc) (*http.Response, error) {
		start := time.Now()

		// Log request with redacted headers
		headers := redactHeaders(req.Header, sensitive)
		log(fmt.Sprintf("HTTP Request: %s %s headers=%v", req.Method, req.URL.Path, headers))

		resp, err := next(req)
		duration := time.Since(start)

		if err != nil {
			log(fmt.Sprintf("HTTP Response: error=%v duration=%v", err, duration))
			return resp, err
		}

		// Log response
		log(fmt.Sprintf("HTTP Response: status=%d duration=%v", resp.StatusCode, duration))
		return resp, nil
	}
}

func redactHeaders(headers http.Header, sensitive map[string]struct{}) map[string]string {
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		if _, isSensitive := sensitive[strings.ToLower(key)]; isSensitive {
			result[key] = "[REDACTED]"
		} else {
			result[key] = strings.Join(values, ", ")
		}
	}
	return result
}
