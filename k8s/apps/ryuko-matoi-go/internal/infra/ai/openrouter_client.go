package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenRouterClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewOpenRouterClient(apiKey string, model string) *OpenRouterClient {
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" {
		trimmedModel = "nousresearch/hermes-3-llama-3.1-405b:free"
	}

	return &OpenRouterClient{
		apiKey: strings.TrimSpace(apiKey),
		model:  trimmedModel,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (client *OpenRouterClient) GenerateReply(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(client.apiKey) == "" {
		return "", fmt.Errorf("openrouter api key is empty")
	}

	requestBody := map[string]any{
		"model": client.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	encodedBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal openrouter request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://openrouter.ai/api/v1/chat/completions",
		bytes.NewReader(encodedBody),
	)
	if err != nil {
		return "", fmt.Errorf("build openrouter request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+client.apiKey)
	req.Header.Set("User-Agent", "ryuko-matoi")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute openrouter request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read openrouter response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("openrouter request failed with status %d", resp.StatusCode)
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode openrouter response: %w", err)
	}

	choices, _ := parsed["choices"].([]any)
	if len(choices) == 0 {
		return "", fmt.Errorf("openrouter response has no choices")
	}

	firstChoice, _ := choices[0].(map[string]any)
	message, _ := firstChoice["message"].(map[string]any)
	result := strings.TrimSpace(extractOpenRouterContent(message["content"]))
	if result == "" {
		if refusal, ok := message["refusal"].(string); ok {
			result = strings.TrimSpace(refusal)
		}
	}
	if result == "" {
		if delta, ok := firstChoice["delta"].(map[string]any); ok {
			result = strings.TrimSpace(extractOpenRouterContent(delta["content"]))
		}
	}
	if result == "" {
		return "", fmt.Errorf("openrouter response text is empty")
	}

	return result, nil
}

func (client *OpenRouterClient) GenerateReplyWithImage(ctx context.Context, prompt string, imageBytes []byte, mimeType string) (string, error) {
	return "", fmt.Errorf("openrouter image input not supported")
}

func extractOpenRouterContent(raw any) string {
	switch typed := raw.(type) {
	case string:
		return typed
	case []any:
		builder := strings.Builder{}
		for _, item := range typed {
			text := strings.TrimSpace(extractOpenRouterContent(item))
			if text == "" {
				continue
			}
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(text)
		}
		return builder.String()
	case map[string]any:
		if text, ok := typed["text"].(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
		if content, exists := typed["content"]; exists {
			return extractOpenRouterContent(content)
		}
		if outputText, ok := typed["output_text"].(string); ok && strings.TrimSpace(outputText) != "" {
			return outputText
		}
	}
	return ""
}
