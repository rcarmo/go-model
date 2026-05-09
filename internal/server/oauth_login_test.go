package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	goai "github.com/rcarmo/go-ai"
	goaioauth "github.com/rcarmo/go-ai/oauth"
)

type fakeOAuthProvider struct{}

func (f *fakeOAuthProvider) ID() string   { return "openai-codex" }
func (f *fakeOAuthProvider) Name() string { return "Fake" }
func (f *fakeOAuthProvider) Login(_ goaioauth.LoginCallbacks) (*goaioauth.Credentials, error) {
	return &goaioauth.Credentials{Refresh: "rt", Access: "at", Expires: time.Now().Add(time.Hour).UnixMilli()}, nil
}
func (f *fakeOAuthProvider) RefreshToken(c *goaioauth.Credentials) (*goaioauth.Credentials, error) {
	return c, nil
}
func (f *fakeOAuthProvider) GetAPIKey(c *goaioauth.Credentials) string { return c.Access }
func (f *fakeOAuthProvider) ModifyModels(models []*goai.Model, _ *goaioauth.Credentials) []*goai.Model {
	return models
}

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

func TestOAuthStartAndSession(t *testing.T) {
	t.Setenv("GOMODEL_OAUTH_TOKEN_STORE", t.TempDir()+"/oauth.json")
	origLookup := oauthProviderLookup
	origStart := oauthStartLogin
	t.Cleanup(func() {
		oauthProviderLookup = origLookup
		oauthStartLogin = origStart
	})
	oauthProviderLookup = func(id string) goaioauth.ProviderInterface {
		if id == "openai-codex" {
			return &fakeOAuthProvider{}
		}
		return nil
	}
	oauthStartLogin = func(p goaioauth.ProviderInterface, callbacks goaioauth.LoginCallbacks) (*goaioauth.Credentials, error) {
		if callbacks.OnAuth != nil {
			callbacks.OnAuth(goaioauth.AuthInfo{URL: "https://example.com", Instructions: "code XYZ"})
		}
		return p.Login(callbacks)
	}

	e := echo.New()
	h := NewHandler(&mockProvider{}, nil, nil, nil)
	e.POST("/auth/oauth/start", h.OAuthStart)
	e.GET("/auth/oauth/session/:id", h.OAuthSession)

	req := httptest.NewRequest(http.MethodPost, "/auth/oauth/start", strings.NewReader(`{"provider":"openai-codex"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var start map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatal(err)
	}
	session := start["session"].(map[string]any)
	id := session["id"].(string)

	var got map[string]any
	for i := 0; i < 20; i++ {
		req2 := httptest.NewRequest(http.MethodGet, "/auth/oauth/session/"+id, nil)
		rec2 := httptest.NewRecorder()
		e.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec2.Code, rec2.Body.String())
		}
		if err := json.Unmarshal(rec2.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got["state"] == "complete" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got["state"] != "complete" {
		t.Fatalf("state=%v, want complete", got["state"])
	}
}
