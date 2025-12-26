package httpclient

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// RequestOption configures individual requests.
type RequestOption func(*requestConfig)

type requestConfig struct {
	timeout     time.Duration
	headers     http.Header
	query       url.Values
	contentType string
}

func newRequestConfig() *requestConfig {
	return &requestConfig{
		headers: make(http.Header),
		query:   make(url.Values),
	}
}

// WithRequestTimeout sets a timeout for this specific request.
func WithRequestTimeout(d time.Duration) RequestOption {
	return func(cfg *requestConfig) {
		cfg.timeout = d
	}
}

// WithRequestHeader adds a header to this specific request.
func WithRequestHeader(key, value string) RequestOption {
	return func(cfg *requestConfig) {
		cfg.headers.Set(key, value)
	}
}

// WithQuery adds a query parameter to this specific request.
func WithQuery(key, value string) RequestOption {
	return func(cfg *requestConfig) {
		cfg.query.Add(key, value)
	}
}

// WithContentType sets the Content-Type for this specific request.
func WithContentType(contentType string) RequestOption {
	return func(cfg *requestConfig) {
		cfg.contentType = contentType
	}
}

// RequestBuilder provides a fluent interface for building complex requests.
type RequestBuilder struct {
	client      *Client
	method      string
	path        string
	body        any
	headers     http.Header
	query       url.Values
	timeout     time.Duration
	contentType string
}

// Request creates a new RequestBuilder.
func (c *Client) Request() *RequestBuilder {
	return &RequestBuilder{
		client:  c,
		method:  http.MethodGet,
		headers: make(http.Header),
		query:   make(url.Values),
	}
}

// Method sets the HTTP method.
func (b *RequestBuilder) Method(method string) *RequestBuilder {
	b.method = method
	return b
}

// Path sets the request path.
func (b *RequestBuilder) Path(path string) *RequestBuilder {
	b.path = path
	return b
}

// Body sets the request body.
func (b *RequestBuilder) Body(body any) *RequestBuilder {
	b.body = body
	return b
}

// Header adds a header to the request.
func (b *RequestBuilder) Header(key, value string) *RequestBuilder {
	b.headers.Set(key, value)
	return b
}

// Query adds a query parameter.
func (b *RequestBuilder) Query(key, value string) *RequestBuilder {
	b.query.Add(key, value)
	return b
}

// Timeout sets a timeout for this request.
func (b *RequestBuilder) Timeout(d time.Duration) *RequestBuilder {
	b.timeout = d
	return b
}

// ContentType sets the Content-Type header.
func (b *RequestBuilder) ContentType(contentType string) *RequestBuilder {
	b.contentType = contentType
	return b
}

// Do executes the request and returns the response.
func (b *RequestBuilder) Do(ctx context.Context) (*Response, error) {
	opts := b.toRequestOptions()
	return b.client.doWithOptions(ctx, b.method, b.path, b.body, nil, opts)
}

// DoInto executes the request and unmarshals the response into result.
func (b *RequestBuilder) DoInto(ctx context.Context, result any) error {
	opts := b.toRequestOptions()
	_, err := b.client.doWithOptions(ctx, b.method, b.path, b.body, result, opts)
	return err
}

func (b *RequestBuilder) toRequestOptions() []RequestOption {
	var opts []RequestOption

	for key, values := range b.headers {
		for _, value := range values {
			k, v := key, value
			opts = append(opts, WithRequestHeader(k, v))
		}
	}

	for key, values := range b.query {
		for _, value := range values {
			k, v := key, value
			opts = append(opts, WithQuery(k, v))
		}
	}

	if b.timeout > 0 {
		opts = append(opts, WithRequestTimeout(b.timeout))
	}

	if b.contentType != "" {
		opts = append(opts, WithContentType(b.contentType))
	}

	return opts
}
