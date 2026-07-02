package httpclient

import "context"

type Client interface {
	Get(ctx context.Context, url string) ([]byte, error)
	Post(ctx context.Context, url string, body []byte) ([]byte, error)
}
