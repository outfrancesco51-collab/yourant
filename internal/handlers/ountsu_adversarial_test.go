package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"seanime/internal/core"
	"seanime/internal/handlers"
	"seanime/internal/ountsu"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*httptest.Server, *ountsu.Hub) {
	e := echo.New()
	hub := ountsu.NewHub()
	nopLogger := zerolog.Nop()
	app := &core.App{
		OuntsuHub: hub,
		Logger:    &nopLogger,
	}
	h := &handlers.Handler{App: app}

	v1 := e.Group("/api/v1")
	v1Ountsu := v1.Group("/ountsu")
	v1Ountsu.GET("/ws", h.HandleOuntsuWS)
	v1Ountsu.POST("/invite", h.HandleOuntsuInvite)

	ts := httptest.NewServer(e)
	t.Cleanup(ts.Close)
	return ts, hub
}

func TestOuntsu_LiveWebSocket_EndToEndSignaling(t *testing.T) {
	ts, hub := setupTestServer(t)

	// 1. Generate Invite Token
	inviteResp, err := http.Post(ts.URL+"/api/v1/ountsu/invite", "application/json", nil)
	require.NoError(t, err)
	defer inviteResp.Body.Close()
	require.Equal(t, http.StatusOK, inviteResp.StatusCode)

	var inviteData map[string]string
	err = json.NewDecoder(inviteResp.Body).Decode(&inviteData)
	require.NoError(t, err)
	roomID := inviteData["inviteKey"]
	require.Len(t, roomID, 8, "Invite key must be 8 hex characters")

	// Verify room was pre-created in hub
	room := hub.GetOrCreateRoom(roomID)
	require.NotNil(t, room)
	require.Equal(t, roomID, room.ID)

	// 2. Connect Client 1 via WebSocket
	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"
	u.Path = "/api/v1/ountsu/ws"
	q1 := u.Query()
	q1.Set("roomId", roomID)
	q1.Set("userId", "peer1")
	u.RawQuery = q1.Encode()

	ws1, resp1, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer ws1.Close()
	defer resp1.Body.Close()

	// Client 1 should receive initial RoomState message
	_ = ws1.SetReadDeadline(time.Now().Add(2 * time.Second))
	var stateMsg ountsu.OuntsuMessage
	err = ws1.ReadJSON(&stateMsg)
	require.NoError(t, err)
	assert.Equal(t, ountsu.TypeRoomState, stateMsg.Type)
	assert.Equal(t, roomID, stateMsg.RoomID)

	users, ok := stateMsg.Payload["users"].([]interface{})
	require.True(t, ok, "Payload must contain users list")
	assert.Len(t, users, 1, "Initial room state must have 1 user")

	// 3. Connect Client 2 via WebSocket
	q2 := u.Query()
	q2.Set("roomId", roomID)
	q2.Set("userId", "peer2")
	u.RawQuery = q2.Encode()

	ws2, resp2, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer ws2.Close()
	defer resp2.Body.Close()

	// Client 1 should receive Join notification for Client 2
	_ = ws1.SetReadDeadline(time.Now().Add(2 * time.Second))
	var joinMsg ountsu.OuntsuMessage
	err = ws1.ReadJSON(&joinMsg)
	require.NoError(t, err)
	assert.Equal(t, ountsu.TypeJoin, joinMsg.Type)
	assert.Equal(t, "peer2", joinMsg.SenderID)

	// Client 2 should receive RoomState with 2 users
	_ = ws2.SetReadDeadline(time.Now().Add(2 * time.Second))
	var c2StateMsg ountsu.OuntsuMessage
	err = ws2.ReadJSON(&c2StateMsg)
	require.NoError(t, err)
	assert.Equal(t, ountsu.TypeRoomState, c2StateMsg.Type)
	c2Users, ok := c2StateMsg.Payload["users"].([]interface{})
	require.True(t, ok)
	assert.Len(t, c2Users, 2, "Room state for peer2 must contain both users")

	// 4. Test Chat Message Exchange
	chatMsg := ountsu.OuntsuMessage{
		Type: ountsu.TypeChat,
		Payload: map[string]interface{}{
			"text": "Hello, watch party!",
		},
	}
	err = ws1.WriteJSON(chatMsg)
	require.NoError(t, err)

	_ = ws2.SetReadDeadline(time.Now().Add(2 * time.Second))
	var receivedChat ountsu.OuntsuMessage
	err = ws2.ReadJSON(&receivedChat)
	require.NoError(t, err)
	assert.Equal(t, ountsu.TypeChat, receivedChat.Type)
	assert.Equal(t, "peer1", receivedChat.SenderID)
	assert.Equal(t, "Hello, watch party!", receivedChat.Payload["text"])

	// 5. Test WebRTC Signaling: Offer & Answer
	offerMsg := ountsu.OuntsuMessage{
		Type: ountsu.TypeOffer,
		Payload: map[string]interface{}{
			"sdp": "v=0\r\no=- 12345 2 IN IP4 127.0.0.1...",
		},
	}
	err = ws1.WriteJSON(offerMsg)
	require.NoError(t, err)

	_ = ws2.SetReadDeadline(time.Now().Add(2 * time.Second))
	var receivedOffer ountsu.OuntsuMessage
	err = ws2.ReadJSON(&receivedOffer)
	require.NoError(t, err)
	assert.Equal(t, ountsu.TypeOffer, receivedOffer.Type)
	assert.Equal(t, "peer1", receivedOffer.SenderID)

	answerMsg := ountsu.OuntsuMessage{
		Type: ountsu.TypeAnswer,
		Payload: map[string]interface{}{
			"sdp": "v=0\r\no=- 54321 2 IN IP4 127.0.0.1...",
		},
	}
	err = ws2.WriteJSON(answerMsg)
	require.NoError(t, err)

	_ = ws1.SetReadDeadline(time.Now().Add(2 * time.Second))
	var receivedAnswer ountsu.OuntsuMessage
	err = ws1.ReadJSON(&receivedAnswer)
	require.NoError(t, err)
	assert.Equal(t, ountsu.TypeAnswer, receivedAnswer.Type)
	assert.Equal(t, "peer2", receivedAnswer.SenderID)

	// 6. Test ICE Candidate
	candMsg := ountsu.OuntsuMessage{
		Type: ountsu.TypeCandidate,
		Payload: map[string]interface{}{
			"candidate": "candidate:1 1 UDP 2130706431 192.168.1.1 50000 typ host",
		},
	}
	err = ws1.WriteJSON(candMsg)
	require.NoError(t, err)

	_ = ws2.SetReadDeadline(time.Now().Add(2 * time.Second))
	var receivedCand ountsu.OuntsuMessage
	err = ws2.ReadJSON(&receivedCand)
	require.NoError(t, err)
	assert.Equal(t, ountsu.TypeCandidate, receivedCand.Type)
	assert.Equal(t, "peer1", receivedCand.SenderID)

	// 7. Test State Update (Mute / Deafen)
	updateStateMsg := ountsu.OuntsuMessage{
		Type: ountsu.TypeUpdateState,
		Payload: map[string]interface{}{
			"muted":    true,
			"deafened": false,
		},
	}
	err = ws1.WriteJSON(updateStateMsg)
	require.NoError(t, err)

	// Both peer1 and peer2 receive state update
	_ = ws1.SetReadDeadline(time.Now().Add(2 * time.Second))
	var stateEcho1 ountsu.OuntsuMessage
	err = ws1.ReadJSON(&stateEcho1)
	require.NoError(t, err)
	assert.Equal(t, ountsu.TypeUpdateState, stateEcho1.Type)
	assert.Equal(t, true, stateEcho1.Payload["muted"])

	_ = ws2.SetReadDeadline(time.Now().Add(2 * time.Second))
	var stateEcho2 ountsu.OuntsuMessage
	err = ws2.ReadJSON(&stateEcho2)
	require.NoError(t, err)
	assert.Equal(t, ountsu.TypeUpdateState, stateEcho2.Type)
	assert.Equal(t, true, stateEcho2.Payload["muted"])

	// 8. Test Malformed & Unknown Messages (Resilience)
	// Invalid JSON string: should be ignored, connection stays alive
	err = ws1.WriteMessage(websocket.TextMessage, []byte(`{invalid-json`))
	require.NoError(t, err)

	// Unknown message type: should be ignored, connection stays alive
	err = ws1.WriteMessage(websocket.TextMessage, []byte(`{"type":"UNKNOWN_FUTURE_TYPE","payload":{}}`))
	require.NoError(t, err)

	// Send normal chat message right after malformed input: should succeed
	err = ws1.WriteJSON(ountsu.OuntsuMessage{
		Type:    ountsu.TypeChat,
		Payload: map[string]interface{}{"text": "Still alive after garbage data"},
	})
	require.NoError(t, err)

	_ = ws2.SetReadDeadline(time.Now().Add(2 * time.Second))
	var postGarbageChat ountsu.OuntsuMessage
	err = ws2.ReadJSON(&postGarbageChat)
	require.NoError(t, err)
	assert.Equal(t, "Still alive after garbage data", postGarbageChat.Payload["text"])

	// 9. Client 2 Disconnects -> Client 1 Receives Leave Notification
	ws2.Close()

	_ = ws1.SetReadDeadline(time.Now().Add(2 * time.Second))
	var leaveMsg ountsu.OuntsuMessage
	err = ws1.ReadJSON(&leaveMsg)
	require.NoError(t, err)
	assert.Equal(t, ountsu.TypeLeave, leaveMsg.Type)
	assert.Equal(t, "peer2", leaveMsg.SenderID)
}

func TestOuntsu_HighConcurrency_InviteAndRooms(t *testing.T) {
	ts, _ := setupTestServer(t)

	const numRoutines = 20
	const keysPerRoutine = 50
	var wg sync.WaitGroup
	wg.Add(numRoutines)

	keys := make(chan string, numRoutines*keysPerRoutine)

	for i := 0; i < numRoutines; i++ {
		go func() {
			defer wg.Done()
			for k := 0; k < keysPerRoutine; k++ {
				resp, err := http.Post(ts.URL+"/api/v1/ountsu/invite", "application/json", nil)
				if err != nil {
					t.Errorf("Invite request error: %v", err)
					return
				}
				var data map[string]string
				_ = json.NewDecoder(resp.Body).Decode(&data)
				_ = resp.Body.Close()
				keys <- data["inviteKey"]
			}
		}()
	}

	wg.Wait()
	close(keys)

	seen := make(map[string]bool)
	for key := range keys {
		assert.Len(t, key, 8)
		assert.False(t, seen[key], "Invite keys must be unique: collision detected on "+key)
		seen[key] = true
	}
	assert.Equal(t, numRoutines*keysPerRoutine, len(seen), "All 1,000 generated invite keys must be unique")
}

func TestOuntsu_Concurrent_RoomBroadcast(t *testing.T) {
	hub := ountsu.NewHub()
	room := hub.GetOrCreateRoom("stress-room")

	const clientCount = 30
	var clients []*ountsu.Client
	for i := 0; i < clientCount; i++ {
		c := ountsu.NewClient(strings.Repeat("u", 4)+string(rune('A'+i)), "stress-room", nil, room, hub)
		room.AddClient(c)
		clients = append(clients, c)
	}

	state := room.GetStateMessage()
	assert.Equal(t, ountsu.TypeRoomState, state.Type)
	users := state.Payload["users"].([]map[string]interface{})
	assert.Len(t, users, clientCount)

	// Concurrently broadcast and read state
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			defer wg.Done()
			for m := 0; m < 50; m++ {
				msg := &ountsu.OuntsuMessage{
					Type:     ountsu.TypeChat,
					SenderID: clients[idx].ID,
					RoomID:   "stress-room",
					Payload:  map[string]interface{}{"num": m},
				}
				room.Broadcast(msg, clients[idx].ID)
				_ = room.GetStateMessage()
			}
		}(i)
	}
	wg.Wait()

	// Clean up
	for _, c := range clients {
		room.RemoveClient(c.ID)
	}
	assert.Empty(t, room.Clients)
}
