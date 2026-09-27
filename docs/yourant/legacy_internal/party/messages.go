package party

import (
	"encoding/json"
	"time"
)

// Message types conforming to the WatchParty protocol specification.
const (
	TypeRoomJoin    = "room:join"
	TypeRoomJoined  = "room:joined"
	TypeUserJoined  = "room:user_joined"
	TypeUserLeft    = "room:user_left"
	TypeHostPlay    = "host:play"
	TypeHostPause   = "host:pause"
	TypeHostSeek    = "host:seek"
	TypeHostBeat    = "host:heartbeat"
	TypeSyncState   = "sync:state"
	TypeChatMessage = "chat:message"
	TypePing        = "ping"
	TypePong        = "pong"
)

// MemberInfo describes an active room member.
type MemberInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsHost   bool   `json:"isHost"`
	JoinedAt int64  `json:"joinedAt"` // Unix ms
}

// WatchPartyMessage represents the standard JSON message frame.
type WatchPartyMessage struct {
	Type       string                 `json:"type"`
	RoomID     string                 `json:"roomId"`
	SenderID   string                 `json:"senderId"`
	SenderName string                 `json:"senderName"`
	IsHost     bool                   `json:"isHost"`
	ClientTime int64                  `json:"clientTime,omitempty"`
	ServerTime int64                  `json:"serverTime"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

// Marshal encodes message to JSON bytes.
func (m *WatchPartyMessage) Marshal() ([]byte, error) {
	if m.ServerTime == 0 {
		m.ServerTime = time.Now().UnixMilli()
	}
	return json.Marshal(m)
}

// NewPong creates a pong response to a client ping.
func NewPong(roomID, senderID string, clientTime int64) *WatchPartyMessage {
	return &WatchPartyMessage{
		Type:       TypePong,
		RoomID:     roomID,
		SenderID:   "server",
		SenderName: "Yourant Server",
		IsHost:     false,
		ClientTime: clientTime,
		ServerTime: time.Now().UnixMilli(),
		Payload:    map[string]interface{}{},
	}
}

// NewUserJoinedMessage broadcasts that a member joined the room.
func NewUserJoinedMessage(roomID string, user MemberInfo, count int, members []MemberInfo) *WatchPartyMessage {
	return &WatchPartyMessage{
		Type:       TypeUserJoined,
		RoomID:     roomID,
		SenderID:   user.ID,
		SenderName: user.Name,
		IsHost:     user.IsHost,
		ServerTime: time.Now().UnixMilli(),
		Payload: map[string]interface{}{
			"userId":      user.ID,
			"userName":    user.Name,
			"isHost":      user.IsHost,
			"memberCount": count,
			"members":     members,
		},
	}
}

// NewUserLeftMessage broadcasts that a member departed.
func NewUserLeftMessage(roomID, userID, userName string, newHostID, newHostName string, count int, members []MemberInfo) *WatchPartyMessage {
	return &WatchPartyMessage{
		Type:       TypeUserLeft,
		RoomID:     roomID,
		SenderID:   userID,
		SenderName: userName,
		IsHost:     false,
		ServerTime: time.Now().UnixMilli(),
		Payload: map[string]interface{}{
			"userId":      userID,
			"userName":    userName,
			"newHostId":   newHostID,
			"newHostName": newHostName,
			"memberCount": count,
			"members":     members,
		},
	}
}

// NewRoomJoinedMessage sends initial state to the newly joined client.
func NewRoomJoinedMessage(roomID, clientID, clientName string, isHost bool, state map[string]interface{}) *WatchPartyMessage {
	return &WatchPartyMessage{
		Type:       TypeRoomJoined,
		RoomID:     roomID,
		SenderID:   clientID,
		SenderName: clientName,
		IsHost:     isHost,
		ServerTime: time.Now().UnixMilli(),
		Payload:    state,
	}
}
