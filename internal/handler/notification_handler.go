package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/service"
)

// NotificationHandler handles HTTP endpoints for notification operations.
type NotificationHandler struct {
	svc    *service.NotificationService
	logger *zap.Logger
}

// NewNotificationHandler creates a new notification HTTP handler.
func NewNotificationHandler(svc *service.NotificationService, logger *zap.Logger) *NotificationHandler {
	return &NotificationHandler{svc: svc, logger: logger}
}

// Send handles POST /api/v1/notifications/send.
func (h *NotificationHandler) Send(w http.ResponseWriter, r *http.Request) {
	var req model.SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Set default priority
	if req.Priority == "" {
		req.Priority = model.PriorityNormal
	}

	n, err := h.svc.Send(r.Context(), &req)
	if err != nil {
		h.logger.Error("Send notification failed",
			zap.Error(err),
			zap.String("user_id", req.UserID),
			zap.String("request_id", GetRequestID(r.Context())),
		)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, n)
}

// SendBatch handles POST /api/v1/notifications/batch.
func (h *NotificationHandler) SendBatch(w http.ResponseWriter, r *http.Request) {
	var req model.BatchSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if len(req.Notifications) == 0 {
		writeError(w, http.StatusBadRequest, "notifications list is empty")
		return
	}

	if len(req.Notifications) > 100 {
		writeError(w, http.StatusBadRequest, "batch size exceeds maximum of 100")
		return
	}

	// Set default priority for each notification
	for i := range req.Notifications {
		if req.Notifications[i].Priority == "" {
			req.Notifications[i].Priority = model.PriorityNormal
		}
	}

	results, errs := h.svc.SendBatch(r.Context(), &req)

	type batchResult struct {
		Index        int                `json:"index"`
		Notification *model.Notification `json:"notification,omitempty"`
		Error        string             `json:"error,omitempty"`
	}

	batchResults := make([]batchResult, len(results))
	for i := range results {
		batchResults[i] = batchResult{Index: i, Notification: results[i]}
		if errs[i] != nil {
			batchResults[i].Error = errs[i].Error()
		}
	}

	writeJSON(w, http.StatusMultiStatus, map[string]interface{}{
		"results": batchResults,
		"total":   len(results),
	})
}

// GetStatus handles GET /api/v1/notifications/{id}/status.
func (h *NotificationHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid notification ID")
		return
	}

	n, err := h.svc.GetNotification(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n == nil {
		writeError(w, http.StatusNotFound, "notification not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":           n.ID,
		"status":       n.Status,
		"retry_count":  n.RetryCount,
		"sent_at":      n.SentAt,
		"delivered_at": n.DeliveredAt,
		"error":        n.ErrorMessage,
	})
}

// GetUserNotifications handles GET /api/v1/notifications/user/{userId}.
func (h *NotificationHandler) GetUserNotifications(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "userId is required")
		return
	}

	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	notifications, err := h.svc.GetUserNotifications(r.Context(), userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"notifications": notifications,
		"limit":         limit,
		"offset":        offset,
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
