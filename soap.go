package httpclient

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

const (
	soap11Namespace = "http://schemas.xmlsoap.org/soap/envelope/"
	soap12Namespace = "http://www.w3.org/2003/05/soap-envelope"
)

// soapBody wraps a value to be sent as a SOAP request.
type soapBody struct {
	value   any
	action  string
	soap12  bool
}

// SOAPBody creates a SOAP 1.1 body from the given value.
func SOAPBody(v any) any {
	return &soapBody{value: v}
}

// SOAP12Body creates a SOAP 1.2 body from the given value.
func SOAP12Body(v any) any {
	return &soapBody{value: v, soap12: true}
}

// SOAPBodyWithAction creates a SOAP 1.1 body with a SOAPAction header.
func SOAPBodyWithAction(action string, v any) any {
	return &soapBody{value: v, action: action}
}

// SOAP12BodyWithAction creates a SOAP 1.2 body with an action.
func SOAP12BodyWithAction(action string, v any) any {
	return &soapBody{value: v, action: action, soap12: true}
}

// IsSOAPBody checks if the value is a SOAP body wrapper.
func IsSOAPBody(v any) bool {
	_, ok := v.(*soapBody)
	return ok
}

// SOAPEnvelope represents a SOAP envelope.
type SOAPEnvelope struct {
	XMLName xml.Name    `xml:"soap:Envelope"`
	NS      string      `xml:"xmlns:soap,attr"`
	Body    SOAPBodyXML `xml:"soap:Body"`
}

// SOAPBodyXML represents the SOAP body element.
type SOAPBodyXML struct {
	Content []byte `xml:",innerxml"`
}

// EncodeSOAPBody encodes a SOAP body to bytes and returns the content type and optional headers.
func EncodeSOAPBody(v any) (io.Reader, string, map[string]string, error) {
	sb, ok := v.(*soapBody)
	if !ok {
		return nil, "", nil, nil
	}

	namespace := soap11Namespace
	contentType := "text/xml; charset=utf-8"
	if sb.soap12 {
		namespace = soap12Namespace
		contentType = "application/soap+xml; charset=utf-8"
	}

	// Encode the inner body content
	innerContent, err := xml.Marshal(sb.value)
	if err != nil {
		return nil, "", nil, err
	}

	envelope := SOAPEnvelope{
		NS: namespace,
		Body: SOAPBodyXML{
			Content: innerContent,
		},
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	encoder := xml.NewEncoder(&buf)
	if err := encoder.Encode(envelope); err != nil {
		return nil, "", nil, err
	}

	headers := make(map[string]string)
	if sb.action != "" {
		headers["SOAPAction"] = `"` + sb.action + `"`
	}

	return &buf, contentType, headers, nil
}

// SOAPFault represents a SOAP fault.
type SOAPFault struct {
	Code   string
	String string
	Detail string
}

// ParseSOAPFault attempts to parse a SOAP fault from the response body.
// Returns the fault and true if found, or nil and false if not a fault.
func ParseSOAPFault(body []byte) (*SOAPFault, bool) {
	bodyStr := string(body)

	// Check if it's a fault
	if !strings.Contains(bodyStr, "Fault") {
		return nil, false
	}

	// Try SOAP 1.1 fault format
	fault := parseSOAP11Fault(body)
	if fault != nil {
		return fault, true
	}

	// Try SOAP 1.2 fault format
	fault = parseSOAP12Fault(body)
	if fault != nil {
		return fault, true
	}

	return nil, false
}

type soap11FaultEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Fault struct {
			FaultCode   string `xml:"faultcode"`
			FaultString string `xml:"faultstring"`
			Detail      string `xml:"detail"`
		} `xml:"Fault"`
	} `xml:"Body"`
}

func parseSOAP11Fault(body []byte) *SOAPFault {
	var env soap11FaultEnvelope
	if err := xml.Unmarshal(body, &env); err != nil {
		return nil
	}

	if env.Body.Fault.FaultCode == "" && env.Body.Fault.FaultString == "" {
		return nil
	}

	return &SOAPFault{
		Code:   env.Body.Fault.FaultCode,
		String: env.Body.Fault.FaultString,
		Detail: env.Body.Fault.Detail,
	}
}

type soap12FaultEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Fault struct {
			Code struct {
				Value string `xml:"Value"`
			} `xml:"Code"`
			Reason struct {
				Text string `xml:"Text"`
			} `xml:"Reason"`
		} `xml:"Fault"`
	} `xml:"Body"`
}

func parseSOAP12Fault(body []byte) *SOAPFault {
	var env soap12FaultEnvelope
	if err := xml.Unmarshal(body, &env); err != nil {
		return nil
	}

	if env.Body.Fault.Code.Value == "" && env.Body.Fault.Reason.Text == "" {
		return nil
	}

	return &SOAPFault{
		Code:   env.Body.Fault.Code.Value,
		String: env.Body.Fault.Reason.Text,
	}
}

type soapResponseEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Content []byte `xml:",innerxml"`
	} `xml:"Body"`
}

// ParseSOAPResponse extracts and unmarshals the SOAP body content.
func ParseSOAPResponse(body []byte, v any) error {
	var env soapResponseEnvelope
	if err := xml.Unmarshal(body, &env); err != nil {
		return err
	}

	return xml.Unmarshal(env.Body.Content, v)
}
