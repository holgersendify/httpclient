# Implementation Plan for HTTP Client

Based on `spec.md` and `claude.md`. Files are created only when needed.

## Phase 1: Core Client & Configuration
- [x] Create `client.go` with Client struct, New(), and ClientOption
- [x] Implement configuration options: WithBaseURL, WithHTTPClient, WithTimeout, WithHeader/WithHeaders, WithUserAgent, WithDefaultContentType
- [x] Add tests for client creation and options

## Phase 2: Error Handling
- [x] Create `error.go` with Error struct and ErrorKind
- [x] Implement error classification helpers (IsTimeout, IsNetwork, IsRetryable, IsClientError, IsServerError, IsStatus)
- [x] Add tests for error types

## Phase 3: Response Handling
- [x] Create `response.go` with Response struct
- [x] Implement Response methods (JSON, XML, String, status checks)
- [x] Add tests for response parsing

## Phase 4: Request Execution
- [x] Implement verb methods in client.go (Get, Post, Put, Patch, Delete)
- [x] Create `internal/body.go` for body serialization
- [x] Implement automatic response deserialization
- [x] Add integration tests with httptest.Server

## Phase 5: Request Builder
- [x] Create `request.go` with RequestBuilder for complex requests
- [x] Implement RequestOption type and per-request options
- [x] Add tests for request building

## Phase 6: Retry & Backoff
- [x] Create `retry.go` with RetryPolicy and backoff calculation
- [x] Implement retry loop with body replay
- [x] Implement Retry-After header parsing
- [x] Add tests for retry behavior

## Phase 7: Rate Limiting
- [x] Create `ratelimit.go` with token bucket RateLimiter
- [x] Integrate rate limiting into request flow
- [x] Add tests for rate limiting

## Phase 8: Middleware
- [x] Create `middleware.go` with Middleware type
- [x] Implement middleware chain execution
- [x] Implement RequestIDMiddleware
- [x] Add logging middleware with header redaction
- [x] Add tests for middleware

## Phase 9: Authentication
- [x] Create `auth.go` with AuthProvider interface
- [x] Implement built-in providers (APIKey, Bearer, Basic, AuthFunc)
- [x] Implement TokenSource for refreshable tokens
- [x] Add tests for auth providers

## Phase 10: XML & SOAP
- [x] Create `xml.go` with XMLBody wrapper
- [x] Create `soap.go` with SOAP envelope handling
- [x] Implement SOAP fault detection
- [x] Add tests for XML/SOAP

## Phase 11: Testing Utilities
- [x] Create `mock.go` with mock client and handlers
- [x] Implement request recording and assertions
- [x] Add tests for mock utilities

## Phase 12: Fuzz & Chaos Testing
- [x] Add fuzz tests for parsing functions
- [x] Add property-based tests with rapid
- [x] Implement chaos testing with failure capture
- [x] Add long-running stability tests

## Phase 13: Quality Assurance
- [ ] Run all linters (go vet, staticcheck, golangci-lint)
- [ ] Verify zero warnings
- [ ] Run tests with -race flag
- [ ] Document thread safety guarantees
