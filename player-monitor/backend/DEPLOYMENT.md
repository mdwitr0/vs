# Player Monitor Backend - Deployment Guide

## Quick Start (Development)

### Prerequisites
- Go 1.22+
- MongoDB 4.4+

### Steps

1. **Clone and navigate to backend directory**
```bash
cd /Users/mdwit/projects/vs/player-monitor/backend
```

2. **Install dependencies**
```bash
go mod download
```

3. **Set environment variables**
```bash
cp .env.example .env
# Edit .env with your values
```

Required environment variables:
```env
JWT_SECRET=your-secret-key-here
ADMIN_PASSWORD=secure-password
```

4. **Run the server**
```bash
go run ./cmd/server
```

Server will start on `http://localhost:8080`

## Docker Deployment

### Using Docker Compose (Recommended)

From the project root directory:

```bash
cd /Users/mdwit/projects/vs/player-monitor
docker-compose up -d backend
```

This will start:
- MongoDB on port 27019
- Backend on port 8090

### Using Docker only

```bash
# Build
docker build -t player-monitor-backend .

# Run
docker run -p 8080:8080 \
  -e MONGO_URL=mongodb://host.docker.internal:27017 \
  -e MONGO_DB=player_monitor \
  -e JWT_SECRET=your-secret-key \
  -e ADMIN_PASSWORD=admin123 \
  player-monitor-backend
```

## Production Deployment

### Environment Variables

```env
# Server
PORT=8080

# Database
MONGO_URL=mongodb://mongo:27017
MONGO_DB=player_monitor

# Security
JWT_SECRET=generate-strong-random-secret-here
JWT_ACCESS_EXPIRY=15m
JWT_REFRESH_EXPIRY=168h

# Admin Account
ADMIN_LOGIN=admin
ADMIN_PASSWORD=use-strong-password-here
```

### Security Recommendations

1. **Generate strong JWT secret**
```bash
openssl rand -base64 64
```

2. **Use environment-specific secrets**
   - Never commit secrets to git
   - Use secret management tools (AWS Secrets Manager, HashiCorp Vault, etc.)

3. **Enable MongoDB authentication**
```yaml
mongodb:
  environment:
    MONGO_INITDB_ROOT_USERNAME: admin
    MONGO_INITDB_ROOT_PASSWORD: strong-password
```

4. **Use HTTPS in production**
   - Configure reverse proxy (nginx, Traefik)
   - Use Let's Encrypt for SSL certificates

### Reverse Proxy Example (nginx)

```nginx
server {
    listen 80;
    server_name api.player-monitor.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Monitoring

### Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{"status":"ok"}
```

### Logs

The application uses structured logging (zerolog).

**Development mode:**
```bash
ENV=development go run ./cmd/server
```

**Production mode:**
```bash
ENV=production go run ./cmd/server
```

Production logs are in JSON format for easy parsing.

## Database Migrations

No migrations needed - collections and indexes are created automatically on startup.

Collections created:
- `users`
- `refresh_tokens`
- `sites`
- `pages`
- `settings`
- `audit_logs`

## Scheduled Tasks

The scheduler automatically runs based on `scan_interval_hours` setting (default: 24h).

To adjust:
```http
PUT /api/settings
Authorization: Bearer <admin_token>

{
  "player_pattern": "<video[^>]*>|<iframe[^>]*player[^>]*>",
  "scan_interval_hours": 12
}
```

## Backup and Restore

### Backup MongoDB

```bash
docker exec player-monitor-mongodb mongodump --db player_monitor --out /backup
docker cp player-monitor-mongodb:/backup ./backup
```

### Restore MongoDB

```bash
docker cp ./backup player-monitor-mongodb:/backup
docker exec player-monitor-mongodb mongorestore --db player_monitor /backup/player_monitor
```

## Troubleshooting

### Cannot connect to MongoDB

1. Check MongoDB is running:
```bash
docker ps | grep mongo
```

2. Check MongoDB connection string in `.env`

3. Test MongoDB connection:
```bash
mongosh "mongodb://localhost:27017/player_monitor"
```

### JWT errors

1. Ensure `JWT_SECRET` is set in environment
2. Check token expiry settings
3. Verify token is sent in `Authorization: Bearer <token>` header

### Scan not working

1. Check site status: `GET /api/sites/:id`
2. Check logs for errors
3. Verify `player_pattern` in settings is valid regex
4. Check network connectivity to target sites

### High memory usage

1. Reduce concurrent scans
2. Limit sitemap URLs (currently capped at 1000)
3. Adjust rate limiting in crawler

## Performance Tuning

### MongoDB Indexes

Indexes are created automatically. To verify:
```javascript
db.pages.getIndexes()
db.sites.getIndexes()
```

### Crawler Rate Limiting

Edit `internal/crawler/crawler.go`:
```go
if scannedCount%10 == 0 {
    time.Sleep(500 * time.Millisecond)  // Adjust delay
}
```

## Scaling

### Horizontal Scaling

The backend is stateless and can be scaled horizontally:

```yaml
backend:
  deploy:
    replicas: 3
```

Use load balancer (nginx, HAProxy, Traefik) to distribute traffic.

### Database Scaling

For high load, consider:
- MongoDB replica sets
- Read replicas for reporting
- Sharding for large datasets

## Support

For issues and questions:
1. Check logs: `docker logs player-monitor-backend`
2. Review API documentation: `API.md`
3. Check MongoDB status: `docker exec -it player-monitor-mongodb mongosh`
