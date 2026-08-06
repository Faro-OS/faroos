package model

import "time"

// Stats is the periodic system snapshot an agent reports.
type Stats struct {
	CPUPercent     float64   `json:"cpuPercent"`
	MemUsedBytes   uint64    `json:"memUsedBytes"`
	MemTotalBytes  uint64    `json:"memTotalBytes"`
	DiskUsedBytes  uint64    `json:"diskUsedBytes"`  // root filesystem ("/"), kept for the dashboard summary card
	DiskTotalBytes uint64    `json:"diskTotalBytes"` // root filesystem ("/")
	Disks          []Disk    `json:"disks,omitempty"`
	Uptime         uint64    `json:"uptimeSeconds"`
	Timestamp      time.Time `json:"timestamp"`
}

// Disk is one real, physical-ish mounted filesystem (pseudo filesystems
// like proc/tmpfs/overlay are filtered out by the collector).
type Disk struct {
	MountPoint string `json:"mountPoint"`
	Filesystem string `json:"filesystem"`
	TotalBytes uint64 `json:"totalBytes"`
	UsedBytes  uint64 `json:"usedBytes"`
}

// Node is a managed server known to the central panel.
type Node struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Token     string    `json:"-"`
	Connected bool      `json:"connected"`
	PairedAt  time.Time `json:"pairedAt"`
	LastSeen  time.Time `json:"lastSeen"`
	Stats     Stats     `json:"stats"`
}
