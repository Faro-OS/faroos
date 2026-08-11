// Package api implements the central panel's HTTP + websocket API.
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/faroos/faroos/internal/auth"
	"github.com/faroos/faroos/internal/catalog"
	"github.com/faroos/faroos/internal/p2p"
	"github.com/faroos/faroos/internal/proto"
	"github.com/faroos/faroos/internal/registry"
	"github.com/gorilla/websocket"
)

type Server struct {
	reg      *registry.Registry
	authSvc  *auth.Auth
	catalog  *catalog.Store
	hub      *hub
	version  string
	relayURL string
	stunURL  string
	p2p      bool
	p2pWait  time.Duration
	upgrader websocket.Upgrader
}

type Option func(*Server)

func WithSTUNURL(stunURL string) Option {
	return func(server *Server) { server.stunURL = strings.TrimSpace(stunURL) }
}

func WithP2PEnabled(enabled bool) Option {
	return func(server *Server) { server.p2p = enabled }
}

func WithP2PTimeout(timeout time.Duration) Option {
	return func(server *Server) { server.p2pWait = timeout }
}

func New(reg *registry.Registry, authSvc *auth.Auth, catalogStore *catalog.Store, version, relayURL string, options ...Option) *Server {
	server := &Server{
		reg:      reg,
		authSvc:  authSvc,
		catalog:  catalogStore,
		hub:      newHub(),
		version:  version,
		relayURL: strings.TrimRight(relayURL, "/"),
		stunURL:  p2p.DefaultSTUNURL,
		p2p:      true,
		p2pWait:  12 * time.Second,
		upgrader: websocket.Upgrader{
			// Agents and the web UI are both first-party; origin checking
			// matters once this is exposed beyond localhost/dev.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	for _, option := range options {
		option(server)
	}
	return server
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/setup", s.handleAuthSetup)
	mux.HandleFunc("POST /api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("GET /api/relay/status", s.requireAuth(s.handleRelayStatus))

	mux.HandleFunc("POST /api/nodes", s.requireAuth(s.handleCreatePairing))
	mux.HandleFunc("GET /api/nodes", s.requireAuth(s.handleListNodes))
	mux.HandleFunc("GET /api/nodes/{id}", s.requireAuth(s.handleGetNode))
	mux.HandleFunc("POST /api/nodes/{id}/pairing", s.requireAuth(s.handleRotatePairing))

	mux.HandleFunc("GET /api/nodes/{id}/containers", s.requireAuth(s.handleListContainers))
	mux.HandleFunc("POST /api/nodes/{id}/containers/{cid}/start", s.requireAuth(s.handleContainerAction("start")))
	mux.HandleFunc("POST /api/nodes/{id}/containers/{cid}/stop", s.requireAuth(s.handleContainerAction("stop")))
	mux.HandleFunc("POST /api/nodes/{id}/containers/{cid}/restart", s.requireAuth(s.handleContainerAction("restart")))
	mux.HandleFunc("GET /api/nodes/{id}/containers/{cid}/logs", s.requireAuth(s.handleContainerLogs))

	mux.HandleFunc("GET /api/nodes/{id}/terminal", s.requireAuth(s.handleTerminal))

	mux.HandleFunc("GET /api/nodes/{id}/files", s.requireAuth(s.handleListFiles))
	mux.HandleFunc("GET /api/nodes/{id}/files/download", s.requireAuth(s.handleDownloadFile))
	mux.HandleFunc("POST /api/nodes/{id}/files/upload", s.requireAuth(s.handleUploadFile))
	mux.HandleFunc("POST /api/nodes/{id}/files/directory", s.requireAuth(s.handleCreateDirectory))
	mux.HandleFunc("PATCH /api/nodes/{id}/files", s.requireAuth(s.handleRenameFile))
	mux.HandleFunc("DELETE /api/nodes/{id}/files", s.requireAuth(s.handleDeleteFile))

	mux.HandleFunc("GET /api/apps", s.requireAuth(s.handleListApps))
	mux.HandleFunc("GET /api/apps/categories", s.requireAuth(s.handleAppCategories))
	mux.HandleFunc("POST /api/apps/refresh", s.requireAuth(s.handleRefreshApps))
	mux.HandleFunc("POST /api/nodes/{id}/apps/{appId}/deploy", s.requireAuth(s.handleDeployApp))
	mux.HandleFunc("POST /api/nodes/{id}/apps/{appId}/remove", s.requireAuth(s.handleRemoveApp))
	mux.HandleFunc("GET /api/nodes/{id}/ports/{port}", s.requireAuth(s.handleInspectPort))
	mux.HandleFunc("POST /api/nodes/{id}/speedtest", s.requireAuth(s.handleInternetSpeedTest))

	// Agents authenticate with their own pairing token inside the hello
	// message, not with an admin session — this endpoint is intentionally
	// outside requireAuth.
	mux.HandleFunc("GET /api/agent/connect", s.handleAgentConnect)
}

func (s *Server) handleRelayStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"enabled":   s.relayURL != "",
		"publicUrl": s.relayURL,
		"p2p":       s.p2p,
	})
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
		"id":       n.ID,
		"name":     n.Name,
		"token":    n.Token,
		"panelUrl": s.relayURL,
	})
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := s.reg.List()
	for index := range nodes {
		if connection, ok := s.hub.get(nodes[index].ID); ok {
			nodes[index].Transport = connection.mode
		}
	}
	writeJSON(w, nodes)
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	n, err := s.reg.Get(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if connection, ok := s.hub.get(n.ID); ok {
		n.Transport = connection.mode
	}
	writeJSON(w, n)
}

func (s *Server) handleRotatePairing(w http.ResponseWriter, r *http.Request) {
	n, err := s.reg.RotatePairingToken(r.PathValue("id"))
	if err != nil {
		if err == registry.ErrNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{
		"id":       n.ID,
		"name":     n.Name,
		"token":    n.Token,
		"panelUrl": s.relayURL,
	})
}

// handleAgentConnect upgrades the connection, expects a hello envelope
// carrying pairing credentials, then streams stats updates.
func (s *Server) handleAgentConnect(w http.ResponseWriter, r *http.Request) {
	responseHeader := http.Header{}
	for _, protocol := range websocket.Subprotocols(r) {
		if s.p2p && protocol == p2p.Subprotocol {
			responseHeader.Set("Sec-WebSocket-Protocol", p2p.Subprotocol)
			break
		}
	}
	conn, err := s.upgrader.Upgrade(w, r, responseHeader)
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

	var active agentConnection = conn
	transportName := "relay"
	if s.p2p && env.Hello.P2POffer != "" && conn.Subprotocol() == p2p.Subprotocol {
		negotiationCtx, cancelNegotiation := context.WithTimeout(context.Background(), 8*time.Second)
		peer, answer, answerErr := p2p.Answer(negotiationCtx, s.stunURL, env.Hello.P2POffer)
		cancelNegotiation()
		if answerErr != nil {
			_ = conn.WriteJSON(proto.Envelope{
				Type:      proto.TypeP2PAnswer,
				P2PAnswer: &proto.P2PAnswer{Error: answerErr.Error()},
			})
			log.Printf("agent %s: direct negotiation unavailable: %v", node.ID, answerErr)
		} else {
			if writeErr := conn.WriteJSON(proto.Envelope{
				Type:      proto.TypeP2PAnswer,
				P2PAnswer: &proto.P2PAnswer{SDP: answer},
			}); writeErr != nil {
				peer.Close()
				return
			}
			directCtx, cancelDirect := context.WithTimeout(context.Background(), s.p2pWait)
			direct, directErr := peer.Connect(directCtx)
			cancelDirect()
			if directErr != nil {
				peer.Close()
				log.Printf("agent %s: direct path unavailable, using relay: %v", node.ID, directErr)
			} else {
				active = direct
				transportName = "direct-p2p"
				_ = conn.Close()
			}
		}
	}
	if transportName == "direct-p2p" {
		defer active.Close()
	}

	s.reg.SetConnected(node.ID, true)
	ac := s.hub.register(node.ID, active, transportName)
	defer func() {
		if s.hub.unregister(node.ID, ac) {
			s.reg.SetConnected(node.ID, false)
		}
	}()
	log.Printf("agent connected: %s (%s), version %s, transport %s", node.Name, node.ID, env.Hello.Version, transportName)
	if shouldBootstrapAgent(s.version, env.Hello.Version) {
		go s.bootstrapRemoteAgentUpdate(node.ID, node.Name, env.Hello.Version, ac)
	}

	for {
		active.SetReadDeadline(time.Now().Add(60 * time.Second))
		var msg proto.Envelope
		if err := active.ReadJSON(&msg); err != nil {
			log.Printf("agent %s disconnected: %v", node.ID, err)
			return
		}
		switch msg.Type {
		case proto.TypeStats:
			if msg.Stats != nil {
				s.reg.UpdateStats(node.ID, *msg.Stats)
			}
		case proto.TypePing:
			_ = ac.writeEnvelope(proto.Envelope{Type: proto.TypePong})
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
