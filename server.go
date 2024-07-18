package rpc

import (
	"context"
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
	ctx          context.Context
	redis        *redis.Client
	handlers     map[string]Handler
	handlersLock *sync.RWMutex
	cancel       context.CancelFunc
	sem          chan struct{}
	wg           *sync.WaitGroup
	stream       string
	group        string
	consumer     string
}

// NewServer creates a new instance of the Server struct.
// It takes a Redis client, stream name, consumer group name, and consumer name as parameters.
// It returns a pointer to the newly created Server instance.
func NewServer(redisClient *redis.Client, stream, group, consumer string) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		redis:        redisClient,
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

// Run starts the server and continuously reads messages from the Redis stream.
// It initializes the reader, sets up the read arguments, and enters an infinite loop
// to read messages from the stream. It processes each message by calling the
// `processMessage` method.
//
// If an error occurs during initialization or reading the stream, it returns
// the error. If the stream is empty, it continues to the next iteration.
//
// The `Run` method is responsible for running the server and handling the
// continuous message processing from the Redis stream.
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

	for s.ctx.Err() == nil {
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

	return nil
}

// initReader initializes the reader by creating a stream and a consumer group.
// It creates the stream if it doesn't exist and creates the consumer group if it doesn't exist.
// If the consumer group already exists, it returns an error.
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

// processMessage processes the incoming Redis XMessage.
// It extracts the method, id, params, deadline, and replyTo fields from the message,
// retrieves the appropriate handler for the method, and executes it in a separate goroutine.
// If a panic occurs during execution, it recovers and logs the error.
// If a deadline is specified, it sets a deadline for the execution context.
// After executing the handler, it marshals the result into JSON and creates a response.
// Finally, it publishes the response to the specified replyTo channel using Redis.
func (s *Server) processMessage(msg redis.XMessage) {
	method := getField(msg, "method")
	if method == "" {
		return
	}

	handler, ok := s.getHandler(method)
	if !ok {
		return
	}

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

		req, cancel, err := parseMessage(s.ctx, method, msg)

		if err != nil {
			slog.Error(fmt.Sprintf("RPC error parsing message for %s: %v", method, err))
			return
		}

		defer cancel()

		result, reqErr := handler(req)

		if err := s.handleResult(req, result, reqErr); err != nil {
			slog.Error(fmt.Sprintf("RPC error handling result for %s: %v", method, err))
		}
	}()
}

// Close stops the server gracefully by cancelling the context and waiting for all goroutines to finish.
func (s *Server) Close() {
	s.cancel()
	s.wg.Wait()
}

// AddHandler adds a new RPC handler to the server.
// It associates the given `handler` with the specified `rpcName`.
// If a handler already exists for the same `rpcName`, it panics.
func (s *Server) AddHandler(rpcName string, handler Handler) {
	s.handlersLock.Lock()
	defer s.handlersLock.Unlock()

	if _, ok := s.handlers[rpcName]; ok {
		panic("rpc handler already exists for " + rpcName)
	}

	s.handlers[rpcName] = handler
}

// getHandler returns the handler function associated with the given RPC name.
// It also returns a boolean value indicating whether the handler was found or not.
func (s *Server) getHandler(rpcName string) (Handler, bool) {
	s.handlersLock.RLock()
	defer s.handlersLock.RUnlock()

	handler, ok := s.handlers[rpcName]

	return handler, ok
}

// getField retrieves the value of a specified field from a redis.XMessage.
// If the field does not exist or the value is not a string, an empty string is returned.
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

// parseMessage parses the given Redis XMessage and returns a Request, context.CancelFunc, and error.
// It extracts the necessary fields from the message and creates a context with optional deadline.
func parseMessage(ctx context.Context, method string, msg redis.XMessage) (Request, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(ctx)

	if deadline := getField(msg, "deadline"); deadline != "" {
		epochTime, err := strconv.ParseInt(deadline, 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("error parsing deadline: %w", err)
		}

		deadlineTime := time.Unix(epochTime, 0)
		ctx, cancel = context.WithDeadline(ctx, deadlineTime)
	}

	if stash := getField(msg, "stash"); stash != "" {
		ctx = putStash(ctx, stash)
	}

	id := getField(msg, "id")
	params := getField(msg, "params")
	replyTo := getField(msg, "reply_to")

	return NewRequest(ctx, method, id, params, replyTo), cancel, nil
}

// handleResult handles the result of a request by creating a response, marshalling it to JSON,
// and publishing it to Redis.
// If the request does not require a reply, it returns nil.
// If there is an error creating the response, marshalling it to JSON, or publishing it to Redis,
// it returns an error with a descriptive message.
func (s *Server) handleResult(req Request, result any, reqErr error) error {
	if req.ReplyTo() == "" {
		return nil
	}

	resp, err := newResponse(req.ID(), result, reqErr)
	if err != nil {
		return fmt.Errorf("error creating response: %w", err)
	}

	respBytes, err := resp.ToJSON()
	if err != nil {
		return fmt.Errorf("error marshalling response: %w", err)
	}

	if err := s.redis.Publish(req.Context(), req.ReplyTo(), respBytes).Err(); err != nil {
		return fmt.Errorf("error publishing response: %w", err)
	}

	return nil
}
