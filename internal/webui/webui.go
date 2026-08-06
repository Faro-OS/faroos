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
