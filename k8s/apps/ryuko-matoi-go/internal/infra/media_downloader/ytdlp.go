package media_downloader

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrYtDlpNotFound = errors.New("yt-dlp binary not found")
	ErrInvalidURL    = errors.New("invalid url")
	ErrNoMediaFound  = errors.New("no media found")
)

type YtDlpDownloader struct {
	binary   string
	tmpDir   string
	validate func(string) bool
}

type YtDlpOption func(*YtDlpDownloader)

func WithURLValidator(fn func(string) bool) YtDlpOption {
	return func(d *YtDlpDownloader) {
		d.validate = fn
	}
}

func NewYtDlpDownloader(binary string, opts ...YtDlpOption) *YtDlpDownloader {
	if binary == "" {
		binary = "yt-dlp"
	}
	d := &YtDlpDownloader{
		binary: binary,
		tmpDir: os.TempDir(),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (d *YtDlpDownloader) Download(ctx context.Context, mediaURL string) ([]Media, error) {
	if _, err := exec.LookPath(d.binary); err != nil {
		return nil, ErrYtDlpNotFound
	}

	if d.validate != nil && !d.validate(mediaURL) {
		return nil, ErrInvalidURL
	}

	outDir, err := os.MkdirTemp(d.tmpDir, "media-dl-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(outDir)

	outPath := filepath.Join(outDir, "media.%(ext)s")
	if err := d.downloadWithFallback(ctx, mediaURL, outPath); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		return nil, fmt.Errorf("read download dir: %w", err)
	}

	var items []Media
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, readErr := os.ReadFile(filepath.Join(outDir, entry.Name()))
		if readErr != nil || len(data) == 0 {
			continue
		}

		mimeType := detectMimeType(data, filepath.Ext(entry.Name()))
		mediaType := mediaTypeFromMime(mimeType)
		if mediaType == "" {
			continue
		}

		items = append(items, Media{
			URL:      mediaURL,
			Type:     mediaType,
			MimeType: mimeType,
			Data:     data,
		})
	}

	if len(items) == 0 {
		return nil, ErrNoMediaFound
	}

	return items, nil
}

func (d *YtDlpDownloader) downloadWithFallback(ctx context.Context, mediaURL string, outPath string) error {
	baseArgs := []string{
		"--no-playlist",
		"--no-warnings",
		"--no-progress",
		"--no-check-certificates",
		"--force-ipv4",
		"--retries", "5",
		"--fragment-retries", "5",
		"--extractor-retries", "3",
		"--socket-timeout", "30",
	}
	if cookiesPath := strings.TrimSpace(os.Getenv("YTDLP_COOKIES_FILE")); cookiesPath != "" {
		if _, err := os.Stat(cookiesPath); err == nil {
			baseArgs = append(baseArgs, "--cookies", cookiesPath)
		}
	}

	attempts := make([][]string, 0, 4)
	if IsYouTubeURL(mediaURL) {
		attempt1 := append([]string{}, baseArgs...)
		attempt1 = append(attempt1,
			"--extractor-args", "youtube:player_client=android,web,tv",
			"--format", "bv*[ext=mp4][height<=720]+ba[ext=m4a]/b[ext=mp4][height<=720]/b",
			"--remux-video", "mp4",
			"--merge-output-format", "mp4",
			"-o", outPath,
			mediaURL,
		)

		attempt2 := append([]string{}, baseArgs...)
		attempt2 = append(attempt2,
			"--format", "b[ext=mp4]/bv*+ba/b",
			"--merge-output-format", "mp4",
			"-o", outPath,
			mediaURL,
		)

		attempt3 := append([]string{}, baseArgs...)
		attempt3 = append(attempt3,
			"--format", "bv*+ba/b",
			"--merge-output-format", "mp4",
			"-o", outPath,
			mediaURL,
		)

		attempt4 := append([]string{}, baseArgs...)
		attempt4 = append(attempt4,
			"--format", "b",
			"-o", outPath,
			mediaURL,
		)

		attempts = append(attempts, attempt1, attempt2, attempt3, attempt4)
	} else if IsTikTokURL(mediaURL) {
		attempt1 := append([]string{}, baseArgs...)
		attempt1 = append(attempt1,
			"--extractor-args", "tiktok:",
			"--format", "bv*+ba/b",
			"--merge-output-format", "mp4",
			"-o", outPath,
			mediaURL,
		)

		attempt2 := append([]string{}, baseArgs...)
		attempt2 = append(attempt2,
			"--format", "b",
			"-o", outPath,
			mediaURL,
		)

		// fallback terakhir: tanpa format agar yt-dlp memilih media terbaik yang tersedia
		// (termasuk photo/slideshow post).
		attempt3 := append([]string{}, baseArgs...)
		attempt3 = append(attempt3,
			"-o", outPath,
			mediaURL,
		)

		attempts = append(attempts, attempt1, attempt2, attempt3)
	} else if IsFacebookURL(mediaURL) {
		attempt1 := append([]string{}, baseArgs...)
		attempt1 = append(attempt1,
			"--format", "bv*[ext=mp4]+ba[ext=m4a]/b[ext=mp4][vcodec!=none]/b[vcodec!=none]",
			"--remux-video", "mp4",
			"--merge-output-format", "mp4",
			"-o", outPath,
			mediaURL,
		)

		attempt2 := append([]string{}, baseArgs...)
		attempt2 = append(attempt2,
			"--format", "bv*[vcodec!=none]+ba/b[vcodec!=none]",
			"--merge-output-format", "mp4",
			"-o", outPath,
			mediaURL,
		)

		attempt3 := append([]string{}, baseArgs...)
		attempt3 = append(attempt3,
			"--format", "b[ext=mp4][vcodec!=none]/b[vcodec!=none]",
			"-o", outPath,
			mediaURL,
		)

		attempt4 := append([]string{}, baseArgs...)
		attempt4 = append(attempt4,
			"-o", outPath,
			mediaURL,
		)

		attempts = append(attempts, attempt1, attempt2, attempt3, attempt4)
	} else {
		attempt1 := append([]string{}, baseArgs...)
		attempt1 = append(attempt1,
			"--format", "bv*[ext=mp4]+ba[ext=m4a]/b[ext=mp4]/b",
			"--remux-video", "mp4",
			"--merge-output-format", "mp4",
			"-o", outPath,
			mediaURL,
		)

		attempt2 := append([]string{}, baseArgs...)
		attempt2 = append(attempt2,
			"--format", "bv*+ba/b",
			"--merge-output-format", "mp4",
			"-o", outPath,
			mediaURL,
		)

		attempt3 := append([]string{}, baseArgs...)
		attempt3 = append(attempt3,
			"--format", "b",
			"-o", outPath,
			mediaURL,
		)

		attempts = append(attempts, attempt1, attempt2, attempt3)
	}

	var combinedErr []string
	for index, args := range attempts {
		cmd := exec.CommandContext(ctx, d.binary, args...)
		output, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}

		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			combinedErr = append(combinedErr, fmt.Sprintf("attempt-%d: %s", index+1, trimmed))
		} else {
			combinedErr = append(combinedErr, fmt.Sprintf("attempt-%d: %s", index+1, err.Error()))
		}
	}

	if len(combinedErr) == 0 {
		return fmt.Errorf("yt-dlp download failed")
	}

	return fmt.Errorf("yt-dlp download failed: %s", truncateJoinedErrors(combinedErr))
}

func DetectMimeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".m4a":
		return "audio/mp4"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func (d *YtDlpDownloader) VideoToMp3(ctx context.Context, mediaURL string) ([]Media, error) {
	if _, err := exec.LookPath(d.binary); err != nil {
		return nil, ErrYtDlpNotFound
	}

	if d.validate != nil && !d.validate(mediaURL) {
		return nil, ErrInvalidURL
	}

	outDir, err := os.MkdirTemp(d.tmpDir, "media-dl-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(outDir)

	outPath := filepath.Join(outDir, "media.%(ext)s")

	args := []string{
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--output", outPath,
		mediaURL,
	}

	cmd := exec.CommandContext(ctx, d.binary, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("yt-dlp download failed: %w: %s", err, string(output))
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		return nil, fmt.Errorf("read download dir: %w", err)
	}

	var items []Media
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, readErr := os.ReadFile(filepath.Join(outDir, entry.Name()))
		if readErr != nil || len(data) == 0 {
			continue
		}

		mimeType := detectMimeType(data, filepath.Ext(entry.Name()))
		mediaType := mediaTypeFromMime(mimeType)
		if mediaType == "" {
			continue
		}

		items = append(items, Media{
			URL:      mediaURL,
			Type:     mediaType,
			MimeType: mimeType,
			Data:     data,
		})
	}

	if len(items) == 0 {
		return nil, ErrNoMediaFound
	}

	return items, nil
}

func detectMimeType(data []byte, ext string) string {
	if len(data) > 0 {
		detected := strings.ToLower(strings.TrimSpace(http.DetectContentType(data)))
		if strings.HasPrefix(detected, "video/") || strings.HasPrefix(detected, "image/") || strings.HasPrefix(detected, "audio/") {
			return detected
		}
	}

	return DetectMimeFromExt(ext)
}

func mediaTypeFromMime(mimeType string) string {
	normalized := strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(normalized, "image/") {
		return "image"
	}
	if strings.HasPrefix(normalized, "video/") {
		return "video"
	}

	return ""
}

func truncateJoinedErrors(parts []string) string {
	joined := strings.Join(parts, " | ")
	joined = strings.Join(strings.Fields(joined), " ")
	if len(joined) > 1500 {
		return joined[:1500] + "..."
	}

	return joined
}
