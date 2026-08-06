package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/faroos/faroos/internal/proto"
	"github.com/faroos/faroos/internal/registry"
)

// newCommandID returns a short random ID used to correlate a Command with
// its CommandResult over the agent websocket.
func newCommandID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// resolveConnectedNode checks the node exists and is currently connected,
// writing an appropriate HTTP error and returning ok=false if not.
func (s *Server) resolveConnectedNode(w http.ResponseWriter, nodeID string) (*agentConn, bool) {
	if _, err := s.reg.Get(nodeID); err != nil {
		if err == registry.ErrNotFound {
			http.Error(w, "node not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to look up node", http.StatusInternalServerError)
		}
		return nil, false
	}
	ac, ok := s.hub.get(nodeID)
	if !ok {
		http.Error(w, "node is not currently connected", http.StatusServiceUnavailable)
		return nil, false
	}
	return ac, true
}

func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}

	result, err := ac.send(proto.Command{ID: newCommandID(), Action: "containers.list"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return
	}
	if !result.OK {
		http.Error(w, result.Error, http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if len(result.Result) == 0 {
		w.Write([]byte("[]"))
		return
	}
	w.Write(result.Result)
}

func (s *Server) handleContainerAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
		if !ok {
			return
		}
		params, err := json.Marshal(map[string]string{"id": r.PathValue("cid")})
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		result, err := ac.send(proto.Command{ID: newCommandID(), Action: "containers." + action, Params: params})
		if err != nil {
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
			return
		}
		if !result.OK {
			http.Error(w, result.Error, http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func (s *Server) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	ac, ok := s.resolveConnectedNode(w, r.PathValue("id"))
	if !ok {
		return
	}

	tail := 200
	if v := r.URL.Query().Get("tail"); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			tail = n
		}
	}
	params, _ := json.Marshal(map[string]any{"id": r.PathValue("cid"), "tail": tail})

	result, err := ac.send(proto.Command{ID: newCommandID(), Action: "containers.logs", Params: params})
	if err != nil {
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return
	}
	if !result.OK {
		http.Error(w, result.Error, http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(result.Result)
}

func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, strconv.ErrSyntax
	}
	return n, nil
}
