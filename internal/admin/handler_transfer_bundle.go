package admin

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"gomodel/internal/authkeys"
	"gomodel/internal/core"
	"gomodel/internal/oauthcreds"
)

const transferBundleVersion = 1

type transferBundle struct {
	Version          int                     `json:"version"`
	GeneratedAt      time.Time               `json:"generated_at,omitempty"`
	ProviderStatus   providerStatusResponse  `json:"provider_status,omitempty"`
	OAuthCredentials []oauthcreds.Record     `json:"oauth_credentials,omitempty"`
	AuthKeys         []authKeyTransferRecord `json:"auth_keys,omitempty"`
}

func (h *Handler) ExportTransferBundle(c *echo.Context) error {
	bundle := transferBundle{
		Version:        transferBundleVersion,
		GeneratedAt:    time.Now().UTC(),
		ProviderStatus: h.buildProviderStatusResponse(),
	}
	if h.oauthCreds != nil {
		records, err := h.oauthCreds.List(c.Request().Context())
		if err != nil {
			return handleError(c, core.NewProviderError("oauth_credentials", http.StatusInternalServerError, "failed to export oauth credentials", err))
		}
		bundle.OAuthCredentials = records
	}
	if h.authKeys != nil {
		for _, key := range h.authKeys.ExportRecords() {
			bundle.AuthKeys = append(bundle.AuthKeys, authKeyTransferRecord{
				ID:            key.ID,
				Name:          key.Name,
				Description:   key.Description,
				UserPath:      key.UserPath,
				RedactedValue: key.RedactedValue,
				SecretHash:    key.SecretHash,
				Enabled:       key.Enabled,
				ExpiresAt:     key.ExpiresAt,
				DeactivatedAt: key.DeactivatedAt,
				CreatedAt:     key.CreatedAt,
				UpdatedAt:     key.UpdatedAt,
			})
		}
	}
	if bundle.OAuthCredentials == nil {
		bundle.OAuthCredentials = []oauthcreds.Record{}
	}
	if bundle.AuthKeys == nil {
		bundle.AuthKeys = []authKeyTransferRecord{}
	}
	return c.JSON(http.StatusOK, bundle)
}

func (h *Handler) ImportTransferBundle(c *echo.Context) error {
	var bundle transferBundle
	if err := c.Bind(&bundle); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body", err))
	}
	if bundle.Version != 0 && bundle.Version != transferBundleVersion {
		return handleError(c, core.NewInvalidRequestError("unsupported transfer bundle version", nil))
	}
	if len(bundle.OAuthCredentials) > 0 {
		if h.oauthCreds == nil {
			return handleError(c, featureUnavailableError("oauth credentials feature is unavailable"))
		}
		for _, rec := range bundle.OAuthCredentials {
			if rec.ProviderID == "" {
				return handleError(c, core.NewInvalidRequestError("provider_id is required", nil))
			}
			if err := h.oauthCreds.Upsert(c.Request().Context(), rec); err != nil {
				return handleError(c, core.NewProviderError("oauth_credentials", http.StatusInternalServerError, "failed to import oauth credentials", err))
			}
		}
	}
	if len(bundle.AuthKeys) > 0 {
		if h.authKeys == nil {
			return handleError(c, featureUnavailableError("auth keys feature is unavailable"))
		}
		records := make([]authkeys.AuthKey, 0, len(bundle.AuthKeys))
		for _, rec := range bundle.AuthKeys {
			records = append(records, authkeys.AuthKey{
				ID:            rec.ID,
				Name:          rec.Name,
				Description:   rec.Description,
				UserPath:      rec.UserPath,
				RedactedValue: rec.RedactedValue,
				SecretHash:    rec.SecretHash,
				Enabled:       rec.Enabled,
				ExpiresAt:     rec.ExpiresAt,
				DeactivatedAt: rec.DeactivatedAt,
				CreatedAt:     rec.CreatedAt,
				UpdatedAt:     rec.UpdatedAt,
			})
		}
		if err := h.authKeys.ImportRecords(c.Request().Context(), records); err != nil {
			return handleError(c, authKeyWriteError(err))
		}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"ok":                         true,
		"imported_oauth_credentials": len(bundle.OAuthCredentials),
		"imported_auth_keys":         len(bundle.AuthKeys),
	})
}
