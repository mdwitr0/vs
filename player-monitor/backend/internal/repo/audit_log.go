package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const auditLogsCollection = "audit_logs"

type AuditLog struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	Action    string             `bson:"action" json:"action"`
	Details   map[string]any     `bson:"details" json:"details"`
	IPAddress string             `bson:"ip_address" json:"ip_address"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

type AuditLogFilter struct {
	UserID string
	Action string
	Limit  int64
	Offset int64
}

type AuditLogRepo struct {
	coll *mongo.Collection
}

func NewAuditLogRepo(db *mongo.Database) *AuditLogRepo {
	coll := db.Collection(auditLogsCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}}},
		{Keys: bson.D{{Key: "action", Value: 1}}},
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &AuditLogRepo{coll: coll}
}

func (r *AuditLogRepo) Create(ctx context.Context, log *AuditLog) error {
	log.CreatedAt = time.Now()

	result, err := r.coll.InsertOne(ctx, log)
	if err != nil {
		return err
	}
	log.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *AuditLogRepo) FindAll(ctx context.Context, filter AuditLogFilter) ([]AuditLog, int64, error) {
	query := bson.M{}
	if filter.UserID != "" {
		userOID, err := primitive.ObjectIDFromHex(filter.UserID)
		if err != nil {
			return nil, 0, err
		}
		query["user_id"] = userOID
	}
	if filter.Action != "" {
		query["action"] = filter.Action
	}

	total, err := r.coll.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetLimit(filter.Limit).
		SetSkip(filter.Offset).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var logs []AuditLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
