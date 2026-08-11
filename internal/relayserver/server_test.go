package relayserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/faroos/faroos/internal/relayclient"
	"github.com/hashicorp/yamux"
)

func TestParseAndRestrictPublicPaths(t *testing.T) {
	id := strings.Repeat("a", 43)
	gotID, path, ok := parsePublicPath("/p/" + id + "/install/update/VERSION")
	if !ok || gotID != id || path != "/install/update/VERSION" {
		t.Fatalf("unexpected parsed path: id=%q path=%q ok=%v", gotID, path, ok)
	}

	for _, test := range []struct {
		method  string
		path    string
		upgrade string
		allowed bool
	}{
		{method: http.MethodGet, path: "/install/agent.sh", allowed: true},
		{method: http.MethodHead, path: "/install/update/VERSION", allowed: true},
		{method: http.MethodPost, path: "/install/agent.sh", allowed: false},
		{method: http.MethodGet, path: "/api/agent/connect", upgrade: "websocket", allowed: true},
		{method: http.MethodGet, path: "/api/agent/connect", allowed: false},
		{method: http.MethodGet, path: "/api/auth/status", allowed: false},
		{method: http.MethodGet, path: "/api/nodes", allowed: false},
	} {
		req, _ := http.NewRequest(test.method, "http://relay"+test.path, nil)
		req.Header.Set("Upgrade", test.upgrade)
		if got := allowedBackendRequest(req, test.path); got != test.allowed {
			t.Errorf("%s %s upgrade=%q allowed=%v, want %v", test.method, test.path, test.upgrade, got, test.allowed)
		}
	}
}

func TestPanelReconnectsAfterTunnelDrop(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer backend.Close()

	relay, err := New(t.TempDir() + "/relay.db")
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	relayHTTP := httptest.NewServer(relay.Handler())
	defer relayHTTP.Close()
	id := strings.Repeat("r", 43)
	client, err := relayclient.New(relayclient.Config{
		RelayURL:     "ws" + strings.TrimPrefix(relayHTTP.URL, "http") + "/relay/connect",
		PublicBase:   relayHTTP.URL + "/p",
		LocalAddress: backend.Listener.Addr().String(),
		Credentials:  relayclient.Credentials{PanelID: id, Secret: strings.Repeat("k", 43)},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go client.Run(ctx)

	var firstSession *yamux.Session
	waitUntil(t, func() bool {
		relay.mu.RLock()
		defer relay.mu.RUnlock()
		firstSession = relay.sessions[id]
		return firstSession != nil
	})
	firstSession.Close()

	waitUntil(t, func() bool {
		relay.mu.RLock()
		defer relay.mu.RUnlock()
		return relay.sessions[id] != nil && relay.sessions[id] != firstSession
	})
	res, err := http.Get(client.PublicURL() + "/install/test")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("relay failed after reconnect with status %d", res.StatusCode)
	}
}

func waitUntil(t *testing.T, condition func() bool) {
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
