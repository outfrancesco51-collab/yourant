package party

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"regexp"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	roomCodeRegex = regexp.MustCompile(`^WP-[A-Z0-9]{4}$`)
	upgrader      = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow cross-origin WebSocket requests
		},
	}
)

// Hub coordinates all watch party rooms and WebSocket client connections.
type Hub struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

// NewHub initializes a watch party hub.
func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]*Room),
	}
}

// ValidateRoomCode checks if the room code matches ^WP-[A-Z0-9]{4}$.
func (h *Hub) ValidateRoomCode(code string) bool {
	return roomCodeRegex.MatchString(code)
}

// GenerateRoomCode generates a cryptographically random room code matching ^WP-[A-Z0-9]{4}$.
func (h *Hub) GenerateRoomCode() string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 4)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			b[i] = charset[i%len(charset)]
		} else {
			b[i] = charset[num.Int64()]
		}
	}
	return fmt.Sprintf("WP-%s", string(b))
}

// CreateRoom registers a new room with a validated code.
func (h *Hub) CreateRoom(roomID, hostID, hostName string, mediaID int, episode int, title string) (*Room, error) {
	if !h.ValidateRoomCode(roomID) {
		return nil, ErrInvalidRoomCode
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, exists := h.rooms[roomID]; exists {
		return existing, nil
	}

	room := NewRoom(roomID, hostID, hostName, mediaID, episode, title, h)
	h.rooms[roomID] = room
	return room, nil
}

// GetRoom retrieves an existing room by ID.
func (h *Hub) GetRoom(roomID string) (*Room, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	room, exists := h.rooms[roomID]
	return room, exists
}

// GetOrCreateRoom retrieves an existing room or creates a new one.
func (h *Hub) GetOrCreateRoom(roomID, hostID, hostName string, mediaID int, episode int, title string) (*Room, error) {
	if !h.ValidateRoomCode(roomID) {
		return nil, ErrInvalidRoomCode
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if room, exists := h.rooms[roomID]; exists {
		return room, nil
	}

	room := NewRoom(roomID, hostID, hostName, mediaID, episode, title, h)
	h.rooms[roomID] = room
	return room, nil
}

// DeleteRoom removes a room from the hub.
func (h *Hub) DeleteRoom(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms, roomID)
}

// HandleWebSocket upgrades the HTTP connection and joins the client to the specified room.
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request, roomID, userID, userName string, isHost bool) error {
	if !h.ValidateRoomCode(roomID) {
		http.Error(w, ErrInvalidRoomCode.Error(), http.StatusBadRequest)
		return ErrInvalidRoomCode
	}

	room, err := h.GetOrCreateRoom(roomID, userID, userName, 0, 0, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[PARTY][HUB] WebSocket upgrade error: %v", err)
		return err
	}

	client := NewClient(userID, userName, roomID, isHost, conn, room, h)
	if err := room.AddClient(client); err != nil {
		log.Printf("[PARTY][HUB] Failed to add client %s to room %s: %v", userID, roomID, err)
		_ = conn.WriteJSON(map[string]string{"error": err.Error()})
		_ = conn.Close()
		return err
	}

	go client.WritePump()
	go client.ReadPump()

	return nil
}
