package code

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Options структура с настройками анализатора
type Options struct {
	URL         string
	Depth       int
	Retries     int
	Delay       time.Duration
	Timeout     time.Duration
	UserAgent   string
	Concurrency int
	IndentJSON  int
	HTTPClient  *http.Client
}

type AnalyzeLinkResponse struct {
	RootURL     string    `json:"root_url" binding:"url"`
	Depth       int       `json:"depth"`
	GeneratedAt time.Time `json:"generated_at"`
	Pages       []Page    `json:"pages"`
}
type Page struct {
	URL        string `json:"url" binding:"url"`
	Depth      int    `json:"depth"`
	HTTPStatus int    `json:"http_status"`
	Status     string `json:"status"`
	Error      string `json:"error"`
}

// Analyze - точка входа анализатора
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{
			Timeout: opts.Timeout,
		}
	}

	statusCode, err := GetPageWithRetries(ctx, opts.URL, opts.HTTPClient, opts.Retries, opts.Delay)

	page := &Page{
		URL:        opts.URL,
		Depth:      1,
		HTTPStatus: statusCode,
		Status:     getStatusString(statusCode),
	}

	pages := []Page{*page}
	response := NewAnalyzeResponse(opts.URL, opts.Depth, pages)
	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

// NewAnalyzeResponse конструктор отчета
func NewAnalyzeResponse(rootURL string, depth int, pages []Page) *AnalyzeLinkResponse {
	return &AnalyzeLinkResponse{
		RootURL:     rootURL,
		Depth:       depth,
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
		Pages:       pages,
	}
}

// GetPageWithRetries - функция с поддержкой ретраев
func GetPageWithRetries(ctx context.Context, url string, client *http.Client, retries int, delay time.Duration) (int, error) {
	var lastErr error

	for i := 0; i <= retries; i++ {
		// Проверяем контекст
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		statusCode, err := GetPage(ctx, url, client)
		if err == nil && statusCode < 500 {
			return statusCode, nil
		}

		lastErr = err

		// Если не последняя попытка - ждём
		if i < retries && delay > 0 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return 0, fmt.Errorf("failed after %d retries: %w", retries, lastErr)
}

func GetPage(ctx context.Context, url string, httpClient *http.Client) (int, error) {

	// Создаем запрос
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	// Выполняем запрос через переданный клиент
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	return resp.StatusCode, err
}

// NewPageResponse конструктор отчета по отдельной странице
func NewPageResponse(result int, url string, depth int) *Page {
	return &Page{
		URL:        url,
		Depth:      depth,
		HTTPStatus: result,
		Status:     getStatusString(result),
		Error:      "",
	}
}

func getStatusString(statusCode int) string {
	switch statusCode {
	case 200, 201, 202, 204:
		return "ok"
	default:
		return "error"
	}
}
