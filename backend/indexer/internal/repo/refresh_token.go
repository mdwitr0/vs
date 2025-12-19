package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const refreshTokensCollection = "refresh_tokens"

type RefreshToken struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"user_id"`
	TokenHash string             `bson:"token_hash"`
	ExpiresAt time.Time          `bson:"expires_at"`
	CreatedAt time.Time          `bson:"created_at"`
}

type RefreshTokenRepo struct {
	coll *mongo.Collection
}

func NewRefreshTokenRepo(db *mongo.Database) *RefreshTokenRepo {
	coll := db.Collection(refreshTokensCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "expires_at", Value: 1}}},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &RefreshTokenRepo{coll: coll}
}

func (r *RefreshTokenRepo) Create(ctx context.Context, token *RefreshToken) error {
	token.CreatedAt = time.Now()

	result, err := r.coll.InsertOne(ctx, token)
	if err != nil {
		return err
	}
	token.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *RefreshTokenRepo) Upsert(ctx context.Context, token *RefreshToken) error {
	token.CreatedAt = time.Now()

	opts := options.Update().SetUpsert(true)
	_, err := r.coll.UpdateOne(
		ctx,
		bson.M{"user_id": token.UserID},
		bson.M{"$set": bson.M{
			"token_hash": token.TokenHash,
			"expires_at": token.ExpiresAt,
			"created_at": token.CreatedAt,
		}},
		opts,
	)
	return err
}

func (r *RefreshTokenRepo) FindByUserID(ctx context.Context, userID string) (*RefreshToken, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	var token RefreshToken
	err = r.coll.FindOne(ctx, bson.M{"user_id": oid}).Decode(&token)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &token, err
}

func (r *RefreshTokenRepo) DeleteByUserID(ctx context.Context, userID string) error {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	_, err = r.coll.DeleteOne(ctx, bson.M{"user_id": oid})
	return err
}

func (r *RefreshTokenRepo) DeleteExpired(ctx context.Context) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{
		"expires_at": bson.M{"$lt": time.Now()},
	})
	return err
}
