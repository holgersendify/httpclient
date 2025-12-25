// Package httpclient provides a production-grade HTTP client for Go.
package httpclient

import (
	"errors"
	"net/http"
	"net/url"
	"time"
)

// Version is the current version of the httpclient package.
const Version = "0.1.0"

// Client is an immutable HTTP client configured via functional options.
// It is safe for concurrent use across goroutines.
type Client struct {
	baseURL            *url.URL
	httpClient         *http.Client
	timeout            time.Duration
	headers            http.Header
	defaultContentType string
}

// ClientOption configures a Client.
type ClientOption func(*Client) error

// New creates a new Client with the given options.
// Returns an error if required options are missing or invalid.
func New(opts ...ClientOption) (*Client, error) {
	c := &Client{
		httpClient:         &http.Client{},
		timeout:            30 * time.Second,
		headers:            make(http.Header),
		defaultContentType: "application/json",
	}

	c.headers.Set("User-Agent", "httpclient/"+Version)
	c.headers.Set("Accept", "application/json")

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	if c.baseURL == nil {
		return nil, errors.New("base URL is required: use WithBaseURL option")
	}

	return c, nil
}

// WithBaseURL sets the base URL for all requests.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) error {
		if baseURL == "" {
			return errors.New("base URL cannot be empty")
		}
		u, err := url.Parse(baseURL)
		if err != nil {
			return err
		}
		c.baseURL = u
		return nil
	}
}

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) error {
		if client == nil {
			return errors.New("http client cannot be nil")
		}
		c.httpClient = client
		return nil
	}
}

// WithTimeout sets the default request timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) error {
		if d <= 0 {
			return errors.New("timeout must be positive")
		}
		c.timeout = d
		return nil
	}
}

// WithHeader adds a default header to all requests.
func WithHeader(key, value string) ClientOption {
	return func(c *Client) error {
		if key == "" {
			return errors.New("header key cannot be empty")
		}
		c.headers.Set(key, value)
		return nil
	}
}

// WithHeaders adds multiple default headers to all requests.
func WithHeaders(headers map[string]string) ClientOption {
	return func(c *Client) error {
		for k, v := range headers {
			if k == "" {
				return errors.New("header key cannot be empty")
			}
			c.headers.Set(k, v)
		}
		return nil
	}
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(userAgent string) ClientOption {
	return func(c *Client) error {
		c.headers.Set("User-Agent", userAgent)
		return nil
	}
}

// WithDefaultContentType sets the default Content-Type for requests with bodies.
func WithDefaultContentType(contentType string) ClientOption {
	return func(c *Client) error {
		if contentType == "" {
			return errors.New("content type cannot be empty")
		}
		c.defaultContentType = contentType
		return nil
	}
}
