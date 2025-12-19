#!/bin/bash

DOMAIN="${1:-narko-tv.com}"
MODE="${2:-docker}"  # docker, local, scan
API="http://localhost:8081/api"
SCRIPT_DIR="$(dirname "$0")"
BACKEND_DIR="$SCRIPT_DIR/../backend"

export REDIS_URL="localhost:6389"
export MONGO_URL="mongodb://localhost:27018"
export MONGO_DB="video_analitics"
export MEILI_URL="http://localhost:7701"
export MEILI_KEY="masterKey"

start_local_parser() {
    echo "Killing existing parser processes..."
    pkill -f "go run ./parser" 2>/dev/null
    docker compose stop parser 2>/dev/null
    sleep 2

    echo "Starting parser locally..."
    cd "$BACKEND_DIR"
    go run ./parser/cmd/main.go 2>&1 | tee /tmp/parser.log &
    PARSER_PID=$!
    cd - > /dev/null
    echo "Parser PID: $PARSER_PID"
    sleep 5
}

stop_local_parser() {
    if [ -n "$PARSER_PID" ]; then
        echo "Stopping local parser..."
        kill $PARSER_PID 2>/dev/null
    fi
}

echo "=== Testing site: $DOMAIN (mode: $MODE) ==="

if [ "$MODE" = "local" ] || [ "$MODE" = "scan" ]; then
    start_local_parser
fi

# Delete if exists
SITE_ID=$(curl -s "$API/sites" | jq -r ".items[] | select(.domain == \"$DOMAIN\") | .id")
if [ -n "$SITE_ID" ]; then
    echo "Deleting existing site $SITE_ID..."
    curl -s -X DELETE "$API/sites/$SITE_ID" | jq .
    sleep 1
fi

# Add site
echo "Adding site..."
RESULT=$(curl -s -X POST "$API/sites" -H "Content-Type: application/json" -d "{\"domain\":\"$DOMAIN\"}")
echo "$RESULT" | jq .
SITE_ID=$(echo "$RESULT" | jq -r '.id')

# Wait for detection
echo "Waiting 30s for detection..."
sleep 30

# Show site after detection
echo "=== Site after detection ==="
curl -s "$API/sites" | jq ".items[] | select(.domain == \"$DOMAIN\")"

# If scan mode, trigger crawl
if [ "$MODE" = "scan" ]; then
    echo ""
    echo "=== Starting scan ==="
    curl -s -X POST "$API/sites/scan" -H "Content-Type: application/json" -d "{\"site_ids\":[\"$SITE_ID\"]}" | jq .

    echo "Waiting 60s for crawl..."
    sleep 60

    echo "=== Scan tasks ==="
    curl -s "$API/scan-tasks" | jq ".items[] | select(.site_id == \"$SITE_ID\")"

    echo "=== Site pages ==="
    curl -s "$API/pages?site_id=$SITE_ID" | jq '.total'
fi

# Logs
echo ""
echo "=== Parser logs ==="
if [ "$MODE" = "docker" ]; then
    docker compose logs parser --since 120s 2>&1 | grep -E "(crawl|captcha|pirate|solve|cookie)" | tail -40
else
    grep -E "(crawl|captcha|pirate|solve|cookie|SPA|urls)" /tmp/parser.log 2>/dev/null | tail -50
fi

stop_local_parser
