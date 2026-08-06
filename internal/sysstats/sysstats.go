// Package sysstats collects local system metrics without pulling in a
// third-party dependency (e.g. gopsutil) — keeps the agent binary small
// and its runtime footprint minimal, which was an explicit requirement.
package sysstats

import "github.com/faroos/faroos/internal/model"

// Collect returns a fresh snapshot. Implementation is platform-specific;
// see sysstats_linux.go and sysstats_other.go.
func Collect() model.Stats {
	return collect()
}
