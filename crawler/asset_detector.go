package crawler

import "strings"

// detectAssetType - определение типа ассета по расширению и тегу
func detectAssetType(urlStr string) AssetType {
	urlStr = strings.ToLower(urlStr)

	// Data URL всегда изображение
	if strings.HasPrefix(urlStr, "data:image/") {
		return AssetTypeImage
	}

	return detectAssetByExtension(urlStr)
}

// detectAssetByExtension - определение типа по расширению
func detectAssetByExtension(urlStr string) AssetType {
	extensions := []extensionType{
		{patterns: []string{".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".ico", ".bmp", ".tiff", ".tif"}, assetType: AssetTypeImage},
		{patterns: []string{".js", ".mjs"}, assetType: AssetTypeScript},
		{patterns: []string{".css"}, assetType: AssetTypeStyle},
	}

	for _, ext := range extensions {
		if containsAnyExtension(urlStr, ext.patterns) {
			return ext.assetType
		}
	}

	return AssetTypeImage
}

// extensionType - структура для маппинга расширений
type extensionType struct {
	patterns  []string
	assetType AssetType
}

// containsAnyExtension - проверка наличия любого из расширений
func containsAnyExtension(urlStr string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.Contains(urlStr, ext) {
			return true
		}
	}
	return false
}
