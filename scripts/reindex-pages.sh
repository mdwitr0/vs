#!/bin/bash
# Re-index pages from MongoDB to Meilisearch

MEILI_URL="http://localhost:7701"
MEILI_KEY="masterKey"
TMP_FILE="/tmp/pages_to_index.json"

# Get all pages from MongoDB with site domains via aggregation
docker exec video-analitics-mongodb mongosh --quiet --eval '
const pages = db.pages.aggregate([
    {
        $lookup: {
            from: "sites",
            let: { siteId: "$site_id" },
            pipeline: [
                { $match: { $expr: { $eq: [{ $toString: "$_id" }, "$$siteId"] } } }
            ],
            as: "site"
        }
    },
    { $unwind: { path: "$site", preserveNullAndEmptyArrays: true } }
]).toArray();

const docs = pages.map(p => ({
    id: p._id.toString(),
    site_id: p.site_id,
    domain: p.site ? p.site.domain : "",
    url: p.url,
    title: p.title || "",
    description: p.description || "",
    main_text: p.main_text || "",
    year: p.year || 0,
    kpid: (p.external_ids && p.external_ids.kpid) || "",
    imdb_id: (p.external_ids && p.external_ids.imdb_id) || "",
    player_urls: p.player_url ? [p.player_url] : [],
    indexed_at: p.indexed_at ? p.indexed_at.toISOString() : new Date().toISOString()
}));

print(JSON.stringify(docs));
' video_analitics > "$TMP_FILE"

count=$(jq 'length' "$TMP_FILE")
echo "Indexing $count pages to Meilisearch..."

# Index to Meilisearch using file
response=$(curl -s -X POST "${MEILI_URL}/indexes/pages/documents" \
    -H "Authorization: Bearer ${MEILI_KEY}" \
    -H "Content-Type: application/json" \
    -d @"$TMP_FILE")

echo "Response: $response"
rm -f "$TMP_FILE"
echo "Done!"
