package oauthcreds

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type mongoRecord struct {
	ProviderID string         `bson:"_id"`
	Refresh    string         `bson:"refresh"`
	Access     string         `bson:"access"`
	Expires    int64          `bson:"expires"`
	Extra      map[string]any `bson:"extra,omitempty"`
	CreatedAt  time.Time      `bson:"created_at"`
	UpdatedAt  time.Time      `bson:"updated_at"`
}

type MongoDBStore struct{ collection *mongo.Collection }

func NewMongoDBStore(database *mongo.Database) (*MongoDBStore, error) {
	if database == nil {
		return nil, fmt.Errorf("database is required")
	}
	coll := database.Collection("oauth_credentials")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "updated_at", Value: -1}}})
	if err != nil {
		return nil, fmt.Errorf("create oauth_credentials indexes: %w", err)
	}
	return &MongoDBStore{collection: coll}, nil
}

func (s *MongoDBStore) List(ctx context.Context) ([]Record, error) {
	cursor, err := s.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list oauth credentials: %w", err)
	}
	defer cursor.Close(ctx)
	result := make([]Record, 0)
	for cursor.Next(ctx) {
		var doc mongoRecord
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode oauth credentials: %w", err)
		}
		result = append(result, Record{ProviderID: doc.ProviderID, Refresh: doc.Refresh, Access: doc.Access, Expires: doc.Expires, Extra: doc.Extra, CreatedAt: doc.CreatedAt.UTC(), UpdatedAt: doc.UpdatedAt.UTC()})
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterate oauth credentials: %w", err)
	}
	return result, nil
}

func (s *MongoDBStore) Get(ctx context.Context, providerID string) (*Record, error) {
	providerID = normalizeProviderID(providerID)
	var doc mongoRecord
	if err := s.collection.FindOne(ctx, bson.M{"_id": providerID}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get oauth credentials: %w", err)
	}
	return &Record{ProviderID: doc.ProviderID, Refresh: doc.Refresh, Access: doc.Access, Expires: doc.Expires, Extra: doc.Extra, CreatedAt: doc.CreatedAt.UTC(), UpdatedAt: doc.UpdatedAt.UTC()}, nil
}

func (s *MongoDBStore) Upsert(ctx context.Context, record Record) error {
	record.ProviderID = normalizeProviderID(record.ProviderID)
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	_, err := s.collection.UpdateOne(ctx, bson.M{"_id": record.ProviderID}, bson.M{"$set": bson.M{
		"refresh":    record.Refresh,
		"access":     record.Access,
		"expires":    record.Expires,
		"extra":      record.Extra,
		"updated_at": record.UpdatedAt,
	}, "$setOnInsert": bson.M{"created_at": record.CreatedAt}}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("upsert oauth credentials: %w", err)
	}
	return nil
}

func (s *MongoDBStore) Close() error { return nil }
