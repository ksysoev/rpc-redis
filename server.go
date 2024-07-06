package redisrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultBatchSize     = 1
	DefaultBlockInterval = 10 * time.Second
	DefaultConcurency    = 25
)

type Handler func(req Request) (any, error)

type token struct{}

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
	method := getField(msg, "method")
	if method == "" {
		return
	}

	handler, ok := s.getHandler(method)
	if !ok {
		return
	}

	id := getField(msg, "id")
	params := getField(msg, "params")
	deadline := getField(msg, "deadline")
	replyTo := getField(msg, "reply_to")

	s.sem <- token{}
	s.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error(fmt.Sprintf("RPC panic for %s: %v", method, r))
			}

			<-s.sem
			s.wg.Done()
		}()

		ctx := s.ctx
		if deadline != "" {
			epochTime, err := strconv.ParseInt(deadline, 10, 64)
			if err != nil {
				slog.Error(fmt.Sprintf("RPC invalid deadline for %s: %v", method, err))
				return
			}

			deadlineTime := time.Unix(epochTime, 0)
			ctx, _ = context.WithDeadline(ctx, deadlineTime)
		}

		resp, err := handler(NewRequest(s.ctx, method, id, params, replyTo))
		if err != nil {
			slog.Error(fmt.Sprintf("RPC unhandled error for %s: %v", method, err))
		}

		if replyTo == "" || resp == nil {
			return
		}

		jsonResp, err := json.Marshal(resp)
		if err != nil {
			slog.Error(fmt.Sprintf("RPC error marshalling response for %s: %v", method, err))
			return
		}

		if err := s.redis.Publish(ctx, replyTo, jsonResp).Err(); err != nil {
			slog.Error(fmt.Sprintf("RPC error publishing response for %s: %v", method, err))
		}
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

func (s *Server) getHandler(rpcName string) (Handler, bool) {
	s.handlersLock.RLock()
	defer s.handlersLock.RUnlock()

	handler, ok := s.handlers[rpcName]
	return handler, ok
}

func getField(msg redis.XMessage, field string) string {
	rawValue, ok := msg.Values[field]
	if !ok {
		return ""
	}

	val, ok := rawValue.(string)
	if !ok {
		return ""
	}

	return val
}
