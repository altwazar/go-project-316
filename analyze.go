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
	HTTPClient  string
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
	page, err := NewPageResponse(ctx, opts.URL, 1)
	if err != nil {
		return nil, err
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
func NewAnalyzeResponse(rootURL string, depth uint, pages []Page) *AnalyzeLinkResponse {
	return &AnalyzeLinkResponse{
		RootURL:     rootURL,
		Depth:       depth,
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
		Pages:       pages,
	}
}

// NewPageResponse конструктор отчета по отдельной странице
func NewPageResponse(ctx context.Context, url string, depth uint) (*Page, error) {
	// Выполняем GET запрос
	resp, err := http.Get(url)
	if err != nil {
		return &Page{
			URL:        url,
			Depth:      depth,
			HTTPStatus: resp.StatusCode,
			Status:     getStatusString(resp.StatusCode),
			Error:      err.Error(),
		}, nil
	} else {
		return &Page{
			URL:        url,
			Depth:      depth,
			HTTPStatus: resp.StatusCode,
			Status:     getStatusString(resp.StatusCode),
			Error:      "",
		}, nil
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
