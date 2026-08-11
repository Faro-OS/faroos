package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/faroos/faroos/internal/auth"
	"github.com/faroos/faroos/internal/catalog"
	"github.com/faroos/faroos/internal/registry"
)

// newTestServer wires up a real Server against a temp-file SQLite registry
// (registry.Open needs a real file, ":memory:" won't persist across the
// separate connections FaroOS's packages each open) and an empty catalog
// store that never hits the network, then wraps it in an httptest.Server
// so tests exercise the actual HTTP routing/middleware, not just handler
// functions in isolation.
func newTestServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	reg, err := registry.Open(dbPath)
	if err != nil {
		t.Fatalf("registry.Open: %v", err)
	}
	t.Cleanup(func() { reg.Close() })

	authSvc, err := auth.New(reg.DB())
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}

	catalogStore := catalog.NewStore(filepath.Join(t.TempDir(), "catalog-cache.json"))

	srv := New(reg, authSvc, catalogStore, "test-version", "https://relay.example/p/test-panel")
	mux := http.NewServeMux()
	srv.Routes(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	return ts, &http.Client{Jar: jar}
}

func doJSON(t *testing.T, client *http.Client, method, url string, body any) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return res
}

func decodeJSON[T any](t *testing.T, res *http.Response) T {
	t.Helper()
	defer res.Body.Close()
	var v T
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func TestAuthAndNodeLifecycle(t *testing.T) {
	ts, client := newTestServer(t)

	// Before setup: needsSetup is true, and protected routes reject us.
	res := doJSON(t, client, http.MethodGet, ts.URL+"/api/auth/status", nil)
	status := decodeJSON[struct {
		NeedsSetup    bool `json:"needsSetup"`
		Authenticated bool `json:"authenticated"`
	}](t, res)
	if !status.NeedsSetup || status.Authenticated {
		t.Fatalf("expected needsSetup=true authenticated=false before setup, got %+v", status)
	}

	res = doJSON(t, client, http.MethodPost, ts.URL+"/api/nodes", map[string]string{"name": "test"})
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 creating a node before login, got %d", res.StatusCode)
	}
	res.Body.Close()

	// Setup creates the admin and logs us in (session cookie set).
	res = doJSON(t, client, http.MethodPost, ts.URL+"/api/auth/setup", map[string]string{
		"username": "gonzalo", "password": "correcthorsebattery",
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from setup, got %d", res.StatusCode)
	}
	res.Body.Close()

	res = doJSON(t, client, http.MethodGet, ts.URL+"/api/auth/status", nil)
	status = decodeJSON[struct {
		NeedsSetup    bool `json:"needsSetup"`
		Authenticated bool `json:"authenticated"`
	}](t, res)
	if status.NeedsSetup || !status.Authenticated {
		t.Fatalf("expected needsSetup=false authenticated=true after setup, got %+v", status)
	}
	res = doJSON(t, client, http.MethodGet, ts.URL+"/api/relay/status", nil)
	relayStatus := decodeJSON[struct {
		Enabled   bool   `json:"enabled"`
		PublicURL string `json:"publicUrl"`
	}](t, res)
	if !relayStatus.Enabled || relayStatus.PublicURL != "https://relay.example/p/test-panel" {
		t.Fatalf("unexpected relay status: %+v", relayStatus)
	}

	// A second setup attempt must fail — only one admin account ever.
	res = doJSON(t, client, http.MethodPost, ts.URL+"/api/auth/setup", map[string]string{
		"username": "someoneelse", "password": "irrelevant123",
	})
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 on a second setup attempt, got %d", res.StatusCode)
	}
	res.Body.Close()

	// Now authenticated: pair a node.
	res = doJSON(t, client, http.MethodPost, ts.URL+"/api/nodes", map[string]string{"name": "my-server"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 pairing a node, got %d", res.StatusCode)
	}
	pairing := decodeJSON[struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Token    string `json:"token"`
		PanelURL string `json:"panelUrl"`
	}](t, res)
	if pairing.ID == "" || pairing.Token == "" || pairing.Name != "my-server" || pairing.PanelURL != "https://relay.example/p/test-panel" {
		t.Fatalf("unexpected pairing result: %+v", pairing)
	}

	// The dashboard can show a fresh one-command installer again. This rotates
	// the secret instead of exposing the previously stored credential.
	res = doJSON(t, client, http.MethodPost, ts.URL+"/api/nodes/"+pairing.ID+"/pairing", nil)
	rotated := decodeJSON[struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Token string `json:"token"`
	}](t, res)
	if rotated.ID != pairing.ID || rotated.Name != pairing.Name || rotated.Token == "" || rotated.Token == pairing.Token {
		t.Fatalf("unexpected rotated pairing result: %+v", rotated)
	}

	// It shows up in the list, disconnected (no agent has connected).
	res = doJSON(t, client, http.MethodGet, ts.URL+"/api/nodes", nil)
	nodes := decodeJSON[[]struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Connected bool   `json:"connected"`
	}](t, res)
	if len(nodes) != 1 || nodes[0].ID != pairing.ID || nodes[0].Connected {
		t.Fatalf("unexpected node list: %+v", nodes)
	}

	// Fetching it directly by ID works; a bogus ID 404s.
	res = doJSON(t, client, http.MethodGet, ts.URL+"/api/nodes/"+pairing.ID, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 fetching the node by id, got %d", res.StatusCode)
	}
	res.Body.Close()

	res = doJSON(t, client, http.MethodGet, ts.URL+"/api/nodes/does-not-exist", nil)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown node id, got %d", res.StatusCode)
	}
	res.Body.Close()

	// A node that isn't connected can't be asked for its containers.
	res = doJSON(t, client, http.MethodGet, ts.URL+"/api/nodes/"+pairing.ID+"/containers", nil)
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 listing containers on a disconnected node, got %d", res.StatusCode)
	}
	res.Body.Close()

	res = doJSON(t, client, http.MethodPost, ts.URL+"/api/nodes/"+pairing.ID+"/speedtest", nil)
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 testing speed on a disconnected node, got %d", res.StatusCode)
	}
	res.Body.Close()

	// Logout invalidates the session — protected routes reject us again.
	res = doJSON(t, client, http.MethodPost, ts.URL+"/api/auth/logout", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from logout, got %d", res.StatusCode)
	}
	res.Body.Close()

	res = doJSON(t, client, http.MethodGet, ts.URL+"/api/nodes", nil)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 listing nodes after logout, got %d", res.StatusCode)
	}
	res.Body.Close()
}

func TestLoginRejectsWrongCredentials(t *testing.T) {
	ts, client := newTestServer(t)

	res := doJSON(t, client, http.MethodPost, ts.URL+"/api/auth/setup", map[string]string{
		"username": "gonzalo", "password": "correcthorsebattery",
	})
	res.Body.Close()

	// Fresh client (no session cookie from setup).
	jar, _ := cookiejar.New(nil)
	freshClient := &http.Client{Jar: jar}

	res = doJSON(t, freshClient, http.MethodPost, ts.URL+"/api/auth/login", map[string]string{
		"username": "gonzalo", "password": "wrongpassword",
	})
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", res.StatusCode)
	}
	res.Body.Close()

	res = doJSON(t, freshClient, http.MethodGet, ts.URL+"/api/nodes", nil)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 listing nodes without a valid session, got %d", res.StatusCode)
	}
	res.Body.Close()

	res = doJSON(t, freshClient, http.MethodPost, ts.URL+"/api/auth/login", map[string]string{
		"username": "gonzalo", "password": "correcthorsebattery",
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for correct credentials, got %d", res.StatusCode)
	}
	res.Body.Close()

	res = doJSON(t, freshClient, http.MethodGet, ts.URL+"/api/nodes", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 listing nodes after a valid login, got %d", res.StatusCode)
	}
	res.Body.Close()
}

func TestListAppsIncludesCuratedCatalog(t *testing.T) {
	ts, client := newTestServer(t)
	res := doJSON(t, client, http.MethodPost, ts.URL+"/api/auth/setup", map[string]string{
		"username": "gonzalo", "password": "correcthorsebattery",
	})
	res.Body.Close()

	res = doJSON(t, client, http.MethodGet, ts.URL+"/api/apps", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 listing apps, got %d", res.StatusCode)
	}
	apps := decodeJSON[[]struct {
		ID     string `json:"id"`
		Source string `json:"source"`
	}](t, res)
	if len(apps) == 0 {
		t.Fatal("expected at least the curated apps to be listed even with an empty imported catalog")
	}
	for _, a := range apps {
		if a.Source != "faroos" {
			t.Fatalf("expected only curated (source=faroos) apps with no import, got source=%q", a.Source)
		}
	}
}
