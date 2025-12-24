package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const sitesCollection = "sites"

type Site struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID             primitive.ObjectID `bson:"user_id" json:"user_id"`
	Domain             string             `bson:"domain" json:"domain"`
	TotalPages         int64              `bson:"total_pages" json:"total_pages"`
	PagesWithPlayer    int64              `bson:"pages_with_player" json:"pages_with_player"`
	PagesWithoutPlayer int64              `bson:"pages_without_player" json:"pages_without_player"`
	LastScanAt         time.Time          `bson:"last_scan_at" json:"last_scan_at"`
	Status             string             `bson:"status" json:"status"`
	CreatedAt          time.Time          `bson:"created_at" json:"created_at"`
}

type SiteFilter struct {
	UserID    string
	Domain    string
	Status    string
	SortBy    string
	SortOrder string
	Limit     int64
	Offset    int64
}

type SiteRepo struct {
	coll *mongo.Collection
}

func NewSiteRepo(db *mongo.Database) *SiteRepo {
	coll := db.Collection(sitesCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "domain", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "user_id", Value: 1}}},
		{Keys: bson.D{{Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "last_scan_at", Value: -1}}},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &SiteRepo{coll: coll}
}

func (r *SiteRepo) Create(ctx context.Context, site *Site) error {
	site.CreatedAt = time.Now()
	if site.Status == "" {
		site.Status = "active"
	}

	result, err := r.coll.InsertOne(ctx, site)
	if err != nil {
		return err
	}
	site.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *SiteRepo) FindByID(ctx context.Context, id string, userID string) (*Site, error) {
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

	var site Site
	err = r.coll.FindOne(ctx, query).Decode(&site)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &site, err
}

func (r *SiteRepo) FindByDomain(ctx context.Context, userID primitive.ObjectID, domain string) (*Site, error) {
	var site Site
	err := r.coll.FindOne(ctx, bson.M{"user_id": userID, "domain": domain}).Decode(&site)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &site, err
}

func (r *SiteRepo) FindAll(ctx context.Context, filter SiteFilter) ([]Site, int64, error) {
	query := bson.M{}
	if filter.UserID != "" {
		userOID, err := primitive.ObjectIDFromHex(filter.UserID)
		if err != nil {
			return nil, 0, err
		}
		query["user_id"] = userOID
	}
	if filter.Domain != "" {
		query["domain"] = bson.M{"$regex": filter.Domain, "$options": "i"}
	}
	if filter.Status != "" {
		query["status"] = filter.Status
	}

	total, err := r.coll.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	sortOrder := -1
	if filter.SortOrder == "asc" {
		sortOrder = 1
	}

	sortKey := "created_at"
	if filter.SortBy == "last_scan_at" {
		sortKey = "last_scan_at"
	} else if filter.SortBy == "domain" {
		sortKey = "domain"
	}

	opts := options.Find().
		SetLimit(filter.Limit).
		SetSkip(filter.Offset).
		SetSort(bson.D{{Key: sortKey, Value: sortOrder}})

	cursor, err := r.coll.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var sites []Site
	if err := cursor.All(ctx, &sites); err != nil {
		return nil, 0, err
	}

	return sites, total, nil
}

func (r *SiteRepo) Update(ctx context.Context, site *Site) error {
	_, err := r.coll.UpdateOne(
		ctx,
		bson.M{"_id": site.ID},
		bson.M{"$set": bson.M{
			"domain":               site.Domain,
			"total_pages":          site.TotalPages,
			"pages_with_player":    site.PagesWithPlayer,
			"pages_without_player": site.PagesWithoutPlayer,
			"last_scan_at":         site.LastScanAt,
			"status":               site.Status,
		}},
	)
	return err
}

func (r *SiteRepo) UpdateStats(ctx context.Context, siteID primitive.ObjectID, totalPages, withPlayer, withoutPlayer int64) error {
	_, err := r.coll.UpdateOne(
		ctx,
		bson.M{"_id": siteID},
		bson.M{"$set": bson.M{
			"total_pages":          totalPages,
			"pages_with_player":    withPlayer,
			"pages_without_player": withoutPlayer,
		}},
	)
	return err
}

func (r *SiteRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	update := bson.M{"status": status}
	if status == "scanning" || status == "active" {
		update["last_scan_at"] = time.Now()
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{"$set": update},
	)
	return err
}

func (r *SiteRepo) Delete(ctx context.Context, id string, userID string) error {
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

	_, err = r.coll.DeleteOne(ctx, query)
	return err
}

func (r *SiteRepo) GetActiveSites(ctx context.Context) ([]Site, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"status": "active"})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var sites []Site
	if err := cursor.All(ctx, &sites); err != nil {
		return nil, err
	}
	return sites, nil
}
