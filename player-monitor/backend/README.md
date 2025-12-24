# Player Monitor Backend

Backend service for monitoring player presence on video streaming websites.

## Features

- JWT authentication (access + refresh tokens)
- Role-based access control (admin, user)
- Site and page management with automatic scanning
- Configurable player pattern detection
- Scheduled automatic scans
- CSV import/export
- Audit logging

## Architecture

```
backend/
├── cmd/server/          # Application entry point
├── internal/
│   ├── config/          # Configuration
│   ├── handler/         # HTTP handlers
│   ├── middleware/      # Auth and audit middleware
│   ├── repo/            # MongoDB repositories
│   ├── scheduler/       # Scheduled tasks
│   └── crawler/         # Site crawling and parsing
└── pkg/logger/          # Logging utilities
```

## Requirements

- Go 1.22+
- MongoDB 4.4+

## Installation

1. Copy environment file:
```bash
cp .env.example .env
```

2. Edit `.env` and set required values:
```bash
JWT_SECRET=your-secret-key-here
ADMIN_PASSWORD=secure-password
```

3. Install dependencies:
```bash
go mod download
```

4. Run the server:
```bash
go run ./cmd/server
```

## API Endpoints

### Authentication
- `POST /api/auth/login` - Login
- `POST /api/auth/refresh` - Refresh access token
- `POST /api/auth/logout` - Logout
- `GET /api/auth/me` - Get current user

### Users (admin only)
- `GET /api/users` - List users
- `POST /api/users` - Create user
- `PUT /api/users/:id` - Update user
- `DELETE /api/users/:id` - Delete user
- `PATCH /api/users/:id/status` - Update user status

### Sites
- `GET /api/sites` - List sites with filtering
- `POST /api/sites` - Add site
- `POST /api/sites/import` - Import sites from CSV
- `POST /api/sites/:id/scan` - Start site scan
- `GET /api/sites/:id` - Get site details
- `GET /api/sites/:id/pages` - Get site pages
- `GET /api/sites/:id/export` - Export pages without player (CSV)
- `DELETE /api/sites/:id` - Delete site

### Settings (admin only)
- `GET /api/settings` - Get settings
- `PUT /api/settings` - Update settings

### Audit Logs (admin only)
- `GET /api/audit-logs` - List audit logs

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8080 | Server port |
| MONGO_URL | mongodb://localhost:27017 | MongoDB connection URL |
| MONGO_DB | player_monitor | MongoDB database name |
| JWT_SECRET | - | JWT signing secret (required) |
| JWT_ACCESS_EXPIRY | 15m | Access token expiry |
| JWT_REFRESH_EXPIRY | 168h | Refresh token expiry (7 days) |
| ADMIN_LOGIN | admin | Admin username |
| ADMIN_PASSWORD | - | Admin password (required) |

## Docker

Build:
```bash
docker build -t player-monitor-backend .
```

Run:
```bash
docker run -p 8080:8080 \
  -e MONGO_URL=mongodb://mongo:27017 \
  -e JWT_SECRET=secret \
  -e ADMIN_PASSWORD=admin123 \
  player-monitor-backend
```

## Development

Run with auto-reload:
```bash
# Install air
go install github.com/cosmtrek/air@latest

# Run
air
```

## Testing

```bash
go test ./...
```

## License

MIT
