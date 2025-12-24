package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const userContentCollection = "user_content"

type UserContent struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	ContentID primitive.ObjectID `bson:"content_id" json:"content_id"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

type UserContentRepo struct {
	coll *mongo.Collection
}

func NewUserContentRepo(db *mongo.Database) *UserContentRepo {
	coll := db.Collection(userContentCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "content_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "user_id", Value: 1}}},
		{Keys: bson.D{{Key: "content_id", Value: 1}}},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &UserContentRepo{coll: coll}
}

func (r *UserContentRepo) Link(ctx context.Context, userID, contentID primitive.ObjectID) error {
	uc := UserContent{
		UserID:    userID,
		ContentID: contentID,
		CreatedAt: time.Now(),
	}

	_, err := r.coll.InsertOne(ctx, uc)
	if mongo.IsDuplicateKeyError(err) {
		return nil
	}
	return err
}

func (r *UserContentRepo) Unlink(ctx context.Context, userID, contentID primitive.ObjectID) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{
		"user_id":    userID,
		"content_id": contentID,
	})
	return err
}

func (r *UserContentRepo) HasAccess(ctx context.Context, userID, contentID primitive.ObjectID) (bool, error) {
	count, err := r.coll.CountDocuments(ctx, bson.M{
		"user_id":    userID,
		"content_id": contentID,
	})
	return count > 0, err
}

func (r *UserContentRepo) GetContentIDs(ctx context.Context, userID primitive.ObjectID) ([]primitive.ObjectID, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var links []UserContent
	if err := cursor.All(ctx, &links); err != nil {
		return nil, err
	}

	ids := make([]primitive.ObjectID, len(links))
	for i, link := range links {
		ids[i] = link.ContentID
	}
	return ids, nil
}

func (r *UserContentRepo) GetUserIDs(ctx context.Context, contentID primitive.ObjectID) ([]primitive.ObjectID, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"content_id": contentID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var links []UserContent
	if err := cursor.All(ctx, &links); err != nil {
		return nil, err
	}

	ids := make([]primitive.ObjectID, len(links))
	for i, link := range links {
		ids[i] = link.UserID
	}
	return ids, nil
}

func (r *UserContentRepo) DeleteByContentID(ctx context.Context, contentID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"content_id": contentID})
	return err
}

func (r *UserContentRepo) CountByContentID(ctx context.Context, contentID primitive.ObjectID) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"content_id": contentID})
}
