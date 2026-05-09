package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestOAuthLoginAndStatus(t *testing.T) {
	t.Setenv("GOMODEL_OAUTH_TOKEN_STORE", t.TempDir()+"/oauth.json")

	e := echo.New()
	h := NewHandler(&mockProvider{}, nil, nil, nil)
	e.POST("/auth/login", h.OAuthLogin)
	e.GET("/auth/status/:provider", h.OAuthStatus)

	body := `{"provider":"openai-codex","refresh_token":"rt-123"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/auth/status/openai-codex", nil)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["hasRefreshToken"] != true {
		t.Fatalf("expected hasRefreshToken=true, got %v", payload["hasRefreshToken"])
	}
}
