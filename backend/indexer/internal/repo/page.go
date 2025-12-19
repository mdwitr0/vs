package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/video-analitics/backend/pkg/models"
)

const pagesCollection = "pages"

type PageRepo struct {
	coll *mongo.Collection
}

func NewPageRepo(db *mongo.Database) *PageRepo {
	coll := db.Collection(pagesCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		// Уникальность страниц (site_id + url)
		{Keys: bson.D{{Key: "site_id", Value: 1}, {Key: "url", Value: 1}}, Options: options.Index().SetUnique(true)},
		// Для FindBySiteID + CountBySiteID + пагинация
		{Keys: bson.D{{Key: "site_id", Value: 1}, {Key: "indexed_at", Value: -1}}},
		// Для Search по KPID + пагинация
		{Keys: bson.D{{Key: "external_ids.kpid", Value: 1}, {Key: "indexed_at", Value: -1}}, Options: options.Index().SetSparse(true)},
		// Для Search по IMDb + пагинация
		{Keys: bson.D{{Key: "external_ids.imdb_id", Value: 1}, {Key: "indexed_at", Value: -1}}, Options: options.Index().SetSparse(true)},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &PageRepo{coll: coll}
}

func (r *PageRepo) FindBySiteID(ctx context.Context, siteID string, limit, offset int64) ([]models.Page, int64, error) {
	filter := bson.M{"site_id": siteID}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetLimit(limit).
		SetSkip(offset).
		SetSort(bson.D{{Key: "indexed_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var pages []models.Page
	if err := cursor.All(ctx, &pages); err != nil {
		return nil, 0, err
	}

	return pages, total, nil
}

func (r *PageRepo) FindByExternalID(ctx context.Context, idType, idValue string, limit, offset int64) ([]models.Page, int64, error) {
	fieldName := "external_ids." + idType
	filter := bson.M{fieldName: idValue}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetLimit(limit).
		SetSkip(offset).
		SetSort(bson.D{{Key: "indexed_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var pages []models.Page
	if err := cursor.All(ctx, &pages); err != nil {
		return nil, 0, err
	}

	return pages, total, nil
}

func (r *PageRepo) Search(ctx context.Context, query PageQuery) ([]models.Page, int64, error) {
	filter := bson.M{}

	if query.SiteID != "" {
		filter["site_id"] = query.SiteID
	}
	if query.KinopoiskID != "" {
		filter["external_ids.kinopoisk_id"] = query.KinopoiskID
	}
	if query.IMDBID != "" {
		filter["external_ids.imdb_id"] = query.IMDBID
	}
	if query.Title != "" {
		filter["title"] = bson.M{"$regex": query.Title, "$options": "i"}
	}
	if query.Year > 0 {
		filter["year"] = query.Year
	}
	if query.HasPlayer != nil {
		if *query.HasPlayer {
			filter["player_url"] = bson.M{"$ne": ""}
		} else {
			filter["$or"] = []bson.M{
				{"player_url": ""},
				{"player_url": bson.M{"$exists": false}},
			}
		}
	}
	if len(query.PageIDs) > 0 {
		oids := make([]primitive.ObjectID, 0, len(query.PageIDs))
		for _, id := range query.PageIDs {
			if oid, err := primitive.ObjectIDFromHex(id); err == nil {
				oids = append(oids, oid)
			}
		}
		if len(oids) > 0 {
			filter["_id"] = bson.M{"$in": oids}
		}
	}
	if len(query.ExcludePageIDs) > 0 {
		oids := make([]primitive.ObjectID, 0, len(query.ExcludePageIDs))
		for _, id := range query.ExcludePageIDs {
			if oid, err := primitive.ObjectIDFromHex(id); err == nil {
				oids = append(oids, oid)
			}
		}
		if len(oids) > 0 {
			filter["_id"] = bson.M{"$nin": oids}
		}
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Сортировка
	sortField := "indexed_at"
	sortOrder := -1 // desc по умолчанию

	if query.SortBy == "year" {
		sortField = "year"
	}
	if query.SortOrder == "asc" {
		sortOrder = 1
	}

	opts := options.Find().
		SetLimit(query.Limit).
		SetSkip(query.Offset).
		SetSort(bson.D{{Key: sortField, Value: sortOrder}})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var pages []models.Page
	if err := cursor.All(ctx, &pages); err != nil {
		return nil, 0, err
	}

	return pages, total, nil
}

func (r *PageRepo) CountBySiteID(ctx context.Context, siteID string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"site_id": siteID})
}

func (r *PageRepo) GetStats(ctx context.Context, siteID string) (*PageStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	baseFilter := bson.M{}
	if siteID != "" {
		baseFilter["site_id"] = siteID
	}

	total, err := r.coll.CountDocuments(ctx, baseFilter)
	if err != nil {
		return nil, err
	}

	kpFilter := bson.M{"external_ids.kpid": bson.M{"$ne": ""}}
	imdbFilter := bson.M{"external_ids.imdb_id": bson.M{"$ne": ""}}
	playerFilter := bson.M{"player_url": bson.M{"$ne": ""}}

	if siteID != "" {
		kpFilter["site_id"] = siteID
		imdbFilter["site_id"] = siteID
		playerFilter["site_id"] = siteID
	}

	withKP, _ := r.coll.CountDocuments(ctx, kpFilter)
	withIMDb, _ := r.coll.CountDocuments(ctx, imdbFilter)
	withPlayer, _ := r.coll.CountDocuments(ctx, playerFilter)

	return &PageStats{
		Total:      total,
		WithKPID:   withKP,
		WithIMDbID: withIMDb,
		WithPlayer: withPlayer,
	}, nil
}

type PageQuery struct {
	SiteID         string
	KinopoiskID    string
	IMDBID         string
	Title          string
	Year           int      // фильтр по году
	HasPlayer      *bool    // только с плеером
	PageIDs        []string // фильтр по ID (для has_violations=true)
	ExcludePageIDs []string // исключить ID (для has_violations=false)
	SortBy         string   // "indexed_at", "year"
	SortOrder      string   // "asc", "desc"
	Limit          int64
	Offset         int64
}

type PageStats struct {
	Total      int64 `json:"total"`
	WithKPID   int64 `json:"with_kpid"`
	WithIMDbID int64 `json:"with_imdb_id"`
	WithPlayer int64 `json:"with_player"`
}

func (r *PageRepo) DeleteBySiteID(ctx context.Context, siteID string) (int64, error) {
	result, err := r.coll.DeleteMany(ctx, bson.M{"site_id": siteID})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
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
