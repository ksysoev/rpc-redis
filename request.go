package redisrpc

type Request struct {
	req_id string
	params []byte
}
