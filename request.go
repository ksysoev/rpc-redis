package redisrpc

import (
	"context"
	"encoding/json"
)

type Request struct {
	ctx    context.Context
	req_id string
	params string
}

func NewRequest(ctx context.Context, req_id string, params string) Request {
	return Request{
		req_id: req_id,
		params: params,
	}
}

func (r Request) Context() context.Context {
	return r.ctx
}

func (r Request) ID() string {
	return r.req_id
}

func (r Request) ParseParams(v any) error {
	return json.Unmarshal([]byte(r.params), v)
}
