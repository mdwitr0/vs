package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

const usersCollection = "users"

type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Login        string             `bson:"login" json:"login"`
	PasswordHash string             `bson:"password_hash" json:"-"`
	Role         string             `bson:"role" json:"role"`
	IsActive     bool               `bson:"is_active" json:"is_active"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at"`
}

type UserFilter struct {
	Role     string
	IsActive *bool
	Limit    int64
	Offset   int64
}

type UserRepo struct {
	coll *mongo.Collection
}

func NewUserRepo(db *mongo.Database) *UserRepo {
	coll := db.Collection(usersCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "login", Value: 1}}, Options: options.Index().SetUnique(true)},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &UserRepo{coll: coll}
}

func (r *UserRepo) Create(ctx context.Context, user *User) error {
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	if user.Role == "" {
		user.Role = "user"
	}
	user.IsActive = true

	result, err := r.coll.InsertOne(ctx, user)
	if err != nil {
		return err
	}
	user.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *UserRepo) FindByLogin(ctx context.Context, login string) (*User, error) {
	var user User
	err := r.coll.FindOne(ctx, bson.M{"login": login}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &user, err
}

func (r *UserRepo) FindByID(ctx context.Context, id string) (*User, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var user User
	err = r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &user, err
}

func (r *UserRepo) FindAll(ctx context.Context, filter UserFilter) ([]User, int64, error) {
	query := bson.M{}
	if filter.Role != "" {
		query["role"] = filter.Role
	}
	if filter.IsActive != nil {
		query["is_active"] = *filter.IsActive
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

	var users []User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *UserRepo) Update(ctx context.Context, user *User) error {
	user.UpdatedAt = time.Now()
	_, err := r.coll.UpdateOne(
		ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": bson.M{
			"login":         user.Login,
			"password_hash": user.PasswordHash,
			"role":          user.Role,
			"is_active":     user.IsActive,
			"updated_at":    user.UpdatedAt,
		}},
	)
	return err
}

func (r *UserRepo) UpdateStatus(ctx context.Context, id string, isActive bool) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{"$set": bson.M{
			"is_active":  isActive,
			"updated_at": time.Now(),
		}},
	)
	return err
}

func (r *UserRepo) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	_, err = r.coll.DeleteOne(ctx, bson.M{"_id": oid})
	return err
}

func (r *UserRepo) Count(ctx context.Context) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{})
}

func (r *UserRepo) SeedAdmin(ctx context.Context, login, password string) error {
	count, err := r.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return r.Create(ctx, &User{
		Login:        login,
		PasswordHash: string(hash),
		Role:         "admin",
		IsActive:     true,
	})
}
