package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"gomodel/internal/authkeys"
	"gomodel/internal/oauthcreds"
)

func TestExportImportTransferBundle(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	oauthStore := &mockOAuthCredStore{records: map[string]oauthcreds.Record{
		"openai-codex": {ProviderID: "openai-codex", Refresh: "refresh", Access: "access", Expires: now.Add(time.Hour).UnixMilli()},
	}}
	authStore := newAuthKeyTestStore(authkeys.AuthKey{
		ID:            "key-1",
		Name:          "primary",
		RedactedValue: "sk_gom_...abcd",
		SecretHash:    "hash-1",
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	h := newAuthKeyHandler(t, authStore)
	h.oauthCreds = oauthStore

	c, rec := newHandlerContext("/admin/api/v1/transfer/export")
	if err := h.ExportTransferBundle(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var exported transferBundle
	if err := json.Unmarshal(rec.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	if exported.Version != transferBundleVersion {
		t.Fatalf("version=%d", exported.Version)
	}
	if len(exported.OAuthCredentials) != 1 || exported.OAuthCredentials[0].Refresh != "refresh" {
		t.Fatalf("unexpected oauth export: %#v", exported.OAuthCredentials)
	}
	if len(exported.AuthKeys) != 1 || exported.AuthKeys[0].SecretHash != "hash-1" {
		t.Fatalf("unexpected auth-key export: %#v", exported.AuthKeys)
	}

	body := `{"version":1,"oauth_credentials":[{"provider_id":"github-copilot","refresh":"rt2","access":"at2","expires":123}],"auth_keys":[{"id":"key-2","name":"secondary","redacted_value":"sk_gom_...efgh","secret_hash":"hash-2","enabled":true,"created_at":"2026-05-10T15:00:00Z","updated_at":"2026-05-10T15:00:00Z"}]}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/v1/transfer/import", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req, rec2)
	if err := h.ImportTransferBundle(c2); err != nil {
		t.Fatal(err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	if _, ok := oauthStore.records["github-copilot"]; !ok {
		t.Fatal("expected oauth credential to be imported")
	}
	if _, ok := authStore.keys["key-2"]; !ok {
		t.Fatal("expected auth key to be imported")
	}
}
