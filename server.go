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
	DefaultBatchSize     = 1
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
	wg           *sync.WaitGroup
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
		wg:           &sync.WaitGroup{},
	}
}

func (s *Server) Run() error {
	err := s.initReader()
	if err != nil {
		return err
	}

	readArgs := &redis.XReadGroupArgs{
		Group:    s.group,
		Consumer: s.consumer,
		Streams:  []string{s.stream, ">"},
		Block:    DefaultBlockInterval,
		Count:    DefaultBatchSize,
		NoAck:    false,
	}

	for {
		streams, err := s.redis.XReadGroup(s.ctx, readArgs).Result()

		switch {
		case err == redis.Nil:
			continue
		case err != nil:
			return fmt.Errorf("error reading stream: %w", err)
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				s.processMessage(message)
			}
		}
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

func (s *Server) processMessage(msg redis.XMessage) {
	rpcName, ok := msg.Values["method"].(string)
	if !ok {
		return
	}

	// get the rpc handler
	s.handlersLock.RLock()
	handler, ok := s.handlers[rpcName]
	s.handlersLock.RUnlock()
	if !ok {
		return
	}

	s.sem <- struct{}{}
	s.wg.Add(1)
	go func() {
		_, err := handler(s.ctx, NewRequest(s.ctx, msg.ID, msg.Values["params"].(string)))
		if err != nil {
			slog.Error(fmt.Sprintf("RPC unhandled error for %s: %v", rpcName, err))
		}
		<-s.sem
		s.wg.Done()
	}()
}

func (s *Server) Close() {
	s.cancel()
	s.wg.Wait()
}

func (s *Server) AddHandler(rpcName string, handler Handler) {
	s.handlersLock.Lock()
	defer s.handlersLock.Unlock()

	if _, ok := s.handlers[rpcName]; ok {
		panic("rpc handler already exists for " + rpcName)
	}

	s.handlers[rpcName] = handler
}
