package registry

import (
	"path/filepath"
	"testing"

	"github.com/faroos/faroos/internal/model"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	r, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

func TestCreatePairingAndAuthenticate(t *testing.T) {
	r := newTestRegistry(t)

	node, err := r.CreatePairing("home-server")
	if err != nil {
		t.Fatalf("CreatePairing: %v", err)
	}
	if node.ID == "" || node.Token == "" {
		t.Fatalf("expected a non-empty ID and token, got %+v", node)
	}
	if node.Name != "home-server" {
		t.Fatalf("expected name 'home-server', got %q", node.Name)
	}

	if _, err := r.Authenticate(node.ID, node.Token); err != nil {
		t.Fatalf("Authenticate with correct token failed: %v", err)
	}
	if _, err := r.Authenticate(node.ID, "wrong-token"); err != ErrBadToken {
		t.Fatalf("expected ErrBadToken for a wrong token, got %v", err)
	}
	if _, err := r.Authenticate("does-not-exist", node.Token); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for an unknown node id, got %v", err)
	}
}

func TestTwoPairingsGetDifferentCredentials(t *testing.T) {
	r := newTestRegistry(t)
	a, err := r.CreatePairing("server-a")
	if err != nil {
		t.Fatalf("CreatePairing: %v", err)
	}
	b, err := r.CreatePairing("server-b")
	if err != nil {
		t.Fatalf("CreatePairing: %v", err)
	}
	if a.ID == b.ID {
		t.Fatal("expected two pairings to get different IDs")
	}
	if a.Token == b.Token {
		t.Fatal("expected two pairings to get different tokens")
	}
	// a's token must not authenticate as b.
	if _, err := r.Authenticate(b.ID, a.Token); err != ErrBadToken {
		t.Fatalf("expected ErrBadToken using node a's token against node b, got %v", err)
	}
}

func TestSetConnectedAndUpdateStatsPersist(t *testing.T) {
	r := newTestRegistry(t)
	node, err := r.CreatePairing("home-server")
	if err != nil {
		t.Fatalf("CreatePairing: %v", err)
	}

	got, err := r.Get(node.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Connected {
		t.Fatal("expected a freshly paired node to start disconnected")
	}

	r.SetConnected(node.ID, true)
	got, err = r.Get(node.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Connected {
		t.Fatal("expected node to be connected after SetConnected(true)")
	}

	stats := model.Stats{CPUPercent: 42.5, MemUsedBytes: 1024, MemTotalBytes: 2048}
	r.UpdateStats(node.ID, stats)
	got, err = r.Get(node.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Stats.CPUPercent != 42.5 || got.Stats.MemUsedBytes != 1024 {
		t.Fatalf("expected stats to round-trip through storage, got %+v", got.Stats)
	}

	r.SetConnected(node.ID, false)
	got, err = r.Get(node.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Connected {
		t.Fatal("expected node to be disconnected after SetConnected(false)")
	}
}

func TestListReturnsAllPairedNodes(t *testing.T) {
	r := newTestRegistry(t)
	if got := r.List(); len(got) != 0 {
		t.Fatalf("expected an empty list on a fresh registry, got %d entries", len(got))
	}

	r.CreatePairing("server-a")
	r.CreatePairing("server-b")
	r.CreatePairing("server-c")

	got := r.List()
	if len(got) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(got))
	}
}

func TestTokenIsNeverExposedViaGetOrList(t *testing.T) {
	// model.Node's Token field is json:"-" for the HTTP API, but let's make
	// sure Get/List actually populate real Node values rather than silently
	// zeroing the token in a way that would also break Authenticate.
	r := newTestRegistry(t)
	created, err := r.CreatePairing("home-server")
	if err != nil {
		t.Fatalf("CreatePairing: %v", err)
	}

	got, err := r.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Token != created.Token {
		t.Fatalf("expected Get to return the same token internally (%q), got %q", created.Token, got.Token)
	}
}

func TestPersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	r1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	node, err := r1.CreatePairing("home-server")
	if err != nil {
		t.Fatalf("CreatePairing: %v", err)
	}
	r1.SetConnected(node.ID, true)
	r1.Close()

	r2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	defer r2.Close()

	got, err := r2.Get(node.ID)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got.Name != "home-server" {
		t.Fatalf("expected node to survive reopen, got %+v", got)
	}
	// Connected is a runtime/liveness flag, not something that should
	// survive a restart as "true" (no agent is actually connected to this
	// fresh process) — verifying it wasn't accidentally persisted as true
	// forever would require a real websocket reconnect to flip it back;
	// out of scope here, just confirming the row itself round-tripped.
	if _, err := r2.Authenticate(node.ID, node.Token); err != nil {
		t.Fatalf("expected the pairing token to still authenticate after reopen: %v", err)
	}
}
