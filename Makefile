.PHONY: help install dev dev-docker build clean check-deps check-docker test

# Default target
.DEFAULT_GOAL := help

# Colors for output
CYAN := \033[0;36m
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

help: ## Show this help message
	@echo "$(CYAN)LogDeck Development Commands$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}'
	@echo ""

check-deps: ## Check if required dependencies are installed
	@echo "$(CYAN)Checking dependencies...$(NC)"
	@command -v go >/dev/null 2>&1 || { echo "$(RED)✗ Go is not installed. Please install Go 1.22+$(NC)"; exit 1; }
	@command -v bun >/dev/null 2>&1 || { echo "$(RED)✗ Bun is not installed. Please install Bun from https://bun.sh$(NC)"; exit 1; }
	@echo "$(GREEN)✓ Go version: $$(go version)$(NC)"
	@echo "$(GREEN)✓ Bun version: $$(bun --version)$(NC)"
	@command -v air >/dev/null 2>&1 || { echo "$(YELLOW)⚠ Air (Go hot-reload) is not installed. Installing...$(NC)"; go install github.com/air-verse/air@latest; }
	@command -v air >/dev/null 2>&1 && echo "$(GREEN)✓ Air (hot-reload) is installed$(NC)" || { echo "$(YELLOW)⚠ Air installation may have failed. Add Go bin to PATH:$(NC)"; echo "$(YELLOW)  export PATH=\$$PATH:\$$(go env GOPATH)/bin$(NC)"; }

check-docker: ## Check if Docker is running
	@echo "$(CYAN)Checking Docker...$(NC)"
	@docker info >/dev/null 2>&1 || { echo "$(RED)✗ Docker is not running. Please start Docker Desktop$(NC)"; exit 1; }
	@echo "$(GREEN)✓ Docker is running$(NC)"

install: check-deps ## Install frontend dependencies
	@echo "$(CYAN)Installing frontend dependencies...$(NC)"
	@cd frontend && bun install
	@echo "$(GREEN)✓ Dependencies installed$(NC)"
	@echo ""
	@echo "$(YELLOW)Note: Backend (Go) dependencies will be downloaded automatically when you run 'make dev'$(NC)"

dev: check-deps check-docker ## Run both backend and frontend in development mode (native)
	@echo "$(CYAN)Starting LogDeck in development mode with hot-reload...$(NC)"
	@echo "$(YELLOW)Backend will run on http://localhost:8080 (with Air hot-reload)$(NC)"
	@echo "$(YELLOW)Frontend will run on http://localhost:5173 (with Vite HMR)$(NC)"
	@echo ""
	@trap 'kill 0' EXIT; \
	(cd server && air) & \
	(cd frontend && bun run dev) & \
	wait

dev-docker: check-docker ## Run both services using Docker Compose
	@echo "$(CYAN)Starting LogDeck with Docker Compose (hot-reload enabled)...$(NC)"
	@echo "$(YELLOW)Backend will run on http://localhost:8080 (with Air hot-reload)$(NC)"
	@echo "$(YELLOW)Frontend will run on http://localhost:5173 (with Vite HMR)$(NC)"
	@echo ""
	@docker compose up

dev-docker-build: check-docker ## Build and run with Docker Compose (rebuild images)
	@echo "$(CYAN)Building and starting LogDeck with Docker Compose...$(NC)"
	@docker compose up --build

build: ## Build both backend and frontend for production
	@echo "$(CYAN)Building backend...$(NC)"
	@cd server && go build -o ../bin/logdeck-server cmd/server/main.go
	@echo "$(GREEN)✓ Backend built: bin/logdeck-server$(NC)"
	@echo ""
	@echo "$(CYAN)Building frontend...$(NC)"
	@cd frontend && bun run build
	@echo "$(GREEN)✓ Frontend built: frontend/dist/$(NC)"

clean: ## Clean build artifacts and dependencies
	@echo "$(CYAN)Cleaning build artifacts...$(NC)"
	@rm -rf bin/
	@rm -rf frontend/dist/
	@rm -rf frontend/node_modules/
	@echo "$(GREEN)✓ Cleaned$(NC)"

test: ## Run tests
	@echo "$(CYAN)Running backend tests...$(NC)"
	@cd server && go test ./...
	@echo ""
	@echo "$(CYAN)Running frontend tests...$(NC)"
	@cd frontend && bun test

stop-docker: ## Stop Docker Compose services
	@echo "$(CYAN)Stopping Docker Compose services...$(NC)"
	@docker compose down
