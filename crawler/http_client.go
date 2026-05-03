package crawler

import (
	"context"
	"fmt"
	"net/http"
)

// checkLinkStatus - проверка статуса ссылки (основная функция)
func checkLinkStatus(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, error) {
	normalizedURL := normalizeOrKeep(urlStr)
	parsedURL, err := parseAndSetScheme(normalizedURL)
	if err != nil {
		return 0, err
	}
	return attemptRequest(ctx, parsedURL.String(), client, userAgent)
}

// attemptRequest - попытка выполнения запроса с fallback на GET
func attemptRequest(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, error) {
	// Пробуем HEAD запрос
	statusCode, err := doHeadRequest(ctx, urlStr, client, userAgent)
	if err == nil {
		// HEAD вернул ответ, но если метод не поддерживается сервером,
		// пробуем GET запрос (он может работать)
		if statusCode == http.StatusMethodNotAllowed || statusCode == http.StatusNotImplemented {
			return doGetRequest(ctx, urlStr, client, userAgent)
		}
		return statusCode, nil
	}

	// Если HEAD вернул транспортную ошибку (таймаут, отказ соединения и т.д.),
	// тоже пробуем GET - возможно, проблема только с HEAD
	return doGetRequest(ctx, urlStr, client, userAgent)
}

// doHeadRequest - выполнение HEAD запроса
func doHeadRequest(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, error) {
	req, err := newRequestWithUserAgent(ctx, http.MethodHead, urlStr, userAgent)
	if err != nil {
		return 0, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return resp.StatusCode, nil
}

// doGetRequest - выполнение GET запроса
func doGetRequest(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, error) {
	req, err := newRequestWithUserAgent(ctx, http.MethodGet, urlStr, userAgent)
	if err != nil {
		return 0, fmt.Errorf("failed to create GET request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("both HEAD and GET requests failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return resp.StatusCode, nil
}

// executeAssetRequest - выполнение запроса для ассета
func executeAssetRequest(ctx context.Context, urlStr string, client *http.Client, userAgent string) (*http.Response, error) {
	req, err := newRequestWithUserAgent(ctx, http.MethodGet, urlStr, userAgent)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	return client.Do(req)
}

// newRequestWithUserAgent создает HTTP-запрос с кастомным User-Agent
func newRequestWithUserAgent(ctx context.Context, method, urlStr, userAgent string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, nil)
	if err != nil {
		return nil, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	return req, nil
}
