.PHONY: up down build rebuild logs reset seed swagger \
        infra backend frontend indexer parser \
        dev dev-backend dev-frontend clean help \
        deploy-prod deploy-parser deploy-home

# === MAIN COMMANDS ===

up: ## Start all services (local dev)
	docker compose up -d

down: ## Stop all services
	docker compose down

build: ## Build all images
	docker compose build

rebuild: ## Rebuild and restart app services (keeps infra running)
	docker compose up -d --build --force-recreate indexer parser frontend

logs: ## Show logs (use: make logs s=indexer)
	@if [ -n "$(s)" ]; then \
		docker compose logs -f $(s); \
	else \
		docker compose logs -f; \
	fi

# === DEPLOY ===

deploy-prod: ## Deploy to va-prod (full stack)
	docker --context va-prod compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
	docker --context va-prod restart va-prod-nginx

deploy-prod-quick: ## Deploy to va-prod without rebuild
	docker --context va-prod compose -f docker-compose.prod.yml --env-file .env.prod up -d
	docker --context va-prod restart va-prod-nginx

deploy-parser-1: ## Deploy parser to va-indexer-1
	docker --context va-indexer-1 compose -f docker-compose.parser.yml --env-file .env.parser up -d --build

deploy-parser-2: ## Deploy parser to va-indexer-2
	docker --context va-indexer-2 compose -f docker-compose.parser.yml --env-file .env.parser up -d --build

deploy-parsers: deploy-parser-1 deploy-parser-2 ## Deploy parsers to all indexer nodes

deploy-home: ## Deploy to home-server (dev)
	docker --context home-server compose up -d --build

deploy-all: deploy-prod deploy-parsers ## Deploy everything (prod + all parsers)

# === LOGS (remote) ===

logs-prod: ## Show va-prod logs (use: make logs-prod s=indexer)
	@if [ -n "$(s)" ]; then \
		docker --context va-prod compose -f docker-compose.prod.yml logs -f $(s); \
	else \
		docker --context va-prod compose -f docker-compose.prod.yml logs -f; \
	fi

logs-parser-1: ## Show va-indexer-1 parser logs
	docker --context va-indexer-1 compose -f docker-compose.parser.yml logs -f parser

logs-parser-2: ## Show va-indexer-2 parser logs
	docker --context va-indexer-2 compose -f docker-compose.parser.yml logs -f parser

# === STATUS ===

status-prod: ## Show va-prod container status
	docker --context va-prod compose -f docker-compose.prod.yml ps

status-parsers: ## Show all parser nodes status
	@echo "=== va-indexer-1 ===" && docker --context va-indexer-1 compose -f docker-compose.parser.yml ps
	@echo "=== va-indexer-2 ===" && docker --context va-indexer-2 compose -f docker-compose.parser.yml ps

# === INFRASTRUCTURE ===

infra: ## Start only infrastructure (mongodb, redis, meilisearch)
	docker compose up -d mongodb redis meilisearch nats

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
