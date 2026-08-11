//go:build linux

package sysstats

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestRealDisksAgainstLocalMachine exercises disk enumeration against
// whatever is actually mounted on the machine running the test — a smoke
// test, not a correctness proof, but it would have caught the collector
// returning nothing or panicking on real /proc/mounts content.
func TestRealDisksAgainstLocalMachine(t *testing.T) {
	disks := realDisks()
	if len(disks) == 0 {
		t.Skip("no real disks detected (unusual environment), skipping assertions")
	}
	for _, d := range disks {
		t.Logf("%s (%s): %d / %d bytes used", d.MountPoint, d.Filesystem, d.UsedBytes, d.TotalBytes)
		if d.TotalBytes == 0 {
			t.Errorf("disk %s reported zero total bytes", d.MountPoint)
		}
		if d.UsedBytes > d.TotalBytes {
			t.Errorf("disk %s used (%d) exceeds total (%d)", d.MountPoint, d.UsedBytes, d.TotalBytes)
		}
	}
}

func TestDefaultRouteInterfaceChoosesLowestMetric(t *testing.T) {
	dir := t.TempDir()
	routePath := filepath.Join(dir, "route")
	contents := "Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\n" +
		"eth1 00000000 01010101 0003 0 0 200 00000000 0 0 0\n" +
		"eth0 00000000 01010101 0003 0 0 50 00000000 0 0 0\n" +
		"eth2 0000FEA9 00000000 0001 0 0 0 0000FFFF 0 0 0\n"
	if err := os.WriteFile(routePath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := defaultRouteInterface(routePath)
	if err != nil {
		t.Fatalf("defaultRouteInterface: %v", err)
	}
	if got != "eth0" {
		t.Fatalf("defaultRouteInterface = %q, want eth0", got)
	}
}

func TestInterfaceCountersReadsReceiveAndTransmitBytes(t *testing.T) {
	dir := t.TempDir()
	devPath := filepath.Join(dir, "dev")
	contents := "Inter-| Receive | Transmit\n" +
		" face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n" +
		"  eth0: 125000 10 0 0 0 0 0 0 375000 12 0 0 0 0 0 0\n"
	if err := os.WriteFile(devPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	received, transmitted, err := interfaceCounters(devPath, "eth0")
	if err != nil {
		t.Fatalf("interfaceCounters: %v", err)
	}
	if received != 125000 || transmitted != 375000 {
		t.Fatalf("interfaceCounters = (%d, %d), want (125000, 375000)", received, transmitted)
	}
}

func TestTransferRateMbps(t *testing.T) {
	if got := transferRateMbps(125_000, 1); math.Abs(got-1) > 0.000001 {
		t.Fatalf("transferRateMbps = %f, want 1 Mbps", got)
	}
	if got := transferRateMbps(125_000, 0.5); math.Abs(got-2) > 0.000001 {
		t.Fatalf("transferRateMbps = %f, want 2 Mbps", got)
	}
	if got := transferRateMbps(125_000, 0); got != 0 {
		t.Fatalf("transferRateMbps with zero interval = %f, want 0", got)
	}
}

func TestDecodeMountField(t *testing.T) {
	got := decodeMountField(`/media/My\040Drive/line\011tab/backslash\134name`)
	want := "/media/My Drive/line\ttab/backslash\\name"
	if got != want {
		t.Fatalf("decodeMountField = %q, want %q", got, want)
	}
}

func TestUnmountedExternalDiskIsIncludedOnce(t *testing.T) {
	sysClass := t.TempDir()
	devicePath := filepath.Join(sysClass, "sdb")
	if err := os.Mkdir(devicePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devicePath, "removable"), []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devicePath, "size"), []byte("125000000\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	disks := unmountedExternalDisks(sysClass, map[string]bool{})
	if len(disks) != 1 {
		t.Fatalf("unmountedExternalDisks returned %d disks, want 1", len(disks))
	}
	if disks[0].MountPoint != "/dev/sdb" || disks[0].Filesystem != "unmounted" || disks[0].TotalBytes != 64_000_000_000 {
		t.Fatalf("unexpected external disk: %+v", disks[0])
	}
	if got := unmountedExternalDisks(sysClass, map[string]bool{"sdb": true}); len(got) != 0 {
		t.Fatalf("mounted external disk was duplicated: %+v", got)
	}
}
