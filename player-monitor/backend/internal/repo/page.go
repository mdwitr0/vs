package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const pagesCollection = "pages"

type Page struct {
	ID                primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID            primitive.ObjectID `bson:"user_id" json:"user_id"`
	SiteID            primitive.ObjectID `bson:"site_id" json:"site_id"`
	URL               string             `bson:"url" json:"url"`
	HasPlayer         bool               `bson:"has_player" json:"has_player"`
	PageType          string             `bson:"page_type" json:"page_type"`
	ExcludeFromReport bool               `bson:"exclude_from_report" json:"exclude_from_report"`
	LastCheckedAt     time.Time          `bson:"last_checked_at" json:"last_checked_at"`
}

type PageFilter struct {
	UserID            string
	SiteID            string
	HasPlayer         *bool
	PageType          string
	ExcludeFromReport *bool
	Limit             int64
	Offset            int64
}

type PageRepo struct {
	coll *mongo.Collection
}

func NewPageRepo(db *mongo.Database) *PageRepo {
	coll := db.Collection(pagesCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "site_id", Value: 1}, {Key: "url", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "user_id", Value: 1}}},
		{Keys: bson.D{{Key: "site_id", Value: 1}, {Key: "has_player", Value: 1}}},
		{Keys: bson.D{{Key: "site_id", Value: 1}, {Key: "page_type", Value: 1}}},
		{Keys: bson.D{{Key: "last_checked_at", Value: -1}}},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &PageRepo{coll: coll}
}

func (r *PageRepo) Create(ctx context.Context, page *Page) error {
	page.LastCheckedAt = time.Now()

	result, err := r.coll.InsertOne(ctx, page)
	if err != nil {
		return err
	}
	page.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *PageRepo) Upsert(ctx context.Context, page *Page) error {
	page.LastCheckedAt = time.Now()

	opts := options.Update().SetUpsert(true)
	_, err := r.coll.UpdateOne(
		ctx,
		bson.M{"user_id": page.UserID, "site_id": page.SiteID, "url": page.URL},
		bson.M{"$set": bson.M{
			"user_id":             page.UserID,
			"has_player":          page.HasPlayer,
			"page_type":           page.PageType,
			"exclude_from_report": page.ExcludeFromReport,
			"last_checked_at":     page.LastCheckedAt,
		}},
		opts,
	)
	return err
}

func (r *PageRepo) FindBySiteID(ctx context.Context, filter PageFilter) ([]Page, int64, error) {
	siteOID, err := primitive.ObjectIDFromHex(filter.SiteID)
	if err != nil {
		return nil, 0, err
	}

	query := bson.M{"site_id": siteOID}
	if filter.UserID != "" {
		userOID, err := primitive.ObjectIDFromHex(filter.UserID)
		if err != nil {
			return nil, 0, err
		}
		query["user_id"] = userOID
	}
	if filter.HasPlayer != nil {
		query["has_player"] = *filter.HasPlayer
	}
	if filter.PageType != "" {
		query["page_type"] = filter.PageType
	}
	if filter.ExcludeFromReport != nil {
		query["exclude_from_report"] = *filter.ExcludeFromReport
	}

	total, err := r.coll.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetLimit(filter.Limit).
		SetSkip(filter.Offset).
		SetSort(bson.D{{Key: "last_checked_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var pages []Page
	if err := cursor.All(ctx, &pages); err != nil {
		return nil, 0, err
	}

	return pages, total, nil
}

func (r *PageRepo) FindByID(ctx context.Context, id string, userID string) (*Page, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": oid}
	if userID != "" {
		userOID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			return nil, err
		}
		query["user_id"] = userOID
	}

	var page Page
	err = r.coll.FindOne(ctx, query).Decode(&page)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &page, err
}

func (r *PageRepo) UpdateExcludeFlag(ctx context.Context, id string, userID string, exclude bool) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	if userID != "" {
		userOID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			return err
		}
		query["user_id"] = userOID
	}

	_, err = r.coll.UpdateOne(
		ctx,
		query,
		bson.M{"$set": bson.M{"exclude_from_report": exclude}},
	)
	return err
}

func (r *PageRepo) DeleteBySiteID(ctx context.Context, siteID string) error {
	siteOID, err := primitive.ObjectIDFromHex(siteID)
	if err != nil {
		return err
	}

	_, err = r.coll.DeleteMany(ctx, bson.M{"site_id": siteOID})
	return err
}

func (r *PageRepo) CountBySiteID(ctx context.Context, siteID primitive.ObjectID) (total, withPlayer, withoutPlayer int64, err error) {
	total, err = r.coll.CountDocuments(ctx, bson.M{
		"site_id":             siteID,
		"exclude_from_report": false,
	})
	if err != nil {
		return 0, 0, 0, err
	}

	withPlayer, err = r.coll.CountDocuments(ctx, bson.M{
		"site_id":             siteID,
		"has_player":          true,
		"exclude_from_report": false,
	})
	if err != nil {
		return 0, 0, 0, err
	}

	withoutPlayer, err = r.coll.CountDocuments(ctx, bson.M{
		"site_id":             siteID,
		"has_player":          false,
		"exclude_from_report": false,
	})
	if err != nil {
		return 0, 0, 0, err
	}

	return total, withPlayer, withoutPlayer, nil
}

func (r *PageRepo) GetPagesWithoutPlayer(ctx context.Context, siteID string, excludeFromReport bool) ([]Page, error) {
	siteOID, err := primitive.ObjectIDFromHex(siteID)
	if err != nil {
		return nil, err
	}

	query := bson.M{
		"site_id":    siteOID,
		"has_player": false,
	}
	if !excludeFromReport {
		query["exclude_from_report"] = false
	}

	cursor, err := r.coll.Find(ctx, query, options.Find().SetSort(bson.D{{Key: "url", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var pages []Page
	if err := cursor.All(ctx, &pages); err != nil {
		return nil, err
	}
	return pages, nil
}
