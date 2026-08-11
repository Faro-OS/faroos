// Package relayserver implements the public FaroOS reverse relay. Panels
// connect outbound and expose only the agent installer, update feed and
// authenticated agent WebSocket through multiplexed streams.
package relayserver

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"

	"github.com/faroos/faroos/internal/relaytransport"
)

var panelIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{32,64}$`)
var panelSecretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{32,128}$`)

type Server struct {
	store *credentialStore

	mu       sync.RWMutex
	sessions map[string]*yamux.Session
	upgrader websocket.Upgrader
}

func New(dbPath string) (*Server, error) {
	store, err := openCredentialStore(dbPath)
	if err != nil {
		return nil, err
	}
	return &Server{
		store:    store,
		sessions: make(map[string]*yamux.Session),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  32 * 1024,
			WriteBufferSize: 32 * 1024,
			CheckOrigin:     func(*http.Request) bool { return true },
		},
	}, nil
}

func (s *Server) Close() error {
	s.mu.Lock()
	for id, session := range s.sessions {
		session.Close()
		delete(s.sessions, id)
	}
	s.mu.Unlock()
	return s.store.close()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /relay/connect", s.handlePanelConnect)
	mux.HandleFunc("/p/", s.handlePublicRequest)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	return mux
}

func (s *Server) handlePanelConnect(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	secret := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if !panelIDPattern.MatchString(id) || !panelSecretPattern.MatchString(secret) {
		http.Error(w, "invalid relay credentials", http.StatusUnauthorized)
		return
	}
	if err := s.store.authorize(id, secret); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrBadCredentials) {
			status = http.StatusUnauthorized
		}
		http.Error(w, "relay authorization failed", status)
		return
	}

	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ws.SetReadLimit(1 << 20)
	transport := relaytransport.NewWebSocketConn(ws)
	config := yamux.DefaultConfig()
	config.EnableKeepAlive = true
	config.KeepAliveInterval = 20 * time.Second
	config.StreamOpenTimeout = 15 * time.Second
	config.LogOutput = io.Discard
	session, err := yamux.Client(transport, config)
	if err != nil {
		transport.Close()
		return
	}

	s.mu.Lock()
	previous := s.sessions[id]
	s.sessions[id] = session
	s.mu.Unlock()
	if previous != nil {
		previous.Close()
	}
	log.Printf("relay: panel %s connected", id)

	<-session.CloseChan()
	s.mu.Lock()
	if s.sessions[id] == session {
		delete(s.sessions, id)
	}
	s.mu.Unlock()
	log.Printf("relay: panel %s disconnected", id)
}

func (s *Server) handlePublicRequest(w http.ResponseWriter, r *http.Request) {
	id, backendPath, ok := parsePublicPath(r.URL.Path)
	if !ok || !allowedBackendRequest(r, backendPath) {
		http.NotFound(w, r)
		return
	}
	s.mu.RLock()
	session := s.sessions[id]
	s.mu.RUnlock()
	if session == nil || session.IsClosed() {
		http.Error(w, "FaroOS panel is offline", http.StatusServiceUnavailable)
		return
	}

	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			type result struct {
				conn net.Conn
				err  error
			}
			opened := make(chan result, 1)
			go func() {
				conn, err := session.Open()
				select {
				case opened <- result{conn: conn, err: err}:
				case <-ctx.Done():
					if conn != nil {
						conn.Close()
					}
				}
			}()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case result := <-opened:
				return result.conn, result.err
			}
		},
	}
	defer transport.CloseIdleConnections()
	target := &url.URL{Scheme: "http", Host: "faroos-panel"}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(target)
			request.Out.URL.Path = backendPath
			request.Out.Host = "faroos-panel"
			request.SetXForwarded()
		},
		Transport:     transport,
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			log.Printf("relay: proxy %s: %v", id, err)
			http.Error(w, "FaroOS panel tunnel unavailable", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}

func parsePublicPath(path string) (id, backendPath string, ok bool) {
	remainder := strings.TrimPrefix(path, "/p/")
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) != 2 || !panelIDPattern.MatchString(parts[0]) {
		return "", "", false
	}
	return parts[0], "/" + parts[1], true
}

func allowedBackendRequest(r *http.Request, path string) bool {
	if strings.HasPrefix(path, "/install/") {
		return r.Method == http.MethodGet || r.Method == http.MethodHead
	}
	return path == "/api/agent/connect" && r.Method == http.MethodGet &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
