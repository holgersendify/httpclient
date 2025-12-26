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

type GetWeatherRequest struct {
	City string `xml:"City"`
}

type GetWeatherResponse struct {
	Temperature int    `xml:"Temperature"`
	Conditions  string `xml:"Conditions"`
}

func TestSOAPEnvelope(t *testing.T) {
	t.Run("wraps body in SOAP envelope", func(t *testing.T) {
		var receivedBody string
		var receivedContentType string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			receivedBody = string(body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
				<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
					<soap:Body>
						<GetWeatherResponse>
							<Temperature>72</Temperature>
							<Conditions>Sunny</Conditions>
						</GetWeatherResponse>
					</soap:Body>
				</soap:Envelope>`))
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		req := GetWeatherRequest{City: "Seattle"}
		_, err = client.Post(context.Background(), "/weather", SOAPBody(req), nil)
		require.NoError(t, err)

		assert.Equal(t, "text/xml; charset=utf-8", receivedContentType)
		assert.Contains(t, receivedBody, "soap:Envelope")
		assert.Contains(t, receivedBody, "soap:Body")
		assert.Contains(t, receivedBody, "<City>Seattle</City>")
	})

	t.Run("uses SOAP 1.2 when specified", func(t *testing.T) {
		var receivedContentType string
		var receivedBody string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedContentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			receivedBody = string(body)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		req := GetWeatherRequest{City: "Portland"}
		_, err = client.Post(context.Background(), "/weather", SOAP12Body(req), nil)
		require.NoError(t, err)

		assert.Equal(t, "application/soap+xml; charset=utf-8", receivedContentType)
		assert.Contains(t, receivedBody, "http://www.w3.org/2003/05/soap-envelope")
	})

	t.Run("includes SOAPAction header when provided", func(t *testing.T) {
		var receivedAction string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAction = r.Header.Get("SOAPAction")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		req := GetWeatherRequest{City: "Denver"}
		_, err = client.Post(context.Background(), "/weather", SOAPBodyWithAction("http://example.com/GetWeather", req), nil)
		require.NoError(t, err)

		assert.Equal(t, `"http://example.com/GetWeather"`, receivedAction)
	})
}

func TestSOAPFault(t *testing.T) {
	t.Run("detects SOAP 1.1 fault", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
				<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
					<soap:Body>
						<soap:Fault>
							<faultcode>soap:Server</faultcode>
							<faultstring>City not found</faultstring>
							<detail>The city "Unknown" was not found in the database</detail>
						</soap:Fault>
					</soap:Body>
				</soap:Envelope>`))
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		req := GetWeatherRequest{City: "Unknown"}
		resp, err := client.Post(context.Background(), "/weather", SOAPBody(req), nil)
		require.Error(t, err)
		require.NotNil(t, resp)

		fault, ok := ParseSOAPFault(resp.Body)
		require.True(t, ok)
		assert.Equal(t, "soap:Server", fault.Code)
		assert.Equal(t, "City not found", fault.String)
		assert.Contains(t, fault.Detail, "Unknown")
	})

	t.Run("detects SOAP 1.2 fault", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/soap+xml")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
				<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
					<soap:Body>
						<soap:Fault>
							<soap:Code>
								<soap:Value>soap:Receiver</soap:Value>
							</soap:Code>
							<soap:Reason>
								<soap:Text>Service unavailable</soap:Text>
							</soap:Reason>
						</soap:Fault>
					</soap:Body>
				</soap:Envelope>`))
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		req := GetWeatherRequest{City: "Test"}
		resp, err := client.Post(context.Background(), "/weather", SOAP12Body(req), nil)
		require.Error(t, err)
		require.NotNil(t, resp)

		fault, ok := ParseSOAPFault(resp.Body)
		require.True(t, ok)
		assert.Contains(t, fault.Code, "Receiver")
		assert.Contains(t, fault.String, "Service unavailable")
	})

	t.Run("returns false for non-fault response", func(t *testing.T) {
		normalResponse := []byte(`<?xml version="1.0"?>
			<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
				<soap:Body>
					<GetWeatherResponse>
						<Temperature>72</Temperature>
					</GetWeatherResponse>
				</soap:Body>
			</soap:Envelope>`)

		fault, ok := ParseSOAPFault(normalResponse)
		assert.False(t, ok)
		assert.Nil(t, fault)
	})
}

func TestSOAPResponseParsing(t *testing.T) {
	t.Run("parses SOAP response body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
				<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
					<soap:Body>
						<GetWeatherResponse>
							<Temperature>72</Temperature>
							<Conditions>Sunny</Conditions>
						</GetWeatherResponse>
					</soap:Body>
				</soap:Envelope>`))
		}))
		defer server.Close()

		client, err := New(WithBaseURL(server.URL))
		require.NoError(t, err)

		resp, err := client.Post(context.Background(), "/weather", SOAPBody(GetWeatherRequest{City: "Seattle"}), nil)
		require.NoError(t, err)

		var weather GetWeatherResponse
		err = ParseSOAPResponse(resp.Body, &weather)
		require.NoError(t, err)

		assert.Equal(t, 72, weather.Temperature)
		assert.Equal(t, "Sunny", weather.Conditions)
	})
}
