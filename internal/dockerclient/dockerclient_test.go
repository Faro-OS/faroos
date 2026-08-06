package dockerclient

import (
	"context"
	"testing"
	"time"
)

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
