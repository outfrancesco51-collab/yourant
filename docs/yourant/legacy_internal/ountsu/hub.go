package ountsu

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

type Hub struct {
	Rooms map[string]*Room
	mu    sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		Rooms: make(map[string]*Room),
	}
}

func (h *Hub) GetOrCreateRoom(id string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()

	if r, ok := h.Rooms[id]; ok {
		return r
	}

	r := NewRoom(id, h)
	h.Rooms[id] = r
	return r
}

func (h *Hub) GenerateInviteKey() string {
	b := make([]byte, 4) // 8 chars hex
	if _, err := rand.Read(b); err != nil {
		return "ERROR"
	}
	key := hex.EncodeToString(b)
	h.GetOrCreateRoom(key)
	return key
}
