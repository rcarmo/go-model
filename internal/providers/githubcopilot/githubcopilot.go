package githubcopilot

import (
	"net/http"
	"os"
	"strings"

	"gomodel/internal/core"
	"gomodel/internal/providers"
	"gomodel/internal/providers/oauthproxy"
	"gomodel/internal/providers/openai"

	goaioauth "github.com/rcarmo/go-ai/oauth"
)

const (
	providerID     = "github-copilot"
	defaultBaseURL = "https://api.individual.githubcopilot.com"
)

var Registration = providers.Registration{
	Type:                        providerID,
	New:                         New,
	PassthroughSemanticEnricher: openai.Registration.PassthroughSemanticEnricher,
	Discovery: providers.DiscoveryConfig{
		DefaultBaseURL:  defaultBaseURL,
		AllowAPIKeyless: true,
	},
}

var manager = oauthproxy.DefaultManager()

type Provider struct {
	*openai.CompatibleProvider
}

func New(cfg providers.ProviderConfig, opts providers.ProviderOptions) core.Provider {
	refresh := strings.TrimSpace(envOr("GITHUB_COPILOT_REFRESH_TOKEN", cfg.APIKey))
	access := strings.TrimSpace(os.Getenv("GITHUB_COPILOT_ACCESS_TOKEN"))
	expires := parseMillis(strings.TrimSpace(os.Getenv("GITHUB_COPILOT_EXPIRES_MS")))
	extra := map[string]interface{}{}
	if domain := strings.TrimSpace(os.Getenv("GITHUB_COPILOT_ENTERPRISE_DOMAIN")); domain != "" {
		extra["enterpriseUrl"] = domain
	}
	manager.SetInitial(providerID, refresh, access, expires, extra)

	baseURL := providers.ResolveBaseURL(cfg.BaseURL, defaultBaseURL)
	if token, creds, err := manager.GetAccessToken(providerID); err == nil {
		domain := ""
		if creds != nil && creds.Extra != nil {
			if d, ok := creds.Extra["enterpriseUrl"].(string); ok {
				domain = d
			}
		}
		if derived := goaioauth.GetGitHubCopilotBaseURL(token, domain); strings.TrimSpace(derived) != "" {
			baseURL = derived
		}
	}

	p := &Provider{}
	p.CompatibleProvider = openai.NewCompatibleProvider("", opts, openai.CompatibleProviderConfig{
		ProviderName: providerID,
		BaseURL:      baseURL,
		SetHeaders:   setHeaders,
	})
	return p
}

func setHeaders(req *http.Request, _ string) {
	token, _, err := manager.GetAccessToken(providerID)
	if err == nil && strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("User-Agent", "GitHubCopilotChat/0.35.0")
	req.Header.Set("Editor-Version", "vscode/1.107.0")
	req.Header.Set("Editor-Plugin-Version", "copilot-chat/0.35.0")
	req.Header.Set("Copilot-Integration-Id", "vscode-chat")
	if requestID := core.GetRequestID(req.Context()); requestID != "" && isValidClientRequestID(requestID) {
		req.Header.Set("X-Client-Request-Id", requestID)
	}
}

func isValidClientRequestID(id string) bool {
	if len(id) > 512 {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] > 127 {
			return false
		}
	}
	return true
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return strings.TrimSpace(fallback)
}

func parseMillis(v string) int64 {
	if v == "" {
		return 0
	}
	var out int64
	for _, r := range v {
		if r < '0' || r > '9' {
			return 0
		}
		out = out*10 + int64(r-'0')
	}
	return out
}
