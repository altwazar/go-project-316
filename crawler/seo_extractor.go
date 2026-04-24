package crawler

import (
	"golang.org/x/net/html"
	"strings"
)

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

// decodeHTMLEntities - декодирует HTML-сущности в тексте
func decodeHTMLEntities(s string) string {
	return html.UnescapeString(s)
}
