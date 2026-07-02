package media_downloader

import "context"

type Media struct {
	URL      string
	Type     string
	MimeType string
	Data     []byte
}

type Client interface {
	Download(ctx context.Context, url string) ([]Media, error)
	VideoToMp3(ctx context.Context, url string) ([]Media, error)
}
