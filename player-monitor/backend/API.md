# Player Monitor API Documentation

Base URL: `http://localhost:8080`

All authenticated endpoints require `Authorization: Bearer <token>` header.

## Authentication

### Login
```http
POST /api/auth/login
Content-Type: application/json

{
  "login": "admin",
  "password": "admin123"
}
```

Response:
```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "abc123...",
  "expires_in": 900
}
```

### Refresh Token
```http
POST /api/auth/refresh
Content-Type: application/json

{
  "refresh_token": "abc123..."
}
```

### Logout
```http
POST /api/auth/logout
Authorization: Bearer <token>
```

### Get Current User
```http
GET /api/auth/me
Authorization: Bearer <token>
```

## Users (Admin Only)

### List Users
```http
GET /api/users?limit=20&offset=0&role=user&is_active=true
Authorization: Bearer <admin_token>
```

Response:
```json
{
  "items": [
    {
      "id": "507f1f77bcf86cd799439011",
      "login": "user1",
      "role": "user",
      "is_active": true,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 1
}
```

### Create User
```http
POST /api/users
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "login": "newuser",
  "password": "password123",
  "role": "user"
}
```

### Update User
```http
PUT /api/users/:id
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "login": "updateduser",
  "password": "newpassword",
  "role": "admin",
  "is_active": true
}
```

### Delete User
```http
DELETE /api/users/:id
Authorization: Bearer <admin_token>
```

### Update User Status
```http
PATCH /api/users/:id/status
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "is_active": false
}
```

## Sites

### List Sites
```http
GET /api/sites?limit=20&offset=0&domain=example&status=active&sort_by=created_at&sort_order=desc
Authorization: Bearer <token>
```

Response:
```json
{
  "items": [
    {
      "id": "507f1f77bcf86cd799439011",
      "domain": "example.com",
      "total_pages": 100,
      "pages_with_player": 80,
      "pages_without_player": 20,
      "last_scan_at": "2024-01-01T00:00:00Z",
      "status": "active",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 1
}
```

### Get Site
```http
GET /api/sites/:id
Authorization: Bearer <token>
```

### Create Site
```http
POST /api/sites
Authorization: Bearer <token>
Content-Type: application/json

{
  "domain": "example.com"
}
```

### Import Sites from CSV
```http
POST /api/sites/import
Authorization: Bearer <token>
Content-Type: multipart/form-data

file: sites.csv
```

CSV format:
```csv
example.com
test.com
demo.org
```

Response:
```json
{
  "created": 2,
  "skipped": 1
}
```

### Scan Site
```http
POST /api/sites/:id/scan
Authorization: Bearer <token>
```

### Get Site Pages
```http
GET /api/sites/:id/pages?limit=20&offset=0&has_player=false&page_type=content&exclude_from_report=false
Authorization: Bearer <token>
```

Response:
```json
{
  "items": [
    {
      "id": "507f1f77bcf86cd799439011",
      "site_id": "507f1f77bcf86cd799439012",
      "url": "https://example.com/page1",
      "has_player": false,
      "page_type": "content",
      "exclude_from_report": false,
      "last_checked_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 1
}
```

### Export Pages Without Player (CSV)
```http
GET /api/sites/:id/export
Authorization: Bearer <token>
```

Returns CSV file with pages without player.

### Delete Site
```http
DELETE /api/sites/:id
Authorization: Bearer <token>
```

## Settings (Admin Only)

### Get Settings
```http
GET /api/settings
Authorization: Bearer <token>
```

Response:
```json
{
  "id": "507f1f77bcf86cd799439011",
  "player_pattern": "<iframe[^>]*player[^>]*>|<video[^>]*>",
  "scan_interval_hours": 24,
  "updated_at": "2024-01-01T00:00:00Z",
  "updated_by": "507f1f77bcf86cd799439012"
}
```

### Update Settings
```http
PUT /api/settings
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "player_pattern": "<video[^>]*>|<iframe[^>]*player[^>]*>",
  "scan_interval_hours": 12
}
```

## Audit Logs (Admin Only)

### List Audit Logs
```http
GET /api/audit-logs?limit=20&offset=0&user_id=507f1f77bcf86cd799439011&action=domain_upload
Authorization: Bearer <admin_token>
```

Response:
```json
{
  "items": [
    {
      "id": "507f1f77bcf86cd799439011",
      "user_id": "507f1f77bcf86cd799439012",
      "action": "domain_upload",
      "details": {},
      "ip_address": "127.0.0.1",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 1
}
```

## Status Codes

- `200` - Success
- `201` - Created
- `204` - No Content
- `400` - Bad Request
- `401` - Unauthorized
- `403` - Forbidden
- `404` - Not Found
- `409` - Conflict
- `500` - Internal Server Error

## Error Response Format

```json
{
  "error": "error message"
}
```

## Success Response Format

```json
{
  "message": "success message"
}
```

## Audit Actions

- `user_create` - User created
- `user_update` - User updated
- `domain_upload` - Domain(s) uploaded
- `settings_change` - Settings changed
- `report_export` - Report exported

## Page Types

- `content` - Content page (video/movie page)
- `catalog` - Catalog/list page
- `static` - Static page (about, contacts, etc.)
- `error` - Error page (404, etc.)

## Site Status

- `active` - Site is active and ready for scanning
- `scanning` - Site is currently being scanned
- `error` - Error occurred during scanning
