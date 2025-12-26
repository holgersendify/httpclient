package httpclient

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
)

// Response represents an HTTP response.
type Response struct {
	StatusCode int
	Status     string
	Headers    http.Header
	Body       []byte
}

// JSON unmarshals the response body as JSON into the given target.
func (r *Response) JSON(v any) error {
	if v == nil {
		return errors.New("target cannot be nil")
	}
	return json.Unmarshal(r.Body, v)
}

// XML unmarshals the response body as XML into the given target.
func (r *Response) XML(v any) error {
	if v == nil {
		return errors.New("target cannot be nil")
	}
	return xml.Unmarshal(r.Body, v)
}

// String returns the response body as a string.
func (r *Response) String() string {
	return string(r.Body)
}

// IsSuccess returns true if the status code is 2xx.
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// IsClientError returns true if the status code is 4xx.
func (r *Response) IsClientError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 500
}

// IsServerError returns true if the status code is 5xx.
func (r *Response) IsServerError() bool {
	return r.StatusCode >= 500 && r.StatusCode < 600
}
