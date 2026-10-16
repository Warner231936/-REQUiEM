package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type HealthStatus struct {
	Status     string         `json:"status"`
	Timestamp  time.Time      `json:"timestamp"`
	Version    string         `json:"version"`
	Uptime     string         `json:"uptime"`
	Database   string         `json:"database"`
	LLM        string         `json:"llm"`
	Memory     string         `json:"memory"`
	Goroutines int            `json:"goroutines"`
	Stats      *models.AnyMap `json:"stats,omitempty"`
}

type HealthChecker struct {
	store     *storage.MongoStore
	registry  *llm.ProviderRegistry
	logger    *slog.Logger
	startTime time.Time
	version   string
}

func NewHealthChecker(store *storage.MongoStore, registry *llm.ProviderRegistry, logger *slog.Logger) *HealthChecker {
	return &HealthChecker{
		store:     store,
		registry:  registry,
		logger:    logger,
		startTime: time.Now().UTC(),
		version:   "1.0.0",
	}
}

func (h *HealthChecker) Check(ctx context.Context) HealthStatus {
	status := HealthStatus{
		Status:     "healthy",
		Timestamp:  time.Now().UTC(),
		Version:    h.version,
		Uptime:     time.Since(h.startTime).String(),
		Goroutines: runtime.NumGoroutine(),
	}

	if _, err := h.store.Memory().Search(ctx, models.MemoryQuery{Limit: 1}); err != nil {
		status.Database = "unhealthy: " + err.Error()
		status.Status = "degraded"
	} else {
		status.Database = "healthy"
	}

	if provider, err := h.registry.Get("ollama"); err == nil {
		if err := provider.HealthCheck(ctx); err != nil {
			status.LLM = "unhealthy: " + err.Error()
			status.Status = "degraded"
		} else {
			status.LLM = "healthy"
		}
	} else {
		status.LLM = "not configured"
	}

	status.Memory = "healthy"

	return status
}

func (h *HealthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := h.Check(r.Context())

	httpStatus := http.StatusOK
	if status.Status == "degraded" {
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(status)
}
