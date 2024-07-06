package redisrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	redis   *redis.Client
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	wg      *sync.WaitGroup
	channel string
}

func NewClient(redis *redis.Client, channel string) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		redis:   redis,
		id:      uuid.New().String(),
		ctx:     ctx,
		cancel:  cancel,
		channel: channel,
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

	return "", nil
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
			fmt.Println(msg.Payload)
		}
	}
}
