package httpclient

import (
	"context"
	"encoding/base64"
	"net/http"
)

// AuthProvider applies authentication to HTTP requests.
type AuthProvider interface {
	Apply(req *http.Request) error
}

// AuthFunc is a function that implements AuthProvider.
type AuthFunc func(req *http.Request) error

// Apply implements AuthProvider.
func (f AuthFunc) Apply(req *http.Request) error {
	return f(req)
}

// BearerAuth returns an AuthProvider that adds a Bearer token header.
func BearerAuth(token string) AuthProvider {
	return AuthFunc(func(req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	})
}

// BasicAuth returns an AuthProvider that adds HTTP Basic authentication.
func BasicAuth(username, password string) AuthProvider {
	encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return AuthFunc(func(req *http.Request) error {
		req.Header.Set("Authorization", "Basic "+encoded)
		return nil
	})
}

// APIKeyAuth returns an AuthProvider that adds an API key header.
func APIKeyAuth(headerName, apiKey string) AuthProvider {
	return AuthFunc(func(req *http.Request) error {
		req.Header.Set(headerName, apiKey)
		return nil
	})
}

// APIKeyQueryAuth returns an AuthProvider that adds an API key to the query string.
func APIKeyQueryAuth(paramName, apiKey string) AuthProvider {
	return AuthFunc(func(req *http.Request) error {
		q := req.URL.Query()
		q.Set(paramName, apiKey)
		req.URL.RawQuery = q.Encode()
		return nil
	})
}

// TokenSource provides tokens dynamically, useful for refreshable tokens.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// TokenSourceFunc is a function that implements TokenSource.
type TokenSourceFunc func(ctx context.Context) (string, error)

// Token implements TokenSource.
func (f TokenSourceFunc) Token(ctx context.Context) (string, error) {
	return f(ctx)
}

// TokenAuth returns an AuthProvider that fetches tokens from a TokenSource.
func TokenAuth(source TokenSource) AuthProvider {
	return AuthFunc(func(req *http.Request) error {
		token, err := source.Token(req.Context())
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	})
}
