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

	resp, err := executeAssetRequest(ctx, validatedURL, client, userAgent)
	if err != nil {
		asset.Error = fmt.Sprintf("request failed: %v", err)
		return asset, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return processAssetResponse(asset, resp), nil
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

// processAssetResponse - обработка ответа для ассета
func processAssetResponse(asset Asset, resp *http.Response) Asset {
	asset.StatusCode = resp.StatusCode

	if isSuccessStatusCode(resp.StatusCode) {
		return processSuccessfulAsset(asset, resp)
	}

	asset.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	return asset
}

// isSuccessStatusCode - проверка успешного статуса
func isSuccessStatusCode(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// processSuccessfulAsset - обработка успешного ассета
func processSuccessfulAsset(asset Asset, resp *http.Response) Asset {
	size, err := getContentSize(resp)
	if err != nil {
		asset.SizeBytes = 0
		asset.Error = fmt.Sprintf("failed to get size: %v", err)
	} else {
		asset.SizeBytes = size
	}
	return asset
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
