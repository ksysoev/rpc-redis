package rpc

import (
	"context"
	"encoding/json"
)

type Request struct {
	ctx     context.Context
	Method  string
	ID      string
	ReplyTo string
	params  []byte
}

// NewRequest creates a new Request with the provided parameters.
// It returns a Request interface.
func NewRequest(ctx context.Context, method, id, params, replyTo string) *Request {
	return &Request{
		ctx:     ctx,
		ID:      id,
		params:  []byte(params),
		Method:  method,
		ReplyTo: replyTo,
	}
}

// Context returns the context of the request.
func (r *Request) Context() context.Context {
	return r.ctx
}

// ParseParams parses the JSON-encoded parameters of the request into the provided value.
// The value must be a pointer to the desired type.
func (r *Request) ParseParams(v any) error {
	return json.Unmarshal(r.params, v)
}

// WithContext returns a copy of r with its context changed
// to ctx. The provided ctx must be non-nil.
func (r *Request) WithContext(ctx context.Context) *Request {
	if ctx == nil {
		panic("nil context")
	}

	r2 := new(Request)
	*r2 = *r
	r2.ctx = ctx

	return r2
}
