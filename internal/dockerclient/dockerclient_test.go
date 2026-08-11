package dockerclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNegotiatesDaemonAPIVersion(t *testing.T) {
	var versionRequests atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		versionRequests.Add(1)
		json.NewEncoder(w).Encode(map[string]string{"ApiVersion": "1.44"})
	})
	mux.HandleFunc("GET /v1.44/containers/json", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("all") != "true" {
			t.Errorf("expected all=true, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := &Client{http: server.Client(), baseURL: server.URL}
	for range 2 {
		containers, err := client.ListContainers(context.Background())
		if err != nil {
			t.Fatalf("ListContainers: %v", err)
		}
		if len(containers) != 0 {
			t.Fatalf("expected no containers, got %d", len(containers))
		}
	}
	if got := versionRequests.Load(); got != 1 {
		t.Fatalf("expected API negotiation once, got %d requests", got)
	}
}

func TestNormalizeAPIVersion(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "1.44", want: "1.44", ok: true},
		{input: "v1.53", want: "1.53", ok: true},
		{input: "", ok: false},
		{input: "latest", ok: false},
		{input: "1.44.1", ok: false},
	} {
		got, err := normalizeAPIVersion(test.input)
		if test.ok && (err != nil || got != test.want) {
			t.Errorf("normalizeAPIVersion(%q) = %q, %v; want %q", test.input, got, err, test.want)
		}
		if !test.ok && err == nil {
			t.Errorf("normalizeAPIVersion(%q) unexpectedly succeeded with %q", test.input, got)
		}
	}
}

func TestCreateContainerSendsExecFormCommand(t *testing.T) {
	var received struct {
		Image string   `json:"Image"`
		Cmd   []string `json:"Cmd"`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1.44/containers/create", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("name"); got != "cloudflare" {
			t.Errorf("container name = %q, want cloudflare", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode create payload: %v", err)
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"Id": "container-id"})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := &Client{http: server.Client(), baseURL: server.URL, apiVersion: "1.44"}
	id, err := client.CreateContainer(context.Background(), ContainerSpec{
		Name:    "cloudflare",
		Image:   "cloudflare/cloudflared:latest",
		Command: []string{"tunnel", "run", "--token", "secret"},
	})
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	if id != "container-id" {
		t.Fatalf("container id = %q, want container-id", id)
	}
	want := []string{"tunnel", "run", "--token", "secret"}
	if len(received.Cmd) != len(want) {
		t.Fatalf("Cmd = %#v, want %#v", received.Cmd, want)
	}
	for i := range want {
		if received.Cmd[i] != want[i] {
			t.Fatalf("Cmd = %#v, want %#v", received.Cmd, want)
		}
	}
}

// TestAgainstLocalDaemon exercises the client against whatever Docker
// daemon is available on the machine running the test. It skips cleanly if
// there's no socket, so CI (which may not have Docker) doesn't fail.
func TestAgainstLocalDaemon(t *testing.T) {
	c := New("/var/run/docker.sock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Ping(ctx); err != nil {
		t.Skipf("no local Docker daemon reachable, skipping: %v", err)
	}

	containers, err := c.ListContainers(ctx)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	t.Logf("found %d containers", len(containers))
	for _, c := range containers {
		t.Logf("- %s %v %s (%s / %s)", c.ID[:12], c.Names, c.Image, c.State, c.Status)
	}
}

// TestLifecycleAgainstTestContainer exercises stop/start/restart/logs
// against a disposable container named faroos-test-container. It expects
// that container to already exist (created out-of-band for this manual
// test run) and skips if it doesn't, so it's safe in CI/other machines.
func TestLifecycleAgainstTestContainer(t *testing.T) {
	c := New("/var/run/docker.sock")
	// Stopping a container gracefully can itself take up to Docker's
	// default 10s SIGTERM grace period, so this needs real headroom across
	// stop+start+restart+logs.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := c.Ping(ctx); err != nil {
		t.Skipf("no local Docker daemon reachable, skipping: %v", err)
	}

	containers, err := c.ListContainers(ctx)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	var id string
	for _, ct := range containers {
		for _, n := range ct.Names {
			if n == "/faroos-test-container" {
				id = ct.ID
			}
		}
	}
	if id == "" {
		t.Skip("faroos-test-container not found, skipping lifecycle test")
	}

	if err := c.StopContainer(ctx, id); err != nil {
		t.Fatalf("StopContainer: %v", err)
	}
	t.Log("stopped OK")

	if err := c.StartContainer(ctx, id); err != nil {
		t.Fatalf("StartContainer: %v", err)
	}
	t.Log("started OK")

	if err := c.RestartContainer(ctx, id); err != nil {
		t.Fatalf("RestartContainer: %v", err)
	}
	t.Log("restarted OK")

	time.Sleep(2 * time.Second)
	logs, err := c.Logs(ctx, id, 5)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	t.Logf("logs:\n%s", logs)
	if logs == "" {
		t.Fatal("expected non-empty logs from a container that prints every second")
	}
}
