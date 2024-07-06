package redisrpc

import "encoding/json"

type Response struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func (r *Response) ParseResut(v interface{}) error {
	return json.Unmarshal(r.Result, v)
}
