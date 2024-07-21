package rpc

// ClientInterceptor is a function type that represents an interceptor for client requests.
// It takes a pointer to a Request struct and returns a pointer to a Response struct and an error.

type RequestHandler func(req *Request) (*Response, error)

type Interceptor func(next RequestHandler) RequestHandler

// useInterceptors applies a list of interceptors to a given RequestHandler.
// The interceptors are applied in reverse order, starting from the last interceptor in the slice.
// Each interceptor is called with the current RequestHandler as an argument, and the returned
// RequestHandler becomes the input for the next interceptor. The final RequestHandler is then
// returned as the result.
func useInterceptors(handler RequestHandler, interceptors []Interceptor) RequestHandler {
	for i := len(interceptors) - 1; i >= 0; i-- {
		handler = interceptors[i](handler)
	}

	return handler
}
