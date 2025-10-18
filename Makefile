.PHONY: test test-server test-client test-coverage test-verbose clean build-server build-client build release help

# Default target
.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## test: Run all tests
test:
	@echo "Running all tests..."
	@go test ./server/...

## test-server: Run server tests with verbose output
test-server:
	@echo "Running server tests..."
	@go test -v ./server/...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.txt -covermode=atomic ./server/...
	@go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

## test-verbose: Run all tests with race detection
test-verbose:
	@echo "Running tests with race detection..."
	@go test -v -race ./server/...

## test-count: Count total number of tests
test-count:
	@echo "Counting tests..."
	@go test -v ./server/... 2>&1 | grep -c "^=== RUN" || echo "0"

## clean: Clean build artifacts and test cache
clean:
	@echo "Cleaning..."
	@rm -rf ./builds
	@rm -f coverage.txt coverage.html
	@go clean -testcache
	@echo "Clean complete"

## build-server: Build server binary
build-server:
	@echo "Building server..."
	@cd server && go build -o ../builds/server .

## build-client: Build client binary
build-client:
	@echo "Building client..."
	@cd cmd/main && go build -o ../../builds/tunnels .

## build: Build all binaries
build: build-server build-client
	@echo "Build complete"

## release: Run goreleaser in snapshot mode (no tag required)
release:
	@echo "Running goreleaser snapshot..."
	@goreleaser release --snapshot --clean

## release-test: Test goreleaser configuration
release-test:
	@echo "Testing goreleaser configuration..."
	@goreleaser check
	@echo "Configuration valid!"

## lint: Run golangci-lint
lint:
	@echo "Running linter..."
	@golangci-lint run --timeout=10m --config .golangci.yml

## mod-tidy: Tidy and verify go modules
mod-tidy:
	@echo "Tidying go modules..."
	@go mod tidy
	@go mod verify

## pre-commit: Run tests and linting before commit
pre-commit: mod-tidy test lint
	@echo "Pre-commit checks passed!"

## ci: Run CI pipeline locally (test + lint)
ci: test lint
	@echo "CI checks passed!"
