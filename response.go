package rpc

import "encoding/json"

// Response represents the response structure for a Redis RPC call.
type Response struct {
	ID     string          `json:"id"`
	Error  string          `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

// ParseResut parses the result of the response into the provided value.
// The result is expected to be in JSON format and will be unmarshaled into the provided value.
func (r *Response) ParseResut(v interface{}) error {
	return json.Unmarshal(r.Result, v)
}
