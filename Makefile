build:
	go build -o bin/hexlet-go-crawler ./cmd/hexlet-go-crawler/main.go
test:
	go test -v ./... -race
run:
	go run cmd/hexlet-go-crawler/main.go $(URL) || true
