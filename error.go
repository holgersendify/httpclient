package httpclient

import (
	"fmt"
	"net/http"
)

// ErrorKind classifies the type of error.
type ErrorKind int

const (
	ErrKindUnknown ErrorKind = iota
	ErrKindTimeout
	ErrKindNetwork
	ErrKindHTTP
	ErrKindParse
	ErrKindRateLimit
)

// Error represents an HTTP client error with classification and context.
type Error struct {
	Kind       ErrorKind
	StatusCode int
	Status     string
	Body       []byte
	Headers    http.Header
	Method     string
	URL        string
	Attempts   int
	Err        error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s %s: %v", e.Method, e.URL, e.Err)
	}
	return fmt.Sprintf("%s %s: %s", e.Method, e.URL, e.Status)
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Err
}

// IsTimeout returns true if the error is a timeout.
func (e *Error) IsTimeout() bool {
	return e.Kind == ErrKindTimeout
}

// IsNetwork returns true if the error is network-related.
func (e *Error) IsNetwork() bool {
	return e.Kind == ErrKindNetwork
}

// IsRetryable returns true if the request can be retried.
func (e *Error) IsRetryable() bool {
	switch e.Kind {
	case ErrKindTimeout, ErrKindNetwork:
		return true
	case ErrKindHTTP:
		switch e.StatusCode {
		case http.StatusRequestTimeout,
			http.StatusTooManyRequests,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout:
			return true
		}
	}
	return false
}

// IsClientError returns true if the status code is 4xx.
func (e *Error) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// IsServerError returns true if the status code is 5xx.
func (e *Error) IsServerError() bool {
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// IsStatus returns true if the status code matches any of the given codes.
func (e *Error) IsStatus(codes ...int) bool {
	for _, code := range codes {
		if e.StatusCode == code {
			return true
		}
	}
	return false
}
