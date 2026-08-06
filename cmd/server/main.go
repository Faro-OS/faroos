// Command server runs the nodectl central panel: the API + websocket
// endpoint agents connect to, and (once built) the web UI.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/faroos/faroos/internal/api"
	"github.com/faroos/faroos/internal/registry"
)

func main() {
	port := os.Getenv("FAROOS_PORT")
	if port == "" {
		port = "8090"
	}

	reg := registry.New()
	srv := api.New(reg)

	mux := http.NewServeMux()
	srv.Routes(mux)
	mux.Handle("/", staticOrPlaceholder())

	addr := ":" + port
	log.Printf("nodectl server listening on %s", addr)
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
		w.Write([]byte("nodectl server is running. Web UI not built yet — see web/README.md.\n"))
	})
}
