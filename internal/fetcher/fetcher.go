// Package fetcher - логика http запросов
package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code/internal/models"
	"code/internal/parser"
	"code/internal/urlutil"
)

// FetchPageWithRetries - получение страницы с повторными попытками
func FetchPageWithRetries(ctx context.Context, rawURL string, depth int, opts models.Options) models.Page {
	normalizedURL, _ := urlutil.NormalizeURL(rawURL)
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
	return newPageResponse(0, normalizedURL, depth, nil, models.SEOData{}, []models.Asset{}, state.getLastErrorMsg())
}

// CheckAsset - проверка ассета и получение его размера
func CheckAsset(ctx context.Context, urlStr string, assetType models.AssetType, client *http.Client, userAgent string) (models.Asset, error) {
	asset := createBaseAsset(urlStr, assetType)
	validatedURL, err := urlutil.ValidateAndNormalizeURL(urlStr)
	if err != nil {
		asset.Error = err.Error()
		return asset, err
	}
	statusCode, contentLength, err := getAssetInfo(ctx, validatedURL, client, userAgent)
	if err != nil {
		asset.Error = fmt.Sprintf("request failed: %v", err)
		return asset, err
	}
	asset.StatusCode = statusCode
	if statusCode >= 200 && statusCode < 300 {
		asset.SizeBytes = contentLength
	} else {
		asset.Error = fmt.Sprintf("HTTP %d", statusCode)
	}
	return asset, nil
}

// ------------------------------------------------------------
// Вспомогательные типы и функции для getPageWithLinks
// ------------------------------------------------------------

type pageProcessor struct {
	originalURL string
	httpClient  *http.Client
	resp        *http.Response
	finalURL    string
	statusCode  int
	links       []string
	assets      []models.Asset
	seo         models.SEOData
	htmlBody    string
}

func getPageWithLinks(ctx context.Context, urlStr string, httpClient *http.Client, userAgent string) (string, int, []string, models.SEOData, []models.Asset, error) {
	proc := &pageProcessor{
		originalURL: urlStr,
		httpClient:  httpClient,
	}
	if err := proc.fetchPage(ctx, urlStr, userAgent); err != nil {
		return urlStr, 0, nil, models.SEOData{}, []models.Asset{}, err
	}
	defer proc.closeResponse()

	if !proc.isHTML() {
		if proc.isXML() {
			return proc.processXML()
		}
		return proc.finalURL, proc.statusCode, nil, models.SEOData{}, []models.Asset{}, nil
	}
	if err := proc.processHTML(); err != nil {
		return proc.finalURL, proc.statusCode, nil, models.SEOData{}, []models.Asset{}, err
	}
	return proc.finalURL, proc.statusCode, proc.links, proc.seo, proc.assets, nil
}

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
	p.finalURL, _ = urlutil.NormalizeURL(resp.Request.URL.String())
	p.statusCode = resp.StatusCode
	return nil
}

func (p *pageProcessor) isXML() bool {
	contentType := p.resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "text/xml") ||
		strings.Contains(contentType, "application/rss+xml")
}

func (p *pageProcessor) processXML() (string, int, []string, models.SEOData, []models.Asset, error) {
	if err := p.readXMLBody(); err != nil {
		return p.finalURL, p.statusCode, nil, models.SEOData{}, []models.Asset{}, err
	}
	seo := parser.ExtractSEOFromXML(p.htmlBody)
	return p.finalURL, p.statusCode, nil, seo, []models.Asset{}, nil
}

func (p *pageProcessor) readXMLBody() error {
	limitedReader := io.LimitReader(p.resp.Body, models.MaxHTMLBodySize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	p.htmlBody = string(bodyBytes)
	return nil
}

func (p *pageProcessor) closeResponse() {
	if p.resp != nil {
		_ = p.resp.Body.Close()
	}
}

func (p *pageProcessor) isHTML() bool {
	contentType := p.resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "text/html")
}

func (p *pageProcessor) processHTML() error {
	if err := p.readHTMLBody(); err != nil {
		return err
	}
	baseURL, err := url.Parse(p.originalURL)
	if err != nil {
		return fmt.Errorf("failed to parse base URL: %w", err)
	}
	links, assets, err := parser.ExtractLinksAndAssets(p.htmlBody, baseURL)
	if err != nil {
		return fmt.Errorf("failed to extract links and assets: %w", err)
	}
	p.links = links
	p.assets = assets
	p.seo = parser.ExtractSEOData(p.htmlBody)
	return nil
}

func (p *pageProcessor) readHTMLBody() error {
	limitedReader := io.LimitReader(p.resp.Body, models.MaxHTMLBodySize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	p.htmlBody = string(bodyBytes)
	return nil
}

// ------------------------------------------------------------
// Asset helpers
// ------------------------------------------------------------

func createBaseAsset(urlStr string, assetType models.AssetType) models.Asset {
	return models.Asset{
		URL:  urlStr,
		Type: assetType,
	}
}

func getAssetInfo(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, int64, error) {
	statusCode, contentLength, err := doHeadRequestForAsset(ctx, urlStr, client, userAgent)
	if err == nil {
		if statusCode == http.StatusMethodNotAllowed || statusCode == http.StatusNotImplemented {
			return doGetRequestForAsset(ctx, urlStr, client, userAgent)
		}
		return statusCode, contentLength, nil
	}
	return doGetRequestForAsset(ctx, urlStr, client, userAgent)
}

func doHeadRequestForAsset(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, int64, error) {
	req, err := newRequestWithUserAgent(ctx, http.MethodHead, urlStr, userAgent)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create HEAD request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, resp.ContentLength, nil
}

func doGetRequestForAsset(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, int64, error) {
	req, err := newRequestWithUserAgent(ctx, http.MethodGet, urlStr, userAgent)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create GET request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	size, err := getContentSize(resp)
	if err != nil {
		return resp.StatusCode, 0, err
	}
	return resp.StatusCode, size, nil
}

func getContentSize(resp *http.Response) (int64, error) {
	if resp.ContentLength >= 0 {
		return resp.ContentLength, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, models.MaxAssetSize))
	if err != nil {
		return 0, fmt.Errorf("failed to read body: %w", err)
	}
	return int64(len(body)), nil
}

// ------------------------------------------------------------
// Retry logic
// ------------------------------------------------------------

type retryState struct {
	ctx     context.Context
	url     string
	depth   int
	opts    models.Options
	lastErr error
}

func (s *retryState) tryAttempt(attempt int) *models.Page {
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

func (s *retryState) isSuccessAndNotRetriable(statusCode int, err error) bool {
	return err == nil && statusCode > 0 && !isRetriable(statusCode, err)
}

func (s *retryState) isContextCancelled() bool {
	select {
	case <-s.ctx.Done():
		return true
	default:
		return false
	}
}

func (s *retryState) createCancelledPage() *models.Page {
	page := newPageResponse(0, s.url, s.depth, nil, models.SEOData{}, []models.Asset{}, s.ctx.Err().Error())
	return &page
}

func (s *retryState) createSuccessPage(finalURL string, statusCode int, links []string, seoData models.SEOData, assets []models.Asset) *models.Page {
	page := newPageResponse(statusCode, finalURL, s.depth, links, seoData, assets, "")
	return &page
}

func (s *retryState) updateLastError(statusCode int, err error) {
	if err != nil {
		s.lastErr = err
	} else if isRetriable(statusCode, nil) {
		s.lastErr = fmt.Errorf("HTTP %d (retriable)", statusCode)
	}
}

func (s *retryState) shouldRetry(attempt int, statusCode int, err error) bool {
	if attempt >= s.opts.Retries {
		return false
	}
	return isRetriable(statusCode, err)
}

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

func (s *retryState) getBaseWaitTime() time.Duration {
	switch {
	case s.opts.RPS > 0:
		return time.Duration(float64(time.Second) / float64(s.opts.RPS))
	case s.opts.Delay > 0:
		return s.opts.Delay
	default:
		return 0
	}
}

func (s *retryState) calculateBackoff(attempt int) time.Duration {
	baseWait := s.getBaseWaitTime()
	if baseWait <= 0 {
		baseWait = 100 * time.Millisecond
	}
	return baseWait * time.Duration(attempt)
}

func (s *retryState) getLastErrorMsg() string {
	if s.lastErr != nil {
		return s.lastErr.Error()
	}
	return ""
}

// ------------------------------------------------------------
// Общие вспомогательные функции
// ------------------------------------------------------------

func isRetriable(statusCode int, err error) bool {
	if err != nil {
		return true
	}
	return statusCode == 429 || statusCode >= 500
}

func newPageResponse(statusCode int, url string, depth int, links []string, seo models.SEOData, assets []models.Asset, errMsg string) models.Page {
	status := getStatusString(statusCode)
	page := models.Page{
		URL:          url,
		Depth:        depth,
		HTTPStatus:   statusCode,
		Status:       status,
		BrokenLinks:  []models.LinkStatus{},
		Assets:       assets,
		Links:        links,
		DiscoveredAt: time.Now().UTC().Truncate(time.Second),
		Error:        errMsg,
		SEO:          seo,
	}
	if status != models.StatusOK {
		page.BrokenLinks = nil
	}
	return page
}

func getStatusString(statusCode int) string {
	switch statusCode {
	case 200, 201, 202, 204:
		return models.StatusOK
	default:
		return models.StatusError
	}
}
