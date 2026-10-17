package handlers

import (
	"encoding/json"
	"net/http"
)

// HealthResponse is returned by GET /api/health.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// Health handles GET /api/health.
//
// @Summary      Health check
// @Description  Returns API health status
// @Tags         health
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Router       /api/health [get]
func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
}
