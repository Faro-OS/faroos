// Command relay runs the public FaroOS relay service. Put it behind an HTTPS
// reverse proxy that supports WebSocket upgrades (for example Caddy).
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/faroos/faroos/internal/relayserver"
	"github.com/faroos/faroos/internal/stunserver"
)

var version = "0.0.1-dev"

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(version)
		return
	}
	addr := os.Getenv("FAROOS_RELAY_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dbPath := os.Getenv("FAROOS_RELAY_DB")
	if dbPath == "" {
		dbPath = "/var/lib/faroos-relay/relay.db"
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		log.Fatalf("create relay data directory: %v", err)
	}

	relay, err := relayserver.New(dbPath)
	if err != nil {
		log.Fatalf("initialize relay: %v", err)
	}
	defer relay.Close()
	stunAddr := os.Getenv("FAROOS_STUN_ADDR")
	if stunAddr == "" {
		stunAddr = ":3478"
	}
	stun, err := stunserver.New(stunAddr)
	if err != nil {
		log.Fatalf("initialize STUN rendezvous service: %v", err)
	}
	defer stun.Close()

	server := &http.Server{
		Addr:              addr,
		Handler:           relay.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	log.Printf("FaroOS relay %s listening on %s", version, addr)
	log.Printf("FaroOS STUN rendezvous listening on %s/udp", stunAddr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("relay shutdown: %v", err)
		}
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}
}
