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

// getPageWithRetries - получение страницы в несколько попыток
func getPageWithRetries(ctx context.Context, url string, depth int, opts Options) Page {
	normalizedURL, _ := normalizeURL(url)
	state := &retryState{
		ctx:     ctx,
		url:     normalizedURL,
		depth:   depth,
		opts:    opts,
		lastErr: nil,
	}

	for attempt := 0; attempt <= opts.Retries; attempt++ {
		if result := state.tryAttempt(attempt); result != nil {
			return *result
		}
	}

	return newPageResponse(0, normalizedURL, depth, nil, SEOData{}, nil, state.getLastErrorMsg())
}

// isRetriable - проверяет, должна ли быть повторная попытка
func isRetriable(statusCode int, err error) bool {
	if err != nil {
		// Сетевые ошибки, таймауты - повторяем
		return true
	}
	// 429 Too Many Requests и все 5xx ошибки - повторяем
	return statusCode == 429 || statusCode >= 500
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
	finalURL, statusCode, links, seoData, assets, err := getPageWithLinks(s.ctx, s.url, s.opts.HTTPClient, s.opts.UserAgent)

	if s.isSuccessAndNotRetriable(statusCode, err) {
		return s.createSuccessPage(finalURL, statusCode, links, seoData, assets)
	}

	s.updateLastError(statusCode, err)

	if s.shouldRetry(attempt, statusCode, err) {
		s.waitBeforeRetry(attempt)
	}

	return nil
}

// isSuccessAndNotRetriable - проверка успешности и отсутствия необходимости ретрая
func (s *retryState) isSuccessAndNotRetriable(statusCode int, err error) bool {
	return err == nil && statusCode > 0 && !isRetriable(statusCode, err)
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
	} else if isRetriable(statusCode, nil) {
		s.lastErr = fmt.Errorf("HTTP %d (retriable)", statusCode)
	}
}

// shouldRetry - проверка необходимости повторной попытки
func (s *retryState) shouldRetry(attempt int, statusCode int, err error) bool {
	if attempt >= s.opts.Retries {
		return false
	}
	return isRetriable(statusCode, err)
}

// waitBeforeRetry - ожидание перед повтором с учетом RPS, Delay и backoff
func (s *retryState) waitBeforeRetry(attempt int) {
	waitTime := s.calculateBackoff(attempt)
	if waitTime <= 0 {
		return
	}

	select {
	case <-s.ctx.Done():
		return
	case <-time.After(waitTime):
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
func getPageWithLinks(ctx context.Context, urlStr string, httpClient *http.Client, userAgent string) (string, int, []string, SEOData, []Asset, error) {
	page := &pageProcessor{
		originalURL: urlStr,
		httpClient:  httpClient,
	}

	if err := page.fetchPage(ctx, urlStr, userAgent); err != nil {
		return urlStr, 0, nil, SEOData{}, []Asset{}, err
	}
	defer page.closeResponse()

	if !page.isHTML() {
		if page.isXML() {
			return page.processXML()
		}
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
func (p *pageProcessor) fetchPage(ctx context.Context, urlStr string, userAgent string) error {
	req, err := newRequestWithUserAgent(ctx, http.MethodGet, urlStr, userAgent)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}

	p.resp = resp
	p.finalURL, _ = normalizeURL(resp.Request.URL.String())
	p.statusCode = resp.StatusCode

	return nil
}

func (p *pageProcessor) isXML() bool {
	contentType := p.resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "text/xml") ||
		strings.Contains(contentType, "application/rss+xml")
}

func (p *pageProcessor) processXML() (string, int, []string, SEOData, []Asset, error) {
	if err := p.readXMLBody(); err != nil {
		return p.finalURL, p.statusCode, nil, SEOData{}, []Asset{}, err
	}

	// Парсим RSS/XML для извлечения title
	seo := extractSEOFromXML(p.htmlBody)

	return p.finalURL, p.statusCode, nil, seo, []Asset{}, nil
}

func (p *pageProcessor) readXMLBody() error {
	limitedReader := io.LimitReader(p.resp.Body, MaxHTMLBodySize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	p.htmlBody = string(bodyBytes)
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
	limitedReader := io.LimitReader(p.resp.Body, MaxHTMLBodySize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	p.htmlBody = string(bodyBytes)
	return nil
}

// newPageResponse - создание ответа страницы
func newPageResponse(statusCode int, url string, depth int, links []string, seo SEOData, assets []Asset, errMsg string) Page {
	status := getStatusString(statusCode)
	if status == "ok" {
		return Page{
			URL:          url,
			Depth:        depth,
			HTTPStatus:   statusCode,
			Status:       status,
			BrokenLinks:  []LinkStatus{},
			Assets:       assets,
			Links:        links,
			DiscoveredAt: time.Now().UTC().Truncate(time.Second),
			Error:        errMsg,
			SEO:          seo,
		}
	}
	return Page{
		URL:          url,
		Depth:        depth,
		HTTPStatus:   statusCode,
		Status:       status,
		BrokenLinks:  nil,
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
		return StatusOK
	default:
		return StatusError
	}
}

// getBaseWaitTime - получение базового времени ожидания из настроек
func (s *retryState) getBaseWaitTime() time.Duration {
	switch {
	case s.opts.RPS > 0:
		// Для RPS задержка = 1 секунда / RPS
		return time.Duration(float64(time.Second) / float64(s.opts.RPS))
	case s.opts.Delay > 0:
		return s.opts.Delay
	default:
		return 0
	}
}

// calculateBackoff - вычисление времени ожидания с учетом backoff
func (s *retryState) calculateBackoff(attempt int) time.Duration {
	baseWait := s.getBaseWaitTime()
	if baseWait <= 0 {
		// Если нет базовой задержки, используем минимальную
		baseWait = 100 * time.Millisecond
	}

	return baseWait * time.Duration(attempt)
}
