package redisrpc

import (
	"context"
	"encoding/json"
)

type Request interface {
	Context() context.Context
	ID() string
	ParseParams(v any) error
	Method() string
	ReplyTo() string
}

type request struct {
	ctx     context.Context
	method  string
	req_id  string
	params  string
	replyTo string
}

func NewRequest(ctx context.Context, method, req_id, params, replyTo string) Request {
	return &request{
		ctx:     ctx,
		req_id:  req_id,
		params:  params,
		method:  method,
		replyTo: replyTo,
	}
}

func (r request) Context() context.Context {
	return r.ctx
}

func (r request) ID() string {
	return r.req_id
}

func (r request) ParseParams(v any) error {
	return json.Unmarshal([]byte(r.params), v)
}

func (r request) Method() string {
	return r.method
}

func (r request) ReplyTo() string {
	return r.replyTo
}
