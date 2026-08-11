package relayclient

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateCredentialsPersistsPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "relay.json")
	first, err := LoadOrCreateCredentials(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.PanelID) < 32 || len(first.Secret) < 32 || first.PanelID == first.Secret {
		t.Fatalf("weak or invalid credentials generated: %+v", first)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credentials mode = %o, want 600", info.Mode().Perm())
	}
	second, err := LoadOrCreateCredentials(path)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("credentials changed between loads: first=%+v second=%+v", first, second)
	}
}

func TestLoadOrCreateCredentialsRejectsMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateCredentials(path); err == nil {
		t.Fatal("expected malformed credentials to fail")
	}
}
