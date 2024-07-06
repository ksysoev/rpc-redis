package redisrpc

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultBlockInterval = 1000
)

type Handler func(ctx context.Context, Request interface{}) (any, error)

type Request struct{}

type Server struct {
	stream       string
	group        string
	consumer     string
	redis        *redis.Client
	handlers     map[string]Handler
	handlersLock *sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewServer(redis *redis.Client, stream, group, consumer string) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		redis:        redis,
		stream:       stream,
		group:        group,
		handlers:     make(map[string]Handler),
		handlersLock: &sync.RWMutex{},
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (s *Server) Run() error {
	for {
		// get the next message
		msg, err := s.redis.XReadGroup(s.ctx, &redis.XReadGroupArgs{
			Group:    s.group,
			Consumer: s.consumer,
			Streams:  []string{s.stream, ">"},
			Block:    DefaultBlockInterval,
			Count:    1,
			NoAck:    false,
		}).Result()

		if err != nil {
			return err
		}

		if len(msg) == 0 {
			continue
		}

		// get the message
		messages := msg[0].Messages
		if len(messages) == 0 {
			continue
		}

		// get the message
		message := messages[0]

		// get the rpc name
		rpcName, ok := message.Values["rpcName"].(string)
		if !ok {
			continue
		}

		// get the rpc handler
		s.handlersLock.RLock()
		handler, ok := s.handlers[rpcName]
		s.handlersLock.RUnlock()
		if !ok {
			continue
		}

		go func() {
			_, err := handler(s.ctx, Request{})
			if err != nil {
				slog.Error(fmt.Sprintf("RPC unhandled error for %s: %v", rpcName, err))
			}
		}()
	}
}

func (s *Server) Close() {
	s.cancel()
}

func (s *Server) AddHandler(rpcName string, handler Handler) {
	s.handlersLock.Lock()
	defer s.handlersLock.Unlock()

	if _, ok := s.handlers[rpcName]; ok {
		panic("rpc handler already exists for " + rpcName)
	}

	s.handlers[rpcName] = handler
}
