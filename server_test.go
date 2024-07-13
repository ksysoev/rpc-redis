package rpc

import (
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
	err := server.initReader()

	if err != nil {
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