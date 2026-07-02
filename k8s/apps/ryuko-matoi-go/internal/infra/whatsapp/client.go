package whatsapp

import "context"

type MessageHandler func(ctx context.Context, event *IncomingMessageEvent) error

type Client interface {
	IsAuthenticated() bool
	QRChannel(ctx context.Context) (<-chan *QREvent, error)
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	SendText(ctx context.Context, request *SendTextRequest) (*SendTextResponse, error)
	SendImage(ctx context.Context, request *SendImageRequest) (*SendImageResponse, error)
	SendSticker(ctx context.Context, request *SendStickerRequest) (*SendStickerResponse, error)
	SendVideo(ctx context.Context, request *SendVideoRequest) (*SendVideoResponse, error)
	// SendAudio(ctx context.Context, request *SendAudioRequest) (*SendAudioResponse, error)
	RegisterMessageHandler(handler MessageHandler)
}
