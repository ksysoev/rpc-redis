package rpc

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type TestEchoRequest struct {
	Value string `json:"value"`
}

func BenchmarkPrimeNumbers(b *testing.B) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	err := redisClient.Ping(context.Background()).Err()
	if err != nil {
		b.Fatalf("Error connecting to Redis: %v", err)
	}

	defer redisClient.Close()

	rpcClient := NewClient(redisClient, "echo.EchoService")
	defer rpcClient.Close()

	rpcServer := NewServer(redisClient, "echo.EchoService", "echo-group", "echo-consumer")

	rpcServer.AddHandler("Echo", func(req *Request) (any, error) {
		var echoReq TestEchoRequest

		err := req.ParseParams(&echoReq)
		if err != nil {
			return nil, fmt.Errorf("error parsing request: %v", err)
		}

		return &echoReq, nil
	})

	go func() {
		err := rpcServer.Run()
		if err != nil {
			b.Errorf("Error running RPC server: %v", err)
		}
	}()

	defer rpcServer.Close()

	ctx := context.Background()
	sem := make(chan struct{}, 3)
	wg := sync.WaitGroup{}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sem <- struct{}{}

		wg.Add(1)

		go func() {
			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)

			defer cancel()

			_, err := rpcClient.Call(ctx, "Echo", &TestEchoRequest{Value: "Hello, world!"})
			if err != nil {
				b.Errorf("Error calling RPC: %v", err)
			}

			<-sem
			wg.Done()
		}()
	}

	wg.Wait()
}
