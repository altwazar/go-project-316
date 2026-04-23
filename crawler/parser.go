package crawler

import (
	"fmt"
	"golang.org/x/net/html"
	"net/url"
	"strings"
)

// decodeHTMLEntities - декодирует HTML-сущности в тексте
func decodeHTMLEntities(s string) string {
	return html.UnescapeString(s)
}

// detectAssetType - определение типа ассета по расширению и тегу (сложность 2)
func detectAssetType(urlStr string) AssetType {
	urlStr = strings.ToLower(urlStr)

	// Data URL всегда изображение
	if strings.HasPrefix(urlStr, "data:image/") {
		return AssetTypeImage
	}

	return detectAssetByExtension(urlStr)
}

// detectAssetByExtension - определение типа по расширению
func detectAssetByExtension(urlStr string) AssetType {
	extensions := []extensionType{
		{patterns: []string{".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".ico", ".bmp", ".tiff", ".tif"}, assetType: AssetTypeImage},
		{patterns: []string{".js", ".mjs"}, assetType: AssetTypeScript},
		{patterns: []string{".css"}, assetType: AssetTypeStyle},
	}

	for _, ext := range extensions {
		if containsAnyExtension(urlStr, ext.patterns) {
			return ext.assetType
		}
	}

	return AssetTypeImage
}

// extensionType - структура для маппинга расширений
type extensionType struct {
	patterns  []string
	assetType AssetType
}

// containsAnyExtension - проверка наличия любого из расширений
func containsAnyExtension(urlStr string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.Contains(urlStr, ext) {
			return true
		}
	}
	return false
}

// extractLinksAndAssets - получение ссылок из ассетов
func extractLinksAndAssets(htmlContent string, baseURL *url.URL) ([]string, []Asset, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	result := &extractResult{
		baseURL:  baseURL,
		links:    []string{},
		assets:   []Asset{},
		linkMap:  make(map[string]bool),
		assetMap: make(map[string]bool),
	}

	result.traverse(doc)
	return result.links, result.assets, nil
}

type extractResult struct {
	baseURL  *url.URL
	links    []string
	assets   []Asset
	linkMap  map[string]bool
	assetMap map[string]bool
}

func (r *extractResult) traverse(n *html.Node) {
	if n.Type == html.ElementNode {
		r.processNode(n)
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		r.traverse(child)
	}
}

// processNode - обработка узла
func (r *extractResult) processNode(n *html.Node) {
	handler := r.getNodeHandler(n.Data)
	if handler != nil {
		handler(n)
	}
}

// getNodeHandler - получение обработчика для тега
func (r *extractResult) getNodeHandler(tagName string) func(*html.Node) {
	handlers := r.getHandlersMap()

	if handler, exists := handlers[tagName]; exists {
		return handler
	}
	return nil
}

// getHandlersMap - карта обработчиков (конфигурация)
func (r *extractResult) getHandlersMap() map[string]func(*html.Node) {
	return map[string]func(*html.Node){
		"a":      r.handleAnchor,
		"img":    r.handleImage,
		"script": r.handleScript,
		"link":   r.handleLink,
	}
}

// handleAnchor - обработка тега a
func (r *extractResult) handleAnchor(n *html.Node) {
	r.processAttr(n, "href", r.addLink)
}

// handleImage - обработка тега img
func (r *extractResult) handleImage(n *html.Node) {
	r.processAttr(n, "src", r.addAsset)
	r.processSrcset(n)
	r.processStyleAttr(n)
}

// handleScript - обработка тега script
func (r *extractResult) handleScript(n *html.Node) {
	r.processAttr(n, "src", r.addAsset)
}

// handleLink - обработка тега link
func (r *extractResult) handleLink(n *html.Node) {
	if !r.isStylesheet(n) {
		return
	}
	r.processAttr(n, "href", r.addAsset)
}

func (r *extractResult) processAttr(n *html.Node, attrName string, fn func(string)) {
	for _, attr := range n.Attr {
		if attr.Key == attrName && attr.Val != "" {
			fn(attr.Val)
		}
	}
}

func (r *extractResult) isStylesheet(n *html.Node) bool {
	for _, attr := range n.Attr {
		if attr.Key == "rel" && attr.Val == "stylesheet" {
			return true
		}
	}
	return false
}

func (r *extractResult) processSrcset(n *html.Node) {
	for _, attr := range n.Attr {
		if attr.Key == "srcset" {
			parts := strings.Split(attr.Val, ",")
			if len(parts) > 0 {
				urlPart := strings.Split(strings.TrimSpace(parts[0]), " ")[0]
				r.addAsset(urlPart)
			}
		}
	}
}

func (r *extractResult) processStyleAttr(n *html.Node) {
	for _, attr := range n.Attr {
		if attr.Key == "style" {
			urls := extractURLsFromStyle(attr.Val)
			for _, url := range urls {
				r.addAsset(url)
			}
		}
	}
}

func (r *extractResult) addLink(rawURL string) {
	if url := r.normalizeURL(rawURL); url != "" && !r.linkMap[url] {
		r.linkMap[url] = true
		r.links = append(r.links, url)
	}
}

func (r *extractResult) addAsset(rawURL string) {
	if r.isSpecialURL(rawURL) {
		return
	}
	if url := r.normalizeURL(rawURL); url != "" && !r.assetMap[url] {
		r.assetMap[url] = true
		r.assets = append(r.assets, Asset{URL: url, Type: detectAssetType(url)})
	}
}

func (r *extractResult) isSpecialURL(rawURL string) bool {
	special := []string{"#", "javascript:", "mailto:", "tel:"}
	for _, s := range special {
		if strings.HasPrefix(rawURL, s) {
			return true
		}
	}
	return strings.HasPrefix(rawURL, "data:")
}

func (r *extractResult) normalizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := r.baseURL.Parse(rawURL)
	if err != nil {
		return ""
	}
	parsed.Fragment = ""
	return parsed.String()
}

// extractURLsFromStyle - извлечение URL из CSS
func extractURLsFromStyle(style string) []string {
	var urls []string

	for i := 0; i < len(style); {
		urlStart := strings.Index(style[i:], "url(")
		if urlStart == -1 {
			break
		}

		urlStart += i
		contentStart := urlStart + 4

		urlEnd := strings.Index(style[contentStart:], ")")
		if urlEnd == -1 {
			break
		}

		urlEnd += contentStart
		urlStr := strings.Trim(style[contentStart:urlEnd], "'\" \t\n\r")

		if urlStr != "" {
			urls = append(urls, urlStr)
		}

		i = urlEnd + 1
	}

	return urls
}

// extractSEOData - извлечение SEO данных
func extractSEOData(htmlContent string) SEOData {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return SEOData{}
	}

	seo := SEOData{}
	traverseForSEO(doc, &seo)
	return seo
}

// traverseForSEO - обход DOM для SEO
func traverseForSEO(n *html.Node, seo *SEOData) {
	if n.Type == html.ElementNode {
		processSEOElement(n, seo)
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		traverseForSEO(child, seo)
	}
}

// processSEOElement - обработка SEO элемента
func processSEOElement(n *html.Node, seo *SEOData) {
	switch n.Data {
	case "title":
		extractTitle(n, seo)
	case "meta":
		extractMetaDescription(n, seo)
	case "h1":
		extractH1(n, seo)
	}
}

// extractTitle - извлечение title
func extractTitle(n *html.Node, seo *SEOData) {
	if n.FirstChild != nil {
		seo.HasTitle = true
		seo.Title = decodeHTMLEntities(strings.TrimSpace(n.FirstChild.Data))
	}
}

// extractMetaDescription - извлечение meta description
func extractMetaDescription(n *html.Node, seo *SEOData) {
	name, content := getMetaAttributes(n)

	if name == "description" && content != "" {
		seo.HasDescription = true
		seo.Description = decodeHTMLEntities(strings.TrimSpace(content))
	}
}

// extractH1 - извлечение H1
func extractH1(n *html.Node, seo *SEOData) {
	if n.FirstChild != nil {
		seo.HasH1 = true
		seo.H1 = decodeHTMLEntities(strings.TrimSpace(n.FirstChild.Data))
	}
}

// getMetaAttributes - получение атрибутов meta
func getMetaAttributes(n *html.Node) (string, string) {
	var name, content string

	for _, attr := range n.Attr {
		if attr.Key == "name" {
			name = strings.ToLower(attr.Val)
		}
		if attr.Key == "content" {
			content = attr.Val
		}
	}

	return name, content
}

// normalizeURL - нормализация URL
func normalizeURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	normalizeSchemeAndHost(u)
	removeDefaultPort(u)
	normalizePath(u)
	u.Fragment = ""
	normalizeQuery(u)

	return u.String(), nil
}

// normalizeSchemeAndHost - нормализация схемы и хоста
func normalizeSchemeAndHost(u *url.URL) {
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
}

// removeDefaultPort - удаление порта по умолчанию
func removeDefaultPort(u *url.URL) {
	if isDefaultPort(u.Scheme, u.Port()) {
		u.Host = strings.TrimSuffix(u.Host, ":"+u.Port())
	}
}

// isDefaultPort - проверка порта по умолчанию
func isDefaultPort(scheme, port string) bool {
	return (scheme == "http" && port == "80") ||
		(scheme == "https" && port == "443")
}

// normalizePath - нормализация пути
func normalizePath(u *url.URL) {
	if u.Path == "" {
		u.Path = "/"
		return
	}

	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path == "" {
		u.Path = "/"
	}
}

// normalizeQuery - нормализация параметров запроса
func normalizeQuery(u *url.URL) {
	if u.RawQuery == "" {
		return
	}

	values := u.Query()
	u.RawQuery = values.Encode()
}

func isSameDomain(urlStr1, urlStr2 string) bool {
	u1, _ := url.Parse(urlStr1)
	u2, _ := url.Parse(urlStr2)
	return u1.Host == u2.Host
}
