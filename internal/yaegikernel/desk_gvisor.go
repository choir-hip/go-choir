package yaegikernel

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// gVisor for every desk worker (MA6.4, owner rulings R39 and R40;
// docs/design/modular-architecture-plan-2026-11-04.md row MA6.4, measured in
// docs/evidence/gvisor-desk-capsule-x86-2026-11-04.md and
// docs/evidence/gvisor-arm64-mac-guest-2026-11-04.md).
//
// The spawner starts each desk worker inside a gVisor sandbox (runsc outer,
// systrap platform, no KVM, --network=none). One sandbox serves every worker
// of a trust class (desk_trust_class.go): A first-party custodians, B
// first-party desks that read untrusted content, C one per other publisher.
//
//   - a sandbox is a runsc container whose init is the worker binary in its
//     pause role (deskSandboxRoleEnv=init), started by the daemon in the
//     foreground with a parent-death signal, so it dies with the daemon (a
//     restart never inherits a sandbox; the state root is swept at the first
//     use after start). It is stopped when its last worker is reaped;
//   - a worker is `runsc exec --process <spec> --pass-fd 3:3` into its class's
//     sandbox: the fd 3 socketpair crosses unchanged (gVisor donates the host
//     descriptor to the same number inside), the slot token rides the
//     process spec's environment (a 0600 file in a root-only directory, never
//     argv), and the worker runs as its activation's own uid inside the
//     sandbox with no capabilities and no_new_privs;
//   - inside, the worker loads its seccomp filter and rlimits as before.
//     gVisor's filesystem view replaces Landlock and the MA6.2 mount
//     namespace: the sandbox root is an empty read-only directory holding the
//     worker binary's own closure (read-only), the time zone database
//     (read-only), an empty read-only /work (the cell's allowed root), a
//     memory /tmp, an empty read-only /sys, and gVisor's own /proc and /dev.
//     Each worker makes its own 0700 tmp dir in the sandbox's /tmp: nothing
//     a cell writes reaches a host disk, and the spawner reads a self-test
//     report through the inspect role, inside;
//   - the sandbox runs in its own child of the desks cgroup (s-<key>) whose
//     memory.max is the sandbox overhead plus the budgets of the workers it
//     has served, so a class can never take more than its desks' budgets.
//
// "required" confinement mode means gVisor-confined. Where gVisor is
// unavailable (no runsc, or the sandbox does not start), first-party desks run
// under the process confinement of MA6.2 and every other desk is refused: a
// correctness refusal, not a switch. There is no setting that turns gVisor
// off; reverting to the process-only profile is an owner act (a revert),
// because it lowers isolation.

// RunscPathEnv names the runsc binary. The guest image sets it to the pinned
// store path (nix/gvisor.nix); without it the spawner looks in the system
// profile and on PATH. It selects a binary, not a policy: an unset or wrong
// path makes gVisor unavailable, which refuses non-first-party desks and
// fails the isolation self-test in required mode.
const RunscPathEnv = "CHOIR_RUNSC"

// deskSandboxRoleEnv selects one of the worker binary's sandbox roles: "init"
// (the sandbox's pause process) or "inspect" (the spawner's reader of a
// worker's /proc status from inside the sandbox). Set only in process specs
// the spawner writes; a cell cannot set its worker's environment.
const (
	deskSandboxRoleEnv       = "CHOIR_DESK_SANDBOX_ROLE"
	deskSandboxInspectPIDEnv = "CHOIR_DESK_SANDBOX_INSPECT_PID"
	// The inspect role may also read one file (the self-test report) or
	// write one (the self-test canary); it runs as the worker's uid then.
	deskSandboxInspectReadEnv  = "CHOIR_DESK_SANDBOX_INSPECT_READ"
	deskSandboxInspectWriteEnv = "CHOIR_DESK_SANDBOX_INSPECT_WRITE"
	deskSandboxRoleInit        = "init"
	deskSandboxRoleInspect     = "inspect"
	// The cleanup role removes a dead worker's tmp dir from the sandbox's
	// /tmp, as that worker's uid (the dir is 0700).
	deskSandboxRoleCleanup = "cleanup"
	deskSandboxCleanupEnv  = "CHOIR_DESK_SANDBOX_CLEANUP"
)

// cellSandboxGvisor is the worker-side level for a worker inside gVisor.
const cellSandboxGvisor cellSandboxLevel = "gvisor"

// In-sandbox paths.
const (
	sandboxWorkDir     = "/work"
	sandboxZoneinfoDir = "/zoneinfo"
	// gvisorMarkerFile exists only inside a gVisor sandbox started with
	// --gvisor-marker-file: procfs on a real kernel cannot hold it.
	gvisorMarkerFile = "/proc/gvisor/kernel_is_gvisor"
)

// GvisorSandboxReceipt is one live sandbox as the spawner reads it back.
type GvisorSandboxReceipt struct {
	Key       string `json:"key"`
	Class     string `json:"class"`
	ID        string `json:"id"`
	StartedAt string `json:"started_at"`
	Workers   int    `json:"workers"`
	// Served counts every worker the sandbox has run.
	Served int `json:"served"`
	// HostPIDs are the sandbox's host processes (runsc, gofer, sentry and
	// helpers); RSSBytes sums their VmRSS and PSSBytes their Pss where the
	// spawner can read smaps_rollup.
	HostPIDs []int  `json:"host_pids,omitempty"`
	RSSBytes uint64 `json:"rss_bytes"`
	PSSBytes uint64 `json:"pss_bytes,omitempty"`
	// Cgroup is the sandbox's desks-cgroup child, with its memory.max and
	// memory.current.
	Cgroup        string `json:"cgroup,omitempty"`
	MemoryMax     string `json:"memory_max,omitempty"`
	MemoryCurrent string `json:"memory_current,omitempty"`
}

// GvisorStatus is the spawner's gVisor state: whether workers can run under
// it, the runsc it uses, and every live sandbox.
type GvisorStatus struct {
	Available bool                   `json:"available"`
	Runsc     string                 `json:"runsc,omitempty"`
	Version   string                 `json:"version,omitempty"`
	Platform  string                 `json:"platform,omitempty"`
	Rootless  bool                   `json:"rootless"`
	Reason    string                 `json:"reason,omitempty"`
	Sandboxes []GvisorSandboxReceipt `json:"sandboxes"`
	// CommittedBytes is the desks memory claimed by live sandboxes and
	// workers; Waiting counts spawns queued for admission.
	CommittedBytes uint64 `json:"committed_bytes"`
	Waiting        int    `json:"waiting"`
}

// AdmissionQueued is how long the worker's spawn waited for desks memory
// (0 for a worker admitted at once), sandboxed or process-confined.
func (w *DeskSessionWorker) AdmissionQueued() time.Duration { return w.queued }

// ConfinementProfile names how the worker is confined: "gvisor" in its
// trust class's sandbox, else "process".
func (w *DeskSessionWorker) ConfinementProfile() string {
	if w.gvisor != nil {
		return "gvisor"
	}
	return "process"
}

// sandboxInspectReport is what the inspect role prints.
type sandboxInspectReport struct {
	Status       map[string]string `json:"status,omitempty"`
	GvisorMarker bool              `json:"gvisor_marker"`
	File         string            `json:"file,omitempty"`
	FileError    string            `json:"file_error,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// RunDeskSandboxRoleIfRequested runs the worker binary's sandbox role when
// the spawner's process spec asked for one, and reports whether it did (and
// the exit code). The worker entrypoint calls it first.
func RunDeskSandboxRoleIfRequested() (int, bool) {
	switch os.Getenv(deskSandboxRoleEnv) {
	case deskSandboxRoleInit:
		// The sandbox's init: it holds the container open for the workers
		// exec'd into it and does nothing else. gVisor reaps their exits.
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
		<-c
		return 0, true
	case deskSandboxRoleInspect:
		rep := sandboxInspectReport{}
		pid, err := strconv.Atoi(os.Getenv(deskSandboxInspectPIDEnv))
		if err != nil || pid <= 0 {
			rep.Error = "no pid to inspect"
		} else if rep.Status = procStatusFields(pid); rep.Status == nil {
			rep.Error = fmt.Sprintf("no /proc/%d/status", pid)
		}
		_, err = os.Stat(gvisorMarkerFile)
		rep.GvisorMarker = err == nil
		if path := os.Getenv(deskSandboxInspectReadEnv); path != "" {
			if b, err := os.ReadFile(path); err != nil {
				rep.FileError = err.Error()
			} else {
				rep.File = string(b)
			}
		}
		if path := os.Getenv(deskSandboxInspectWriteEnv); path != "" {
			if err := os.WriteFile(path, []byte("another desk's file\n"), 0o644); err != nil {
				rep.FileError = err.Error()
			} else if err := os.Chmod(path, 0o644); err != nil {
				rep.FileError = err.Error()
			}
		}
		raw, _ := json.Marshal(rep)
		fmt.Println(string(raw))
		return 0, true
	case deskSandboxRoleCleanup:
		dir := os.Getenv(deskSandboxCleanupEnv)
		if !strings.HasPrefix(dir, "/tmp/choir-cell-") || strings.Contains(dir, "..") {
			fmt.Fprintln(os.Stderr, "cleanup: not a cell dir")
			return 2, true
		}
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, "cleanup:", err)
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// gvisorAddressOverheadBytes is what a desk worker maps beyond the heap
// it may hold, by architecture: its RLIMIT_AS is its heap share plus this.
// A worker in a shared sandbox has no cgroup of its own, so RLIMIT_AS is its
// runaway stop (gVisor enforces it, not RLIMIT_DATA): a runaway cell ends
// its own worker ("out of memory"), never its class-mates. The memory bound
// proper is the desks-memory admission and the sandbox cgroups.
//
// Most of it is reservation, not memory: the Go runtime's page-allocator
// and scavenger indexes (about 1.2 GiB, fixed), each thread's stack
// (RLIMIT_STACK, set to 2 MiB in the process spec) and glibc's malloc
// arenas (64 MiB each; MALLOC_ARENA_MAX=1 in the worker's environment, or
// the reservation swings with the thread count). Calibrated in gVisor
// 20261102.0 with Go 1.26 (2026-11-06, MA6.4 r2): a worker holding live
// 8 MiB slabs until it dies holds at least 90% of its share at shares 96,
// 124, 256 and 512 MiB on both architectures (design doc, calibration
// table; TestGvisorAddressCalibration re-measures it).
var gvisorAddressOverheadBytes = func() uint64 {
	if runtime.GOARCH == "arm64" {
		return 1408 << 20
	}
	return 1440 << 20
}()

// gvisorAddressOverheadMaxBytes is the largest overhead on any
// architecture, for judges that read a receipt from either.
const gvisorAddressOverheadMaxBytes = 1440 << 20

// gvisorAddressLimit is a sandboxed worker's RLIMIT_AS: its heap share (its
// desk's budget, or less where the desks cgroup is small; see
// workerShare) plus the Go runtime's own mappings, never above the
// spawner's cap (0 for none).
func gvisorAddressLimit(cap, budget uint64) uint64 {
	if budget == 0 {
		return cap
	}
	as := budget + gvisorAddressOverheadBytes
	if cap != 0 && cap < as {
		return cap
	}
	return as
}

var (
	deskMemoryPressureMu sync.Mutex
	deskMemoryPressure   func() bool
)

// SetDeskMemoryPressureHandler names what the spawner calls while a spawn
// waits for desks memory: it ends one idle desk session (the least recently
// used) and reports whether it did, so an idle worker's claim goes to a
// waiting one instead of holding it for its retention. Nil clears it.
func SetDeskMemoryPressureHandler(f func() bool) {
	deskMemoryPressureMu.Lock()
	deskMemoryPressure = f
	deskMemoryPressureMu.Unlock()
}

func relieveDeskMemoryPressure() bool {
	deskMemoryPressureMu.Lock()
	f := deskMemoryPressure
	deskMemoryPressureMu.Unlock()
	return f != nil && f()
}
