package httpclient

import (
	"testing"
)

// FuzzParseRetryAfter tests ParseRetryAfter with random inputs.
func FuzzParseRetryAfter(f *testing.F) {
	// Add seed corpus
	f.Add("0")
	f.Add("1")
	f.Add("60")
	f.Add("3600")
	f.Add("")
	f.Add("abc")
	f.Add("-1")
	f.Add("999999999999999999999")
	f.Add("Wed, 21 Oct 2025 07:28:00 GMT")
	f.Add("Fri, 31 Dec 2025 23:59:59 GMT")

	f.Fuzz(func(t *testing.T, input string) {
		// ParseRetryAfter should never panic
		result := ParseRetryAfter(input)

		// Result should be non-negative (0 means invalid/unparseable)
		if result < 0 {
			t.Errorf("ParseRetryAfter(%q) returned negative duration: %v", input, result)
		}
	})
}

// FuzzParseSOAPFault tests ParseSOAPFault with random inputs.
func FuzzParseSOAPFault(f *testing.F) {
	// Add seed corpus with valid and invalid SOAP
	f.Add([]byte(``))
	f.Add([]byte(`not xml`))
	f.Add([]byte(`<xml>`))
	f.Add([]byte(`<?xml version="1.0"?><root></root>`))
	f.Add([]byte(`<?xml version="1.0"?>
		<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
			<soap:Body>
				<soap:Fault>
					<faultcode>soap:Server</faultcode>
					<faultstring>Error</faultstring>
				</soap:Fault>
			</soap:Body>
		</soap:Envelope>`))
	f.Add([]byte(`<?xml version="1.0"?>
		<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
			<soap:Body>
				<soap:Fault>
					<soap:Code><soap:Value>soap:Receiver</soap:Value></soap:Code>
					<soap:Reason><soap:Text>Error</soap:Text></soap:Reason>
				</soap:Fault>
			</soap:Body>
		</soap:Envelope>`))

	f.Fuzz(func(t *testing.T, input []byte) {
		// ParseSOAPFault should never panic
		fault, ok := ParseSOAPFault(input)

		// If ok is true, fault should not be nil
		if ok && fault == nil {
			t.Error("ParseSOAPFault returned ok=true but fault=nil")
		}

		// If ok is false, fault should be nil
		if !ok && fault != nil {
			t.Error("ParseSOAPFault returned ok=false but fault!=nil")
		}
	})
}

// FuzzResponseJSON tests Response.JSON with random inputs.
func FuzzResponseJSON(f *testing.F) {
	// Add seed corpus
	f.Add([]byte(``))
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`null`))
	f.Add([]byte(`"string"`))
	f.Add([]byte(`123`))
	f.Add([]byte(`{"key": "value"}`))
	f.Add([]byte(`{"nested": {"key": "value"}}`))
	f.Add([]byte(`[1, 2, 3]`))
	f.Add([]byte(`not json`))
	f.Add([]byte(`{"incomplete`))

	f.Fuzz(func(t *testing.T, input []byte) {
		resp := &Response{
			StatusCode: 200,
			Body:       input,
		}

		var result any
		// JSON should never panic
		_ = resp.JSON(&result)
	})
}

// FuzzResponseXML tests Response.XML with random inputs.
func FuzzResponseXML(f *testing.F) {
	// Add seed corpus
	f.Add([]byte(``))
	f.Add([]byte(`<root/>`))
	f.Add([]byte(`<root></root>`))
	f.Add([]byte(`<?xml version="1.0"?><root></root>`))
	f.Add([]byte(`<root><child>value</child></root>`))
	f.Add([]byte(`not xml`))
	f.Add([]byte(`<incomplete`))
	f.Add([]byte(`<root attr="value"/>`))

	f.Fuzz(func(t *testing.T, input []byte) {
		resp := &Response{
			StatusCode: 200,
			Body:       input,
		}

		var result any
		// XML should never panic
		_ = resp.XML(&result)
	})
}
