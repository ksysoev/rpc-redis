package rpc

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
)

func TestNewServer(t *testing.T) {
	redisClient, _ := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	server := NewServer(redisClient, stream, group, consumer)

	if server.redis != redisClient {
		t.Errorf("Expected redis client to be set")
	}

	if server.stream != stream {
		t.Errorf("Expected stream to be set")
	}

	if server.group != group {
		t.Errorf("Expected group to be set")
	}

	if server.consumer != consumer {
		t.Errorf("Expected consumer to be set")
	}
}
func TestServer_InitReader(t *testing.T) {
	redisClient, mock := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	server := NewServer(redisClient, stream, group, consumer)

	// Expect XGroupCreateMkStream to be called with the correct arguments
	mock.ExpectXGroupCreateMkStream(stream, group, "$").SetVal("OK")

	// Expect XGroupCreateConsumer to be called with the correct arguments
	mock.ExpectXGroupCreateConsumer(stream, group, consumer).SetVal(1)

	if err := server.initReader(); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestServer_InitReader_CreateStreamError(t *testing.T) {
	redisClient, mock := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	server := NewServer(redisClient, stream, group, consumer)

	// Test creating stream error
	mock.ExpectXGroupCreateMkStream(stream, group, "$").SetErr(fmt.Errorf("failed to create stream"))

	err := server.initReader()
	if err == nil {
		t.Error("Expected error, but got nil")
	}

	// Verify that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestServer_InitReader_CreateConsumerError(t *testing.T) {
	redisClient, mock := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	server := NewServer(redisClient, stream, group, consumer)

	// Test creating consumer error
	mock.ExpectXGroupCreateMkStream(stream, group, "$").SetVal("OK")
	mock.ExpectXGroupCreateConsumer(stream, group, consumer).SetErr(fmt.Errorf("failed to create consumer"))

	err := server.initReader()
	if err == nil {
		t.Error("Expected error, but got nil")
	}

	// Verify that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestServer_InitReader_ConsumerGroupExistsError(t *testing.T) {
	redisClient, mock := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	server := NewServer(redisClient, stream, group, consumer)

	// Expect XGroupCreateMkStream to be called with the correct arguments
	mock.ExpectXGroupCreateMkStream(stream, group, "$").SetVal("OK")

	// Expect XGroupCreateConsumer to return an error indicating that the consumer group already exists
	mock.ExpectXGroupCreateConsumer(stream, group, consumer).SetErr(fmt.Errorf("BUSYGROUP Consumer Group name already exists"))

	err := server.initReader()
	if err == nil {
		t.Error("Expected error, but got nil")
	}

	// Verify that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}
func TestServer_Run(t *testing.T) {
	redisClient, mock := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	server := NewServer(redisClient, stream, group, consumer)

	xReadArgs := &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Block:    DefaultBlockInterval,
		Count:    DefaultBatchSize,
		NoAck:    false,
	}

	expectedErr := fmt.Errorf("failed to read stream")

	mock.ExpectXGroupCreateMkStream(stream, group, "$").SetVal("OK")
	mock.ExpectXGroupCreateConsumer(stream, group, consumer).SetVal(1)
	mock.ExpectXReadGroup(xReadArgs).SetVal([]redis.XStream{{Messages: []redis.XMessage{{}, {}}}})
	mock.ExpectXReadGroup(xReadArgs).SetErr(redis.Nil)
	mock.ExpectXReadGroup(xReadArgs).SetErr(expectedErr)

	err := server.Run()
	if err != nil && !errors.Is(err, expectedErr) {
		t.Errorf("Unexpected error: %v", err)
	}
}
func TestServer_AddHandler(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{})
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	server := NewServer(redisClient, stream, group, consumer)

	rpcName := "myRPC"
	handler := func(req Request) (any, error) {
		return nil, nil
	}

	server.AddHandler(rpcName, handler)

	// Verify that the handler was added successfully
	if _, ok := server.getHandler(rpcName); !ok {
		t.Errorf("Expected handler to be added for RPC: %s", rpcName)
	}

	// Verify that adding the same handler again panics
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic when adding duplicate handler for RPC: %s", rpcName)
		}
	}()

	server.AddHandler(rpcName, handler)
}
func TestGetField_ExistingField(t *testing.T) {
	msg := redis.XMessage{
		Values: map[string]interface{}{
			"field1": "value1",
			"field2": "value2",
		},
	}

	field := "field1"
	expected := "value1"
	actual := getField(msg, field)

	if actual != expected {
		t.Errorf("Expected field value %s, but got %s", expected, actual)
	}
}

func TestGetField_NonExistingField(t *testing.T) {
	msg := redis.XMessage{
		Values: map[string]interface{}{
			"field1": "value1",
			"field2": "value2",
		},
	}

	field := "field3"
	expected := ""
	actual := getField(msg, field)

	if actual != expected {
		t.Errorf("Expected empty field value, but got %s", actual)
	}
}

func TestGetField_NonStringFieldValue(t *testing.T) {
	msg := redis.XMessage{
		Values: map[string]interface{}{
			"field1": 123,
			"field2": true,
		},
	}

	field := "field1"
	expected := ""
	actual := getField(msg, field)

	if actual != expected {
		t.Errorf("Expected empty field value, but got %s", actual)
	}
}
func TestServer_Close(t *testing.T) {
	redisClient, mock := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	xReadArgs := &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Block:    DefaultBlockInterval,
		Count:    DefaultBatchSize,
		NoAck:    false,
	}

	mock.ExpectXGroupCreateMkStream(stream, group, "$").SetVal("OK")
	mock.ExpectXGroupCreateConsumer(stream, group, consumer).SetVal(1)
	mock.ExpectXReadGroup(xReadArgs).SetErr(redis.Nil)

	server := NewServer(redisClient, stream, group, consumer)

	done := make(chan struct{})
	go func() {
		err := server.Run()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		close(done)
	}()

	// Close the server
	server.Close()

	// Verify that the server has stopped
	select {
	case <-done:
		// The context is cancelled, which means the server has stopped
	case <-time.After(1 * time.Second):
		t.Error("Server did not stop within the expected time")
	}
}
func TestParseMessage_ValidMessage(t *testing.T) {
	ctx := context.Background()

	msg := redis.XMessage{
		Values: map[string]interface{}{
			"method":   "add",
			"id":       "123",
			"params":   `{"a": 1, "b": 2}`,
			"deadline": "1672531200",
			"reply_to": "response-channel",
		},
	}

	req, cancel, err := parseMessage(ctx, "add", msg)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if req.Method() != "add" {
		t.Errorf("Expected method 'add', but got '%s'", req.Method())
	}

	if req.ID() != "123" {
		t.Errorf("Expected ID '123', but got '%s'", req.ID())
	}

	if req.ReplyTo() != "response-channel" {
		t.Errorf("Expected replyTo 'response-channel', but got '%s'", req.ReplyTo())
	}

	deadline, ok := req.Context().Deadline()
	if !ok {
		t.Error("Expected deadline to be set")
	}

	expectedDeadline := time.Unix(1672531200, 0)
	if deadline != expectedDeadline {
		t.Errorf("Expected deadline '%s', but got '%s'", expectedDeadline, deadline)
	}

	cancel()
}

func TestParseMessage_InvalidDeadline(t *testing.T) {
	ctx := context.Background()
	msg := redis.XMessage{
		Values: map[string]interface{}{
			"method":   "subtract",
			"id":       "456",
			"params":   `{"a": 3, "b": 4}`,
			"deadline": "invalid",
			"reply_to": "response-channel",
		},
	}

	_, _, err := parseMessage(ctx, "subtract", msg)

	if err == nil {
		t.Error("Expected error, but got nil")
	}
}

func TestParseMessage_NoDeadline(t *testing.T) {
	ctx := context.Background()
	msg := redis.XMessage{
		Values: map[string]interface{}{
			"method":   "multiply",
			"id":       "789",
			"params":   `{"a": 5, "b": 6}`,
			"reply_to": "response-channel",
		},
	}

	req, cancel, err := parseMessage(ctx, "multiply", msg)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	_, ok := req.Context().Deadline()
	if ok {
		t.Error("Expected no deadline to be set")
	}

	cancel()
}
func TestServer_HandleResult_NoReplyTo(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{})
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	server := NewServer(redisClient, stream, group, consumer)

	req := NewRequest(context.Background(), "method", "123", "params", "")
	result := "result"
	reqErr := errors.New("request error")

	err := server.handleResult(req, result, reqErr)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestServer_HandleResult_CreateResponseError(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{})
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	server := NewServer(redisClient, stream, group, consumer)

	req := NewRequest(context.Background(), "method", "123", "params", "reply-channel")
	result := "result"
	reqErr := errors.New("request error")

	err := server.handleResult(req, result, reqErr)
	if err == nil {
		t.Error("Expected error, but got nil")
	}
}

func TestServer_HandleResult_MarshalResponseError(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{})
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	server := NewServer(redisClient, stream, group, consumer)

	req := NewRequest(context.Background(), "method", "123", "params", "reply-channel")
	result := make(chan int) // Invalid type for marshalling
	reqErr := errors.New("request error")

	err := server.handleResult(req, result, reqErr)
	if err == nil {
		t.Error("Expected error, but got nil")
	}
}

func TestServer_HandleResult_PublishResponseError(t *testing.T) {
	redisClient, mock := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	server := NewServer(redisClient, stream, group, consumer)

	req := NewRequest(context.Background(), "method", "123", "params", "reply-channel")
	reqErr := errors.New("request error")

	expecetdResp, _ := newResponse(req.ID(), nil, reqErr)
	expectedJSON, _ := expecetdResp.ToJSON()

	expectedError := fmt.Errorf("failed to publish response")

	mock.ExpectPublish("reply-channel", expectedJSON).SetErr(expectedError)

	err := server.handleResult(req, nil, reqErr)
	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error: %v, but got: %v", expectedError, err)
	}
}

func TestServer_HandleResult_Success(t *testing.T) {
	redisClient, mock := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"

	server := NewServer(redisClient, stream, group, consumer)

	req := NewRequest(context.Background(), "method", "123", "params", "reply-channel")
	result := "result"

	expecetdResp, _ := newResponse(req.ID(), result, nil)
	expectedJSON, _ := expecetdResp.ToJSON()

	mock.ExpectPublish("reply-channel", expectedJSON).SetVal(1)

	err := server.handleResult(req, result, nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestServer_ProcessMessage_ValidMessageWithExistingHandler(t *testing.T) {
	redisClient, mock := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"
	server := NewServer(redisClient, stream, group, consumer)

	mock.ExpectPublish("response-channel", []byte(`{"id":"123","result":"test data"}`)).SetVal(1)

	msg := redis.XMessage{
		Values: map[string]interface{}{
			"method":   "add",
			"id":       "123",
			"params":   `{"a": 1, "b": 2}`,
			"reply_to": "response-channel",
			"stash":    "\"test stash\"",
		},
	}

	isCalled := false
	handler := func(req Request) (any, error) {
		isCalled = true
		return "test data", nil
	}
	server.AddHandler("add", handler)

	server.processMessage(msg)

	server.Close()

	if !isCalled {
		t.Error("Expected handler to be called")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestServer_ProcessMessage_ValidMessageWithNonExistingHandler(t *testing.T) {
	redisClient, mock := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"
	server := NewServer(redisClient, stream, group, consumer)

	msg := redis.XMessage{
		Values: map[string]interface{}{
			"method":   "subtract",
			"id":       "456",
			"params":   `{"a": 3, "b": 4}`,
			"reply_to": "response-channel",
		},
	}

	server.processMessage(msg)

	server.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestServer_ProcessMessage_InvalidMessageWithExistingHandler(t *testing.T) {
	redisClient, mock := redismock.NewClientMock()
	stream := "mytream"
	group := "myGroup"
	consumer := "myConsumer"
	server := NewServer(redisClient, stream, group, consumer)

	msg := redis.XMessage{
		Values: map[string]interface{}{
			"method":   "multiply",
			"id":       "789",
			"params":   `{"a": 5, "b": 6}`,
			"deadline": "invalid",
			"reply_to": "response-channel",
		},
	}

	isCalled := false
	handler := func(req Request) (any, error) {
		isCalled = true
		return nil, nil
	}
	server.AddHandler("multiply", handler)

	server.processMessage(msg)

	server.Close()

	if isCalled {
		t.Error("Expected handler not to be called")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestServer_ProcessMessage_InvalidMessageWithNonExistingHandler(t *testing.T) {
	redisClient, mock := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"
	server := NewServer(redisClient, stream, group, consumer)

	msg := redis.XMessage{
		Values: map[string]interface{}{
			"method":   "divide",
			"id":       "987",
			"params":   `{"a": 8, "b": 9}`,
			"deadline": "invalid",
			"reply_to": "response-channel",
		},
	}

	server.processMessage(msg)

	server.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestServer_ProcessMessage_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Error("Expected no panic")
		}
	}()

	redisClient, mock := redismock.NewClientMock()
	stream := "myStream"
	group := "myGroup"
	consumer := "myConsumer"
	server := NewServer(redisClient, stream, group, consumer)

	msg := redis.XMessage{
		Values: map[string]interface{}{
			"method":   "panic",
			"id":       "987",
			"params":   `{"a": 8, "b": 9}`,
			"deadline": "1672531200",
			"reply_to": "response-channel",
		},
	}

	handler := func(req Request) (any, error) {
		panic("test panic")
	}
	server.AddHandler("panic", handler)

	server.processMessage(msg)

	server.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}
