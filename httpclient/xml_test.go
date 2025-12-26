package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type XMLPerson struct {
	Name string `xml:"name"`
	Age  int    `xml:"age"`
}

func TestXMLBody(t *testing.T) {
	t.Run("sends XML body with correct content type", func(t *testing.T) {
		var receivedBody string
		var receivedContentType string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			receivedBody = string(body)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		person := XMLPerson{Name: "Alice", Age: 30}
		_, err = client.Post(context.Background(), "/test", XMLBody(person), nil)
		require.NoError(t, err)

		assert.Equal(t, "application/xml", receivedContentType)
		assert.Contains(t, receivedBody, "<name>Alice</name>")
		assert.Contains(t, receivedBody, "<age>30</age>")
	})

	t.Run("sends XML body with custom root element", func(t *testing.T) {
		var receivedBody string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			receivedBody = string(body)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		person := XMLPerson{Name: "Bob", Age: 25}
		_, err = client.Post(context.Background(), "/test", XMLBodyWithRoot("person", person), nil)
		require.NoError(t, err)

		assert.Contains(t, receivedBody, "<person>")
		assert.Contains(t, receivedBody, "</person>")
	})

	t.Run("parses XML response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<?xml version="1.0"?><person><name>Charlie</name><age>35</age></person>`))
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		resp, err := client.Get(context.Background(), "/test", nil)
		require.NoError(t, err)

		var person XMLPerson
		err = resp.XML(&person)
		require.NoError(t, err)

		assert.Equal(t, "Charlie", person.Name)
		assert.Equal(t, 35, person.Age)
	})
}

func TestXMLBodyEncoding(t *testing.T) {
	t.Run("handles special characters", func(t *testing.T) {
		var receivedBody string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			receivedBody = string(body)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		person := XMLPerson{Name: "O'Brien & Sons", Age: 40}
		_, err = client.Post(context.Background(), "/test", XMLBody(person), nil)
		require.NoError(t, err)

		// XML should escape special characters
		assert.Contains(t, receivedBody, "O&#39;Brien &amp; Sons")
	})

	t.Run("includes XML declaration when requested", func(t *testing.T) {
		var receivedBody string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			receivedBody = string(body)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		person := XMLPerson{Name: "Dana", Age: 28}
		_, err = client.Post(context.Background(), "/test", XMLBodyWithDeclaration(person), nil)
		require.NoError(t, err)

		assert.Contains(t, receivedBody, `<?xml version="1.0" encoding="UTF-8"?>`)
	})
}
