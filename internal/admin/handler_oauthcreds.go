package admin

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"gomodel/internal/core"
	"gomodel/internal/oauthcreds"
)

type importOAuthCredentialsRequest struct {
	Records []oauthcreds.Record `json:"records"`
}

func (h *Handler) ExportOAuthCredentials(c *echo.Context) error {
	if h.oauthCreds == nil {
		return handleError(c, featureUnavailableError("oauth credentials feature is unavailable"))
	}
	records, err := h.oauthCreds.List(c.Request().Context())
	if err != nil {
		return handleError(c, core.NewProviderError("oauth_credentials", http.StatusInternalServerError, "failed to export oauth credentials", err))
	}
	if records == nil {
		records = []oauthcreds.Record{}
	}
	return c.JSON(http.StatusOK, map[string]any{"records": records})
}

func (h *Handler) ImportOAuthCredentials(c *echo.Context) error {
	if h.oauthCreds == nil {
		return handleError(c, featureUnavailableError("oauth credentials feature is unavailable"))
	}
	var req importOAuthCredentialsRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body", err))
	}
	for _, rec := range req.Records {
		if rec.ProviderID == "" {
			return handleError(c, core.NewInvalidRequestError("provider_id is required", nil))
		}
		if err := h.oauthCreds.Upsert(c.Request().Context(), rec); err != nil {
			return handleError(c, core.NewProviderError("oauth_credentials", http.StatusInternalServerError, "failed to import oauth credentials", err))
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "imported": len(req.Records)})
}
