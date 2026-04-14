package code

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Options структура с настройками анализатора
type Options struct {
	URL         string
	Depth       uint
	Retries     uint
	Delay       uint
	Timeout     uint
	UserAgent   string
	Concurrency uint
	IndentJSON  uint
	HTTPClient  *http.Client
}

type AnalyzeLinkResponse struct {
	RootURL     string    `json:"root_url" binding:"url"`
	Depth       uint      `json:"depth"`
	GeneratedAt time.Time `json:"generated_at"`
	Pages       []Page    `json:"pages"`
}
type Page struct {
	URL        string `json:"url" binding:"url"`
	Depth      uint   `json:"depth"`
	HTTPStatus int    `json:"http_status"`
	Status     string `json:"status"`
	Error      string `json:"error"`
}

// Analyze - точка входа анализатора
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	getResult, err := GetPage(ctx, opts.URL, opts.HTTPClient)
	if err != nil {
		return nil, err
	}
	page := NewPageResponse(getResult, opts.URL, 1)
	pages := []Page{*page}
	response := NewAnalyzeResponse(opts.URL, opts.Depth, pages)
	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

// NewAnalyzeResponse конструктор отчета
func NewAnalyzeResponse(rootURL string, depth uint, pages []Page) *AnalyzeLinkResponse {
	return &AnalyzeLinkResponse{
		RootURL:     rootURL,
		Depth:       depth,
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
		Pages:       pages,
	}
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
func NewPageResponse(result int, url string, depth uint) *Page {
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
