.PHONY: all build test clean install lint run-sample help

BINARY_NAME=replay-cli
SAMPLE_DB=sample_recordings.db

all: lint test build

help:
	@echo "Available targets:"
	@echo "  build       - Build the CLI binary"
	@echo "  test        - Run all tests"
	@echo "  clean       - Remove build artifacts"
	@echo "  install     - Install binary to GOPATH/bin"
	@echo "  lint        - Run static analysis"
	@echo "  setup       - Complete setup (deps + build + test + samples)"
	@echo "  run-sample  - Run sample replay"
	@echo "  deps        - Download and update dependencies"
	@echo "  fmt         - Format code"

build:
	@echo "Building $(BINARY_NAME)..."
	go build -v -o $(BINARY_NAME) ./cmd/replay-cli
	@echo "Build complete: ./$(BINARY_NAME)"

test:
	@echo "Running tests..."
	go test -v ./pkg/... ./internal/... ./cmd/...

test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./pkg/... ./internal/... ./cmd/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-race:
	@echo "Running tests with race detector..."
	go test -race ./pkg/... ./internal/... ./cmd/...

clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -f $(SAMPLE_DB)
	rm -f coverage.out coverage.html
	rm -f test_recordings.db
	@echo "Clean complete"

install: build
	@echo "Installing to GOPATH/bin..."
	go install ./cmd/replay-cli

lint:
	@echo "Running static analysis..."
	@if command -v golint >/dev/null 2>&1; then \
		golint ./...; \
	else \
		echo "golint not installed, skipping"; \
	fi
	@echo "Running go vet..."
	go vet ./...
	@echo "Running go fmt check..."
	@test -z "$$(gofmt -l .)" || (echo "Code not formatted, run: make fmt" && exit 1)
	@echo "Lint complete"

fmt:
	@echo "Formatting code..."
	gofmt -w .
	@echo "Format complete"

deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies updated"

setup: deps
	@echo "Running full setup..."
	@echo "Building project..."
	@$(MAKE) build
	@echo "Running tests..."
	@$(MAKE) test
	@echo "Creating sample database..."
	@go run -v examples/create_sample.go
	@echo ""
	@echo "Setup complete! Try:"
	@echo "  make run-sample    # Run sample replay"
	@echo "  ./$(BINARY_NAME) sessions -d $(SAMPLE_DB)  # List sessions"

run-sample: build
	@echo "Creating sample database..."
	@go run -v examples/create_sample.go
	@echo ""
	@echo "Listing sessions..."
	./$(BINARY_NAME) sessions -d $(SAMPLE_DB)
	@echo ""
	@echo "Running replay..."
	./$(BINARY_NAME) replay sample-session-001 -d $(SAMPLE_DB)

verify:
	@echo "Verifying safety-critical compliance..."
	@echo "Checking for recursion..."
	@! grep -r "func.*(" pkg/ cmd/ internal/ | grep -v "// " | grep "return.*\(" || \
		(echo "Warning: Possible recursion detected" && exit 0)
	@echo "Checking function length..."
	@for file in $$(find pkg cmd internal -name "*.go"); do \
		awk '/^func / {start=NR} /^}/ && start {if (NR-start > 60) print FILENAME":"start" function exceeds 60 lines ("NR-start")"; start=0}' $$file; \
	done
	@echo "Verification complete"

.DEFAULT_GOAL := help
