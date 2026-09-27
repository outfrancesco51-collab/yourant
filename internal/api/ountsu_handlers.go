package api

import (
	"encoding/json"
	"log"
	"net/http"
	"yourant/internal/ountsu"

	"github.com/gorilla/websocket"
)

var ountsuUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all for this setup
	},
}

func (h *Handler) HandleOuntsuWS(w http.ResponseWriter, r *http.Request) {
	if h.ountsuHub == nil {
		http.Error(w, "Ountsu not enabled", http.StatusNotImplemented)
		return
	}

	roomID := r.URL.Query().Get("roomId")
	userID := r.URL.Query().Get("userId")

	if roomID == "" || userID == "" {
		http.Error(w, "roomId and userId are required", http.StatusBadRequest)
		return
	}

	conn, err := ountsuUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[OUNTSU] Upgrade error: %v", err)
		return
	}

	room := h.ountsuHub.GetOrCreateRoom(roomID)
	client := ountsu.NewClient(userID, roomID, conn, room, h.ountsuHub)

	room.AddClient(client)

	// Broadcast join to others
	room.Broadcast(&ountsu.OuntsuMessage{
		Type:     ountsu.TypeJoin,
		SenderID: userID,
		RoomID:   roomID,
	}, userID)

	// Provide the newly joined client with current room state
	client.SendMessage(room.GetStateMessage())

	go client.WritePump()
	go client.ReadPump()
}

func (h *Handler) HandleOuntsuInvite(w http.ResponseWriter, r *http.Request) {
	if h.ountsuHub == nil {
		http.Error(w, "Ountsu not enabled", http.StatusNotImplemented)
		return
	}

	key := h.ountsuHub.GenerateInviteKey()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"inviteKey": key,
	})
}
