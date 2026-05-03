package fetcher

import (
	"context"
	"fmt"
	"net/http"

	"code/internal/urlutil"
)

// CheckLinkStatus - проверка статуса ссылки (HEAD, при необходимости GET)
func CheckLinkStatus(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, error) {
	normalizedURL := urlutil.NormalizeOrKeep(urlStr)
	validatedURL, err := urlutil.ValidateAndNormalizeURL(normalizedURL)
	if err != nil {
		return 0, err
	}
	return attemptRequest(ctx, validatedURL, client, userAgent)
}

func attemptRequest(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, error) {
	statusCode, err := doHeadRequest(ctx, urlStr, client, userAgent)
	if err == nil {
		if statusCode == http.StatusMethodNotAllowed || statusCode == http.StatusNotImplemented {
			return doGetRequest(ctx, urlStr, client, userAgent)
		}
		return statusCode, nil
	}
	return doGetRequest(ctx, urlStr, client, userAgent)
}

func doHeadRequest(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, error) {
	req, err := newRequestWithUserAgent(ctx, http.MethodHead, urlStr, userAgent)
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

func doGetRequest(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, error) {
	req, err := newRequestWithUserAgent(ctx, http.MethodGet, urlStr, userAgent)
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
