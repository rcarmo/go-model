package oauthproxy

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	goaioauth "github.com/rcarmo/go-ai/oauth"
)

const defaultStorePath = "/workspace/tmp/gomodel-oauth-tokens.json"

type providerState struct {
	ProviderID string                 `json:"providerId"`
	Creds      *goaioauth.Credentials `json:"credentials"`
}

type tokenStoreFile struct {
	Providers map[string]*providerState `json:"providers"`
}

// Manager handles OAuth access token reuse and refresh with file-backed persistence
// so a single proxy instance can serve multiple machines without per-machine logins.
type Manager struct {
	mu        sync.Mutex
	storePath string
	states    map[string]*providerState
}

func NewManager() *Manager {
	path := strings.TrimSpace(os.Getenv("GOMODEL_OAUTH_TOKEN_STORE"))
	if path == "" {
		path = defaultStorePath
	}
	m := &Manager{storePath: path, states: make(map[string]*providerState)}
	_ = m.loadLocked()
	return m
}

func (m *Manager) SetInitial(providerID, refresh, access string, expires int64, extra map[string]interface{}) {
	if strings.TrimSpace(providerID) == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.states[providerID]
	if st == nil {
		st = &providerState{ProviderID: providerID, Creds: &goaioauth.Credentials{}}
		m.states[providerID] = st
	}
	if st.Creds == nil {
		st.Creds = &goaioauth.Credentials{}
	}
	if strings.TrimSpace(refresh) != "" {
		st.Creds.Refresh = strings.TrimSpace(refresh)
	}
	if strings.TrimSpace(access) != "" {
		st.Creds.Access = strings.TrimSpace(access)
	}
	if expires > 0 {
		st.Creds.Expires = expires
	}
	if extra != nil {
		st.Creds.Extra = extra
	}
	_ = m.saveLocked()
}

func (m *Manager) GetAccessToken(providerID string) (string, *goaioauth.Credentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.loadLocked(); err != nil {
		return "", nil, err
	}

	st := m.states[providerID]
	if st == nil || st.Creds == nil {
		return "", nil, fmt.Errorf("oauth creds for %s not configured", providerID)
	}
	if strings.TrimSpace(st.Creds.Refresh) == "" {
		return "", nil, fmt.Errorf("oauth refresh token for %s is empty", providerID)
	}

	// Refresh when missing or near expiry (60s skew)
	now := time.Now().UnixMilli()
	if strings.TrimSpace(st.Creds.Access) == "" || st.Creds.Expires <= now+60_000 {
		provider := goaioauth.GetProvider(providerID)
		if provider == nil {
			return "", nil, fmt.Errorf("oauth provider %s not registered", providerID)
		}
		fresh, err := provider.RefreshToken(st.Creds)
		if err != nil {
			return "", nil, fmt.Errorf("oauth refresh failed for %s: %w", providerID, err)
		}
		st.Creds = fresh
		m.states[providerID] = st
		if err := m.saveLocked(); err != nil {
			return "", nil, err
		}
	}

	provider := goaioauth.GetProvider(providerID)
	if provider == nil {
		return "", nil, fmt.Errorf("oauth provider %s not registered", providerID)
	}
	return provider.GetAPIKey(st.Creds), st.Creds, nil
}

func (m *Manager) loadLocked() error {
	b, err := os.ReadFile(m.storePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var f tokenStoreFile
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	if f.Providers == nil {
		return nil
	}
	for k, v := range f.Providers {
		m.states[k] = v
	}
	return nil
}

func (m *Manager) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(m.storePath), 0o755); err != nil {
		return err
	}
	f := tokenStoreFile{Providers: m.states}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.storePath, b, 0o600)
}

func ParseOpenAICodexAccountID(jwt string) string {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var raw map[string]any
	if json.Unmarshal(payload, &raw) != nil {
		return ""
	}
	auth, _ := raw["https://api.openai.com/auth"].(map[string]any)
	acct, _ := auth["chatgpt_account_id"].(string)
	return strings.TrimSpace(acct)
}
