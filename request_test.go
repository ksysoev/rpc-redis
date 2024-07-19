package rpc

import (
	"context"
	"testing"
)

type testStruct struct {
	Param1 string `json:"param1"`
	Param2 string `json:"param2"`
}

func TestRequest(t *testing.T) {
	ctx := context.Background()
	method := "GET"
	id := "123"
	params := `{"param1": "value1", "param2": "value2"}`
	replyTo := "replyQueue"

	req := NewRequest(ctx, method, id, params, replyTo)

	if req.Context() != ctx {
		t.Errorf("Expected context %v, but got %v", ctx, req.Context())
	}

	if req.ID != id {
		t.Errorf("Expected ID %s, but got %s", id, req.ID)
	}

	var parsedParams testStruct
	if err := req.ParseParams(&parsedParams); err != nil {
		t.Errorf("Error parsing params: %v", err)
	}

	expectedParams := testStruct{
		Param1: "value1",
		Param2: "value2",
	}

	if parsedParams != expectedParams {
		t.Errorf("Expected parsed params %v, but got %v", expectedParams, parsedParams)
	}

	// Test Method()
	if req.Method != method {
		t.Errorf("Expected method %s, but got %s", method, req.Method)
	}

	// Test ReplyTo()
	if req.ReplyTo != replyTo {
		t.Errorf("Expected replyTo %s, but got %s", replyTo, req.ReplyTo)
	}
}
func TestWithContext(t *testing.T) {
	ctx := context.Background()
	method := "GET"
	id := "123"
	params := `{"param1": "value1", "param2": "value2"}`
	replyTo := "replyQueue"

	req := NewRequest(ctx, method, id, params, replyTo)

	newCtx := context.TODO()
	newReq := req.WithContext(newCtx)

	if newReq.Context() != newCtx {
		t.Errorf("Expected context %v, but got %v", newCtx, newReq.Context())
	}

	if newReq.ID != req.ID {
		t.Errorf("Expected ID %s, but got %s", req.ID, newReq.ID)
	}

	if newReq.Method != req.Method {
		t.Errorf("Expected method %s, but got %s", req.Method, newReq.Method)
	}

	if newReq.ReplyTo != req.ReplyTo {
		t.Errorf("Expected replyTo %s, but got %s", req.ReplyTo, newReq.ReplyTo)
	}
}
