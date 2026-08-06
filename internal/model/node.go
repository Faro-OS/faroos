package model

import "time"

// Stats is the periodic system snapshot an agent reports.
type Stats struct {
	CPUPercent     float64   `json:"cpuPercent"`
	MemUsedBytes   uint64    `json:"memUsedBytes"`
	MemTotalBytes  uint64    `json:"memTotalBytes"`
	DiskUsedBytes  uint64    `json:"diskUsedBytes"`
	DiskTotalBytes uint64    `json:"diskTotalBytes"`
	Uptime         uint64    `json:"uptimeSeconds"`
	Timestamp      time.Time `json:"timestamp"`
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
