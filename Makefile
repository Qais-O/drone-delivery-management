.PHONY: build run test clean lint fmt proto help docker-build

# Build variables
BINARY_NAME=drone-app
BUILD_DIR=.
LDFLAGS=-ldflags="-s -w"

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: ## Build the application
	@echo "Building $(BINARY_NAME)..."
	@go build -tags grpcserver -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server
	@echo "✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

run: build ## Build and run the application
	@echo "Running $(BINARY_NAME)..."
	@./$(BINARY_NAME)

dev: ## Run in development mode with hot reload (requires entr)
	@echo "Running in development mode..."
	@find . -name "*.go" | entr -r make run

test: ## Run all tests
	@echo "Running tests..."
	@go test -tags grpcserver -v -race -timeout 60s ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -tags grpcserver -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "✓ Coverage report: coverage.out"

coverage-html: test-coverage ## Generate and open HTML coverage report
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"
	@open coverage.html 2>/dev/null || xdg-open coverage.html 2>/dev/null || echo "Please open coverage.html manually"

fmt: ## Format code with gofmt
	@echo "Formatting code..."
	@gofmt -s -w .
	@echo "✓ Code formatted"

lint: ## Run golangci-lint
	@echo "Linting code..."
	@golangci-lint run ./...
	@echo "✓ Linting complete"

vet: ## Run go vet
	@echo "Vetting code..."
	@go vet ./...
	@echo "✓ Vetting complete"

proto: ## Generate code from .proto files
	@echo "Generating protobuf code..."
	@protoc --go_out=. --go-grpc_out=. ./api/**/*.proto
	@echo "✓ Protobuf generation complete"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@go clean -testcache
	@echo "✓ Clean complete"

deps: ## Download and verify dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod verify
	@echo "✓ Dependencies verified"

mod-tidy: ## Tidy and vendor dependencies
	@echo "Tidying modules..."
	@go mod tidy
	@go mod vendor
	@echo "✓ Modules tidied"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t $(BINARY_NAME):latest .
	@echo "✓ Docker image built: $(BINARY_NAME):latest"

docker-run: docker-build ## Build and run Docker container
	@echo "Running Docker container..."
	@docker run -e JWT_SECRET="dev-secret" -p 50051:50051 $(BINARY_NAME):latest

check: fmt vet lint test ## Run all checks (format, vet, lint, test)
	@echo "✓ All checks passed"

db-migrate: ## Run database migrations (automatic on startup)
	@echo "Note: Migrations run automatically on startup"
	@echo "To add a migration, create files in: internal/db/migrations/NNNN_name.up.sql"

help-env: ## Show environment variables
	@echo "Environment variables (with defaults):"
	@echo "  JWT_SECRET              = dev-secret-change-me-in-production"
	@echo "  DB_PATH                 = app.db"
	@echo "  GRPC_ADDRESS            = :50051"
	@echo ""
	@echo "Example:"
	@echo "  JWT_SECRET=prod-secret DB_PATH=/var/lib/app/app.db make run"

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "✓ Tools installed"

# Default target
.DEFAULT_GOAL := help

