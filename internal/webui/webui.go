// Package webui embeds the built SvelteKit frontend (web/build, copied
// here as web/vite.config.ts's adapter-static output path) directly into
// the faroos-server binary. This makes every distribution method
// (curl-install, .deb/.rpm, the install ISO) self-contained — the server
// binary alone carries its own UI, with no separate directory that has to
// be shipped and kept alongside it at the right relative path.
//
// dist/ is tracked in git with only a placeholder index.html so `go build`
// keeps working from a fresh clone without the frontend having been built
// first; a real `npm run build` overwrites its contents with the actual
// site before compiling the server for a release.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the embedded frontend rooted at its content (i.e. dist/index.html
// becomes index.html), ready to hand to http.FileServer.
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// Can only happen if the "dist" directory embed above stops
		// matching reality, which would be a compile-time-detectable
		// packaging bug, not a runtime condition to recover from.
		panic("internal/webui: " + err.Error())
	}
	return sub
}

// Handler serves the embedded frontend and falls back to index.html for
// client-side routes such as /login. http.FileServer alone returns a 404 for
// those URLs even though SvelteKit's static adapter intentionally emits a SPA
// fallback, which makes direct navigation and browser refreshes fail.
func Handler() http.Handler {
	assets := FS()
	files := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "." {
			name = "index.html"
		}

		// HTML and SvelteKit's version marker must always be revalidated so an
		// already-open control centre can discover a newly installed frontend.
		// Hashed build assets are immutable and safe to cache for a long time.
		if name == "index.html" || name == "_app/version.json" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else if strings.HasPrefix(name, "_app/immutable/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		if _, err := fs.Stat(assets, name); err == nil {
			files.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead ||
			strings.HasPrefix(name, "api/") || strings.HasPrefix(name, "_app/") {
			http.NotFound(w, r)
			return
		}

		fallback := r.Clone(r.Context())
		urlCopy := *r.URL
		urlCopy.Path = "/"
		fallback.URL = &urlCopy
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		files.ServeHTTP(w, fallback)
	})
}
