// Package appcatalog holds FaroOS's deployable app catalog: a small set of
// hand-curated apps plus (optionally) a large imported catalog from Unraid
// Community Applications. Deliberately simple for the MVP: single-container
// apps only (no multi-container compose orchestration yet), deployed
// straight against the Docker Engine API.
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

// EnvVar is an environment variable the container should be started with.
// Ordered (unlike a map) and carries the description/default a catalog
// source provided, since imported catalogs (Unraid CA) document these per
// variable.
type EnvVar struct {
	Key         string `json:"key"`
	Default     string `json:"default"`
	Description string `json:"description,omitempty"`
}

type App struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon,omitempty"` // URL; empty means "show a generated initial-letter tile"
	Image       string   `json:"image"`
	Category    string   `json:"category,omitempty"`
	Source      string   `json:"source"` // "faroos" | "unraid-ca"
	Ports       []Port   `json:"ports"`
	Volumes     []Volume `json:"volumes"`
	Env         []EnvVar `json:"env,omitempty"`
}

var Curated = []App{
	{
		ID:          "uptime-kuma",
		Name:        "Uptime Kuma",
		Description: "Self-hosted monitoring for websites and services, with a clean status dashboard.",
		Image:       "louislam/uptime-kuma:1",
		Category:    "Tools",
		Source:      "faroos",
		Ports:       []Port{{Container: 3001, Host: 3101, Protocol: "tcp"}},
		Volumes:     []Volume{{Name: "data", Container: "/app/data"}},
	},
	{
		ID:          "vaultwarden",
		Name:        "Vaultwarden",
		Description: "Lightweight, Bitwarden-compatible password manager server.",
		Image:       "vaultwarden/server:latest",
		Category:    "Security",
		Source:      "faroos",
		Ports:       []Port{{Container: 80, Host: 3102, Protocol: "tcp"}},
		Volumes:     []Volume{{Name: "data", Container: "/data"}},
	},
	{
		ID:          "jellyfin",
		Name:        "Jellyfin",
		Description: "Stream your own movies, shows, and music library from home.",
		Image:       "jellyfin/jellyfin:latest",
		Category:    "Media Servers",
		Source:      "faroos",
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
		Category:    "Tools",
		Source:      "faroos",
		Ports:       []Port{{Container: 80, Host: 3104, Protocol: "tcp"}},
		Volumes:     []Volume{{Name: "srv", Container: "/srv"}},
	},
	{
		ID:          "nextcloud",
		Name:        "Nextcloud",
		Description: "A private cloud for files, calendars, and collaboration.",
		Image:       "nextcloud:apache",
		Category:    "Files & Productivity",
		Source:      "faroos",
		Ports:       []Port{{Container: 80, Host: 3105, Protocol: "tcp"}},
		Volumes:     []Volume{{Name: "html", Container: "/var/www/html"}},
	},
	{
		ID:          "homeassistant",
		Name:        "Home Assistant",
		Description: "Open-source home automation hub (web UI only — device discovery needs host networking, not enabled by this catalog entry).",
		Image:       "homeassistant/home-assistant:stable",
		Category:    "Home Automation",
		Source:      "faroos",
		Ports:       []Port{{Container: 8123, Host: 3106, Protocol: "tcp"}},
		Volumes:     []Volume{{Name: "config", Container: "/config"}},
	},
}

// DeploySpec is exactly what an agent needs to deploy an app — the server
// resolves it from the merged catalog (curated + imported) and sends it
// whole, so the agent never needs its own copy of the catalog to deploy
// something. Only apps.remove needs just an ID (ContainerName is a pure
// naming convention, not catalog data).
type DeploySpec struct {
	AppID   string   `json:"appId"`
	Image   string   `json:"image"`
	Ports   []Port   `json:"ports"`
	Volumes []Volume `json:"volumes"`
	Env     []EnvVar `json:"env"`
}

// ContainerName is the deterministic Docker container name FaroOS gives a
// deployed app instance, used both to create it and to recognize it later
// (e.g. to show install status or to remove it).
func ContainerName(appID string) string {
	return "faroos-app-" + appID
}
