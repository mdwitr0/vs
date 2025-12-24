package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const settingsCollection = "settings"

type Settings struct {
	ID                primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID            primitive.ObjectID `bson:"user_id" json:"user_id"`
	PlayerPattern     string             `bson:"player_pattern" json:"player_pattern"`
	ScanIntervalHours int                `bson:"scan_interval_hours" json:"scan_interval_hours"`
	UpdatedAt         time.Time          `bson:"updated_at" json:"updated_at"`
}

type SettingsRepo struct {
	coll *mongo.Collection
}

func NewSettingsRepo(db *mongo.Database) *SettingsRepo {
	coll := db.Collection(settingsCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}}, Options: options.Index().SetUnique(true)},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &SettingsRepo{coll: coll}
}

func (r *SettingsRepo) GetByUserID(ctx context.Context, userID primitive.ObjectID) (*Settings, error) {
	var settings Settings
	err := r.coll.FindOne(ctx, bson.M{"user_id": userID}).Decode(&settings)
	if err == mongo.ErrNoDocuments {
		defaultSettings := &Settings{
			UserID:            userID,
			PlayerPattern:     `<iframe[^>]*player[^>]*>|<video[^>]*>|<div[^>]*player[^>]*>`,
			ScanIntervalHours: 24,
			UpdatedAt:         time.Now(),
		}
		result, insertErr := r.coll.InsertOne(ctx, defaultSettings)
		if insertErr != nil {
			return nil, insertErr
		}
		defaultSettings.ID = result.InsertedID.(primitive.ObjectID)
		return defaultSettings, nil
	}
	return &settings, err
}

func (r *SettingsRepo) Update(ctx context.Context, settings *Settings) error {
	settings.UpdatedAt = time.Now()

	opts := options.Update().SetUpsert(true)
	_, err := r.coll.UpdateOne(
		ctx,
		bson.M{"user_id": settings.UserID},
		bson.M{"$set": bson.M{
			"player_pattern":      settings.PlayerPattern,
			"scan_interval_hours": settings.ScanIntervalHours,
			"updated_at":          settings.UpdatedAt,
		}},
		opts,
	)
	return err
}
