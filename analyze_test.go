package code

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// isContextCanceledError - проверяет, является ли ошибка ошибкой отмены контекста
func isContextCanceledError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.Canceled) ||
		strings.Contains(err.Error(), "context canceled")
}

// TestSuccessfulRequest_WithHttptest - тест успешного запроса (200 OK)
func TestSuccessfulRequest_WithHttptest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Errorf("Expected path /, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 5 * time.Second

	ctx := context.Background()
	statusCode, err := GetPage(ctx, server.URL, client)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got: %d", http.StatusOK, statusCode)
	}
}

// TestNotFoundError_WithHttptest - тест ошибки 404 Not Found
func TestNotFoundError_WithHttptest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 5 * time.Second

	ctx := context.Background()
	statusCode, err := GetPage(ctx, server.URL, client)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if statusCode != http.StatusNotFound {
		t.Errorf("Expected status code %d, got: %d", http.StatusNotFound, statusCode)
	}
}

// TestServerError_WithHttptest - тест ошибки сервера 500
func TestServerError_WithHttptest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 5 * time.Second

	ctx := context.Background()
	statusCode, err := GetPage(ctx, server.URL, client)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if statusCode != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got: %d", http.StatusInternalServerError, statusCode)
	}
}

// TestVariousStatusCodes_WithHttptest - тест различных статус-кодов
func TestVariousStatusCodes_WithHttptest(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"204 No Content", http.StatusNoContent},
		{"301 Moved Permanently", http.StatusMovedPermanently},
		{"302 Found", http.StatusFound},
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"403 Forbidden", http.StatusForbidden},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"502 Bad Gateway", http.StatusBadGateway},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			client := server.Client()
			client.Timeout = 5 * time.Second

			ctx := context.Background()
			statusCode, err := GetPage(ctx, server.URL, client)

			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if statusCode != tc.statusCode {
				t.Errorf("Expected status code %d, got: %d", tc.statusCode, statusCode)
			}
		})
	}
}

// TestTimeout_WithHttptest - тест таймаута
func TestTimeout_WithHttptest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 1 * time.Second

	ctx := context.Background()
	_, err := GetPage(ctx, server.URL, client)

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

// TestCanceledContext_WithHttptest - тест отмены контекста ДО запроса
func TestCanceledContext_WithHttptest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := server.Client()
	client.Timeout = 5 * time.Second

	_, err := GetPage(ctx, server.URL, client)

	if err == nil {
		t.Error("Expected context canceled error, got nil")
	}

	if !isContextCanceledError(err) {
		t.Errorf("Expected context canceled error, got: %v", err)
	}
}

// TestCanceledContextDuringRequest_WithHttptest - тест отмены контекста во время запроса
func TestCanceledContextDuringRequest_WithHttptest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			return
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(1*time.Second, cancel)

	client := server.Client()
	client.Timeout = 5 * time.Second

	_, err := GetPage(ctx, server.URL, client)

	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// TestGetPageWithRetries_Success_WithHttptest - тест успешного запроса с ретраями
func TestGetPageWithRetries_Success_WithHttptest(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 5 * time.Second

	ctx := context.Background()
	statusCode, err := GetPageWithRetries(ctx, server.URL, client, 3, 100*time.Millisecond)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Errorf("Expected 200, got: %d", statusCode)
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request, got: %d", requestCount)
	}
}

// TestGetPageWithRetries_RetryOnError_WithHttptest - тест ретраев при ошибке
func TestGetPageWithRetries_RetryOnError_WithHttptest(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 5 * time.Second

	ctx := context.Background()
	statusCode, err := GetPageWithRetries(ctx, server.URL, client, 3, 100*time.Millisecond)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Errorf("Expected 200, got: %d", statusCode)
	}
	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got: %d", requestCount)
	}
}

// TestGetPageWithRetries_AllFail_WithHttptest - тест всех ретраев с ошибкой
func TestGetPageWithRetries_AllFail_WithHttptest(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 5 * time.Second

	ctx := context.Background()
	_, err := GetPageWithRetries(ctx, server.URL, client, 3, 100*time.Millisecond)

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if requestCount != 4 {
		t.Errorf("Expected 4 requests, got: %d", requestCount)
	}
}

// TestGetPageWithRetries_ContextCanceled_WithHttptest - тест отмены контекста во время ретраев
func TestGetPageWithRetries_ContextCanceled_WithHttptest(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(1*time.Second, cancel)

	client := server.Client()
	client.Timeout = 5 * time.Second

	_, err := GetPageWithRetries(ctx, server.URL, client, 3, 200*time.Millisecond)

	if err == nil {
		t.Error("Expected context error, got nil")
	}
}

// TestAnalyze_Success_WithHttptest - тест Analyze с успешным ответом
func TestAnalyze_Success_WithHttptest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	opts := Options{
		URL:         server.URL,
		Depth:       1,
		Retries:     1,
		Delay:       0,
		Timeout:     5 * time.Second,
		UserAgent:   "",
		Concurrency: 1,
		HTTPClient:  server.Client(),
	}

	ctx := context.Background()
	result, err := Analyze(ctx, opts)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Error("Expected result, got nil")
	}
}

// TestAnalyze_NotFound_WithHttptest - тест Analyze с 404
func TestAnalyze_NotFound_WithHttptest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	opts := Options{
		URL:         server.URL,
		Depth:       1,
		Retries:     1,
		Delay:       0,
		Timeout:     5 * time.Second,
		UserAgent:   "",
		Concurrency: 1,
		HTTPClient:  server.Client(),
	}

	ctx := context.Background()
	result, err := Analyze(ctx, opts)

	if err != nil {
		t.Errorf("Expected no error for 404, got: %v", err)
	}
	if result == nil {
		t.Error("Expected result, got nil")
	}
}

// TestAnalyze_WithRetries_WithHttptest - тест Analyze с ретраями
func TestAnalyze_WithRetries_WithHttptest(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount < 2 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	opts := Options{
		URL:         server.URL,
		Depth:       1,
		Retries:     3,
		Delay:       100 * time.Millisecond,
		Timeout:     5 * time.Second,
		UserAgent:   "",
		Concurrency: 1,
		HTTPClient:  server.Client(),
	}

	ctx := context.Background()
	result, err := Analyze(ctx, opts)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Error("Expected result, got nil")
	}
	if requestCount != 2 {
		t.Errorf("Expected 2 requests (first fail, second success), got: %d", requestCount)
	}
}

// TestAnalyze_Timeout_WithHttptest - тест Analyze с таймаутом
func TestAnalyze_Timeout_WithHttptest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 1 * time.Second

	opts := Options{
		URL:         server.URL,
		Depth:       1,
		Retries:     0,
		Delay:       0,
		Timeout:     1 * time.Second,
		UserAgent:   "",
		Concurrency: 1,
		HTTPClient:  client,
	}

	ctx := context.Background()
	_, err := Analyze(ctx, opts)

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

// TestGetPage_WithCustomHeaders_WithHttptest - тест с проверкой заголовков
func TestGetPage_WithCustomHeaders_WithHttptest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != "TestAgent/1.0" {
			t.Errorf("Expected User-Agent 'TestAgent/1.0', got: %s", r.UserAgent())
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "TestAgent/1.0")

	client := server.Client()
	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got: %d", resp.StatusCode)
	}
}

// TestContextPassedToHTTPRequest_WithHttptest - тест передачи контекста
func TestContextPassedToHTTPRequest_WithHttptest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context() == nil {
			t.Error("Request context is nil")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := server.Client()
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got: %d", resp.StatusCode)
	}
}

// TestMultipleRequests_WithHttptest - тест множественных запросов
func TestMultipleRequests_WithHttptest(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := server.Client()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		statusCode, err := GetPage(ctx, server.URL, client)
		if err != nil {
			t.Errorf("Request %d: unexpected error: %v", i, err)
		}
		if statusCode != http.StatusOK {
			t.Errorf("Request %d: expected 200, got: %d", i, statusCode)
		}
	}

	if requestCount != 5 {
		t.Errorf("Expected 5 requests, got: %d", requestCount)
	}
}

// BenchmarkGetPage_WithHttptest - бенчмарк для измерения производительности
func BenchmarkGetPage_WithHttptest(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := server.Client()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetPage(ctx, server.URL, client)
	}
}

// BenchmarkGetPageWithRetries_WithHttptest - бенчмарк для GetPageWithRetries
func BenchmarkGetPageWithRetries_WithHttptest(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := server.Client()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetPageWithRetries(ctx, server.URL, client, 3, 10*time.Millisecond)
	}
}
