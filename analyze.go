package code

import (
	"context"
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
	Pages       Pages     `json:"pages"`
}
type Pages struct {
	URL        string `json:"url" binding:"url"`
	Depth      uint   `json:"depth"`
	HTTPStatus uint   `json:"http_status"`
	Status     string `json:"status"`
	Error      string `json:"error"`
}

// Analyze - точка входа анализатора
func Analyze(ctx context.Context, opts Options) ([]byte, error) {

	return nil, nil
}
