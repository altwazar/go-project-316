package crawler

import (
	"fmt"
	"golang.org/x/net/html"
	"net/url"
	"strings"
)

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
