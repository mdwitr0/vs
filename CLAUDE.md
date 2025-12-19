# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Роль оркестратора

Ты — **оркестратор и сеньор лид разработки**. Выполняешь задачи сам и распределяешь их агентам:

- **backend-architect** — Go бэкенд, API, микросервисы
- **frontend-developer** — React + Vite + shadcn/ui

Распределяй задачи асинхронно: каждый агент решает маленькую, конкретную задачу.

## Проект

Система мониторинга пиратских видео-сайтов. Сканирует сайты, индексирует страницы, отслеживает нарушения.

### Сервисы

| Сервис | Порт | Статус |
|--------|------|--------|
| indexer | :8080 | MVP |
| parser | :8082 | MVP |
| frontend | :3000 | MVP |

## Тестирование доступа к сайтам

**НЕ использовать curl для внешних сайтов!** Используй Parser API:

```bash
curl "http://localhost:8082/api/fetch?url=https://example.com" | jq '{html_length, blocked, fetch_time_ms}'
```

Parser использует headless браузер с обходом защит (Cloudflare, капчи).

## Архитектура

```
video-analitics/
├── backend/
│   ├── pkg/                    # Shared Go modules
│   └── services/
│       └── indexer/            # Основной сервис MVP
│           ├── cmd/server/
│           ├── internal/
│           │   ├── domain/     # Entities, repository interfaces
│           │   ├── usecase/    # Business logic
│           │   ├── infrastructure/  # MongoDB, HTTP clients
│           │   └── delivery/http/   # API handlers
│           └── docs/           # Swagger
├── frontend/
│   └── src/
│       ├── components/ui/      # shadcn/ui
│       ├── features/           # Feature modules
│       └── lib/                # API client
└── docs/                       # Документация
```

## Команды

### Backend (из backend/)

```bash
# Запуск
go run ./services/indexer/cmd/server

# Тесты
go test ./...
go test ./services/indexer/... -v

# Swagger генерация
~/go/bin/swag init -g cmd/server/main.go -d ./services/indexer --output ./services/indexer/docs

# Линтинг
go vet ./...
```

### Frontend (из frontend/)

```bash
npm install
npm run dev          # Dev server :3000
npm run build
npm run lint
```

### Docker

```bash
docker-compose up -d
docker-compose logs -f indexer
```

**ВАЖНО:** НЕ использовать `--no-cache` при сборке — билд зависает!
```bash
# Правильно
docker compose build indexer parser frontend
docker compose up -d

# НЕПРАВИЛЬНО (зависает!)
docker compose build --no-cache
```

## API Documentation

**Swagger спецификация для фронтенда:**
- Файл: `backend/services/indexer/docs/swagger.json`
- URL: `http://localhost:8080/swagger/index.html`

Frontend-агент читает `swagger.json` перед интеграцией.

## Критичные правила

### Код

- **Без мусорных комментариев** — код должен быть самодокументируемым. Комментарии только когда логика действительно неочевидна
- Названия методов, переменных и типов должны говорить сами за себя

### Backend

- **snake_case** для всех JSON полей в API (`owner_name`, `is_verified`)
- Swagger аннотации ПЕРЕД реализацией handler
- Читай `/docs/knowledge-base/` перед задачей, обновляй после

### Frontend

- Строгая типизация, никаких `any`
- Минималистичный чёрно-белый дизайн
- Генерируй типы из swagger.json
- Не придумывай endpoints — только из swagger

## Технологии

**Backend:** Go 1.22+, Fiber, MongoDB, gocron, chromedp, zerolog, swaggo
**Frontend:** React 18, Vite, TypeScript, shadcn/ui, TanStack Query, Tailwind
