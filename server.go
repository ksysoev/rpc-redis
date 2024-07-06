package redisrpc

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultBlockInterval = time.Second
	DefaultConcurency    = 25
)

type Handler func(ctx context.Context, req Request) (any, error)

type Server struct {
	stream       string
	group        string
	consumer     string
	redis        *redis.Client
	handlers     map[string]Handler
	handlersLock *sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	sem          chan struct{}
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
		consumer:     consumer,
		sem:          make(chan struct{}, DefaultConcurency),
	}
}

func (s *Server) Run() error {
	err := s.initReader()
	if err != nil {
		return err
	}

	for {
		s.sem <- struct{}{}

		msg, err := s.redis.XReadGroup(s.ctx, &redis.XReadGroupArgs{
			Group:    s.group,
			Consumer: s.consumer,
			Streams:  []string{s.stream, ">"},
			Block:    DefaultBlockInterval,
			Count:    1,
			NoAck:    false,
		}).Result()

		switch {
		case err == redis.Nil:
			continue
		case err != nil:
			return fmt.Errorf("error reading stream: %w", err)
		case len(msg) == 0:
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

			<-s.sem
		}()
	}
}

func (s *Server) initReader() error {
	// create the stream
	err := s.redis.XGroupCreateMkStream(s.ctx, s.stream, s.group, "$").Err()
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP Consumer Group name already exists") {
		return fmt.Errorf("error creating stream: %w", err)
	}

	// create the consumer
	if err := s.redis.XGroupCreateConsumer(s.ctx, s.stream, s.group, s.consumer).Err(); err != nil {
		return fmt.Errorf("error creating consumer: %w", err)
	}

	return nil
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
