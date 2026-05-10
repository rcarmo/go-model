package oauthcreds

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrNotFound = errors.New("oauth credentials not found")

type Record struct {
	ProviderID string         `json:"provider_id" bson:"provider_id"`
	Refresh    string         `json:"refresh" bson:"refresh"`
	Access     string         `json:"access" bson:"access"`
	Expires    int64          `json:"expires" bson:"expires"`
	Extra      map[string]any `json:"extra,omitempty" bson:"extra,omitempty"`
	CreatedAt  time.Time      `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at" bson:"updated_at"`
}

type Store interface {
	Get(ctx context.Context, providerID string) (*Record, error)
	Upsert(ctx context.Context, record Record) error
	Close() error
}

func normalizeProviderID(providerID string) string {
	return strings.TrimSpace(providerID)
}
