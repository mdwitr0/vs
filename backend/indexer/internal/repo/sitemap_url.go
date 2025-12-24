package repo

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/video-analitics/backend/pkg/status"
)

const sitemapURLsCollection = "sitemap_urls"

type SitemapURL struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SiteID        string             `bson:"site_id" json:"site_id"`
	URL           string             `bson:"url" json:"url"`
	SitemapSource string             `bson:"sitemap_source" json:"sitemap_source"`
	LastMod       *time.Time         `bson:"lastmod,omitempty" json:"lastmod,omitempty"`
	Priority      float64            `bson:"priority,omitempty" json:"priority,omitempty"`
	ChangeFreq    string             `bson:"changefreq,omitempty" json:"changefreq,omitempty"`
	Status        status.URL         `bson:"status" json:"status"`
	DiscoveredAt  time.Time          `bson:"discovered_at" json:"discovered_at"`
	IndexedAt     *time.Time         `bson:"indexed_at,omitempty" json:"indexed_at,omitempty"`
	Error         string             `bson:"error,omitempty" json:"error,omitempty"`
	IsXML         bool               `bson:"is_xml" json:"is_xml"`
	RetryCount    int                `bson:"retry_count" json:"retry_count"`
	LastAttemptAt *time.Time         `bson:"last_attempt_at,omitempty" json:"last_attempt_at,omitempty"`
	LockedUntil   *time.Time         `bson:"locked_until,omitempty" json:"locked_until,omitempty"`

	// Глубина: 0 = из sitemap/главная, 1-3 = найдены при парсинге страниц
	Depth int `bson:"depth" json:"depth"`
}

type SitemapURLInput struct {
	URL        string
	LastMod    *time.Time
	Priority   float64
	ChangeFreq string
	Depth      int // 0 = из sitemap, 1-3 = найдены при парсинге
}

type SitemapURLStats struct {
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Indexed    int64 `json:"indexed"`
	Error      int64 `json:"error"`
	Skipped    int64 `json:"skipped"`
	Total      int64 `json:"total"`
}

type SitemapURLRepo struct {
	coll *mongo.Collection
}

func NewSitemapURLRepo(db *mongo.Database) *SitemapURLRepo {
	coll := db.Collection(sitemapURLsCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "site_id", Value: 1}, {Key: "url", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "site_id", Value: 1}, {Key: "status", Value: 1}, {Key: "discovered_at", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "sitemap_source", Value: 1}},
		},
		{
			Keys: bson.D{
				{Key: "site_id", Value: 1},
				{Key: "status", Value: 1},
				{Key: "retry_count", Value: 1},
				{Key: "last_attempt_at", Value: 1},
			},
		},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &SitemapURLRepo{coll: coll}
}

func (r *SitemapURLRepo) UpsertBatch(ctx context.Context, siteID string, sitemapSource string, urls []SitemapURLInput) (int, int, error) {
	if len(urls) == 0 {
		return 0, 0, nil
	}

	now := time.Now()
	var inserted, updated int

	models := make([]mongo.WriteModel, 0, len(urls))

	for _, u := range urls {
		isXML := isXMLURL(u.URL)
		urlStatus := status.URLPending
		if isXML {
			urlStatus = status.URLSkipped
		}

		filter := bson.M{"site_id": siteID, "url": u.URL}
		update := bson.M{
			"$setOnInsert": bson.M{
				"site_id":       siteID,
				"url":           u.URL,
				"discovered_at": now,
				"status":        urlStatus,
				"is_xml":        isXML,
				"depth":         u.Depth,
			},
			"$set": bson.M{
				"sitemap_source": sitemapSource,
				"lastmod":        u.LastMod,
				"priority":       u.Priority,
				"changefreq":     u.ChangeFreq,
			},
		}

		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(update).
			SetUpsert(true))
	}

	opts := options.BulkWrite().SetOrdered(false)
	result, err := r.coll.BulkWrite(ctx, models, opts)
	if err != nil {
		return 0, 0, err
	}

	inserted = int(result.UpsertedCount)
	updated = int(result.ModifiedCount)

	return inserted, updated, nil
}

const maxRetryCount = 5
const retryDelay = 5 * time.Minute

func (r *SitemapURLRepo) FindPending(ctx context.Context, siteID string, limit int) ([]SitemapURL, error) {
	retryThreshold := time.Now().Add(-retryDelay)

	filter := bson.M{
		"site_id": siteID,
		"status":  status.URLPending,
		"$or": []bson.M{
			{"retry_count": bson.M{"$exists": false}},
			{"retry_count": bson.M{"$lt": maxRetryCount}},
		},
		"$and": []bson.M{
			{"$or": []bson.M{
				{"last_attempt_at": nil},
				{"last_attempt_at": bson.M{"$exists": false}},
				{"last_attempt_at": bson.M{"$lt": retryThreshold}},
			}},
		},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "discovered_at", Value: 1}}).
		SetLimit(int64(limit))

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var urls []SitemapURL
	if err := cursor.All(ctx, &urls); err != nil {
		return nil, err
	}

	return urls, nil
}

const lockDuration = 5 * time.Minute

func (r *SitemapURLRepo) FindPendingAndLock(ctx context.Context, siteID string, limit int) ([]SitemapURL, error) {
	now := time.Now()
	retryThreshold := now.Add(-retryDelay)
	lockUntil := now.Add(lockDuration)

	filter := bson.M{
		"site_id": siteID,
		"status":  status.URLPending,
		"$or": []bson.M{
			{"retry_count": bson.M{"$exists": false}},
			{"retry_count": bson.M{"$lt": maxRetryCount}},
		},
		"$and": []bson.M{
			{"$or": []bson.M{
				{"last_attempt_at": nil},
				{"last_attempt_at": bson.M{"$exists": false}},
				{"last_attempt_at": bson.M{"$lt": retryThreshold}},
			}},
			{"$or": []bson.M{
				{"locked_until": nil},
				{"locked_until": bson.M{"$exists": false}},
				{"locked_until": bson.M{"$lt": now}},
			}},
		},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "discovered_at", Value: 1}}).
		SetLimit(int64(limit))

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var urls []SitemapURL
	if err := cursor.All(ctx, &urls); err != nil {
		return nil, err
	}

	if len(urls) == 0 {
		return urls, nil
	}

	var urlIDs []primitive.ObjectID
	for _, u := range urls {
		urlIDs = append(urlIDs, u.ID)
	}

	_, err = r.coll.UpdateMany(ctx, bson.M{"_id": bson.M{"$in": urlIDs}}, bson.M{
		"$set": bson.M{
			"locked_until": lockUntil,
			"status":       status.URLProcessing,
		},
	})
	if err != nil {
		return nil, err
	}

	return urls, nil
}

func (r *SitemapURLRepo) MarkIndexed(ctx context.Context, siteID, url string) error {
	now := time.Now()
	filter := bson.M{"site_id": siteID, "url": url}
	update := bson.M{
		"$set": bson.M{
			"status":     status.URLIndexed,
			"indexed_at": now,
			"error":      "",
		},
		"$unset": bson.M{"locked_until": ""},
	}

	_, err := r.coll.UpdateOne(ctx, filter, update)
	return err
}

func (r *SitemapURLRepo) MarkError(ctx context.Context, siteID, url, errMsg string) error {
	now := time.Now()
	filter := bson.M{"site_id": siteID, "url": url}

	// Return to pending for retry, increment retry_count
	_, err := r.coll.UpdateOne(ctx, filter, bson.M{
		"$inc":   bson.M{"retry_count": 1},
		"$set":   bson.M{"error": errMsg, "last_attempt_at": now, "status": status.URLPending},
		"$unset": bson.M{"locked_until": ""},
	})
	if err != nil {
		return err
	}

	// If max retries exceeded, mark as error (terminal state)
	_, err = r.coll.UpdateOne(ctx, bson.M{
		"site_id":     siteID,
		"url":         url,
		"retry_count": bson.M{"$gte": maxRetryCount},
	}, bson.M{
		"$set": bson.M{"status": status.URLError},
	})

	return err
}

func (r *SitemapURLRepo) GetStats(ctx context.Context, siteID string) (*SitemapURLStats, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"site_id": siteID}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$status",
			"count": bson.M{"$sum": 1},
		}}},
	}

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	stats := &SitemapURLStats{}
	for cursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cursor.Decode(&result); err != nil {
			continue
		}

		switch status.URL(result.ID) {
		case status.URLPending:
			stats.Pending = result.Count
		case status.URLProcessing:
			stats.Processing = result.Count
		case status.URLIndexed:
			stats.Indexed = result.Count
		case status.URLError:
			stats.Error = result.Count
		case status.URLSkipped:
			stats.Skipped = result.Count
		}
		stats.Total += result.Count
	}

	return stats, nil
}

func (r *SitemapURLRepo) FindByFilter(ctx context.Context, siteID string, urlStatus string, limit, offset int) ([]SitemapURL, int64, error) {
	filter := bson.M{"site_id": siteID}
	if urlStatus != "" {
		filter["status"] = urlStatus
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "discovered_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var urls []SitemapURL
	if err := cursor.All(ctx, &urls); err != nil {
		return nil, 0, err
	}

	return urls, total, nil
}

func (r *SitemapURLRepo) ExistsURL(ctx context.Context, siteID, url string) (bool, error) {
	filter := bson.M{"site_id": siteID, "url": url}
	count, err := r.coll.CountDocuments(ctx, filter, options.Count().SetLimit(1))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SitemapURLRepo) DeleteBySiteID(ctx context.Context, siteID string) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"site_id": siteID})
	return err
}

func (r *SitemapURLRepo) SkipPendingBySiteID(ctx context.Context, siteID string, reason string) (int64, error) {
	result, err := r.coll.UpdateMany(ctx,
		bson.M{
			"site_id": siteID,
			"status":  status.URLPending,
		},
		bson.M{
			"$set": bson.M{
				"status": status.URLSkipped,
				"error":  reason,
			},
		},
	)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (r *SitemapURLRepo) GetPendingCounts(ctx context.Context, siteIDs []string) (map[string]int64, error) {
	if len(siteIDs) == 0 {
		return make(map[string]int64), nil
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"site_id": bson.M{"$in": siteIDs},
			"status":  status.URLPending,
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$site_id",
			"count": bson.M{"$sum": 1},
		}}},
	}

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	result := make(map[string]int64)
	for cursor.Next(ctx) {
		var item struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cursor.Decode(&item); err != nil {
			continue
		}
		result[item.ID] = item.Count
	}

	return result, nil
}

func (r *SitemapURLRepo) ResetErrorsToPending(ctx context.Context, siteID string) (int64, error) {
	result, err := r.coll.UpdateMany(ctx,
		bson.M{
			"site_id": siteID,
			"status":  status.URLError,
		},
		bson.M{
			"$set": bson.M{
				"status":      status.URLPending,
				"error":       "",
				"retry_count": 0,
			},
			"$unset": bson.M{
				"last_attempt_at": "",
				"locked_until":    "",
			},
		},
	)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (r *SitemapURLRepo) ResetPendingRetryDelay(ctx context.Context, siteID string) (int64, error) {
	result, err := r.coll.UpdateMany(ctx,
		bson.M{
			"site_id": siteID,
			"status":  status.URLPending,
		},
		bson.M{
			"$set": bson.M{
				"retry_count": 0,
			},
			"$unset": bson.M{
				"last_attempt_at": "",
				"locked_until":    "",
			},
		},
	)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (r *SitemapURLRepo) ResetAllToPending(ctx context.Context, siteID string) (int64, error) {
	result, err := r.coll.UpdateMany(ctx,
		bson.M{
			"site_id": siteID,
			"status":  bson.M{"$in": []status.URL{status.URLIndexed, status.URLError}},
		},
		bson.M{
			"$set": bson.M{
				"status":      status.URLPending,
				"error":       "",
				"retry_count": 0,
			},
			"$unset": bson.M{
				"indexed_at":      "",
				"last_attempt_at": "",
				"locked_until":    "",
			},
		},
	)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (r *SitemapURLRepo) GetAllURLStrings(ctx context.Context, siteID string) ([]string, error) {
	filter := bson.M{"site_id": siteID}

	opts := options.Find().SetProjection(bson.M{"url": 1, "_id": 0})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		URL string `bson:"url"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	urls := make([]string, len(results))
	for i, r := range results {
		urls[i] = r.URL
	}

	return urls, nil
}

func isXMLURL(url string) bool {
	lowerURL := strings.ToLower(url)

	extensions := []string{".xml", ".rss", ".atom", ".feed"}
	for _, ext := range extensions {
		if strings.HasSuffix(lowerURL, ext) {
			return true
		}
	}

	patterns := []string{"/rss", "/feed/", "/atom/", "/sitemap"}
	for _, p := range patterns {
		if strings.Contains(lowerURL, p) {
			return true
		}
	}

	return false
}

// RecoverStaleURLs returns URLs stuck in processing state (lock expired) back to pending
func (r *SitemapURLRepo) RecoverStaleURLs(ctx context.Context) (int64, error) {
	now := time.Now()

	// Find URLs that are in processing state but lock has expired
	result, err := r.coll.UpdateMany(ctx,
		bson.M{
			"status": status.URLProcessing,
			"$or": []bson.M{
				{"locked_until": bson.M{"$lt": now}},
				{"locked_until": bson.M{"$exists": false}},
				{"locked_until": nil},
			},
		},
		bson.M{
			"$set":   bson.M{"status": status.URLPending},
			"$unset": bson.M{"locked_until": ""},
		},
	)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}
