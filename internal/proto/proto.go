// Package proto defines the JSON message envelope exchanged over the
// agent<->server websocket connection.
package proto

import (
	"encoding/json"

	"github.com/faroos/faroos/internal/model"
)

type MessageType string

const (
	TypeHello         MessageType = "hello"
	TypeStats         MessageType = "stats"
	TypePing          MessageType = "ping"
	TypePong          MessageType = "pong"
	TypeCommand       MessageType = "command"        // server -> agent
	TypeCommandResult MessageType = "command_result" // agent -> server
)

// Envelope wraps every message sent over the websocket. Payload is decoded
// based on Type.
type Envelope struct {
	Type          MessageType    `json:"type"`
	Hello         *Hello         `json:"hello,omitempty"`
	Stats         *model.Stats   `json:"stats,omitempty"`
	Command       *Command       `json:"command,omitempty"`
	CommandResult *CommandResult `json:"commandResult,omitempty"`
}

// Hello is sent once by the agent right after connecting, identifying
// itself and its pairing credentials.
type Hello struct {
	NodeID  string `json:"nodeId"`
	Token   string `json:"token"`
	Version string `json:"version"`
}

// Command is a request the server sends to a connected agent — e.g. "list
// containers on this node". Action names are dotted, grouped by subsystem
// ("containers.list", "containers.start", ...) so new subsystems (files,
// terminal, apps) can share the same envelope without a proto change.
type Command struct {
	ID     string          `json:"id"`
	Action string          `json:"action"`
	Params json.RawMessage `json:"params,omitempty"`
}

// CommandResult is the agent's reply to a Command, correlated by ID.
type CommandResult struct {
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Error  string          `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}
