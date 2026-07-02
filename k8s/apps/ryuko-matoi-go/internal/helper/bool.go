package helper

import "strings"

func LooksLikeHTML(payload []byte) bool {
	text := strings.ToLower(strings.TrimSpace(string(payload)))
	if text == "" {
		return false
	}

	return strings.HasPrefix(text, "<!doctype html") ||
		strings.HasPrefix(text, "<html") ||
		strings.Contains(text, "<head") ||
		strings.Contains(text, "<body")
}
