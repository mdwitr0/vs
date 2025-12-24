.PHONY: dev build up down logs clean

# Development
dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

# Docker
build:
	docker compose build

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

logs-backend:
	docker compose logs -f backend

logs-frontend:
	docker compose logs -f frontend

# Clean
clean:
	docker compose down -v
	rm -rf backend/server
	rm -rf frontend/dist
	rm -rf frontend/node_modules

# Backend
backend-deps:
	cd backend && go mod download

backend-build:
	cd backend && go build -o server ./cmd/server

backend-test:
	cd backend && go test ./...

# Frontend
frontend-deps:
	cd frontend && npm install

frontend-build:
	cd frontend && npm run build

frontend-lint:
	cd frontend && npm run lint
