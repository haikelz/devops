package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GeminiClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewGeminiClient(apiKey string, model string) *GeminiClient {
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" {
		// We set fallback model to gemini 2.5 flash because its free
		trimmedModel = "gemini-2.5-flash"
	}

	return &GeminiClient{
		apiKey: strings.TrimSpace(apiKey),
		model:  trimmedModel,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (client *GeminiClient) GenerateReply(ctx context.Context, prompt string) (string, error) {
	requestBody := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	}

	return client.generate(ctx, requestBody)
}

func (client *GeminiClient) GenerateReplyWithImage(ctx context.Context, prompt string, imageBytes []byte, mimeType string) (string, error) {
	if len(imageBytes) == 0 {
		return "", fmt.Errorf("image bytes is empty")
	}

	trimmedMimeType := strings.TrimSpace(mimeType)
	if trimmedMimeType == "" {
		trimmedMimeType = "image/jpeg"
	}

	requestBody := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": prompt},
					{
						"inline_data": map[string]string{
							"mime_type": trimmedMimeType,
							"data":      base64.StdEncoding.EncodeToString(imageBytes),
						},
					},
				},
			},
		},
	}

	return client.generate(ctx, requestBody)
}

func (client *GeminiClient) generate(ctx context.Context, requestBody map[string]any) (string, error) {
	if strings.TrimSpace(client.apiKey) == "" {
		return "", fmt.Errorf("gemini api key is empty")
	}

	encodedBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal gemini request: %w", err)
	}

	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		client.model,
		client.apiKey,
	)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encodedBody))
	if err != nil {
		return "", fmt.Errorf("build gemini request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("execute gemini request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("read gemini response: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("gemini request failed with status %d", response.StatusCode)
	}

	var parsed map[string]any
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", fmt.Errorf("decode gemini response: %w", err)
	}

	candidates, ok := parsed["candidates"].([]any)
	if !ok || len(candidates) == 0 {
		return "", fmt.Errorf("gemini response has no candidates")
	}

	firstCandidate, ok := candidates[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid candidate format")
	}

	content, ok := firstCandidate["content"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid content format")
	}

	parts, ok := content["parts"].([]any)
	if !ok || len(parts) == 0 {
		return "", fmt.Errorf("gemini response has no parts")
	}

	textBuilder := strings.Builder{}
	for _, rawPart := range parts {
		part, ok := rawPart.(map[string]any)
		if !ok {
			continue
		}
		text, ok := part["text"].(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		if textBuilder.Len() > 0 {
			textBuilder.WriteString("\n")
		}
		textBuilder.WriteString(trimmed)
	}

	result := strings.TrimSpace(textBuilder.String())
	if result == "" {
		return "", fmt.Errorf("gemini response text is empty")
	}

	return result, nil
}
