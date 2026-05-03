package parser

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"code/internal/models"
)

// ExtractLinksAndAssets - извлечение ссылок и ассетов из HTML
func ExtractLinksAndAssets(htmlContent string, baseURL *url.URL) ([]string, []models.Asset, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	result := &extractResult{
		baseURL:  baseURL,
		links:    []string{},
		assets:   []models.Asset{},
		linkMap:  make(map[string]bool),
		assetMap: make(map[string]bool),
	}

	result.traverse(doc)
	return result.links, result.assets, nil
}

type extractResult struct {
	baseURL  *url.URL
	links    []string
	assets   []models.Asset
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

func (r *extractResult) processNode(n *html.Node) {
	handler := r.getNodeHandler(n.Data)
	if handler != nil {
		handler(n)
	}
}

func (r *extractResult) getNodeHandler(tagName string) func(*html.Node) {
	handlers := r.getHandlersMap()
	if handler, exists := handlers[tagName]; exists {
		return handler
	}
	return nil
}

func (r *extractResult) getHandlersMap() map[string]func(*html.Node) {
	return map[string]func(*html.Node){
		"a":      r.handleAnchor,
		"img":    r.handleImage,
		"script": r.handleScript,
		"link":   r.handleLink,
	}
}

func (r *extractResult) handleAnchor(n *html.Node) {
	r.processAttr(n, "href", r.addLink)
}

func (r *extractResult) handleImage(n *html.Node) {
	r.processAttr(n, "src", r.addAsset)
	r.processSrcset(n)
	r.processStyleAttr(n)
}

func (r *extractResult) handleScript(n *html.Node) {
	r.processAttr(n, "src", r.addAsset)
}

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
	if isSpecialURL(rawURL) {
		return
	}
	if url := r.normalizeURL(rawURL); url != "" && !r.assetMap[url] {
		r.assetMap[url] = true
		r.assets = append(r.assets, models.Asset{
			URL:  url,
			Type: detectAssetType(url),
		})
	}
}

func isSpecialURL(rawURL string) bool {
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

// ExtractSEOData - извлечение SEO данных из HTML
func ExtractSEOData(htmlContent string) models.SEOData {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return models.SEOData{}
	}
	seo := models.SEOData{}
	traverseForSEO(doc, &seo)
	return seo
}

func traverseForSEO(n *html.Node, seo *models.SEOData) {
	if n.Type == html.ElementNode {
		processSEOElement(n, seo)
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		traverseForSEO(child, seo)
	}
}

func processSEOElement(n *html.Node, seo *models.SEOData) {
	switch n.Data {
	case "title":
		extractTitle(n, seo)
	case "meta":
		extractMetaDescription(n, seo)
	case "h1":
		extractH1(n, seo)
	}
}

func extractTitle(n *html.Node, seo *models.SEOData) {
	if n.FirstChild != nil {
		seo.HasTitle = true
		seo.Title = html.UnescapeString(strings.TrimSpace(n.FirstChild.Data))
	}
}

func extractMetaDescription(n *html.Node, seo *models.SEOData) {
	name, content := getMetaAttributes(n)
	if name == "description" && content != "" {
		seo.HasDescription = true
		seo.Description = html.UnescapeString(strings.TrimSpace(content))
	}
}

func extractH1(n *html.Node, seo *models.SEOData) {
	if n.FirstChild != nil {
		seo.HasH1 = true
		// seo.H1 = html.UnescapeString(strings.TrimSpace(n.FirstChild.Data)) // закомментировано, как в оригинале
	}
}

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

// ExtractSEOFromXML - получение SEO из XML (RSS)
func ExtractSEOFromXML(xmlContent string) models.SEOData {
	seo := models.SEOData{}
	channelIdx := strings.Index(xmlContent, "<channel>")
	if channelIdx != -1 {
		titleStart := strings.Index(xmlContent[channelIdx:], "<title>")
		if titleStart != -1 {
			titleStart += channelIdx + 7
			titleEnd := strings.Index(xmlContent[titleStart:], "</title>")
			if titleEnd != -1 {
				title := xmlContent[titleStart : titleStart+titleEnd]
				seo.HasTitle = true
				seo.Title = html.UnescapeString(strings.TrimSpace(title))
			}
		}
	}
	return seo
}

// detectAssetType - определение типа ассета по расширению и тегу
func detectAssetType(urlStr string) models.AssetType {
	urlStr = strings.ToLower(urlStr)
	if strings.HasPrefix(urlStr, "data:image/") {
		return models.AssetTypeImage
	}
	extensions := []extensionType{
		{patterns: []string{".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".ico", ".bmp", ".tiff", ".tif"}, assetType: models.AssetTypeImage},
		{patterns: []string{".js", ".mjs"}, assetType: models.AssetTypeScript},
		{patterns: []string{".css"}, assetType: models.AssetTypeStyle},
	}
	for _, ext := range extensions {
		if containsAnyExtension(urlStr, ext.patterns) {
			return ext.assetType
		}
	}
	return models.AssetTypeImage
}

type extensionType struct {
	patterns  []string
	assetType models.AssetType
}

func containsAnyExtension(urlStr string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.Contains(urlStr, ext) {
			return true
		}
	}
	return false
}
