# Quick Start Guide

## 🚀 Get Running in 5 Minutes

### Prerequisites
- Go 1.22+ installed
- MongoDB running (or use Docker)

### Option 1: Local Development

```bash
# 1. Navigate to backend directory
cd /Users/mdwit/projects/vs/player-monitor/backend

# 2. Copy environment file
cp .env.example .env

# 3. Edit .env and set required values
# Minimum required:
# JWT_SECRET=your-secret-key
# ADMIN_PASSWORD=your-password

# 4. Start MongoDB (if not running)
docker run -d -p 27017:27017 --name mongo mongo:7.0

# 5. Download dependencies
go mod download

# 6. Run the server
go run ./cmd/server
```

Server starts at `http://localhost:8080`

### Option 2: Docker (Easiest)

```bash
# From project root
cd /Users/mdwit/projects/vs/player-monitor

# Start everything
docker-compose up -d

# View logs
docker-compose logs -f backend
```

Backend runs at `http://localhost:8090`

## 🧪 Test It Works

### 1. Health Check
```bash
curl http://localhost:8080/health
# Should return: {"status":"ok"}
```

### 2. Login as Admin
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "login": "admin",
    "password": "admin123"
  }'
```

You'll get back:
```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "abc123...",
  "expires_in": 900
}
```

Save the `access_token` for next steps.

### 3. Add Your First Site
```bash
# Replace YOUR_TOKEN with the access_token from step 2
curl -X POST http://localhost:8080/api/sites \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "domain": "example.com"
  }'
```

### 4. Start Scanning
```bash
# Replace SITE_ID with the id from step 3
curl -X POST http://localhost:8080/api/sites/SITE_ID/scan \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 5. Check Results
```bash
curl http://localhost:8080/api/sites/SITE_ID \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## 📚 Next Steps

1. **Read API Docs:** Check `API.md` for all available endpoints
2. **Configure Settings:** Adjust player pattern and scan interval via `/api/settings`
3. **Import Sites:** Upload CSV file with multiple domains
4. **Export Reports:** Download CSV of pages without player

## 🔧 Common Tasks

### Change Admin Password
```bash
curl -X PUT http://localhost:8080/api/users/USER_ID \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -d '{
    "password": "new-secure-password"
  }'
```

### Create New User
```bash
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -d '{
    "login": "newuser",
    "password": "password123",
    "role": "user"
  }'
```

### Update Player Pattern
```bash
curl -X PUT http://localhost:8080/api/settings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -d '{
    "player_pattern": "<video[^>]*>|<iframe[^>]*player[^>]*>",
    "scan_interval_hours": 12
  }'
```

### Import Sites from CSV
```bash
# Create sites.csv with one domain per line:
# example.com
# test.com
# demo.org

curl -X POST http://localhost:8080/api/sites/import \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "file=@sites.csv"
```

### Export Pages Without Player
```bash
curl http://localhost:8080/api/sites/SITE_ID/export \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -o pages_without_player.csv
```

## 🐛 Troubleshooting

### Can't Connect to MongoDB
```bash
# Check MongoDB is running
docker ps | grep mongo

# Or start it
docker run -d -p 27017:27017 --name mongo mongo:7.0
```

### JWT Errors
- Make sure `JWT_SECRET` is set in `.env`
- Check token is sent as `Authorization: Bearer <token>`
- Token expires after 15 minutes - use refresh token

### Scan Not Starting
- Check site status: `GET /api/sites/:id`
- Verify domain is accessible
- Check logs for errors

## 📖 Documentation

- `README.md` - Project overview
- `API.md` - Complete API documentation
- `DEPLOYMENT.md` - Production deployment guide
- `IMPLEMENTATION.md` - Technical implementation details

## 💡 Tips

1. **Development Mode:** Set `ENV=development` for pretty logs
2. **Production Mode:** Set `ENV=production` for JSON logs
3. **Auto-reload:** Use `air` for development with auto-reload
4. **Database GUI:** Use MongoDB Compass to view data

## 🎯 What You Get

- ✅ JWT authentication with refresh tokens
- ✅ Role-based access (admin/user)
- ✅ Site crawling and monitoring
- ✅ Configurable player detection
- ✅ Automatic scheduled scans
- ✅ CSV import/export
- ✅ Audit logging
- ✅ RESTful API

## 🚨 Security Reminder

**Before deploying to production:**

1. Generate strong JWT secret:
   ```bash
   openssl rand -base64 64
   ```

2. Use strong admin password

3. Enable MongoDB authentication

4. Use HTTPS (reverse proxy with SSL)

5. Set appropriate CORS policies

---

**Ready to use! Start building your player monitoring system! 🎉**
