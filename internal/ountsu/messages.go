package ountsu

import "encoding/json"

type MessageType string

const (
	TypeJoin        MessageType = "join"
	TypeLeave       MessageType = "leave"
	TypeChat        MessageType = "chat"
	TypeOffer       MessageType = "offer"
	TypeAnswer      MessageType = "answer"
	TypeCandidate   MessageType = "candidate"
	TypeUpdateState MessageType = "update_state"
	TypeRoomState   MessageType = "room_state"
)

type OuntsuMessage struct {
	Type     MessageType            `json:"type"`
	SenderID string                 `json:"senderId"`
	RoomID   string                 `json:"roomId"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
}

func (m *OuntsuMessage) Marshal() ([]byte, error) {
	return json.Marshal(m)
}
