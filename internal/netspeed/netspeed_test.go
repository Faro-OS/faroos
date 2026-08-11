package netspeed

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	ookla "github.com/showwin/speedtest-go/speedtest"
)

func TestRunUsesOoklaResult(t *testing.T) {
	testedAt := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	tester := &Tester{run: func(context.Context) (Result, error) {
		return Result{
			DownloadMbps: 941.25,
			UploadMbps:   812.75,
			LatencyMs:    4.2,
			TestedAt:     testedAt,
			Provider:     "wrong label",
			Server:       "Madrid · Example ISP",
		}, nil
	}}

	result, err := tester.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Provider != "Ookla" || result.DownloadMbps != 941.25 || result.UploadMbps != 812.75 || result.LatencyMs != 4.2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !result.TestedAt.Equal(testedAt) || result.Server != "Madrid · Example ISP" {
		t.Fatalf("unexpected metadata: %+v", result)
	}
}

func TestRunRejectsIncompleteMeasurement(t *testing.T) {
	tester := &Tester{run: func(context.Context) (Result, error) {
		return Result{DownloadMbps: 100, LatencyMs: 5}, nil
	}}
	if _, err := tester.Run(context.Background()); err == nil {
		t.Fatal("expected incomplete measurement to fail")
	}
}

func TestRunPropagatesMeasurementFailure(t *testing.T) {
	want := errors.New("Ookla unavailable")
	tester := &Tester{run: func(context.Context) (Result, error) { return Result{}, want }}
	if _, err := tester.Run(context.Background()); !errors.Is(err, want) {
		t.Fatalf("Run error = %v, want %v", err, want)
	}
}

func TestRunSerializesMeasurements(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	tester := &Tester{run: func(context.Context) (Result, error) {
		current := active.Add(1)
		if current > maximum.Load() {
			maximum.Store(current)
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		return Result{DownloadMbps: 1, UploadMbps: 1, LatencyMs: 1}, nil
	}}
	done := make(chan struct{}, 2)
	for range 2 {
		go func() {
			_, _ = tester.Run(context.Background())
			done <- struct{}{}
		}()
	}
	<-done
	<-done
	if maximum.Load() != 1 {
		t.Fatalf("maximum concurrent measurements = %d, want 1", maximum.Load())
	}
}

func TestServerName(t *testing.T) {
	server := &ookla.Server{Name: "Madrid", Country: "Spain", Sponsor: "Example ISP"}
	if got, want := serverName(server), "Madrid, Spain · Example ISP"; got != want {
		t.Fatalf("serverName = %q, want %q", got, want)
	}
}

func TestRunOoklaIntegration(t *testing.T) {
	if os.Getenv("FAROOS_OOKLA_INTEGRATION") != "1" {
		t.Skip("set FAROOS_OOKLA_INTEGRATION=1 to perform a real speed test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()

	result, err := New().Run(ctx)
	if err != nil {
		t.Fatalf("real Ookla speed test: %v", err)
	}
	if result.Server == "" {
		t.Fatal("real Ookla speed test did not report the selected server")
	}
	t.Logf("server=%q latency=%.2fms download=%.2fMbps upload=%.2fMbps", result.Server, result.LatencyMs, result.DownloadMbps, result.UploadMbps)
}
