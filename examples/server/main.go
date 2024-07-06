package main

import (
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

	rpcServer := redisrpc.NewServer(redisClient, "echo", "echo-group", "echo-consumer")

	rpcServer.AddHandler("echo", func(req redisrpc.Request) (any, error) {
		var echoReq EchoRequest

		err := req.ParseParams(&echoReq)
		if err != nil {
			return nil, fmt.Errorf("error parsing request: %v", err)
		}

		slog.Info("Received request: " + echoReq.Message)

		return &echoReq, nil
	})

	slog.Info("Starting RPC server")
	err := rpcServer.Run()
	if err != nil {
		slog.Error("Error running RPC server: " + err.Error())
	}

	slog.Info("Server stopped")

}
