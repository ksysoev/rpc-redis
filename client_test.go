package rpc

import (
	"context"
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

	clientMock.ExpectXAdd(&redis.XAddArgs{
		Stream: "test-channel",
		Values: []string{
			"id", id,
			"method", method,
			"params", fmt.Sprintf("%q", params),
			"reply_to", client.id,
		},
	}).SetVal("OK")

	done := make(chan struct{})
	go func() {
		_, err := client.Call(context.Background(), method, params)

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

	clientMock.ExpectXAdd(&redis.XAddArgs{
		Stream: "test-channel",
		Values: []string{
			"id", id,
			"method", method,
			"params", fmt.Sprintf("%q", params),
			"reply_to", client.id,
		},
	}).SetVal("OK")

	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()

		_, err := client.Call(ctx, method, params)

		if err != context.DeadlineExceeded {
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
