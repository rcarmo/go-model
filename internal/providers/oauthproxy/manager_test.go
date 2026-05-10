package oauthproxy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	goaioauth "github.com/rcarmo/go-ai/oauth"
)

type fakeStore struct {
	data map[string]*goaioauth.Credentials
}

func (f *fakeStore) Get(_ context.Context, providerID string) (*goaioauth.Credentials, error) {
	if f.data == nil || f.data[providerID] == nil {
		return nil, os.ErrNotExist
	}
	return f.data[providerID], nil
}

func (f *fakeStore) Put(_ context.Context, providerID string, creds *goaioauth.Credentials) error {
	if f.data == nil {
		f.data = map[string]*goaioauth.Credentials{}
	}
	f.data[providerID] = creds
	return nil
}

func TestParseOpenAICodexAccountID(t *testing.T) {
	jwt := "eyJhbGciOiJub25lIn0.eyJodHRwczovL2FwaS5vcGVuYWkuY29tL2F1dGgiOnsiY2hhdGdwdF9hY2NvdW50X2lkIjoiYWNjdF8xMjMifX0."
	if got := ParseOpenAICodexAccountID(jwt); got != "acct_123" {
		t.Fatalf("ParseOpenAICodexAccountID() = %q, want acct_123", got)
	}
}

func TestManagerGetAccessTokenWithoutRefreshWhenValid(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "tokens.json")
	t.Setenv("GOMODEL_OAUTH_TOKEN_STORE", store)

	m := NewManager()
	m.SetInitial("openai-codex", "refresh-token", "access-token", time.Now().Add(10*time.Minute).UnixMilli(), nil)
	tok, creds, err := m.GetAccessToken("openai-codex")
	if err != nil {
		t.Fatalf("GetAccessToken error: %v", err)
	}
	if tok != "access-token" {
		t.Fatalf("token = %q, want access-token", tok)
	}
	if creds == nil || creds.Access != "access-token" {
		t.Fatalf("unexpected creds: %#v", creds)
	}
	if _, err := os.Stat(store); err != nil {
		t.Fatalf("expected token store file: %v", err)
	}
}

func TestManagerUseStoreMigratesAndReadsPersistentCredentials(t *testing.T) {
	m := NewManager()
	m.states["openai-codex"] = &providerState{ProviderID: "openai-codex", Creds: &goaioauth.Credentials{Refresh: "rt", Access: "at", Expires: time.Now().Add(time.Hour).UnixMilli()}}
	fs := &fakeStore{}
	if err := m.UseStore(fs); err != nil {
		t.Fatal(err)
	}
	if fs.data["openai-codex"] == nil || fs.data["openai-codex"].Refresh != "rt" {
		t.Fatalf("store not migrated: %#v", fs.data)
	}
	m.states = map[string]*providerState{}
	tok, creds, err := m.GetAccessToken("openai-codex")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "at" || creds.Access != "at" {
		t.Fatalf("unexpected creds after store read: %q %#v", tok, creds)
	}
}
