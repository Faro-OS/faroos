package catalog

import (
	"context"
	"testing"
	"time"
)

// TestFetchUnraidCatalogReal hits the real, live Unraid CA feed — this is
// an integration smoke test, not a unit test: it validates that our parser
// still matches the real feed's current shape (community-maintained feeds
// drift), which a fixture-based test can't catch. Skips cleanly if the
// network's unreachable.
func TestFetchUnraidCatalogReal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	apps, err := FetchUnraidCatalog(ctx)
	if err != nil {
		t.Skipf("could not reach the Unraid CA feed, skipping: %v", err)
	}

	if len(apps) < 1000 {
		t.Fatalf("expected at least 1000 imported apps, got %d — parser may be out of sync with the feed", len(apps))
	}
	t.Logf("imported %d apps", len(apps))

	var withPorts, withVolumes, withIcon, withEnv int
	ids := make(map[string]bool, len(apps))
	for _, a := range apps {
		if a.ID == "" {
			t.Fatalf("app %q has an empty ID", a.Name)
		}
		if ids[a.ID] {
			t.Fatalf("duplicate ID %q (app %q)", a.ID, a.Name)
		}
		ids[a.ID] = true

		if a.Image == "" {
			t.Fatalf("app %q (%s) has an empty image", a.Name, a.ID)
		}
		if len(a.Ports) > 0 {
			withPorts++
		}
		if len(a.Volumes) > 0 {
			withVolumes++
		}
		if a.Icon != "" {
			withIcon++
		}
		if len(a.Env) > 0 {
			withEnv++
		}
	}
	t.Logf("with ports: %d, with volumes: %d, with icon: %d, with env: %d", withPorts, withVolumes, withIcon, withEnv)

	if withIcon < len(apps)/2 {
		t.Errorf("expected most apps to have an icon URL, only %d/%d did", withIcon, len(apps))
	}
}
