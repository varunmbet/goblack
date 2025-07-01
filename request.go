package goblack

import (
	"bytes"
	"io"
)

// RequestBody represents a request body.
type RequestBody struct {
	reader io.ReadCloser
	ctx    *Context
}

// NewRequestBody ...
func NewRequestBody(r io.ReadCloser, c *Context) *RequestBody {
	return &RequestBody{
		reader: r,
		ctx:    c,
	}
}

// Bytes reads and returns content of request body in bytes.
func (rb *RequestBody) Bytes() ([]byte, error) {
	data, err := io.ReadAll(rb.reader)
	rb.ctx.Req.Body = io.NopCloser(bytes.NewBuffer(data))
	return data, err
}

// String reads and returns content of request body in string.
func (rb *RequestBody) String() (string, error) {
	data, err := rb.Bytes()
	if err != nil {
		return "", err
	}
	return string(data), err
}

// ReadCloser returns a ReadCloser for request body.
func (rb *RequestBody) ReadCloser() io.ReadCloser {
	return rb.reader
}
