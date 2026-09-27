package party

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 64 * 1024 // 64 KB
)

// Client represents a connected user in a watch party room.
type Client struct {
	ID       string
	Name     string
	RoomID   string
	IsHost   bool
	JoinedAt time.Time

	conn   *websocket.Conn
	send   chan []byte
	room   *Room
	hub    *Hub
	closed bool
	mu     sync.Mutex
}

// NewClient initializes a party client.
func NewClient(id, name, roomID string, isHost bool, conn *websocket.Conn, room *Room, hub *Hub) *Client {
	return &Client{
		ID:       id,
		Name:     name,
		RoomID:   roomID,
		IsHost:   isHost,
		JoinedAt: time.Now(),
		conn:     conn,
		send:     make(chan []byte, 256),
		room:     room,
		hub:      hub,
	}
}

// SendMessage enqueues a message for transmission.
func (c *Client) SendMessage(msg *WatchPartyMessage) {
	data, err := msg.Marshal()
	if err != nil {
		log.Printf("[PARTY][CLIENT] Failed to marshal message: %v", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}

	select {
	case c.send <- data:
	default:
		log.Printf("[PARTY][CLIENT] Send buffer full for client %s, dropping connection", c.ID)
		c.closeInternal()
	}
}

// Close closes the client connection safely.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeInternal()
}

func (c *Client) closeInternal() {
	if c.closed {
		return
	}
	c.closed = true
	close(c.send)
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

// ReadPump reads incoming WebSocket messages and routes them.
func (c *Client) ReadPump() {
	defer func() {
		c.room.RemoveClient(c.ID)
		c.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[PARTY][WS] Unexpected close from %s: %v", c.ID, err)
			}
			break
		}

		var msg WatchPartyMessage
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			log.Printf("[PARTY][WS] Malformed JSON from %s: %v", c.ID, err)
			continue
		}

		// Ensure sender metadata is authentic
		msg.RoomID = c.RoomID
		msg.SenderID = c.ID
		msg.SenderName = c.Name
		msg.IsHost = c.IsHost
		msg.ServerTime = time.Now().UnixMilli()

		c.handleIncomingMessage(&msg)
	}
}

// WritePump pushes buffered messages to the WebSocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				return
			}

			// Flush any queued messages into current frame
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleIncomingMessage processes parsed client protocol messages.
func (c *Client) handleIncomingMessage(msg *WatchPartyMessage) {
	switch msg.Type {
	case TypePing:
		// Return pong echoing clientTime with authoritative serverTime
		pong := NewPong(c.RoomID, c.ID, msg.ClientTime)
		c.SendMessage(pong)

	case TypeHostPlay, TypeHostPause, TypeHostSeek, TypeHostBeat:
		// Only host is authoritative for playback commands
		if !c.IsHost {
			log.Printf("[PARTY] Non-host client %s attempted host command %s", c.ID, msg.Type)
			return
		}

		// Extract playback state from payload if present
		if msg.Payload != nil {
			var isPlaying bool
			var currentTime float64
			var playbackRate float64 = 1.0

			if val, ok := msg.Payload["isPlaying"].(bool); ok {
				isPlaying = val
			} else if msg.Type == TypeHostPlay {
				isPlaying = true
			} else if msg.Type == TypeHostPause {
				isPlaying = false
			}

			if val, ok := msg.Payload["timestamp"].(float64); ok {
				currentTime = val
			}
			if val, ok := msg.Payload["playbackRate"].(float64); ok && val > 0 {
				playbackRate = val
			}

			c.room.UpdateState(isPlaying, currentTime, playbackRate)
		}

		// Broadcast host event to all members in the room
		c.room.Broadcast(msg, "")

	case TypeChatMessage:
		if msg.Payload == nil {
			return
		}
		chatText, _ := msg.Payload["chatText"].(string)
		if len(chatText) == 0 {
			return
		}
		// Truncate overly long chat messages to 500 chars
		if len(chatText) > 500 {
			chatText = chatText[:500]
			msg.Payload["chatText"] = chatText
		}
		c.room.Broadcast(msg, "")

	case TypeSyncState:
		// Client requested current authoritative room state
		stateMsg := c.room.GetStateMessage(c.ID, c.Name)
		c.SendMessage(stateMsg)

	default:
		log.Printf("[PARTY] Unknown message type %s from client %s", msg.Type, c.ID)
	}
}
