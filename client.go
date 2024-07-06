package redisrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Response string

type rawResponse struct {
	ID     string `json:"id"`
	Result string `json:"result"`
	Error  string `json:"error"`
}

type Client struct {
	redis    *redis.Client
	id       string
	ctx      context.Context
	cancel   context.CancelFunc
	wg       *sync.WaitGroup
	channel  string
	requests map[string]chan<- Response
	lock     *sync.RWMutex
}

func NewClient(redis *redis.Client, channel string) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		redis:    redis,
		id:       uuid.New().String(),
		ctx:      ctx,
		cancel:   cancel,
		channel:  channel,
		requests: make(map[string]chan<- Response),
		wg:       &sync.WaitGroup{},
		lock:     &sync.RWMutex{},
	}

	go client.handleResponses()

	return client
}

func (c *Client) Call(ctx context.Context, method string, params any) (string, error) {
	if method == "" {
		return "", fmt.Errorf("method cannot be empty")
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("error marshalling params: %w", err)
	}

	respChan := c.addRequest(c.id)
	defer c.removeRequest(c.id)

	msg := map[string]interface{}{
		"method":   method,
		"params":   paramsBytes,
		"reply_to": c.id,
	}

	err = c.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: c.channel,
		Values: msg,
	}).Err()

	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case resp := <-respChan:
		return string(resp), nil
	}
}

func (c *Client) Close() {
	c.cancel()
}

func (c *Client) handleResponses() {
	pubsub := c.redis.Subscribe(c.ctx, c.id)
	defer pubsub.Close()

	pubsubChan := pubsub.Channel()

	for {
		select {
		case <-c.ctx.Done():
			return
		case msg := <-pubsubChan:
			var rawResp rawResponse

			err := json.Unmarshal([]byte(msg.Payload), &rawResp)
			if err != nil {
				slog.Error("Error unmarshalling response: " + err.Error())
				continue
			}

			c.lock.RLock()
			respChan, ok := c.requests[rawResp.ID]
			c.lock.RUnlock()

			if !ok {
				continue
			}

			respChan <- Response(rawResp.Result)
		}
	}
}

func (c *Client) addRequest(id string) <-chan Response {
	c.lock.Lock()
	defer c.lock.Unlock()

	respChan := make(chan Response)

	c.requests[id] = respChan

	return respChan
}

func (c *Client) removeRequest(id string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	delete(c.requests, id)
}
