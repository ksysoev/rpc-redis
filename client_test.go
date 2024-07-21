package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
)

func TestNewClient(t *testing.T) {
	redisClient, _ := redismock.NewClientMock()
	channel := "test-channel"

	client := NewClient(redisClient, channel)

	defer client.Close()

	if client.redis != redisClient {
		t.Errorf("Expected redis client to be set")
	}

	if client.channel != channel {
		t.Errorf("Expected channel to be set")
	}
}
func TestAddRequest(t *testing.T) {
	redisClient, _ := redismock.NewClientMock()
	client := NewClient(redisClient, "test-channel")

	defer client.Close()

	id := "test-id"
	_ = client.addRequest(id)

	// Verify that the response channel is added to the requests map
	if _, ok := client.requests[id]; !ok {
		t.Errorf("Expected response channel to be added to the requests map")
	}
}
func TestRemoveRequest(t *testing.T) {
	redisClient, _ := redismock.NewClientMock()
	client := NewClient(redisClient, "test-channel")

	defer client.Close()

	id := "test-id"
	client.addRequest(id)

	client.removeRequest(id)

	// Verify that the request is removed from the requests map
	if _, ok := client.requests[id]; ok {
		t.Errorf("Expected request to be removed from the requests map")
	}
}

func TestCall_ClientClosed(t *testing.T) {
	redisClient, clientMock := redismock.NewClientMock()
	client := NewClient(redisClient, "test-channel")

	id := "1"
	method := "test-method"
	params := "test-params"
	ctx, _ := SetStash(context.Background(), "test-stash")

	clientMock.ExpectXAdd(&redis.XAddArgs{
		Stream: "test-channel",
		Values: []string{
			"id", id,
			"method", method,
			"params", fmt.Sprintf("%q", params),
			"reply_to", client.id,
			"stash", fmt.Sprintf("%q", "test-stash"),
		},
	}).SetVal("OK")

	done := make(chan struct{})
	go func() {
		_, err := client.Call(ctx, method, params)

		if err != ErrClientClosed {
			t.Errorf("Expected error to be ErrClientClosed but got %v", err)
		}

		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	client.Close()

	<-done
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Errorf("Expected call to return")
	}
}

func TestCall_Timeout(t *testing.T) {
	redisClient, clientMock := redismock.NewClientMock()
	client := NewClient(redisClient, "test-channel")

	defer client.Close()

	id := "1"
	method := "test-method"
	params := "test-params"

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	clientMock.ExpectXAdd(&redis.XAddArgs{
		Stream: "test-channel",
		Values: []string{
			"id", id,
			"method", method,
			"params", fmt.Sprintf("%q", params),
			"reply_to", client.id,
			"deadline", fmt.Sprintf("%d", time.Now().Add(time.Millisecond).Unix()),
		},
	}).SetVal("OK")

	done := make(chan struct{})
	go func() {
		if _, err := client.Call(ctx, method, params); err != context.DeadlineExceeded {
			t.Errorf("Expected error to be ErrClientClosed but got %v", err)
		}

		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Errorf("Expected call to return")
	}
}
func TestProcessMessage(t *testing.T) {
	client := NewClient(nil, "test-channel")
	defer client.Close()

	respChan := make(chan *Response)
	client.requests["test-id"] = respChan

	msg := &redis.Message{
		Payload: `{"ID": "test-id", "Result": "test-result"}`,
	}

	go func() {
		client.processMessage(msg)
	}()

	select {
	case resp := <-respChan:
		if resp.ID != "test-id" {
			t.Errorf("Expected response ID to be 'test-id', but got '%s'", resp.ID)
		}

		var res string
		if err := resp.ParseResut(&res); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if res != "test-result" {
			t.Errorf("Expected response Result to be 'test-result', but got '%s'", resp.Result)
		}
	case <-time.After(time.Second):
		t.Errorf("Expected response to be received on the channel")
	}
}

func TestProcessMessage_InvalidPayload(t *testing.T) {
	client := NewClient(nil, "test-channel")
	defer client.Close()

	respChan := make(chan *Response)
	client.requests["test-id"] = respChan

	msg := &redis.Message{
		Payload: "invalid-json",
	}

	go func() {
		client.processMessage(msg)
	}()

	select {
	case <-respChan:
		t.Errorf("Expected no response to be received on the channel")
	case <-time.After(10 * time.Millisecond): // No response should be received
	}
}

func TestProcessMessage_NoRequest(_ *testing.T) {
	client := NewClient(nil, "test-channel")
	defer client.Close()

	msg := &redis.Message{
		Payload: `{"ID": "test-id", "Result": "test-result"}`,
	}

	client.processMessage(msg)
}
func TestWithInterceptors(t *testing.T) {
	interceptor1 := func(handler RequestHandler) RequestHandler {
		return func(req *Request) (*Response, error) {
			// Interceptor 1 logic
			return handler(req)
		}
	}

	interceptor2 := func(handler RequestHandler) RequestHandler {
		return func(req *Request) (*Response, error) {
			// Interceptor 2 logic
			return handler(req)
		}
	}

	client := NewClient(nil, "test-channel", WithInterceptors(interceptor1, interceptor2))

	// Assert that the interceptors are correctly added to the client
	if len(client.interceptors) != 2 {
		t.Errorf("Expected 2 interceptors, but got %d", len(client.interceptors))
	}
}

func TestPrepareRequest(t *testing.T) {
	c := NewClient(nil, "test-channel")

	ctx := context.Background()
	method := "test-method"
	params := struct {
		Name string
		Age  int
	}{
		Name: "John",
		Age:  30,
	}

	req, err := c.prepareRequest(ctx, method, params)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if req.ID == "" {
		t.Errorf("Expected request ID to be non-empty")
	}

	if req.Method != method {
		t.Errorf("Expected request Method to be '%s', but got '%s'", method, req.Method)
	}

	expectedParams, _ := json.Marshal(params)
	if !bytes.Equal(req.params, expectedParams) {
		t.Errorf("Expected request params to be '%s', but got '%s'", expectedParams, req.params)
	}

	if req.ReplyTo != c.id {
		t.Errorf("Expected request ReplyTo to be '%s', but got '%s'", c.id, req.ReplyTo)
	}

	if req.ctx != ctx {
		t.Errorf("Expected request ctx to be the same as the provided context")
	}
}

func TestPrepareRequest_EmptyMethod(t *testing.T) {
	c := NewClient(nil, "test-channel")

	ctx := context.Background()
	params := struct {
		Name string
		Age  int
	}{
		Name: "John",
		Age:  30,
	}

	_, err := c.prepareRequest(ctx, "", params)

	if err == nil {
		t.Errorf("Expected error due to empty method")
	}
}

func TestPrepareRequest_MarshalError(t *testing.T) {
	c := NewClient(nil, "test-channel")

	ctx := context.Background()
	method := "test-method"
	params := make(chan int) // Invalid type for JSON marshaling

	_, err := c.prepareRequest(ctx, method, params)

	if err == nil {
		t.Errorf("Expected error due to JSON marshaling error")
	}
}
