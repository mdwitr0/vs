# Auth System

> **Updated**: 2025-12-15

## Overview

JWT-based authentication system with access/refresh token pairs. Single active session per user via unique refresh tokens stored in MongoDB.

## Why

- Secure API access without session storage overhead
- Support for token refresh without re-authentication
- Role-based access control (RBAC) for admin functionality

## Components

### Config (`internal/config/config.go`)

Environment variables:
- `JWT_SECRET` - Required. HMAC signing key
- `JWT_ACCESS_EXPIRY` - Access token lifetime (default: 15m)
- `JWT_REFRESH_EXPIRY` - Refresh token lifetime (default: 168h/7d)
- `ADMIN_EMAIL` - Initial admin email (default: "admin")
- `ADMIN_PASSWORD` - Required for seeding first admin user

### Repositories

**UserRepo** (`internal/repo/user.go`):
- Unique email index
- Password stored as bcrypt hash
- Soft delete via `is_active` flag
- `SeedAdmin()` - creates first admin if no users exist

**RefreshTokenRepo** (`internal/repo/refresh_token.go`):
- One refresh token per user (user_id unique index)
- Token stored as bcrypt hash
- Manual expiry cleanup (no TTL index)

### Middleware (`internal/middleware/auth.go`)

- `AuthMiddleware(secret)` - Validates Bearer JWT, populates `c.Locals("user_id", "role")`
- `AdminOnly()` - Checks `role == "admin"`, returns 403 otherwise
- Helper functions: `GetUserID()`, `GetRole()`, `IsAdmin()`

### Handlers

**AuthHandler** (`internal/handler/auth.go`):
- `POST /api/auth/login` - Returns access + refresh tokens
- `POST /api/auth/refresh` - Exchange refresh token for new pair
- `POST /api/auth/logout` - Invalidates refresh token
- `GET /api/auth/me` - Returns current user profile

**UserHandler** (`internal/handler/user.go`):
- Admin-only CRUD for user management
- `GET /api/users` - List with filters
- `POST /api/users` - Create user
- `PUT /api/users/:id` - Update user
- `DELETE /api/users/:id` - Soft delete

## Key Decisions

### Single Session Per User
Refresh tokens use unique user_id index - logging in invalidates previous session. Simpler than device management, sufficient for admin-focused system.

### Refresh Token Format
Token embeds user ID (first 24 bytes) + random bytes. Allows lookup by user without token storage query. Hash comparison for security.

### Soft Delete
Users are deactivated (`is_active=false`) not deleted. Preserves audit trail, allows reactivation.

## Integration Notes

Routes must be registered in `cmd/main.go`:
1. Public routes: `/api/auth/login`, `/api/auth/refresh`
2. Protected routes: `/api/auth/logout`, `/api/auth/me` (require AuthMiddleware)
3. Admin routes: `/api/users/*` (require AuthMiddleware + AdminOnly)

Seed admin on startup:
```go
userRepo.SeedAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword)
```

## See Also

- `knowledge-base/backend/api.md` - API endpoints documentation
