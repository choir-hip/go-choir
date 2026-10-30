// Package gomemlimit sets a soft Go memory limit for the guest daemon and its
// desk-session workers, derived from guest RAM.
//
// Why (docs/reports/lr-guest-memory-return-2026-10-14.md): nothing bounds
// memory inside a guest. Without a limit the Go GC lets a heap grow to twice
// its live size before collecting, and every page the guest touches is held
// by the host (free page reporting returns only what the guest frees). A soft
// limit makes the GC collect, and the scavenger return memory to the kernel,
// once the heap nears it. It never kills: past the limit the GC works harder,
// capped by the runtime near 50% CPU. The hard bound for a worker stays its
// RLIMIT_AS.
//
// GOMEMLIMIT in the environment always wins (GOMEMLIMIT=off is the rollback).
package gomemlimit

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

// Role names the process whose limit is computed.
type Role string

const (
	RoleDaemon     Role = "daemon"
	RoleDeskWorker Role = "desk-worker"
)

const (
	mib = 1 << 20

	// The daemon gets half of guest RAM, at least 512 MiB. The paper
	// computer's daemon measured Sys 1.46 GB / HeapInuse 1.05 GB on an 8 GiB
	// guest, so 4 GiB is about 2.7x its footprint: no GC thrash at today's
	// working set. It binds only when the heap would pass half the guest,
	// where the page cache and workers are squeezed.
	daemonDivisor = 2
	daemonFloor   = 512 * mib

	// Each desk worker gets 1/16 of guest RAM, at least 256 MiB (512 MiB on
	// 8 GiB). About 16 workers live at once (12 idle plus busy research
	// children) and together sum to one guest. Not measured yet.
	workerDivisor = 16
	workerFloor   = 256 * mib

	// WorkerOverrideEnv in the daemon's environment is handed to each worker
	// as GOMEMLIMIT. Workers get a clean environment, so this is the only
	// way to change (or turn off: "off") their limit.
	WorkerOverrideEnv = "CHOIR_DESK_WORKER_GOMEMLIMIT"
	// workerOverrideMin refuses tiny worker limits: GOMEMLIMIT=0 is valid to
	// the runtime and makes the GC run without pause.
	workerOverrideMin = 64 * mib

	meminfoPath = "/proc/meminfo"
)

// Decision is what Apply did, for the boot log.
type Decision struct {
	Role             Role
	MemTotalBytes    int64
	LimitBytes       int64 // the soft limit applied; 0 when none was applied
	WorkerLimitBytes int64 // daemon only: the limit each desk worker computes
	Skipped          string
}

func (d Decision) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "role=%s mem_total_mib=%d", d.Role, d.MemTotalBytes/mib)
	if d.Skipped != "" {
		fmt.Fprintf(&b, " limit=none reason=%q", d.Skipped)
	} else {
		fmt.Fprintf(&b, " limit_mib=%d", d.LimitBytes/mib)
	}
	if d.WorkerLimitBytes > 0 {
		fmt.Fprintf(&b, " desk_worker_limit_mib=%d", d.WorkerLimitBytes/mib)
	}
	return b.String()
}

// ApplyDaemon sets the daemon's soft limit.
func ApplyDaemon() Decision {
	return apply(RoleDaemon, os.Getenv, os.ReadFile, setMemoryLimit)
}

// ApplyDeskWorker sets a desk-session worker's soft limit.
func ApplyDeskWorker() Decision {
	return apply(RoleDeskWorker, os.Getenv, os.ReadFile, setMemoryLimit)
}

// WorkerEnv returns the environment to add to a desk worker so an operator
// override in the daemon's environment reaches it, or nil.
func WorkerEnv(getenv func(string) string) []string {
	v := strings.TrimSpace(getenv(WorkerOverrideEnv))
	if !ValidGOMEMLIMIT(v) {
		return nil
	}
	if v != "off" {
		if n, ok := parseByteCount(v); !ok || n < workerOverrideMin {
			return nil
		}
	}
	return []string{"GOMEMLIMIT=" + v}
}

// MemTotal reads the machine's total memory from /proc/meminfo.
func MemTotal() (int64, error) {
	data, err := os.ReadFile(meminfoPath)
	if err != nil {
		return 0, err
	}
	return parseMemTotal(data)
}

// LimitFor is the soft limit for a role on a machine of memTotal bytes, or 0
// when memTotal is unknown. A floor never exceeds the machine.
func LimitFor(role Role, memTotal int64) int64 {
	if memTotal <= 0 {
		return 0
	}
	divisor, floor := int64(daemonDivisor), int64(daemonFloor)
	if role == RoleDeskWorker {
		divisor, floor = workerDivisor, int64(workerFloor)
	}
	limit := memTotal / divisor
	if limit < floor {
		limit = floor
	}
	if limit > memTotal {
		limit = memTotal
	}
	return limit
}

// ValidGOMEMLIMIT reports whether the Go runtime accepts v as GOMEMLIMIT. A
// value it does not accept makes the runtime refuse to start.
func ValidGOMEMLIMIT(v string) bool {
	if v == "off" {
		return true
	}
	_, ok := parseByteCount(v)
	return ok
}

var setMemoryLimit = debug.SetMemoryLimit

func apply(role Role, getenv func(string) string, readFile func(string) ([]byte, error), setLimit func(int64) int64) Decision {
	d := Decision{Role: role}
	if data, err := readFile(meminfoPath); err != nil {
		d.Skipped = "meminfo unavailable"
	} else if total, err := parseMemTotal(data); err != nil {
		d.Skipped = err.Error()
	} else {
		d.MemTotalBytes = total
	}
	if v := getenv("GOMEMLIMIT"); v != "" {
		// The runtime already applied it at start.
		d.Skipped = "GOMEMLIMIT set in environment"
	}
	if d.Skipped == "" {
		d.LimitBytes = LimitFor(role, d.MemTotalBytes)
	}
	if role == RoleDaemon {
		d.WorkerLimitBytes = LimitFor(RoleDeskWorker, d.MemTotalBytes)
	}

	if d.LimitBytes > 0 {
		setLimit(d.LimitBytes)
	}
	return d
}

func parseMemTotal(data []byte) (int64, error) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 3 || fields[0] != "MemTotal:" {
			continue
		}
		if fields[2] != "kB" {
			return 0, fmt.Errorf("meminfo MemTotal unit %q", fields[2])
		}
		kib, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || kib <= 0 {
			return 0, fmt.Errorf("meminfo MemTotal %q", fields[1])
		}
		return kib * 1024, nil
	}
	return 0, fmt.Errorf("meminfo has no MemTotal")
}

// parseByteCount mirrors the Go runtime's GOMEMLIMIT parser: a non-negative
// integer with an optional B, KiB, MiB, GiB or TiB suffix, no spaces.
func parseByteCount(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	units := []struct {
		suffix string
		shift  uint
	}{{"KiB", 10}, {"MiB", 20}, {"GiB", 30}, {"TiB", 40}, {"B", 0}}
	shift := uint(0)
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			s, shift = strings.TrimSuffix(s, u.suffix), u.shift
			break
		}
	}
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	if shift > 0 && n > math.MaxInt64>>shift {
		return 0, false
	}
	return n << shift, true
}
