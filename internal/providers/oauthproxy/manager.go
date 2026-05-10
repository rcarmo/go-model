package oauthproxy

import (
	"context"
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

// ProviderStatus is a redacted auth status payload for UX/API usage.
type ProviderStatus struct {
	ProviderID         string `json:"providerId"`
	Configured         bool   `json:"configured"`
	HasRefreshToken    bool   `json:"hasRefreshToken"`
	HasAccessToken     bool   `json:"hasAccessToken"`
	AccessTokenExpires int64  `json:"accessTokenExpires"`
}

type CredentialStore interface {
	Get(ctx context.Context, providerID string) (*goaioauth.Credentials, error)
	Put(ctx context.Context, providerID string, creds *goaioauth.Credentials) error
}

type tokenStoreFile struct {
	Providers map[string]*providerState `json:"providers"`
}

// Manager handles OAuth access token reuse and refresh.
// It prefers a first-class persistent store when configured and falls back to a
// local JSON file for legacy/dev compatibility.
type Manager struct {
	mu        sync.Mutex
	storePath string
	states    map[string]*providerState
	store     CredentialStore
}

var (
	defaultManager *Manager
	defaultOnce    sync.Once
)

func DefaultManager() *Manager {
	defaultOnce.Do(func() { defaultManager = NewManager() })
	return defaultManager
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

func (m *Manager) UseStore(store CredentialStore) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = store
	if m.store == nil {
		return nil
	}
	if err := m.loadLocked(); err != nil {
		return err
	}
	for providerID, st := range m.states {
		if st == nil || st.Creds == nil {
			continue
		}
		if err := m.store.Put(context.Background(), providerID, cloneCredentials(st.Creds)); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) SetInitial(providerID, refresh, access string, expires int64, extra map[string]interface{}) {
	_ = m.UpsertCredentials(providerID, refresh, access, expires, extra)
}

func (m *Manager) UpsertCredentials(providerID, refresh, access string, expires int64, extra map[string]interface{}) error {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return fmt.Errorf("provider id is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	st, err := m.loadStateLocked(providerID)
	if err != nil {
		return err
	}
	if st == nil {
		st = &providerState{ProviderID: providerID, Creds: &goaioauth.Credentials{}}
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
	m.states[providerID] = st
	return m.persistStateLocked(providerID, st.Creds)
}

func (m *Manager) Status(providerID string) (ProviderStatus, error) {
	providerID = strings.TrimSpace(providerID)
	status := ProviderStatus{ProviderID: providerID}
	if providerID == "" {
		return status, fmt.Errorf("provider id is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	st, err := m.loadStateLocked(providerID)
	if err != nil {
		return status, err
	}
	if st == nil || st.Creds == nil {
		return status, nil
	}
	status.Configured = true
	status.HasRefreshToken = strings.TrimSpace(st.Creds.Refresh) != ""
	status.HasAccessToken = strings.TrimSpace(st.Creds.Access) != ""
	status.AccessTokenExpires = st.Creds.Expires
	return status, nil
}

func (m *Manager) GetAccessToken(providerID string) (string, *goaioauth.Credentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, err := m.loadStateLocked(providerID)
	if err != nil {
		return "", nil, err
	}
	if st == nil || st.Creds == nil {
		return "", nil, fmt.Errorf("oauth creds for %s not configured", providerID)
	}
	if strings.TrimSpace(st.Creds.Refresh) == "" {
		return "", nil, fmt.Errorf("oauth refresh token for %s is empty", providerID)
	}

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
		if err := m.persistStateLocked(providerID, st.Creds); err != nil {
			return "", nil, err
		}
	}

	provider := goaioauth.GetProvider(providerID)
	if provider == nil {
		return "", nil, fmt.Errorf("oauth provider %s not registered", providerID)
	}
	return provider.GetAPIKey(st.Creds), cloneCredentials(st.Creds), nil
}

func (m *Manager) loadStateLocked(providerID string) (*providerState, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return nil, nil
	}
	if m.store != nil {
		creds, err := m.store.Get(context.Background(), providerID)
		if err == nil {
			st := &providerState{ProviderID: providerID, Creds: cloneCredentials(creds)}
			m.states[providerID] = st
			return st, nil
		}
		if !errors.Is(err, os.ErrNotExist) && !strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, err
		}
	}
	if err := m.loadLocked(); err != nil {
		return nil, err
	}
	return m.states[providerID], nil
}

func (m *Manager) persistStateLocked(providerID string, creds *goaioauth.Credentials) error {
	if m.store != nil {
		return m.store.Put(context.Background(), providerID, cloneCredentials(creds))
	}
	return m.saveLocked()
}

func cloneCredentials(creds *goaioauth.Credentials) *goaioauth.Credentials {
	if creds == nil {
		return nil
	}
	cloned := &goaioauth.Credentials{Refresh: creds.Refresh, Access: creds.Access, Expires: creds.Expires}
	if creds.Extra != nil {
		cloned.Extra = make(map[string]interface{}, len(creds.Extra))
		for k, v := range creds.Extra {
			cloned.Extra[k] = v
		}
	}
	return cloned
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
