package party

import (
	"fmt"
	"sync"
	"testing"
)

func TestRoomCodeValidationAndGeneration(t *testing.T) {
	hub := NewHub()

	validCodes := []string{"WP-7F3A", "WP-0000", "WP-ZZZZ", "WP-1A2B"}
	for _, code := range validCodes {
		if !hub.ValidateRoomCode(code) {
			t.Errorf("Expected valid code %s, got rejected", code)
		}
	}

	invalidCodes := []string{"WP-123", "WP-12345", "wp-7f3a", "ROOM-1234", "WP-!@#$"}
	for _, code := range invalidCodes {
		if hub.ValidateRoomCode(code) {
			t.Errorf("Expected invalid code %s to be rejected", code)
		}
	}

	for i := 0; i < 50; i++ {
		generated := hub.GenerateRoomCode()
		if !hub.ValidateRoomCode(generated) {
			t.Fatalf("Generated code %s does not match pattern ^WP-[A-Z0-9]{4}$", generated)
		}
	}
}

func TestRoomCreationAndHostAssignment(t *testing.T) {
	hub := NewHub()
	room, err := hub.CreateRoom("WP-7F3A", "user-1", "Francy", 154587, 1, "Frieren")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	if room.ID != "WP-7F3A" || room.HostID != "user-1" || room.HostName != "Francy" {
		t.Errorf("Room fields mismatch: %+v", room)
	}
}

func TestRoomHostMigrationToOldest(t *testing.T) {
	hub := NewHub()
	room, _ := hub.CreateRoom("WP-8888", "host-orig", "Host", 1, 1, "Test")

	cHost := &Client{ID: "host-orig", Name: "Host", IsHost: true, send: make(chan []byte, 10)}
	cGuest1 := &Client{ID: "guest-1", Name: "Guest One", IsHost: false, send: make(chan []byte, 10)}
	cGuest2 := &Client{ID: "guest-2", Name: "Guest Two", IsHost: false, send: make(chan []byte, 10)}

	_ = room.AddClient(cHost)
	_ = room.AddClient(cGuest1)
	_ = room.AddClient(cGuest2)

	if room.MemberCount() != 3 {
		t.Fatalf("Expected 3 members, got %d", room.MemberCount())
	}

	// Host departs
	newHost, isEmpty := room.RemoveClient("host-orig")
	if isEmpty {
		t.Fatalf("Expected room not to be empty")
	}
	if newHost == nil || newHost.ID != "guest-1" {
		t.Fatalf("Expected host to migrate to oldest remaining guest-1, got %v", newHost)
	}
	if room.HostID != "guest-1" || !cGuest1.IsHost {
		t.Errorf("HostID not updated correctly on room: %s", room.HostID)
	}
}

func TestRoomCapacityLimit(t *testing.T) {
	hub := NewHub()
	room, _ := hub.CreateRoom("WP-9999", "host", "Host", 1, 1, "Test")

	for i := 0; i < MaxRoomCapacity; i++ {
		c := &Client{ID: fmt.Sprintf("u-%d", i), Name: fmt.Sprintf("User %d", i), send: make(chan []byte, 10)}
		if err := room.AddClient(c); err != nil {
			t.Fatalf("Failed adding member %d within capacity: %v", i, err)
		}
	}

	// 101st client must be rejected
	cOver := &Client{ID: "u-overflow", Name: "Overflow", send: make(chan []byte, 10)}
	if err := room.AddClient(cOver); err != ErrRoomCapacityReached {
		t.Fatalf("Expected ErrRoomCapacityReached, got: %v", err)
	}
}

func TestConcurrentRoomCodeGeneration(t *testing.T) {
	hub := NewHub()
	var wg sync.WaitGroup
	codes := make(chan string, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- hub.GenerateRoomCode()
		}()
	}
	wg.Wait()
	close(codes)

	for code := range codes {
		if !hub.ValidateRoomCode(code) {
			t.Errorf("Invalid code generated concurrently: %s", code)
		}
	}
}
