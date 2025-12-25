# Claude Code Guidelines for httpclient

This Go HTTP client project follows three core principles. All code contributions must adhere to these rules.

## Core Principles

1. **Negative Space Programming** - Define what must NOT happen; valid behavior emerges from the remaining space
2. **Power of Ten Rules** - NASA/JPL safety-critical coding adapted for Go
3. **Idiomatic Go** - Clear, simple, explicit code following Go conventions

---

## Negative Space Programming

Explicitly handle all invalid states first. The "happy path" is what remains after eliminating everything that shouldn't happen.

### Guard Clauses First

```go
// CORRECT: Eliminate invalid cases immediately
func ProcessRequest(req *Request) (*Response, error) {
    if req == nil {
        return nil, errors.New("request cannot be nil")
    }
    if req.URL == "" {
        return nil, errors.New("URL cannot be empty")
    }
    if req.Timeout <= 0 {
        return nil, errors.New("timeout must be positive")
    }

    // Happy path emerges naturally
    return doRequest(req)
}
```

### Rules

- Validate all inputs at system boundaries
- Reject invalid states immediately with clear error messages
- Never allow invalid data to propagate deeper into the system
- Return early on any error condition
- Error messages must be actionable: `fmt.Errorf("timeout %v exceeds maximum allowed %v", timeout, maxTimeout)`

---

## Power of Ten Rules

Based on NASA/JPL's safety-critical coding guidelines.

### 1. Simple Control Flow

- **No goto statements**
- **No recursion** - Use iteration with explicit bounds
- Prefer early returns over deep nesting
- Maximum nesting depth: 3 levels

### 2. All Loops Must Have Fixed Upper Bounds

- Every loop must have a provable maximum iteration count
- Use explicit iteration limits for potentially unbounded operations
- Timeouts are mandatory for any network or I/O operation

```go
const (
    MaxRetries   = 5
    MaxRedirects = 10
    MaxBodySize  = 10 * 1024 * 1024 // 10MB
)
```

### 3. No Dynamic Memory Allocation After Initialization

- Pre-allocate buffers and pools during initialization
- Use `sync.Pool` for frequently allocated objects
- Specify capacity hints for slices and maps

### 4. Functions Must Be Short

- Maximum 60 lines of code per function
- Each function should do one thing well

### 5. Assert Invariants

- Validate preconditions at function entry
- Validate postconditions before return
- Distinguish between external errors and internal bugs

**External input (network, user, API)** - Always return an error:

```go
func (c *Client) Do(ctx context.Context, req *Request) (*Response, error) {
    if req == nil {
        return nil, errors.New("request cannot be nil")
    }
    // ...
}
```

**Internal invariants (programmer bugs)** - Panic is acceptable:

```go
// For production code, use a simple assert helper
func assert(condition bool, msg string) {
    if !condition {
        panic("assertion failed: " + msg)
    }
}

func (c *Client) mustGetTransport() http.RoundTripper {
    assert(c.transport != nil, "transport is nil - client not initialized")
    return c.transport
}
```

| Condition | Response |
|-----------|----------|
| Bad external input | Return error |
| Internal invariant violation | Panic |

### 6. Smallest Possible Scope

- Declare variables as close to usage as possible
- Avoid package-level variables except for true constants
- Place `defer` statements immediately after resource allocation

### 7. Check All Return Values

- **Every error must be checked**
- **Never use `_` to discard errors**

```go
resp, err := client.Do(req)
if err != nil {
    return nil, fmt.Errorf("request failed: %w", err)
}
defer func() {
    if closeErr := resp.Body.Close(); closeErr != nil {
        log.Printf("failed to close response body: %v", closeErr)
    }
}()
```

### 8. Minimize Indirection

- Maximum one level of pointer indirection
- Avoid pointer-to-pointer
- Avoid pointers to interfaces - interfaces are already reference types
- Prefer value receivers unless mutation is required

### 9. Use Static Analysis

- Run `go vet`, `staticcheck`, `golangci-lint`
- Zero warnings policy

---

## Idiomatic Go

### Accept Interfaces, Return Structs

```go
func NewClient(transport http.RoundTripper) *Client {
    return &Client{transport: transport}
}
```

### Small Interfaces

Prefer single-method interfaces over large ones.

### Error Wrapping with Context

```go
if err != nil {
    return fmt.Errorf("failed to parse response from %s: %w", url, err)
}
```

### Context for Cancellation and Timeouts

- All blocking operations must accept `context.Context`
- Context should be the first parameter
- Never store context in a struct
- Never use `context.Background()` or `context.TODO()` in production code

### Naming

- Package names: short, lowercase, no underscores
- Variable names: short for small scope, descriptive for larger scope
- Avoid stuttering: `http.Client` not `http.HTTPClient`

---

## Testing Requirements

### Required Packages

- `github.com/stretchr/testify/assert` - Assertions that report failure but continue
- `github.com/stretchr/testify/require` - Assertions that stop test immediately on failure
- `pgregory.net/rapid` - Property-based testing

### Assertions in Tests

Use `testify/assert` for non-critical checks and `testify/require` for preconditions:

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestClient_Do(t *testing.T) {
    client := NewClient()
    require.NotNil(t, client, "client must be created")  // stops if nil

    resp, err := client.Do(ctx, req)
    require.NoError(t, err)                              // stops if error

    assert.Equal(t, 200, resp.StatusCode)                // continues if fails
    assert.NotEmpty(t, resp.Body)                        // continues if fails
}
```

### Table-Driven Tests

All tests should use table-driven format with `t.Run()` subtests.

### Fuzz Testing

- All public functions accepting external input must have fuzz tests
- Use Go's native fuzzing (`testing.F`) for crash detection
- Use `pgregory.net/rapid` for property-based testing
- Run fuzz tests with `-race` flag

### Reproducible Failures

All fuzz tests must capture the exact input that caused a failure for replay.

**Go native fuzzing** - Crashes are automatically saved to `testdata/fuzz/<TestName>/`:

```go
func FuzzParseURL(f *testing.F) {
    f.Add("http://example.com")
    f.Add("")
    f.Add("://invalid")

    f.Fuzz(func(t *testing.T, input string) {
        // Crashes saved automatically to testdata/fuzz/FuzzParseURL/
        _, _ = ParseURL(input)
    })
}
```

**rapid** - Use seeds for deterministic replay:

```go
func TestClient_PropertyBased(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        // Generate random config
        config := &ClientConfig{
            Timeout:      rapid.Int64Range(0, 30000).Draw(t, "timeout"),
            MaxRetries:   rapid.IntRange(-1, 10).Draw(t, "maxRetries"),
            MaxRedirects: rapid.IntRange(-1, 20).Draw(t, "maxRedirects"),
        }

        // On failure, rapid prints the seed and drawn values
        // Re-run with: go test -run TestClient_PropertyBased -rapid.seed=<seed>
        client, err := NewClient(config)
        if err != nil {
            return // expected for invalid config
        }

        assert.NotNil(t, client)
    })
}
```

**Persistent failure capture** - Recover from panics, continue simulation, save all failures to file:

```go
type FailureRecord struct {
    Timestamp  time.Time   `json:"timestamp"`
    Iteration  int         `json:"iteration"`
    Seed       int64       `json:"seed"`
    Config     interface{} `json:"config"`
    PanicValue interface{} `json:"panic_value,omitempty"`
    Error      string      `json:"error,omitempty"`
    Stack      string      `json:"stack,omitempty"`
}

func TestChaosWithRecovery(t *testing.T) {
    seed := time.Now().UnixNano()
    rng := rand.New(rand.NewSource(seed))

    failuresFile, err := os.Create(fmt.Sprintf("testdata/failures_%d.jsonl", seed))
    require.NoError(t, err)
    defer failuresFile.Close()

    encoder := json.NewEncoder(failuresFile)
    var failureCount int

    for i := 0; i < 10000; i++ {
        config := generateRandomConfig(rng)

        func() {
            defer func() {
                if r := recover(); r != nil {
                    record := FailureRecord{
                        Timestamp:  time.Now(),
                        Iteration:  i,
                        Seed:       seed,
                        Config:     config,
                        PanicValue: fmt.Sprintf("%v", r),
                        Stack:      string(debug.Stack()),
                    }
                    encoder.Encode(record)
                    failureCount++
                }
            }()

            if err := runTest(config); err != nil {
                record := FailureRecord{
                    Timestamp: time.Now(),
                    Iteration: i,
                    Seed:      seed,
                    Config:    config,
                    Error:     err.Error(),
                }
                encoder.Encode(record)
                failureCount++
            }
        }()
    }

    t.Logf("Completed with %d failures. See: testdata/failures_%d.jsonl", failureCount, seed)
    if failureCount > 0 {
        t.Fail()
    }
}
```

**Replay a specific failure**:

```go
func TestReplayFailure(t *testing.T) {
    // Load a specific failure from the JSONL file
    data := `{"config":{"timeout":0,"maxRetries":-1}}`
    var record FailureRecord
    require.NoError(t, json.Unmarshal([]byte(data), &record))

    // This will panic/fail - useful for debugging
    runTest(record.Config)
}
```

### Long-Running Simulation

- Stability tests must run for minimum 10 minutes
- Use bounded concurrency (max 50 goroutines)
- All simulations must have explicit upper bounds on iterations
- Use `httptest.Server` for deterministic, isolated testing

### Chaos Testing

- Inject random latency (10ms-500ms)
- Inject random failures (connection reset, refused, timeout)
- Client must never panic from external conditions (bad input, network errors)
- Client must always return either a valid response or an error

### Invariants to Verify

1. No panics from external input (internal assertion panics are acceptable for bugs)
2. Invalid external input must return an error
3. All resources must be released
4. Memory usage must not grow unboundedly
5. Operations must complete within timeout

---

## Quick Reference

| Rule | Constraint |
|------|------------|
| Max function length | 60 lines |
| Max nesting depth | 3 levels |
| Max retries | 5 |
| Max redirects | 10 |
| Max body size | 10MB |
| Recursion | Forbidden |
| Discarding errors | Forbidden |
| goto | Forbidden |

---

## Git Conventions

- Do not include Co-Authored-By or author attribution in commits
- Commit small diffs often - prefer incremental commits over large changes
- Never commit without explicit user approval
- Use conventional commit format: `type(scope): description`

### Commit Types

| Type | Usage |
|------|-------|
| `feat` | New feature |
| `fix` | Bug fix |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `test` | Adding or updating tests |
| `docs` | Documentation only |
| `chore` | Maintenance tasks |

### Examples

```
feat(client): add retry policy support
fix(auth): handle token refresh race condition
refactor(middleware): simplify chain execution
test(retry): add property-based tests for backoff
```
