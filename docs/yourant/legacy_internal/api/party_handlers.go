package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// HandleWatchPartyWS handles the WebSocket upgrade and connection for watch parties.
// URL: /api/watchparty/ws?roomId={roomId}&userId={userId}&name={name}&isHost={bool}
func (h *Handler) HandleWatchPartyWS(w http.ResponseWriter, r *http.Request) {
	// If party hub is not initialized, fallback
	if h.partyHub == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error":  "Watch Party hub is unavailable",
			"status": http.StatusServiceUnavailable,
		})
		return
	}

	q := r.URL.Query()
	roomID := strings.TrimSpace(q.Get("roomId"))
	userID := strings.TrimSpace(q.Get("userId"))
	userName := strings.TrimSpace(q.Get("name"))
	isHostStr := strings.TrimSpace(q.Get("isHost"))

	isHost, _ := strconv.ParseBool(isHostStr)
	if isHostStr == "1" {
		isHost = true
	}

	// Default user ID if omitted
	if userID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		userID = fmt.Sprintf("u-%s", hex.EncodeToString(b))
	}

	// Default user name if omitted
	if userName == "" {
		userName = "Guest"
	}

	// If no room ID provided, generate one and mark as host
	if roomID == "" {
		roomID = h.partyHub.GenerateRoomCode()
		isHost = true
	}

	// Validate room code format: ^WP-[A-Z0-9]{4}$
	if !h.partyHub.ValidateRoomCode(roomID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":  "invalid room code format: must match ^WP-[A-Z0-9]{4}$",
			"status": http.StatusBadRequest,
		})
		return
	}

	// Upgrade HTTP connection to WebSocket
	if err := h.partyHub.HandleWebSocket(w, r, roomID, userID, userName, isHost); err != nil {
		// HandleWebSocket already responded or logged
		return
	}
}

// HandleCreateRoomCode returns a newly generated room code matching ^WP-[A-Z0-9]{4}$.
func (h *Handler) HandleCreateRoomCode(w http.ResponseWriter, r *http.Request) {
	if h.partyHub == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "Watch party hub not initialized",
		})
		return
	}

	code := h.partyHub.GenerateRoomCode()
	writeJSON(w, http.StatusOK, map[string]any{
		"roomId":  code,
		"pattern": "^WP-[A-Z0-9]{4}$",
	})
}
