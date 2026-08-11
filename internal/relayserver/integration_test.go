package relayserver_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/faroos/faroos/internal/relayclient"
	"github.com/faroos/faroos/internal/relayserver"
)

func TestRelayProxiesOnlyAgentSurfaces(t *testing.T) {
	largePayload := bytes.Repeat([]byte("faroos-relay-data-"), 128*1024)
	backendMux := http.NewServeMux()
	backendMux.HandleFunc("GET /install/agent.sh", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("installer-through-relay"))
	})
	backendMux.HandleFunc("GET /install/large", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(largePayload)
	})
	backendMux.HandleFunc("GET /api/agent/connect", func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		messageType, message, err := conn.ReadMessage()
		if err == nil {
			conn.WriteMessage(messageType, append([]byte("relay:"), message...))
		}
	})
	backend := httptest.NewServer(backendMux)
	defer backend.Close()

	relay, err := relayserver.New(t.TempDir() + "/relay.db")
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	relayHTTP := httptest.NewServer(relay.Handler())
	defer relayHTTP.Close()

	credentials := relayclient.Credentials{
		PanelID: strings.Repeat("p", 43),
		Secret:  strings.Repeat("s", 43),
	}
	client, err := relayclient.New(relayclient.Config{
		RelayURL:     "ws" + strings.TrimPrefix(relayHTTP.URL, "http") + "/relay/connect",
		PublicBase:   relayHTTP.URL + "/p",
		LocalAddress: backend.Listener.Addr().String(),
		Credentials:  credentials,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go client.Run(ctx)
	waitFor(t, client.Connected)

	res, err := http.Get(client.PublicURL() + "/install/agent.sh")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || string(body) != "installer-through-relay" {
		t.Fatalf("unexpected installer response: %d %q", res.StatusCode, body)
	}
	res, err = http.Get(client.PublicURL() + "/install/large")
	if err != nil {
		t.Fatal(err)
	}
	largeBody, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !bytes.Equal(largeBody, largePayload) {
		t.Fatalf("large relay transfer failed: status=%d bytes=%d", res.StatusCode, len(largeBody))
	}

	res, err = http.Get(client.PublicURL() + "/api/auth/status")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("private panel API was exposed with status %d", res.StatusCode)
	}

	websocketURL := "ws" + strings.TrimPrefix(client.PublicURL(), "http") + "/api/agent/connect"
	conn, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	_, message, err := conn.ReadMessage()
	conn.Close()
	if err != nil || string(message) != "relay:hello" {
		t.Fatalf("unexpected relayed WebSocket response %q, %v", message, err)
	}

	badHeaders := http.Header{"Authorization": []string{"Bearer " + strings.Repeat("x", 43)}}
	_, response, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(relayHTTP.URL, "http")+"/relay/connect?id="+credentials.PanelID,
		badHeaders,
	)
	if err == nil || response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected credential takeover to be rejected, response=%v err=%v", response, err)
	}

	secondBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("second-panel"))
	}))
	defer secondBackend.Close()
	second, err := relayclient.New(relayclient.Config{
		RelayURL:     "ws" + strings.TrimPrefix(relayHTTP.URL, "http") + "/relay/connect",
		PublicBase:   relayHTTP.URL + "/p",
		LocalAddress: secondBackend.Listener.Addr().String(),
		Credentials: relayclient.Credentials{
			PanelID: strings.Repeat("q", 43),
			Secret:  strings.Repeat("t", 43),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	secondCtx, secondCancel := context.WithCancel(context.Background())
	defer secondCancel()
	go second.Run(secondCtx)
	waitFor(t, second.Connected)
	res, err = http.Get(second.PublicURL() + "/install/check")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if string(body) != "second-panel" {
		t.Fatalf("request crossed panel sessions: %q", body)
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not become true")
}
