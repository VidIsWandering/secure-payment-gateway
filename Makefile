.PHONY: build test lint run clean docker-build docker-up docker-down migrate coverage

# Variables
APP_NAME := spg-api
BUILD_DIR := bin
MAIN_PKG := ./cmd/api

# Build
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PKG)

# Run
run:
	go run $(MAIN_PKG)

# Test
test:
	go test ./... -race -count=1

test-v:
	go test ./... -race -count=1 -v

# Coverage
coverage:
	go test ./... -race -coverprofile=coverage.out -covermode=atomic -count=1
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Lint
lint:
	golangci-lint run ./...

# Docker
docker-build:
	docker build -t $(APP_NAME):latest .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Database migrations
migrate-up:
	psql "$$DATABASE_URL" -f db/migrations/001_init_schema.up.sql

migrate-down:
	psql "$$DATABASE_URL" -f db/migrations/001_init_schema.down.sql

# Mock generation
mocks:
	mockgen -source=internal/core/ports/repositories.go -destination=internal/core/ports/mocks/mock_repositories.go -package=mocks
	mockgen -source=internal/core/ports/services.go -destination=internal/core/ports/mocks/mock_services.go -package=mocks

# Clean
clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  run          - Run the application"
	@echo "  test         - Run tests with race detector"
	@echo "  test-v       - Run tests verbose"
	@echo "  coverage     - Run tests with coverage report"
	@echo "  lint         - Run golangci-lint"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-up    - Start docker-compose stack"
	@echo "  docker-down  - Stop docker-compose stack"
	@echo "  migrate-up   - Apply database migrations"
	@echo "  migrate-down - Rollback database migrations"
	@echo "  mocks        - Regenerate mock files"
	@echo "  clean        - Remove build artifacts"
