// Command server runs the FaroOS central panel: the API + websocket
// endpoint agents connect to, and the web UI.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/faroos/faroos/internal/api"
	"github.com/faroos/faroos/internal/auth"
	"github.com/faroos/faroos/internal/catalog"
	"github.com/faroos/faroos/internal/installer"
	"github.com/faroos/faroos/internal/p2p"
	"github.com/faroos/faroos/internal/registry"
	"github.com/faroos/faroos/internal/relayclient"
	"github.com/faroos/faroos/internal/webui"
)

// version is overridden at release build time via
// -ldflags "-X main.version=vX.Y.Z" (see .github/workflows/release.yml).
var version = "0.0.1-dev"

const (
	managedRelayURL        = "wss://relay.faroos.dev/relay/connect"
	managedRelayPublicBase = "https://relay.faroos.dev/p"
)

type networkConfig struct {
	relayURL        string
	relayPublicBase string
	stunURL         string
	p2pEnabled      bool
}

func resolveNetworkConfig(getenv func(string) string) networkConfig {
	config := networkConfig{
		relayURL:        strings.TrimSpace(getenv("FAROOS_RELAY_URL")),
		relayPublicBase: strings.TrimSpace(getenv("FAROOS_RELAY_PUBLIC_BASE")),
		stunURL:         strings.TrimSpace(getenv("FAROOS_STUN_URL")),
		p2pEnabled:      !truthy(getenv("FAROOS_P2P_DISABLED")),
	}
	if !truthy(getenv("FAROOS_RELAY_DISABLED")) && config.relayURL == "" {
		config.relayURL = managedRelayURL
		if config.relayPublicBase == "" {
			config.relayPublicBase = managedRelayPublicBase
		}
	}
	if config.stunURL == "" {
		config.stunURL = p2p.DefaultSTUNURL
	}
	if !config.p2pEnabled {
		config.stunURL = ""
	}
	if truthy(getenv("FAROOS_RELAY_DISABLED")) {
		config.relayURL = ""
		config.relayPublicBase = ""
	}
	return config
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(version)
		return
	}

	port := os.Getenv("FAROOS_PORT")
	if port == "" {
		port = "8090"
	}

	dbPath := os.Getenv("FAROOS_DB")
	if dbPath == "" {
		dbPath = "faroos.db"
	}
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("failed to create db directory %s: %v", dir, err)
		}
	}

	reg, err := registry.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open registry database %s: %v", dbPath, err)
	}
	defer reg.Close()

	authSvc, err := auth.New(reg.DB())
	if err != nil {
		log.Fatalf("failed to initialize auth: %v", err)
	}

	catalogCachePath := os.Getenv("FAROOS_CATALOG_CACHE")
	if catalogCachePath == "" {
		catalogCachePath = filepath.Join(filepath.Dir(dbPath), "catalog-cache.json")
	}
	catalogStore := catalog.NewStore(catalogCachePath)
	if err := catalogStore.LoadCache(); err != nil {
		log.Printf("catalog: failed to load cache %s: %v", catalogCachePath, err)
	}
	if catalogStore.NeedsRefresh() {
		log.Printf("catalog: fetching Unraid Community Applications catalog in the background...")
		catalogStore.RefreshInBackground()
	}

	network := resolveNetworkConfig(os.Getenv)
	relayPublicURL := ""
	if network.relayURL != "" {
		credentialsPath := os.Getenv("FAROOS_RELAY_CREDENTIALS")
		if credentialsPath == "" {
			credentialsPath = filepath.Join(filepath.Dir(dbPath), "relay-credentials.json")
		}
		credentials, credentialErr := relayclient.LoadOrCreateCredentials(credentialsPath)
		if credentialErr != nil {
			log.Printf("relay: credentials unavailable: %v", credentialErr)
		} else {
			client, clientErr := relayclient.New(relayclient.Config{
				RelayURL:     network.relayURL,
				PublicBase:   network.relayPublicBase,
				LocalAddress: "127.0.0.1:" + port,
				Credentials:  credentials,
			})
			if clientErr != nil {
				log.Printf("relay: disabled: %v", clientErr)
			} else {
				relayPublicURL = client.PublicURL()
				go client.Run(context.Background())
			}
		}
	}

	srv := api.New(
		reg,
		authSvc,
		catalogStore,
		version,
		relayPublicURL,
		api.WithSTUNURL(network.stunURL),
		api.WithP2PEnabled(network.p2pEnabled),
	)

	mux := http.NewServeMux()
	srv.Routes(mux)
	agentBinaryDir := os.Getenv("FAROOS_AGENT_BIN_DIR")
	if agentBinaryDir == "" {
		agentBinaryDir = filepath.Join(filepath.Dir(dbPath), "downloads")
	}
	mux.Handle("/install/", installer.Handler(agentBinaryDir))
	mux.Handle("/", webui.Handler())

	addr := ":" + port
	log.Printf("FaroOS server %s listening on %s (db: %s)", version, addr, dbPath)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
