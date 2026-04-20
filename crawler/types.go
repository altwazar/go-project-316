package crawler

import (
	"net/http"
	"time"
)

// Options - настройки для краулера
type Options struct {
	URL         string
	Depth       int
	Retries     int
	Delay       time.Duration
	Timeout     time.Duration
	UserAgent   string
	Concurrency int
	RPS         int
	IndentJSON  int
	HTTPClient  *http.Client
}

// AnalyzeLinkResponse - структура с отчетом
type AnalyzeLinkResponse struct {
	RootURL     string    `json:"root_url"`
	Depth       int       `json:"depth"`
	GeneratedAt time.Time `json:"generated_at"`
	Pages       []Page    `json:"pages"`
}

// Page - информация о странице
type Page struct {
	URL          string       `json:"url"`
	Depth        int          `json:"depth"`
	HTTPStatus   int          `json:"http_status"`
	Status       string       `json:"status"`
	Error        string       `json:"error"`
	BrokenLinks  []LinkStatus `json:"broken_links"`
	Links        []string     `json:"-"`
	DiscoveredAt time.Time    `json:"discovered_at"`
	SEO          SEOData      `json:"seo"`
}

// LinkStatus - информация о статусе ссылки
type LinkStatus struct {
	URL    string `json:"url"`
	Status int    `json:"status_code,omitempty"`
	Error  string `json:"error,omitempty"`
}

// SEOData - SEO-данные
type SEOData struct {
	HasTitle       bool   `json:"has_title"`
	Title          string `json:"title,omitempty"`
	HasDescription bool   `json:"has_description"`
	Description    string `json:"description,omitempty"`
	HasH1          bool   `json:"has_h1"`
	H1             string `json:"h1,omitempty"`
}
