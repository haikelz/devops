package ocr

import "context"

type Client interface {
	ExtractText(ctx context.Context, content []byte) (string, error)
}
