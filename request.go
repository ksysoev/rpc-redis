package rpc

import (
	"context"
	"encoding/json"
)

// Request represents a request in the Redis RPC system.
type Request interface {
	// Context returns the context associated with the request.
	Context() context.Context

	// ID returns the unique identifier of the request.
	ID() string

	// ParseParams parses the request parameters into the provided value.
	// It returns an error if the parsing fails.
	ParseParams(v any) error

	// Method returns the name of the method associated with the request.
	Method() string

	// ReplyTo returns the name of the queue where the reply should be sent.
	ReplyTo() string
}

type request struct {
	ctx     context.Context
	method  string
	req_id  string
	params  string
	replyTo string
}

// NewRequest creates a new Request with the provided parameters.
// It returns a Request interface.
func NewRequest(ctx context.Context, method, req_id, params, replyTo string) Request {
	return &request{
		ctx:     ctx,
		req_id:  req_id,
		params:  params,
		method:  method,
		replyTo: replyTo,
	}
}

// Context returns the context associated with the request.
func (r request) Context() context.Context {
	return r.ctx
}

// ID returns the unique identifier of the request.
func (r request) ID() string {
	return r.req_id
}

// ParseParams parses the JSON-encoded parameters of the request into the provided value.
// The value must be a pointer to the desired type.
func (r request) ParseParams(v any) error {
	return json.Unmarshal([]byte(r.params), v)
}

// Method returns the HTTP method of the request.
func (r request) Method() string {
	return r.method
}

// ReplyTo returns the replyTo field of the request.
func (r request) ReplyTo() string {
	return r.replyTo
}
