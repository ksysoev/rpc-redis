package rpc

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Closer interface {
	Close() error
}

type redisWrap struct {
	redisClient *redis.Client
}

// NewRedisWrapper creates a new instance of the RedisWrapper struct.
// It takes a Redis client as a parameter and returns a pointer to the newly created RedisWrapper.
func newRedisWrapper(redisClient *redis.Client) *redisWrap {
	return &redisWrap{redisClient: redisClient}
}

func (r *redisWrap) sendRequest(ctx context.Context, stream string, msg any) error {
	return r.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: msg,
	}).Err()
}

func (r *redisWrap) subscribeOnResponses(ctx context.Context, channel string) (<-chan *redis.Message, Closer) {
	sub := r.redisClient.Subscribe(ctx, channel)
	return sub.Channel(), sub
}
