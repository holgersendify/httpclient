package httpclient

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponse_JSON(t *testing.T) {
	t.Run("unmarshals valid JSON", func(t *testing.T) {
		resp := &Response{
			StatusCode: 200,
			Body:       []byte(`{"name":"test","value":42}`),
		}

		var result struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}
		err := resp.JSON(&result)

		require.NoError(t, err)
		assert.Equal(t, "test", result.Name)
		assert.Equal(t, 42, result.Value)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		resp := &Response{
			StatusCode: 200,
			Body:       []byte(`{invalid`),
		}

		var result struct{}
		err := resp.JSON(&result)

		require.Error(t, err)
	})

	t.Run("returns error for nil target", func(t *testing.T) {
		resp := &Response{
			StatusCode: 200,
			Body:       []byte(`{}`),
		}

		err := resp.JSON(nil)

		require.Error(t, err)
	})
}

func TestResponse_XML(t *testing.T) {
	t.Run("unmarshals valid XML", func(t *testing.T) {
		resp := &Response{
			StatusCode: 200,
			Body:       []byte(`<root><name>test</name><value>42</value></root>`),
		}

		var result struct {
			Name  string `xml:"name"`
			Value int    `xml:"value"`
		}
		err := resp.XML(&result)

		require.NoError(t, err)
		assert.Equal(t, "test", result.Name)
		assert.Equal(t, 42, result.Value)
	})

	t.Run("returns error for invalid XML", func(t *testing.T) {
		resp := &Response{
			StatusCode: 200,
			Body:       []byte(`<root><unclosed>`),
		}

		var result struct{}
		err := resp.XML(&result)

		require.Error(t, err)
	})

	t.Run("returns error for nil target", func(t *testing.T) {
		resp := &Response{
			StatusCode: 200,
			Body:       []byte(`<root/>`),
		}

		err := resp.XML(nil)

		require.Error(t, err)
	})
}

func TestResponse_String(t *testing.T) {
	t.Run("returns body as string", func(t *testing.T) {
		resp := &Response{
			Body: []byte("hello world"),
		}

		assert.Equal(t, "hello world", resp.String())
	})

	t.Run("returns empty string for nil body", func(t *testing.T) {
		resp := &Response{}

		assert.Equal(t, "", resp.String())
	})
}

func TestResponse_IsSuccess(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"200 is success", http.StatusOK, true},
		{"201 is success", http.StatusCreated, true},
		{"204 is success", http.StatusNoContent, true},
		{"299 is success", 299, true},
		{"199 is not success", 199, false},
		{"300 is not success", 300, false},
		{"400 is not success", 400, false},
		{"500 is not success", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{StatusCode: tt.statusCode}
			assert.Equal(t, tt.expected, resp.IsSuccess())
		})
	}
}

func TestResponse_IsClientError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"400 is client error", 400, true},
		{"404 is client error", 404, true},
		{"499 is client error", 499, true},
		{"399 is not client error", 399, false},
		{"500 is not client error", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{StatusCode: tt.statusCode}
			assert.Equal(t, tt.expected, resp.IsClientError())
		})
	}
}

func TestResponse_IsServerError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"500 is server error", 500, true},
		{"503 is server error", 503, true},
		{"599 is server error", 599, true},
		{"499 is not server error", 499, false},
		{"600 is not server error", 600, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{StatusCode: tt.statusCode}
			assert.Equal(t, tt.expected, resp.IsServerError())
		})
	}
}
