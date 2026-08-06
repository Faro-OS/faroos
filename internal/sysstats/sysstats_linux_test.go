//go:build linux

package sysstats

import "testing"

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
