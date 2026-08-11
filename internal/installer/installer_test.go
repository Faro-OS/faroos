package installer

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlerServesInstallerAndBinary(t *testing.T) {
	dir := t.TempDir()
	binary := []byte("fake-agent")
	updater := []byte("#!/bin/sh\necho update\n")
	if err := os.WriteFile(filepath.Join(dir, "faroos-agent-linux-amd64"), binary, 0o755); err != nil {
		t.Fatal(err)
	}
	updaterPath := filepath.Join(dir, "faroos-update")
	if err := os.WriteFile(updaterPath, updater, 0o755); err != nil {
		t.Fatal(err)
	}
	feedDir := filepath.Join(dir, "feed")
	if err := os.Mkdir(feedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(feedDir, "VERSION"), []byte("v1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(HandlerWithUpdaterAndFeed(dir, updaterPath, feedDir))
	defer ts.Close()

	res, err := http.Get(ts.URL + "/install/agent.sh")
	if err != nil {
		t.Fatal(err)
	}
	script, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.Contains(string(script), "FaroOS agent installed") {
		t.Fatalf("unexpected installer response: %d %q", res.StatusCode, script)
	}

	res, err = http.Get(ts.URL + "/install/agent/amd64")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || string(got) != string(binary) {
		t.Fatalf("unexpected binary response: %d %q", res.StatusCode, got)
	}

	res, err = http.Get(ts.URL + "/install/agent/ppc64")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("unsupported architecture returned %d", res.StatusCode)
	}

	res, err = http.Get(ts.URL + "/install/updater")
	if err != nil {
		t.Fatal(err)
	}
	got, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || string(got) != string(updater) {
		t.Fatalf("unexpected updater response: %d %q", res.StatusCode, got)
	}

	res, err = http.Get(ts.URL + "/install/update/VERSION")
	if err != nil {
		t.Fatal(err)
	}
	got, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || string(got) != "v1.2.3\n" {
		t.Fatalf("unexpected feed response: %d %q", res.StatusCode, got)
	}

	res, err = http.Get(ts.URL + "/install/update/not-allowed")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected feed file returned %d", res.StatusCode)
	}
}
