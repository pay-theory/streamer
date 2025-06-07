.PHONY: help test test-short test-integration test-coverage lint fmt clean build docker-dynamo

# Default target
help:
	@echo "Streamer Project - Available targets:"
	@echo "  make test              - Run all tests"
	@echo "  make test-short        - Run unit tests only"
	@echo "  make test-integration  - Run integration tests"
	@echo "  make test-coverage     - Run tests with coverage"
	@echo "  make lint              - Run linters"
	@echo "  make fmt               - Format code"
	@echo "  make clean             - Clean build artifacts"
	@echo "  make build             - Build the project"
	@echo "  make build-lambdas     - Build Lambda deployment packages"
	@echo "  make docker-dynamo     - Start local DynamoDB"
	@echo "  make create-tables     - Create DynamoDB tables locally"

# Test targets
test:
	go test -v ./...

test-short:
	go test -v -short ./...

test-integration:
	go test -v -run Integration ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Code quality
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

fmt:
	go fmt ./...
	go mod tidy

# Build
build:
	go build -v ./...

# Clean
clean:
	rm -f coverage.out coverage.html
	rm -rf lambda/*/deployment.zip
	go clean ./...

# Development helpers
docker-dynamo:
	@echo "Starting local DynamoDB..."
	docker run -d --name dynamodb-local -p 8000:8000 amazon/dynamodb-local || docker start dynamodb-local
	@echo "DynamoDB running on http://localhost:8000"

stop-dynamo:
	@echo "Stopping local DynamoDB..."
	docker stop dynamodb-local || true

# Create tables for local development
create-tables:
	@echo "Creating DynamoDB tables..."
	go run scripts/create_tables.go

# Dependencies
deps:
	go mod download
	go mod verify

# Check for security vulnerabilities
security:
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not installed. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi

# Run specific storage tests
test-store:
	go test -v ./internal/store/...

test-store-coverage:
	go test -v -coverprofile=store-coverage.out ./internal/store/...
	go tool cover -html=store-coverage.out -o store-coverage.html
	@echo "Storage layer coverage report: store-coverage.html"

# Lambda deployment targets
build-lambdas: build-lambda-connect build-lambda-disconnect build-lambda-router build-lambda-processor
	@echo "All Lambda deployment packages built successfully"

build-lambda-connect:
	@echo "Building connect Lambda..."
	@cd lambda/connect && \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap . && \
		zip deployment.zip bootstrap && \
		rm bootstrap

build-lambda-disconnect:
	@echo "Building disconnect Lambda..."
	@cd lambda/disconnect && \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap . && \
		zip deployment.zip bootstrap && \
		rm bootstrap

build-lambda-router:
	@echo "Building router Lambda..."
	@cd lambda/router && \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap . && \
		zip deployment.zip bootstrap && \
		rm bootstrap

build-lambda-processor:
	@echo "Building processor Lambda..."
	@cd lambda/processor && \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap . && \
		zip deployment.zip bootstrap && \
		rm bootstrap 