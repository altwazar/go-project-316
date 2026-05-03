package urlutil

import (
	"fmt"
	"net/url"
	"strings"

	"code/internal/models"
)

const (
	defaultHTTPPort  = "80"
	defaultHTTPSPort = "443"
)

// NormalizeURL - нормализация URL
func NormalizeURL(rawURL string) (string, error) {
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

func normalizeSchemeAndHost(u *url.URL) {
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
}

func removeDefaultPort(u *url.URL) {
	if isDefaultPort(u.Scheme, u.Port()) {
		u.Host = strings.TrimSuffix(u.Host, ":"+u.Port())
	}
}

func isDefaultPort(scheme, port string) bool {
	return (scheme == models.DefaultScheme && port == defaultHTTPPort) ||
		(scheme == "https" && port == defaultHTTPSPort)
}

func normalizePath(u *url.URL) {
	// Оригинальная логика (закомментированная) сохранена
	u.Path = strings.TrimRight(u.Path, "/")
}

func normalizeQuery(u *url.URL) {
	if u.RawQuery == "" {
		return
	}
	values := u.Query()
	u.RawQuery = values.Encode()
}

// IsSameDomain - проверка принадлежности к одному домену
func IsSameDomain(urlStr1, urlStr2 string) bool {
	u1, _ := url.Parse(urlStr1)
	u2, _ := url.Parse(urlStr2)
	return u1.Host == u2.Host
}

// NormalizeOrKeep - нормализация с fallback на исходную строку
func NormalizeOrKeep(urlStr string) string {
	normalizedURL, err := NormalizeURL(urlStr)
	if err != nil {
		return urlStr
	}
	return normalizedURL
}

// ValidateAndNormalizeURL - валидация и нормализация URL для внешних запросов
func ValidateAndNormalizeURL(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = models.DefaultScheme
	}
	return parsedURL.String(), nil
}
