package rpc

import (
	"errors"
	"testing"
)

func TestNewResponse(t *testing.T) {
	id := "123"
	result := "success"
	err := errors.New("some error")

	response, err := newResponse(id, result, err)
	if err == nil {
		t.Errorf("expected error, got nil")
	}

	if response != nil {
		t.Errorf("expected nil response, got %+v", response)
	}

	err = nil
	response, err = newResponse(id, result, err)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response == nil {
		t.Fatalf("expected non-nil response, got nil")
	}

	if response.ID != id {
		t.Errorf("expected ID %q, got %q", id, response.ID)
	}

	if response.Error != "" {
		t.Errorf("expected empty error, got %q", response.Error)
	}

	if string(response.Result) != "\"success\"" {
		t.Errorf("expected result %q, got %q", "\"success\"", string(response.Result))
	}
}
func TestParseResult(t *testing.T) {
	response := &Response{
		ID:     "123",
		Result: []byte(`{"name":"John","age":30}`),
	}

	var data struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	err := response.ParseResut(&data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedName := "John"
	if data.Name != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, data.Name)
	}

	expectedAge := 30
	if data.Age != expectedAge {
		t.Errorf("expected age %d, got %d", expectedAge, data.Age)
	}
}
