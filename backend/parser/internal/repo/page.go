package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/video-analitics/backend/pkg/models"
)

const pagesCollection = "pages"

type PageRepo struct {
	client *mongo.Client
	coll   *mongo.Collection
}

func NewPageRepo(mongoURL, dbName string) (*PageRepo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	coll := client.Database(dbName).Collection(pagesCollection)

	// Create indexes
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "site_id", Value: 1}, {Key: "url", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "url", Value: 1}}},
		{Keys: bson.D{{Key: "external_ids.kpid", Value: 1}}},
		{Keys: bson.D{{Key: "external_ids.imdb_id", Value: 1}}},
		{Keys: bson.D{{Key: "indexed_at", Value: -1}}},
	}

	_, err = coll.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		// Log but don't fail - indexes might already exist
	}

	return &PageRepo{client: client, coll: coll}, nil
}

func (r *PageRepo) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r.client.Disconnect(ctx)
}

func (r *PageRepo) Upsert(ctx context.Context, page *models.Page) error {
	filter := bson.M{"site_id": page.SiteID, "url": page.URL}
	update := bson.M{"$set": page}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var result models.Page
	err := r.coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		return err
	}
	page.ID = result.ID
	return nil
}

func (r *PageRepo) FindByURL(ctx context.Context, url string) (*models.Page, error) {
	var page models.Page
	err := r.coll.FindOne(ctx, bson.M{"url": url}).Decode(&page)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &page, err
}

func (r *PageRepo) FindBySiteID(ctx context.Context, siteID string, limit int64) ([]models.Page, error) {
	opts := options.Find().SetLimit(limit).SetSort(bson.D{{Key: "indexed_at", Value: -1}})
	cursor, err := r.coll.Find(ctx, bson.M{"site_id": siteID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var pages []models.Page
	if err := cursor.All(ctx, &pages); err != nil {
		return nil, err
	}
	return pages, nil
}

func (r *PageRepo) FindByKPID(ctx context.Context, kpid string) ([]models.Page, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"external_ids.kpid": kpid})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var pages []models.Page
	if err := cursor.All(ctx, &pages); err != nil {
		return nil, err
	}
	return pages, nil
}

func (r *PageRepo) ExistingURLs(ctx context.Context, siteID string, urls []string) (map[string]bool, error) {
	if len(urls) == 0 {
		return make(map[string]bool), nil
	}

	filter := bson.M{
		"site_id": siteID,
		"url":     bson.M{"$in": urls},
	}

	cursor, err := r.coll.Find(ctx, filter, options.Find().SetProjection(bson.M{"url": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	existing := make(map[string]bool)
	for cursor.Next(ctx) {
		var doc struct {
			URL string `bson:"url"`
		}
		if err := cursor.Decode(&doc); err == nil {
			existing[doc.URL] = true
		}
	}

	return existing, nil
}
