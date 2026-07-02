package helper

import (
	"encoding/json"
	"strconv"
	"strings"
)

func NormalizeMessage(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	return strings.Join(strings.Fields(trimmed), " ")
}

func SanitizeUserError(err error) string {
	if err == nil {
		return "unknown error"
	}

	text := strings.TrimSpace(err.Error())
	if text == "" {
		return "unknown error"
	}
	if len(text) > 220 {
		return text[:220] + "..."
	}

	return text
}

func FormatBulletList(items []string) string {
	if len(items) == 0 {
		return "-"
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, "- "+item)
	}

	return strings.Join(lines, "\n")
}

func SlugifyBasic(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return ""
	}

	builder := strings.Builder{}
	lastDash := false
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}

	result := strings.Trim(builder.String(), "-")
	return result
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func StringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strings.TrimSpace(strconv.FormatFloat(typed, 'f', -1, 64))
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func CompactJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}

	text := strings.TrimSpace(string(payload))
	if len(text) > 3000 {
		return text[:3000] + "..."
	}

	return text
}

func ExtractStringByPath(value any, path ...string) string {
	current := value
	for _, key := range path {
		mapValue, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		next, exists := mapValue[key]
		if !exists {
			return ""
		}
		current = next
	}

	return StringValue(current)
}
