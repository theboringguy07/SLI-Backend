package handlers

import (
	"net/http"

	"github.com/sli/backend/internal/http/response"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	// In a complete implementation, this would check DB connectivity
	response.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
