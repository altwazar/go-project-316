package code

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
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

// Под задачи
type taskType int

const (
	GetPageTask taskType = iota
	CheckLinkTask
)

type task struct {
	url      string
	taskType taskType
	depth    int
}

func newTask(url string, t taskType, depth int) *task {
	return &task{
		url:      url,
		taskType: t,
		depth:    depth,
	}
}

// Выполняет задачу
func (t *task) ExecuteTask(p *Pool) {
	u, err := normalizeURL(t.url)
	// Обработка двух типов задач, получение страницы и простая проверка доступности
	if t.taskType == GetPageTask {
		// Если страница с таким url уже в обработке, то задача прерывается
		_, inProgress := p.getPagesInProgress[u]
		if inProgress {
			return
		}
		// Задача в обработке
		p.getPagesInProgress[u] = 1
		pg := Page{}
		// Если была ошибка нормализации url, то запросить мы его не можем
		if err != nil {
			pg = Page{
				URL:   t.url,
				Error: err.Error(),
			}
		} else {
			// Получение страницы
			pg = GetPageWithRetries(p.ctx, u, t.depth, p.opts)
			// Если глубина не исчерпана и линк в рамках начального домена,
			// то из линка создаётся задача на получение страницы.
			// В остальных случаях задача на простую проверку.
			if p.opts.Depth > t.depth {
				for _, ln := range pg.Links {
					if isSameDomain(p.opts.URL, ln) {
						p.tasks = append(p.tasks, *newTask(ln, GetPageTask, t.depth+1))
					} else {
						p.tasks = append(p.tasks, *newTask(ln, CheckLinkTask, t.depth+1))
					}
				}
			} else {
				for _, ln := range pg.Links {
					p.tasks = append(p.tasks, *newTask(ln, CheckLinkTask, t.depth+1))
				}
			}
		}
		// Готовая страница идет в список страниц
		p.pages = append(p.pages, pg)
	} else {
		// Если страница с таким url уже в обработке, то задача прерывается
		_, inProgress := p.linkChecksInProgress[u]
		if inProgress {
			return
		}
		// Задача в обработке
		p.linkChecksInProgress[u] = 1
		ln := linkStatus{}
		// Если была ошибка нормализации url, то запросить мы его не можем
		if err != nil {
			ln = linkStatus{
				URL:   t.url,
				Error: err.Error(),
			}
		} else {
			// Получаем статус страницы и/или ошибку
			s, err := checkLinkStatus(p.ctx, u, p.opts.HTTPClient)
			if err == nil {
				ln = linkStatus{
					URL:    t.url,
					Status: s,
					Error:  "",
				}
			} else {
				ln = linkStatus{
					URL:    t.url,
					Status: 0,
					Error:  err.Error(),
				}
			}
		}
		// Линк идет в список линуков, для формирования отчета
		p.linkStatuses[u] = ln
	}
}

func isSameDomain(urlStr1, urlStr2 string) bool {
	u1, _ := url.Parse(urlStr1)
	u2, _ := url.Parse(urlStr2)

	return u1.Host == u2.Host
}

// После выполнения задач сборка отчета
func parseResult(p *Pool) AnalyzeLinkResponse {
	// Перебираем страницы
	for i := range p.pages {
		// Перебираем линки на странице
		for _, link := range p.pages[i].Links {
			// Если в списке обработанных линков такой присутсвует,
			// и он с ошибкой или статусом 4xx, 5xx, то добавляется
			// в список сломанных слинков страницы
			l, ok := p.linkStatuses[link]
			if ok && l.Status >= 400 || l.Error != "" {
				p.pages[i].BrokenLinks = append(p.pages[i].BrokenLinks, l)
			}
		}
	}
	return *NewAnalyzeResponse(p.opts.URL, p.opts.Depth, p.pages)
}

// Перебор задач, получение страниц и статусов
// Макет концепта однопоточного перебора без воркеров
func crawl(p *Pool) {
	// Пока список задач не будет пустым идет их запуск в цикле.
	for {
		if len(p.tasks) == 0 {
			break
		}
		var t task
		// Забираем первую задачу из списка
		t, p.tasks = p.tasks[0], p.tasks[1:]
		fmt.Println(t)
		// Выполняем её
		t.ExecuteTask(p)
	}
}

func checkLinkStatus(ctx context.Context, urlStr string, client *http.Client) (int, error) {
	// Парсим URL для валидации
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return 0, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "http"
	}

	// Создаем HEAD запрос
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, parsedURL.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	// Выполняем HEAD запрос
	headResp, err := client.Do(headReq)
	if err == nil {
		defer headResp.Body.Close()
		return headResp.StatusCode, nil
	}

	// Если HEAD не поддерживается (405) или другая ошибка, пробуем GET
	// Также пробуем GET при ошибках сети или таймаутах
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create GET request: %w", err)
	}

	getResp, err := client.Do(getReq)
	if err != nil {
		return 0, fmt.Errorf("both HEAD and GET requests failed: %w", err)
	}
	defer getResp.Body.Close()

	return getResp.StatusCode, nil
}

// Пул для обработки задач
type Pool struct {
	ctx                  context.Context
	opts                 Options
	tasks                []task
	linkChecksInProgress map[string]int
	getPagesInProgress   map[string]int
	linkStatuses         map[string]linkStatus
	pages                []Page
}

func NewPool(ctx context.Context, opts Options) *Pool {
	url, err := normalizeURL(opts.URL)
	if err != nil {
		log.Fatal("Ошибка с корневым url: ", err)
	}
	firstTask := newTask(url, GetPageTask, 1)
	return &Pool{
		ctx:                  ctx,
		opts:                 opts,
		tasks:                []task{*firstTask},
		linkChecksInProgress: map[string]int{},
		getPagesInProgress:   map[string]int{},
		linkStatuses:         map[string]linkStatus{},
		pages:                []Page{},
	}
}

func normalizeURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Приводим схему и хост к нижнему регистру
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	// Убираем стандартные порты
	if (u.Scheme == "http" && u.Port() == "80") ||
		(u.Scheme == "https" && u.Port() == "443") {
		u.Host = strings.TrimSuffix(u.Host, ":"+u.Port())
	}

	// Нормализуем путь (убираем . и ..)
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path == "" {
		u.Path = "/"
	}

	// Убираем фрагмент (#...)
	u.Fragment = ""

	// Сортируем параметры запроса для единообразия
	if u.RawQuery != "" {
		values := u.Query()
		u.RawQuery = values.Encode() // сортирует ключи
	}

	return u.String(), nil
}

// SeoData содержит базовые SEO-показатели страницы
type SeoData struct {
	HasTitle       bool   `json:"has_title"`
	Title          string `json:"title,omitempty"`
	HasDescription bool   `json:"has_description"`
	Description    string `json:"description,omitempty"`
	HasH1          bool   `json:"has_h1"`
	H1             string `json:"h1,omitempty"`
}

// AnalyzeLinkResponse содержит структуру с готовым отчетом
type AnalyzeLinkResponse struct {
	RootURL     string    `json:"root_url" binding:"url"`
	Depth       int       `json:"depth"`
	GeneratedAt time.Time `json:"generated_at"`
	Pages       []Page    `json:"pages"`
}

// Page содержит полученные со страницы данные
type Page struct {
	URL          string       `json:"url" binding:"url"`
	Depth        int          `json:"depth"`
	HTTPStatus   int          `json:"http_status"`
	Status       string       `json:"status"`
	Error        string       `json:"error"`
	BrokenLinks  []linkStatus `json:"broken_links"`
	Links        []string     `json:"-"`
	DiscoveredAt time.Time    `json:"discovered_at"`
	Seo          SeoData      `json:"seo"`
}

type linkStatus struct {
	URL    string `json:"url" binding:"url"`
	Status int    `json:"status_code,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Analyze - точка входа анализатора
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{
			Timeout: opts.Timeout,
		}
	}

	p := NewPool(ctx, opts)
	crawl(p)
	response := parseResult(p)

	// Формируем строку отступа из пробелов
	indent := "  " // значение по умолчанию
	if opts.IndentJSON > 0 {
		indent = strings.Repeat(" ", opts.IndentJSON)
	}

	jsonData, err := json.MarshalIndent(response, "", indent)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

// ExtractSeoData извлекает title, description и h1 из HTML
func ExtractSeoData(htmlContent string) SeoData {
	seo := SeoData{}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return seo
	}

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Ищем <title>
			if n.Data == "title" && n.FirstChild != nil {
				seo.HasTitle = true
				seo.Title = strings.TrimSpace(n.FirstChild.Data)
			}

			// Ищем <meta name="description">
			if n.Data == "meta" {
				var name, content string
				for _, attr := range n.Attr {
					if attr.Key == "name" {
						name = strings.ToLower(attr.Val)
					}
					if attr.Key == "content" {
						content = attr.Val
					}
				}
				if name == "description" && content != "" {
					seo.HasDescription = true
					seo.Description = strings.TrimSpace(content)
				}
			}

			// Ищем <h1>
			if n.Data == "h1" && n.FirstChild != nil {
				seo.HasH1 = true
				seo.H1 = strings.TrimSpace(n.FirstChild.Data)
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			traverse(child)
		}
	}

	traverse(doc)
	return seo
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
// Поддерживает теги <a href="..."> и <link href="...">
func ExtractLinks(htmlContent string, baseURL *url.URL) ([]string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var links []string
	linkMap := make(map[string]bool) // Для уникальности ссылок

	// Рекурсивный обход DOM-дерева
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var hrefAttr string
			var isALink bool

			// Проверяем, является ли узел тегом <a> или <link>
			if n.Data == "a" || n.Data == "link" {
				isALink = true
				// Ищем атрибут href
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						hrefAttr = attr.Val
						break
					}
				}
			}

			// Если нашли href в поддерживаемых тегах
			if isALink && hrefAttr != "" {
				// Фильтруем нежелательные ссылки
				if !strings.HasPrefix(hrefAttr, "#") &&
					!strings.HasPrefix(hrefAttr, "javascript:") &&
					!strings.HasPrefix(hrefAttr, "mailto:") &&
					!strings.HasPrefix(hrefAttr, "tel:") {

					// Парсим ссылку относительно базового URL
					parsed, err := baseURL.Parse(hrefAttr)
					if err == nil {
						// Очищаем URL (убираем якоря и параметры)
						parsed.Fragment = ""
						normalizedURL := parsed.String()

						// Добавляем только уникальные ссылки
						if !linkMap[normalizedURL] {
							linkMap[normalizedURL] = true
							links = append(links, normalizedURL)
						}
					}
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

func GetPageWithRetries(ctx context.Context, url string, depth int, opts Options) Page {
	var lastErr error

	for i := 0; i <= opts.Retries; i++ {
		select {
		case <-ctx.Done():
			return *NewPageResponse(0, url, depth, []string{}, SeoData{}, ctx.Err().Error())
		default:
		}

		finalUrl, statusCode, links, seoData, err := GetPageWithLinks(ctx, url, opts.HTTPClient)
		if err == nil && statusCode < 500 {
			return *NewPageResponse(statusCode, url, depth, links, seoData, "")
		}

		lastErr = err

		if i < opts.Retries && opts.Delay > 0 {
			select {
			case <-ctx.Done():
				return *NewPageResponse(0, finalUrl, depth, []string{}, SeoData{}, ctx.Err().Error())
			case <-time.After(opts.Delay):
			}
		}
	}

	return *NewPageResponse(0, url, depth, []string{}, SeoData{}, lastErr.Error())
}

// GetPageWithLinks делает запрос и возвращает статус, ссылки и SEO-данные
func GetPageWithLinks(ctx context.Context, urlStr string, httpClient *http.Client) (string, int, []string, SeoData, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return urlStr, 0, nil, SeoData{}, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return urlStr, 0, nil, SeoData{}, err
	}
	defer resp.Body.Close()

	finalUrl := resp.Request.URL.String()
	seoData := SeoData{}

	// Проверяем Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return finalUrl, resp.StatusCode, nil, seoData, nil
	}

	// Читаем тело ответа
	const maxSize = 10 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return finalUrl, resp.StatusCode, nil, seoData, fmt.Errorf("failed to read body: %w", err)
	}

	// Парсим базовый URL
	baseURL, err := url.Parse(urlStr)
	if err != nil {
		return finalUrl, resp.StatusCode, nil, seoData, fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Извлекаем ссылки
	links, err := ExtractLinks(string(body), baseURL)
	if err != nil {
		return finalUrl, resp.StatusCode, nil, seoData, err
	}

	// ← ДОБАВИТЬ: Извлекаем SEO-данные
	seoData = ExtractSeoData(string(body))

	return finalUrl, resp.StatusCode, links, seoData, nil
}

func NewPageResponse(statusCode int, url string, depth int, links []string, seo SeoData, error string) *Page {
	return &Page{
		URL:          url,
		Depth:        depth,
		HTTPStatus:   statusCode,
		Status:       getStatusString(statusCode),
		BrokenLinks:  []linkStatus{},
		Links:        links,
		DiscoveredAt: time.Now().UTC().Truncate(time.Second),
		Error:        error,
		Seo:          seo, // ← ДОБАВИТЬ
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
