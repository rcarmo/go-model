package oauthcreds

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteStoreUpsertAndGet(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Upsert(ctx, Record{ProviderID: "openai-codex", Refresh: "rt", Access: "at", Expires: 123, Extra: map[string]any{"x": "y"}}); err != nil {
		t.Fatal(err)
	}
	rec, err := store.Get(ctx, "openai-codex")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Refresh != "rt" || rec.Access != "at" || rec.Expires != 123 {
		t.Fatalf("unexpected record: %#v", rec)
	}
	if rec.Extra["x"] != "y" {
		t.Fatalf("unexpected extra: %#v", rec.Extra)
	}
}
