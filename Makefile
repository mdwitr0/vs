.PHONY: up down build rebuild logs reset seed swagger \
        infra backend frontend indexer parser \
        dev dev-backend dev-frontend clean help

# === MAIN COMMANDS ===

up: ## Start all services
	docker compose up -d

down: ## Stop all services
	docker compose down

build: ## Build all images
	docker compose build

rebuild: ## Rebuild and restart app services (keeps infra running)
	docker --context home-server compose up -d --force-recreate indexer parser frontend

logs: ## Show logs (use: make logs s=indexer)
	@if [ -n "$(s)" ]; then \
		docker compose logs -f $(s); \
	else \
		docker compose logs -f; \
	fi

# === INFRASTRUCTURE ===

infra: ## Start only infrastructure (mongodb, redis, meilisearch)
	docker compose up -d mongodb redis meilisearch

# === DEV MODE (local services + docker infra) ===

dev: infra dev-backend dev-frontend ## Run everything in dev mode

dev-backend: ## Run backend locally (requires infra)
	cd backend && go run ./indexer/cmd/main.go

dev-frontend: ## Run frontend locally
	cd frontend && npm run dev

# === INDIVIDUAL SERVICES ===

indexer: ## Rebuild and restart indexer
	docker compose up -d --build indexer

parser: ## Rebuild and restart parser
	docker compose up -d --build parser

frontend: ## Rebuild and restart frontend
	docker compose up -d --build frontend

# === DATABASE ===

reset: ## Reset all data (MongoDB + Meilisearch)
	cd backend && go run ./indexer/cmd/reset -y

seed: ## Seed test data
	cd backend && go run ./indexer/cmd/seed

# === BACKEND TOOLS ===

swagger: ## Regenerate Swagger docs
	cd backend && ~/go/bin/swag init -g cmd/main.go -d ./indexer --output ./indexer/docs --parseDependency --parseInternal

test: ## Run backend tests
	cd backend && go test ./...

lint: ## Lint backend
	cd backend && go vet ./...

# === FRONTEND TOOLS ===

frontend-install: ## Install frontend dependencies
	cd frontend && npm install

frontend-build: ## Build frontend for production
	cd frontend && npm run build

frontend-lint: ## Lint frontend
	cd frontend && npm run lint

# === CLEANUP ===

clean: ## Remove all containers, volumes, and build artifacts
	docker compose down -v --remove-orphans
	cd backend && rm -rf bin/
	cd frontend && rm -rf dist/ node_modules/.vite

clean-volumes: ## Remove only docker volumes (keeps images)
	docker compose down -v

# === HELP ===

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
