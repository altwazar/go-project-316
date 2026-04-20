package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// checkLinkStatus - проверка статуса ссылки
func checkLinkStatus(ctx context.Context, urlStr string, client *http.Client) (int, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return 0, fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "http"
	}

	// HEAD запрос
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, parsedURL.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	headResp, err := client.Do(headReq)
	if err == nil {
		defer func() {
			_ = headResp.Body.Close()
		}()
		return headResp.StatusCode, nil
	}

	// GET запрос как fallback
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create GET request: %w", err)
	}

	getResp, err := client.Do(getReq)
	if err != nil {
		return 0, fmt.Errorf("both HEAD and GET requests failed: %w", err)
	}
	defer func() {
		_ = getResp.Body.Close()
	}()
	return getResp.StatusCode, nil
}

// getPageWithRetries - получение страницы в несколько попыток
func getPageWithRetries(ctx context.Context, url string, depth int, opts Options) Page {
	var lastErr error

	for i := 0; i <= opts.Retries; i++ {
		select {
		case <-ctx.Done():
			return newPageResponse(0, url, depth, nil, SEOData{}, ctx.Err().Error())
		default:
		}

		finalURL, statusCode, links, seoData, err := getPageWithLinks(ctx, url, opts.HTTPClient)
		if err == nil && statusCode < 500 {
			return newPageResponse(statusCode, url, depth, links, seoData, "")
		}

		lastErr = err

		if i < opts.Retries && opts.Delay > 0 {
			select {
			case <-ctx.Done():
				return newPageResponse(0, finalURL, depth, nil, SEOData{}, ctx.Err().Error())
			case <-time.After(opts.Delay):
			}
		}
	}

	return newPageResponse(0, url, depth, nil, SEOData{}, lastErr.Error())
}

// getPageWithLinks - получение страницы
func getPageWithLinks(ctx context.Context, urlStr string, httpClient *http.Client) (string, int, []string, SEOData, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return urlStr, 0, nil, SEOData{}, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return urlStr, 0, nil, SEOData{}, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	finalURL := resp.Request.URL.String()

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return finalURL, resp.StatusCode, nil, SEOData{}, nil
	}

	const maxSize = 10 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return finalURL, resp.StatusCode, nil, SEOData{}, fmt.Errorf("failed to read body: %w", err)
	}

	baseURL, err := url.Parse(urlStr)
	if err != nil {
		return finalURL, resp.StatusCode, nil, SEOData{}, fmt.Errorf("failed to parse base URL: %w", err)
	}

	links, err := extractLinks(string(body), baseURL)
	if err != nil {
		return finalURL, resp.StatusCode, nil, SEOData{}, err
	}

	seoData := extractSEOData(string(body))

	return finalURL, resp.StatusCode, links, seoData, nil
}

func newPageResponse(statusCode int, url string, depth int, links []string, seo SEOData, errMsg string) Page {
	return Page{
		URL:          url,
		Depth:        depth,
		HTTPStatus:   statusCode,
		Status:       getStatusString(statusCode),
		BrokenLinks:  []LinkStatus{},
		Links:        links,
		DiscoveredAt: time.Now().UTC().Truncate(time.Second),
		Error:        errMsg,
		SEO:          seo,
	}
}

// getStatusString - получение статуса из кода
func getStatusString(statusCode int) string {
	switch statusCode {
	case 200, 201, 202, 204:
		return "ok"
	default:
		return "error"
	}
}
