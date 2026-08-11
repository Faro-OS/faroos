package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/faroos/faroos/internal/auth"
	"github.com/faroos/faroos/internal/catalog"
	"github.com/faroos/faroos/internal/p2p"
	"github.com/faroos/faroos/internal/proto"
	"github.com/faroos/faroos/internal/registry"
	"github.com/gorilla/websocket"
)

func TestAgentUpgradesFromSignalingWebSocketToDirectP2P(t *testing.T) {
	reg, err := registry.Open(filepath.Join(t.TempDir(), "p2p.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	authSvc, err := auth.New(reg.DB())
	if err != nil {
		t.Fatal(err)
	}
	pairing, err := reg.CreatePairing("direct-node")
	if err != nil {
		t.Fatal(err)
	}

	server := New(
		reg,
		authSvc,
		catalog.NewStore(filepath.Join(t.TempDir(), "catalog.json")),
		"test-version",
		"https://relay.example/p/test",
		WithSTUNURL(""),
		WithP2PTimeout(45*time.Second),
	)
	mux := http.NewServeMux()
	server.Routes(mux)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	offerPeer, offer, err := p2p.NewOffer(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer offerPeer.Close()

	dialer := *websocket.DefaultDialer
	dialer.Subprotocols = []string{p2p.Subprotocol}
	websocketURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/agent/connect"
	signaling, _, err := dialer.Dial(websocketURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer signaling.Close()
	if signaling.Subprotocol() != p2p.Subprotocol {
		t.Fatalf("server did not negotiate %s", p2p.Subprotocol)
	}
	if err := signaling.WriteJSON(proto.Envelope{
		Type: proto.TypeHello,
		Hello: &proto.Hello{
			NodeID:   pairing.ID,
			Token:    pairing.Token,
			Version:  "test-version",
			P2POffer: offer,
		},
	}); err != nil {
		t.Fatal(err)
	}
	var response proto.Envelope
	if err := signaling.ReadJSON(&response); err != nil {
		t.Fatal(err)
	}
	if response.Type != proto.TypeP2PAnswer || response.P2PAnswer == nil || response.P2PAnswer.SDP == "" {
		t.Fatalf("unexpected P2P answer: %+v", response)
	}
	if err := offerPeer.SetAnswer(response.P2PAnswer.SDP); err != nil {
		t.Fatal(err)
	}
	direct, err := offerPeer.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer direct.Close()

	// The panel closes the central signaling/relay stream as soon as the
	// direct encrypted channel opens.
	signaling.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := signaling.ReadMessage(); err == nil {
		t.Fatal("signaling WebSocket remained open after direct upgrade")
	}

	if err := direct.WriteJSON(proto.Envelope{Type: proto.TypePing}); err != nil {
		t.Fatal(err)
	}
	if err := direct.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var pong proto.Envelope
	if err := direct.ReadJSON(&pong); err != nil {
		t.Fatal(err)
	}
	if pong.Type != proto.TypePong {
		t.Fatalf("unexpected response over direct channel: %s", pong.Type)
	}
}

func TestAgentFallsBackToWebSocketWhenDirectNegotiationFails(t *testing.T) {
	reg, err := registry.Open(filepath.Join(t.TempDir(), "fallback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	authSvc, err := auth.New(reg.DB())
	if err != nil {
		t.Fatal(err)
	}
	pairing, err := reg.CreatePairing("fallback-node")
	if err != nil {
		t.Fatal(err)
	}
	server := New(
		reg,
		authSvc,
		catalog.NewStore(filepath.Join(t.TempDir(), "catalog.json")),
		"test-version",
		"https://relay.example/p/test",
		WithSTUNURL(""),
	)
	mux := http.NewServeMux()
	server.Routes(mux)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	dialer := *websocket.DefaultDialer
	dialer.Subprotocols = []string{p2p.Subprotocol}
	websocketURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/agent/connect"
	connection, _, err := dialer.Dial(websocketURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.WriteJSON(proto.Envelope{
		Type: proto.TypeHello,
		Hello: &proto.Hello{
			NodeID:   pairing.ID,
			Token:    pairing.Token,
			Version:  "test-version",
			P2POffer: "{}",
		},
	}); err != nil {
		t.Fatal(err)
	}
	var answer proto.Envelope
	if err := connection.ReadJSON(&answer); err != nil {
		t.Fatal(err)
	}
	if answer.Type != proto.TypeP2PAnswer || answer.P2PAnswer == nil || answer.P2PAnswer.Error == "" {
		t.Fatalf("expected a failed P2P answer, got %+v", answer)
	}
	if err := connection.WriteJSON(proto.Envelope{Type: proto.TypePing}); err != nil {
		t.Fatal(err)
	}
	connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	var pong proto.Envelope
	if err := connection.ReadJSON(&pong); err != nil {
		t.Fatal(err)
	}
	if pong.Type != proto.TypePong {
		t.Fatalf("fallback WebSocket did not remain usable: %s", pong.Type)
	}
}
