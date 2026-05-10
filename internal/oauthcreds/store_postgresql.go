package oauthcreds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgreSQLStore struct{ pool *pgxpool.Pool }

func NewPostgreSQLStore(ctx context.Context, pool *pgxpool.Pool) (*PostgreSQLStore, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if pool == nil {
		return nil, fmt.Errorf("connection pool is required")
	}
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS oauth_credentials (
			provider_id TEXT PRIMARY KEY,
			refresh_token TEXT NOT NULL DEFAULT '',
			access_token TEXT NOT NULL DEFAULT '',
			expires_at_ms BIGINT NOT NULL DEFAULT 0,
			extra_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create oauth_credentials table: %w", err)
	}
	return &PostgreSQLStore{pool: pool}, nil
}

func (s *PostgreSQLStore) List(ctx context.Context) ([]Record, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT provider_id, refresh_token, access_token, expires_at_ms, extra_json, created_at, updated_at
		FROM oauth_credentials ORDER BY provider_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list oauth credentials: %w", err)
	}
	defer rows.Close()
	result := make([]Record, 0)
	for rows.Next() {
		var rec Record
		var extraJSON []byte
		var createdAt, updatedAt int64
		if err := rows.Scan(&rec.ProviderID, &rec.Refresh, &rec.Access, &rec.Expires, &extraJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("iterate oauth credentials: %w", err)
		}
		if err := json.Unmarshal(extraJSON, &rec.Extra); err != nil {
			rec.Extra = map[string]any{}
		}
		rec.CreatedAt = time.Unix(createdAt, 0).UTC()
		rec.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		result = append(result, rec)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate oauth credentials: %w", rows.Err())
	}
	return result, nil
}

func (s *PostgreSQLStore) Get(ctx context.Context, providerID string) (*Record, error) {
	providerID = normalizeProviderID(providerID)
	var rec Record
	var extraJSON []byte
	var createdAt, updatedAt int64
	err := s.pool.QueryRow(ctx, `
		SELECT provider_id, refresh_token, access_token, expires_at_ms, extra_json, created_at, updated_at
		FROM oauth_credentials WHERE provider_id = $1
	`, providerID).Scan(&rec.ProviderID, &rec.Refresh, &rec.Access, &rec.Expires, &extraJSON, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get oauth credentials: %w", err)
	}
	if err := json.Unmarshal(extraJSON, &rec.Extra); err != nil {
		rec.Extra = map[string]any{}
	}
	rec.CreatedAt = time.Unix(createdAt, 0).UTC()
	rec.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &rec, nil
}

func (s *PostgreSQLStore) Upsert(ctx context.Context, record Record) error {
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
	_, err = s.pool.Exec(ctx, `
		INSERT INTO oauth_credentials (provider_id, refresh_token, access_token, expires_at_ms, extra_json, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT(provider_id) DO UPDATE SET
			refresh_token=excluded.refresh_token,
			access_token=excluded.access_token,
			expires_at_ms=excluded.expires_at_ms,
			extra_json=excluded.extra_json,
			updated_at=excluded.updated_at
	`, record.ProviderID, record.Refresh, record.Access, record.Expires, extraJSON, record.CreatedAt.Unix(), record.UpdatedAt.Unix())
	if err != nil {
		return fmt.Errorf("upsert oauth credentials: %w", err)
	}
	return nil
}

func (s *PostgreSQLStore) Close() error { return nil }
