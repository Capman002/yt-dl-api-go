.PHONY: dev build test lint docker up down clean

# Development
dev:
	go run ./cmd/api

# Build
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/api ./cmd/api

# Tests
test:
	go test -v ./...

test-cover:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Linting
lint:
	golangci-lint run

# Docker
docker:
	docker build -t yt-dl-api-go .

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

# Cleanup
clean:
	rm -rf bin/
	rm -rf tmp/*
	rm -f coverage.out

# Mod
tidy:
	go mod tidy

deps:
	go mod download

# Help
help:
	@echo "Available targets:"
	@echo "  dev        - Run in development mode"
	@echo "  build      - Build binary to bin/api"
	@echo "  test       - Run tests"
	@echo "  test-cover - Run tests with coverage"
	@echo "  lint       - Run linter"
	@echo "  docker     - Build Docker image"
	@echo "  up         - Start with docker compose"
	@echo "  down       - Stop docker compose"
	@echo "  logs       - View docker compose logs"
	@echo "  clean      - Clean build artifacts"
	@echo "  tidy       - Run go mod tidy"
	@echo "  deps       - Download dependencies"
