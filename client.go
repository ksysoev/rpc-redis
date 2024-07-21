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

var ErrClientClosed = errors.New("client closed")

type Client struct {
	ctx          context.Context
	redis        *redis.Client
	cancel       context.CancelFunc
	wg           *sync.WaitGroup
	requests     map[string]chan<- *Response
	lock         *sync.Mutex
	counter      *atomic.Uint64
	once         *sync.Once
	handler      RequestHandler
	id           string
	channel      string
	interceptors []Interceptor
}

type ClientOption func(*Client)

// NewClient creates a new instance of the Client struct.
// It takes a Redis client and a channel name as parameters.
// It returns a pointer to the newly created Client.
func NewClient(redisClient *redis.Client, channel string, opts ...ClientOption) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		redis:    redisClient,
		id:       uuid.New().String(),
		ctx:      ctx,
		cancel:   cancel,
		channel:  channel,
		requests: make(map[string]chan<- *Response),
		wg:       &sync.WaitGroup{},
		lock:     &sync.Mutex{},
		counter:  &atomic.Uint64{},
		once:     &sync.Once{},
	}

	for _, opt := range opts {
		opt(client)
	}

	client.handler = useInterceptors(client.call, client.interceptors)

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
	if c.ctx.Err() != nil {
		return nil, ErrClientClosed
	}

	req, err := c.prepareRequest(ctx, method, params)
	if err != nil {
		return nil, err
	}

	// Ensure that the handleResponses function is only called once
	c.once.Do(c.handleResponses)

	return c.handler(req)
}

func useInterceptors(handler RequestHandler, interceptors []Interceptor) RequestHandler {
	for i := len(interceptors) - 1; i >= 0; i-- {
		handler = interceptors[i](handler)
	}

	return handler
}

// call sends the request to the server and waits for the response.
// It sends the request to the server by publishing the request message to the Redis stream.
// It then waits for the response on the response channel and returns the response or an error.
func (c *Client) call(req *Request) (*Response, error) {
	respChan := c.addRequest(req.ID)
	defer c.removeRequest(req.ID)

	msg := []string{
		"id", req.ID,
		"method", req.Method,
		"params", string(req.params),
		"reply_to", req.ReplyTo,
	}

	if deadline, ok := req.ctx.Deadline(); ok {
		msg = append(msg, "deadline", fmt.Sprintf("%d", deadline.Unix()))
	}

	if stash := getStash(req.ctx); stash != "" {
		msg = append(msg, "stash", stash)
	}

	err := c.redis.XAdd(req.ctx, &redis.XAddArgs{
		Stream: c.channel,
		Values: msg,
	}).Err()

	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	select {
	case <-req.ctx.Done():
		return nil, req.ctx.Err()
	case <-c.ctx.Done():
		return nil, ErrClientClosed
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
	pubsub := c.redis.Subscribe(c.ctx, c.id)
	pubsubChan := pubsub.Channel()

	go func() {
		defer pubsub.Close()

		for {
			select {
			case <-c.ctx.Done():
				return
			case msg, ok := <-pubsubChan:
				if !ok {
					c.cancel()
					return
				}

				c.processMessage(msg)
			}
		}
	}()
}

// processMessage processes the incoming Redis message and handles the response.
// It unmarshals the message payload into a Response struct and sends the response
// to the corresponding request channel.
func (c *Client) processMessage(msg *redis.Message) {
	resp := &Response{}

	err := json.Unmarshal([]byte(msg.Payload), resp)
	if err != nil {
		slog.Error("Error unmarshalling response: " + err.Error())
		return
	}

	c.lock.Lock()
	respChan, ok := c.requests[resp.ID]
	c.lock.Unlock()

	if !ok {
		return
	}

	respChan <- resp
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

// prepareRequest prepares a request to be sent to the RPC server.
// It takes the method name and parameters, marshals the parameters into JSON,
// and returns a Request object with the necessary information.
// If the method name is empty or there is an error marshalling the parameters,
// it returns an error.
func (c *Client) prepareRequest(ctx context.Context, method string, params any) (*Request, error) {
	if method == "" {
		return nil, fmt.Errorf("method cannot be empty")
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("error marshalling params: %w", err)
	}

	return &Request{
		ID:      fmt.Sprintf("%d", c.counter.Add(1)),
		Method:  method,
		params:  paramsBytes,
		ReplyTo: c.id,
		ctx:     ctx,
	}, nil
}

// WithInterceptors adds the provided interceptors to the client.
func WithInterceptors(interceptors ...Interceptor) ClientOption {
	return func(c *Client) {
		c.interceptors = interceptors
	}
}
