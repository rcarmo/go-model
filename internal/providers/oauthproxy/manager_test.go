package oauthproxy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
