package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Elexation/onyx/internal/domain"
	"github.com/Elexation/onyx/internal/service"
)

type SettingsHandler struct {
	settings *service.SettingsService
	shares   *service.ShareService
	versions *service.VersionService
}

func NewSettingsHandler(settings *service.SettingsService, shares *service.ShareService, versions *service.VersionService) *SettingsHandler {
	return &SettingsHandler{settings: settings, shares: shares, versions: versions}
}

func (h *SettingsHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	all, err := h.settings.GetAll()
	if err != nil {
		http.Error(w, `{"error":"failed to load settings"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, all)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if len(updates) == 0 {
		http.Error(w, `{"error":"no settings provided"}`, http.StatusBadRequest)
		return
	}

	saved, errors := h.settings.Update(updates)

	// Delete all share links when sharing is disabled
	if v, ok := updates[domain.SettingSharesEnabled]; ok && v == "false" {
		if _, exists := errors[domain.SettingSharesEnabled]; !exists {
			if count, err := h.shares.DeleteAll(); err != nil {
				slog.Warn("failed to delete shares on disable", "error", err)
			} else if count > 0 {
				slog.Info("deleted all shares on sharing disable", "count", count)
			}
		}
	}

	// Delete all version files when versioning is disabled
	if v, ok := updates[domain.SettingVersionsEnabled]; ok && v == "false" {
		if _, exists := errors[domain.SettingVersionsEnabled]; !exists {
			if count, err := h.versions.PurgeAll(); err != nil {
				slog.Warn("failed to purge versions on disable", "error", err)
			} else if count > 0 {
				slog.Info("purged all versions on versioning disable", "count", count)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"saved":  saved,
		"errors": errors,
	})
}

