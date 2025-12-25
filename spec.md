# HTTP Client Package Specification

## Overview

A production-grade HTTP client package for Go, designed for communicating with third-party APIs over HTTPS. The package emphasizes upfront configuration with sensible defaults, enabling simple request execution after initial setup.

### Design Philosophy

**Configure once, use simply.** All behavioral configuration happens at client construction time. Individual requests require minimal ceremony.

```go
// Setup: all complexity lives here
client := httpclient.New(
    httpclient.WithBaseURL("https://api.stripe.com/v1"),
    httpclient.WithBearerToken(os.Getenv("STRIPE_KEY")),
    httpclient.WithRetry(httpclient.DefaultRetryPolicy()),
    httpclient.WithTimeout(10*time.Second),
)

// Usage: dead simple
var customer Customer
err := client.Post(ctx, "/customers", createReq, &customer)
```

### Goals

1. **Reliability**: Automatic retries with exponential backoff, respect for `Retry-After` headers, and proper error handling
2. **Safety**: Client-side rate limiting, request timeouts, and context cancellation support
3. **Observability**: Structured logging, request tracing, and meaningful error messages
4. **Testability**: First-class mocking support without requiring interface abstractions
5. **Ergonomics**: Minimal boilerplate for common cases, escape hatches for complex ones

### Non-Goals

1. Connection pooling tuning (rely on `http.DefaultTransport` defaults)
2. HTTP/2 specific features (handled transparently by Go's standard library)
3. Response caching (too API-specific; build on top if needed)
4. GraphQL or other protocol-specific abstractions

---

## Package Structure

```
httpclient/
├── client.go         # Client struct, New(), verb methods
├── request.go        # Request builder for complex cases
├── response.go       # Response type and helpers
├── error.go          # Error types and classification
├── retry.go          # RetryPolicy and backoff calculation
├── ratelimit.go      # Token bucket rate limiter
├── auth.go           # Authentication providers
├── middleware.go     # Middleware types and built-in middleware
├── options.go        # ClientOption and RequestOption definitions
├── xml.go            # XML serialization, XMLBody wrapper
├── soap.go           # SOAP envelope, fault handling, SOAPConfig
├── mock.go           # Testing utilities
└── internal/
    └── body.go       # Body serialization and replay helpers
```

---

## Core Types

### Client

The `Client` is immutable after construction. All configuration is set via functional options passed to `New()`.

```go
type Client struct {
    baseURL     *url.URL
    httpClient  *http.Client
    timeout     time.Duration
    headers     http.Header
    retryPolicy RetryPolicy
    rateLimiter *RateLimiter
    auth        AuthProvider
    middlewares []Middleware
    logger      *slog.Logger
    logLevel    LogLevel
}

func New(opts ...ClientOption) (*Client, error)
```

**Immutability Rationale**: Clients are safe to share across goroutines. Configuration cannot drift during runtime. Creating a modified client requires explicit cloning.

### ClientOption

```go
type ClientOption func(*Client) error
```

| Option | Signature | Description |
|--------|-----------|-------------|
| `WithBaseURL` | `(string) ClientOption` | Base URL for all requests. Required. |
| `WithHTTPClient` | `(*http.Client) ClientOption` | Custom HTTP client. Defaults to a client with sensible timeouts. |
| `WithTimeout` | `(time.Duration) ClientOption` | Default request timeout. Applies to entire request lifecycle including retries. |
| `WithHeader` | `(key, value string) ClientOption` | Header applied to all requests. Can be called multiple times. |
| `WithHeaders` | `(map[string]string) ClientOption` | Multiple headers at once. |
| `WithUserAgent` | `(string) ClientOption` | Sets User-Agent header. Defaults to `httpclient/VERSION`. |
| `WithRetry` | `(RetryPolicy) ClientOption` | Retry configuration. Defaults to `NoRetry()`. |
| `WithRateLimit` | `(requests int, per time.Duration) ClientOption` | Client-side rate limiting. |
| `WithAuth` | `(AuthProvider) ClientOption` | Authentication provider. |
| `WithMiddleware` | `(Middleware) ClientOption` | Add middleware to the chain. Can be called multiple times. |
| `WithLogger` | `(*slog.Logger) ClientOption` | Structured logger instance. |
| `WithLogLevel` | `(LogLevel) ClientOption` | Logging verbosity: `LogNone`, `LogError`, `LogInfo`, `LogDebug`. |
| `WithSOAP` | `(SOAPConfig) ClientOption` | Configure client for SOAP APIs with envelope handling. |
| `WithDefaultContentType` | `(string) ClientOption` | Default Content-Type for requests. Defaults to `application/json`. |

### Request Methods

Simple verb methods cover 90% of use cases:

```go
func (c *Client) Get(ctx context.Context, path string, response any, opts ...RequestOption) error
func (c *Client) Post(ctx context.Context, path string, body any, response any, opts ...RequestOption) error
func (c *Client) Put(ctx context.Context, path string, body any, response any, opts ...RequestOption) error
func (c *Client) Patch(ctx context.Context, path string, body any, response any, opts ...RequestOption) error
func (c *Client) Delete(ctx context.Context, path string, response any, opts ...RequestOption) error
```

**Parameters**:
- `ctx`: Controls cancellation and deadlines
- `path`: Relative to client's base URL (leading slash optional)
- `body`: Serialized as JSON by default. Can be `[]byte`, `io.Reader`, `url.Values`, or any JSON-serializable type.
- `response`: Pointer to unmarshal response into. Can be `*T`, `*[]byte`, `*string`, or `**http.Response` for raw access.
- `opts`: Per-request overrides

### RequestOption

```go
type RequestOption func(*requestConfig) error
```

| Option | Signature | Description |
|--------|-----------|-------------|
| `WithRequestTimeout` | `(time.Duration) RequestOption` | Override client timeout for this request. |
| `WithRequestHeader` | `(key, value string) RequestOption` | Add/override header for this request. |
| `WithQuery` | `(key, value string) RequestOption` | Add query parameter. Can be called multiple times. |
| `WithQueryParams` | `(url.Values) RequestOption` | Set multiple query parameters. |
| `WithBody` | `(any) RequestOption` | Override body (for verb methods that don't take body). |
| `WithContentType` | `(string) RequestOption` | Override Content-Type header. |
| `WithNoRetry` | `() RequestOption` | Disable retries for this request. |
| `WithIdempotencyKey` | `(string) RequestOption` | Set idempotency key header (enables POST retry). |
| `WithXML` | `() RequestOption` | Serialize request body as XML, expect XML response. |
| `WithExpectXML` | `() RequestOption` | Force XML deserialization of response. |
| `WithSOAP11` | `() RequestOption` | Use SOAP 1.1 envelope for this request (per-request SOAP). |
| `WithSOAP12` | `() RequestOption` | Use SOAP 1.2 envelope for this request (per-request SOAP). |
| `WithSOAPAction` | `(string) RequestOption` | Set SOAPAction header/parameter. |
| `WithSOAPHeader` | `(any) RequestOption` | Add element to SOAP Header. |
| `WithSOAPHeaders` | `(...any) RequestOption` | Add multiple elements to SOAP Header. |
| `WithSOAPNamespace` | `(string) RequestOption` | Set namespace for SOAP body (per-request). |
| `WithRawSOAP` | `() RequestOption` | Disable automatic envelope wrap/unwrap. |

### Request Builder

For complex requests, use the builder pattern:

```go
type RequestBuilder struct { ... }

func (c *Client) Request() *RequestBuilder

func (b *RequestBuilder) Method(method string) *RequestBuilder
func (b *RequestBuilder) Path(path string) *RequestBuilder
func (b *RequestBuilder) Query(key, value string) *RequestBuilder
func (b *RequestBuilder) Header(key, value string) *RequestBuilder
func (b *RequestBuilder) Body(body any) *RequestBuilder
func (b *RequestBuilder) Timeout(d time.Duration) *RequestBuilder
func (b *RequestBuilder) Do(ctx context.Context) (*Response, error)
func (b *RequestBuilder) DoInto(ctx context.Context, response any) error
```

**Example**:

```go
resp, err := client.Request().
    Method(http.MethodPost).
    Path("/uploads").
    Header("Content-Type", "application/octet-stream").
    Query("filename", "report.pdf").
    Body(fileReader).
    Timeout(5 * time.Minute).
    Do(ctx)
```

---

## Response Handling

### Response Type

```go
type Response struct {
    StatusCode int
    Status     string
    Headers    http.Header
    Body       []byte
}

func (r *Response) JSON(v any) error           // Unmarshal body as JSON
func (r *Response) XML(v any) error            // Unmarshal body as XML
func (r *Response) String() string             // Body as string
func (r *Response) IsSuccess() bool            // 2xx status
func (r *Response) IsClientError() bool        // 4xx status
func (r *Response) IsServerError() bool        // 5xx status
```

### Automatic Deserialization

When a response pointer is provided to verb methods, the package automatically deserializes based on Content-Type:

| Response Content-Type | Behavior |
|-----------------------|----------|
| `application/json` | JSON unmarshal into provided pointer |
| `text/*` | If pointer is `*string`, assigns body as string |
| Any | If pointer is `*[]byte`, assigns raw body |
| Any | If pointer is `**http.Response`, assigns raw response |

### Raw Response Access

For streaming or custom handling:

```go
var rawResp *http.Response
err := client.Get(ctx, "/large-file", &rawResp)
if err != nil {
    return err
}
defer rawResp.Body.Close()

// Stream the body
io.Copy(destination, rawResp.Body)
```

---

## Content Types & Serialization

### Supported Formats

The package supports multiple content types for request and response bodies.

#### Automatic Serialization (Request Bodies)

| Body Type | Content-Type | Behavior |
|-----------|--------------|----------|
| `struct`, `map`, `slice` | `application/json` | JSON marshal (default) |
| `XMLBody{}` wrapper or `WithXML()` | `application/xml` | XML marshal |
| `url.Values` | `application/x-www-form-urlencoded` | Form encoding |
| `[]byte` | As-is or specified | Raw bytes |
| `io.Reader` | As-is or specified | Streamed |
| `string` | `text/plain` | Raw string |

#### Automatic Deserialization (Response Bodies)

| Response Content-Type | Target Type | Behavior |
|-----------------------|-------------|----------|
| `application/json`, `*+json` | `*T` | JSON unmarshal |
| `application/xml`, `text/xml`, `*+xml` | `*T` | XML unmarshal |
| `text/*` | `*string` | Assign as string |
| Any | `*[]byte` | Assign raw bytes |
| Any | `**http.Response` | Assign raw response |

### XML Support

#### Basic XML Requests

```go
type CreateOrderRequest struct {
    XMLName xml.Name `xml:"Order"`
    ID      string   `xml:"OrderId"`
    Amount  int      `xml:"Amount"`
}

type CreateOrderResponse struct {
    XMLName xml.Name `xml:"OrderResponse"`
    Status  string   `xml:"Status"`
}

var resp CreateOrderResponse
err := client.Post(ctx, "/orders", req, &resp,
    httpclient.WithXML(), // Serialize request as XML, expect XML response
)
```

#### XML Options

| Option | Signature | Description |
|--------|-----------|-------------|
| `WithXML` | `() RequestOption` | Serialize request as XML, sets `Content-Type: application/xml` |
| `WithContentType` | `(string) RequestOption` | Override Content-Type (e.g., `text/xml` for older APIs) |
| `WithExpectXML` | `() RequestOption` | Force XML deserialization regardless of response Content-Type |

#### XML Body Wrapper

For mixed clients that sometimes send JSON and sometimes XML:

```go
// Explicit XML body (no option needed)
err := client.Post(ctx, "/orders", httpclient.XMLBody{Value: req}, &resp)

// Equivalent to:
err := client.Post(ctx, "/orders", req, &resp, httpclient.WithXML())
```

### SOAP Support

For SOAP APIs, the package provides specialized support for envelope wrapping, SOAPAction headers, and fault handling.

#### SOAP Client Configuration

```go
client, _ := httpclient.New(
    httpclient.WithBaseURL("https://legacy.example.com/services"),
    httpclient.WithSOAP(httpclient.SOAPConfig{
        Version:   httpclient.SOAP11, // or SOAP12
        Namespace: "http://example.com/orders",
    }),
    httpclient.WithBasicAuth(user, pass),
    httpclient.WithTimeout(30*time.Second),
)
```

#### SOAPConfig

```go
type SOAPVersion int

const (
    SOAP11 SOAPVersion = iota  // Content-Type: text/xml; charset=utf-8
    SOAP12                      // Content-Type: application/soap+xml; charset=utf-8
)

type SOAPConfig struct {
    // Required
    Version   SOAPVersion

    // Optional
    Namespace     string            // Default namespace for body elements
    EnvelopeAttrs map[string]string // Additional xmlns declarations
    HeaderFunc    func(ctx context.Context) (any, error) // Generate SOAP headers per request
}
```

#### Making SOAP Requests

```go
// Request body - will be wrapped in SOAP envelope automatically
type GetOrderRequest struct {
    XMLName xml.Name `xml:"GetOrder"`
    OrderID string   `xml:"OrderId"`
}

// Response body - extracted from SOAP envelope automatically
type GetOrderResponse struct {
    XMLName xml.Name `xml:"GetOrderResponse"`
    Order   Order    `xml:"Order"`
}

var resp GetOrderResponse
err := client.Post(ctx, "/OrderService", GetOrderRequest{OrderID: "123"}, &resp,
    httpclient.WithSOAPAction("http://example.com/GetOrder"),
)
```

#### Envelope Handling

The package automatically wraps request bodies in SOAP envelopes:

**SOAP 1.1 Request:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
               xmlns:ns="http://example.com/orders">
    <soap:Header>
        <!-- Optional: from HeaderFunc or WithSOAPHeader -->
    </soap:Header>
    <soap:Body>
        <ns:GetOrder>
            <ns:OrderId>123</ns:OrderId>
        </ns:GetOrder>
    </soap:Body>
</soap:Envelope>
```

**SOAP 1.2 Request:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:ns="http://example.com/orders">
    <soap:Header>
        <!-- Optional: from HeaderFunc or WithSOAPHeader -->
    </soap:Header>
    <soap:Body>
        <ns:GetOrder>
            <ns:OrderId>123</ns:OrderId>
        </ns:GetOrder>
    </soap:Body>
</soap:Envelope>
```

Response envelopes are automatically unwrapped, with the Body content unmarshaled into your response struct.

#### Content-Type Handling

| SOAP Version | Request Content-Type | SOAPAction |
|--------------|---------------------|------------|
| SOAP 1.1 | `text/xml; charset=utf-8` | `SOAPAction` HTTP header |
| SOAP 1.2 | `application/soap+xml; charset=utf-8; action="..."` | In Content-Type parameter |

#### SOAP Headers

For requests requiring SOAP headers (WS-Security, correlation IDs, etc.):

```go
// Per-request header
type SecurityHeader struct {
    XMLName  xml.Name `xml:"wsse:Security"`
    Username string   `xml:"wsse:UsernameToken>wsse:Username"`
    Password string   `xml:"wsse:UsernameToken>wsse:Password"`
}

err := client.Post(ctx, "/OrderService", req, &resp,
    httpclient.WithSOAPAction("http://example.com/GetOrder"),
    httpclient.WithSOAPHeader(SecurityHeader{
        Username: user,
        Password: pass,
    }),
)
```

```go
// Or configure dynamic headers at client level
client, _ := httpclient.New(
    httpclient.WithBaseURL("https://legacy.example.com/services"),
    httpclient.WithSOAP(httpclient.SOAPConfig{
        Version:   httpclient.SOAP11,
        Namespace: "http://example.com/orders",
        HeaderFunc: func(ctx context.Context) (any, error) {
            return SecurityHeader{
                Username:  user,
                Password:  pass,
                Nonce:     generateNonce(),
                Timestamp: time.Now().UTC().Format(time.RFC3339),
            }, nil
        },
    }),
)
```

#### Multiple SOAP Headers

```go
err := client.Post(ctx, "/Service", req, &resp,
    httpclient.WithSOAPAction("http://example.com/Action"),
    httpclient.WithSOAPHeaders(
        securityHeader,
        correlationHeader,
        routingHeader,
    ),
)
```

#### SOAP Faults

SOAP faults are automatically detected and converted to structured errors:

```go
// SOAP 1.1 Fault structure
type SOAPFault struct {
    Code   string `xml:"faultcode"`
    String string `xml:"faultstring"`
    Actor  string `xml:"faultactor"`
    Detail []byte `xml:"detail,innerxml"` // Raw XML for custom parsing
}

// SOAP 1.2 Fault structure
type SOAP12Fault struct {
    Code   SOAP12FaultCode   `xml:"Code"`
    Reason SOAP12FaultReason `xml:"Reason"`
    Detail []byte            `xml:"Detail,innerxml"`
}

type SOAP12FaultCode struct {
    Value   string           `xml:"Value"`
    Subcode *SOAP12FaultCode `xml:"Subcode"`
}

type SOAP12FaultReason struct {
    Text []SOAP12ReasonText `xml:"Text"`
}
```

**Error classification helpers:**

```go
func (e *Error) IsSOAPFault() bool
func (e *Error) SOAPFault() *SOAPFault       // Returns nil if not SOAP 1.1 fault
func (e *Error) SOAP12Fault() *SOAP12Fault   // Returns nil if not SOAP 1.2 fault
```

**Handling SOAP Faults:**

```go
err := client.Post(ctx, "/OrderService", req, &resp,
    httpclient.WithSOAPAction("http://example.com/GetOrder"),
)
if err != nil {
    var httpErr *httpclient.Error
    if errors.As(err, &httpErr) && httpErr.IsSOAPFault() {
        fault := httpErr.SOAPFault()
        switch fault.Code {
        case "soap:Client":
            return fmt.Errorf("invalid request: %s", fault.String)
        case "soap:Server":
            return fmt.Errorf("server error: %s", fault.String)
        default:
            return fmt.Errorf("SOAP fault [%s]: %s", fault.Code, fault.String)
        }
    }
    return err
}
```

#### Parsing Custom Fault Details

Many SOAP services include structured error details:

```go
type OrderFaultDetail struct {
    ErrorCode    string `xml:"ErrorCode"`
    ErrorMessage string `xml:"ErrorMessage"`
    OrderID      string `xml:"OrderId"`
}

if httpErr.IsSOAPFault() {
    fault := httpErr.SOAPFault()
    
    var detail OrderFaultDetail
    if err := xml.Unmarshal(fault.Detail, &detail); err == nil {
        return fmt.Errorf("order error %s: %s (order: %s)",
            detail.ErrorCode, detail.ErrorMessage, detail.OrderID)
    }
}
```

#### Raw Envelope Access

For complex scenarios requiring manual envelope construction:

```go
// Build envelope manually
envelope := &httpclient.SOAPEnvelope{
    Headers: []any{securityHeader, customHeader},
    Body:    myRequestBody,
}

var respEnvelope httpclient.SOAPEnvelope
err := client.Post(ctx, "/Service", envelope, &respEnvelope,
    httpclient.WithSOAPAction("http://example.com/Action"),
    httpclient.WithRawSOAP(), // Don't auto-wrap/unwrap
)

// Access raw response body
var myResponse MyResponseType
if err := xml.Unmarshal(respEnvelope.Body, &myResponse); err != nil {
    return err
}
```

#### SOAP Request Options Summary

| Option | Signature | Description |
|--------|-----------|-------------|
| `WithSOAPAction` | `(action string) RequestOption` | Sets SOAPAction (required for most SOAP calls) |
| `WithSOAPHeader` | `(header any) RequestOption` | Add single SOAP header element |
| `WithSOAPHeaders` | `(...any) RequestOption` | Add multiple SOAP header elements |
| `WithRawSOAP` | `() RequestOption` | Disable automatic envelope wrap/unwrap |

#### SOAP + Non-SOAP Mixed Client

If you need to call both SOAP and REST endpoints on the same host:

```go
// Create base client without SOAP config
baseClient, _ := httpclient.New(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithBasicAuth(user, pass),
)

// REST calls work normally
var user User
baseClient.Get(ctx, "/api/users/1", &user)

// SOAP calls use per-request options
var orderResp GetOrderResponse
baseClient.Post(ctx, "/soap/OrderService", GetOrderRequest{ID: "123"}, &orderResp,
    httpclient.WithSOAP11(),
    httpclient.WithSOAPAction("http://example.com/GetOrder"),
    httpclient.WithSOAPNamespace("http://example.com/orders"),
)
```

#### Common SOAP Patterns

**WS-Addressing:**
```go
type WSAddressingHeader struct {
    XMLName   xml.Name `xml:"wsa:Header"`
    Action    string   `xml:"wsa:Action"`
    MessageID string   `xml:"wsa:MessageID"`
    To        string   `xml:"wsa:To"`
    ReplyTo   string   `xml:"wsa:ReplyTo>wsa:Address"`
}

err := client.Post(ctx, "/Service", req, &resp,
    httpclient.WithSOAPHeader(WSAddressingHeader{
        Action:    "http://example.com/GetOrder",
        MessageID: uuid.New().String(),
        To:        "http://example.com/OrderService",
        ReplyTo:   "http://www.w3.org/2005/08/addressing/anonymous",
    }),
)
```

**MTOM Attachments (Future):**
```go
// Not in MVP, but interface designed for future support
err := client.Post(ctx, "/Service", req, &resp,
    httpclient.WithSOAPAction("http://example.com/UploadDocument"),
    httpclient.WithMTOM(true),
    httpclient.WithAttachment("document", documentBytes, "application/pdf"),
)
```

---

## Error Handling

### Error Type

```go
type Error struct {
    // Classification
    Kind       ErrorKind  // Timeout, Network, HTTP, Parse, Unknown

    // HTTP details (populated for HTTP errors)
    StatusCode int
    Status     string
    Body       []byte
    Headers    http.Header

    // Request context
    Method     string
    URL        string
    Attempts   int

    // Underlying error
    Err        error
}

func (e *Error) Error() string
func (e *Error) Unwrap() error

// Classification helpers
func (e *Error) IsTimeout() bool
func (e *Error) IsNetwork() bool
func (e *Error) IsRetryable() bool
func (e *Error) IsClientError() bool     // 4xx
func (e *Error) IsServerError() bool     // 5xx
func (e *Error) IsStatus(codes ...int) bool
```

### ErrorKind

```go
type ErrorKind int

const (
    ErrKindUnknown ErrorKind = iota
    ErrKindTimeout           // Context deadline or timeout
    ErrKindNetwork           // Connection refused, DNS, etc.
    ErrKindHTTP              // 4xx/5xx response
    ErrKindParse             // JSON unmarshal failed
    ErrKindRateLimit         // Client-side rate limit exceeded (if non-blocking mode)
)
```

### Error Handling Patterns

```go
// Check specific status
var user User
err := client.Get(ctx, "/users/123", &user)
if err != nil {
    var httpErr *httpclient.Error
    if errors.As(err, &httpErr) {
        switch {
        case httpErr.IsStatus(404):
            return nil, ErrUserNotFound
        case httpErr.IsStatus(403):
            return nil, ErrForbidden
        case httpErr.IsTimeout():
            return nil, fmt.Errorf("request timed out after %d attempts", httpErr.Attempts)
        default:
            return nil, fmt.Errorf("api error: %s", httpErr.Body)
        }
    }
    return nil, err
}
```

```go
// Simple retry check
if err != nil {
    var httpErr *httpclient.Error
    if errors.As(err, &httpErr) && httpErr.IsRetryable() {
        // Could retry with backoff at application level
    }
}
```

---

## Retry Behavior

### RetryPolicy

```go
type RetryPolicy struct {
    // Limits
    MaxAttempts  int           // Total attempts (1 = no retry). Default: 1
    MaxDuration  time.Duration // Max total time for all attempts. 0 = no limit.

    // Backoff
    InitialDelay time.Duration // Delay after first failure. Default: 500ms
    MaxDelay     time.Duration // Cap on backoff. Default: 30s
    Multiplier   float64       // Backoff multiplier. Default: 2.0
    Jitter       float64       // Random factor 0-1. Default: 0.25

    // Retry conditions
    RetryableStatusCodes []int                                    // Default: [408, 429, 502, 503, 504]
    IsRetryable          func(resp *http.Response, err error) bool // Custom predicate (overrides status codes)

    // Method safety
    RetryNonIdempotent bool   // Retry POST/PATCH without idempotency key. Default: false
    IdempotencyHeader  string // Header name for idempotency key. Default: "Idempotency-Key"
}
```

### Preset Policies

```go
func NoRetry() RetryPolicy
// MaxAttempts: 1

func DefaultRetryPolicy() RetryPolicy
// MaxAttempts: 3
// InitialDelay: 500ms
// MaxDelay: 30s
// Multiplier: 2.0
// Jitter: 0.25
// RetryableStatusCodes: [408, 429, 502, 503, 504]

func AggressiveRetryPolicy() RetryPolicy
// MaxAttempts: 5
// InitialDelay: 1s
// MaxDelay: 60s
// Multiplier: 2.0
// Jitter: 0.25
// RetryableStatusCodes: [408, 429, 500, 502, 503, 504]
```

### Retry Logic

1. **Method Safety**: By default, only idempotent methods (GET, HEAD, PUT, DELETE, OPTIONS) are retried. POST and PATCH require either `RetryNonIdempotent: true` or an idempotency key via `WithIdempotencyKey()`.

2. **Retry-After Respect**: If the response includes `Retry-After` or `Retry-After-Ms` headers, that value is used instead of calculated backoff (capped at `MaxDelay`).

3. **X-Should-Retry Header**: If present, the server's explicit instruction overrides status code logic.

4. **Backoff Calculation**:
   ```
   delay = min(InitialDelay * (Multiplier ^ attempt), MaxDelay)
   jitter = delay * Jitter * random()
   finalDelay = delay - jitter/2 + random()*jitter
   ```

5. **Body Replay**: Request bodies are buffered to enable retry. Streaming bodies (non-seekable `io.Reader`) are fully read into memory on first use.

---

## Rate Limiting

### Client-Side Rate Limiter

```go
WithRateLimit(requests int, per time.Duration) ClientOption
```

Implements a token bucket algorithm:
- Bucket fills at rate of `requests` per `per` duration
- Requests block (respecting context) until a token is available
- Separate limit per `Client` instance

### Automatic Backpressure

When a `429 Too Many Requests` response is received:
1. The retry mechanism handles it (if retries enabled)
2. If `Retry-After` header is present, the client respects it
3. The rate limiter can optionally auto-adjust (future enhancement)

### Example

```go
// 100 requests per second max
client := httpclient.New(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithRateLimit(100, time.Second),
)
```

---

## Authentication

### AuthProvider Interface

```go
type AuthProvider interface {
    Apply(req *http.Request) error
}
```

### Built-in Providers

#### Static API Key

```go
// As header
WithAPIKey(key string, header string) ClientOption
// Example: WithAPIKey("sk_live_xxx", "X-API-Key")

// As query parameter
WithAPIKeyQuery(key string, param string) ClientOption
// Example: WithAPIKeyQuery("abc123", "api_key") -> ?api_key=abc123
```

#### Bearer Token

```go
// Static token
WithBearerToken(token string) ClientOption
// Sets: Authorization: Bearer <token>
```

#### Refreshable Token

```go
type TokenSource func(ctx context.Context) (token string, expiry time.Time, err error)

WithTokenSource(source TokenSource) ClientOption
```

The client:
1. Calls `TokenSource` before the first request
2. Caches the token until `expiry` (minus a small buffer)
3. Automatically refreshes when expired
4. Thread-safe for concurrent requests

**Example: OAuth2 Client Credentials**

```go
tokenSource := func(ctx context.Context) (string, time.Time, error) {
    resp, err := oauth2Client.Token(ctx, clientID, clientSecret)
    if err != nil {
        return "", time.Time{}, err
    }
    return resp.AccessToken, time.Now().Add(resp.ExpiresIn), nil
}

client := httpclient.New(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithTokenSource(tokenSource),
)
```

#### Basic Auth

```go
WithBasicAuth(username, password string) ClientOption
```

#### Custom Auth

```go
WithAuthFunc(func(req *http.Request) error) ClientOption
```

For complex auth schemes (AWS Signature, custom HMAC, etc.):

```go
client := httpclient.New(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithAuthFunc(func(req *http.Request) error {
        signature := computeHMAC(req)
        req.Header.Set("X-Signature", signature)
        return nil
    }),
)
```

---

## Middleware

### Middleware Type

```go
type Middleware func(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error)
```

Middleware wraps the HTTP transport, executing in the order added (first added = outermost).

### Execution Order

```
Request → Middleware1 → Middleware2 → Middleware3 → HTTP Transport
                                                          ↓
Response ← Middleware1 ← Middleware2 ← Middleware3 ← HTTP Response
```

### Built-in Middleware

#### Request ID Propagation

```go
func RequestIDMiddleware(headerName string) Middleware
```

Reads request ID from context and adds to outgoing request headers. If no ID in context, generates a new UUID.

```go
// Reads/writes "X-Request-ID" header
WithMiddleware(httpclient.RequestIDMiddleware("X-Request-ID"))
```

#### Logging Middleware

Automatically added when `WithLogger` is set. Controlled by `WithLogLevel`.

| Level | Logs |
|-------|------|
| `LogNone` | Nothing |
| `LogError` | Failed requests only |
| `LogInfo` | All requests (method, URL, status, duration) |
| `LogDebug` | All requests + headers + body (with redaction) |

#### Custom Middleware Example

```go
// Add timing header
timingMiddleware := func(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
    start := time.Now()
    resp, err := next(req)
    if resp != nil {
        resp.Header.Set("X-Client-Duration", time.Since(start).String())
    }
    return resp, err
}

client := httpclient.New(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithMiddleware(timingMiddleware),
)
```

---

## Observability

### Logging

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

client := httpclient.New(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithLogger(logger),
    httpclient.WithLogLevel(httpclient.LogInfo),
)
```

**Log Fields**:
- `method`: HTTP method
- `url`: Request URL (query params redacted at Debug level)
- `status`: Response status code
- `duration`: Request duration
- `attempt`: Attempt number (if retrying)
- `error`: Error message (if failed)

**Sensitive Data Redaction**: At `LogDebug` level, the following headers are redacted:
- `Authorization`
- `X-API-Key`
- `Cookie`
- Any header containing `token`, `secret`, `password`, `key`

### Request ID

```go
// Add to context
ctx = httpclient.WithRequestID(ctx, "req-123")

// Or generate automatically
ctx = httpclient.WithNewRequestID(ctx)

// Retrieve from context
id := httpclient.GetRequestID(ctx)
```

When `RequestIDMiddleware` is enabled, the ID propagates to downstream services.

### Metrics (Future)

The middleware system supports custom metrics integration:

```go
metricsMiddleware := func(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
    start := time.Now()
    resp, err := next(req)
    
    labels := prometheus.Labels{
        "method": req.Method,
        "host":   req.URL.Host,
        "status": strconv.Itoa(resp.StatusCode),
    }
    httpRequestDuration.With(labels).Observe(time.Since(start).Seconds())
    httpRequestsTotal.With(labels).Inc()
    
    return resp, err
}
```

---

## Testing Support

### Mock Client

```go
func NewMock(handlers ...MockHandler) *Client
```

Creates a client that returns predefined responses without making HTTP calls.

### Mock Handlers

```go
// Match method + path exactly
func MockResponse(method, path string, statusCode int, body string) MockHandler

// Match method + path with JSON body
func MockJSON(method, path string, statusCode int, body any) MockHandler

// Return an error
func MockError(method, path string, err error) MockHandler

// Match with custom logic
func MockFunc(predicate func(*http.Request) bool, handler func(*http.Request) (*http.Response, error)) MockHandler
```

### Request Recording

```go
mock := httpclient.NewMock(
    httpclient.MockJSON(http.MethodPost, "/users", 201, map[string]any{"id": 1}),
)

// ... use mock client ...

// Assert requests were made
mock.AssertCalled(t, http.MethodPost, "/users")
mock.AssertNotCalled(t, http.MethodDelete, "/users/1")
mock.AssertCallCount(t, http.MethodPost, "/users", 1)

// Inspect recorded request
req := mock.LastRequest()
req := mock.Requests()[0]

// Assert on request details
mock.AssertHeader(t, http.MethodPost, "/users", "Content-Type", "application/json")
mock.AssertBodyJSON(t, http.MethodPost, "/users", expectedBody)
```

### Example Test

```go
func TestCreateUser(t *testing.T) {
    mock := httpclient.NewMock(
        httpclient.MockJSON(http.MethodPost, "/users", 201, User{ID: 1, Name: "Alice"}),
    )
    
    svc := &UserService{client: mock}
    
    user, err := svc.CreateUser(context.Background(), "Alice")
    
    require.NoError(t, err)
    assert.Equal(t, 1, user.ID)
    assert.Equal(t, "Alice", user.Name)
    
    mock.AssertCalled(t, http.MethodPost, "/users")
    mock.AssertBodyJSON(t, http.MethodPost, "/users", map[string]any{"name": "Alice"})
}
```

### Testing Retries

```go
func TestRetryBehavior(t *testing.T) {
    callCount := 0
    mock := httpclient.NewMock(
        httpclient.MockFunc(
            func(r *http.Request) bool { return r.URL.Path == "/flaky" },
            func(r *http.Request) (*http.Response, error) {
                callCount++
                if callCount < 3 {
                    return mockResponse(503, ""), nil
                }
                return mockResponse(200, `{"ok":true}`), nil
            },
        ),
    )
    mock.retryPolicy = httpclient.DefaultRetryPolicy()
    
    var resp struct{ OK bool }
    err := mock.Get(context.Background(), "/flaky", &resp)
    
    require.NoError(t, err)
    assert.True(t, resp.OK)
    assert.Equal(t, 3, callCount)
}
```

---

## Complete Example

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "time"
    
    "yourcompany/httpclient"
)

type StripeClient struct {
    client *httpclient.Client
}

func NewStripeClient(apiKey string) *StripeClient {
    client, _ := httpclient.New(
        httpclient.WithBaseURL("https://api.stripe.com/v1"),
        httpclient.WithBearerToken(apiKey),
        httpclient.WithTimeout(30*time.Second),
        httpclient.WithRetry(httpclient.DefaultRetryPolicy()),
        httpclient.WithRateLimit(100, time.Second),
        httpclient.WithLogger(slog.Default()),
        httpclient.WithLogLevel(httpclient.LogInfo),
        httpclient.WithHeader("Stripe-Version", "2023-10-16"),
    )
    
    return &StripeClient{client: client}
}

type Customer struct {
    ID    string `json:"id"`
    Email string `json:"email"`
    Name  string `json:"name"`
}

type CreateCustomerRequest struct {
    Email string `json:"email"`
    Name  string `json:"name"`
}

func (s *StripeClient) CreateCustomer(ctx context.Context, email, name string) (*Customer, error) {
    var customer Customer
    err := s.client.Post(ctx, "/customers", CreateCustomerRequest{
        Email: email,
        Name:  name,
    }, &customer,
        httpclient.WithIdempotencyKey(generateIdempotencyKey()),
    )
    if err != nil {
        return nil, err
    }
    return &customer, nil
}

func (s *StripeClient) GetCustomer(ctx context.Context, id string) (*Customer, error) {
    var customer Customer
    err := s.client.Get(ctx, "/customers/"+id, &customer)
    if err != nil {
        var httpErr *httpclient.Error
        if errors.As(err, &httpErr) && httpErr.IsStatus(404) {
            return nil, ErrCustomerNotFound
        }
        return nil, err
    }
    return &customer, nil
}

func (s *StripeClient) DeleteCustomer(ctx context.Context, id string) error {
    return s.client.Delete(ctx, "/customers/"+id, nil)
}
```

### SOAP Example

```go
package main

import (
    "context"
    "encoding/xml"
    "errors"
    "log/slog"
    "time"
    
    "yourcompany/httpclient"
)

// OrderServiceClient wraps a legacy SOAP order service
type OrderServiceClient struct {
    client *httpclient.Client
}

func NewOrderServiceClient(endpoint, username, password string) *OrderServiceClient {
    client, _ := httpclient.New(
        httpclient.WithBaseURL(endpoint),
        httpclient.WithSOAP(httpclient.SOAPConfig{
            Version:   httpclient.SOAP11,
            Namespace: "http://orders.example.com/",
            HeaderFunc: func(ctx context.Context) (any, error) {
                // WS-Security UsernameToken
                return WSSecurity{
                    Username: username,
                    Password: password,
                    Nonce:    generateNonce(),
                    Created:  time.Now().UTC().Format(time.RFC3339),
                }, nil
            },
        }),
        httpclient.WithTimeout(30*time.Second),
        httpclient.WithRetry(httpclient.DefaultRetryPolicy()),
        httpclient.WithLogger(slog.Default()),
    )
    
    return &OrderServiceClient{client: client}
}

// WS-Security header
type WSSecurity struct {
    XMLName  xml.Name `xml:"wsse:Security"`
    Username string   `xml:"wsse:UsernameToken>wsse:Username"`
    Password string   `xml:"wsse:UsernameToken>wsse:Password"`
    Nonce    string   `xml:"wsse:UsernameToken>wsse:Nonce"`
    Created  string   `xml:"wsse:UsernameToken>wsu:Created"`
}

// Request/Response types
type GetOrderRequest struct {
    XMLName xml.Name `xml:"GetOrder"`
    OrderID string   `xml:"OrderId"`
}

type GetOrderResponse struct {
    XMLName xml.Name `xml:"GetOrderResponse"`
    Order   Order    `xml:"Order"`
}

type Order struct {
    ID        string  `xml:"OrderId"`
    Status    string  `xml:"Status"`
    Total     float64 `xml:"TotalAmount"`
    Currency  string  `xml:"Currency"`
    CreatedAt string  `xml:"CreatedDate"`
}

type CreateOrderRequest struct {
    XMLName  xml.Name    `xml:"CreateOrder"`
    Customer string      `xml:"CustomerId"`
    Items    []OrderItem `xml:"Items>Item"`
}

type OrderItem struct {
    SKU      string  `xml:"SKU"`
    Quantity int     `xml:"Quantity"`
    Price    float64 `xml:"UnitPrice"`
}

type CreateOrderResponse struct {
    XMLName xml.Name `xml:"CreateOrderResponse"`
    OrderID string   `xml:"OrderId"`
    Status  string   `xml:"Status"`
}

func (c *OrderServiceClient) GetOrder(ctx context.Context, orderID string) (*Order, error) {
    var resp GetOrderResponse
    err := c.client.Post(ctx, "/OrderService", GetOrderRequest{
        OrderID: orderID,
    }, &resp,
        httpclient.WithSOAPAction("http://orders.example.com/GetOrder"),
    )
    if err != nil {
        var httpErr *httpclient.Error
        if errors.As(err, &httpErr) && httpErr.IsSOAPFault() {
            fault := httpErr.SOAPFault()
            if fault.Code == "OrderNotFound" {
                return nil, ErrOrderNotFound
            }
            return nil, fmt.Errorf("SOAP fault: %s", fault.String)
        }
        return nil, err
    }
    return &resp.Order, nil
}

func (c *OrderServiceClient) CreateOrder(ctx context.Context, customerID string, items []OrderItem) (string, error) {
    var resp CreateOrderResponse
    err := c.client.Post(ctx, "/OrderService", CreateOrderRequest{
        Customer: customerID,
        Items:    items,
    }, &resp,
        httpclient.WithSOAPAction("http://orders.example.com/CreateOrder"),
    )
    if err != nil {
        return "", err
    }
    return resp.OrderID, nil
}
```

---

## Default Configuration

When no options are provided, the client uses these defaults:

| Setting | Default Value |
|---------|---------------|
| Timeout | 30 seconds |
| User-Agent | `httpclient/VERSION` |
| Retry Policy | `NoRetry()` (no retries) |
| Rate Limit | None |
| Auth | None |
| Log Level | `LogNone` |
| Content-Type | `application/json` (when body provided) |
| Accept | `application/json` |
| SOAP | Disabled (use `WithSOAP()` to enable) |
| SOAP Version | SOAP 1.1 (when SOAP enabled) |

---

## Thread Safety

- `Client` is immutable and safe for concurrent use across goroutines
- `RequestBuilder` is NOT thread-safe; create one per request
- Mock clients record requests in a thread-safe manner
- Rate limiter is thread-safe
- Token refresh (via `TokenSource`) is thread-safe with automatic deduplication

---

## Error Handling Philosophy

1. **All errors are wrapped** in `*httpclient.Error` for consistent handling
2. **Original errors are preserved** via `Unwrap()` for `errors.Is/As` checks
3. **HTTP errors include response body** for debugging and error message extraction
4. **Classification methods** (`IsTimeout()`, `IsRetryable()`, etc.) enable clean branching without status code memorization

---

## Version History

| Version | Changes |
|---------|---------|
| 1.0.0 | Initial release |