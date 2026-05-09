package openaicodex

import (
	"net/http"
	"os"
	"strings"

	"gomodel/internal/core"
	"gomodel/internal/providers"
	"gomodel/internal/providers/oauthproxy"
	"gomodel/internal/providers/openai"
)

const (
	providerID     = "openai-codex"
	defaultBaseURL = "https://api.openai.com/v1"
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
	refresh := strings.TrimSpace(envOr("OPENAI_CODEX_REFRESH_TOKEN", cfg.APIKey))
	access := strings.TrimSpace(os.Getenv("OPENAI_CODEX_ACCESS_TOKEN"))
	expires := parseMillis(strings.TrimSpace(os.Getenv("OPENAI_CODEX_EXPIRES_MS")))
	manager.SetInitial(providerID, refresh, access, expires, nil)

	baseURL := providers.ResolveBaseURL(cfg.BaseURL, defaultBaseURL)
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
		if acct := oauthproxy.ParseOpenAICodexAccountID(token); acct != "" {
			req.Header.Set("chatgpt-account-id", acct)
		}
	}
	req.Header.Set("originator", "pi")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
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
