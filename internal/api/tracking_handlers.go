package api

import (
	"encoding/json"
	"net/http"
	"yourant/internal/storage"
)

// HandleGetTrackingList retrieves all tracked anime or filtered by status.
func (h *Handler) HandleGetTrackingList(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	entries := h.storage.GetList(statusFilter)

	if entries == nil {
		entries = []*storage.UserLibraryEntry{}
	}

	writeJSON(w, http.StatusOK, storage.UserLibraryResponse{
		Lists: entries,
	})
}

// HandleUpdateTracking modifies or creates a user tracking entry.
func (h *Handler) HandleUpdateTracking(w http.ResponseWriter, r *http.Request) {
	var req storage.TrackingUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if req.MediaID <= 0 {
		writeError(w, http.StatusBadRequest, "mediaId is required and must be positive")
		return
	}

	updated, err := h.storage.Update(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Persist changes asynchronously or immediately
	_ = h.storage.Save()

	writeJSON(w, http.StatusOK, updated)
}
