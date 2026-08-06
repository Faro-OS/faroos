// Command server runs the FaroOS central panel: the API + websocket
// endpoint agents connect to, and the web UI.
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/faroos/faroos/internal/api"
	"github.com/faroos/faroos/internal/auth"
	"github.com/faroos/faroos/internal/registry"
	"github.com/faroos/faroos/internal/webui"
)

// version is overridden at release build time via
// -ldflags "-X main.version=vX.Y.Z" (see .github/workflows/release.yml).
var version = "0.0.1-dev"

func main() {
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

	srv := api.New(reg, authSvc)

	mux := http.NewServeMux()
	srv.Routes(mux)
	mux.Handle("/", http.FileServer(http.FS(webui.FS())))

	addr := ":" + port
	log.Printf("FaroOS server %s listening on %s (db: %s)", version, addr, dbPath)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
