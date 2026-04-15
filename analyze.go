package code

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	URL          string      `json:"url" binding:"url"`
	Depth        int         `json:"depth"`
	HTTPStatus   int         `json:"http_status"`
	Status       string      `json:"status"`
	Error        string      `json:"error"`
	BrokenLinks  []LinkCheck `json:"broken_links"`
	Links        []string    `json:"-"`
	DiscoveredAt time.Time   `json:"discovered_at"`
}
type LinkCheck struct {
	URL    string `json:"url binding:"url"`
	Status string `json:"status"`
}

// Analyze - точка входа анализатора
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{
			Timeout: opts.Timeout,
		}
	}

	response, err := PageCrawler(ctx, opts)

	if err != nil {
		return nil, err
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

func PageCrawler(ctx context.Context, opts Options) (AnalyzeLinkResponse, error) {
	// Отчет
	analyzeResponse := NewAnalyzeResponse(opts.URL, opts.Depth, []Page{})
	// Текущая глубина
	curDepth := 1
	// Текущий список ссылок на проверку
	links := []string{opts.URL}
	pages := []Page{}
	// Переебор ссылок, пока они есть и позволяет глубина
	for {
		if len(links) == 0 || curDepth > opts.Depth {
			break
		}
		pagesOnLevel := []Page{}
		linksOnLevel := []string{}
		for _, link := range links {
			page, err := GetPageWithRetries(ctx, link, curDepth, opts.HTTPClient, opts.Retries, opts.Delay)

			if err != nil {
				return *analyzeResponse, err
			}

			pagesOnLevel = append(pagesOnLevel, page)
			linksOnLevel = append(linksOnLevel, page.Links...)
		}
		pages = append(pages, pagesOnLevel...)
		curDepth += 1
		links = linksOnLevel
	}
	analyzeResponse.Pages = pages
	return *analyzeResponse, nil
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

// ExtractLinks извлекает все ссылки из HTML-страницы
func ExtractLinks(htmlContent string, baseURL *url.URL) ([]string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var links []string

	// Рекурсивный обход DOM-дерева
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		// Проверяем, является ли узел тегом <a>
		if n.Type == html.ElementNode && n.Data == "a" {
			// Ищем атрибут href
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					// Преобразуем относительную ссылку в абсолютную
					href := attr.Val
					if href != "" && !strings.HasPrefix(href, "#") &&
						!strings.HasPrefix(href, "javascript:") &&
						!strings.HasPrefix(href, "mailto:") &&
						!strings.HasPrefix(href, "tel:") {

						// Парсим ссылку относительно базового URL
						parsed, err := baseURL.Parse(href)
						if err == nil {
							// Очищаем URL (убираем якоря и параметры)
							parsed.Fragment = ""
							links = append(links, parsed.String())
						}
					}
					break
				}
			}
		}
		// Рекурсивно обходим всех детей
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			traverse(child)
		}
	}

	traverse(doc)
	return links, nil
}

// GetPageWithRetries - функция с поддержкой ретраев, возвращает ссылки
func GetPageWithRetries(ctx context.Context, url string, depth int, client *http.Client, retries int, delay time.Duration) (Page, error) {
	var lastErr error

	for i := 0; i <= retries; i++ {
		select {
		case <-ctx.Done():
			return Page{}, ctx.Err()
		default:
		}

		statusCode, links, err := GetPageWithLinks(ctx, url, client)
		if err == nil && statusCode < 500 {
			return *NewPageResponse(statusCode, url, depth, links), nil
		}

		lastErr = err

		if i < retries && delay > 0 {
			select {
			case <-ctx.Done():
				return Page{}, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return Page{}, fmt.Errorf("failed after %d retries: %w", retries, lastErr)
}

// GetPageWithLinks делает запрос и возвращает статус и список ссылок
func GetPageWithLinks(ctx context.Context, urlStr string, httpClient *http.Client) (int, []string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return 0, nil, err
	}

	// Выполняем запрос
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	// Проверяем Content-Type (парсим только HTML)
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return resp.StatusCode, nil, nil // Не HTML, ссылок нет
	}

	// Читаем тело ответа
	// Ограничиваем размер для защиты от больших файлов
	const maxSize = 10 * 1024 * 1024 // 10MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed to read body: %w", err)
	}

	// Парсим базовый URL
	baseURL, err := url.Parse(urlStr)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Извлекаем ссылки
	links, err := ExtractLinks(string(body), baseURL)
	if err != nil {
		return resp.StatusCode, nil, err
	}

	return resp.StatusCode, links, nil
}

// GetPage делает запрос и отдаёт его статус
func GetPage(ctx context.Context, url string, httpClient *http.Client) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	// Выполняем запрос через переданный клиент
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

// NewPageResponse конструктор отчета по отдельной странице
func NewPageResponse(statusCode int, url string, depth int, links []string) *Page {
	return &Page{
		URL:          url,
		Depth:        depth,
		HTTPStatus:   statusCode,
		Status:       getStatusString(statusCode),
		BrokenLinks:  []LinkCheck{},
		Links:        links,
		DiscoveredAt: time.Now().UTC().Truncate(time.Second),
		Error:        "",
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
