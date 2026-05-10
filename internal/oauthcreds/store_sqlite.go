package oauthcreds

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type SQLiteStore struct{ db *sql.DB }

func NewSQLiteStore(db *sql.DB) (*SQLiteStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS oauth_credentials (
			provider_id TEXT PRIMARY KEY,
			refresh_token TEXT NOT NULL DEFAULT '',
			access_token TEXT NOT NULL DEFAULT '',
			expires_at_ms INTEGER NOT NULL DEFAULT 0,
			extra_json TEXT NOT NULL DEFAULT '{}',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create oauth_credentials table: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Get(ctx context.Context, providerID string) (*Record, error) {
	providerID = normalizeProviderID(providerID)
	var rec Record
	var extraJSON string
	var createdAt, updatedAt int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT provider_id, refresh_token, access_token, expires_at_ms, extra_json, created_at, updated_at
		FROM oauth_credentials WHERE provider_id = ?
	`, providerID).Scan(&rec.ProviderID, &rec.Refresh, &rec.Access, &rec.Expires, &extraJSON, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get oauth credentials: %w", err)
	}
	if err := json.Unmarshal([]byte(extraJSON), &rec.Extra); err != nil {
		rec.Extra = map[string]any{}
	}
	rec.CreatedAt = time.Unix(createdAt, 0).UTC()
	rec.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &rec, nil
}

func (s *SQLiteStore) Upsert(ctx context.Context, record Record) error {
	record.ProviderID = normalizeProviderID(record.ProviderID)
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	extraJSON, err := json.Marshal(record.Extra)
	if err != nil {
		return fmt.Errorf("marshal oauth extra: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO oauth_credentials (provider_id, refresh_token, access_token, expires_at_ms, extra_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider_id) DO UPDATE SET
			refresh_token=excluded.refresh_token,
			access_token=excluded.access_token,
			expires_at_ms=excluded.expires_at_ms,
			extra_json=excluded.extra_json,
			updated_at=excluded.updated_at
	`, record.ProviderID, record.Refresh, record.Access, record.Expires, string(extraJSON), record.CreatedAt.Unix(), record.UpdatedAt.Unix())
	if err != nil {
		return fmt.Errorf("upsert oauth credentials: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Close() error { return nil }
