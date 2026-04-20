package crawler

import (
	"fmt"
	"golang.org/x/net/html"
	"net/url"
	"strings"
)

// extractLinks - функция извлечения ссылок
func extractLinks(htmlContent string, baseURL *url.URL) ([]string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var links []string
	linkMap := make(map[string]bool)

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var hrefAttr string
			var isALink bool

			if n.Data == "a" || n.Data == "link" {
				isALink = true
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						hrefAttr = attr.Val
						break
					}
				}
			}

			if isALink && hrefAttr != "" {
				if !strings.HasPrefix(hrefAttr, "#") &&
					!strings.HasPrefix(hrefAttr, "javascript:") &&
					!strings.HasPrefix(hrefAttr, "mailto:") &&
					!strings.HasPrefix(hrefAttr, "tel:") {

					parsed, err := baseURL.Parse(hrefAttr)
					if err == nil {
						parsed.Fragment = ""
						normalizedURL := parsed.String()

						if !linkMap[normalizedURL] {
							linkMap[normalizedURL] = true
							links = append(links, normalizedURL)
						}
					}
				}
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			traverse(child)
		}
	}

	traverse(doc)
	return links, nil
}

func normalizeURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	if (u.Scheme == "http" && u.Port() == "80") ||
		(u.Scheme == "https" && u.Port() == "443") {
		u.Host = strings.TrimSuffix(u.Host, ":"+u.Port())
	}

	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path == "" {
		u.Path = "/"
	}

	u.Fragment = ""

	if u.RawQuery != "" {
		values := u.Query()
		u.RawQuery = values.Encode()
	}

	return u.String(), nil
}

func isSameDomain(urlStr1, urlStr2 string) bool {
	u1, _ := url.Parse(urlStr1)
	u2, _ := url.Parse(urlStr2)
	return u1.Host == u2.Host
}
