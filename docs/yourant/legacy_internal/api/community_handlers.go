package api

import (
	"encoding/json"
	"net/http"
	"time"
)

type BanStatusResponse struct {
	IsBanned bool    `json:"isBanned"`
	Reason   string  `json:"reason,omitempty"`
	DaysLeft float64 `json:"daysLeft,omitempty"`
}

func (h *Handler) HandleGetBanStatus(w http.ResponseWriter, r *http.Request) {
	if h.banSystem == nil {
		http.Error(w, "Community mod not enabled", http.StatusNotImplemented)
		return
	}

	userID := r.PathValue("id")
	if userID == "" {
		http.Error(w, "missing user id", http.StatusBadRequest)
		return
	}

	record, banned := h.banSystem.GetBan(userID)
	var resp BanStatusResponse

	if banned && record != nil {
		resp.IsBanned = true
		resp.Reason = record.Reason
		resp.DaysLeft = time.Until(record.Expiration).Hours() / 24.0
	} else {
		resp.IsBanned = false
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) HandleCommunityPost(w http.ResponseWriter, r *http.Request) {
	if h.autoMod == nil {
		http.Error(w, "Community mod not enabled", http.StatusNotImplemented)
		return
	}

	var req struct {
		UserID  string `json:"userId"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	sanitizedMsg, wasBanned := h.autoMod.ProcessMessage(req.UserID, req.Message)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sanitized": sanitizedMsg,
		"banned":    wasBanned,
	})
}
