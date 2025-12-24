# Video Analytics

Система мониторинга пиратских видео-сайтов.

## Architecture

```
va-prod (146.19.213.178)
├── indexer      - API + task management
├── parser       - page parsing
├── frontend     - React UI
├── mongodb      - database
├── meilisearch  - search
├── nats         - message queue
└── nginx        - reverse proxy

va-indexer-1, va-indexer-2
└── parser       - remote parsing nodes (connect to va-prod)
```

## Quick Start

```bash
make help              # Show all commands
make deploy-prod       # Deploy to production
make deploy-parsers    # Deploy remote parsers
make logs-prod s=indexer  # View logs
```

## Deploy Commands

### Production (va-prod)

```bash
make deploy-prod       # Full rebuild + deploy
make deploy-prod-quick # Deploy without rebuild (config changes only)
make status-prod       # Check container status
make logs-prod         # All logs
make logs-prod s=indexer   # Indexer logs
make logs-prod s=parser    # Parser logs
```

### Remote Parsers

```bash
make deploy-parser-1   # Deploy to va-indexer-1
make deploy-parser-2   # Deploy to va-indexer-2
make deploy-parsers    # Deploy to all parser nodes
make logs-parser-1     # va-indexer-1 logs
make logs-parser-2     # va-indexer-2 logs
make status-parsers    # Check all parser nodes
```

### Deploy Everything

```bash
make deploy-all        # Deploy prod + all parsers
```

### Dev (home-server)

```bash
make deploy-home       # Deploy to home-server
```

## Configuration Files

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Local dev |
| `docker-compose.prod.yml` | Production (va-prod) |
| `docker-compose.parser.yml` | Remote parsers |
| `.env.prod` | Production secrets |
| `.env.parser` | Parser node config |

## Docker Contexts

| Context | Endpoint | Description |
|---------|----------|-------------|
| home-server | ssh://192.168.2.2 | Dev server |
| va-indexer-1 | ssh://root@91.208.184.173 | Parser node 1 |
| va-indexer-2 | ssh://root@146.19.213.36 | Parser node 2 |
| va-prod | ssh://root@146.19.213.178 | Production |

### Setup contexts

```bash
docker context create va-prod --docker "host=ssh://root@146.19.213.178"
docker context create va-indexer-1 --docker "host=ssh://root@91.208.184.173"
docker context create va-indexer-2 --docker "host=ssh://root@146.19.213.36"
```

## Local Development

```bash
make infra         # Start MongoDB, Meilisearch, NATS
make dev-backend   # Run indexer locally
make dev-frontend  # Run frontend locally
make dev           # Run everything in dev mode
```

## Other Commands

```bash
make test          # Run backend tests
make lint          # Lint backend
make swagger       # Regenerate Swagger docs
make reset         # Reset all data
make clean         # Full cleanup
```
