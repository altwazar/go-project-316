package models

import (
	"net/http"
	"time"
)

// AssetType - тип ассета
type AssetType string

const (
	AssetTypeImage  AssetType = "image"
	AssetTypeScript AssetType = "script"
	AssetTypeStyle  AssetType = "style"
)

// Asset - информация об ассете
type Asset struct {
	URL        string    `json:"url"`
	Type       AssetType `json:"type"`
	StatusCode int       `json:"status_code"`
	SizeBytes  int64     `json:"size_bytes"`
	Error      string    `json:"error,omitempty"`
}

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
	IndentJSON  bool
	HTTPClient  *http.Client
}

// Report - структура с отчетом
type Report struct {
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
	Error        string       `json:"error,omitempty"`
	BrokenLinks  []LinkStatus `json:"broken_links"`
	Links        []string     `json:"-"`
	Assets       []Asset      `json:"assets"`
	DiscoveredAt time.Time    `json:"discovered_at"`
	SEO          SEOData      `json:"seo"`
}

// LinkStatus - информация о статусе ссылки
type LinkStatus struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
}

// SEOData - SEO-данные
type SEOData struct {
	HasTitle       bool   `json:"has_title"`
	Title          string `json:"title"`
	HasDescription bool   `json:"has_description"`
	Description    string `json:"description"`
	HasH1          bool   `json:"has_h1"`
	// H1             string `json:"h1"` // закомментировано, как в оригинале
}
