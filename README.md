# Video Analytics

Система мониторинга пиратских видео-сайтов.

## Quick Start

```bash
make help          # Показать все команды
make up            # Запустить всё
make logs s=indexer  # Логи сервиса
```

## Deploy

### Dev (home-server)

```bash
# Все сервисы
docker --context home-server compose up -d --build

# Отдельные сервисы
docker --context home-server compose up -d --build indexer
docker --context home-server compose up -d --build parser
docker --context home-server compose up -d --build frontend

# Force recreate
docker --context home-server compose up -d --force-recreate indexer parser frontend
```

### Indexer nodes

```bash
# va-indexer-1
docker --context va-indexer-1 compose up -d --build parser

# va-indexer-2
docker --context va-indexer-2 compose up -d --build parser
```

### Production

```bash
docker --context va-prod compose -f docker-compose.prod.yml up -d
```

## Make Commands

```bash
make up            # Start all services
make down          # Stop all services
make build         # Build all images
make rebuild       # Rebuild and restart app services

make indexer       # Rebuild and restart indexer
make parser        # Rebuild and restart parser
make frontend      # Rebuild and restart frontend

make infra         # Start only infrastructure (mongodb, redis, meilisearch)
make dev           # Run in dev mode (local backend + frontend)

make logs s=indexer  # Show logs for service
make reset         # Reset all data
make swagger       # Regenerate Swagger docs
make test          # Run backend tests
```

## Docker Contexts

| Context | Endpoint | Description |
|---------|----------|-------------|
| home-server | ssh://192.168.2.2 | Dev server |
| va-indexer-1 | ssh://root@91.208.184.173 | Indexer node 1 |
| va-indexer-2 | ssh://root@146.19.213.36 | Indexer node 2 |
| va-prod | ssh://root@146.19.213.178 | Production |
