package main

import (
	"fmt"
	"log/slog"

	"github.com/ksysoev/rpc-redis"
	"github.com/redis/go-redis/v9"
)

type EchoRequest struct {
	Value string `json:"value"`
}

type Stash struct {
	Value string `json:"value"`
}

func main() {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	rpcServer := rpc.NewServer(redisClient, "echo.EchoService", "echo-group", "echo-consumer")

	rpcServer.AddHandler("Echo", func(req rpc.Request) (any, error) {
		var echoReq EchoRequest

		err := req.ParseParams(&echoReq)
		if err != nil {
			return nil, fmt.Errorf("error parsing request: %v", err)
		}

		slog.Info("Received request: " + echoReq.Value)

		var stash Stash
		err = rpc.ParseStash(req.Context(), &stash)
		if err != nil {
			slog.Error("Error parsing stash: " + err.Error())
			return nil, err
		}

		slog.Info("Stash value: " + stash.Value)

		return &echoReq, nil
	})

	slog.Info("Starting RPC server")
	err := rpcServer.Run()
	if err != nil {
		slog.Error("Error running RPC server: " + err.Error())
	}

	slog.Info("Server stopped")

}
