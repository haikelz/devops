package ai

import "context"

type Client interface {
	GenerateReply(ctx context.Context, prompt string) (string, error)
	GenerateReplyWithImage(ctx context.Context, prompt string, imageBytes []byte, mimeType string) (string, error)
}
