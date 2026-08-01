.PHONY: test test-server test-client test-coverage test-verbose clean build-server build-client build-app build release release-test lint mod-tidy pre-commit ci help

.DEFAULT_GOAL := help

help:
	@echo 'Usage: make <target>'
	@echo 'Targets: test test-server test-coverage test-verbose test-count clean build-server build-client build-app build release release-test lint mod-tidy pre-commit ci'

test:
	@echo "Running all tests..."
	@go test ./server/...

test-server:
	@echo "Running server tests..."
	@go test -v ./server/...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.txt -covermode=atomic ./server/...
	@go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-verbose:
	@echo "Running tests with race detection..."
	@go test -v -race ./server/...

test-count:
	@echo "Counting tests..."
	@go test -v ./server/... 2>&1 | grep -c "^=== RUN" || echo "0"

clean:
	@echo "Cleaning..."
	@rm -rf ./builds
	@rm -f coverage.txt coverage.html
	@go clean -testcache
	@echo "Clean complete"

build-server:
	@echo "Building tunnels-server..."
	@mkdir -p builds
	@cd server && go build -o ../builds/tunnels-server .

build-client:
	@echo "Building tunnels-cli..."
	@mkdir -p builds
	@cd cmd/main && go build -o ../../builds/tunnels-cli .

build-app:
	@echo "Building tunnels-app..."
	@./build-wails.sh

build: build-server build-client
	@echo "Build complete"

release:
	@echo "Running goreleaser snapshot..."
	@goreleaser release --snapshot --clean

release-test:
	@echo "Testing goreleaser configuration..."
	@goreleaser check
	@echo "Configuration valid!"

lint:
	@echo "Running linter..."
	@golangci-lint run --timeout=10m --config .golangci.yml

mod-tidy:
	@echo "Tidying go modules..."
	@go mod tidy
	@go mod verify

pre-commit: mod-tidy test lint
	@echo "Pre-commit checks passed!"

ci: test lint
	@echo "CI checks passed!"
