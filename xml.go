package httpclient

import (
	"bytes"
	"encoding/xml"
	"io"
)

// xmlBody wraps a value to be serialized as XML.
type xmlBody struct {
	value       any
	rootElement string
	declaration bool
}

// XMLBody creates an XML body from the given value.
func XMLBody(v any) any {
	return &xmlBody{value: v}
}

// XMLBodyWithRoot creates an XML body with a custom root element name.
func XMLBodyWithRoot(rootElement string, v any) any {
	return &xmlBody{value: v, rootElement: rootElement}
}

// XMLBodyWithDeclaration creates an XML body with XML declaration.
func XMLBodyWithDeclaration(v any) any {
	return &xmlBody{value: v, declaration: true}
}

// XMLBodyFull creates an XML body with all options.
func XMLBodyFull(v any, rootElement string, declaration bool) any {
	return &xmlBody{value: v, rootElement: rootElement, declaration: declaration}
}

// IsXMLBody checks if the value is an XML body wrapper.
func IsXMLBody(v any) bool {
	_, ok := v.(*xmlBody)
	return ok
}

// EncodeXMLBody encodes an XML body to bytes and returns the content type.
func EncodeXMLBody(v any) (io.Reader, string, error) {
	xb, ok := v.(*xmlBody)
	if !ok {
		return nil, "", nil
	}

	var buf bytes.Buffer

	if xb.declaration {
		buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	}

	if xb.rootElement != "" {
		buf.WriteString("<" + xb.rootElement + ">")
	}

	data, err := xml.Marshal(xb.value)
	if err != nil {
		return nil, "", err
	}
	buf.Write(data)

	if xb.rootElement != "" {
		buf.WriteString("</" + xb.rootElement + ">")
	}

	return &buf, "application/xml", nil
}
