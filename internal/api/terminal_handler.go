package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/faroos/faroos/internal/proto"
)

// clientMessage is what the browser sends over its terminal websocket —
// deliberately separate from the internal agent proto so the two can
// evolve independently.
type clientMessage struct {
	Type string `json:"type"` // "input" | "resize"
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// serverMessage is what the browser receives.
type serverMessage struct {
	Type string `json:"type"` // "output" | "closed"
	Data string `json:"data,omitempty"`
}

// handleTerminal bridges a browser websocket to a PTY session on the
// chosen node's agent, multiplexed over that agent's single connection to
// the panel by a per-tab session ID.
func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}

	cols, rows := 80, 24
	if v := r.URL.Query().Get("cols"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cols = n
		}
	}
	if v := r.URL.Query().Get("rows"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rows = n
		}
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("terminal: upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	sessionID := newCommandID()
	outCh := ac.openStream(sessionID)

	if err := ac.writeEnvelope(proto.Envelope{
		Type:         proto.TypeTerminalOpen,
		TerminalOpen: &proto.TerminalOpen{SessionID: sessionID, Cols: cols, Rows: rows},
	}); err != nil {
		ac.closeStream(sessionID)
		return
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			var msg clientMessage
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			switch msg.Type {
			case "input":
				ac.writeEnvelope(proto.Envelope{
					Type:         proto.TypeTerminalInput,
					TerminalData: &proto.TerminalData{SessionID: sessionID, DataB64: msg.Data},
				})
			case "resize":
				ac.writeEnvelope(proto.Envelope{
					Type:           proto.TypeTerminalResize,
					TerminalResize: &proto.TerminalResize{SessionID: sessionID, Cols: msg.Cols, Rows: msg.Rows},
				})
			}
		}
	}()

loop:
	for {
		select {
		case env, ok := <-outCh:
			if !ok {
				break loop
			}
			switch env.Type {
			case proto.TypeTerminalOutput:
				if env.TerminalData != nil {
					conn.WriteJSON(serverMessage{Type: "output", Data: env.TerminalData.DataB64})
				}
			case proto.TypeTerminalClose:
				conn.WriteJSON(serverMessage{Type: "closed"})
				break loop
			}
		case <-stopped:
			break loop
		}
	}

	ac.writeEnvelope(proto.Envelope{
		Type:          proto.TypeTerminalClose,
		TerminalClose: &proto.TerminalClose{SessionID: sessionID, Reason: "client disconnected"},
	})
	ac.closeStream(sessionID)
}
