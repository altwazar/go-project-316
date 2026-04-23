build:
	go build -o bin/hexlet-go-crawler ./cmd/hexlet-go-crawler/main.go

test:
	go test -v ./... -race -coverprofile=coverage.out

test-coverage:
	go test -v ./... -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run

run:
	go run cmd/hexlet-go-crawler/main.go $(URL) || true

.PHONY: build test test-coverage benchmark lint run
