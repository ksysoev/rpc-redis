package rpc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
)

type mockRedisClient struct {
	mock.Mock
}

func (m *mockRedisClient) sendRequest(ctx context.Context, stream string, msg any) error {
	args := m.Called(ctx, stream, msg)
	return args.Error(0)
}

func (m *mockRedisClient) subscribeOnResponses(ctx context.Context, channel string) (<-chan *redis.Message, Closer) {
	args := m.Called(ctx, channel)
	return args.Get(0).(chan *redis.Message), args.Get(1).(Closer)
}

type mockCloser struct {
	mock.Mock
}

func (c mockCloser) Close() error {
	args := c.Called()
	return args.Error(0)
}

func TestNewClient(t *testing.T) {
	redisClient, _ := redismock.NewClientMock()
	channel := "test-channel"

	client := NewClient(redisClient, channel)

	defer client.Close()

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

func TestHandleResponses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	respChan := make(chan *Response)

	client := &Client{
		ctx:     ctx,
		channel: "test-channel",
		requests: map[string]chan<- *Response{
			"test-id": respChan,
		},
		cancel: cancel,
		lock:   &sync.RWMutex{},
	}
	defer client.Close()

	subChan := make(chan *redis.Message)
	redisMock := &mockRedisClient{}
	closerMock := &mockCloser{}
	client.redis = redisMock
	redisMock.On("subscribeOnResponses", mock.Anything, mock.Anything).Return(subChan, closerMock)
	closerMock.On("Close").Return(nil)

	done := make(chan struct{})
	go func() {
		client.handleResponses()
		close(done)
	}()

	go func() {
		subChan <- &redis.Message{Payload: "{\"id\":\"test-id\",\"result\":\"{}\"}"}
		time.Sleep(time.Microsecond)
		client.Close()
	}()

	select {
	case resp := <-respChan:
		if resp.ID != "test-id" {
			t.Errorf("Expected response ID to match")
		}
	case <-time.After(time.Second):
		t.Errorf("Expected response to be received")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Errorf("Expected handleResponses to exit")
	}
}
