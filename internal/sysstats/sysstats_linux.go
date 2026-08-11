//go:build linux

package sysstats

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/faroos/faroos/internal/model"
)

func collect(c *Collector) model.Stats {
	interfaceName, receivedMbps, transmitMbps := networkThroughput(c, time.Now())
	return model.Stats{
		CPUPercent:          cpuPercent(),
		MemUsedBytes:        memUsed(),
		MemTotalBytes:       memTotal(),
		DiskUsedBytes:       diskUsed("/"),
		DiskTotalBytes:      diskTotal("/"),
		Disks:               realDisks(),
		NetworkInterface:    interfaceName,
		NetworkReceiveMbps:  receivedMbps,
		NetworkTransmitMbps: transmitMbps,
		Uptime:              uptimeSeconds(),
		Timestamp:           time.Now(),
	}
}

// networkThroughput reads the cumulative counters for the interface carrying
// the default route. It does not download test data: the reported rate is only
// traffic that actually crossed the server's active network interface between
// the two most recent samples.
func networkThroughput(c *Collector, now time.Time) (string, float64, float64) {
	interfaceName, err := defaultRouteInterface("/proc/net/route")
	if err != nil {
		c.previousNetwork = networkSample{}
		return "", 0, 0
	}
	received, transmitted, err := interfaceCounters("/proc/net/dev", interfaceName)
	if err != nil {
		c.previousNetwork = networkSample{}
		return "", 0, 0
	}

	current := networkSample{
		interfaceName: interfaceName,
		receivedBytes: received,
		transmitBytes: transmitted,
		at:            now,
	}
	previous := c.previousNetwork
	c.previousNetwork = current

	if previous.interfaceName != interfaceName || previous.at.IsZero() ||
		received < previous.receivedBytes || transmitted < previous.transmitBytes {
		return interfaceName, 0, 0
	}
	seconds := now.Sub(previous.at).Seconds()
	if seconds <= 0 {
		return interfaceName, 0, 0
	}

	receivedMbps := transferRateMbps(received-previous.receivedBytes, seconds)
	transmitMbps := transferRateMbps(transmitted-previous.transmitBytes, seconds)
	return interfaceName, receivedMbps, transmitMbps
}

func transferRateMbps(byteDelta uint64, seconds float64) float64 {
	if seconds <= 0 {
		return 0
	}
	return float64(byteDelta) * 8 / seconds / 1_000_000
}

func defaultRouteInterface(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	bestInterface := ""
	bestMetric := uint64(^uint32(0))
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 || fields[1] != "00000000" {
			continue
		}
		flags, err := strconv.ParseUint(fields[3], 16, 32)
		if err != nil || flags&1 == 0 {
			continue
		}
		metric, err := strconv.ParseUint(fields[6], 10, 64)
		if err != nil {
			metric = 0
		}
		if bestInterface == "" || metric < bestMetric {
			bestInterface = fields[0]
			bestMetric = metric
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if bestInterface == "" {
		return "", errors.New("default network interface not found")
	}
	return bestInterface, nil
}

func interfaceCounters(path, interfaceName string) (uint64, uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		name, counters, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) != interfaceName {
			continue
		}
		fields := strings.Fields(counters)
		if len(fields) < 9 {
			return 0, 0, errors.New("invalid network counters")
		}
		received, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		transmitted, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		return received, transmitted, nil
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}
	return 0, 0, errors.New("network interface counters not found")
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
	mountedBlocks := make(map[string]bool)
	var disks []model.Disk

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		device := decodeMountField(fields[0])
		mountPoint := decodeMountField(fields[1])
		fsType := fields[2]

		if pseudoFilesystems[fsType] || !strings.HasPrefix(device, "/dev/") {
			continue
		}
		markMountedBlockDevice("/sys/class/block", blockNameForDevice(device), mountedBlocks)
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
	return append(disks, unmountedExternalDisks("/sys/class/block", mountedBlocks)...)
}

func decodeMountField(value string) string {
	return strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\`,
	).Replace(value)
}

func blockNameForDevice(device string) string {
	if resolved, err := filepath.EvalSymlinks(device); err == nil {
		device = resolved
	}
	return filepath.Base(device)
}

// markMountedBlockDevice follows partitions and device-mapper slave links to
// their backing disks. This prevents a mounted USB disk from also appearing
// as a second, unmounted device.
func markMountedBlockDevice(sysClass, name string, mounted map[string]bool) {
	if name == "" || name == "." || mounted[name] {
		return
	}
	mounted[name] = true
	devicePath := filepath.Join(sysClass, name)
	if _, err := os.Stat(filepath.Join(devicePath, "partition")); err == nil {
		if resolved, resolveErr := filepath.EvalSymlinks(devicePath); resolveErr == nil {
			parent := filepath.Base(filepath.Dir(resolved))
			if parent != "" && parent != "block" && parent != filepath.Base(sysClass) {
				markMountedBlockDevice(sysClass, parent, mounted)
			}
		}
	}
	if slaves, err := os.ReadDir(filepath.Join(devicePath, "slaves")); err == nil {
		for _, slave := range slaves {
			markMountedBlockDevice(sysClass, slave.Name(), mounted)
		}
	}
}

func unmountedExternalDisks(sysClass string, mounted map[string]bool) []model.Disk {
	entries, err := os.ReadDir(sysClass)
	if err != nil {
		return nil
	}
	var disks []model.Disk
	for _, entry := range entries {
		name := entry.Name()
		devicePath := filepath.Join(sysClass, name)
		if _, err := os.Stat(filepath.Join(devicePath, "partition")); err == nil {
			continue
		}
		if mounted[name] || !isExternalBlockDevice(devicePath) {
			continue
		}
		rawSize, err := os.ReadFile(filepath.Join(devicePath, "size"))
		if err != nil {
			continue
		}
		sectors, err := strconv.ParseUint(strings.TrimSpace(string(rawSize)), 10, 64)
		if err != nil || sectors == 0 {
			continue
		}
		disks = append(disks, model.Disk{
			MountPoint: "/dev/" + name,
			Filesystem: "unmounted",
			TotalBytes: sectors * 512,
		})
	}
	return disks
}

func isExternalBlockDevice(devicePath string) bool {
	if removable, err := os.ReadFile(filepath.Join(devicePath, "removable")); err == nil && strings.TrimSpace(string(removable)) == "1" {
		return true
	}
	resolved, err := filepath.EvalSymlinks(devicePath)
	return err == nil && strings.Contains(filepath.ToSlash(resolved), "/usb")
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
