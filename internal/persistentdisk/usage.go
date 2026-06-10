package persistentdisk

import (
	"fmt"
	"syscall"
)

const (
	// DefaultCapBytes is the platform default mutable data image size (8 GiB).
	DefaultCapBytes = 8192 * 1024 * 1024
	// WarningUsedBytes warns when guest persistent usage crosses ~7 GiB.
	WarningUsedBytes = 7 * 1024 * 1024 * 1024
	// CriticalAvailBytes warns when free space drops below 512 MiB.
	CriticalAvailBytes = 512 * 1024 * 1024
)

// Usage reports filesystem capacity for a persistent root (for example /mnt/persistent).
type Usage struct {
	TotalBytes uint64 `json:"total_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
	AvailBytes uint64 `json:"avail_bytes"`
}

// Status is the operator-facing disk summary shared by runtime health and compute monitor.
type Status struct {
	Source          string  `json:"source"`
	UsedBytes       uint64  `json:"used_bytes"`
	TotalBytes      uint64  `json:"total_bytes"`
	AvailBytes      uint64  `json:"avail_bytes"`
	CapBytes        uint64  `json:"cap_bytes"`
	UsedPercent     float64 `json:"used_percent"`
	Warning         bool    `json:"warning"`
	Critical        bool    `json:"critical"`
	DefaultCapBytes uint64  `json:"default_cap_bytes"`
}

// Statfs reports usage for the filesystem containing dir.
func Statfs(dir string) (Usage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return Usage{}, fmt.Errorf("statfs %s: %w", dir, err)
	}
	blockSize := uint64(stat.Bsize)
	total := stat.Blocks * blockSize
	avail := stat.Bavail * blockSize
	if total < avail {
		return Usage{}, fmt.Errorf("statfs %s: invalid block accounting", dir)
	}
	return Usage{
		TotalBytes: total,
		UsedBytes:  total - avail,
		AvailBytes: avail,
	}, nil
}

// UsedPercent returns used/total as a 0-100 percentage.
func UsedPercent(usage Usage) float64 {
	if usage.TotalBytes == 0 {
		return 0
	}
	return (float64(usage.UsedBytes) / float64(usage.TotalBytes)) * 100
}

// Warning reports high-water usage before the default cap is exhausted.
func Warning(usage Usage) bool {
	return usage.UsedBytes >= WarningUsedBytes
}

// Critical reports dangerously low free space or very high utilization.
func Critical(usage Usage) bool {
	if usage.AvailBytes > 0 && usage.AvailBytes <= CriticalAvailBytes {
		return true
	}
	if usage.TotalBytes == 0 {
		return false
	}
	return float64(usage.UsedBytes)/float64(usage.TotalBytes) >= 0.875
}

// StatusFromGuestUsage maps guest statfs usage into the shared status shape.
func StatusFromGuestUsage(usage Usage) Status {
	capBytes := usage.TotalBytes
	if capBytes == 0 {
		capBytes = DefaultCapBytes
	}
	return Status{
		Source:          "guest",
		UsedBytes:       usage.UsedBytes,
		TotalBytes:      usage.TotalBytes,
		AvailBytes:      usage.AvailBytes,
		CapBytes:        capBytes,
		UsedPercent:     UsedPercent(usage),
		Warning:         Warning(usage),
		Critical:        Critical(usage),
		DefaultCapBytes: DefaultCapBytes,
	}
}

// StatusFromHostImage maps host-side data.img allocation into the shared status shape.
func StatusFromHostImage(fileBytes, capBytes uint64) Status {
	if capBytes == 0 {
		capBytes = DefaultCapBytes
	}
	usage := Usage{
		TotalBytes: capBytes,
		UsedBytes:  fileBytes,
	}
	if capBytes > fileBytes {
		usage.AvailBytes = capBytes - fileBytes
	}
	return Status{
		Source:          "host",
		UsedBytes:       fileBytes,
		TotalBytes:      capBytes,
		AvailBytes:      usage.AvailBytes,
		CapBytes:        capBytes,
		UsedPercent:     UsedPercent(usage),
		Warning:         Warning(usage),
		Critical:        Critical(usage),
		DefaultCapBytes: DefaultCapBytes,
	}
}
