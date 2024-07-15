package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type redisClient interface {
	sendRequest(ctx context.Context, stream string, msg any) error
	subscribeOnResponses(ctx context.Context, channel string) (<-chan *redis.Message, Closer)
}

type Client struct {
	ctx      context.Context
	redis    redisClient
	cancel   context.CancelFunc
	wg       *sync.WaitGroup
	requests map[string]chan<- *Response
	lock     *sync.RWMutex
	counter  *atomic.Uint64
	id       string
	channel  string
}

// NewClient creates a new instance of the Client struct.
// It takes a Redis client and a channel name as parameters.
// It returns a pointer to the newly created Client.
func NewClient(redisClient *redis.Client, channel string) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	redisWrap := newRedisWrapper(redisClient)

	client := &Client{
		redis:    redisWrap,
		id:       uuid.New().String(),
		ctx:      ctx,
		cancel:   cancel,
		channel:  channel,
		requests: make(map[string]chan<- *Response),
		wg:       &sync.WaitGroup{},
		lock:     &sync.RWMutex{},
		counter:  &atomic.Uint64{},
	}

	go client.handleResponses()

	return client
}

// Call sends a request to the server using the specified method and parameters.
// It returns the response received from the server or an error if the request fails.
// The context `ctx` is used for cancellation and timeout.
// The `method` parameter specifies the method to be called on the server.
// The `params` parameter contains the parameters to be passed to the server method.
// The returned response is of type `*Response` and contains the result of the server method call.
// If an error occurs during the request or if the server returns an error response,
// an error is returned with a descriptive error message.
func (c *Client) Call(ctx context.Context, method string, params any) (*Response, error) {
	if method == "" {
		return nil, fmt.Errorf("method cannot be empty")
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("error marshalling params: %w", err)
	}

	id := fmt.Sprintf("%d", c.counter.Add(1))

	respChan := c.addRequest(id)
	defer c.removeRequest(id)

	msg := map[string]interface{}{
		"id":       id,
		"method":   method,
		"params":   paramsBytes,
		"reply_to": c.id,
	}

	err = c.redis.sendRequest(ctx, c.channel, msg)

	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respChan:
		if resp.Error != "" {
			return nil, errors.New(resp.Error)
		}

		return resp, nil
	}
}

// Close cancels any pending requests and closes the client connection.
func (c *Client) Close() {
	c.cancel()
}

// handleResponses listens for responses from the Redis pub/sub channel and handles them accordingly.
// It subscribes to the Redis pub/sub channel using the provided context and client ID.
// It continuously receives messages from the channel and unmarshals them into a Response struct.
// If unmarshaling fails, it logs an error and continues to the next message.
// It then looks up the corresponding response channel in the client's request map and sends the response on that channel.
// If no response channel is found, it continues to the next message.
// The function runs until the context is canceled or an error occurs.
func (c *Client) handleResponses() {
	ch, sub := c.redis.subscribeOnResponses(c.ctx, c.id)
	defer sub.Close()

	for {
		select {
		case <-c.ctx.Done():
			return
		case msg := <-ch:
			resp := &Response{}

			err := json.Unmarshal([]byte(msg.Payload), resp)
			if err != nil {
				slog.Error("Error unmarshalling response: " + err.Error())
				continue
			}

			c.lock.RLock()
			respChan, ok := c.requests[resp.ID]
			c.lock.RUnlock()

			if !ok {
				continue
			}

			respChan <- resp
		}
	}
}

// addRequest adds a new request to the client's request map and returns a channel to receive the response.
// The response channel will be used by the caller to receive the response for the corresponding request ID.
// The caller should wait for the response on the returned channel.
func (c *Client) addRequest(id string) <-chan *Response {
	c.lock.Lock()
	defer c.lock.Unlock()

	respChan := make(chan *Response)

	c.requests[id] = respChan

	return respChan
}

// removeRequest removes a request from the client's request map.
func (c *Client) removeRequest(id string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	delete(c.requests, id)
}
