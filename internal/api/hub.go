package api

import (
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"

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

func newHub() *hub {
	return &hub{conns: make(map[string]*agentConn)}
}

type agentConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan proto.CommandResult
}

func (h *hub) register(nodeID string, conn *websocket.Conn) *agentConn {
	ac := &agentConn{conn: conn, pending: make(map[string]chan proto.CommandResult)}
	h.mu.Lock()
	h.conns[nodeID] = ac
	h.mu.Unlock()
	return ac
}

func (h *hub) unregister(nodeID string, ac *agentConn) {
	h.mu.Lock()
	if h.conns[nodeID] == ac {
		delete(h.conns, nodeID)
	}
	h.mu.Unlock()

	ac.pendingMu.Lock()
	for id, ch := range ac.pending {
		close(ch)
		delete(ac.pending, id)
	}
	ac.pendingMu.Unlock()
}

func (h *hub) get(nodeID string) (*agentConn, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ac, ok := h.conns[nodeID]
	return ac, ok
}

// send writes a command to the agent and blocks until the matching
// command_result arrives (matched by command ID) or the timeout elapses.
func (ac *agentConn) send(cmd proto.Command) (proto.CommandResult, error) {
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
	case <-time.After(commandTimeout):
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
