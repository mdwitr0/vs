# Player Monitor Backend - Implementation Summary

## Overview

Complete backend implementation for player-monitor project based on the existing indexer codebase template. The system monitors video streaming websites for player presence using configurable patterns.

## Technology Stack

- **Language:** Go 1.22
- **Web Framework:** Fiber v2
- **Database:** MongoDB
- **Authentication:** JWT (access + refresh tokens)
- **Scheduler:** gocron
- **Logging:** zerolog
- **Password Hashing:** bcrypt

## Architecture

```
backend/
├── cmd/server/              # Application entry point
│   └── main.go             # Server initialization and routing
├── internal/
│   ├── config/             # Configuration management
│   │   └── config.go       # Environment variables
│   ├── handler/            # HTTP request handlers
│   │   ├── auth.go         # Authentication (login, refresh, logout, me)
│   │   ├── user.go         # User management (CRUD)
│   │   ├── site.go         # Site management and scanning
│   │   ├── settings.go     # Settings management
│   │   ├── audit.go        # Audit log listing
│   │   └── common.go       # Common types (ErrorResponse, SuccessResponse)
│   ├── middleware/         # HTTP middleware
│   │   ├── auth.go         # JWT authentication and authorization
│   │   └── audit.go        # Audit logging middleware
│   ├── repo/               # MongoDB repositories
│   │   ├── user.go         # User repository
│   │   ├── refresh_token.go # Refresh token repository
│   │   ├── site.go         # Site repository
│   │   ├── page.go         # Page repository
│   │   ├── settings.go     # Settings repository (singleton)
│   │   └── audit_log.go    # Audit log repository
│   ├── scheduler/          # Scheduled tasks
│   │   └── scheduler.go    # Automatic site scanning
│   └── crawler/            # Web crawling logic
│       ├── crawler.go      # Main crawler orchestration
│       ├── parser.go       # Sitemap parsing and HTTP fetching
│       └── detector.go     # Page type detection
└── pkg/logger/             # Logging utilities
    └── logger.go           # Zerolog initialization
```

## Database Schema

### Users Collection
```json
{
  "_id": "ObjectId",
  "login": "string (unique)",
  "password_hash": "string (bcrypt)",
  "role": "enum(admin, user)",
  "is_active": "boolean",
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

Indexes:
- `login` (unique)

### Refresh Tokens Collection
```json
{
  "_id": "ObjectId",
  "user_id": "ObjectId (unique)",
  "token_hash": "string (bcrypt)",
  "expires_at": "timestamp",
  "created_at": "timestamp"
}
```

Indexes:
- `user_id` (unique)
- `expires_at`

### Sites Collection
```json
{
  "_id": "ObjectId",
  "domain": "string (unique)",
  "total_pages": "int64",
  "pages_with_player": "int64",
  "pages_without_player": "int64",
  "last_scan_at": "timestamp",
  "status": "enum(active, scanning, error)",
  "created_at": "timestamp"
}
```

Indexes:
- `domain` (unique)
- `status`
- `created_at`
- `last_scan_at`

### Pages Collection
```json
{
  "_id": "ObjectId",
  "site_id": "ObjectId",
  "url": "string",
  "has_player": "boolean",
  "page_type": "enum(content, catalog, static, error)",
  "exclude_from_report": "boolean",
  "last_checked_at": "timestamp"
}
```

Indexes:
- `(site_id, url)` (compound unique)
- `(site_id, has_player)` (compound)
- `(site_id, page_type)` (compound)
- `last_checked_at`

### Settings Collection (Singleton)
```json
{
  "_id": "ObjectId",
  "player_pattern": "string (regex)",
  "scan_interval_hours": "int",
  "updated_at": "timestamp",
  "updated_by": "ObjectId"
}
```

Default values:
- `player_pattern`: `<iframe[^>]*player[^>]*>|<video[^>]*>|<div[^>]*player[^>]*>`
- `scan_interval_hours`: 24

### Audit Logs Collection
```json
{
  "_id": "ObjectId",
  "user_id": "ObjectId",
  "action": "enum(domain_upload, settings_change, report_export, user_create, user_update)",
  "details": "object",
  "ip_address": "string",
  "created_at": "timestamp"
}
```

Indexes:
- `user_id`
- `action`
- `created_at`

## API Endpoints

### Public Endpoints
- `POST /api/auth/login` - User login
- `POST /api/auth/refresh` - Refresh access token

### Authenticated Endpoints
- `POST /api/auth/logout` - Logout (invalidate refresh token)
- `GET /api/auth/me` - Get current user info

### User Management (Admin Only)
- `GET /api/users` - List users with filtering
- `POST /api/users` - Create user
- `PUT /api/users/:id` - Update user
- `DELETE /api/users/:id` - Delete user
- `PATCH /api/users/:id/status` - Update user active status

### Site Management
- `GET /api/sites` - List sites with filtering and pagination
- `POST /api/sites` - Add single site
- `POST /api/sites/import` - Import sites from CSV file
- `POST /api/sites/:id/scan` - Start manual scan
- `GET /api/sites/:id` - Get site details
- `GET /api/sites/:id/pages` - Get site pages with filtering
- `GET /api/sites/:id/export` - Export pages without player (CSV)
- `DELETE /api/sites/:id` - Delete site and all its pages

### Settings Management (Admin Only)
- `GET /api/settings` - Get current settings
- `PUT /api/settings` - Update settings

### Audit Logs (Admin Only)
- `GET /api/audit-logs` - List audit logs with filtering

### Health Check
- `GET /health` - Health check endpoint

## Key Features

### 1. JWT Authentication
- Access tokens (15min default)
- Refresh tokens (7 days default)
- Secure token storage with bcrypt
- Role-based access control

### 2. Crawler System
- **Sitemap parsing:** Automatically discovers URLs from sitemap.xml
- **Recursive crawling:** Follows nested sitemaps
- **Player detection:** Uses configurable regex pattern
- **Page type detection:** Classifies pages as content/catalog/static/error
- **Rate limiting:** Built-in delays to avoid overwhelming servers
- **Error handling:** Graceful handling of network errors and timeouts

### 3. Scheduler
- Automatic periodic scans based on `scan_interval_hours`
- Dynamic interval updates without restart
- Singleton mode to prevent duplicate scans
- Context-aware cancellation

### 4. Audit Logging
- Automatic logging of sensitive operations
- IP address tracking
- User action tracking
- Admin visibility into system changes

### 5. Data Export
- CSV export of pages without player
- UTF-8 BOM for Excel compatibility
- Filtered exports (exclude from report flag)

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8080 | HTTP server port |
| MONGO_URL | mongodb://localhost:27017 | MongoDB connection URL |
| MONGO_DB | player_monitor | MongoDB database name |
| JWT_SECRET | - | JWT signing secret (required) |
| JWT_ACCESS_EXPIRY | 15m | Access token expiration |
| JWT_REFRESH_EXPIRY | 168h | Refresh token expiration |
| ADMIN_LOGIN | admin | Default admin username |
| ADMIN_PASSWORD | - | Default admin password (required) |

### Default Settings

On first run, creates default settings:
- Player pattern: `<iframe[^>]*player[^>]*>|<video[^>]*>|<div[^>]*player[^>]*>`
- Scan interval: 24 hours

## Security Features

### Authentication
- JWT tokens with HMAC-SHA256 signing
- Refresh token rotation
- Secure password storage with bcrypt (cost 10)
- Token expiration and validation

### Authorization
- Role-based access control (admin/user)
- Middleware-based protection
- Route-level permissions

### Audit Trail
- All critical operations logged
- IP address tracking
- User action tracking
- Admin-only visibility

### Input Validation
- Request body validation
- Domain sanitization
- Unique constraint enforcement

## Error Handling

### HTTP Status Codes
- `200` - Success
- `201` - Created
- `204` - No Content (delete operations)
- `400` - Bad Request (validation errors)
- `401` - Unauthorized (authentication required)
- `403` - Forbidden (insufficient permissions)
- `404` - Not Found
- `409` - Conflict (duplicate entries)
- `500` - Internal Server Error

### Error Response Format
```json
{
  "error": "descriptive error message"
}
```

## Performance Considerations

### Database
- Automatic index creation on startup
- Compound indexes for common queries
- Efficient pagination with offset/limit

### Crawler
- Rate limiting (500ms delay every 10 pages)
- 1000 page limit per scan
- 30s timeout per page request
- 10MB response size limit

### Concurrency
- Async scanning (non-blocking)
- Context cancellation support
- Goroutine-safe operations

## Testing Strategy

### Manual Testing
```bash
# Health check
curl http://localhost:8080/health

# Login
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"login":"admin","password":"admin123"}'

# List sites
curl http://localhost:8080/api/sites \
  -H "Authorization: Bearer <token>"
```

### Integration Testing
- Test with real MongoDB instance
- Test crawler with test websites
- Test scheduled tasks

## Deployment

### Development
```bash
go run ./cmd/server
```

### Production (Docker)
```bash
docker-compose up -d backend
```

### Build Binary
```bash
go build -o server ./cmd/server
./server
```

## Monitoring

### Logs
- Structured JSON logging in production
- Pretty console logging in development
- Error tracking with stack traces

### Health Check
- Simple `/health` endpoint
- Returns `{"status":"ok"}`

## Future Enhancements

### Potential Improvements
1. **Metrics:** Prometheus metrics for monitoring
2. **Caching:** Redis for frequently accessed data
3. **Queue System:** Message queue for async operations
4. **Webhooks:** Notifications on scan completion
5. **API Rate Limiting:** Protect against abuse
6. **Swagger Documentation:** Auto-generated API docs
7. **Unit Tests:** Comprehensive test coverage
8. **CI/CD:** Automated testing and deployment

## Files Created

### Core Application
- `cmd/server/main.go` - Main entry point
- `internal/config/config.go` - Configuration
- `pkg/logger/logger.go` - Logging utilities

### Handlers (6 files)
- `internal/handler/auth.go` - Authentication
- `internal/handler/user.go` - User management
- `internal/handler/site.go` - Site management
- `internal/handler/settings.go` - Settings management
- `internal/handler/audit.go` - Audit logs
- `internal/handler/common.go` - Common types

### Middleware (2 files)
- `internal/middleware/auth.go` - Authentication/authorization
- `internal/middleware/audit.go` - Audit logging

### Repositories (6 files)
- `internal/repo/user.go` - User data access
- `internal/repo/refresh_token.go` - Token data access
- `internal/repo/site.go` - Site data access
- `internal/repo/page.go` - Page data access
- `internal/repo/settings.go` - Settings data access
- `internal/repo/audit_log.go` - Audit log data access

### Crawler (3 files)
- `internal/crawler/crawler.go` - Main crawler logic
- `internal/crawler/parser.go` - Sitemap and page parsing
- `internal/crawler/detector.go` - Page type detection

### Scheduler
- `internal/scheduler/scheduler.go` - Scheduled scans

### Configuration Files
- `go.mod` - Go module definition
- `Dockerfile` - Docker image definition
- `.env.example` - Environment variables template
- `.gitignore` - Git ignore rules
- `Makefile` - Build and run commands

### Documentation
- `README.md` - Project overview and setup
- `API.md` - Complete API documentation
- `DEPLOYMENT.md` - Deployment guide
- `IMPLEMENTATION.md` - This file

**Total: 29 files**

## Dependencies

Main dependencies:
- `github.com/gofiber/fiber/v2` - HTTP framework
- `go.mongodb.org/mongo-driver` - MongoDB driver
- `github.com/golang-jwt/jwt/v5` - JWT tokens
- `golang.org/x/crypto/bcrypt` - Password hashing
- `github.com/rs/zerolog` - Structured logging
- `github.com/go-co-op/gocron` - Task scheduling

All dependencies are production-ready and actively maintained.

## Conclusion

The implementation provides a complete, production-ready backend for the player-monitor project with:
- ✅ JWT authentication with access and refresh tokens
- ✅ Role-based access control (admin/user)
- ✅ Complete CRUD for users, sites, and pages
- ✅ Automated site crawling and scanning
- ✅ Configurable player pattern detection
- ✅ Scheduled automatic scans
- ✅ CSV import/export functionality
- ✅ Audit logging
- ✅ Clean architecture following the existing indexer pattern
- ✅ All JSON fields in snake_case
- ✅ Comprehensive documentation

The codebase is ready for immediate use and can be extended with additional features as needed.
