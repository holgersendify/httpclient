package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestClientConfig_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random timeout (0 to 60 seconds)
		timeoutMs := rapid.Int64Range(0, 60000).Draw(t, "timeoutMs")
		timeout := time.Duration(timeoutMs) * time.Millisecond

		// Generate random base URL components
		host := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "host")
		baseURL := "http://" + host + ".example.com"

		// Try to create client - may fail for invalid configs
		opts := []ClientOption{WithBaseURL(baseURL)}
		if timeout > 0 {
			opts = append(opts, WithTimeout(timeout))
		}

		client, err := New(opts...)

		// If creation succeeded, client should be usable
		if err == nil {
			assert.NotNil(t, client)
		}
	})
}

func TestRetryPolicy_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random retry policy parameters
		maxAttempts := rapid.IntRange(1, 10).Draw(t, "maxAttempts")
		initialDelayMs := rapid.Int64Range(1, 5000).Draw(t, "initialDelayMs")
		maxDelayMs := rapid.Int64Range(1, 30000).Draw(t, "maxDelayMs")
		multiplier := rapid.Float64Range(1.0, 5.0).Draw(t, "multiplier")
		jitter := rapid.Float64Range(0.0, 1.0).Draw(t, "jitter")

		policy := &RetryPolicy{
			MaxAttempts:  maxAttempts,
			InitialDelay: time.Duration(initialDelayMs) * time.Millisecond,
			MaxDelay:     time.Duration(maxDelayMs) * time.Millisecond,
			Multiplier:   multiplier,
			Jitter:       jitter,
		}

		// Test backoff calculation for each attempt
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			backoff := policy.Backoff(attempt)

			// Backoff should always be positive
			assert.GreaterOrEqual(t, backoff, time.Duration(0), "backoff should be non-negative")

			// Backoff should not exceed max delay (with some tolerance for jitter)
			maxWithJitter := time.Duration(float64(policy.MaxDelay) * (1 + policy.Jitter))
			assert.LessOrEqual(t, backoff, maxWithJitter, "backoff should not exceed max delay with jitter")
		}
	})
}

func TestRateLimiter_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random rate limiter parameters
		requests := rapid.IntRange(1, 100).Draw(t, "requests")
		durationMs := rapid.Int64Range(100, 10000).Draw(t, "durationMs")
		duration := time.Duration(durationMs) * time.Millisecond

		limiter := NewRateLimiter(requests, duration)

		// Should be able to make 'requests' number of requests immediately
		ctx := context.Background()
		for i := 0; i < requests; i++ {
			err := limiter.Wait(ctx)
			assert.NoError(t, err, "should allow %d requests", requests)
		}
	})
}

func TestError_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random error properties
		statusCode := rapid.IntRange(100, 599).Draw(t, "statusCode")
		method := rapid.SampledFrom([]string{"GET", "POST", "PUT", "DELETE", "PATCH"}).Draw(t, "method")
		url := rapid.StringMatching(`https?://[a-z]+\.[a-z]+/[a-z]*`).Draw(t, "url")

		err := &Error{
			Kind:       ErrKindHTTP,
			StatusCode: statusCode,
			Method:     method,
			URL:        url,
		}

		// Error methods should be consistent
		if statusCode >= 400 && statusCode < 500 {
			assert.True(t, err.IsClientError(), "4xx should be client error")
			assert.False(t, err.IsServerError(), "4xx should not be server error")
		}

		if statusCode >= 500 {
			assert.True(t, err.IsServerError(), "5xx should be server error")
			assert.False(t, err.IsClientError(), "5xx should not be client error")
		}

		// IsStatus should work correctly
		assert.True(t, err.IsStatus(statusCode), "IsStatus should match")

		// Error string should not be empty
		assert.NotEmpty(t, err.Error(), "error message should not be empty")
	})
}

func TestMockTransport_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate unique random paths using a map to avoid duplicates
		numPaths := rapid.IntRange(1, 10).Draw(t, "numPaths")
		pathMap := make(map[string]int) // path -> status code
		for i := 0; i < numPaths; i++ {
			path := "/" + rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "path")
			status := rapid.SampledFrom([]int{200, 201, 400, 404, 500}).Draw(t, "status")
			pathMap[path] = status // Last one wins for duplicates
		}

		mock := NewMockTransport()
		for path, status := range pathMap {
			mock.AddResponse(path, status, nil)
		}

		client, err := New(
			WithBaseURL("http://test.example.com"),
			WithHTTPClient(&http.Client{Transport: mock}),
		)
		assert.NoError(t, err)

		// Make requests and verify responses
		for path, expectedStatus := range pathMap {
			resp, _ := client.Get(context.Background(), path, nil)
			if resp != nil {
				assert.Equal(t, expectedStatus, resp.StatusCode)
			}
		}

		// Verify call counts
		for path := range pathMap {
			assert.True(t, mock.WasCalled(path))
		}
	})
}

func TestXMLBody_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random struct data
		name := rapid.StringMatching(`[A-Za-z ]{1,50}`).Draw(t, "name")
		age := rapid.IntRange(0, 150).Draw(t, "age")

		type Person struct {
			Name string `xml:"name"`
			Age  int    `xml:"age"`
		}

		person := Person{Name: name, Age: age}
		body := XMLBody(person)

		// Should be recognized as XML body
		assert.True(t, IsXMLBody(body))

		// Should encode without error
		reader, contentType, err := EncodeXMLBody(body)
		assert.NoError(t, err)
		assert.NotNil(t, reader)
		assert.Equal(t, "application/xml", contentType)
	})
}

func TestSOAPBody_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random request data
		value := rapid.StringMatching(`[A-Za-z0-9 ]{1,100}`).Draw(t, "value")
		useSoap12 := rapid.Bool().Draw(t, "useSoap12")

		type Request struct {
			Value string `xml:"Value"`
		}

		req := Request{Value: value}
		var body any
		if useSoap12 {
			body = SOAP12Body(req)
		} else {
			body = SOAPBody(req)
		}

		// Should be recognized as SOAP body
		assert.True(t, IsSOAPBody(body))

		// Should encode without error
		reader, contentType, headers, err := EncodeSOAPBody(body)
		assert.NoError(t, err)
		assert.NotNil(t, reader)

		if useSoap12 {
			assert.Equal(t, "application/soap+xml; charset=utf-8", contentType)
		} else {
			assert.Equal(t, "text/xml; charset=utf-8", contentType)
		}
		assert.NotNil(t, headers)
	})
}

func TestClientIntegration_Property(t *testing.T) {
	// Start a test server that echoes back request info
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	rapid.Check(t, func(t *rapid.T) {
		// Generate random request parameters
		path := "/" + rapid.StringMatching(`[a-z]{0,20}`).Draw(t, "path")
		method := rapid.SampledFrom([]string{"GET", "POST", "PUT", "DELETE"}).Draw(t, "method")

		client, err := New(WithBaseURL(server.URL))
		assert.NoError(t, err)

		ctx := context.Background()
		var resp *Response

		switch method {
		case "GET":
			resp, err = client.Get(ctx, path, nil)
		case "POST":
			resp, err = client.Post(ctx, path, nil, nil)
		case "PUT":
			resp, err = client.Put(ctx, path, nil, nil)
		case "DELETE":
			resp, err = client.Delete(ctx, path, nil)
		}

		// Should never panic and should return valid response
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
