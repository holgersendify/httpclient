package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Logger defines the interface for structured logging.
type Logger interface {
	Log(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr)
}

// LogBodyConfig configures body logging behavior.
type LogBodyConfig struct {
	MaxBodySize    int // total body limit in bytes (default: 4096)
	MaxStringValue int // max JSON string value in bytes (default: 1024)
}

// DefaultLogBodyConfig returns the default body logging configuration.
func DefaultLogBodyConfig() LogBodyConfig {
	return LogBodyConfig{
		MaxBodySize:    4096, // 4KB
		MaxStringValue: 1024, // 1KB
	}
}

// defaultLogger wraps slog.Logger to implement Logger interface.
type defaultLogger struct {
	logger *slog.Logger
}

func newDefaultLogger() *defaultLogger {
	return &defaultLogger{
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (l *defaultLogger) Log(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	l.logger.LogAttrs(ctx, level, msg, attrs...)
}

// sensitiveHeaders contains headers that should be redacted in logs.
var sensitiveHeaders = []string{
	"authorization",
	"x-api-key",
	"cookie",
	"set-cookie",
}

// sensitivePatterns contains patterns that indicate sensitive headers.
var sensitivePatterns = []string{
	"token",
	"secret",
	"password",
	"key",
}

// binaryContentTypes contains content types that should not be logged.
var binaryContentTypes = []string{
	"image/",
	"video/",
	"audio/",
	"application/octet-stream",
	"application/pdf",
	"application/zip",
	"application/gzip",
	"application/x-tar",
}

// isSensitiveHeader checks if a header should be redacted.
func isSensitiveHeader(name string) bool {
	lower := strings.ToLower(name)

	for _, h := range sensitiveHeaders {
		if lower == h {
			return true
		}
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// isBinaryContentType checks if a content type is binary.
func isBinaryContentType(contentType string) bool {
	lower := strings.ToLower(contentType)
	for _, binary := range binaryContentTypes {
		if strings.HasPrefix(lower, binary) {
			return true
		}
	}
	return false
}

// redactHeadersForLog returns a copy of headers with sensitive values redacted for logging.
func redactHeadersForLog(headers map[string][]string) map[string]string {
	result := make(map[string]string)
	for name, values := range headers {
		if isSensitiveHeader(name) {
			result[name] = "[REDACTED]"
		} else if len(values) > 0 {
			result[name] = values[0]
		}
	}
	return result
}

// formatBodyForLog formats a body for logging, applying truncation rules.
func formatBodyForLog(body []byte, contentType string, config LogBodyConfig) any {
	if len(body) == 0 {
		return nil
	}

	// Handle binary content types
	if isBinaryContentType(contentType) {
		return fmt.Sprintf("[binary: %s]", formatBytes(len(body)))
	}

	// Handle large bodies
	if len(body) > config.MaxBodySize {
		return fmt.Sprintf("[body: %s truncated]", formatBytes(len(body)))
	}

	// Try to parse as JSON for string truncation
	if strings.Contains(strings.ToLower(contentType), "json") {
		var data any
		if err := json.Unmarshal(body, &data); err == nil {
			return truncateJSONStrings(data, config.MaxStringValue)
		}
	}

	// Return as string for text content
	if strings.HasPrefix(strings.ToLower(contentType), "text/") {
		return string(body)
	}

	// Default: return as string
	return string(body)
}

// truncateJSONStrings recursively truncates large string values in JSON data.
func truncateJSONStrings(data any, maxSize int) any {
	switch v := data.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, value := range v {
			result[key] = truncateJSONStrings(value, maxSize)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, value := range v {
			result[i] = truncateJSONStrings(value, maxSize)
		}
		return result
	case string:
		if len(v) > maxSize {
			return fmt.Sprintf("[string: %s truncated]", formatBytes(len(v)))
		}
		return v
	default:
		return v
	}
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(bytes int) string {
	const (
		KB = 1024
		MB = 1024 * KB
	)

	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
