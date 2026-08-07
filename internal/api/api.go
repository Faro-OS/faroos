// Package api implements the central panel's HTTP + websocket API.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/faroos/faroos/internal/auth"
	"github.com/faroos/faroos/internal/catalog"
	"github.com/faroos/faroos/internal/proto"
	"github.com/faroos/faroos/internal/registry"
	"github.com/gorilla/websocket"
)

type Server struct {
	reg      *registry.Registry
	authSvc  *auth.Auth
	catalog  *catalog.Store
	hub      *hub
	upgrader websocket.Upgrader
}

func New(reg *registry.Registry, authSvc *auth.Auth, catalogStore *catalog.Store) *Server {
	return &Server{
		reg:     reg,
		authSvc: authSvc,
		catalog: catalogStore,
		hub:     newHub(),
		upgrader: websocket.Upgrader{
			// Agents and the web UI are both first-party; origin checking
			// matters once this is exposed beyond localhost/dev.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/setup", s.handleAuthSetup)
	mux.HandleFunc("POST /api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleAuthLogout)

	mux.HandleFunc("POST /api/nodes", s.requireAuth(s.handleCreatePairing))
	mux.HandleFunc("GET /api/nodes", s.requireAuth(s.handleListNodes))
	mux.HandleFunc("GET /api/nodes/{id}", s.requireAuth(s.handleGetNode))

	mux.HandleFunc("GET /api/nodes/{id}/containers", s.requireAuth(s.handleListContainers))
	mux.HandleFunc("POST /api/nodes/{id}/containers/{cid}/start", s.requireAuth(s.handleContainerAction("start")))
	mux.HandleFunc("POST /api/nodes/{id}/containers/{cid}/stop", s.requireAuth(s.handleContainerAction("stop")))
	mux.HandleFunc("POST /api/nodes/{id}/containers/{cid}/restart", s.requireAuth(s.handleContainerAction("restart")))
	mux.HandleFunc("GET /api/nodes/{id}/containers/{cid}/logs", s.requireAuth(s.handleContainerLogs))

	mux.HandleFunc("GET /api/nodes/{id}/terminal", s.requireAuth(s.handleTerminal))

	mux.HandleFunc("GET /api/nodes/{id}/files", s.requireAuth(s.handleListFiles))
	mux.HandleFunc("GET /api/nodes/{id}/files/download", s.requireAuth(s.handleDownloadFile))
	mux.HandleFunc("POST /api/nodes/{id}/files/upload", s.requireAuth(s.handleUploadFile))
	mux.HandleFunc("DELETE /api/nodes/{id}/files", s.requireAuth(s.handleDeleteFile))

	mux.HandleFunc("GET /api/apps", s.requireAuth(s.handleListApps))
	mux.HandleFunc("GET /api/apps/categories", s.requireAuth(s.handleAppCategories))
	mux.HandleFunc("POST /api/apps/refresh", s.requireAuth(s.handleRefreshApps))
	mux.HandleFunc("POST /api/nodes/{id}/apps/{appId}/deploy", s.requireAuth(s.handleDeployApp))
	mux.HandleFunc("POST /api/nodes/{id}/apps/{appId}/remove", s.requireAuth(s.handleRemoveApp))

	// Agents authenticate with their own pairing token inside the hello
	// message, not with an admin session — this endpoint is intentionally
	// outside requireAuth.
	mux.HandleFunc("GET /api/agent/connect", s.handleAgentConnect)
}

type createPairingReq struct {
	Name string `json:"name"`
}

func (s *Server) handleCreatePairing(w http.ResponseWriter, r *http.Request) {
	var req createPairingReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	n, err := s.reg.CreatePairing(req.Name)
	if err != nil {
		http.Error(w, "failed to create pairing", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{
		"id":    n.ID,
		"name":  n.Name,
		"token": n.Token,
	})
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.reg.List())
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	n, err := s.reg.Get(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, n)
}

// handleAgentConnect upgrades the connection, expects a hello envelope
// carrying pairing credentials, then streams stats updates.
func (s *Server) handleAgentConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("agent connect: upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	var env proto.Envelope
	if err := conn.ReadJSON(&env); err != nil || env.Type != proto.TypeHello || env.Hello == nil {
		log.Printf("agent connect: expected hello, got err=%v", err)
		return
	}

	node, err := s.reg.Authenticate(env.Hello.NodeID, env.Hello.Token)
	if err != nil {
		log.Printf("agent connect: auth failed for node %s: %v", env.Hello.NodeID, err)
		conn.WriteJSON(map[string]string{"error": "unauthorized"})
		return
	}

	s.reg.SetConnected(node.ID, true)
	ac := s.hub.register(node.ID, conn)
	defer func() {
		s.reg.SetConnected(node.ID, false)
		s.hub.unregister(node.ID, ac)
	}()
	log.Printf("agent connected: %s (%s)", node.Name, node.ID)

	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		var msg proto.Envelope
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("agent %s disconnected: %v", node.ID, err)
			return
		}
		switch msg.Type {
		case proto.TypeStats:
			if msg.Stats != nil {
				s.reg.UpdateStats(node.ID, *msg.Stats)
			}
		case proto.TypePing:
			conn.WriteJSON(proto.Envelope{Type: proto.TypePong})
		case proto.TypeCommandResult:
			if msg.CommandResult != nil {
				ac.resolve(*msg.CommandResult)
			}
		case proto.TypeTerminalOutput:
			if msg.TerminalData != nil {
				ac.dispatchStream(msg.TerminalData.SessionID, msg)
			}
		case proto.TypeTerminalClose:
			if msg.TerminalClose != nil {
				ac.dispatchStream(msg.TerminalClose.SessionID, msg)
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
