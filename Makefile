.PHONY: help test test-integration build clean lint fmt vet tidy run-examples docker-rabbitmq

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development
fmt: ## Format Go code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run

tidy: ## Tidy go modules
	go mod tidy
	go mod verify

# Testing
test: ## Run unit tests
	go test -v -race -short ./...

test-integration: ## Run integration tests (requires RabbitMQ)
	go test -v -race ./...

test-coverage: ## Run tests with coverage
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Build
build: ## Build examples
	go build -o bin/publisher examples/publisher/main.go
	go build -o bin/consumer examples/consumer/main.go
	go build -o bin/advanced examples/advanced/main.go

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html

# Docker
docker-rabbitmq: ## Start RabbitMQ in Docker with streams plugin enabled
	docker run -d --name rabbitmq-dev \
		-p 5672:5672 \
		-p 15672:15672 \
		-e RABBITMQ_DEFAULT_USER=guest \
		-e RABBITMQ_DEFAULT_PASS=guest \
		rabbitmq:4-management
	@echo "Waiting for RabbitMQ to start..."
	@sleep 10
	@echo "Enabling RabbitMQ streams plugin..."
	docker exec rabbitmq-dev rabbitmq-plugins enable rabbitmq_stream
	@echo "RabbitMQ with streams plugin is ready!"

docker-stop: ## Stop RabbitMQ Docker container
	docker stop rabbitmq-dev || true
	docker rm rabbitmq-dev || true

# Examples
run-publisher: ## Run publisher example
	go run examples/publisher/main.go

run-consumer: ## Run consumer example
	go run examples/consumer/main.go

run-advanced: ## Run advanced example
	go run examples/advanced/main.go

run-connection-names: ## Run connection names example
	go run examples/connection-names/main.go

run-reconnection-test: ## Run auto-reconnection test example
	go run examples/reconnection-test/main.go

run-production-queues: ## Run production-ready queues example
	go run examples/production-queues/main.go

# CI
ci: tidy fmt vet test ## Run CI pipeline
