package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// FailureRecord captures information about a test failure.
type FailureRecord struct {
	Timestamp  time.Time   `json:"timestamp"`
	Iteration  int         `json:"iteration"`
	Seed       int64       `json:"seed"`
	Config     interface{} `json:"config"`
	PanicValue interface{} `json:"panic_value,omitempty"`
	Error      string      `json:"error,omitempty"`
	Stack      string      `json:"stack,omitempty"`
}

// ChaosConfig holds configuration for a chaos test iteration.
type ChaosConfig struct {
	Latency       time.Duration `json:"latency_ms"`
	FailureRate   float64       `json:"failure_rate"`
	StatusCode    int           `json:"status_code"`
	Timeout       time.Duration `json:"timeout_ms"`
	MaxRetries    int           `json:"max_retries"`
	ConcurrentReq int           `json:"concurrent_requests"`
}

func TestChaos_RandomFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))
	t.Logf("Chaos test seed: %d", seed)

	const iterations = 100
	var failures []FailureRecord

	for i := 0; i < iterations; i++ {
		config := generateChaosConfig(rng)

		func() {
			defer func() {
				if r := recover(); r != nil {
					failures = append(failures, FailureRecord{
						Timestamp:  time.Now(),
						Iteration:  i,
						Seed:       seed,
						Config:     config,
						PanicValue: fmt.Sprintf("%v", r),
						Stack:      string(debug.Stack()),
					})
				}
			}()

			if err := runChaosIteration(config); err != nil {
				// Errors are expected in chaos testing, but we log unexpected ones
				if !isExpectedError(err) {
					failures = append(failures, FailureRecord{
						Timestamp: time.Now(),
						Iteration: i,
						Seed:      seed,
						Config:    config,
						Error:     err.Error(),
					})
				}
			}
		}()
	}

	if len(failures) > 0 {
		// Write failures to file for analysis
		failuresFile := fmt.Sprintf("testdata/chaos_failures_%d.json", seed)
		if err := os.MkdirAll("testdata", 0755); err == nil {
			if f, err := os.Create(failuresFile); err == nil {
				encoder := json.NewEncoder(f)
				encoder.SetIndent("", "  ")
				_ = encoder.Encode(failures)
				f.Close()
				t.Logf("Failures written to: %s", failuresFile)
			}
		}
		t.Errorf("Chaos test had %d unexpected failures (see %s)", len(failures), failuresFile)
	}
}

func generateChaosConfig(rng *rand.Rand) ChaosConfig {
	return ChaosConfig{
		Latency:       time.Duration(rng.Intn(100)) * time.Millisecond,
		FailureRate:   rng.Float64() * 0.5, // Up to 50% failure rate
		StatusCode:    []int{200, 400, 404, 500, 502, 503}[rng.Intn(6)],
		Timeout:       time.Duration(50+rng.Intn(200)) * time.Millisecond,
		MaxRetries:    rng.Intn(4) + 1,
		ConcurrentReq: rng.Intn(5) + 1,
	}
}

func runChaosIteration(config ChaosConfig) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Random latency
		time.Sleep(config.Latency)

		// Random failure
		if rng.Float64() < config.FailureRate {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(config.StatusCode)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	var retryPolicy *RetryPolicy
	if config.MaxRetries > 1 {
		retryPolicy = &RetryPolicy{
			MaxAttempts:  config.MaxRetries,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     50 * time.Millisecond,
			Multiplier:   2.0,
			Jitter:       0.1,
		}
	}

	opts := []ClientOption{
		WithBaseURL(server.URL),
		WithTimeout(config.Timeout),
	}
	if retryPolicy != nil {
		opts = append(opts, WithRetry(retryPolicy))
	}

	client, err := New(opts...)
	if err != nil {
		return err
	}

	ctx := context.Background()
	_, err = client.Get(ctx, "/test", nil)
	return err
}

func isExpectedError(err error) bool {
	if err == nil {
		return true
	}

	// These errors are expected in chaos testing
	httpErr, ok := err.(*Error)
	if ok {
		// HTTP errors are expected
		if httpErr.Kind == ErrKindHTTP {
			return true
		}
		// Timeouts are expected
		if httpErr.Kind == ErrKindTimeout {
			return true
		}
		// Network errors are expected
		if httpErr.Kind == ErrKindNetwork {
			return true
		}
	}

	return false
}

func TestChaos_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		// Random latency
		time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client, err := New(
		WithBaseURL(server.URL),
		WithTimeout(5*time.Second),
	)
	assert.NoError(t, err)

	const numGoroutines = 50
	const requestsPerGoroutine = 20

	var wg sync.WaitGroup
	var errors int32
	var panics int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt32(&panics, 1)
				}
			}()

			for j := 0; j < requestsPerGoroutine; j++ {
				ctx := context.Background()
				_, err := client.Get(ctx, "/test", nil)
				if err != nil {
					atomic.AddInt32(&errors, 1)
				}
			}
		}()
	}

	wg.Wait()

	t.Logf("Total requests: %d, Errors: %d, Panics: %d",
		atomic.LoadInt32(&requestCount),
		atomic.LoadInt32(&errors),
		atomic.LoadInt32(&panics))

	// No panics should occur
	assert.Equal(t, int32(0), atomic.LoadInt32(&panics), "client should never panic")
}

func TestChaos_RateLimiterUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(
		WithBaseURL(server.URL),
		WithRateLimit(10, time.Second), // 10 requests per second
	)
	assert.NoError(t, err)

	const numGoroutines = 20
	var wg sync.WaitGroup
	var panics int32

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt32(&panics, 1)
				}
			}()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					_, _ = client.Get(ctx, "/test", nil)
				}
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int32(0), atomic.LoadInt32(&panics), "rate limiter should never panic under load")
}

func TestChaos_MiddlewareChainUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create multiple middleware layers
	var mu sync.Mutex
	var logs []string
	logger := func(msg string) {
		mu.Lock()
		logs = append(logs, msg)
		mu.Unlock()
	}

	client, err := New(
		WithBaseURL(server.URL),
		WithMiddleware(RequestIDMiddleware("X-Request-ID")),
		WithMiddleware(LoggingMiddleware(logger)),
	)
	assert.NoError(t, err)

	const numGoroutines = 30
	const requestsPerGoroutine = 10

	var wg sync.WaitGroup
	var panics int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt32(&panics, 1)
				}
			}()

			for j := 0; j < requestsPerGoroutine; j++ {
				ctx := context.Background()
				_, _ = client.Get(ctx, "/test", nil)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int32(0), atomic.LoadInt32(&panics), "middleware chain should never panic")

	// Verify logs were recorded (thread-safe)
	mu.Lock()
	assert.NotEmpty(t, logs, "logs should be recorded")
	mu.Unlock()
}

func TestChaos_RetryUnderFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	var requestCount int32
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		// Fail 70% of requests
		if rng.Float64() < 0.7 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"request":%d}`, count)))
	}))
	defer server.Close()

	client, err := New(
		WithBaseURL(server.URL),
		WithRetry(&RetryPolicy{
			MaxAttempts:  5,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
			Jitter:       0.1,
		}),
	)
	assert.NoError(t, err)

	const numGoroutines = 10
	var wg sync.WaitGroup
	var panics int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt32(&panics, 1)
				}
			}()

			for j := 0; j < 5; j++ {
				ctx := context.Background()
				_, _ = client.Get(ctx, "/test", nil)
			}
		}()
	}

	wg.Wait()

	t.Logf("Total requests made: %d", atomic.LoadInt32(&requestCount))
	assert.Equal(t, int32(0), atomic.LoadInt32(&panics), "retry logic should never panic")
}
