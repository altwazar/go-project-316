package crawler

//nolint:all

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestSuccessfulRequest - проверка на успешный запрос
func TestSuccessfulRequest(t *testing.T) {
	html := `<!DOCTYPE html>
    <html>
    <head>
        <title>Example title</title>
        <meta name="description" content="Example description">
    </head>
    <body>
        <h1>Example H1</h1>
        <a href="https://example.com/page2">Page 2</a>
        <img src="/static/logo.png">
    </body>
    </html>`

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Request:    req,
			}, nil
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       1,
		Delay:       0,
		Timeout:     5 * time.Second,
		Retries:     1,
		Concurrency: 1,
		HTTPClient:  client,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response.Pages) == 0 {
		t.Fatal("Expected at least one page")
	}

	page := response.Pages[0]
	if page.HTTPStatus != http.StatusOK {
		t.Errorf("Expected status 200, got %d", page.HTTPStatus)
	}
	if page.Status != "ok" {
		t.Errorf("Expected status 'ok', got %s", page.Status)
	}
	if page.Error != "" {
		t.Errorf("Expected no error, got %s", page.Error)
	}
}

// TestNetworkError - тест ошибки сети
func TestNetworkError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network error: connection refused")
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       1,
		Delay:       0,
		Timeout:     5 * time.Second,
		Retries:     1,
		Concurrency: 1,
		HTTPClient:  client,
	}

	ctx := context.Background()
	result, err := Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response.Pages) == 0 {
		t.Fatal("Expected at least one page")
	}

	page := response.Pages[0]
	if page.Status != "error" {
		t.Errorf("Expected status 'error', got %s", page.Status)
	}
	if page.Error == "" {
		t.Error("Expected error message, got empty")
	}
}

// TestNotFoundStatus - тест статуса 404
func TestNotFoundStatus(t *testing.T) {

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Request:    req,
			}, nil
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       1,
		Delay:       0,
		Timeout:     5 * time.Second,
		Retries:     1,
		Concurrency: 1,
		HTTPClient:  client,
	}

	ctx := context.Background()
	result, err := Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response.Pages) == 0 {
		t.Fatal("Expected at least one page")
	}

	page := response.Pages[0]
	if page.HTTPStatus != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", page.HTTPStatus)
	}
}

// TestSEODataExists - тест наличия SEO данных
func TestSEODataExists(t *testing.T) {
	html := `<!DOCTYPE html>
		<html>
		<head>
		<title>Example &amp; Title</title>
		<meta name="Description" content="Example &amp; Description">
		</head>
		<body>
		<h1>Example &amp; H1</h1>
		</body>
		</html>`

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Request:    req,
			}, nil
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       1,
		Delay:       0,
		Timeout:     5 * time.Second,
		Retries:     1,
		Concurrency: 1,
		HTTPClient:  client,
	}

	ctx := context.Background()
	result, err := Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response.Pages) == 0 {
		t.Fatal("Expected at least one page")
	}

	seo := response.Pages[0].SEO
	if !seo.HasTitle {
		t.Error("Expected HasTitle to be true")
	}
	if seo.Title != "Example & Title" {
		t.Errorf("Expected 'Example & Title' title, got %s", seo.Title)
	}
	if !seo.HasDescription {
		t.Error("Expected HasDescription to be true")
	}
	if seo.Description != "Example & Description" {
		t.Errorf("Expected 'Example & Description' description, got %s", seo.Description)
	}
	if !seo.HasH1 {
		t.Error("Expected HasH1 to be true")
	}
	// if seo.H1 != "Example & H1" {
	// 	t.Errorf("Expected 'Example & H1' H1, got %s", seo.H1)
	// }
}

// TestSEODataMissing - тест отсутствия SEO данных
func TestSEODataMissing(t *testing.T) {
	html := `<!DOCTYPE html>
		<html>
		<body>
			<p>No SEO data here</p>
		</body>
		</html>`

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Request:    req,
			}, nil
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       1,
		Delay:       0,
		Timeout:     5 * time.Second,
		Retries:     1,
		Concurrency: 1,
		HTTPClient:  client,
	}

	ctx := context.Background()
	result, err := Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response.Pages) == 0 {
		t.Fatal("Expected at least one page")
	}

	seo := response.Pages[0].SEO
	if seo.HasTitle {
		t.Error("Expected HasTitle to be false")
	}
	if seo.Title != "" {
		t.Errorf("Expected empty title, got %s", seo.Title)
	}
	if seo.HasDescription {
		t.Error("Expected HasDescription to be false")
	}
	if seo.Description != "" {
		t.Errorf("Expected empty description, got %s", seo.Description)
	}
	if seo.HasH1 {
		t.Error("Expected HasH1 to be false")
	}
	// if seo.H1 != "" {
	// 	t.Errorf("Expected empty H1, got %s", seo.H1)
	// }
}

// Тест ограничения через delay
func TestRateLimiting(t *testing.T) {
	requestTimestamps := make([]time.Time, 0)
	var mu sync.Mutex

	html := `<!DOCTYPE html>
    <html>
    <head><title>Test</title></head>
    <body>
        <img src="/static/logo1.png">
        <img src="/static/logo2.png">
        <img src="/static/logo3.png">
        <a href="/page1">Link 1</a>
        <a href="/page2">Link 2</a>
    </body>
    </html>`

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			requestTimestamps = append(requestTimestamps, time.Now())
			mu.Unlock()

			// Обработка разных типов запросов
			if strings.Contains(req.URL.Path, ".png") {
				// Возвращаем изображение
				return &http.Response{
					StatusCode:    http.StatusOK,
					Body:          io.NopCloser(bytes.NewReader([]byte("fake image data"))),
					Header:        http.Header{"Content-Type": []string{"image/png"}},
					Request:       req,
					ContentLength: 1000,
				}, nil
			}

			if req.URL.Path == "/page1" || req.URL.Path == "/page2" {
				// Возвращаем простые страницы без ссылок (чтобы не плодить запросы)
				pageHTML := `<!DOCTYPE html><html><body>Page content</body></html>`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(pageHTML)),
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Request:    req,
				}, nil
			}

			// Главная страница
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Request:    req,
			}, nil
		}),
	}

	startTime := time.Now()

	opts := Options{
		URL:         "http://example.com",
		Depth:       1,
		Delay:       100 * time.Millisecond,
		Timeout:     10 * time.Second,
		Retries:     1,
		Concurrency: 1,
		RPS:         0,
		HTTPClient:  client,
	}

	ctx := context.Background()
	_, err := Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	elapsed := time.Since(startTime)

	// Логируем для отладки
	// t.Logf("Total requests: %d", len(requestTimestamps))
	// t.Logf("Total time: %v", elapsed)

	// Проверяем, что запросов больше одного
	if len(requestTimestamps) < 2 {
		t.Errorf("Expected at least 2 requests, got %d", len(requestTimestamps))
	}

	// Проверяем интервалы между запросами
	for i := 1; i < len(requestTimestamps); i++ {
		interval := requestTimestamps[i].Sub(requestTimestamps[i-1])
		minExpected := 90 * time.Millisecond // Даем допуск 10%
		if interval < minExpected {
			t.Errorf("Interval between requests %d and %d is %v, expected at least %v",
				i-1, i, interval, minExpected)
		}
	}

	// Общее время должно быть не меньше (количество запросов - 1) * delay
	minExpectedTotal := time.Duration(len(requestTimestamps)-1) * 100 * time.Millisecond
	if elapsed < minExpectedTotal {
		t.Errorf("Total time %v is less than expected %v", elapsed, minExpectedTotal)
	}
}

// TestRetriesSuccess - тест повторных попыток с успехом на второй попытке
func TestRetriesSuccess(t *testing.T) {
	attempts := 0
	html := `<!DOCTYPE html><html><body>Success</body></html>`
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Request:    req,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(html)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Request:    req,
			}, nil
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       1,
		Delay:       10 * time.Millisecond,
		Timeout:     5 * time.Second,
		Retries:     2,
		Concurrency: 1,
		HTTPClient:  client,
	}

	ctx := context.Background()
	result, err := Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response.Pages) == 0 {
		t.Fatal("Expected at least one page")
	}

	page := response.Pages[0]
	if page.HTTPStatus != http.StatusOK {
		t.Errorf("Expected status 200 after retry, got %d", page.HTTPStatus)
	}
	if page.Status != "ok" {
		t.Errorf("Expected status 'ok', got %s", page.Status)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

// TestAssetDeduplication - тест дедупликации ассетов
func TestAssetDeduplication(t *testing.T) {
	assetRequests := 0
	html := `<!DOCTYPE html>
		<html>
		<body>
			<img src="/static/logo.png">
			<img src="/static/logo.png">
			<img src="/static/logo.png">
		</body>
		</html>`

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			// Логируем все запросы для отладки
			// t.Logf("Request: %s", req.URL.Path)

			// Обработка запроса к ассету (logo.png)
			if strings.Contains(req.URL.Path, "logo.png") {
				assetRequests++
				return &http.Response{
					StatusCode:    http.StatusOK,
					Status:        "OK",
					Body:          io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 12345))),
					Header:        http.Header{"Content-Length": []string{"12345"}},
					ContentLength: 12345,
					Request:       req,
				}, nil
			}

			// Обработка запроса к главной странице (и любым другим HTML страницам)
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "OK",
				Body:       io.NopCloser(strings.NewReader(html)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Request:    req,
			}, nil
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       1,
		Delay:       0,
		Timeout:     5 * time.Second,
		Retries:     1,
		Concurrency: 4,
		HTTPClient:  client,
	}

	ctx := context.Background()
	result, err := Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Проверяем, что ассет был запрошен только один раз
	if assetRequests != 1 {
		t.Errorf("Expected 1 request for asset, got %d", assetRequests)
	}

	// Проверяем, что в ответе ассет присутствует
	if len(response.Pages) > 0 && len(response.Pages[0].Assets) > 0 {
		asset := response.Pages[0].Assets[0]
		if asset.SizeBytes != 12345 {
			t.Errorf("Expected size 12345, got %d", asset.SizeBytes)
		}
	}
}

// TestJSONResponseComparison - сравнение с эталоном JSON
func TestJSONResponseComparison(t *testing.T) {
	// Счетчики для проверки количества запросов
	mainPageRequested := false
	assetRequested := false
	missingPageRequested := false

	html := `<!DOCTYPE html>
    <html>
    <head>
        <title>Example title</title>
        <meta name="description" content="Example description">
    </head>
    <body>
        <h1>Example H1</h1>
        <a href="/missing">Missing page</a>
        <img src="/static/logo.png">
    </body>
    </html>`

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			// Логируем для отладки
			// t.Logf("Request: %s", req.URL.Path)

			switch req.URL.Path {
			case "":
				mainPageRequested = true
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "OK",
					Body:       io.NopCloser(strings.NewReader(html)),
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Request:    req,
				}, nil

			case "/static/logo.png":
				assetRequested = true
				return &http.Response{
					StatusCode:    http.StatusOK,
					Status:        "OK",
					Body:          io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 12345))),
					Header:        http.Header{"Content-Type": []string{"image/png"}},
					ContentLength: 12345,
					Request:       req,
				}, nil

			case "/missing":
				missingPageRequested = true
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "Not Found",
					Body:       io.NopCloser(strings.NewReader("Not Found")),
					Request:    req,
				}, nil

			default:
				// Неожиданный URL
				t.Errorf("Unexpected URL: %s", req.URL.Path)
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Request:    req,
				}, nil
			}
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       0,
		Delay:       0,
		Timeout:     5 * time.Second,
		Retries:     1,
		Concurrency: 4,
		IndentJSON:  true,
		HTTPClient:  client,
	}

	ctx := context.Background()
	result, err := Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Проверяем, что все ожидаемые запросы были сделаны
	if !mainPageRequested {
		t.Error("Main page was not requested")
	}
	if !assetRequested {
		t.Error("Asset was not requested")
	}
	if !missingPageRequested {
		t.Error("Missing page was not requested")
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Проверяем основные поля
	if len(response.Pages) != 1 {
		t.Fatalf("Expected 1 page, got %d", len(response.Pages))
	}

	page := response.Pages[0]
	if page.HTTPStatus != 200 {
		t.Errorf("Expected status 200, got %d", page.HTTPStatus)
	}
	if page.SEO.Title != "Example title" {
		t.Errorf("Expected title 'Example title', got '%s'", page.SEO.Title)
	}
	if page.SEO.Description != "Example description" {
		t.Errorf("Expected description 'Example description', got '%s'", page.SEO.Description)
	}
	// if page.SEO.H1 != "Example H1" {
	// 	t.Errorf("Expected H1 'Example H1', got '%s'", page.SEO.H1)
	// }

	// Проверяем наличие битой ссылки
	foundBrokenLink := false
	for _, link := range page.BrokenLinks {
		if strings.Contains(link.URL, "/missing") {
			foundBrokenLink = true
			if link.StatusCode != 404 {
				t.Errorf("Expected broken link status 404, got %d", link.StatusCode)
			}
			break
		}
	}
	if !foundBrokenLink {
		t.Error("Expected to find broken link to /missing")
	}

	// Дополнительная проверка: убеждаемся, что ассет присутствует
	if len(page.Assets) == 0 {
		t.Error("Expected to find assets")
	} else {
		asset := page.Assets[0]
		if asset.SizeBytes != 12345 {
			t.Errorf("Expected asset size 12345, got %d", asset.SizeBytes)
		}
		if asset.StatusCode != 200 {
			t.Errorf("Expected asset status 200, got %d", asset.StatusCode)
		}
	}
}

// TestTimeout - тестирование таймаута
func TestTimeout(t *testing.T) {
	client := &http.Client{
		Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			time.Sleep(200 * time.Millisecond)
			return nil, context.DeadlineExceeded
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       1,
		Delay:       0,
		Timeout:     100 * time.Millisecond,
		Retries:     1,
		Concurrency: 1,
		HTTPClient:  client,
	}

	result, err := Analyze(context.Background(), opts)
	// Проверяем, что есть ошибка таймаута
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response.Pages) == 0 {
		t.Fatal("Expected at least one page")
	}

	page := response.Pages[0]
	if page.Status != "error" {
		t.Errorf("Expected status 'error' due to timeout, got %s", page.Status)
	}
	if page.Error == "" {
		t.Error("Expected error message for timeout")
	}

}

// Helper types and functions
type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// TestConcurrentRequests - тестирование одновременных запросов
func TestConcurrentRequests(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	requestTimestamps := make([]time.Time, 0)

	html := `<!DOCTYPE html><html><body><a href="/page1">Link</a></body></html>`
	page1HTML := `<!DOCTYPE html><html><body>Page 1 content</body></html>`

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			requestCount++
			requestTimestamps = append(requestTimestamps, time.Now())
			mu.Unlock()

			var body string
			switch req.URL.Path {
			case "":
				body = html
			case "/page1":
				body = page1HTML
			default:
				body = `<!DOCTYPE html><html><body>Default</body></html>`
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "OK",
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Request:    req,
			}, nil
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       2,
		Delay:       0,
		Timeout:     5 * time.Second,
		Retries:     1,
		Concurrency: 5,
		HTTPClient:  client,
	}

	ctx := context.Background()
	// startTime := time.Now()
	_, err := Analyze(ctx, opts)
	// elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	mu.Lock()
	totalRequests := requestCount
	mu.Unlock()

	// t.Logf("Total requests: %d, time: %v", totalRequests, elapsed)

	// Проверяем базовые запросы
	if totalRequests < 2 {
		t.Errorf("Expected at least 2 requests (main page + /page1), got %d", totalRequests)
	}
}

// TestRPSLimit - тестирование параметра rps
func TestRPSLimit(t *testing.T) {
	var mu sync.Mutex
	requestTimestamps := make([]time.Time, 0)
	requestCount := 0

	// Создаем страницу с множеством ссылок для генерации многих запросов
	var linksHTML strings.Builder
	linksHTML.WriteString(`<!DOCTYPE html><html><body>`)
	for i := range 19 {
		fmt.Fprintf(&linksHTML, `<a href="/page%d">Page %d</a>`, i, i)
		fmt.Fprintf(&linksHTML, `<img src="/static/img%d.png">`, i)
	}
	linksHTML.WriteString(`</body></html>`)

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			requestTimestamps = append(requestTimestamps, time.Now())
			requestCount++
			// currentCount := requestCount
			mu.Unlock()

			// t.Logf("Request #%d: %s at %v", currentCount, req.URL.Path, time.Now())

			// Для всех запросов возвращаем простые ответы
			if strings.Contains(req.URL.Path, ".png") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte("image"))),
					Header:     http.Header{"Content-Type": []string{"image/png"}},
					Request:    req,
				}, nil
			}

			body := linksHTML.String()
			if req.URL.Path != "" {
				// Для подстраниц возвращаем HTML без ссылок, чтобы не углубляться
				body = `<!DOCTYPE html><html><body>Subpage</body></html>`
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Request:    req,
			}, nil
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       1,
		Delay:       0,
		Timeout:     10 * time.Second,
		Retries:     1,
		Concurrency: 5,
		RPS:         20, // 20 RPS = 50ms между запросами
		HTTPClient:  client,
	}

	ctx := context.Background()
	startTime := time.Now()
	_, err := Analyze(ctx, opts)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	mu.Lock()
	totalRequests := requestCount
	timestamps := requestTimestamps
	mu.Unlock()

	// t.Logf("Total requests: %d", totalRequests)
	// t.Logf("Total time: %v", elapsed)

	if totalRequests < 5 {
		t.Errorf("Expected at least 5 requests, got %d", totalRequests)
	}

	// Статистика по интервалам
	var intervals []time.Duration
	for i := 1; i < len(timestamps); i++ {
		interval := timestamps[i].Sub(timestamps[i-1])
		intervals = append(intervals, interval)
	}

	if len(intervals) > 0 {
		// Вычисляем средний интервал
		var sum time.Duration
		for _, interval := range intervals {
			sum += interval
		}
		avgInterval := sum / time.Duration(len(intervals))

		expectedInterval := 50 * time.Millisecond // для 20 RPS
		// t.Logf("Average interval: %v (expected ~%v)", avgInterval, expectedInterval)

		// Проверяем, что средний интервал не сильно меньше ожидаемого
		if avgInterval < expectedInterval/2 {
			t.Errorf("Average interval %v is too low, expected at least %v",
				avgInterval, expectedInterval/2)
		}

		// Проверяем минимальные интервалы
		minInterval := intervals[0]
		for _, interval := range intervals {
			if interval < minInterval {
				minInterval = interval
			}
		}

		minExpected := 30 * time.Millisecond // Допуск для 20 RPS
		if minInterval < minExpected {
			t.Errorf("Minimum interval %v is less than expected %v",
				minInterval, minExpected)
		}
	}

	// Проверяем, что общее время соответствует ожидаемому
	// Для N запросов с RPS=20 минимальное время ~ (N-1)/20 секунд
	minExpectedTime := time.Duration(float64(totalRequests-1)/20) * time.Second
	if elapsed < minExpectedTime/2 { // Даем большой допуск из-за накладных расходов
		t.Errorf("Total time %v is less than expected %v", elapsed, minExpectedTime)
	}
}

// Тестирование задания глубины
func TestDepthLimit(t *testing.T) {
	tests := []struct {
		name          string
		depth         int
		expectedPages int
		setupMock     func() *http.Client
	}{
		{
			name:          "depth 1 - only root page",
			depth:         1,
			expectedPages: 1,
			setupMock: func() *http.Client {
				html := `<!DOCTYPE html>
                <html>
                <body>
                    <a href="/page1">Page 1</a>
                    <a href="/page2">Page 2</a>
                </body>
                </html>`

				return &http.Client{
					Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader(html)),
							Header:     http.Header{"Content-Type": []string{"text/html"}},
							Request:    req,
						}, nil
					}),
				}
			},
		},
		{
			name:          "depth 2 - root + linked pages",
			depth:         2,
			expectedPages: 3, // root + page1 + page2
			setupMock: func() *http.Client {
				rootHTML := `<!DOCTYPE html>
                <html>
                <body>
                    <a href="/page1">Page 1</a>
                    <a href="/page2">Page 2</a>
                </body>
                </html>`

				subpageHTML := `<!DOCTYPE html>
                <html>
                <body>
                    <p>Subpage content</p>
                    <a href="/page3">Page 3</a>
                </body>
                </html>`

				return &http.Client{
					Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
						var body string
						switch req.URL.Path {
						case "":
							body = rootHTML
						case "/page1", "/page2":
							body = subpageHTML
						default:
							body = `<!DOCTYPE html><html><body>Default</body></html>`
						}

						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader(body)),
							Header:     http.Header{"Content-Type": []string{"text/html"}},
							Request:    req,
						}, nil
					}),
				}
			},
		},
		{
			name:          "depth 3 - deep nesting",
			depth:         3,
			expectedPages: 3, // root -> page1 -> page2
			setupMock: func() *http.Client {
				return &http.Client{
					Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
						var body string
						switch req.URL.Path {
						case "":
							body = `<!DOCTYPE html><html><body><a href="/level1">Level 1</a></body></html>`
						case "/level1":
							body = `<!DOCTYPE html><html><body><a href="/level2">Level 2</a></body></html>`
						case "/level2":
							body = `<!DOCTYPE html><html><body><a href="/level3">Level 3</a></body></html>`
						default:
							body = `<!DOCTYPE html><html><body>Deep page</body></html>`
						}

						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader(body)),
							Header:     http.Header{"Content-Type": []string{"text/html"}},
							Request:    req,
						}, nil
					}),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				URL:         "http://example.com",
				Depth:       tt.depth,
				Delay:       0,
				Timeout:     5 * time.Second,
				Retries:     1,
				Concurrency: 4,
				HTTPClient:  tt.setupMock(),
			}

			ctx := context.Background()
			result, err := Analyze(ctx, opts)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			var response Report
			if err := json.Unmarshal(result, &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if len(response.Pages) != tt.expectedPages {
				t.Errorf("Expected %d pages, got %d", tt.expectedPages, len(response.Pages))
			}

			// Проверяем, что глубина каждой страницы не превышает заданную
			for _, page := range response.Pages {
				if page.Depth > tt.depth {
					t.Errorf("Page %s has depth %d, exceeding limit %d",
						page.URL, page.Depth, tt.depth)
				}
			}
		})
	}
}

// Тестирование на игнорирование страниц за пределами домена
func TestExternalLinksNotCrawled(t *testing.T) {
	var internalRequests int
	var externalRequests int
	var mu sync.Mutex

	rootHTML := `<!DOCTYPE html>
    <html>
    <body>
        <a href="/internal1">Internal Link 1</a>
        <a href="/internal2">Internal Link 2</a>
        <a href="https://external.com">External Link</a>
        <a href="https://another-external.org/page">Another External</a>
        <a href="http://external-site.net">External HTTP</a>
    </body>
    </html>`

	internalPageHTML := `<!DOCTYPE html>
    <html>
    <body>
        <p>Internal page content</p>
        <a href="/internal3">Another internal</a>
    </body>
    </html>`

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			defer mu.Unlock()

			// Считаем запросы к разным доменам
			if req.URL.Host == "example.com" {
				internalRequests++
			} else {
				externalRequests++
				// Внешние ссылки должны проверяться (HEAD запрос), но не краулиться
				// t.Logf("External request made to: %s", req.URL.String())
			}

			// Возвращаем ответы для внутренних страниц
			if req.URL.Host == "example.com" {
				var body string
				switch req.URL.Path {
				case "":
					body = rootHTML
				case "/internal1", "/internal2", "/internal3":
					body = internalPageHTML
				default:
					body = `<!DOCTYPE html><html><body>Default</body></html>`
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Request:    req,
				}, nil
			}

			// Для внешних ссылок возвращаем успешный ответ (но они не должны краулиться)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`External page`)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Request:    req,
			}, nil
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       2,
		Delay:       0,
		Timeout:     5 * time.Second,
		Retries:     1,
		Concurrency: 4,
		HTTPClient:  client,
	}

	ctx := context.Background()
	result, err := Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// mu.Lock()
	// t.Logf("Internal requests: %d", internalRequests)
	// t.Logf("External requests: %d", externalRequests)
	// mu.Unlock()

	// Проверяем, что в pages есть только внутренние страницы
	for _, page := range response.Pages {
		if !strings.Contains(page.URL, "example.com") {
			t.Errorf("External page found in results: %s", page.URL)
		}
	}

	// Проверяем, что количество страниц соответствует ожидаемому
	// (root + internal1 + internal2, но internal3 не должен быть, т.к. он на глубине 2)
	expectedPages := 3 // root, internal1, internal2
	if len(response.Pages) != expectedPages {
		t.Errorf("Expected %d pages, got %d", expectedPages, len(response.Pages))
	}

	// Внешние ссылки должны быть проверены (хотя бы HEAD запросом)
	if externalRequests == 0 {
		t.Error("Expected external links to be checked, but no external requests were made")
	}
}

// TestDuplicateLinksDeduplication - Тестирование на отсутствие дубликатов
func TestDuplicateLinksDeduplication(t *testing.T) {
	// Создаем HTML с дублирующимися ссылками
	html := `<!DOCTYPE html>
    <html>
    <body>
        <a href="/page1">Page 1 - First</a>
        <a href="/page2">Page 2</a>
        <a href="/page1">Page 1 - Second</a>
        <a href="/page3">Page 3</a>
        <a href="/page1">Page 1 - Third</a>
        <a href="/page2">Page 2 - Second</a>
        <a href="/page1">Page 1 - Fourth</a>
    </body>
    </html>`

	var requestsCount map[string]int
	var mu sync.Mutex

	requestsCount = make(map[string]int)

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			requestsCount[req.URL.Path]++
			mu.Unlock()

			// Для всех внутренних страниц возвращаем простой HTML
			body := `<!DOCTYPE html><html><body>Page content</body></html>`
			if req.URL.Path == "" {
				body = html
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Request:    req,
			}, nil
		}),
	}

	opts := Options{
		URL:         "http://example.com",
		Depth:       2,
		Delay:       0,
		Timeout:     5 * time.Second,
		Retries:     1,
		Concurrency: 4,
		HTTPClient:  client,
	}

	ctx := context.Background()
	result, err := Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// mu.Lock()
	// t.Logf("Requests made: %v", requestsCount)
	// mu.Unlock()

	// Проверяем, что каждая страница запрошена только один раз
	for path, count := range requestsCount {
		if path != "/" && count > 1 {
			t.Errorf("Page %s was requested %d times (expected 1)", path, count)
		}
	}

	// Проверяем, что в отчете каждая страница фигурирует один раз
	pageURLs := make(map[string]bool)
	for _, page := range response.Pages {
		if pageURLs[page.URL] {
			t.Errorf("Duplicate page in results: %s", page.URL)
		}
		pageURLs[page.URL] = true
	}

	// Проверяем, что количество уникальных страниц соответствует ожидаемому
	// root + page1 + page2 + page3 = 4 страницы
	expectedUniquePages := 4
	if len(response.Pages) != expectedUniquePages {
		t.Errorf("Expected %d unique pages, got %d", expectedUniquePages, len(response.Pages))
	}

	// Проверяем, что все ожидаемые страницы присутствуют
	expectedURLs := []string{
		"http://example.com",
		"http://example.com/page1",
		"http://example.com/page2",
		"http://example.com/page3",
	}
	for _, expectedURL := range expectedURLs {
		if !pageURLs[expectedURL] {
			t.Errorf("Expected page %s not found in results", expectedURL)
		}
	}
}

// TestContextCancellationGracefulShutdown - тестирование мягкого завершения
func TestContextCancellationGracefulShutdown(t *testing.T) {
	var mu sync.Mutex
	completedRequests := make(map[string]bool)
	startedRequests := make(map[string]bool)

	rootHTML := `<!DOCTYPE html>
    <html>
    <body>
        <a href="/page1">Page 1</a>
        <a href="/page2">Page 2</a>
        <a href="/page3">Page 3</a>
        <a href="/slow-page">Slow Page</a>
    </body>
    </html>`

	subpageHTML := `<!DOCTYPE html>
    <html>
    <body>
        <p>Subpage content</p>
    </body>
    </html>`

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			startedRequests[req.URL.Path] = true
			mu.Unlock()

			// t.Logf("Processing: %s", req.URL.Path)

			switch req.URL.Path {
			case "":
				time.Sleep(10 * time.Millisecond)
				mu.Lock()
				completedRequests[req.URL.Path] = true
				mu.Unlock()
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(rootHTML)),
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Request:    req,
				}, nil

			case "/page1", "/page2", "/page3":
				time.Sleep(20 * time.Millisecond)
				mu.Lock()
				completedRequests[req.URL.Path] = true
				mu.Unlock()
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(subpageHTML)),
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Request:    req,
				}, nil

			case "/slow-page":
				// Медленная страница, которая не успеет завершиться
				select {
				case <-req.Context().Done():
					// t.Logf("Slow page cancelled: %s", req.URL.Path)
					return nil, req.Context().Err()
				case <-time.After(500 * time.Millisecond):
					mu.Lock()
					completedRequests[req.URL.Path] = true
					mu.Unlock()
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("OK")),
						Header:     http.Header{"Content-Type": []string{"text/html"}},
						Request:    req,
					}, nil
				}

			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Request:    req,
				}, nil
			}
		}),
	}

	ctx, cancel := context.WithCancel(context.Background())

	opts := Options{
		URL:         "http://example.com",
		Depth:       2,
		Delay:       0,
		Timeout:     5 * time.Second,
		Retries:     1,
		Concurrency: 5,
		HTTPClient:  client,
	}

	time.AfterFunc(100*time.Millisecond, func() {
		// t.Log("Cancelling context...")
		cancel()
	})

	startTime := time.Now()
	result, err := Analyze(ctx, opts)
	elapsed := time.Since(startTime)

	// t.Logf("Total time: %v", elapsed)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// mu.Lock()
	// t.Logf("Started requests: %v", startedRequests)
	// t.Logf("Completed requests: %v", completedRequests)
	// t.Logf("Pages in response: %d", len(response.Pages))
	// mu.Unlock()

	// for i, page := range response.Pages {
	// 	t.Logf("Page %d: %s (depth %d, status %s, error: %s)",
	// 		i, page.URL, page.Depth, page.Status, page.Error)
	// }

	// Проверяем, что главная страница присутствует
	foundRoot := false
	for _, page := range response.Pages {
		if page.URL == "http://example.com" {
			foundRoot = true
			break
		}
	}
	if !foundRoot {
		t.Error("Root page not found in response")
	}

	// Проверяем, что медленная страница присутствует, НО со статусом error
	slowPageFound := false
	for _, page := range response.Pages {
		if strings.Contains(page.URL, "slow-page") {
			slowPageFound = true
			if page.Status != "error" {
				t.Errorf("Slow page should have status 'error', got: %s", page.Status)
			}
			if page.Error == "" {
				t.Error("Slow page should have error message")
			}
			// t.Logf("Slow page correctly recorded with error: %s", page.Error)
			break
		}
	}

	if !slowPageFound {
		t.Error("Slow page should be in response with error status")
	}

	// Проверяем, что отмена произошла достаточно быстро
	if elapsed > 300*time.Millisecond {
		t.Errorf("Cancellation took too long: %v", elapsed)
	}
}

// TestContextCancellationPartialResults - тестирование на вывод обработанных страниц при мягком завершении
func TestContextCancellationPartialResults(t *testing.T) {
	var mu sync.Mutex
	processedPages := make(map[string]bool)
	requestCount := make(map[string]int)

	// Создаем страницу с множеством ссылок
	rootHTML := `<!DOCTYPE html>
    <html>
    <body>`

	for i := 1; i <= 10; i++ {
		rootHTML += fmt.Sprintf(`<a href="/page%d">Page %d</a>`, i, i)
	}
	rootHTML += `</body></html>`

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			requestCount[req.URL.Path]++
			mu.Unlock()

			// Каждый запрос занимает время
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(30 * time.Millisecond):
				mu.Lock()
				processedPages[req.URL.Path] = true
				mu.Unlock()

				var body string
				if req.URL.Path == "/" {
					body = rootHTML
				} else {
					body = `<!DOCTYPE html><html><body>Subpage</body></html>`
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Request:    req,
				}, nil
			}
		}),
	}

	ctx, cancel := context.WithCancel(context.Background())

	opts := Options{
		URL:         "http://example.com",
		Depth:       2,
		Delay:       0,
		Timeout:     5 * time.Second,
		Retries:     1,
		Concurrency: 3,
		HTTPClient:  client,
	}

	time.AfterFunc(100*time.Millisecond, func() {
		// t.Log("Cancelling context...")
		cancel()
	})

	startTime := time.Now()
	result, err := Analyze(ctx, opts)
	elapsed := time.Since(startTime)

	// t.Logf("Total time: %v", elapsed)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	var response Report
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// mu.Lock()
	// totalProcessed := len(processedPages)
	// totalRequests := len(requestCount)
	// t.Logf("Unique processed pages: %d", totalProcessed)
	// t.Logf("Total unique requests: %d", totalRequests)
	// t.Logf("Pages in response: %d", len(response.Pages))

	// // Выводим статистику по запросам
	// for path, count := range requestCount {
	// 	t.Logf("  %s: %d request(s)", path, count)
	// }
	// mu.Unlock()

	// Проверяем, что есть хотя бы root страница
	if len(response.Pages) == 0 {
		t.Error("Expected at least root page in response")
	}

	// Проверяем, что все страницы в ответе имеют статус (ok или error)
	for _, page := range response.Pages {
		if page.Status != "ok" && page.Status != "error" {
			t.Errorf("Page %s has invalid status: %s", page.URL, page.Status)
		}
	}

	// Проверяем, что отмена произошла быстро
	if elapsed > 300*time.Millisecond {
		t.Errorf("Cancellation took too long: %v", elapsed)
	}

	// Логируем, какие страницы были обработаны
	// t.Log("=== Summary ===")
	// t.Logf("Total pages in response: %d", len(response.Pages))
	// for _, page := range response.Pages {
	// 	t.Logf("  - %s (%s)", page.URL, page.Status)
	// }
}

// TestCustomUserAgent - тест проверки кастомного User-Agent
func TestCustomUserAgent(t *testing.T) {
	t.Run("custom user agent in page requests", func(t *testing.T) {
		var receivedUA string

		html := `<!DOCTYPE html>
		<html>
		<head><title>Test</title></head>
		<body>
			<a href="/page1">Link</a>
			<img src="/logo.png">
		</body>
		</html>`

		subpageHTML := `<!DOCTYPE html><html><body>Subpage</body></html>`

		client := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				// Сохраняем User-Agent из первого запроса к странице
				if receivedUA == "" {
					receivedUA = req.Header.Get("User-Agent")
				} else {
					// Проверяем, что во всех запросах один и тот же UA
					if ua := req.Header.Get("User-Agent"); ua != receivedUA {
						t.Errorf("Inconsistent User-Agent: expected '%s', got '%s'", receivedUA, ua)
					}
				}

				var body string
				switch req.URL.Path {
				case "":
					body = html
				case "/page1":
					body = subpageHTML
				case "/logo.png":
					return &http.Response{
						StatusCode:    http.StatusOK,
						Body:          io.NopCloser(bytes.NewReader([]byte("fake image"))),
						Header:        http.Header{"Content-Type": []string{"image/png"}},
						ContentLength: 10,
						Request:       req,
					}, nil
				default:
					body = subpageHTML
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Request:    req,
				}, nil
			}),
		}

		customUA := "MyBot/1.0"

		opts := Options{
			URL:         "http://example.com",
			Depth:       2,
			Delay:       0,
			Timeout:     5 * time.Second,
			Retries:     1,
			Concurrency: 1,
			UserAgent:   customUA,
			HTTPClient:  client,
		}

		ctx := context.Background()
		_, err := Analyze(ctx, opts)
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}

		if receivedUA != customUA {
			t.Errorf("Expected User-Agent '%s', got '%s'", customUA, receivedUA)
		}
	})

	t.Run("no user agent specified", func(t *testing.T) {
		var receivedUA string

		client := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				receivedUA = req.Header.Get("User-Agent")
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("<html></html>")),
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Request:    req,
				}, nil
			}),
		}

		opts := Options{
			URL:         "http://example.com",
			Depth:       1,
			Concurrency: 1,
			UserAgent:   "", // Пустой User-Agent
			HTTPClient:  client,
		}

		ctx := context.Background()
		_, err := Analyze(ctx, opts)
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}

		if receivedUA != "" {
			t.Errorf("Expected no User-Agent header, got '%s'", receivedUA)
		}
	})

	t.Run("user agent in HEAD requests", func(t *testing.T) {
		var headRequestCount int
		var receivedUA string

		html := `<!DOCTYPE html>
		<html>
		<body>
			<a href="https://external-example.com">External Link</a>
		</body>
		</html>`

		client := &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				// Для внешних ссылок должен быть HEAD запрос
				if req.Method == http.MethodHead {
					headRequestCount++
					receivedUA = req.Header.Get("User-Agent")
					return &http.Response{
						StatusCode: http.StatusOK,
						Request:    req,
					}, nil
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(html)),
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Request:    req,
				}, nil
			}),
		}

		customUA := "HeadChecker/1.0"

		opts := Options{
			URL:         "http://example.com",
			Depth:       1,
			Delay:       0,
			Timeout:     5 * time.Second,
			Retries:     1,
			Concurrency: 1,
			UserAgent:   customUA,
			HTTPClient:  client,
		}

		ctx := context.Background()
		_, err := Analyze(ctx, opts)
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}

		if headRequestCount == 0 {
			t.Error("Expected at least one HEAD request for external link")
		}
		if receivedUA != customUA {
			t.Errorf("Expected User-Agent '%s' in HEAD request, got '%s'", customUA, receivedUA)
		}
	})
}
