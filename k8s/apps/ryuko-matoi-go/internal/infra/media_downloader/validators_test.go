package media_downloader

import "testing"

func TestIsInstagramURL(t *testing.T) {
	t.Parallel()
	ok := []string{
		"https://www.instagram.com/p/abc123/",
		"https://instagram.com/reel/abc123/",
		"https://www.instagram.com/tv/abc123/",
	}
	bad := []string{
		"https://example.com/reel/abc123/",
		"https://www.instagram.com/stories/user/123/",
	}
	for _, u := range ok {
		if !IsInstagramURL(u) {
			t.Errorf("expected true for %q", u)
		}
	}
	for _, u := range bad {
		if IsInstagramURL(u) {
			t.Errorf("expected false for %q", u)
		}
	}
}

func TestIsTikTokURL(t *testing.T) {
	t.Parallel()
	ok := []string{
		"https://www.tiktok.com/@user/video/1234567890",
		"https://vm.tiktok.com/ZMhAbCdEf/",
		"https://www.tiktok.com/t/ZTRxyz/",
	}
	bad := []string{
		"https://example.com/video/123",
		"https://tiktok.com/",
	}
	for _, u := range ok {
		if !IsTikTokURL(u) {
			t.Errorf("expected true for %q", u)
		}
	}
	for _, u := range bad {
		if IsTikTokURL(u) {
			t.Errorf("expected false for %q", u)
		}
	}
}

func TestIsTwitterURL(t *testing.T) {
	t.Parallel()
	ok := []string{
		"https://twitter.com/user/status/1234567890",
		"https://x.com/user/status/1234567890",
	}
	bad := []string{
		"https://twitter.com/user",
		"https://example.com/status/123",
	}
	for _, u := range ok {
		if !IsTwitterURL(u) {
			t.Errorf("expected true for %q", u)
		}
	}
	for _, u := range bad {
		if IsTwitterURL(u) {
			t.Errorf("expected false for %q", u)
		}
	}
}

func TestIsFacebookURL(t *testing.T) {
	t.Parallel()
	ok := []string{
		"https://www.facebook.com/user/videos/1234567890",
		"https://fb.watch/abc123/",
		"https://www.facebook.com/reel/1234567890",
		"https://www.facebook.com/share/v/abc123/",
		"https://l.facebook.com/l.php?u=https%3A%2F%2Fwww.facebook.com%2Freel%2F1",
		"https://lm.facebook.com/l.php?u=https%3A%2F%2Fexample.com",
		"https://m.facebook.com/story.php?story_fbid=1&id=2",
		"https://www.facebook.com/groups/123456789/permalink/987654321/",
		"https://www.facebook.com/username/posts/1234567890",
	}
	bad := []string{
		"https://facebook.com/user",
		"https://example.com/videos/123",
	}
	for _, u := range ok {
		if !IsFacebookURL(u) {
			t.Errorf("expected true for %q", u)
		}
	}
	for _, u := range bad {
		if IsFacebookURL(u) {
			t.Errorf("expected false for %q", u)
		}
	}
}

func TestIsYouTubeURL(t *testing.T) {
	t.Parallel()
	ok := []string{
		"https://www.youtube.com/watch?v=abc123",
		"https://youtu.be/abc123",
		"https://www.youtube.com/shorts/abc123",
		"https://www.youtube.com/live/abc123",
	}
	bad := []string{
		"https://youtube.com/",
		"https://example.com/watch?v=123",
	}
	for _, u := range ok {
		if !IsYouTubeURL(u) {
			t.Errorf("expected true for %q", u)
		}
	}
	for _, u := range bad {
		if IsYouTubeURL(u) {
			t.Errorf("expected false for %q", u)
		}
	}
}

func TestDetectMimeFromExt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ext      string
		expected string
	}{
		{".mp4", "video/mp4"},
		{".webm", "video/webm"},
		{".jpg", "image/jpeg"},
		{".MP4", "video/mp4"},
		{".xyz", "application/octet-stream"},
	}
	for _, tt := range tests {
		if got := DetectMimeFromExt(tt.ext); got != tt.expected {
			t.Errorf("DetectMimeFromExt(%q) = %q, want %q", tt.ext, got, tt.expected)
		}
	}
}
