package server

import (
	"net/http"
	"strconv"
	"strings"

	"gomodel/internal/core"
	"gomodel/internal/providers/oauthproxy"

	"github.com/labstack/echo/v5"
)

var supportedOAuthProviders = map[string]struct{}{
	"openai-codex":   {},
	"github-copilot": {},
}

type oauthLoginRequest struct {
	Provider         string `json:"provider" form:"provider"`
	RefreshToken     string `json:"refresh_token" form:"refresh_token"`
	AccessToken      string `json:"access_token" form:"access_token"`
	ExpiresMS        string `json:"expires_ms" form:"expires_ms"`
	EnterpriseDomain string `json:"enterprise_domain" form:"enterprise_domain"`
}

func (h *Handler) OAuthLoginUI(c *echo.Context) error {
	html := `<!doctype html><html><head><meta charset="utf-8"/><title>GoModel OAuth Login</title><style>body{font-family:system-ui;max-width:720px;margin:2rem auto;padding:0 1rem}input,select,button{width:100%;padding:.6rem;margin:.3rem 0}label{font-weight:600}small{color:#666}pre{background:#111;color:#0f0;padding:.75rem;overflow:auto}</style></head><body><h2>GoModel OAuth Login</h2><p>Save refresh tokens for OpenAI Codex and GitHub Copilot.</p><form id="f"><label>Provider</label><select name="provider"><option value="openai-codex">openai-codex</option><option value="github-copilot">github-copilot</option></select><label>Refresh token</label><input name="refresh_token" type="password" required/><label>Optional access token</label><input name="access_token" type="password"/><label>Optional expires (unix ms)</label><input name="expires_ms"/><label>Copilot enterprise domain (optional)</label><input name="enterprise_domain" placeholder="company.ghe.com"/><button type="submit">Save</button></form><pre id="out"></pre><script>const f=document.getElementById('f');const out=document.getElementById('out');f.addEventListener('submit',async(e)=>{e.preventDefault();const fd=new FormData(f);const body=Object.fromEntries(fd.entries());const r=await fetch('/auth/login',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(body)});out.textContent=JSON.stringify(await r.json(),null,2);});</script></body></html>`
	return c.HTML(http.StatusOK, html)
}

func (h *Handler) OAuthLogin(c *echo.Context) error {
	var req oauthLoginRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body", err))
	}
	provider := strings.TrimSpace(req.Provider)
	if _, ok := supportedOAuthProviders[provider]; !ok {
		return handleError(c, core.NewInvalidRequestError("unsupported provider", nil))
	}
	refresh := strings.TrimSpace(req.RefreshToken)
	if refresh == "" {
		return handleError(c, core.NewInvalidRequestError("refresh_token is required", nil))
	}
	var expires int64
	if v := strings.TrimSpace(req.ExpiresMS); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return handleError(c, core.NewInvalidRequestError("expires_ms must be unix milliseconds", err))
		}
		expires = n
	}
	extra := map[string]any{}
	if provider == "github-copilot" {
		if d := strings.TrimSpace(req.EnterpriseDomain); d != "" {
			extra["enterpriseUrl"] = d
		}
	}
	if len(extra) == 0 {
		extra = nil
	}
	if err := oauthproxy.DefaultManager().UpsertCredentials(provider, refresh, strings.TrimSpace(req.AccessToken), expires, extra); err != nil {
		return handleError(c, core.NewProviderError(provider, http.StatusInternalServerError, "failed to save oauth credentials", err))
	}
	status, err := oauthproxy.DefaultManager().Status(provider)
	if err != nil {
		return handleError(c, core.NewProviderError(provider, http.StatusInternalServerError, "failed to read oauth status", err))
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "status": status})
}

func (h *Handler) OAuthStatus(c *echo.Context) error {
	provider := strings.TrimSpace(c.Param("provider"))
	if _, ok := supportedOAuthProviders[provider]; !ok {
		return handleError(c, core.NewInvalidRequestError("unsupported provider", nil))
	}
	status, err := oauthproxy.DefaultManager().Status(provider)
	if err != nil {
		return handleError(c, core.NewProviderError(provider, http.StatusInternalServerError, "failed to read oauth status", err))
	}
	return c.JSON(http.StatusOK, status)
}
