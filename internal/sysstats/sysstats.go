// Package sysstats collects local system metrics without pulling in a
// third-party dependency (e.g. gopsutil) — keeps the agent binary small
// and its runtime footprint minimal, which was an explicit requirement.
package sysstats

import (
	"sync"
	"time"

	"github.com/faroos/faroos/internal/model"
)

type networkSample struct {
	interfaceName string
	receivedBytes uint64
	transmitBytes uint64
	at            time.Time
}

// Collector keeps the previous network counter sample so it can report a
// real transfer rate. Linux exposes cumulative byte counters, not an
// instantaneous Mbps value, so a rate always needs two consecutive samples.
type Collector struct {
	mu              sync.Mutex
	previousNetwork networkSample
}

func NewCollector() *Collector {
	return &Collector{}
}

// Collect returns a fresh snapshot. Implementation is platform-specific;
// see sysstats_linux.go and sysstats_other.go.
func (c *Collector) Collect() model.Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return collect(c)
}

var defaultCollector = NewCollector()

// Collect uses a process-wide collector for callers that do not need an
// independently resettable sampling history.
func Collect() model.Stats { return defaultCollector.Collect() }
