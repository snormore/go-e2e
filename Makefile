.PHONY: test
test:
	@echo "=== Running tests..."
	go test ./... -v
	@echo "=== Running tests with race detector..."
	go test ./... -race

.PHONY: lint
lint:
	@echo "=== Running linter..."
	golangci-lint run ./...

.PHONY: build
build:
	@echo "=== Building..."
	go build ./...

.PHONY: run-help
run-help:
	go run ./cmd/go-e2e/main.go --help

.PHONY: run-example
run-example:
	@echo "=== Running example..."
	cd example && go tool go-e2e

.PHONY: checks
checks: lint test build run-example
