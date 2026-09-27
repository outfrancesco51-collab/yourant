package party

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrRoomCapacityReached = errors.New("room capacity limit reached (max 100 members)")
	ErrRoomNotFound        = errors.New("watch party room not found")
	ErrInvalidRoomCode     = errors.New("invalid room code format: must match ^WP-[A-Z0-9]{4}$")
)

const MaxRoomCapacity = 100

// Room manages members and authoritative playback state for a session.
type Room struct {
	ID            string
	HostID        string
	HostName      string
	MediaID       int
	EpisodeNumber int
	MediaTitle    string
	IsPlaying     bool
	CurrentTime   float64
	PlaybackRate  float64
	LastSyncTime  time.Time
	CreatedAt     time.Time

	clients     map[string]*Client
	clientOrder []string // User IDs in order of arrival (oldest first)
	mu          sync.RWMutex
	hub         *Hub
}

// NewRoom creates a new room with default state.
func NewRoom(id, hostID, hostName string, mediaID int, episode int, title string, hub *Hub) *Room {
	return &Room{
		ID:            id,
		HostID:        hostID,
		HostName:      hostName,
		MediaID:       mediaID,
		EpisodeNumber: episode,
		MediaTitle:    title,
		PlaybackRate:  1.0,
		CurrentTime:   0.0,
		IsPlaying:     false,
		LastSyncTime:  time.Now(),
		CreatedAt:     time.Now(),
		clients:       make(map[string]*Client),
		clientOrder:   make([]string, 0),
		hub:           hub,
	}
}

// AddClient adds a client to the room, handling capacity and host assignment.
func (r *Room) AddClient(client *Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if existing connection from same user ID -> replace
	if existing, found := r.clients[client.ID]; found {
		go existing.Close()
		delete(r.clients, client.ID)
	}

	if len(r.clients) >= MaxRoomCapacity {
		return ErrRoomCapacityReached
	}

	// Assign host if room has no host or client is designated host
	if r.HostID == "" || r.HostID == client.ID || (client.IsHost && len(r.clients) == 0) {
		r.HostID = client.ID
		r.HostName = client.Name
		client.IsHost = true
	} else if client.ID != r.HostID {
		client.IsHost = false
	}

	r.clients[client.ID] = client

	// Track order for oldest-client host migration
	foundInOrder := false
	for _, id := range r.clientOrder {
		if id == client.ID {
			foundInOrder = true
			break
		}
	}
	if !foundInOrder {
		r.clientOrder = append(r.clientOrder, client.ID)
	}

	members := r.getMemberListLocked()

	// 1. Send initial state and room:joined confirmation to the joining client
	joinedMsg := NewRoomJoinedMessage(r.ID, client.ID, client.Name, client.IsHost, map[string]interface{}{
		"roomId":        r.ID,
		"hostId":        r.HostID,
		"hostName":      r.HostName,
		"mediaId":       r.MediaID,
		"episodeNumber": r.EpisodeNumber,
		"mediaTitle":    r.MediaTitle,
		"isPlaying":     r.IsPlaying,
		"currentTime":   r.CurrentTime,
		"playbackRate":  r.PlaybackRate,
		"memberCount":   len(r.clients),
		"members":       members,
	})
	client.SendMessage(joinedMsg)

	// 2. Broadcast room:user_joined event to all OTHER members
	userJoinedMsg := NewUserJoinedMessage(r.ID, MemberInfo{
		ID:       client.ID,
		Name:     client.Name,
		IsHost:   client.IsHost,
		JoinedAt: client.JoinedAt.UnixMilli(),
	}, len(r.clients), members)

	r.broadcastLocked(userJoinedMsg, client.ID)

	return nil
}

// RemoveClient removes a client and migrates host to the oldest remaining member.
func (r *Room) RemoveClient(userID string) (*Client, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	client, exists := r.clients[userID]
	if !exists {
		return nil, len(r.clients) == 0
	}

	delete(r.clients, userID)

	// Remove from clientOrder
	newOrder := make([]string, 0, len(r.clientOrder))
	for _, id := range r.clientOrder {
		if id != userID {
			newOrder = append(newOrder, id)
		}
	}
	r.clientOrder = newOrder

	var newHost *Client
	// If the departing client was the host, migrate to oldest remaining client
	if r.HostID == userID && len(r.clients) > 0 {
		for _, nextID := range r.clientOrder {
			if nextClient, ok := r.clients[nextID]; ok {
				nextClient.IsHost = true
				r.HostID = nextClient.ID
				r.HostName = nextClient.Name
				newHost = nextClient
				break
			}
		}
	}

	members := r.getMemberListLocked()

	// Broadcast user departure
	var newHostID, newHostName string
	if newHost != nil {
		newHostID = newHost.ID
		newHostName = newHost.Name
	}
	leftMsg := NewUserLeftMessage(r.ID, client.ID, client.Name, newHostID, newHostName, len(r.clients), members)
	r.broadcastLocked(leftMsg, "")

	isEmpty := len(r.clients) == 0
	return newHost, isEmpty
}

// UpdateState updates the authoritative playback state from host.
func (r *Room) UpdateState(isPlaying bool, currentTime float64, playbackRate float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.IsPlaying = isPlaying
	r.CurrentTime = currentTime
	if playbackRate > 0 {
		r.PlaybackRate = playbackRate
	}
	r.LastSyncTime = time.Now()
}

// Broadcast sends a message to all connected room members, optionally skipping excludeID.
func (r *Room) Broadcast(msg *WatchPartyMessage, excludeID string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.broadcastLocked(msg, excludeID)
}

func (r *Room) broadcastLocked(msg *WatchPartyMessage, excludeID string) {
	for id, client := range r.clients {
		if id != excludeID {
			client.SendMessage(msg)
		}
	}
}

// GetMemberList returns an ordered slice of current room members.
func (r *Room) GetMemberList() []MemberInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getMemberListLocked()
}

func (r *Room) getMemberListLocked() []MemberInfo {
	list := make([]MemberInfo, 0, len(r.clients))
	for _, id := range r.clientOrder {
		if c, ok := r.clients[id]; ok {
			list = append(list, MemberInfo{
				ID:       c.ID,
				Name:     c.Name,
				IsHost:   c.IsHost,
				JoinedAt: c.JoinedAt.UnixMilli(),
			})
		}
	}
	return list
}

// GetStateMessage constructs a sync:state message with current playback coordinates.
func (r *Room) GetStateMessage(senderID, senderName string) *WatchPartyMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return &WatchPartyMessage{
		Type:       TypeSyncState,
		RoomID:     r.ID,
		SenderID:   senderID,
		SenderName: senderName,
		IsHost:     senderID == r.HostID,
		ServerTime: time.Now().UnixMilli(),
		Payload: map[string]interface{}{
			"roomId":        r.ID,
			"hostId":        r.HostID,
			"hostName":      r.HostName,
			"mediaId":       r.MediaID,
			"episodeNumber": r.EpisodeNumber,
			"mediaTitle":    r.MediaTitle,
			"isPlaying":     r.IsPlaying,
			"timestamp":     r.CurrentTime,
			"playbackRate":  r.PlaybackRate,
			"memberCount":   len(r.clients),
			"members":       r.getMemberListLocked(),
		},
	}
}

// MemberCount returns the number of active room members.
func (r *Room) MemberCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}
