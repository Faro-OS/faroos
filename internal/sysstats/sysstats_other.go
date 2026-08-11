//go:build !linux

package sysstats

import (
	"time"

	"github.com/faroos/faroos/internal/model"
)

// collect is a stub on non-Linux platforms for now. Managing macOS/Windows
// as full nodes (not just desktop clients) is deferred; this just keeps the
// agent buildable everywhere.
func collect(_ *Collector) model.Stats {
	return model.Stats{Timestamp: time.Now()}
}
