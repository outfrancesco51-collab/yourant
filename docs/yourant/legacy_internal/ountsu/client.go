package ountsu

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

type Client struct {
	ID       string
	RoomID   string
	Muted    bool
	Deafened bool

	conn   *websocket.Conn
	send   chan []byte
	room   *Room
	hub    *Hub
	closed bool
	mu     sync.Mutex
}

func NewClient(id, roomID string, conn *websocket.Conn, room *Room, hub *Hub) *Client {
	return &Client{
		ID:     id,
		RoomID: roomID,
		conn:   conn,
		send:   make(chan []byte, 256),
		room:   room,
		hub:    hub,
	}
}

func (c *Client) SendMessage(msg *OuntsuMessage) {
	data, err := msg.Marshal()
	if err != nil {
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
		c.closeInternal()
	}
}

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

func (c *Client) ReadPump() {
	defer func() {
		c.room.RemoveClient(c.ID)
		
		// Broadcast leave message
		c.room.Broadcast(&OuntsuMessage{
			Type:     TypeLeave,
			SenderID: c.ID,
			RoomID:   c.RoomID,
		}, c.ID)

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
			break
		}

		var msg OuntsuMessage
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			continue
		}

		msg.RoomID = c.RoomID
		msg.SenderID = c.ID

		c.handleIncomingMessage(&msg)
	}
}

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
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

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

func (c *Client) handleIncomingMessage(msg *OuntsuMessage) {
	switch msg.Type {
	case TypeChat, TypeOffer, TypeAnswer, TypeCandidate:
		// Broadcast these signaling and chat messages to everyone else
		c.room.Broadcast(msg, c.ID)

	case TypeUpdateState:
		if msg.Payload != nil {
			if val, ok := msg.Payload["muted"].(bool); ok {
				c.Muted = val
			}
			if val, ok := msg.Payload["deafened"].(bool); ok {
				c.Deafened = val
			}
		}
		// Broadcast the state update to everyone
		c.room.Broadcast(msg, "")

	default:
		log.Printf("[OUNTSU] Unknown message type %s from client %s", msg.Type, c.ID)
	}
}
