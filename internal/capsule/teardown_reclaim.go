package capsule

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Teardown reclaim (docs/design/visual-evidence-slice-2026-10-15.md part 5,
// V27 and V28). When a capsule's cgroup is empty and about to be removed, the
// page cache it still holds (a browser run leaves 0.19 to 0.43 GiB: the
// binary, fonts, written files) would be reparented to the parent cgroup by
// the removal, and the guest never frees clean cache on its own, so free page
// reporting would never return it to the host
// (docs/reports/playwright-in-capsules-memory-2026-10-15.md section 1.1).
// Writing the cgroup's usage to memory.reclaim drops it first.

// teardownReclaimEnv is the control-run switch: "off" leaves the cache to be
// reparented, as before this slice.
const teardownReclaimEnv = "CHOIR_CAPSULE_TEARDOWN_RECLAIM"

func teardownReclaimEnabled() bool {
	return strings.TrimSpace(strings.ToLower(os.Getenv(teardownReclaimEnv))) != "off"
}

// cgroupMemoryStats is the optional extra a cgroup directory offers for the
// log line: memory.peak and the file and anon charge.
type cgroupMemoryStats interface {
	MemoryStats() (peak, file, anon int64)
}

// MemoryStats reads memory.peak and memory.stat's file and anon; -1 for a
// value that cannot be read (memory.peak needs Linux 5.19).
func (d cgroupMemoryDir) MemoryStats() (peak, file, anon int64) {
	peak = -1
	if raw, err := os.ReadFile(filepath.Join(string(d), "memory.peak")); err == nil {
		if v, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64); err == nil {
			peak = v
		}
	}
	file, anon = -1, -1
	if raw, err := os.ReadFile(filepath.Join(string(d), "memory.stat")); err == nil {
		file, anon = parseMemoryStatFileAnon(string(raw))
	}
	return peak, file, anon
}

func parseMemoryStatFileAnon(stat string) (file, anon int64) {
	file, anon = -1, -1
	for _, line := range strings.Split(stat, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		v, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "file":
			file = v
		case "anon":
			anon = v
		}
	}
	return file, anon
}

// teardownReclaim asks the kernel to reclaim everything an emptied capsule
// cgroup is charged for and returns a one-line report for the guest log. A
// partial reclaim (EAGAIN) is not an error; a failure never blocks teardown.
// It is one synchronous write of clean cache and runs under the executor
// lock (V28); its duration is in the report.
func teardownReclaim(cg cgroupMemory) string {
	if cg == nil {
		return ""
	}
	before, err := cg.Current()
	if err != nil {
		return "memory.current unreadable: " + err.Error()
	}
	if before <= 0 {
		return "nothing charged to reclaim"
	}
	var peak, fileBefore, anonBefore int64 = -1, -1, -1
	stats, hasStats := cg.(cgroupMemoryStats)
	if hasStats {
		peak, fileBefore, anonBefore = stats.MemoryStats()
	}
	start := time.Now()
	reclaimErr := cg.Reclaim(before)
	took := time.Since(start)
	result := "ok"
	if reclaimErr != nil {
		if errors.Is(reclaimErr, syscall.EAGAIN) {
			result = "partial"
		} else {
			result = "error: " + reclaimErr.Error()
		}
	}
	after, err := cg.Current()
	if err != nil {
		after = -1
	}
	report := fmt.Sprintf("current=%dMiB->%dMiB reclaim=%s in %s", before>>20, after>>20, result, took.Round(time.Millisecond))
	if hasStats {
		_, fileAfter, anonAfter := stats.MemoryStats()
		report += fmt.Sprintf(" peak=%dMiB file=%dMiB->%dMiB anon=%dMiB->%dMiB", peak>>20, fileBefore>>20, fileAfter>>20, anonBefore>>20, anonAfter>>20)
	}
	return report
}
