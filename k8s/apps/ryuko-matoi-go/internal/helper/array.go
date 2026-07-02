package helper

import (
	"strings"
)

func DeduplicateStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}

func PickFirstMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		if data, exists := typed["data"]; exists {
			if nested := PickFirstMap(data); nested != nil {
				return nested
			}
		}
		return typed
	case []any:
		for _, item := range typed {
			if mapped, ok := item.(map[string]any); ok {
				return mapped
			}
		}
	}

	return nil
}

func ExstractStringList(root map[string]any, key string) []string {
	raw, exists := root[key]
	if !exists {
		return nil
	}

	rawList, ok := raw.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(rawList))
	for _, item := range rawList {
		text := StringValue(item)
		if text == "" {
			continue
		}
		result = append(result, text)
	}

	return result
}
