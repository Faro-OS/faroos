// Package appcatalog holds FaroOS's curated list of one-click-deployable
// self-hosted apps. Deliberately simple for the MVP: single-container apps
// only (no multi-container compose orchestration yet), deployed straight
// against the Docker Engine API.
package appcatalog

// Port maps a container port to a host port.
type Port struct {
	Container int    `json:"container"`
	Host      int    `json:"host"`
	Protocol  string `json:"protocol"` // "tcp" or "udp"
}

// Volume maps a named, agent-managed host directory to a path inside the
// container. The host-side path is computed by the agent
// (<data dir>/<app id>/<name>), never supplied by the catalog or the
// caller, so apps can't be pointed at arbitrary host paths.
type Volume struct {
	Name      string `json:"name"`
	Container string `json:"container"`
}

type App struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Image       string            `json:"image"`
	Ports       []Port            `json:"ports"`
	Volumes     []Volume          `json:"volumes"`
	Env         map[string]string `json:"env,omitempty"`
}

var Catalog = []App{
	{
		ID:          "uptime-kuma",
		Name:        "Uptime Kuma",
		Description: "Self-hosted monitoring for websites and services, with a clean status dashboard.",
		Image:       "louislam/uptime-kuma:1",
		Ports:       []Port{{Container: 3001, Host: 3101, Protocol: "tcp"}},
		Volumes:     []Volume{{Name: "data", Container: "/app/data"}},
	},
	{
		ID:          "vaultwarden",
		Name:        "Vaultwarden",
		Description: "Lightweight, Bitwarden-compatible password manager server.",
		Image:       "vaultwarden/server:latest",
		Ports:       []Port{{Container: 80, Host: 3102, Protocol: "tcp"}},
		Volumes:     []Volume{{Name: "data", Container: "/data"}},
	},
	{
		ID:          "jellyfin",
		Name:        "Jellyfin",
		Description: "Stream your own movies, shows, and music library from home.",
		Image:       "jellyfin/jellyfin:latest",
		Ports:       []Port{{Container: 8096, Host: 3103, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "config", Container: "/config"},
			{Name: "cache", Container: "/cache"},
			{Name: "media", Container: "/media"},
		},
	},
	{
		ID:          "filebrowser",
		Name:        "File Browser",
		Description: "A simple, standalone web file manager for a directory on the server.",
		Image:       "filebrowser/filebrowser:latest",
		Ports:       []Port{{Container: 80, Host: 3104, Protocol: "tcp"}},
		Volumes:     []Volume{{Name: "srv", Container: "/srv"}},
	},
	{
		ID:          "nextcloud",
		Name:        "Nextcloud",
		Description: "A private cloud for files, calendars, and collaboration.",
		Image:       "nextcloud:apache",
		Ports:       []Port{{Container: 80, Host: 3105, Protocol: "tcp"}},
		Volumes:     []Volume{{Name: "html", Container: "/var/www/html"}},
	},
	{
		ID:          "homeassistant",
		Name:        "Home Assistant",
		Description: "Open-source home automation hub (web UI only — device discovery needs host networking, not enabled by this catalog entry).",
		Image:       "homeassistant/home-assistant:stable",
		Ports:       []Port{{Container: 8123, Host: 3106, Protocol: "tcp"}},
		Volumes:     []Volume{{Name: "config", Container: "/config"}},
	},
}

func Find(id string) (App, bool) {
	for _, a := range Catalog {
		if a.ID == id {
			return a, true
		}
	}
	return App{}, false
}

// ContainerName is the deterministic Docker container name FaroOS gives a
// deployed app instance, used both to create it and to recognize it later
// (e.g. to show install status or to remove it).
func ContainerName(appID string) string {
	return "faroos-app-" + appID
}
