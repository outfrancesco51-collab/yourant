package ountsu

import (
	"testing"
)

func TestHub_GetOrCreateRoom(t *testing.T) {
	hub := NewHub()
	
	room1 := hub.GetOrCreateRoom("roomA")
	if room1.ID != "roomA" {
		t.Errorf("Expected roomA, got %s", room1.ID)
	}

	room2 := hub.GetOrCreateRoom("roomA")
	if room1 != room2 {
		t.Errorf("Expected same room instance for same ID")
	}
}

func TestHub_GenerateInviteKey(t *testing.T) {
	hub := NewHub()
	
	key := hub.GenerateInviteKey()
	if len(key) != 8 {
		t.Errorf("Expected 8 character key, got %d", len(key))
	}

	room := hub.GetOrCreateRoom(key)
	if room.ID != key {
		t.Errorf("Expected room to be pre-created with key ID")
	}
}

func TestRoom_AddRemoveClient(t *testing.T) {
	hub := NewHub()
	room := hub.GetOrCreateRoom("testRoom")
	
	client := &Client{ID: "client1"}
	room.AddClient(client)
	
	if len(room.Clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(room.Clients))
	}

	room.RemoveClient("client1")
	if len(room.Clients) != 0 {
		t.Errorf("Expected 0 clients, got %d", len(room.Clients))
	}
}
