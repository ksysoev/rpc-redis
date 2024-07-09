package redisrpc

import "encoding/json"

// Response represents the response structure for a Redis RPC call.
type Response struct {
	ID     string          `json:"id"`               // ID is the unique identifier for the request.
	Result json.RawMessage `json:"result,omitempty"` // Result contains the response result, if any.
	Error  string          `json:"error,omitempty"`  // Error contains the error message, if any.
}

// ParseResut parses the result of the response into the provided value.
// The result is expected to be in JSON format and will be unmarshaled into the provided value.
func (r *Response) ParseResut(v interface{}) error {
	return json.Unmarshal(r.Result, v)
}
