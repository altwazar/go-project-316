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

const httpString = "http"

// Основная функция - оркестратор
func checkLinkStatus(ctx context.Context, urlStr string, client *http.Client) (int, error) {
	normalizedURL := normalizeOrKeep(urlStr)

	parsedURL, err := parseAndSetScheme(normalizedURL)
	if err != nil {
		return 0, err
	}

	return attemptRequest(ctx, parsedURL.String(), client)
}

// Нормализация с fallback
func normalizeOrKeep(urlStr string) string {
	normalizedURL, err := normalizeURL(urlStr)
	if err != nil {
		return urlStr
	}
	return normalizedURL
}

// Парсинг и установка схемы по умолчанию
func parseAndSetScheme(rawURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme == "" {
		parsedURL.Scheme = httpString
	}

	return parsedURL, nil
}

// Попытка выполнения запроса с fallback на GET
func attemptRequest(ctx context.Context, urlStr string, client *http.Client) (int, error) {
	// Пробуем HEAD запрос
	if statusCode, err := doHeadRequest(ctx, urlStr, client); err == nil {
		return statusCode, nil
	}

	// Fallback на GET
	return doGetRequest(ctx, urlStr, client)
}

// Выполнение HEAD запроса
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

// Выполнение GET запроса
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

// getPageWithRetries - получение страницы в несколько попыток
func getPageWithRetries(ctx context.Context, url string, depth int, opts Options) Page {
	state := &retryState{
		ctx:     ctx,
		url:     url,
		depth:   depth,
		opts:    opts,
		lastErr: nil,
	}

	for attempt := 0; attempt <= opts.Retries; attempt++ {
		if result := state.tryAttempt(attempt); result != nil {
			return *result
		}
	}

	return newPageResponse(0, url, depth, nil, SEOData{}, []Asset{}, state.getLastErrorMsg())
}

// retryState хранит состояние между попытками
type retryState struct {
	ctx     context.Context
	url     string
	depth   int
	opts    Options
	lastErr error
}

// tryAttempt - одна попытка загрузки страницы
func (s *retryState) tryAttempt(attempt int) *Page {
	if s.isContextCancelled() {
		return s.createCancelledPage()
	}

	finalURL, statusCode, links, seoData, assets, err := getPageWithLinks(s.ctx, s.url, s.opts.HTTPClient)

	if s.isSuccessful(statusCode, err) {
		return s.createSuccessPage(finalURL, statusCode, links, seoData, assets)
	}

	s.updateLastError(statusCode, err)

	if s.shouldRetry(attempt) {
		s.waitBeforeRetry()
	}

	return nil
}

// isContextCancelled - проверка отмены контекста
func (s *retryState) isContextCancelled() bool {
	select {
	case <-s.ctx.Done():
		return true
	default:
		return false
	}
}

// isSuccessful - проверка успешности запроса
func (s *retryState) isSuccessful(statusCode int, err error) bool {
	return err == nil && statusCode > 0 && statusCode < 500
}

// createCancelledPage - создание страницы с ошибкой отмены
func (s *retryState) createCancelledPage() *Page {
	page := newPageResponse(0, s.url, s.depth, nil, SEOData{}, []Asset{}, s.ctx.Err().Error())
	return &page
}

// createSuccessPage - создание успешной страницы
func (s *retryState) createSuccessPage(finalURL string, statusCode int, links []string, seoData SEOData, assets []Asset) *Page {
	page := newPageResponse(statusCode, finalURL, s.depth, links, seoData, assets, "")
	return &page
}

// updateLastError - обновление последней ошибки
func (s *retryState) updateLastError(statusCode int, err error) {
	if err != nil {
		s.lastErr = err
	} else if statusCode >= 500 {
		s.lastErr = fmt.Errorf("HTTP %d", statusCode)
	}
}

// shouldRetry - проверка необходимости повторной попытки
func (s *retryState) shouldRetry(attempt int) bool {
	return attempt < s.opts.Retries
}

// waitBeforeRetry - ожидание перед повтором
func (s *retryState) waitBeforeRetry() {
	if s.opts.Delay <= 0 {
		return
	}

	select {
	case <-s.ctx.Done():
		return
	case <-time.After(s.opts.Delay):
	}
}

// getLastErrorMsg - получение сообщения последней ошибки
func (s *retryState) getLastErrorMsg() string {
	if s.lastErr != nil {
		return s.lastErr.Error()
	}
	return ""
}

// getPageWithLinks - получение страницы
func getPageWithLinks(ctx context.Context, urlStr string, httpClient *http.Client) (string, int, []string, SEOData, []Asset, error) {
	page := &pageProcessor{
		originalURL: urlStr,
		httpClient:  httpClient,
	}

	if err := page.fetchPage(ctx, urlStr); err != nil {
		return urlStr, 0, nil, SEOData{}, []Asset{}, err
	}
	defer page.closeResponse()

	if !page.isHTML() {
		return page.finalURL, page.statusCode, nil, SEOData{}, []Asset{}, nil
	}

	if err := page.processHTML(); err != nil {
		return page.finalURL, page.statusCode, nil, SEOData{}, []Asset{}, err
	}

	return page.finalURL, page.statusCode, page.links, page.seo, page.assets, nil
}

// pageProcessor - структура для обработки страницы
type pageProcessor struct {
	originalURL string
	httpClient  *http.Client
	resp        *http.Response
	finalURL    string
	statusCode  int
	links       []string
	assets      []Asset
	seo         SEOData
	htmlBody    string
}

// fetchPage - загрузка страницы
func (p *pageProcessor) fetchPage(ctx context.Context, urlStr string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request execution failed: %w", err)
	}

	p.resp = resp
	p.finalURL = resp.Request.URL.String()
	p.statusCode = resp.StatusCode

	return nil
}

// closeResponse - закрытие тела ответа
func (p *pageProcessor) closeResponse() {
	if p.resp != nil {
		_ = p.resp.Body.Close()
	}
}

// isHTML - проверка HTML контента
func (p *pageProcessor) isHTML() bool {
	contentType := p.resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "text/html")
}

// processHTML - обработка HTML контента
func (p *pageProcessor) processHTML() error {
	if err := p.readHTMLBody(); err != nil {
		return err
	}

	baseURL, err := url.Parse(p.originalURL)
	if err != nil {
		return fmt.Errorf("failed to parse base URL: %w", err)
	}

	links, assets, err := extractLinksAndAssets(p.htmlBody, baseURL)
	if err != nil {
		return fmt.Errorf("failed to extract links and assets: %w", err)
	}

	p.links = links
	p.assets = assets
	p.seo = extractSEOData(p.htmlBody)

	return nil
}

// readHTMLBody - чтение тела с ограничением
func (p *pageProcessor) readHTMLBody() error {
	const maxSize = 10 * 1024 * 1024 // 10MB

	limitedReader := io.LimitReader(p.resp.Body, maxSize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	p.htmlBody = string(bodyBytes)
	return nil
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

// checkAsset - проверка ассета и получение его размера (сложность 3)
func checkAsset(ctx context.Context, urlStr string, assetType AssetType, client *http.Client) (Asset, error) {
	asset := createBaseAsset(urlStr, assetType)

	validatedURL, err := validateAndNormalizeURL(urlStr)
	if err != nil {
		asset.Error = err.Error()
		return asset, err
	}

	resp, err := executeAssetRequest(ctx, validatedURL, client)
	if err != nil {
		asset.Error = fmt.Sprintf("request failed: %v", err)
		return asset, err
	}
	defer resp.Body.Close()

	return processAssetResponse(asset, resp), nil
}

// createBaseAsset - создание базовой структуры ассета
func createBaseAsset(urlStr string, assetType AssetType) Asset {
	return Asset{
		URL:        urlStr,
		Type:       assetType,
		StatusCode: 0,
		SizeBytes:  0,
	}
}

// validateAndNormalizeURL - валидация и нормализация URL
func validateAndNormalizeURL(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme == "" {
		parsedURL.Scheme = httpString
	}

	return parsedURL.String(), nil
}

// executeAssetRequest - выполнение запроса для ассета
func executeAssetRequest(ctx context.Context, urlStr string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return client.Do(req)
}

// processAssetResponse - обработка ответа для ассета
func processAssetResponse(asset Asset, resp *http.Response) Asset {
	asset.StatusCode = resp.StatusCode

	if isSuccessStatusCode(resp.StatusCode) {
		return processSuccessfulAsset(asset, resp)
	}

	asset.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	return asset
}

// isSuccessStatusCode - проверка успешного статуса
func isSuccessStatusCode(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// processSuccessfulAsset - обработка успешного ассета
func processSuccessfulAsset(asset Asset, resp *http.Response) Asset {
	size, err := getContentSize(resp)
	if err != nil {
		asset.SizeBytes = 0
		asset.Error = fmt.Sprintf("failed to get size: %v", err)
	} else {
		asset.SizeBytes = size
	}
	return asset
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
