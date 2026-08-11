// Package installer serves the zero-touch agent installer and architecture-
// specific agent binaries from the FaroOS panel itself.
package installer

import (
	"embed"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed agent-install.sh
var assets embed.FS

type handler struct {
	binaryDir   string
	updaterPath string
	updateDir   string
}

func Handler(binaryDir string) http.Handler {
	return HandlerWithUpdaterAndFeed(
		binaryDir,
		"/usr/local/libexec/faroos-server-update",
		filepath.Join(filepath.Dir(binaryDir), "update-feed"),
	)

}

func HandlerWithUpdater(binaryDir, updaterPath string) http.Handler {
	return HandlerWithUpdaterAndFeed(binaryDir, updaterPath, "")
}

func HandlerWithUpdaterAndFeed(binaryDir, updaterPath, updateDir string) http.Handler {
	return &handler{binaryDir: binaryDir, updaterPath: updaterPath, updateDir: updateDir}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/install/agent.sh":
		data, err := assets.ReadFile("agent-install.sh")
		if err != nil {
			http.Error(w, "installer unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(data)
	case "/install/agent/amd64", "/install/agent/arm64":
		arch := filepath.Base(r.URL.Path)
		filePath := filepath.Join(h.binaryDir, "faroos-agent-linux-"+arch)
		file, err := os.Open(filePath)
		if err != nil {
			http.Error(w, "agent binary is not available for "+arch, http.StatusNotFound)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			http.Error(w, "agent binary is unavailable", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="faroos-agent-linux-`+arch+`"`)
		w.Header().Set("Cache-Control", "public, max-age=300")
		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
	case "/install/updater":
		h.serveFile(w, r, h.updaterPath, "text/x-shellscript; charset=utf-8", false)
	case "/install/update/VERSION", "/install/update/SHA256SUMS",
		"/install/update/faroos-agent-linux-amd64", "/install/update/faroos-agent-linux-arm64",
		"/install/update/faroos-update":
		if h.updateDir == "" {
			http.NotFound(w, r)
			return
		}
		name := filepath.Base(r.URL.Path)
		contentType := "application/octet-stream"
		if name == "VERSION" || name == "SHA256SUMS" {
			contentType = "text/plain; charset=utf-8"
		} else if name == "faroos-update" {
			contentType = "text/x-shellscript; charset=utf-8"
		}
		h.serveFile(w, r, filepath.Join(h.updateDir, name), contentType, true)
	default:
		http.NotFound(w, r)
	}
}

func (h *handler) serveFile(w http.ResponseWriter, r *http.Request, path, contentType string, cache bool) {
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "update file is unavailable", http.StatusNotFound)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "update file is unavailable", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	if cache {
		w.Header().Set("Cache-Control", "no-cache")
	} else {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Disposition", `attachment; filename="`+info.Name()+`"`)
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}
