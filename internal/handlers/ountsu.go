package handlers

import (
	"net/http"
	"seanime/internal/ountsu"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var ountsuUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all for this setup
	},
}

// HandleOuntsuWS upgrades the HTTP request to a WebSocket connection for Ountsu real-time voice and signaling.
//
//	@summary upgrades to Ountsu WebSocket
//	@desc Connects a client to an Ountsu room and initializes WebRTC signaling pumps.
//	@route /api/v1/ountsu/ws [GET]
func (h *Handler) HandleOuntsuWS(c echo.Context) error {
	if h.App.OuntsuHub == nil {
		return c.String(http.StatusNotImplemented, "Ountsu not enabled")
	}

	roomID := c.QueryParam("roomId")
	userID := c.QueryParam("userId")

	if roomID == "" || userID == "" {
		return c.String(http.StatusBadRequest, "roomId and userId are required")
	}

	conn, err := ountsuUpgrader.Upgrade(c.Response().Writer, c.Request(), nil)
	if err != nil {
		h.App.Logger.Error().Err(err).Msg("[OUNTSU] Upgrade error")
		return err
	}

	room := h.App.OuntsuHub.GetOrCreateRoom(roomID)
	client := ountsu.NewClient(userID, roomID, conn, room, h.App.OuntsuHub)

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

	return nil
}

// HandleOuntsuInvite generates an 8-character crypto invite token for an Ountsu room.
//
//	@summary generates an invite key
//	@desc Generates a crypto-secure 8-character hex invite key pre-creating an Ountsu room.
//	@route /api/v1/ountsu/invite [POST]
//	@returns map[string]string
func (h *Handler) HandleOuntsuInvite(c echo.Context) error {
	if h.App.OuntsuHub == nil {
		return c.String(http.StatusNotImplemented, "Ountsu not enabled")
	}

	key := h.App.OuntsuHub.GenerateInviteKey()

	return c.JSON(http.StatusOK, map[string]string{
		"inviteKey": key,
	})
}
