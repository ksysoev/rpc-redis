package rpc

import (
	"testing"

	"github.com/go-redis/redismock/v9"
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
