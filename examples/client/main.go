package main

import (
	"context"
	"fmt"
	"log/slog"

	redisrpc "github.com/ksysoev/redis-rpc"
	"github.com/redis/go-redis/v9"
)

type EchoRequest struct {
	Message string `json:"msg"`
}

func main() {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	rpcClient := redisrpc.NewClient(redisClient, "echoStream")
	defer rpcClient.Close()

	ctx := context.Background()
	resp, err := rpcClient.Call(ctx, "echo", &EchoRequest{Message: "Hello, world!"})

	if err != nil {
		slog.Error("Error calling RPC: " + err.Error())
		return
	}

	var result EchoRequest
	err = resp.ParseResut(&result)
	if err != nil {
		slog.Error("Error parsing result: " + err.Error())
		return
	}

	fmt.Println(result)
}
