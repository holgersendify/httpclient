// Package httpclient provides a production-grade HTTP client for Go.
package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"sendify/httpclient/internal"
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
	retryPolicy        *RetryPolicy
	rateLimiter        *RateLimiter
	middlewares        []Middleware
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

// WithRetry sets the retry policy.
func WithRetry(policy *RetryPolicy) ClientOption {
	return func(c *Client) error {
		c.retryPolicy = policy
		return nil
	}
}

// WithRateLimit configures client-side rate limiting.
func WithRateLimit(requests int, duration time.Duration) ClientOption {
	return func(c *Client) error {
		c.rateLimiter = NewRateLimiter(requests, duration)
		return nil
	}
}

// WithMiddleware adds a middleware to the client's middleware chain.
func WithMiddleware(mw Middleware) ClientOption {
	return func(c *Client) error {
		if mw == nil {
			return errors.New("middleware cannot be nil")
		}
		c.middlewares = append(c.middlewares, mw)
		return nil
	}
}

// Get performs an HTTP GET request.
func (c *Client) Get(ctx context.Context, path string, result any, opts ...RequestOption) (*Response, error) {
	return c.doWithOptions(ctx, http.MethodGet, path, nil, result, opts)
}

// Post performs an HTTP POST request.
func (c *Client) Post(ctx context.Context, path string, body any, result any, opts ...RequestOption) (*Response, error) {
	return c.doWithOptions(ctx, http.MethodPost, path, body, result, opts)
}

// Put performs an HTTP PUT request.
func (c *Client) Put(ctx context.Context, path string, body any, result any, opts ...RequestOption) (*Response, error) {
	return c.doWithOptions(ctx, http.MethodPut, path, body, result, opts)
}

// Patch performs an HTTP PATCH request.
func (c *Client) Patch(ctx context.Context, path string, body any, result any, opts ...RequestOption) (*Response, error) {
	return c.doWithOptions(ctx, http.MethodPatch, path, body, result, opts)
}

// Delete performs an HTTP DELETE request.
func (c *Client) Delete(ctx context.Context, path string, result any, opts ...RequestOption) (*Response, error) {
	return c.doWithOptions(ctx, http.MethodDelete, path, nil, result, opts)
}

func (c *Client) doWithOptions(ctx context.Context, method, path string, body any, result any, opts []RequestOption) (*Response, error) {
	cfg := newRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	reqURL := c.baseURL.JoinPath(path)

	if len(cfg.query) > 0 {
		q := reqURL.Query()
		for key, values := range cfg.query {
			for _, value := range values {
				q.Add(key, value)
			}
		}
		reqURL.RawQuery = q.Encode()
	}

	// Encode body once for potential replay
	bodyReader, contentType, err := internal.EncodeBody(body)
	if err != nil {
		return nil, err
	}

	var bodyBytes []byte
	if bodyReader != nil {
		bodyBytes, err = io.ReadAll(bodyReader)
		if err != nil {
			return nil, err
		}
	}

	// Apply rate limiting
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, &Error{
				Kind:   ErrKindRateLimit,
				Method: method,
				URL:    reqURL.String(),
				Err:    err,
			}
		}
	}

	if cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}

	maxAttempts := 1
	if c.retryPolicy != nil {
		maxAttempts = c.retryPolicy.MaxAttempts
	}

	var response *Response
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Create fresh body reader for each attempt
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), reqBody)
		if err != nil {
			return nil, err
		}

		for key, values := range c.headers {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		for key, values := range cfg.headers {
			for _, value := range values {
				req.Header.Set(key, value)
			}
		}

		if cfg.contentType != "" {
			req.Header.Set("Content-Type", cfg.contentType)
		} else if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		} else if body != nil {
			req.Header.Set("Content-Type", c.defaultContentType)
		}

		// Build middleware chain
		transport := func(r *http.Request) (*http.Response, error) {
			return c.httpClient.Do(r)
		}

		// Wrap transport with middlewares (in reverse order so first added executes first)
		for i := len(c.middlewares) - 1; i >= 0; i-- {
			mw := c.middlewares[i]
			next := transport
			transport = func(r *http.Request) (*http.Response, error) {
				return mw(r, next)
			}
		}

		resp, err := transport(req)
		if err != nil {
			lastErr = c.wrapError(err, method, reqURL.String())
			// Network errors are retryable
			if c.retryPolicy != nil && attempt < maxAttempts {
				c.waitForRetry(ctx, c.retryPolicy.Backoff(attempt))
				continue
			}
			return nil, lastErr
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		response = &Response{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Headers:    resp.Header,
			Body:       respBody,
		}

		if resp.StatusCode >= 400 {
			lastErr = &Error{
				Kind:       ErrKindHTTP,
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Body:       respBody,
				Headers:    resp.Header,
				Method:     method,
				URL:        reqURL.String(),
				Attempts:   attempt,
			}

			// Check if we should retry
			if c.retryPolicy != nil && attempt < maxAttempts && c.retryPolicy.ShouldRetry(resp.StatusCode) {
				delay := c.retryPolicy.Backoff(attempt)

				// Check for Retry-After header
				if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
					if parsed := ParseRetryAfter(retryAfter); parsed > 0 {
						delay = parsed
					}
				}

				c.waitForRetry(ctx, delay)
				continue
			}

			return response, lastErr
		}

		// Success
		if result != nil && len(respBody) > 0 {
			if err := response.JSON(result); err != nil {
				return response, err
			}
		}

		return response, nil
	}

	return response, lastErr
}

func (c *Client) waitForRetry(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (c *Client) wrapError(err error, method, url string) error {
	kind := ErrKindUnknown
	if errors.Is(err, context.DeadlineExceeded) {
		kind = ErrKindTimeout
	} else if errors.Is(err, context.Canceled) {
		kind = ErrKindNetwork
	}

	return &Error{
		Kind:   kind,
		Method: method,
		URL:    url,
		Err:    err,
	}
}
