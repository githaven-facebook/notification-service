package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// HealthHandler handles health and readiness check endpoints.
type HealthHandler struct {
	db          *pgxpool.Pool
	redisClient *redis.Client
	startTime   time.Time
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(db *pgxpool.Pool, redisClient *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:          db,
		redisClient: redisClient,
		startTime:   time.Now(),
	}
}

type healthResponse struct {
	Status    string            `json:"status"`
	Uptime    string            `json:"uptime"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// Health handles GET /health - basic liveness check.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, &healthResponse{
		Status:    "ok",
		Uptime:    time.Since(h.startTime).String(),
		Timestamp: time.Now(),
	})
}

// Ready handles GET /ready - readiness check including dependencies.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string)
	allOK := true

	// Check PostgreSQL
	if err := h.db.Ping(r.Context()); err != nil {
		checks["postgres"] = "unhealthy: " + err.Error()
		allOK = false
	} else {
		checks["postgres"] = "healthy"
	}

	// Check Redis
	if err := h.redisClient.Ping(r.Context()).Err(); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
		allOK = false
	} else {
		checks["redis"] = "healthy"
	}

	status := "ok"
	statusCode := http.StatusOK
	if !allOK {
		status = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	writeJSON(w, statusCode, &healthResponse{
		Status:    status,
		Uptime:    time.Since(h.startTime).String(),
		Timestamp: time.Now(),
		Checks:    checks,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
