package models

const (
	// MaxHTMLBodySize - максимальный размер HTML-тела (10MB)
	MaxHTMLBodySize = 10 * 1024 * 1024

	// MaxAssetSize - максимальный размер ассета (50MB)
	MaxAssetSize = 50 * 1024 * 1024

	// DefaultScheme - схема по умолчанию
	DefaultScheme = "http"

	// DefaultWorkers - количество воркеров по умолчанию
	DefaultWorkers = 4

	// DefaultDepth - глубина по умолчанию
	DefaultDepth = 10

	// DefaultTimeout - таймаут по умолчанию
	DefaultTimeout = 15 // seconds

	// DefaultRetries - количество повторных попыток по умолчанию
	DefaultRetries = 1

	// MinTaskChannelSize - минимальный размер канала задач
	MinTaskChannelSize = 100

	// TaskChannelMultiplier - множитель для размера канала задач
	TaskChannelMultiplier = 10

	// StatusOK - статус "ok"
	StatusOK = "ok"

	// StatusError - статус "error"
	StatusError = "error"
)
