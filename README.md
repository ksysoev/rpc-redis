# RPC Redis

[![RPC Redis](https://github.com/ksysoev/rpc-redis/actions/workflows/main.yml/badge.svg)](https://github.com/ksysoev/rpc-redis/actions/workflows/main.yml)
[![codecov](https://codecov.io/gh/ksysoev/rpc-redis/graph/badge.svg?token=Q1G80IX95U)](https://codecov.io/gh/ksysoev/rpc-redis)

`rpc-redis` is a Go package that implements a JSON-RPC-like protocol over Redis Streams and channels. This package allows you to build scalable and efficient RPC servers and clients using Redis as the underlying transport mechanism.

## Features

- **JSON-RPC-like Protocol**: Implements a protocol similar to JSON-RPC for seamless integration.
- **Redis Streams and Channels**: Utilizes Redis Streams and channels for message passing, ensuring high performance and reliability.
- **Easy to Use**: Simple API for setting up RPC servers and clients.
- **Flexible Handlers**: Easily add custom handlers for different RPC methods.

## Installation

To install the package, run:

```sh
go get github.com/yourusername/rpc-redis
```

## RPC Server

Below is an example of how to set up an RPC server:

```golang
redisClient := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

rpcServer := rpc.NewServer(redisClient, "echo.EchoService", "echo-group", "echo-consumer")

rpcServer.AddHandler("Echo", func(req rpc.Request) (any, error) {
    var echoReq EchoRequest
    if err := req.ParseParams(&echoReq); err != nil {
        return nil, fmt.Errorf("error parsing request: %v", err)
    }

    var stash Stash
    if err = rpc.ParseStash(req.Context(), &stash); err != nil {
        return nil, fmt.Errorf("error parsing stash: %v", err)
    }

    slog.Info("Received request: " + echoReq.Value)

    return &echoReq, nil
})

slog.Info("Starting RPC server")
if err := rpcServer.Run(); err != nil {
    slog.Error("Error running RPC server: " + err.Error())
}

slog.Info("Server stopped")
```

## RPC Client

Below is an example of how to set up an RPC client:

```golang
redisClient := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})
defer redisClient.Close()

rpcClient := rpc.NewClient(redisClient, "echo.EchoService")
defer rpcClient.Close()

ctx := context.Background()
ctx, err := rpc.SetStash(ctx, &Stash{Value: "Hello, stash!"})
if err != nil {
	slog.Error("Error setting stash: " + err.Error())
	return
}

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
```

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue if you encounter any problems or have suggestions for improvements.


## License

This project is licensed under the MIT License.