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
	go run main.go --help

.PHONY: run-example-passing
run-example-passing:
	@echo "=== Running example simple-passing..."
	cd examples/simple-passing && go tool go-e2e -vv

.PHONY: run-example-failing
run-example-failing:
	@echo "=== Running example simple-failing..."
	cd examples/simple-failing && go tool go-e2e -vv

.PHONY: checks
checks: lint test build run-example-passing
