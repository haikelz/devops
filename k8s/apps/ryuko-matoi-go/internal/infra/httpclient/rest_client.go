package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RestClient struct {
	httpClient *http.Client
}

func NewRestClient(timeout time.Duration) *RestClient {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	return &RestClient{
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (client *RestClient) Get(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build get request: %w", err)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("execute get request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read get response body: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("get request failed with status %d", response.StatusCode)
	}

	return body, nil
}

func (client *RestClient) Post(ctx context.Context, url string, body []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build post request: %w", err)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("execute post request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read post response body: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("post request failed with status %d", response.StatusCode)
	}

	return responseBody, nil
}
