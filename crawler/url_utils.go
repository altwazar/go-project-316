package crawler

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	// defaultHTTPPort - стандартный порт HTTP
	defaultHTTPPort = "80"
	// defaultHTTPSPort - стандартный порт HTTPS
	defaultHTTPSPort = "443"
)

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
	return (scheme == DefaultScheme && port == defaultHTTPPort) ||
		(scheme == "https" && port == defaultHTTPSPort)
}

// normalizePath - нормализация пути
func normalizePath(u *url.URL) {
	// if u.Path == "" {
	// 	u.Path = "/"
	// 	return
	// }

	u.Path = strings.TrimRight(u.Path, "/")
	// if u.Path == "" {
	// 	u.Path = "/"
	// }
}

// normalizeQuery - нормализация параметров запроса
func normalizeQuery(u *url.URL) {
	if u.RawQuery == "" {
		return
	}

	values := u.Query()
	u.RawQuery = values.Encode()
}

// isSameDomain - проверка принадлежности к одному домену
func isSameDomain(urlStr1, urlStr2 string) bool {
	u1, _ := url.Parse(urlStr1)
	u2, _ := url.Parse(urlStr2)
	return u1.Host == u2.Host
}

// normalizeOrKeep - нормализация с fallback на исходную строку
func normalizeOrKeep(urlStr string) string {
	normalizedURL, err := normalizeURL(urlStr)
	if err != nil {
		return urlStr
	}
	return normalizedURL
}

// validateAndNormalizeURL - валидация и нормализация URL для внешних запросов
func validateAndNormalizeURL(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme == "" {
		parsedURL.Scheme = DefaultScheme
	}

	return parsedURL.String(), nil
}
