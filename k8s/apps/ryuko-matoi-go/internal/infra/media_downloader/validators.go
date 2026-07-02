package media_downloader

import (
	"net/url"
	"strings"
)

func IsInstagramURL(rawURL string) bool {
	return matchHost(rawURL, func(host, path string) bool {
		if host != "instagram.com" && !strings.HasSuffix(host, ".instagram.com") {
			return false
		}
		return strings.HasPrefix(path, "p/") ||
			strings.HasPrefix(path, "reel/") ||
			strings.HasPrefix(path, "tv/")
	})
}

func IsTikTokURL(rawURL string) bool {
	return matchHost(rawURL, func(host, path string) bool {
		if host != "tiktok.com" && !strings.HasSuffix(host, ".tiktok.com") {
			return false
		}
		if host == "vm.tiktok.com" || host == "vt.tiktok.com" {
			return len(path) > 0
		}
		return strings.Contains(path, "/video/") ||
			strings.HasPrefix(path, "t/") ||
			(strings.Contains(path, "/") && len(path) > 1)
	})
}

func IsTwitterURL(rawURL string) bool {
	return matchHost(rawURL, func(host, path string) bool {
		isTwitter := host == "twitter.com" || host == "x.com" ||
			strings.HasSuffix(host, ".twitter.com") || strings.HasSuffix(host, ".x.com")
		if !isTwitter {
			return false
		}
		return strings.Contains(path, "/status/")
	})
}

func IsFacebookURL(rawURL string) bool {
	return matchHost(rawURL, func(host, path string) bool {
		isFB := host == "facebook.com" || host == "fb.watch" || host == "fb.com" ||
			strings.HasSuffix(host, ".facebook.com") || strings.HasSuffix(host, ".fb.com")
		if !isFB {
			return false
		}
		if host == "fb.watch" {
			return len(path) > 0
		}
		return facebookPathLooksLikeVideoOrPost(path)
	})
}

func facebookPathLooksLikeVideoOrPost(path string) bool {
	if path == "" {
		return false
	}
	return strings.Contains(path, "videos/") ||
		strings.Contains(path, "watch") ||
		strings.Contains(path, "reel/") ||
		strings.Contains(path, "share/v/") ||
		strings.Contains(path, "share/r/") ||
		strings.Contains(path, "permalink") ||
		strings.Contains(path, "story.php") ||
		strings.Contains(path, "photo.php") ||
		strings.HasPrefix(path, "l.php") ||
		strings.Contains(path, "groups/") ||
		strings.Contains(path, "/posts/") ||
		strings.Contains(path, "plugins/video.php") ||
		strings.Contains(path, "/share/")
}

func IsYouTubeURL(rawURL string) bool {
	return matchHost(rawURL, func(host, path string) bool {
		isYT := host == "youtube.com" || host == "youtu.be" ||
			strings.HasSuffix(host, ".youtube.com")
		if !isYT {
			return false
		}
		if host == "youtu.be" {
			return true
		}
		return strings.HasPrefix(path, "watch") ||
			strings.HasPrefix(path, "shorts/") ||
			strings.HasPrefix(path, "live/")
	})
}

func matchHost(rawURL string, matcher func(host, path string) bool) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	cleanPath := strings.Trim(parsed.EscapedPath(), "/")
	return matcher(host, cleanPath)
}
