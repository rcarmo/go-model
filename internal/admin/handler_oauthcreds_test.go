package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"gomodel/internal/oauthcreds"
)

type mockOAuthCredStore struct{ records map[string]oauthcreds.Record }

func (m *mockOAuthCredStore) List(_ context.Context) ([]oauthcreds.Record, error) {
	out := make([]oauthcreds.Record, 0, len(m.records))
	for _, rec := range m.records {
		out = append(out, rec)
	}
	return out, nil
}
func (m *mockOAuthCredStore) Get(_ context.Context, providerID string) (*oauthcreds.Record, error) {
	rec, ok := m.records[providerID]
	if !ok {
		return nil, oauthcreds.ErrNotFound
	}
	return &rec, nil
}
func (m *mockOAuthCredStore) Upsert(_ context.Context, rec oauthcreds.Record) error {
	if m.records == nil {
		m.records = map[string]oauthcreds.Record{}
	}
	m.records[rec.ProviderID] = rec
	return nil
}
func (m *mockOAuthCredStore) Close() error { return nil }

func TestExportImportOAuthCredentials(t *testing.T) {
	store := &mockOAuthCredStore{records: map[string]oauthcreds.Record{
		"openai-codex": {ProviderID: "openai-codex", Refresh: "rt", Access: "at", Expires: time.Now().Add(time.Hour).UnixMilli()},
	}}
	h := NewHandler(nil, nil, WithOAuthCredentials(store))

	c, rec := newHandlerContext("/admin/api/v1/providers/oauth-credentials/export")
	if err := h.ExportOAuthCredentials(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	body := `{"records":[{"provider_id":"github-copilot","refresh":"rt2","access":"at2","expires":123}]}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/v1/providers/oauth-credentials/import", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req, rec2)
	if err := h.ImportOAuthCredentials(c2); err != nil {
		t.Fatal(err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	if _, ok := store.records["github-copilot"]; !ok {
		t.Fatal("expected github-copilot to be imported")
	}

	var exported map[string][]oauthcreds.Record
	if err := json.Unmarshal(rec.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	if len(exported["records"]) != 1 {
		t.Fatalf("expected 1 record, got %d", len(exported["records"]))
	}
}
