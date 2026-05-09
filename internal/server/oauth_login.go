package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gomodel/internal/core"
	"gomodel/internal/providers/oauthproxy"

	"github.com/labstack/echo/v5"
	goaioauth "github.com/rcarmo/go-ai/oauth"
)

var supportedOAuthProviders = map[string]struct{}{
	"openai-codex":   {},
	"github-copilot": {},
}

var (
	oauthProviderLookup = goaioauth.GetProvider
	oauthStartLogin     = func(p goaioauth.ProviderInterface, callbacks goaioauth.LoginCallbacks) (*goaioauth.Credentials, error) {
		return p.Login(callbacks)
	}
)

type oauthLoginRequest struct {
	Provider         string `json:"provider" form:"provider"`
	RefreshToken     string `json:"refresh_token" form:"refresh_token"`
	AccessToken      string `json:"access_token" form:"access_token"`
	ExpiresMS        string `json:"expires_ms" form:"expires_ms"`
	EnterpriseDomain string `json:"enterprise_domain" form:"enterprise_domain"`
}

type oauthStartRequest struct {
	Provider         string `json:"provider" form:"provider"`
	EnterpriseDomain string `json:"enterprise_domain" form:"enterprise_domain"`
}

type oauthSession struct {
	ID              string `json:"id"`
	Provider        string `json:"provider"`
	State           string `json:"state"`
	AuthURL         string `json:"authUrl,omitempty"`
	Instructions    string `json:"instructions,omitempty"`
	ProgressMessage string `json:"progressMessage,omitempty"`
	Error           string `json:"error,omitempty"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
}

type oauthSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*oauthSession
}

var loginSessions = &oauthSessionStore{sessions: map[string]*oauthSession{}}

func (s *oauthSessionStore) put(session *oauthSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
}

func (s *oauthSessionStore) get(id string) *oauthSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session := s.sessions[id]
	if session == nil {
		return nil
	}
	copy := *session
	return &copy
}

func (s *oauthSessionStore) patch(id string, fn func(*oauthSession)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[id]
	if session == nil {
		return
	}
	fn(session)
	session.UpdatedAt = time.Now().UnixMilli()
}

func newSessionID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return "sess_" + hex.EncodeToString(buf)
}

func (h *Handler) OAuthLoginUI(c *echo.Context) error {
	html := `<!doctype html><html><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/><title>GoModel OAuth Login</title><link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet"><style>:root{--bg:#0f172a;--card:#111827;--fg:#e5e7eb;--muted:#94a3b8;--accent:#38bdf8;--ok:#34d399;--err:#f87171;--border:#1f2937}[data-theme='light']{--bg:#f8fafc;--card:#fff;--fg:#0f172a;--muted:#475569;--accent:#0284c7;--ok:#059669;--err:#dc2626;--border:#e2e8f0}*{box-sizing:border-box}body{font-family:Inter,system-ui;background:var(--bg);color:var(--fg);margin:0;padding:2rem}main{max-width:900px;margin:0 auto}h1{margin:.2rem 0 1rem}section{background:var(--card);border:1px solid var(--border);border-radius:12px;padding:1rem;margin:0 0 1rem}label{display:block;margin:.6rem 0 .2rem;font-weight:600}input,select,button{width:100%;padding:.7rem;border-radius:10px;border:1px solid var(--border);background:transparent;color:inherit}button{cursor:pointer;background:var(--accent);border:none;color:white;font-weight:600}.row{display:grid;grid-template-columns:1fr 1fr;gap:.8rem}.muted{color:var(--muted)}pre{white-space:pre-wrap;word-break:break-word;background:#0003;padding:.8rem;border-radius:8px}a{color:var(--accent)}.ok{color:var(--ok)}.err{color:var(--err)}</style></head><body><main><h1>Provider Login</h1><p class="muted">OAuth device flow for OpenAI Codex and GitHub Copilot. Tokens are stored server-side and auto-refreshed.</p><section><h3>Start OAuth device flow</h3><div class="row"><div><label>Provider</label><select id="provider"><option value="openai-codex">openai-codex</option><option value="github-copilot">github-copilot</option></select></div><div><label>Copilot enterprise domain (optional)</label><input id="enterprise" placeholder="company.ghe.com"></div></div><p><button id="startBtn">Start login</button></p><div id="status" class="muted"></div><p id="authLinkWrap" style="display:none"><a id="authLink" target="_blank" rel="noreferrer">Open verification page</a></p><pre id="out"></pre></section><section><h3>Current token status</h3><div class="row"><button data-provider="openai-codex" class="checkBtn">Check openai-codex</button><button data-provider="github-copilot" class="checkBtn">Check github-copilot</button></div><pre id="statusOut"></pre></section></main><script>const out=document.getElementById('out');const status=document.getElementById('status');const authLinkWrap=document.getElementById('authLinkWrap');const authLink=document.getElementById('authLink');const statusOut=document.getElementById('statusOut');let pollTimer=null;async function check(provider){const r=await fetch('/auth/status/'+provider);statusOut.textContent=JSON.stringify(await r.json(),null,2)}document.querySelectorAll('.checkBtn').forEach(b=>b.onclick=()=>check(b.dataset.provider));async function poll(id){if(pollTimer)clearTimeout(pollTimer);const r=await fetch('/auth/oauth/session/'+id);const data=await r.json();out.textContent=JSON.stringify(data,null,2);status.textContent='State: '+(data.state||'unknown');status.className='muted '+(data.state==='complete'?'ok':(data.state==='error'?'err':''));if(data.authUrl){authLink.href=data.authUrl;authLink.textContent=data.instructions||'Open verification page';authLinkWrap.style.display='block'}if(data.state!=='complete'&&data.state!=='error'){pollTimer=setTimeout(()=>poll(id),2000)}}document.getElementById('startBtn').onclick=async()=>{authLinkWrap.style.display='none';status.textContent='Starting...';const payload={provider:document.getElementById('provider').value,enterprise_domain:document.getElementById('enterprise').value};const r=await fetch('/auth/oauth/start',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(payload)});const data=await r.json();if(!r.ok){out.textContent=JSON.stringify(data,null,2);status.textContent='Failed to start';status.className='muted err';return}if(data.session&&data.session.authUrl){authLink.href=data.session.authUrl;authLink.textContent=data.session.instructions||'Open verification page';authLinkWrap.style.display='block';status.textContent='Click the verification link below.';}poll(data.session.id)};</script></body></html>`
	return c.HTML(http.StatusOK, html)
}

func (h *Handler) OAuthStart(c *echo.Context) error {
	var req oauthStartRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body", err))
	}
	providerID := strings.TrimSpace(req.Provider)
	if _, ok := supportedOAuthProviders[providerID]; !ok {
		return handleError(c, core.NewInvalidRequestError("unsupported provider", nil))
	}
	provider := oauthProviderLookup(providerID)
	if provider == nil {
		return handleError(c, core.NewInvalidRequestError("oauth provider is unavailable", nil))
	}

	now := time.Now().UnixMilli()
	session := &oauthSession{ID: newSessionID(), Provider: providerID, State: "starting", CreatedAt: now, UpdatedAt: now}
	loginSessions.put(session)

	enterpriseDomain := strings.TrimSpace(req.EnterpriseDomain)
	go func(id string) {
		creds, err := oauthStartLogin(provider, goaioauth.LoginCallbacks{
			OnAuth: func(info goaioauth.AuthInfo) {
				loginSessions.patch(id, func(s *oauthSession) {
					s.State = "awaiting_verification"
					s.AuthURL = strings.TrimSpace(info.URL)
					s.Instructions = strings.TrimSpace(info.Instructions)
				})
			},
			OnProgress: func(message string) {
				loginSessions.patch(id, func(s *oauthSession) { s.ProgressMessage = strings.TrimSpace(message) })
			},
			OnPrompt: func(prompt goaioauth.Prompt) (string, error) {
				if providerID == "github-copilot" {
					return enterpriseDomain, nil
				}
				if prompt.AllowEmpty {
					return "", nil
				}
				return "", fmt.Errorf("interactive prompt %q requires user input", prompt.Message)
			},
		})
		if err != nil {
			loginSessions.patch(id, func(s *oauthSession) {
				s.State = "error"
				s.Error = err.Error()
			})
			return
		}
		extra := map[string]interface{}{}
		if providerID == "github-copilot" && enterpriseDomain != "" {
			extra["enterpriseUrl"] = enterpriseDomain
		}
		if len(extra) == 0 {
			extra = nil
		}
		if err := oauthproxy.DefaultManager().UpsertCredentials(providerID, creds.Refresh, creds.Access, creds.Expires, extra); err != nil {
			loginSessions.patch(id, func(s *oauthSession) {
				s.State = "error"
				s.Error = err.Error()
			})
			return
		}
		loginSessions.patch(id, func(s *oauthSession) { s.State = "complete" })
	}(session.ID)

	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) {
		current := loginSessions.get(session.ID)
		if current != nil {
			session = current
			if strings.TrimSpace(current.AuthURL) != "" || current.State == "error" {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	return c.JSON(http.StatusOK, map[string]any{"ok": true, "session": session})
}

func (h *Handler) OAuthSession(c *echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	session := loginSessions.get(id)
	if session == nil {
		return handleError(c, core.NewNotFoundError("oauth session not found"))
	}
	return c.JSON(http.StatusOK, session)
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
