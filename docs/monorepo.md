# Структура монорепозитория

```
/video-analitics
├── frontend/                        # React SPA
├── backend/                         # Go workspace
├── docs/                            # Документация
└── docker-compose.yml
```

---

## Backend (Go Workspace)

```
backend/
├── go.work                          # Go workspace file
├── go.work.sum
├── Makefile
│
├── pkg/                             # Shared библиотеки (отдельный Go module)
│   ├── go.mod
│   ├── mongodb/                     # MongoDB клиент и утилиты
│   │   ├── client.go
│   │   └── errors.go
│   ├── httputil/                    # HTTP утилиты
│   │   ├── response.go              # Стандартные ответы API
│   │   ├── middleware.go            # Logging, recovery, cors
│   │   └── pagination.go
│   ├── logger/                      # zerolog wrapper
│   │   └── logger.go
│   └── config/                      # Базовая конфигурация
│       └── config.go
│
└── services/
    │
    ├── indexer/                     # Сервис индексации (MVP)
    │   ├── go.mod
    │   ├── Dockerfile
    │   ├── cmd/
    │   │   └── server/
    │   │       └── main.go
    │   │
    │   └── internal/
    │       ├── config/              # Конфигурация сервиса
    │       │
    │       ├── domain/              # Domain layer
    │       │   ├── entity/          # Site, Page, ScanTask
    │       │   └── repository/      # Repository interfaces
    │       │
    │       ├── usecase/             # Business logic
    │       │   ├── site/            # CRUD сайтов
    │       │   ├── scanner/         # Сканирование
    │       │   ├── detector/        # Детекция технологий
    │       │   ├── browser/         # chromedp
    │       │   └── scheduler/       # gocron
    │       │
    │       ├── infrastructure/      # Infrastructure layer
    │       │   ├── repository/      # MongoDB implementations
    │       │   └── http/            # HTTP клиент (resty)
    │       │
    │       └── delivery/            # API layer
    │           └── http/
    │               ├── router.go
    │               ├── handler/
    │               └── dto/
    │
    ├── search/                      # Позже
    │   ├── go.mod
    │   ├── Dockerfile
    │   ├── cmd/server/
    │   └── internal/
    │
    └── licenses/                    # Позже
        ├── go.mod
        ├── Dockerfile
        ├── cmd/server/
        └── internal/
```

### Go Workspace

**go.work:**
```go
go 1.22

use (
    ./pkg
    ./services/indexer
    ./services/search
    ./services/licenses
)
```

---

## Frontend (React + Vite)

```
frontend/
├── package.json
├── vite.config.ts
├── tsconfig.json
├── index.html
├── Dockerfile
│
├── public/
│
└── src/
    ├── main.tsx
    ├── App.tsx
    │
    ├── components/
    │   ├── ui/                      # shadcn/ui
    │   ├── layout/                  # Header, Sidebar, MainLayout
    │   └── shared/                  # StatusBadge, CSVUploader, etc
    │
    ├── features/
    │   ├── sites/                   # Управление сайтами
    │   │   ├── components/
    │   │   ├── hooks/
    │   │   ├── api/
    │   │   └── SitesPage.tsx
    │   │
    │   ├── content/                 # Управление контентом
    │   │   ├── components/
    │   │   ├── hooks/
    │   │   ├── api/
    │   │   └── ContentPage.tsx
    │   │
    │   └── dashboard/               # Опционально
    │
    ├── lib/
    │   ├── api.ts                   # Axios/fetch instance
    │   └── utils.ts
    │
    ├── hooks/
    │
    ├── types/
    │
    └── styles/
        └── globals.css
```

---

## Docs

```
docs/
├── architecture.md                  # Описание сервисов
├── design.mmd                       # Системный дизайн (Mermaid)
├── monorepo.md                      # Этот файл
├── code-style.md                    # Code style
│
└── plan/                            # Планы по задачам
    ├── indexer.md
    ├── search.md
    └── licenses.md
```

---

## Docker Compose

```
docker-compose.yml                   # Основной файл
```

Сервисы:
- `mongodb` - MongoDB 7.0
- `indexer` - :8080
- `search` - :8081 (позже)
- `licenses` - :8082 (позже)
- `frontend` - :3000
