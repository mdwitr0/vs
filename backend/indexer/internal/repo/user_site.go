package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const userSitesCollection = "user_sites"

type UserSite struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	SiteID    primitive.ObjectID `bson:"site_id" json:"site_id"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

type UserSiteRepo struct {
	coll *mongo.Collection
}

func NewUserSiteRepo(db *mongo.Database) *UserSiteRepo {
	coll := db.Collection(userSitesCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "site_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "user_id", Value: 1}}},
		{Keys: bson.D{{Key: "site_id", Value: 1}}},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &UserSiteRepo{coll: coll}
}

func (r *UserSiteRepo) Create(ctx context.Context, userSite *UserSite) error {
	userSite.CreatedAt = time.Now()
	result, err := r.coll.InsertOne(ctx, userSite)
	if err != nil {
		return err
	}
	userSite.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *UserSiteRepo) FindByUserID(ctx context.Context, userID string) ([]UserSite, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	cursor, err := r.coll.Find(ctx, bson.M{"user_id": oid})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var userSites []UserSite
	if err := cursor.All(ctx, &userSites); err != nil {
		return nil, err
	}
	return userSites, nil
}

func (r *UserSiteRepo) FindByUserAndSite(ctx context.Context, userID, siteID string) (*UserSite, error) {
	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}
	siteOID, err := primitive.ObjectIDFromHex(siteID)
	if err != nil {
		return nil, err
	}

	var userSite UserSite
	err = r.coll.FindOne(ctx, bson.M{
		"user_id": userOID,
		"site_id": siteOID,
	}).Decode(&userSite)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &userSite, err
}

func (r *UserSiteRepo) GetSiteIDsForUser(ctx context.Context, userID string) ([]primitive.ObjectID, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	cursor, err := r.coll.Find(ctx, bson.M{"user_id": oid}, options.Find().SetProjection(bson.M{"site_id": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		SiteID primitive.ObjectID `bson:"site_id"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	siteIDs := make([]primitive.ObjectID, len(results))
	for i, r := range results {
		siteIDs[i] = r.SiteID
	}
	return siteIDs, nil
}

func (r *UserSiteRepo) DeleteByUserAndSite(ctx context.Context, userID, siteID string) error {
	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}
	siteOID, err := primitive.ObjectIDFromHex(siteID)
	if err != nil {
		return err
	}

	_, err = r.coll.DeleteOne(ctx, bson.M{
		"user_id": userOID,
		"site_id": siteOID,
	})
	return err
}

func (r *UserSiteRepo) DeleteBySiteID(ctx context.Context, siteID string) (int64, error) {
	siteOID, err := primitive.ObjectIDFromHex(siteID)
	if err != nil {
		return 0, err
	}

	result, err := r.coll.DeleteMany(ctx, bson.M{"site_id": siteOID})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

func (r *UserSiteRepo) ExistsByUserAndSite(ctx context.Context, userID, siteID string) (bool, error) {
	link, err := r.FindByUserAndSite(ctx, userID, siteID)
	if err != nil {
		return false, err
	}
	return link != nil, nil
}

// GetSiteIDsByUserID returns site IDs as strings for a user
func (r *UserSiteRepo) GetSiteIDsByUserID(ctx context.Context, userID string) ([]string, error) {
	oids, err := r.GetSiteIDsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]string, len(oids))
	for i, oid := range oids {
		result[i] = oid.Hex()
	}
	return result, nil
}
