// Package netspeed measures the Internet connection seen by a FaroOS agent.
package netspeed

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	ookla "github.com/showwin/speedtest-go/speedtest"
)

const measurementDuration = 6 * time.Second

// Result is a compact connection snapshot suitable for dashboard widgets.
type Result struct {
	DownloadMbps float64   `json:"downloadMbps"`
	UploadMbps   float64   `json:"uploadMbps"`
	LatencyMs    float64   `json:"latencyMs"`
	TestedAt     time.Time `json:"testedAt"`
	Provider     string    `json:"provider"`
	Server       string    `json:"server,omitempty"`
}

type runner func(context.Context) (Result, error)

// Tester serializes tests so two dashboard widgets cannot saturate the same
// server connection at once.
type Tester struct {
	run runner
	mu  sync.Mutex
}

func New() *Tester {
	return &Tester{run: runOokla}
}

// Run performs a real download, upload and latency measurement against the
// closest responsive Speedtest.net server.
func (t *Tester) Run(ctx context.Context) (Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	run := t.run
	if run == nil {
		run = runOokla
	}
	result, err := run(ctx)
	if err != nil {
		return Result{}, err
	}
	if result.DownloadMbps <= 0 || result.UploadMbps <= 0 || result.LatencyMs < 0 {
		return Result{}, fmt.Errorf("Ookla returned an incomplete measurement")
	}
	if result.TestedAt.IsZero() {
		result.TestedAt = time.Now()
	}
	result.Provider = "Ookla"
	return result, nil
}

func runOokla(ctx context.Context) (Result, error) {
	client := ookla.New(ookla.WithUserConfig(&ookla.UserConfig{
		UserAgent:      "FaroOS/1.0 Speedtest.net measurement",
		PingMode:       ookla.TCP,
		MaxConnections: 4,
	}))
	client.SetCaptureTime(measurementDuration)
	client.SetRateCaptureFrequency(100 * time.Millisecond)

	servers, err := client.FetchServerListContext(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("find Ookla servers: %w", err)
	}
	targets, err := servers.FindServer(nil)
	if err != nil || len(targets) == 0 {
		if err == nil {
			err = ookla.ErrServerNotFound
		}
		return Result{}, fmt.Errorf("select Ookla server: %w", err)
	}
	target := targets[0]
	if err := target.PingTestContext(ctx, nil); err != nil {
		return Result{}, fmt.Errorf("Ookla latency test: %w", err)
	}
	if err := target.DownloadTestContext(ctx); err != nil {
		return Result{}, fmt.Errorf("Ookla download test: %w", err)
	}
	if err := target.UploadTestContext(ctx); err != nil {
		return Result{}, fmt.Errorf("Ookla upload test: %w", err)
	}

	return Result{
		DownloadMbps: target.DLSpeed.Mbps(),
		UploadMbps:   target.ULSpeed.Mbps(),
		LatencyMs:    float64(target.Latency) / float64(time.Millisecond),
		TestedAt:     time.Now(),
		Provider:     "Ookla",
		Server:       serverName(target),
	}, nil
}

func serverName(server *ookla.Server) string {
	parts := make([]string, 0, 2)
	if location := strings.Trim(strings.Join(nonEmpty(server.Name, server.Country), ", "), " ,"); location != "" {
		parts = append(parts, location)
	}
	if sponsor := strings.TrimSpace(server.Sponsor); sponsor != "" && sponsor != "?" {
		parts = append(parts, sponsor)
	}
	return strings.Join(parts, " · ")
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" && value != "?" {
			result = append(result, value)
		}
	}
	return result
}
