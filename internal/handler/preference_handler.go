package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/service"
)

// PreferenceHandler handles HTTP endpoints for user notification preferences.
type PreferenceHandler struct {
	svc    *service.PreferenceService
	logger *zap.Logger
}

// NewPreferenceHandler creates a new preference HTTP handler.
func NewPreferenceHandler(svc *service.PreferenceService, logger *zap.Logger) *PreferenceHandler {
	return &PreferenceHandler{svc: svc, logger: logger}
}

// GetPreferences handles GET /api/v1/preferences/{userId}.
func (h *PreferenceHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "userId is required")
		return
	}

	prefs, err := h.svc.GetPreferences(r.Context(), userID)
	if err != nil {
		h.logger.Error("Get preferences failed",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":     userID,
		"preferences": prefs,
	})
}

// UpdatePreference handles PUT /api/v1/preferences/{userId}/{channel}.
func (h *PreferenceHandler) UpdatePreference(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	channelStr := chi.URLParam(r, "channel")

	if userID == "" {
		writeError(w, http.StatusBadRequest, "userId is required")
		return
	}

	channel := model.NotificationChannel(channelStr)
	if !channel.IsValid() {
		writeError(w, http.StatusBadRequest, "invalid channel: "+channelStr)
		return
	}

	var req model.UpdatePreferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	pref, err := h.svc.UpdatePreference(r.Context(), userID, channel, &req)
	if err != nil {
		h.logger.Error("Update preference failed",
			zap.Error(err),
			zap.String("user_id", userID),
			zap.String("channel", channelStr),
		)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, pref)
}
