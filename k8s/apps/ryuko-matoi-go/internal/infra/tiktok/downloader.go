package tiktok

import (
	"ryuko-matoi/internal/infra/media_downloader"
)

var (
	ErrInvalidTikTokURL = media_downloader.ErrInvalidURL
	ErrNoMediaFound     = media_downloader.ErrNoMediaFound
	ErrYtDlpNotFound    = media_downloader.ErrYtDlpNotFound
)

func NewDownloader(ytDlpBinary string) *media_downloader.YtDlpDownloader {
	return media_downloader.NewYtDlpDownloader(
		ytDlpBinary,
		media_downloader.WithURLValidator(media_downloader.IsTikTokURL),
	)
}
