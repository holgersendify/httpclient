# Implementation Plan for HTTP Client

Based on `spec.md` and `claude.md`, the implementation is divided into logical phases to build the client incrementally.

## Phase 1: Core Architecture & Configuration
- [ ] Initialize project structure (create files/packages as per `spec.md` Package Structure):
    - [ ] `client.go` - Client struct, New(), verb methods
    - [ ] `request.go` - Request builder for complex cases
    - [ ] `response.go` - Response type and helpers
    - [ ] `error.go` - Error types and classification
    - [ ] `retry.go` - RetryPolicy and backoff calculation
    - [ ] `ratelimit.go` - Token bucket rate limiter
    - [ ] `auth.go` - Authentication providers
    - [ ] `middleware.go` - Middleware types and built-in middleware
    - [ ] `options.go` - ClientOption and RequestOption definitions
    - [ ] `xml.go` - XML serialization, XMLBody wrapper
    - [ ] `soap.go` - SOAP envelope, fault handling, SOAPConfig
    - [ ] `mock.go` - Testing utilities
    - [ ] `internal/body.go` - Body serialization and replay helpers
- [ ] Define `Client` struct in `client.go` with all required fields (immutable design).
- [ ] Implement `ClientOption` type and pattern in `options.go`.
- [ ] Implement `New()` constructor in `client.go` with default values.
- [ ] Implement basic Configuration Options:
    - [ ] `WithBaseURL` (required)
    - [ ] `WithHTTPClient`
    - [ ] `WithTimeout` (default: 30s)
    - [ ] `WithHeader` / `WithHeaders`
    - [ ] `WithUserAgent` (default: `httpclient/VERSION`)
    - [ ] `WithDefaultContentType` (default: `application/json`)

## Phase 2: Error Handling & Core Types
- [ ] Define `ErrorKind` enum in `error.go`:
    - [ ] `ErrKindUnknown`
    - [ ] `ErrKindTimeout`
    - [ ] `ErrKindNetwork`
    - [ ] `ErrKindHTTP`
    - [ ] `ErrKindParse`
    - [ ] `ErrKindRateLimit`
- [ ] Define `Error` struct with fields: Kind, StatusCode, Status, Body, Headers, Method, URL, Attempts, Err.
- [ ] Implement `Error()` and `Unwrap()` methods.
- [ ] Implement error classification helpers:
    - [ ] `IsTimeout()`
    - [ ] `IsNetwork()`
    - [ ] `IsRetryable()`
    - [ ] `IsClientError()` (4xx)
    - [ ] `IsServerError()` (5xx)
    - [ ] `IsStatus(codes ...int)`
- [ ] Implement `Response` struct in `response.go`:
    - [ ] Fields: StatusCode, Status, Headers, Body
    - [ ] `JSON(v any) error`
    - [ ] `XML(v any) error`
    - [ ] `String() string`
    - [ ] `IsSuccess()` (2xx)
    - [ ] `IsClientError()` (4xx)
    - [ ] `IsServerError()` (5xx)

## Phase 3: Request Execution (Basic)
- [ ] Implement internal body handling in `internal/body.go`:
    - [ ] JSON serialization (struct, map, slice)
    - [ ] `url.Values` form encoding
    - [ ] `[]byte` raw bytes
    - [ ] `io.Reader` streaming
    - [ ] `string` raw string
    - [ ] Body replay buffer for retries
- [ ] Implement `RequestBuilder` in `request.go`:
    - [ ] `Method(method string)`
    - [ ] `Path(path string)`
    - [ ] `Query(key, value string)`
    - [ ] `Header(key, value string)`
    - [ ] `Body(body any)`
    - [ ] `Timeout(d time.Duration)`
    - [ ] `Do(ctx context.Context) (*Response, error)`
    - [ ] `DoInto(ctx context.Context, response any) error`
- [ ] Implement verb methods in `client.go`:
    - [ ] `Get(ctx, path, response, opts...)`
    - [ ] `Post(ctx, path, body, response, opts...)`
    - [ ] `Put(ctx, path, body, response, opts...)`
    - [ ] `Patch(ctx, path, body, response, opts...)`
    - [ ] `Delete(ctx, path, response, opts...)`
- [ ] Implement `RequestOption` type and options:
    - [ ] `WithRequestTimeout`
    - [ ] `WithRequestHeader`
    - [ ] `WithQuery`
    - [ ] `WithQueryParams` (url.Values)
    - [ ] `WithBody`
    - [ ] `WithContentType`
    - [ ] `WithNoRetry`
    - [ ] `WithIdempotencyKey`
- [ ] Implement automatic response deserialization:
    - [ ] `application/json` → JSON unmarshal
    - [ ] `application/xml`, `text/xml` → XML unmarshal
    - [ ] `text/*` → string assignment
    - [ ] `*[]byte` → raw bytes
    - [ ] `**http.Response` → raw response access

## Phase 4: Resiliency (Retry & Rate Limit)
- [ ] Implement `RetryPolicy` struct in `retry.go`:
    - [ ] MaxAttempts, MaxDuration
    - [ ] InitialDelay, MaxDelay, Multiplier, Jitter
    - [ ] RetryableStatusCodes (default: [408, 429, 502, 503, 504])
    - [ ] IsRetryable custom predicate
    - [ ] RetryNonIdempotent flag
    - [ ] IdempotencyHeader (default: "Idempotency-Key")
- [ ] Implement preset policies:
    - [ ] `NoRetry()` - MaxAttempts: 1
    - [ ] `DefaultRetryPolicy()` - MaxAttempts: 3, InitialDelay: 500ms
    - [ ] `AggressiveRetryPolicy()` - MaxAttempts: 5, InitialDelay: 1s
- [ ] Implement backoff calculation (exponential + jitter).
- [ ] Implement `Retry-After` and `Retry-After-Ms` header parsing.
- [ ] Implement `X-Should-Retry` header handling.
- [ ] Implement retry loop logic with body replay.
- [ ] Implement `RateLimiter` (token bucket) in `ratelimit.go`.
- [ ] Implement `WithRetry` client option.
- [ ] Implement `WithRateLimit` client option.

## Phase 5: Middleware & Observability
- [ ] Define `Middleware` type in `middleware.go`:
    ```go
    type Middleware func(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error)
    ```
- [ ] Implement `WithMiddleware` option and chain execution logic.
- [ ] Implement `RequestIDMiddleware(headerName string)`.
- [ ] Implement context helpers:
    - [ ] `WithRequestID(ctx, id)`
    - [ ] `WithNewRequestID(ctx)`
    - [ ] `GetRequestID(ctx)`
- [ ] Implement Logging:
    - [ ] `WithLogger(*slog.Logger)`
    - [ ] `WithLogLevel(LogLevel)` - LogNone, LogError, LogInfo, LogDebug
    - [ ] Log fields: method, url, status, duration, attempt, error
    - [ ] Sensitive header redaction (Authorization, X-API-Key, Cookie, *token*, *secret*, *password*, *key*)

## Phase 6: Authentication
- [ ] Define `AuthProvider` interface in `auth.go`:
    ```go
    type AuthProvider interface {
        Apply(req *http.Request) error
    }
    ```
- [ ] Implement `WithAuth(AuthProvider)` option.
- [ ] Implement built-in providers:
    - [ ] `WithAPIKey(key, header string)` - header-based
    - [ ] `WithAPIKeyQuery(key, param string)` - query param-based
    - [ ] `WithBearerToken(token string)`
    - [ ] `WithBasicAuth(username, password string)`
    - [ ] `WithAuthFunc(func(req *http.Request) error)`
- [ ] Implement `TokenSource` for refreshable tokens:
    ```go
    type TokenSource func(ctx context.Context) (token string, expiry time.Time, err error)
    ```
- [ ] Implement `WithTokenSource(TokenSource)` with:
    - [ ] Token caching until expiry
    - [ ] Automatic refresh
    - [ ] Thread-safe concurrent access

## Phase 7: XML & SOAP Support
- [ ] Implement XML support in `xml.go`:
    - [ ] `XMLBody{Value any}` wrapper struct
    - [ ] `WithXML()` request option
    - [ ] `WithExpectXML()` request option
- [ ] Define SOAP types in `soap.go`:
    - [ ] `SOAPVersion` (SOAP11, SOAP12)
    - [ ] `SOAPConfig` struct (Version, Namespace, EnvelopeAttrs, HeaderFunc)
    - [ ] `SOAPEnvelope` struct
    - [ ] `SOAPFault` struct (SOAP 1.1)
    - [ ] `SOAP12Fault` struct (SOAP 1.2)
- [ ] Implement `WithSOAP(SOAPConfig)` client option.
- [ ] Implement SOAP request options:
    - [ ] `WithSOAPAction(action string)`
    - [ ] `WithSOAPHeader(header any)`
    - [ ] `WithSOAPHeaders(...any)`
    - [ ] `WithSOAPNamespace(ns string)`
    - [ ] `WithSOAP11()` - per-request SOAP 1.1
    - [ ] `WithSOAP12()` - per-request SOAP 1.2
    - [ ] `WithRawSOAP()` - disable auto wrap/unwrap
- [ ] Implement SOAP envelope wrapping (request).
- [ ] Implement SOAP envelope unwrapping (response).
- [ ] Implement SOAP fault detection and mapping to `Error`:
    - [ ] `IsSOAPFault() bool`
    - [ ] `SOAPFault() *SOAPFault`
    - [ ] `SOAP12Fault() *SOAP12Fault`
- [ ] Implement Content-Type handling:
    - [ ] SOAP 1.1: `text/xml; charset=utf-8` + `SOAPAction` header
    - [ ] SOAP 1.2: `application/soap+xml; charset=utf-8; action="..."`

## Phase 8: Testing Support
- [ ] Implement mock client in `mock.go`:
    - [ ] `NewMock(handlers ...MockHandler) *Client`
- [ ] Implement mock handlers:
    - [ ] `MockResponse(method, path string, statusCode int, body string)`
    - [ ] `MockJSON(method, path string, statusCode int, body any)`
    - [ ] `MockError(method, path string, err error)`
    - [ ] `MockFunc(predicate, handler)`
- [ ] Implement request recording and assertions:
    - [ ] `AssertCalled(t, method, path)`
    - [ ] `AssertNotCalled(t, method, path)`
    - [ ] `AssertCallCount(t, method, path, count)`
    - [ ] `AssertHeader(t, method, path, key, value)`
    - [ ] `AssertBodyJSON(t, method, path, expected)`
    - [ ] `LastRequest() *http.Request`
    - [ ] `Requests() []*http.Request`

## Phase 9: Fuzz Testing & Chaos Testing
- [ ] Implement fuzz tests using `testing.F`:
    - [ ] URL parsing functions
    - [ ] Header parsing
    - [ ] Response deserialization
    - [ ] SOAP envelope parsing
- [ ] Implement property-based tests using `pgregory.net/rapid`:
    - [ ] Client configuration invariants
    - [ ] Request/response round-trip properties
    - [ ] Retry policy behavior
- [ ] Implement chaos testing with failure capture:
    - [ ] `FailureRecord` struct (Timestamp, Iteration, Seed, Config, PanicValue, Error, Stack)
    - [ ] Panic recovery with `defer/recover`
    - [ ] JSONL file output for failures (`testdata/failures_<seed>.jsonl`)
    - [ ] Failure replay helper
- [ ] Implement chaos injection:
    - [ ] Random latency injection (10ms-500ms)
    - [ ] Random failure injection (connection reset, refused, timeout)
    - [ ] Configurable failure rate (default 10%)
- [ ] Implement long-running stability tests:
    - [ ] Minimum 10 minute duration
    - [ ] Bounded concurrency (max 50 goroutines)
    - [ ] Success/error rate tracking

## Phase 10: Quality Assurance
- [ ] Run `go vet` on all code.
- [ ] Run `staticcheck` on all code.
- [ ] Run `golangci-lint` with strict configuration.
- [ ] Verify zero warnings policy.
- [ ] Run all tests with `-race` flag.
- [ ] Verify all fuzz test corpus files are committed.
- [ ] Document thread safety guarantees.

