package violations

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const collectionName = "violations"

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	coll := db.Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "content_id", Value: 1}}},
		{Keys: bson.D{{Key: "site_id", Value: 1}}},
		{Keys: bson.D{{Key: "page_id", Value: 1}}},
		{
			Keys:    bson.D{{Key: "content_id", Value: 1}, {Key: "page_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &Repository{coll: coll}
}

func (r *Repository) Upsert(ctx context.Context, v *Violation) error {
	filter := bson.M{
		"content_id": v.ContentID,
		"page_id":    v.PageID,
	}

	update := bson.M{
		"$set": bson.M{
			"site_id":    v.SiteID,
			"page_url":   v.PageURL,
			"page_title": v.PageTitle,
			"match_type": v.MatchType,
			"found_at":   v.FoundAt,
		},
		"$setOnInsert": bson.M{
			"content_id": v.ContentID,
			"page_id":    v.PageID,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.coll.UpdateOne(ctx, filter, update, opts)
	return err
}

func (r *Repository) UpsertMany(ctx context.Context, violations []Violation) error {
	if len(violations) == 0 {
		return nil
	}

	models := make([]mongo.WriteModel, len(violations))
	for i, v := range violations {
		filter := bson.M{
			"content_id": v.ContentID,
			"page_id":    v.PageID,
		}
		update := bson.M{
			"$set": bson.M{
				"site_id":    v.SiteID,
				"page_url":   v.PageURL,
				"page_title": v.PageTitle,
				"match_type": v.MatchType,
				"found_at":   v.FoundAt,
			},
			"$setOnInsert": bson.M{
				"content_id": v.ContentID,
				"page_id":    v.PageID,
			},
		}
		models[i] = mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true)
	}

	opts := options.BulkWrite().SetOrdered(false)
	_, err := r.coll.BulkWrite(ctx, models, opts)
	return err
}

func (r *Repository) DeleteByContentID(ctx context.Context, contentID string) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"content_id": contentID})
	return err
}

func (r *Repository) DeleteByPageID(ctx context.Context, pageID string) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"page_id": pageID})
	return err
}

func (r *Repository) FindByContentID(ctx context.Context, contentID string, limit, offset int64) ([]Violation, int64, error) {
	filter := bson.M{"content_id": contentID}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetLimit(limit).
		SetSkip(offset).
		SetSort(bson.D{{Key: "found_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var violations []Violation
	if err := cursor.All(ctx, &violations); err != nil {
		return nil, 0, err
	}

	return violations, total, nil
}

func (r *Repository) FindBySiteID(ctx context.Context, siteID string, limit, offset int64) ([]Violation, int64, error) {
	filter := bson.M{"site_id": siteID}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetLimit(limit).
		SetSkip(offset).
		SetSort(bson.D{{Key: "found_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var violations []Violation
	if err := cursor.All(ctx, &violations); err != nil {
		return nil, 0, err
	}

	return violations, total, nil
}

func (r *Repository) GetContentStats(ctx context.Context, contentID string) (*ContentStats, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"content_id": contentID}}},
		{{Key: "$group", Value: bson.M{
			"_id":      "$content_id",
			"count":    bson.M{"$sum": 1},
			"page_ids": bson.M{"$addToSet": "$page_id"},
			"site_ids": bson.M{"$addToSet": "$site_id"},
		}}},
	}

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID      string   `bson:"_id"`
		Count   int64    `bson:"count"`
		PageIDs []string `bson:"page_ids"`
		SiteIDs []string `bson:"site_ids"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return &ContentStats{
			ContentID:       contentID,
			ViolationsCount: 0,
			SitesCount:      0,
		}, nil
	}

	r0 := results[0]
	return &ContentStats{
		ContentID:       contentID,
		ViolationsCount: r0.Count,
		SitesCount:      int64(len(r0.SiteIDs)),
		PageIDs:         r0.PageIDs,
		SiteIDs:         r0.SiteIDs,
	}, nil
}

func (r *Repository) GetSiteStats(ctx context.Context, siteID string) (*SiteStats, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"site_id": siteID}}},
		{{Key: "$group", Value: bson.M{
			"_id":         "$site_id",
			"count":       bson.M{"$sum": 1},
			"page_ids":    bson.M{"$addToSet": "$page_id"},
			"content_ids": bson.M{"$addToSet": "$content_id"},
		}}},
	}

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID         string   `bson:"_id"`
		Count      int64    `bson:"count"`
		PageIDs    []string `bson:"page_ids"`
		ContentIDs []string `bson:"content_ids"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return &SiteStats{
			SiteID:          siteID,
			ViolationsCount: 0,
			ContentsCount:   0,
		}, nil
	}

	r0 := results[0]
	return &SiteStats{
		SiteID:          siteID,
		ViolationsCount: r0.Count,
		ContentsCount:   int64(len(r0.ContentIDs)),
		PageIDs:         r0.PageIDs,
		ContentIDs:      r0.ContentIDs,
	}, nil
}

func (r *Repository) GetAllContentStats(ctx context.Context) (map[string]*ContentStats, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":      "$content_id",
			"count":    bson.M{"$sum": 1},
			"page_ids": bson.M{"$addToSet": "$page_id"},
			"site_ids": bson.M{"$addToSet": "$site_id"},
		}}},
	}

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID      string   `bson:"_id"`
		Count   int64    `bson:"count"`
		PageIDs []string `bson:"page_ids"`
		SiteIDs []string `bson:"site_ids"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	statsMap := make(map[string]*ContentStats, len(results))
	for _, r := range results {
		statsMap[r.ID] = &ContentStats{
			ContentID:       r.ID,
			ViolationsCount: r.Count,
			SitesCount:      int64(len(r.SiteIDs)),
			PageIDs:         r.PageIDs,
			SiteIDs:         r.SiteIDs,
		}
	}

	return statsMap, nil
}

func (r *Repository) GetAllSiteStats(ctx context.Context) (map[string]*SiteStats, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":         "$site_id",
			"count":       bson.M{"$sum": 1},
			"page_ids":    bson.M{"$addToSet": "$page_id"},
			"content_ids": bson.M{"$addToSet": "$content_id"},
		}}},
	}

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID         string   `bson:"_id"`
		Count      int64    `bson:"count"`
		PageIDs    []string `bson:"page_ids"`
		ContentIDs []string `bson:"content_ids"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	statsMap := make(map[string]*SiteStats, len(results))
	for _, r := range results {
		statsMap[r.ID] = &SiteStats{
			SiteID:          r.ID,
			ViolationsCount: r.Count,
			ContentsCount:   int64(len(r.ContentIDs)),
			PageIDs:         r.PageIDs,
			ContentIDs:      r.ContentIDs,
		}
	}

	return statsMap, nil
}

func (r *Repository) CountBySiteID(ctx context.Context, siteID string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"site_id": siteID})
}

func (r *Repository) CountByContentID(ctx context.Context, contentID string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"content_id": contentID})
}

func (r *Repository) GetDistinctSiteIDs(ctx context.Context, contentID string) ([]string, error) {
	result, err := r.coll.Distinct(ctx, "site_id", bson.M{"content_id": contentID})
	if err != nil {
		return nil, err
	}

	siteIDs := make([]string, len(result))
	for i, v := range result {
		if s, ok := v.(string); ok {
			siteIDs[i] = s
		}
	}
	return siteIDs, nil
}

func (r *Repository) GetDistinctContentIDs(ctx context.Context, siteID string) ([]string, error) {
	result, err := r.coll.Distinct(ctx, "content_id", bson.M{"site_id": siteID})
	if err != nil {
		return nil, err
	}

	contentIDs := make([]string, len(result))
	for i, v := range result {
		if s, ok := v.(string); ok {
			contentIDs[i] = s
		}
	}
	return contentIDs, nil
}

func (r *Repository) GetPageIDsBySiteID(ctx context.Context, siteID string) ([]string, error) {
	result, err := r.coll.Distinct(ctx, "page_id", bson.M{"site_id": siteID})
	if err != nil {
		return nil, err
	}

	pageIDs := make([]string, len(result))
	for i, v := range result {
		if s, ok := v.(string); ok {
			pageIDs[i] = s
		}
	}
	return pageIDs, nil
}

func (r *Repository) FindAllByContentID(ctx context.Context, contentID string) ([]Violation, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"content_id": contentID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var violations []Violation
	if err := cursor.All(ctx, &violations); err != nil {
		return nil, err
	}

	return violations, nil
}

func (r *Repository) DeleteNotInPageIDs(ctx context.Context, contentID string, validPageIDs []string) error {
	if len(validPageIDs) == 0 {
		return r.DeleteByContentID(ctx, contentID)
	}

	filter := bson.M{
		"content_id": contentID,
		"page_id":    bson.M{"$nin": validPageIDs},
	}
	_, err := r.coll.DeleteMany(ctx, filter)
	return err
}

// DeleteByContentAndSiteNotInPageIDs удаляет violations для content+site, которых нет в validPageIDs
func (r *Repository) DeleteByContentAndSiteNotInPageIDs(ctx context.Context, contentID, siteID string, validPageIDs []string) error {
	filter := bson.M{
		"content_id": contentID,
		"site_id":    siteID,
	}

	if len(validPageIDs) == 0 {
		_, err := r.coll.DeleteMany(ctx, filter)
		return err
	}

	filter["page_id"] = bson.M{"$nin": validPageIDs}
	_, err := r.coll.DeleteMany(ctx, filter)
	return err
}

// DeleteBySiteID удаляет все violations для сайта
func (r *Repository) DeleteBySiteID(ctx context.Context, siteID string) (int64, error) {
	result, err := r.coll.DeleteMany(ctx, bson.M{"site_id": siteID})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}
