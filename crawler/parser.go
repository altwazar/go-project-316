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

// detectAssetType - определение типа ассета по расширению и тегу
func detectAssetType(urlStr string) AssetType {
	lowerURL := strings.ToLower(urlStr)

	// Изображения
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".ico", ".bmp", ".tiff", ".tif"}
	for _, ext := range imageExts {
		if strings.Contains(lowerURL, ext) {
			return AssetTypeImage
		}
	}

	// Скрипты
	if strings.Contains(lowerURL, ".js") || strings.Contains(lowerURL, ".mjs") {
		return AssetTypeScript
	}

	// Стили
	if strings.Contains(lowerURL, ".css") {
		return AssetTypeStyle
	}

	// По умолчанию - изображение (для data:image и прочих)
	if strings.HasPrefix(lowerURL, "data:image/") {
		return AssetTypeImage
	}

	return AssetTypeImage
}

// extractLinksAndAssets - функция извлечения ссылок и ассетов
func extractLinksAndAssets(htmlContent string, baseURL *url.URL) ([]string, []Asset, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var links []string
	var assets []Asset
	linkMap := make(map[string]bool)
	assetMap := make(map[string]bool)

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Обработка ссылок
			if n.Data == "a" {
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						processURL(attr.Val, baseURL, linkMap, &links, nil, nil)
						break
					}
				}
			}
			if n.Data == "link" {
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						processURL(attr.Val, baseURL, linkMap, &links, assetMap, &assets)
						break
					}
				}
			}

			// Обработка изображений
			if n.Data == "img" {
				for _, attr := range n.Attr {
					if attr.Key == "src" {
						processURL(attr.Val, baseURL, nil, nil, assetMap, &assets)
						break
					}
				}
				// Также проверяем srcset
				for _, attr := range n.Attr {
					if attr.Key == "srcset" {
						// Простая обработка srcset - берем первый URL
						parts := strings.Split(attr.Val, ",")
						if len(parts) > 0 {
							firstPart := strings.TrimSpace(parts[0])
							urlPart := strings.Split(firstPart, " ")[0]
							processURL(urlPart, baseURL, nil, nil, assetMap, &assets)
						}
						break
					}
				}
			}

			// Обработка скриптов
			if n.Data == "script" {
				for _, attr := range n.Attr {
					if attr.Key == "src" {
						processURL(attr.Val, baseURL, nil, nil, assetMap, &assets)
						break
					}
				}
			}

			// Обработка стилей
			if n.Data == "link" {
				isStylesheet := false
				for _, attr := range n.Attr {
					if attr.Key == "rel" && attr.Val == "stylesheet" {
						isStylesheet = true
						break
					}
				}
				if isStylesheet {
					for _, attr := range n.Attr {
						if attr.Key == "href" {
							processURL(attr.Val, baseURL, nil, nil, assetMap, &assets)
							break
						}
					}
				}
			}

			// Обработка фоновых изображений в style атрибуте
			for _, attr := range n.Attr {
				if attr.Key == "style" {
					// Простое извлечение url() из style
					extractURLsFromStyle(attr.Val, baseURL, assetMap, &assets)
				}
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			traverse(child)
		}
	}

	traverse(doc)
	return links, assets, nil
}

// extractURLsFromStyle - извлечение URL из CSS
func extractURLsFromStyle(style string, baseURL *url.URL, assetMap map[string]bool, assets *[]Asset) {
	// Ищем url(...)
	start := 0
	for {
		urlIndex := strings.Index(style[start:], "url(")
		if urlIndex == -1 {
			break
		}
		urlIndex += start
		start = urlIndex + 4

		end := strings.Index(style[start:], ")")
		if end == -1 {
			break
		}
		end += start

		urlStr := style[start:end]
		// Убираем кавычки
		urlStr = strings.Trim(urlStr, "'\" \t\n\r")

		if urlStr != "" {
			processURL(urlStr, baseURL, nil, nil, assetMap, assets)
		}

		start = end + 1
	}
}

func processURL(hrefAttr string, baseURL *url.URL, linkMap map[string]bool, links *[]string, assetMap map[string]bool, assets *[]Asset) {
	if hrefAttr == "" {
		return
	}

	// Пропускаем специальные схемы
	if strings.HasPrefix(hrefAttr, "#") ||
		strings.HasPrefix(hrefAttr, "javascript:") ||
		strings.HasPrefix(hrefAttr, "mailto:") ||
		strings.HasPrefix(hrefAttr, "tel:") {
		return
	}

	// Обработка data: URL
	if strings.HasPrefix(hrefAttr, "data:") {
		if assets != nil && assetMap != nil {
			assetType := detectAssetType(hrefAttr)
			// Для data URL не добавляем в список для проверки,
			// но можем учесть как ассет
			if !assetMap[hrefAttr] {
				assetMap[hrefAttr] = true
				*assets = append(*assets, Asset{URL: hrefAttr, Type: assetType})
			}
		}
		return
	}

	parsed, err := baseURL.Parse(hrefAttr)
	if err != nil {
		return
	}

	parsed.Fragment = ""
	normalizedURL := parsed.String()

	if links != nil && linkMap != nil && !linkMap[normalizedURL] {
		linkMap[normalizedURL] = true
		*links = append(*links, normalizedURL)
	}

	if assets != nil && assetMap != nil && !assetMap[normalizedURL] {
		assetMap[normalizedURL] = true
		assetType := detectAssetType(normalizedURL)
		*assets = append(*assets, Asset{URL: normalizedURL, Type: assetType})
	}
}

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
				seo.Title = decodeHTMLEntities(strings.TrimSpace(n.FirstChild.Data))
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
					seo.Description = decodeHTMLEntities(strings.TrimSpace(content))
				}
			}

			if n.Data == "h1" && n.FirstChild != nil {
				seo.HasH1 = true
				seo.H1 = decodeHTMLEntities(strings.TrimSpace(n.FirstChild.Data))
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			traverse(child)
		}
	}

	traverse(doc)
	return seo
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

	// Для корневого пути оставляем слеш
	if u.Path == "" {
		u.Path = "/"
	} else {
		u.Path = strings.TrimRight(u.Path, "/")
		if u.Path == "" {
			u.Path = "/"
		}
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
