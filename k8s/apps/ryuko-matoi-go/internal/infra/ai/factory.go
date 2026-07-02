package ai

import (
	"fmt"
	"strings"

	"ryuko-matoi/internal/config"
)

func NewClientFromConfig(cfg *config.AIConfig) (Client, error) {
	if cfg == nil {
		return nil, nil
	}

	apiKey := strings.TrimSpace(cfg.ApiKey)
	if apiKey == "" {
		return nil, nil
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))

	// We use more than one provider here. At this time, I use Gemini and OpenRouter. Set the configuration for it in .env file.
	// AI_PROVIDER, AI_API_KEY, AI_MODEL
	switch provider {
	case "", "google-genai", "gemini":
		return NewGeminiClient(apiKey, cfg.Model), nil
	case "openrouter":
		return NewOpenRouterClient(apiKey, cfg.Model), nil
	default:
		return nil, fmt.Errorf("unsupported AI_PROVIDER: %s", provider)
	}
}
