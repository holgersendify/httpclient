package httpclient

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogger captures log entries for testing.
type testLogger struct {
	mu      sync.Mutex
	entries []logEntry
}

type logEntry struct {
	Level slog.Level
	Msg   string
	Attrs map[string]any
}

func (l *testLogger) Log(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := logEntry{
		Level: level,
		Msg:   msg,
		Attrs: make(map[string]any),
	}
	for _, attr := range attrs {
		entry.Attrs[attr.Key] = attr.Value.Any()
	}
	l.entries = append(l.entries, entry)
}

func (l *testLogger) LastEntry() logEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) == 0 {
		return logEntry{}
	}
	return l.entries[len(l.entries)-1]
}

func (l *testLogger) Entries() []logEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]logEntry{}, l.entries...)
}

func TestLogger_Interface(t *testing.T) {
	// Verify testLogger implements Logger interface
	var _ Logger = (*testLogger)(nil)
}

func TestWithThirdPartyCode(t *testing.T) {
	logger := &testLogger{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client, err := New(
		WithBaseURL(server.URL),
		WithThirdPartyCode("stripe"),
		WithLogger(logger),
	)
	require.NoError(t, err)

	_, err = client.Get(context.Background(), "/test", nil)
	require.NoError(t, err)

	entry := logger.LastEntry()
	assert.Equal(t, "http_request", entry.Msg)
	assert.Equal(t, "stripe", entry.Attrs["third_party_code"])
}

func TestLogger_LogsRequestAndResponse(t *testing.T) {
	logger := &testLogger{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"123","name":"test"}`))
	}))
	defer server.Close()

	client, err := New(
		WithBaseURL(server.URL),
		WithThirdPartyCode("test-api"),
		WithLogger(logger),
	)
	require.NoError(t, err)

	type request struct {
		Amount int `json:"amount"`
	}
	_, err = client.Post(context.Background(), "/charges", request{Amount: 1000}, nil)
	require.NoError(t, err)

	entry := logger.LastEntry()
	assert.Equal(t, slog.LevelInfo, entry.Level)
	assert.Equal(t, "http_request", entry.Msg)
	assert.Equal(t, "POST", entry.Attrs["method"])
	assert.Contains(t, entry.Attrs["url"], "/charges")
	assert.EqualValues(t, 200, entry.Attrs["status"]) // EqualValues handles int/int64
	assert.NotNil(t, entry.Attrs["duration_ms"])
	assert.NotNil(t, entry.Attrs["request_body"])
	assert.NotNil(t, entry.Attrs["response_body"])
}

func TestLogger_RedactsSensitiveHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header string
		value  string
	}{
		{"Authorization", "Authorization", "Bearer secret-token"},
		{"X-API-Key", "X-Api-Key", "sk_live_123456"}, // Canonical form
		{"Cookie", "Cookie", "session=abc123"},
		{"X-Auth-Token", "X-Auth-Token", "token123"},
		{"X-Secret-Key", "X-Secret-Key", "secret123"},
		{"Password-Header", "X-Password", "pass123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &testLogger{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client, err := New(
				WithBaseURL(server.URL),
				WithLogger(logger),
				WithHeader(tt.header, tt.value),
			)
			require.NoError(t, err)

			_, err = client.Get(context.Background(), "/test", nil)
			require.NoError(t, err)

			entry := logger.LastEntry()
			headers, ok := entry.Attrs["request_headers"].(map[string]string)
			require.True(t, ok, "request_headers should be map[string]string")
			// Use canonical header key for lookup
			canonicalKey := http.CanonicalHeaderKey(tt.header)
			assert.Equal(t, "[REDACTED]", headers[canonicalKey], "header %s should be redacted", tt.header)
		})
	}
}

func TestLogger_TruncatesLargeBody(t *testing.T) {
	logger := &testLogger{}

	// Create a response larger than MaxBodySize (4KB)
	largeBody := strings.Repeat("x", 5000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(largeBody))
	}))
	defer server.Close()

	client, err := New(
		WithBaseURL(server.URL),
		WithLogger(logger),
	)
	require.NoError(t, err)

	_, err = client.Get(context.Background(), "/large", nil)
	require.NoError(t, err)

	entry := logger.LastEntry()
	respBody, ok := entry.Attrs["response_body"].(string)
	require.True(t, ok)
	assert.Contains(t, respBody, "[body:")
	assert.Contains(t, respBody, "truncated]")
}

func TestLogger_TruncatesLargeJSONStringValues(t *testing.T) {
	logger := &testLogger{}

	// Create JSON with a string value larger than MaxStringValue (1KB)
	largeString := strings.Repeat("x", 2000)
	responseJSON := map[string]any{
		"id":   "123",
		"data": largeString,
	}
	responseBytes, _ := json.Marshal(responseJSON)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(responseBytes)
	}))
	defer server.Close()

	client, err := New(
		WithBaseURL(server.URL),
		WithLogger(logger),
	)
	require.NoError(t, err)

	_, err = client.Get(context.Background(), "/json", nil)
	require.NoError(t, err)

	entry := logger.LastEntry()
	respBody, ok := entry.Attrs["response_body"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "123", respBody["id"])
	dataStr, ok := respBody["data"].(string)
	require.True(t, ok)
	assert.Contains(t, dataStr, "[string:")
	assert.Contains(t, dataStr, "truncated]")
}

func TestLogger_SkipsBinaryContentTypes(t *testing.T) {
	binaryTypes := []string{
		"image/png",
		"image/jpeg",
		"video/mp4",
		"audio/mpeg",
		"application/octet-stream",
		"application/pdf",
		"application/zip",
	}

	for _, contentType := range binaryTypes {
		t.Run(contentType, func(t *testing.T) {
			logger := &testLogger{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", contentType)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("binary data here"))
			}))
			defer server.Close()

			client, err := New(
				WithBaseURL(server.URL),
				WithLogger(logger),
			)
			require.NoError(t, err)

			_, err = client.Get(context.Background(), "/binary", nil)
			require.NoError(t, err)

			entry := logger.LastEntry()
			respBody, ok := entry.Attrs["response_body"].(string)
			require.True(t, ok)
			assert.Contains(t, respBody, "[binary:")
		})
	}
}

func TestLogger_DefaultEnabled(t *testing.T) {
	// Logging should be enabled by default with slog JSON logger
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(
		WithBaseURL(server.URL),
	)
	require.NoError(t, err)

	// Should not panic - default logger should be set
	_, err = client.Get(context.Background(), "/test", nil)
	require.NoError(t, err)
}

func TestLogger_DisableWithNilLogger(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(
		WithBaseURL(server.URL),
		WithLoggerDisabled(),
	)
	require.NoError(t, err)

	// Should not panic with logging disabled
	_, err = client.Get(context.Background(), "/test", nil)
	require.NoError(t, err)
}

func TestLogger_LogsErrorResponses(t *testing.T) {
	logger := &testLogger{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid request"}`))
	}))
	defer server.Close()

	client, err := New(
		WithBaseURL(server.URL),
		WithLogger(logger),
	)
	require.NoError(t, err)

	_, _ = client.Get(context.Background(), "/error", nil)

	entry := logger.LastEntry()
	assert.Equal(t, slog.LevelError, entry.Level)
	assert.EqualValues(t, 400, entry.Attrs["status"]) // EqualValues handles int/int64
}
