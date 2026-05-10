package admin

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"gomodel/internal/authkeys"
	"gomodel/internal/core"
)

type authKeyTransferRecord struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description,omitempty"`
	UserPath      string     `json:"user_path,omitempty"`
	RedactedValue string     `json:"redacted_value"`
	SecretHash    string     `json:"secret_hash"`
	Enabled       bool       `json:"enabled"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type importAuthKeysRequest struct {
	Records []authKeyTransferRecord `json:"records"`
}

func (h *Handler) ExportAuthKeys(c *echo.Context) error {
	if h.authKeys == nil {
		return handleError(c, featureUnavailableError("auth keys feature is unavailable"))
	}
	raw := h.authKeys.ExportRecords()
	records := make([]authKeyTransferRecord, 0, len(raw))
	for _, key := range raw {
		records = append(records, authKeyTransferRecord{
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
	return c.JSON(http.StatusOK, map[string]any{"records": records})
}

func (h *Handler) ImportAuthKeys(c *echo.Context) error {
	if h.authKeys == nil {
		return handleError(c, featureUnavailableError("auth keys feature is unavailable"))
	}
	var req importAuthKeysRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body", err))
	}
	records := make([]authkeys.AuthKey, 0, len(req.Records))
	for _, rec := range req.Records {
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
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "imported": len(req.Records)})
}
