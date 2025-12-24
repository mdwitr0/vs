package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const contentCollection = "content"

type Content struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title           string             `bson:"title" json:"title"`
	OriginalTitle   string             `bson:"original_title,omitempty" json:"original_title,omitempty"`
	Year            int                `bson:"year,omitempty" json:"year,omitempty"`
	KinopoiskID     string             `bson:"kinopoisk_id,omitempty" json:"kinopoisk_id,omitempty"`
	IMDBID          string             `bson:"imdb_id,omitempty" json:"imdb_id,omitempty"`
	MALID           string             `bson:"mal_id,omitempty" json:"mal_id,omitempty"`
	ShikimoriID     string             `bson:"shikimori_id,omitempty" json:"shikimori_id,omitempty"`
	MyDramaListID   string             `bson:"mydramalist_id,omitempty" json:"mydramalist_id,omitempty"`
	ViolationsCount int64              `bson:"violations_count" json:"violations_count"`
	SitesCount      int64              `bson:"sites_count" json:"sites_count"`
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
}

type ContentRepo struct {
	coll *mongo.Collection
}

func NewContentRepo(db *mongo.Database) *ContentRepo {
	coll := db.Collection(contentCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "kinopoisk_id", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
		{Keys: bson.D{{Key: "imdb_id", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
		{Keys: bson.D{{Key: "mal_id", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
		{Keys: bson.D{{Key: "shikimori_id", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
		{Keys: bson.D{{Key: "mydramalist_id", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
		{Keys: bson.D{{Key: "title", Value: "text"}}},
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "violations_count", Value: -1}, {Key: "created_at", Value: -1}}},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &ContentRepo{coll: coll}
}

func (r *ContentRepo) Create(ctx context.Context, content *Content) error {
	content.CreatedAt = time.Now()
	result, err := r.coll.InsertOne(ctx, content)
	if err != nil {
		return err
	}
	content.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ContentRepo) FindByID(ctx context.Context, id string) (*Content, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var content Content
	err = r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&content)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &content, err
}

type ContentFilter struct {
	Title         string
	KinopoiskID   string
	IMDBID        string
	MALID         string
	ShikimoriID   string
	MyDramaListID string
	HasViolations *bool
	SortBy        string
	SortOrder     string
	Limit         int64
	Offset        int64
}

func (r *ContentRepo) FindAll(ctx context.Context, f ContentFilter) ([]Content, int64, error) {
	filter := bson.M{}
	if f.Title != "" {
		filter["$text"] = bson.M{"$search": f.Title}
	}
	if f.KinopoiskID != "" {
		filter["kinopoisk_id"] = f.KinopoiskID
	}
	if f.IMDBID != "" {
		filter["imdb_id"] = f.IMDBID
	}
	if f.MALID != "" {
		filter["mal_id"] = f.MALID
	}
	if f.ShikimoriID != "" {
		filter["shikimori_id"] = f.ShikimoriID
	}
	if f.MyDramaListID != "" {
		filter["mydramalist_id"] = f.MyDramaListID
	}
	if f.HasViolations != nil {
		if *f.HasViolations {
			filter["violations_count"] = bson.M{"$gt": 0}
		} else {
			filter["violations_count"] = bson.M{"$eq": 0}
		}
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	sortOrder := -1
	if f.SortOrder == "asc" {
		sortOrder = 1
	}

	sortKey := "violations_count"
	if f.SortBy == "created_at" {
		sortKey = "created_at"
	}

	sortDoc := bson.D{{Key: sortKey, Value: sortOrder}}
	if sortKey == "violations_count" {
		sortDoc = append(sortDoc, bson.E{Key: "created_at", Value: -1})
	}

	opts := options.Find().
		SetLimit(f.Limit).
		SetSkip(f.Offset).
		SetSort(sortDoc)

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var contents []Content
	if err := cursor.All(ctx, &contents); err != nil {
		return nil, 0, err
	}

	return contents, total, nil
}

func (r *ContentRepo) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	_, err = r.coll.DeleteOne(ctx, bson.M{"_id": oid})
	return err
}

func (r *ContentRepo) GetAll(ctx context.Context) ([]Content, error) {
	cursor, err := r.coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var contents []Content
	if err := cursor.All(ctx, &contents); err != nil {
		return nil, err
	}
	return contents, nil
}

func (r *ContentRepo) UpdateViolationsCount(ctx context.Context, id string, violationsCount, sitesCount int64) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	_, err = r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$set": bson.M{
			"violations_count": violationsCount,
			"sites_count":      sitesCount,
		},
	})
	return err
}

func (r *ContentRepo) FindByIDs(ctx context.Context, ids []primitive.ObjectID, f ContentFilter) ([]Content, int64, error) {
	filter := bson.M{"_id": bson.M{"$in": ids}}

	if f.Title != "" {
		filter["$text"] = bson.M{"$search": f.Title}
	}
	if f.KinopoiskID != "" {
		filter["kinopoisk_id"] = f.KinopoiskID
	}
	if f.IMDBID != "" {
		filter["imdb_id"] = f.IMDBID
	}
	if f.MALID != "" {
		filter["mal_id"] = f.MALID
	}
	if f.ShikimoriID != "" {
		filter["shikimori_id"] = f.ShikimoriID
	}
	if f.MyDramaListID != "" {
		filter["mydramalist_id"] = f.MyDramaListID
	}
	if f.HasViolations != nil {
		if *f.HasViolations {
			filter["violations_count"] = bson.M{"$gt": 0}
		} else {
			filter["violations_count"] = bson.M{"$eq": 0}
		}
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	sortOrder := -1
	if f.SortOrder == "asc" {
		sortOrder = 1
	}

	sortKey := "violations_count"
	if f.SortBy == "created_at" {
		sortKey = "created_at"
	}

	sortDoc := bson.D{{Key: sortKey, Value: sortOrder}}
	if sortKey == "violations_count" {
		sortDoc = append(sortDoc, bson.E{Key: "created_at", Value: -1})
	}

	opts := options.Find().
		SetLimit(f.Limit).
		SetSkip(f.Offset).
		SetSort(sortDoc)

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var contents []Content
	if err := cursor.All(ctx, &contents); err != nil {
		return nil, 0, err
	}

	return contents, total, nil
}

func (r *ContentRepo) FindByExternalID(ctx context.Context, c *Content) (*Content, error) {
	var conditions []bson.M

	if c.KinopoiskID != "" {
		conditions = append(conditions, bson.M{"kinopoisk_id": c.KinopoiskID})
	}
	if c.IMDBID != "" {
		conditions = append(conditions, bson.M{"imdb_id": c.IMDBID})
	}
	if c.MALID != "" {
		conditions = append(conditions, bson.M{"mal_id": c.MALID})
	}
	if c.ShikimoriID != "" {
		conditions = append(conditions, bson.M{"shikimori_id": c.ShikimoriID})
	}
	if c.MyDramaListID != "" {
		conditions = append(conditions, bson.M{"mydramalist_id": c.MyDramaListID})
	}

	if len(conditions) == 0 {
		return nil, nil
	}

	var existing Content
	err := r.coll.FindOne(ctx, bson.M{"$or": conditions}).Decode(&existing)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &existing, nil
}

func (r *ContentRepo) EnrichExternalIDs(ctx context.Context, id primitive.ObjectID, c *Content) error {
	update := bson.M{}

	if c.KinopoiskID != "" {
		update["kinopoisk_id"] = c.KinopoiskID
	}
	if c.IMDBID != "" {
		update["imdb_id"] = c.IMDBID
	}
	if c.MALID != "" {
		update["mal_id"] = c.MALID
	}
	if c.ShikimoriID != "" {
		update["shikimori_id"] = c.ShikimoriID
	}
	if c.MyDramaListID != "" {
		update["mydramalist_id"] = c.MyDramaListID
	}

	if len(update) == 0 {
		return nil
	}

	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}
