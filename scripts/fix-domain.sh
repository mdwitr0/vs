#!/bin/bash
# Fix domain in Meilisearch for existing pages

MEILI_URL="http://localhost:7701"
MEILI_KEY="masterKey"
MONGO_URL="mongodb://localhost:27018/video_analitics"

# Get all sites
sites=$(docker exec video-analitics-mongodb mongosh --quiet --eval 'db.sites.find({}, {_id: 1, domain: 1}).forEach(s => print(JSON.stringify({id: s._id.toString(), domain: s.domain})))' video_analitics)

echo "Found sites:"
echo "$sites"

# For each site, update pages in Meilisearch
echo "$sites" | while read -r line; do
    site_id=$(echo "$line" | jq -r '.id')
    domain=$(echo "$line" | jq -r '.domain')
    
    if [ -z "$site_id" ] || [ "$site_id" == "null" ]; then
        continue
    fi
    
    echo "Processing site: $site_id -> $domain"
    
    # Get pages for this site from Meilisearch
    offset=0
    limit=1000
    
    while true; do
        response=$(curl -s "${MEILI_URL}/indexes/pages/documents?filter=site_id%3D${site_id}&limit=${limit}&offset=${offset}" -H "Authorization: Bearer ${MEILI_KEY}")
        
        count=$(echo "$response" | jq '.results | length')
        
        if [ "$count" -eq 0 ]; then
            break
        fi
        
        echo "  Found $count pages at offset $offset"
        
        # Update each page with domain
        updates=$(echo "$response" | jq --arg domain "$domain" '[.results[] | {id: .id, domain: $domain}]')
        
        curl -s -X POST "${MEILI_URL}/indexes/pages/documents" \
            -H "Authorization: Bearer ${MEILI_KEY}" \
            -H "Content-Type: application/json" \
            -d "$updates" > /dev/null
            
        echo "  Updated $count pages"
        
        offset=$((offset + limit))
        
        if [ "$count" -lt "$limit" ]; then
            break
        fi
    done
done

echo "Done!"
