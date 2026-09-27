package ountsu

import (
	"sync"
)

type Room struct {
	ID      string
	Clients map[string]*Client
	mu      sync.Mutex
	hub     *Hub
}

func NewRoom(id string, hub *Hub) *Room {
	return &Room{
		ID:      id,
		Clients: make(map[string]*Client),
		hub:     hub,
	}
}

func (r *Room) AddClient(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Clients[c.ID] = c
}

func (r *Room) RemoveClient(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Clients, id)
}

func (r *Room) Broadcast(msg *OuntsuMessage, excludeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, client := range r.Clients {
		if id == excludeID {
			continue
		}
		client.SendMessage(msg)
	}
}

func (r *Room) GetStateMessage() *OuntsuMessage {
	r.mu.Lock()
	defer r.mu.Unlock()

	users := make([]map[string]interface{}, 0, len(r.Clients))
	for _, c := range r.Clients {
		users = append(users, map[string]interface{}{
			"id":       c.ID,
			"muted":    c.Muted,
			"deafened": c.Deafened,
		})
	}

	return &OuntsuMessage{
		Type:   TypeRoomState,
		RoomID: r.ID,
		Payload: map[string]interface{}{
			"users": users,
		},
	}
}
