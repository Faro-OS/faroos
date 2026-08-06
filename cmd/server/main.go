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
)

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
	mux.Handle("/", staticOrPlaceholder())

	addr := ":" + port
	log.Printf("FaroOS server listening on %s (db: %s)", addr, dbPath)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// staticOrPlaceholder serves the built SvelteKit app from web/build if it
// exists, otherwise a placeholder so `go run` is useful before the frontend
// is built.
func staticOrPlaceholder() http.Handler {
	const webDir = "web/build"
	if info, err := os.Stat(webDir); err == nil && info.IsDir() {
		return http.FileServer(http.Dir(webDir))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("FaroOS server is running. Web UI not built yet — see web/README.md.\n"))
	})
}
