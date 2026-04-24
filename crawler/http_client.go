package crawler

import (
	"context"
	"fmt"
	"net/http"
)

// checkLinkStatus - проверка статуса ссылки (основная функция)
func checkLinkStatus(ctx context.Context, urlStr string, client *http.Client) (int, error) {
	normalizedURL := normalizeOrKeep(urlStr)

	parsedURL, err := parseAndSetScheme(normalizedURL)
	if err != nil {
		return 0, err
	}

	return attemptRequest(ctx, parsedURL.String(), client)
}

// attemptRequest - попытка выполнения запроса с fallback на GET
func attemptRequest(ctx context.Context, urlStr string, client *http.Client) (int, error) {
	// Пробуем HEAD запрос
	if statusCode, err := doHeadRequest(ctx, urlStr, client); err == nil {
		return statusCode, nil
	}

	// Fallback на GET
	return doGetRequest(ctx, urlStr, client)
}

// doHeadRequest - выполнение HEAD запроса
func doHeadRequest(ctx context.Context, urlStr string, client *http.Client) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, urlStr, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

// doGetRequest - выполнение GET запроса
func doGetRequest(ctx context.Context, urlStr string, client *http.Client) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create GET request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("both HEAD and GET requests failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

// executeAssetRequest - выполнение запроса для ассета
func executeAssetRequest(ctx context.Context, urlStr string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return client.Do(req)
}
