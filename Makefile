.PHONY: build test test-v lint run clean docker-build docker-up docker-down migrate-up migrate-down migrate-version migrate-create mocks coverage help

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
COVERPKG = $(shell go list ./... | grep -v -e /mocks -e /cmd/ -e /tests/ | paste -sd, -)

coverage:
	go test ./... -race -coverpkg=$(COVERPKG) -coverprofile=coverage.out -covermode=atomic -count=1
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

# Database migrations (golang-migrate). The API also applies them at startup
# unless SPG_DATABASE_AUTO_MIGRATE=false.
MIGRATE := go run -tags pgx5 github.com/golang-migrate/migrate/v4/cmd/migrate@v4.20.1
MIGRATE_DB = $(subst postgres://,pgx5://,$(DATABASE_URL))

migrate-up:
	$(MIGRATE) -path db/migrations -database "$(MIGRATE_DB)" up

migrate-down:
	$(MIGRATE) -path db/migrations -database "$(MIGRATE_DB)" down 1

migrate-version:
	$(MIGRATE) -path db/migrations -database "$(MIGRATE_DB)" version

migrate-create:
	@test -n "$(name)" || (echo "usage: make migrate-create name=add_something" && exit 1)
	$(MIGRATE) create -ext sql -dir db/migrations -seq -digits 3 $(name)

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
	@echo "  migrate-up      - Apply pending migrations (needs DATABASE_URL)"
	@echo "  migrate-down    - Roll back the latest migration"
	@echo "  migrate-version - Show the current schema version"
	@echo "  migrate-create  - Create a new migration: make migrate-create name=..."
	@echo "  mocks        - Regenerate mock files"
	@echo "  clean        - Remove build artifacts"
