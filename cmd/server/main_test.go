package main

import (
	"testing"

	"github.com/faroos/faroos/internal/p2p"
)

func TestResolveNetworkConfigUsesManagedP2PByDefault(t *testing.T) {
	config := resolveNetworkConfig(func(string) string { return "" })
	if config.relayURL != managedRelayURL || config.relayPublicBase != managedRelayPublicBase {
		t.Fatalf("unexpected managed relay defaults: %+v", config)
	}
	if !config.p2pEnabled || config.stunURL != p2p.DefaultSTUNURL {
		t.Fatalf("unexpected managed P2P defaults: %+v", config)
	}
}

func TestResolveNetworkConfigHonorsOverridesAndOptOut(t *testing.T) {
	values := map[string]string{
		"FAROOS_RELAY_URL":         " wss://self.example/relay/connect ",
		"FAROOS_RELAY_PUBLIC_BASE": " https://self.example/p ",
		"FAROOS_STUN_URL":          " stun:self.example:3478 ",
	}
	config := resolveNetworkConfig(func(name string) string { return values[name] })
	if config.relayURL != "wss://self.example/relay/connect" ||
		config.relayPublicBase != "https://self.example/p" ||
		config.stunURL != "stun:self.example:3478" || !config.p2pEnabled {
		t.Fatalf("overrides were not preserved: %+v", config)
	}

	values["FAROOS_RELAY_DISABLED"] = "yes"
	values["FAROOS_P2P_DISABLED"] = "1"
	config = resolveNetworkConfig(func(name string) string { return values[name] })
	if config.relayURL != "" || config.relayPublicBase != "" || config.stunURL != "" || config.p2pEnabled {
		t.Fatalf("opt-out was not honored: %+v", config)
	}
}
