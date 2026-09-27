package api

import (
	"net/http"
	"time"
)

// HandleHealth responds to health check queries.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"version":       h.cfg.Version,
		"uptimeSeconds": time.Since(h.startTime).Seconds(),
		"timestamp":     time.Now().Unix(),
	})
}
