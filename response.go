package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Response represents the response structure for a Redis RPC call.
type Response struct {
	ID     string          `json:"id"`
	Error  string          `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

// newResponse creates a new instance of the Response struct.
func newResponse(id string, result any, err error) (*Response, error) {
	if err != nil && result != nil || err == nil && result == nil {
		return nil, errors.New("either result or error must be nil")
	}

	if err != nil {
		return &Response{
			ID:    id,
			Error: err.Error(),
		}, nil
	}

	encodedResult, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("error marshalling result: %w", err)
	}

	return &Response{
		ID:     id,
		Result: encodedResult,
	}, nil
}

// ParseResut parses the result of the response into the provided value.
// The result is expected to be in JSON format and will be unmarshaled into the provided value.
func (r *Response) ParseResut(v interface{}) error {
	return json.Unmarshal(r.Result, v)
}

// MarshalJSON returns the JSON encoding of the Response struct.
func (r *Response) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
