package httpclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
)

// MockHandler is a function that handles mock requests.
type MockHandler func(req *http.Request) (*http.Response, error)

// MockTransport implements http.RoundTripper for testing.
type MockTransport struct {
	mu           sync.RWMutex
	handlers     map[string]MockHandler
	methodRoutes map[string]map[string]MockHandler
	sequences    map[string]*responseSequence
	requests     []*http.Request
}

type responseSequence struct {
	responses []*http.Response
	index     int
}

// NewMockTransport creates a new mock transport.
func NewMockTransport() *MockTransport {
	return &MockTransport{
		handlers:     make(map[string]MockHandler),
		methodRoutes: make(map[string]map[string]MockHandler),
		sequences:    make(map[string]*responseSequence),
	}
}

// RoundTrip implements http.RoundTripper.
func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	// Store a copy of the request
	m.requests = append(m.requests, req)
	m.mu.Unlock()

	path := req.URL.Path

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check for response sequence
	if seq, ok := m.sequences[path]; ok {
		if seq.index < len(seq.responses) {
			resp := seq.responses[seq.index]
			seq.index++
			return resp, nil
		}
	}

	// Check for method-specific handler
	if methodHandlers, ok := m.methodRoutes[req.Method]; ok {
		if handler, ok := methodHandlers[path]; ok {
			return handler(req)
		}
	}

	// Check for path handler
	if handler, ok := m.handlers[path]; ok {
		return handler(req)
	}

	return nil, errors.New("mock: no handler registered for " + req.Method + " " + path)
}

// AddResponse adds a simple JSON response for a path.
func (m *MockTransport) AddResponse(path string, statusCode int, body any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers[path] = func(req *http.Request) (*http.Response, error) {
		return MockJSONResponse(statusCode, body), nil
	}
}

// AddResponseForMethod adds a response for a specific method and path.
func (m *MockTransport) AddResponseForMethod(method, path string, statusCode int, body any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.methodRoutes[method] == nil {
		m.methodRoutes[method] = make(map[string]MockHandler)
	}

	m.methodRoutes[method][path] = func(req *http.Request) (*http.Response, error) {
		return MockJSONResponse(statusCode, body), nil
	}
}

// AddHandler adds a custom handler for a path.
func (m *MockTransport) AddHandler(path string, handler MockHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers[path] = handler
}

// AddResponseSequence adds a sequence of responses for a path.
func (m *MockTransport) AddResponseSequence(path string, responses ...*http.Response) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sequences[path] = &responseSequence{
		responses: responses,
		index:     0,
	}
}

// Requests returns all recorded requests.
func (m *MockTransport) Requests() []*http.Request {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*http.Request, len(m.requests))
	copy(result, m.requests)
	return result
}

// WasCalled returns true if the path was called at least once.
func (m *MockTransport) WasCalled(path string) bool {
	return m.CallCount(path) > 0
}

// CallCount returns the number of times a path was called.
func (m *MockTransport) CallCount(path string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, req := range m.requests {
		if req.URL.Path == path {
			count++
		}
	}
	return count
}

// Reset clears all recorded requests and resets sequences.
func (m *MockTransport) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests = nil
	for _, seq := range m.sequences {
		seq.index = 0
	}
}

// MockJSONResponse creates a mock HTTP response with JSON body.
func MockJSONResponse(statusCode int, body any) *http.Response {
	var bodyReader io.ReadCloser

	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			bodyReader = io.NopCloser(bytes.NewReader([]byte(`{"error":"marshal failed"}`)))
		} else {
			bodyReader = io.NopCloser(bytes.NewReader(data))
		}
	} else {
		bodyReader = io.NopCloser(bytes.NewReader(nil))
	}

	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: bodyReader,
	}
}

// MockErrorResponse creates a mock HTTP error response.
func MockErrorResponse(statusCode int, message string) *http.Response {
	body := map[string]string{"error": message}
	return MockJSONResponse(statusCode, body)
}

// mockNetworkError represents a network error for testing.
type mockNetworkError struct {
	message string
}

func (e *mockNetworkError) Error() string {
	return e.message
}

// MockNetworkError creates a mock network error.
func MockNetworkError(message string) error {
	return &mockNetworkError{message: message}
}
