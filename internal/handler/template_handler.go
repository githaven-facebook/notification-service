package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/nicedavid98/notification-service/internal/model"
	"github.com/nicedavid98/notification-service/internal/repository"
	tmpl "github.com/nicedavid98/notification-service/internal/template"
)

// TemplateHandler handles HTTP endpoints for notification template CRUD (admin only).
type TemplateHandler struct {
	repo   repository.TemplateRepository
	engine *tmpl.Engine
	logger *zap.Logger
}

// NewTemplateHandler creates a new template HTTP handler.
func NewTemplateHandler(repo repository.TemplateRepository, engine *tmpl.Engine, logger *zap.Logger) *TemplateHandler {
	return &TemplateHandler{repo: repo, engine: engine, logger: logger}
}

// List handles GET /api/v1/admin/templates.
func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	channelStr := r.URL.Query().Get("channel")
	channel := model.NotificationChannel(channelStr)

	limit := 50
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

	templates, err := h.repo.List(r.Context(), channel, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"templates": templates,
		"limit":     limit,
		"offset":    offset,
	})
}

// Get handles GET /api/v1/admin/templates/{id}.
func (h *TemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	t, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if t == nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	writeJSON(w, http.StatusOK, t)
}

// Create handles POST /api/v1/admin/templates.
func (h *TemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Validate template syntax
	if err := tmpl.ValidateSyntax("body", req.Body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Subject != "" {
		if err := tmpl.ValidateSyntax("subject", req.Subject); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	t := &model.NotificationTemplate{
		Name:    req.Name,
		Channel: req.Channel,
		Subject: req.Subject,
		Body:    req.Body,
		Locale:  req.Locale,
		Version: req.Version,
		Active:  true,
	}
	if t.Locale == "" {
		t.Locale = "en"
	}
	if t.Version == 0 {
		t.Version = 1
	}

	if err := h.repo.Create(r.Context(), t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Invalidate cache
	h.engine.InvalidateCache()

	h.logger.Info("Template created",
		zap.String("name", t.Name),
		zap.String("channel", string(t.Channel)),
		zap.String("locale", t.Locale),
	)

	writeJSON(w, http.StatusCreated, t)
}

// Update handles PUT /api/v1/admin/templates/{id}.
func (h *TemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	var req model.UpdateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Body != "" {
		if err := tmpl.ValidateSyntax("body", req.Body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if err := h.repo.Update(r.Context(), id, &req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Invalidate cache
	h.engine.InvalidateCache()

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// Delete handles DELETE /api/v1/admin/templates/{id}.
func (h *TemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.engine.InvalidateCache()

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
