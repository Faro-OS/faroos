// Package catalog merges FaroOS's hand-curated app list with a large
// imported catalog from Unraid Community Applications, so the App Store
// has real breadth (thousands of apps, with real icons) instead of just
// the half-dozen hand-picked ones.
package catalog

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/faroos/faroos/internal/appcatalog"
)

// unraidFeedURL is Unraid Community Applications' public, community-run
// aggregate feed (maintained by Squidly271, the same data source the CA
// plugin itself uses). ~4000 apps, ~23MB.
const unraidFeedURL = "https://raw.githubusercontent.com/Squidly271/AppFeed/master/applicationFeed.json"

type feedRoot struct {
	Apps       int       `json:"apps"`
	LastUpdate string    `json:"last_updated"`
	AppList    []feedApp `json:"applist"`
}

type feedApp struct {
	Name         string      `json:"Name"`
	Repository   string      `json:"Repository"`
	Overview     string      `json:"Overview"`
	Icon         string      `json:"Icon"`
	CategoryList stringSlice `json:"CategoryList"`
	Config       configList  `json:"Config"`
}

type feedConfigAttrs struct {
	Name        string `json:"Name"`
	Target      string `json:"Target"`
	Default     string `json:"Default"`
	Mode        string `json:"Mode"`
	Description string `json:"Description"`
	Type        string `json:"Type"`
}

type feedConfig struct {
	Attributes feedConfigAttrs `json:"@attributes"`
	Value      string          `json:"value"`
}

// configList tolerates the feed's XML-to-JSON quirk where a template with
// exactly one Config entry serializes it as a bare object instead of a
// one-element array.
type configList []feedConfig

func (c *configList) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*c = nil
		return nil
	}
	if data[0] == '[' {
		var arr []feedConfig
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*c = arr
		return nil
	}
	var single feedConfig
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	*c = []feedConfig{single}
	return nil
}

// stringSlice tolerates the same single-value-vs-array quirk for
// CategoryList.
type stringSlice []string

func (s *stringSlice) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*s = nil
		return nil
	}
	if data[0] == '[' {
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*s = arr
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	*s = []string{single}
	return nil
}

// FetchUnraidCatalog downloads and parses the Unraid Community Applications
// feed, converting each usable template into a FaroOS app. Templates
// missing a name/image, or using multi-container/device-passthrough
// features FaroOS's simple deploy model doesn't support, are skipped
// rather than imported broken.
func FetchUnraidCatalog(ctx context.Context) ([]appcatalog.App, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, unraidFeedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "FaroOS (+https://github.com/Faro-OS/faroos)")

	client := &http.Client{Timeout: 2 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return nil, fmt.Errorf("unraid feed: unexpected status %d: %s", res.StatusCode, body)
	}

	var root feedRoot
	if err := json.NewDecoder(res.Body).Decode(&root); err != nil {
		return nil, fmt.Errorf("unraid feed: decode: %w", err)
	}

	seen := make(map[string]bool, len(root.AppList))
	apps := make([]appcatalog.App, 0, len(root.AppList))
	for _, fa := range root.AppList {
		app, ok := convertFeedApp(fa)
		if !ok {
			continue
		}
		if seen[app.ID] {
			continue
		}
		seen[app.ID] = true
		apps = append(apps, app)
	}
	return apps, nil
}

func convertFeedApp(fa feedApp) (appcatalog.App, bool) {
	name := strings.TrimSpace(fa.Name)
	image := strings.TrimSpace(fa.Repository)
	if name == "" || image == "" {
		return appcatalog.App{}, false
	}

	var ports []appcatalog.Port
	var volumes []appcatalog.Volume
	var env []appcatalog.EnvVar

	for _, c := range fa.Config {
		a := c.Attributes
		switch a.Type {
		case "Port":
			containerPort, err := strconv.Atoi(strings.TrimSpace(a.Target))
			if err != nil {
				continue
			}
			hostPort, err := strconv.Atoi(strings.TrimSpace(firstNonEmpty(c.Value, a.Default)))
			if err != nil {
				continue
			}
			proto := "tcp"
			if strings.EqualFold(a.Mode, "udp") {
				proto = "udp"
			}
			ports = append(ports, appcatalog.Port{Container: containerPort, Host: hostPort, Protocol: proto})

		case "Path":
			target := strings.TrimSpace(a.Target)
			if target == "" {
				continue
			}
			volumes = append(volumes, appcatalog.Volume{
				Name:      volumeName(a.Name, target, len(volumes)),
				Container: target,
			})

		case "Variable":
			key := strings.TrimSpace(a.Target)
			if key == "" {
				continue
			}
			env = append(env, appcatalog.EnvVar{
				Key:         key,
				Default:     firstNonEmpty(c.Value, a.Default),
				Description: a.Description,
			})

		case "Device":
			// Device passthrough isn't supported by FaroOS's deploy model
			// yet; the app is still imported (it may work without it), just
			// without that binding.
		}
	}

	if len(ports) == 0 {
		// Not fatal — plenty of legitimate apps have no exposed web UI
		// (workers, agents). Still importable; the dashboard just won't
		// show a clickable tile for it (no known port to link to).
	}

	category := "Other"
	if len(fa.CategoryList) > 0 {
		category = fa.CategoryList[0]
	}

	overview := strings.TrimSpace(strings.ReplaceAll(fa.Overview, "\r\n", "\n"))
	if len(overview) > 400 {
		overview = overview[:400] + "…"
	}

	return appcatalog.App{
		ID:          importedID(name, image),
		Name:        name,
		Description: overview,
		Icon:        strings.TrimSpace(fa.Icon),
		Image:       image,
		Category:    category,
		Source:      "unraid-ca",
		Ports:       ports,
		Volumes:     volumes,
		Env:         env,
	}, true
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// importedID is derived from name+image (not just name) and hashed, so
// it's both stable across refreshes (needed since deployed containers are
// named from this ID — see appcatalog.ContainerName) and collision-free
// even though many CA templates share a display name.
func importedID(name, image string) string {
	slug := strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		slug = "app"
	}
	sum := sha1.Sum([]byte(image))
	return "ca-" + slug + "-" + hex.EncodeToString(sum[:])[:8]
}

func volumeName(label, target string, index int) string {
	slug := strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(label), "-"), "-")
	if slug == "" {
		slug = strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(target), "-"), "-")
	}
	if slug == "" {
		slug = fmt.Sprintf("data-%d", index)
	}
	return slug
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
