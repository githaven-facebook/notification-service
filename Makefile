.PHONY: all build test lint clean run docker-build docker-up docker-down migrate

BINARY_NAME=notification-service
BUILD_DIR=bin
CMD_DIR=cmd/notification

all: lint test build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)/...

test:
	@echo "Running tests..."
	go test -v -race -count=1 ./tests/...

test-unit:
	@echo "Running unit tests..."
	go test -v -race -count=1 ./tests/unit/...

test-integration:
	@echo "Running integration tests..."
	go test -v -race -count=1 -tags=integration ./tests/integration/...

lint:
	@echo "Running linter..."
	golangci-lint run ./...

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@go clean ./...

run:
	go run ./$(CMD_DIR)/... -config=config.yaml

docker-build:
	docker build -t $(BINARY_NAME):latest .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

migrate:
	@echo "Running migrations..."
	@for f in migrations/*.sql; do \
		echo "Applying $$f..."; \
		psql $${DB_URL} -f $$f; \
	done

tidy:
	go mod tidy

vet:
	go vet ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
