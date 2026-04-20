package crawler

import (
	"golang.org/x/net/html"
	"strings"
)

// extractSEOData - функция извлечения SEO-данных
func extractSEOData(htmlContent string) SEOData {
	seo := SEOData{}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return seo
	}

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "title" && n.FirstChild != nil {
				seo.HasTitle = true
				seo.Title = strings.TrimSpace(n.FirstChild.Data)
			}

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
