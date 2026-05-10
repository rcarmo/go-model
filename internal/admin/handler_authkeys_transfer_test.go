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
)

func TestExportImportAuthKeys(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	store := newAuthKeyTestStore(authkeys.AuthKey{
		ID:            "key-1",
		Name:          "primary",
		RedactedValue: "sk_gom_...abcd",
		SecretHash:    "hash-1",
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	h := newAuthKeyHandler(t, store)

	c, rec := newHandlerContext("/admin/api/v1/auth-keys/export")
	if err := h.ExportAuthKeys(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var exported struct {
		Records []authKeyTransferRecord `json:"records"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	if len(exported.Records) != 1 || exported.Records[0].SecretHash != "hash-1" {
		t.Fatalf("unexpected export payload: %#v", exported.Records)
	}

	body := `{"records":[{"id":"key-2","name":"secondary","redacted_value":"sk_gom_...efgh","secret_hash":"hash-2","enabled":true,"created_at":"2026-05-10T15:00:00Z","updated_at":"2026-05-10T15:00:00Z"}]}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/v1/auth-keys/import", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req, rec2)
	if err := h.ImportAuthKeys(c2); err != nil {
		t.Fatal(err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	if _, ok := store.keys["key-2"]; !ok {
		t.Fatal("expected imported auth key")
	}
}
