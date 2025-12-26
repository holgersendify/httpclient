// Package internal contains internal utilities for httpclient.
package internal

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"strings"
)

// EncodeBody encodes the given body into an io.Reader and returns the content type.
// Supported types:
//   - nil: returns nil reader with empty content type
//   - []byte: returns as-is
//   - string: returns as string reader
//   - io.Reader: returns as-is
//   - url.Values: returns form-encoded
//   - other: JSON encodes
func EncodeBody(body any) (io.Reader, string, error) {
	if body == nil {
		return nil, "", nil
	}

	switch v := body.(type) {
	case []byte:
		return bytes.NewReader(v), "", nil
	case string:
		return strings.NewReader(v), "", nil
	case io.Reader:
		return v, "", nil
	case url.Values:
		return strings.NewReader(v.Encode()), "application/x-www-form-urlencoded", nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, "", err
		}
		return bytes.NewReader(data), "application/json", nil
	}
}
