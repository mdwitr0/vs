#!/bin/bash

# Parser performance metrics
# Usage: ./parser-stats.sh [seconds]

PERIOD=${1:-30}

echo "=== Parser Stats (last ${PERIOD}s) ==="
echo ""

for ctx in va-prod va-indexer-1 va-indexer-2; do
    container="va-parser"
    [[ "$ctx" == "va-prod" ]] && container="va-prod-parser"

    # Get pages count
    pages=$(docker --context $ctx logs $container --since ${PERIOD}s 2>&1 | grep -c "page fetched" || echo 0)

    # Get stats
    stats=$(docker --context $ctx stats --no-stream --format "{{.CPUPerc}}\t{{.MemUsage}}" $container 2>/dev/null | head -1)
    cpu=$(echo "$stats" | cut -f1)
    mem=$(echo "$stats" | cut -f2)

    # Calculate speed
    speed=$(echo "scale=2; $pages / $PERIOD" | bc)

    printf "%-15s | %4d pages | %6s p/s | CPU: %8s | MEM: %s\n" "$ctx" "$pages" "$speed" "$cpu" "$mem"
done

echo ""
echo "Chrome containers:"
for ctx in va-prod va-indexer-1 va-indexer-2; do
    container="va-chrome"
    [[ "$ctx" == "va-prod" ]] && container="va-prod-chrome"

    stats=$(docker --context $ctx stats --no-stream --format "{{.CPUPerc}}\t{{.MemUsage}}" $container 2>/dev/null | head -1)
    cpu=$(echo "$stats" | cut -f1)
    mem=$(echo "$stats" | cut -f2)

    printf "%-15s | CPU: %8s | MEM: %s\n" "$ctx" "$cpu" "$mem"
done
