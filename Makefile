# File Storage Service Makefile

.PHONY: build run clean test docker-build docker-run docker-stop logs help

# Binary name
BINARY_NAME=file-storage-service

# Build the application
build:
	go build -o $(BINARY_NAME) -ldflags="-s -w" .

# Run the application locally
run:
	go run .

# Clean build artifacts
clean:
	go clean
	rm -f $(BINARY_NAME)

# Run tests
test:
	go test -v ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Docker commands
docker-build:
	docker-compose build

docker-run:
	docker-compose up -d

docker-stop:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Development commands
dev: docker-stop docker-run logs

# Production deployment
deploy:
	docker-compose -f compose.yml up -d --build

# Health check
health:
	curl -f http://localhost:8080/ || exit 1

# Show logs
logs:
	docker-compose logs -f app

# Help
help:
	@echo "Available commands:"
	@echo "  build        - Build the Go binary"
	@echo "  run          - Run the application locally"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  docker-build - Build Docker images"
	@echo "  docker-run   - Start services with Docker Compose"
	@echo "  docker-stop  - Stop Docker services"
	@echo "  docker-logs  - Show Docker logs"
	@echo "  dev          - Development mode (restart and show logs)"
	@echo "  deploy       - Production deployment"
	@echo "  health       - Check service health"
	@echo "  logs         - Show application logs"
	@echo "  help         - Show this help message"