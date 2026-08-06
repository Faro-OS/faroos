//go:build linux

package sysstats

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/faroos/faroos/internal/model"
)

func collect() model.Stats {
	return model.Stats{
		CPUPercent:     cpuPercent(),
		MemUsedBytes:   memUsed(),
		MemTotalBytes:  memTotal(),
		DiskUsedBytes:  diskUsed("/"),
		DiskTotalBytes: diskTotal("/"),
		Disks:          realDisks(),
		Uptime:         uptimeSeconds(),
		Timestamp:      time.Now(),
	}
}

// pseudoFilesystems are mount types that don't represent real, physical-ish
// storage — no point showing them on a storage page.
var pseudoFilesystems = map[string]bool{
	"proc": true, "sysfs": true, "tmpfs": true, "devtmpfs": true,
	"cgroup": true, "cgroup2": true, "overlay": true, "squashfs": true,
	"devpts": true, "mqueue": true, "debugfs": true, "tracefs": true,
	"fusectl": true, "configfs": true, "ramfs": true, "nsfs": true,
	"binfmt_misc": true, "autofs": true, "rpc_pipefs": true,
	"securityfs": true, "pstore": true, "bpf": true, "hugetlbfs": true,
	"sunrpc": true, "efivarfs": true, "fuse.gvfsd-fuse": true,
}

// realDisks reads /proc/mounts and reports usage for real, physical-ish
// filesystems only, deduplicated by source device (bind mounts and
// container overlay mounts otherwise show the same disk many times over).
func realDisks() []model.Disk {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()

	seenDevices := make(map[string]bool)
	var disks []model.Disk

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		device, mountPoint, fsType := fields[0], fields[1], fields[2]

		if pseudoFilesystems[fsType] || !strings.HasPrefix(device, "/dev/") {
			continue
		}
		if seenDevices[device] {
			continue
		}

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mountPoint, &stat); err != nil {
			continue
		}
		total := stat.Blocks * uint64(stat.Bsize)
		if total == 0 {
			continue
		}
		free := stat.Bfree * uint64(stat.Bsize)

		seenDevices[device] = true
		disks = append(disks, model.Disk{
			MountPoint: mountPoint,
			Filesystem: fsType,
			TotalBytes: total,
			UsedBytes:  total - free,
		})
	}
	return disks
}

// cpuPercent samples /proc/stat twice, 200ms apart, and returns overall
// (non-idle) CPU usage as a percentage.
func cpuPercent() float64 {
	idle1, total1, err := readCPUTotals()
	if err != nil {
		return 0
	}
	time.Sleep(200 * time.Millisecond)
	idle2, total2, err := readCPUTotals()
	if err != nil {
		return 0
	}

	idleDelta := float64(idle2 - idle1)
	totalDelta := float64(total2 - total1)
	if totalDelta <= 0 {
		return 0
	}
	return (1 - idleDelta/totalDelta) * 100
}

func readCPUTotals() (idle, total uint64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, 0, scanner.Err()
	}
	fields := strings.Fields(scanner.Text()) // "cpu  user nice system idle iowait irq softirq steal ..."
	var vals []uint64
	for _, f := range fields[1:] {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			continue
		}
		vals = append(vals, v)
		total += v
	}
	if len(vals) >= 4 {
		idle = vals[3] // idle field
	}
	return idle, total, nil
}

func memInfo() map[string]uint64 {
	out := make(map[string]uint64)
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return out
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		v, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}
		out[key] = v * 1024 // values are in kB
	}
	return out
}

func memTotal() uint64 {
	return memInfo()["MemTotal"]
}

func memUsed() uint64 {
	m := memInfo()
	total := m["MemTotal"]
	avail, ok := m["MemAvailable"]
	if !ok {
		// Fallback for very old kernels without MemAvailable.
		avail = m["MemFree"] + m["Buffers"] + m["Cached"]
	}
	if avail > total {
		return 0
	}
	return total - avail
}

func diskTotal(path string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	return stat.Blocks * uint64(stat.Bsize)
}

func diskUsed(path string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	return total - free
}

func uptimeSeconds() uint64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return uint64(seconds)
}
