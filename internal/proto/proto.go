// Package proto defines the JSON message envelope exchanged over the
// agent<->server websocket connection.
package proto

import "github.com/faroos/faroos/internal/model"

type MessageType string

const (
	TypeHello MessageType = "hello"
	TypeStats MessageType = "stats"
	TypePing  MessageType = "ping"
	TypePong  MessageType = "pong"
)

// Envelope wraps every message sent over the websocket. Payload is decoded
// based on Type.
type Envelope struct {
	Type  MessageType  `json:"type"`
	Hello *Hello       `json:"hello,omitempty"`
	Stats *model.Stats `json:"stats,omitempty"`
}

// Hello is sent once by the agent right after connecting, identifying
// itself and its pairing credentials.
type Hello struct {
	NodeID  string `json:"nodeId"`
	Token   string `json:"token"`
	Version string `json:"version"`
}
