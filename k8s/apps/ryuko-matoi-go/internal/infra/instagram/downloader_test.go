package instagram

import "testing"

func TestNewDownloaderNotNil(t *testing.T) {
	d := NewDownloader("")
	if d == nil {
		t.Fatal("expected non-nil downloader")
	}
}
