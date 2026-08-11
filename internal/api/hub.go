package api

import (
	"errors"
	"sync"
	"time"

	"github.com/faroos/faroos/internal/proto"
)

var errNodeNotConnected = errors.New("node is not currently connected")
var errCommandTimeout = errors.New("command timed out waiting for agent response")

const commandTimeout = 15 * time.Second

// hub tracks live agent websocket connections by node ID so HTTP handlers
// can send them commands (e.g. "list containers") and wait for the reply,
// turning the async websocket protocol into a synchronous-looking call for
// the REST API.
type hub struct {
	mu    sync.Mutex
	conns map[string]*agentConn
}

type agentConnection interface {
	ReadJSON(any) error
	WriteJSON(any) error
	SetReadDeadline(time.Time) error
	Close() error
}

func newHub() *hub {
	return &hub{conns: make(map[string]*agentConn)}
}

type agentConn struct {
	conn    agentConnection
	mode    string
	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan proto.CommandResult

	// streams carries terminal_output/terminal_close envelopes for
	// long-lived PTY sessions, keyed by session ID — unlike pending
	// (request/response), a stream stays open across many messages.
	streamsMu sync.Mutex
	streams   map[string]chan proto.Envelope
}

func (h *hub) register(nodeID string, conn agentConnection, mode string) *agentConn {
	ac := &agentConn{
		conn:    conn,
		mode:    mode,
		pending: make(map[string]chan proto.CommandResult),
		streams: make(map[string]chan proto.Envelope),
	}
	h.mu.Lock()
	previous := h.conns[nodeID]
	h.conns[nodeID] = ac
	h.mu.Unlock()
	if previous != nil {
		_ = previous.conn.Close()
	}
	return ac
}

func (h *hub) unregister(nodeID string, ac *agentConn) bool {
	h.mu.Lock()
	removed := false
	if h.conns[nodeID] == ac {
		delete(h.conns, nodeID)
		removed = true
	}
	h.mu.Unlock()

	ac.pendingMu.Lock()
	for id, ch := range ac.pending {
		close(ch)
		delete(ac.pending, id)
	}
	ac.pendingMu.Unlock()

	ac.streamsMu.Lock()
	for id, ch := range ac.streams {
		close(ch)
		delete(ac.streams, id)
	}
	ac.streamsMu.Unlock()
	return removed
}

func (h *hub) get(nodeID string) (*agentConn, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ac, ok := h.conns[nodeID]
	return ac, ok
}

// send writes a command to the agent and blocks until the matching
// command_result arrives (matched by command ID) or the default timeout
// elapses.
func (ac *agentConn) send(cmd proto.Command) (proto.CommandResult, error) {
	return ac.sendWithTimeout(cmd, commandTimeout)
}

// sendWithTimeout is send with a caller-chosen timeout, for commands that
// are expected to take longer than the default (e.g. deploying an app,
// which may need to pull a large image first).
func (ac *agentConn) sendWithTimeout(cmd proto.Command, timeout time.Duration) (proto.CommandResult, error) {
	resultCh := make(chan proto.CommandResult, 1)

	ac.pendingMu.Lock()
	ac.pending[cmd.ID] = resultCh
	ac.pendingMu.Unlock()

	defer func() {
		ac.pendingMu.Lock()
		delete(ac.pending, cmd.ID)
		ac.pendingMu.Unlock()
	}()

	ac.writeMu.Lock()
	err := ac.conn.WriteJSON(proto.Envelope{Type: proto.TypeCommand, Command: &cmd})
	ac.writeMu.Unlock()
	if err != nil {
		return proto.CommandResult{}, err
	}

	select {
	case result, ok := <-resultCh:
		if !ok {
			return proto.CommandResult{}, errNodeNotConnected
		}
		return result, nil
	case <-time.After(timeout):
		return proto.CommandResult{}, errCommandTimeout
	}
}

// resolve delivers a command_result to whoever is waiting on it.
func (ac *agentConn) resolve(result proto.CommandResult) {
	ac.pendingMu.Lock()
	ch, ok := ac.pending[result.ID]
	ac.pendingMu.Unlock()
	if ok {
		ch <- result
	}
}

// writeEnvelope sends a raw envelope to the agent without expecting a
// correlated reply — used for one-way terminal control messages.
func (ac *agentConn) writeEnvelope(env proto.Envelope) error {
	ac.writeMu.Lock()
	defer ac.writeMu.Unlock()
	return ac.conn.WriteJSON(env)
}

// openStream registers a channel that will receive every terminal_output /
// terminal_close envelope for the given session ID until closeStream is
// called (or the agent disconnects).
func (ac *agentConn) openStream(sessionID string) chan proto.Envelope {
	ch := make(chan proto.Envelope, 32)
	ac.streamsMu.Lock()
	ac.streams[sessionID] = ch
	ac.streamsMu.Unlock()
	return ch
}

func (ac *agentConn) closeStream(sessionID string) {
	ac.streamsMu.Lock()
	if ch, ok := ac.streams[sessionID]; ok {
		close(ch)
		delete(ac.streams, sessionID)
	}
	ac.streamsMu.Unlock()
}

// dispatchStream delivers a terminal envelope to its session's channel, if
// still open. Non-blocking: a slow/gone reader shouldn't stall the agent's
// single read loop. Guards against the inherent race between looking up
// the channel and closeStream closing it concurrently — a send on a
// closed channel would otherwise panic and take down the whole process,
// since this runs in the connection's read-loop goroutine.
func (ac *agentConn) dispatchStream(sessionID string, env proto.Envelope) {
	ac.streamsMu.Lock()
	ch, ok := ac.streams[sessionID]
	ac.streamsMu.Unlock()
	if !ok {
		return
	}
	defer func() { recover() }()
	select {
	case ch <- env:
	default:
	}
}
