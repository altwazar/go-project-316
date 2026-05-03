package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// checkAsset - проверка ассета и получение его размера
func checkAsset(ctx context.Context, urlStr string, assetType AssetType, client *http.Client, userAgent string) (Asset, error) {
	asset := createBaseAsset(urlStr, assetType)

	validatedURL, err := validateAndNormalizeURL(urlStr)
	if err != nil {
		asset.Error = err.Error()
		return asset, err
	}

	// Пробуем получить информацию об ассете (сначала HEAD, потом GET при необходимости)
	statusCode, contentLength, err := getAssetInfo(ctx, validatedURL, client, userAgent)
	if err != nil {
		asset.Error = fmt.Sprintf("request failed: %v", err)
		return asset, err
	}

	asset.StatusCode = statusCode

	if isSuccessStatusCode(statusCode) {
		asset.SizeBytes = contentLength
	} else {
		asset.Error = fmt.Sprintf("HTTP %d", statusCode)
	}

	return asset, nil
}

// getAssetInfo - получение информации об ассете (статус и размер)
// Сначала пробует HEAD, при необходимости (405/501 или ошибка) использует GET
func getAssetInfo(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, int64, error) {
	// Пробуем HEAD запрос
	statusCode, contentLength, err := doHeadRequestForAsset(ctx, urlStr, client, userAgent)
	if err == nil {
		// HEAD вернул ответ, но если метод не поддерживается сервером,
		// пробуем GET запрос (он может работать)
		if statusCode == http.StatusMethodNotAllowed || statusCode == http.StatusNotImplemented {
			return doGetRequestForAsset(ctx, urlStr, client, userAgent)
		}
		return statusCode, contentLength, nil
	}

	// Если HEAD вернул транспортную ошибку (таймаут, отказ соединения и т.д.),
	// тоже пробуем GET - возможно, проблема только с HEAD
	return doGetRequestForAsset(ctx, urlStr, client, userAgent)
}

// doHeadRequestForAsset - выполнение HEAD запроса для ассета
func doHeadRequestForAsset(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, int64, error) {
	req, err := newRequestWithUserAgent(ctx, http.MethodHead, urlStr, userAgent)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return resp.StatusCode, resp.ContentLength, nil
}

// doGetRequestForAsset - выполнение GET запроса для ассета и получение его размера
func doGetRequestForAsset(ctx context.Context, urlStr string, client *http.Client, userAgent string) (int, int64, error) {
	req, err := newRequestWithUserAgent(ctx, http.MethodGet, urlStr, userAgent)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create GET request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Получаем размер контента
	size, err := getContentSize(resp)
	if err != nil {
		return resp.StatusCode, 0, err
	}

	return resp.StatusCode, size, nil
}

// createBaseAsset - создание базовой структуры ассета
func createBaseAsset(urlStr string, assetType AssetType) Asset {
	return Asset{
		URL:        urlStr,
		Type:       assetType,
		StatusCode: 0,
		SizeBytes:  0,
	}
}

// isSuccessStatusCode - проверка успешного статуса
func isSuccessStatusCode(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// getContentSize - получение размера контента
func getContentSize(resp *http.Response) (int64, error) {
	if resp.ContentLength >= 0 {
		return resp.ContentLength, nil
	}

	// Читаем тело для определения размера
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxAssetSize))
	if err != nil {
		return 0, fmt.Errorf("failed to read body: %w", err)
	}

	return int64(len(body)), nil
}
