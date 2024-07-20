package rpc

// ClientInterceptor is a function type that represents an interceptor for client requests.
// It takes a pointer to a Request struct and returns a pointer to a Response struct and an error.
type ClientInterceptor func(req *Request) (*Response, error)

type ServerInterceptor func(req *Request) (*Response, error)
