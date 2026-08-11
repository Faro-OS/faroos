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
	TypeP2PAnswer     MessageType = "p2p_answer"     // server -> agent, during connection setup

	// Terminal messages implement a persistent PTY session multiplexed over
	// the single agent websocket by SessionID — unlike Command/CommandResult
	// these are streaming, not request/response.
	TypeTerminalOpen   MessageType = "terminal_open"   // server -> agent
	TypeTerminalInput  MessageType = "terminal_input"  // server -> agent
	TypeTerminalResize MessageType = "terminal_resize" // server -> agent
	TypeTerminalOutput MessageType = "terminal_output" // agent -> server
	TypeTerminalClose  MessageType = "terminal_close"  // either direction
)

// Envelope wraps every message sent over the websocket. Payload is decoded
// based on Type.
type Envelope struct {
	Type          MessageType    `json:"type"`
	Hello         *Hello         `json:"hello,omitempty"`
	Stats         *model.Stats   `json:"stats,omitempty"`
	Command       *Command       `json:"command,omitempty"`
	CommandResult *CommandResult `json:"commandResult,omitempty"`
	P2PAnswer     *P2PAnswer     `json:"p2pAnswer,omitempty"`

	TerminalOpen   *TerminalOpen   `json:"terminalOpen,omitempty"`
	TerminalData   *TerminalData   `json:"terminalData,omitempty"`
	TerminalResize *TerminalResize `json:"terminalResize,omitempty"`
	TerminalClose  *TerminalClose  `json:"terminalClose,omitempty"`
}

// TerminalOpen asks the agent to spawn a new shell PTY.
type TerminalOpen struct {
	SessionID string `json:"sessionId"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

// TerminalData carries a chunk of terminal bytes in either direction
// (keystrokes going to the agent, output coming back), base64-encoded so
// arbitrary bytes survive JSON.
type TerminalData struct {
	SessionID string `json:"sessionId"`
	DataB64   string `json:"dataB64"`
}

type TerminalResize struct {
	SessionID string `json:"sessionId"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

type TerminalClose struct {
	SessionID string `json:"sessionId"`
	Reason    string `json:"reason,omitempty"`
}

// Hello is sent once by the agent right after connecting, identifying
// itself and its pairing credentials.
type Hello struct {
	NodeID   string `json:"nodeId"`
	Token    string `json:"token"`
	Version  string `json:"version"`
	P2POffer string `json:"p2pOffer,omitempty"`
}

// P2PAnswer completes the authenticated WebRTC negotiation. The SDP contains
// DTLS fingerprints and ICE candidates, but never the node pairing token.
type P2PAnswer struct {
	SDP   string `json:"sdp,omitempty"`
	Error string `json:"error,omitempty"`
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
