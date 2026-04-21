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
			return newPageResponse(0, url, depth, nil, SEOData{}, []Asset{}, ctx.Err().Error())
		default:
		}

		finalURL, statusCode, links, seoData, assets, err := getPageWithLinks(ctx, url, opts.HTTPClient)
		if err == nil && statusCode < 500 {
			return newPageResponse(statusCode, url, depth, links, seoData, assets, "")
		}

		lastErr = err

		if i < opts.Retries && opts.Delay > 0 {
			select {
			case <-ctx.Done():
				return newPageResponse(0, finalURL, depth, nil, SEOData{}, []Asset{}, ctx.Err().Error())
			case <-time.After(opts.Delay):
			}
		}
	}

	return newPageResponse(0, url, depth, nil, SEOData{}, []Asset{}, lastErr.Error())
}

// getPageWithLinks - получение страницы
func getPageWithLinks(ctx context.Context, urlStr string, httpClient *http.Client) (string, int, []string, SEOData, []Asset, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return urlStr, 0, nil, SEOData{}, []Asset{}, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return urlStr, 0, nil, SEOData{}, []Asset{}, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	finalURL := resp.Request.URL.String()

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return finalURL, resp.StatusCode, nil, SEOData{}, []Asset{}, nil
	}

	const maxSize = 10 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return finalURL, resp.StatusCode, nil, SEOData{}, []Asset{}, fmt.Errorf("failed to read body: %w", err)
	}

	baseURL, err := url.Parse(urlStr)
	if err != nil {
		return finalURL, resp.StatusCode, nil, SEOData{}, []Asset{}, fmt.Errorf("failed to parse base URL: %w", err)
	}

	links, assets, err := extractLinksAndAssets(string(body), baseURL)
	if err != nil {
		return finalURL, resp.StatusCode, nil, SEOData{}, []Asset{}, err
	}

	seoData := extractSEOData(string(body))

	return finalURL, resp.StatusCode, links, seoData, assets, nil
}

func newPageResponse(statusCode int, url string, depth int, links []string, seo SEOData, assets []Asset, errMsg string) Page {
	return Page{
		URL:          url,
		Depth:        depth,
		HTTPStatus:   statusCode,
		Status:       getStatusString(statusCode),
		BrokenLinks:  []LinkStatus{},
		Assets:       assets,
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

// checkAsset - проверка ассета и получение его размера
func checkAsset(ctx context.Context, urlStr string, assetType AssetType, client *http.Client) (Asset, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return Asset{URL: urlStr, Type: assetType, Error: fmt.Sprintf("invalid URL: %v", err)}, err
	}
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "http"
	}

	asset := Asset{
		URL:        urlStr,
		Type:       assetType,
		StatusCode: 0,
		SizeBytes:  0,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		asset.Error = fmt.Sprintf("failed to create request: %v", err)
		return asset, err
	}

	resp, err := client.Do(req)
	if err != nil {
		asset.Error = fmt.Sprintf("request failed: %v", err)
		return asset, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	asset.StatusCode = resp.StatusCode

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		size, err := getContentSize(resp)
		if err != nil {
			asset.SizeBytes = 0
			asset.Error = fmt.Sprintf("failed to get size: %v", err)
		} else {
			asset.SizeBytes = size
		}
	} else {
		asset.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return asset, nil
}

// getContentSize - получение размера контента
func getContentSize(resp *http.Response) (int64, error) {
	if resp.ContentLength >= 0 {
		return resp.ContentLength, nil
	}

	// Читаем тело для определения размера
	const maxSize = 50 * 1024 * 1024 // 50MB максимум для ассетов
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return 0, fmt.Errorf("failed to read body: %w", err)
	}

	return int64(len(body)), nil
}
