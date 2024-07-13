package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ksysoev/rpc-redis"
	"github.com/redis/go-redis/v9"
)

type EchoRequest struct {
	Value string `json:"value"`
}

func main() {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	rpcClient := rpc.NewClient(redisClient, "echo.EchoService")
	defer rpcClient.Close()

	ctx := context.Background()
	resp, err := rpcClient.Call(ctx, "Echo", &EchoRequest{Value: "Hello, world!"})

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
