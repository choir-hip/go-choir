package yaegikernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Desk cell self-test: the staging proof of desk cell confinement (F4,
// docs/problems/desk-boundary-residuals-2026-10-21.md).
//
// Local tests cannot prove Landlock (the CI and Docker kernels have none), and
// a guest is never reached by SSH. So the guest runtime can, on an owner's
// request, spawn one worker per desk through the production spawner
// (SpawnDeskSessionWorker, the same confinement plan and entrypoint as
// desk_go_eval) with one extra environment entry. That worker confines
// itself exactly as a production worker does, then, before it serves its
// first cell, acts as an escaped cell would: it opens sockets, execs, forks,
// signals, reads files outside its grants (including another desk's tmp
// dir) and writes outside its own tmp dir. It writes what happened to
// selftest.json in its own tmp dir and then serves cells as usual, so the
// spawner also proves the confined worker still runs a plain cell.
//
// The spawner reads that file (as root) and the worker's /proc status and
// namespaces from outside, which the worker cannot forge. Production desk
// workers never run the probes: only this spawner sets the environment entry,
// and a cell cannot set its worker's environment.
//
// Ways this could mislead (written first):
//  1. a probe "fails" for an unrelated reason (ENOENT on a missing path, a
//     directory read as a file) and reads as a denial: read targets are
//     stat'ed by the spawner first and only existing ones are probed; reads
//     open, then read a byte or a directory entry; ENOENT never counts as a
//     denial;
//  2. the worker reports on itself after compromise: probes run before the
//     first cell, and identity, capabilities, no_new_privs, seccomp and the
//     namespaces are read by the spawner from /proc;
//  3. Landlock is absent and DAC alone denies the reads: /etc/passwd is
//     world-readable, so only Landlock refuses it; the spawner records the
//     kernel's Landlock ABI;
//  4. another desk's files are unreadable only because they do not exist:
//     the spawner writes a world-readable canary into the previous desk's
//     live tmp dir before the next desk's worker starts;
//  5. the probes leave a worker changed (a new user namespace) and it serves
//     a real cell: self-test workers serve one check cell and are closed;
//  6. the self-test hides a regression by judging on the server only: the
//     receipt carries the raw facts, and the staging script judges them
//     again (scripts/desk_cell_confinement_probe.py).

const (
	cellSelfTestEnv     = "SELFTEST"
	cellSelfTestReadEnv = "SELFTEST_READ"
	cellSelfTestFile    = "selftest.json"
	// deskCellSelfTestCanary is written by the spawner into a live desk
	// worker's tmp dir, world-readable, for the next desk's worker to try.
	deskCellSelfTestCanary = "selftest-canary"
)

// DeskCellSelfTestRoles are the desks whose cells run in session workers.
var DeskCellSelfTestRoles = deskmanifest.CellDeskNames()

// DeskCellSelfTestProbes is the worker-side report: probe name to "" when the
// operation succeeded, else "ERRNO: message".
type DeskCellSelfTestProbes map[string]string

// Probe names the judge expects. Allowed probes must succeed; denied probes
// must fail, with EPERM from the cell seccomp filter except write_etc; read
// probes ("read:<path>") must fail, and never with ENOENT.
var (
	deskCellAllowedProbes = []string{"write_own_tmp", "list_allowed_root"}
	deskCellDeniedProbes  = []string{
		"socket_inet_stream", "socket_inet_dgram", "socket_inet6_stream", "socket_unix_stream",
		"socket_netlink", "socket_packet_raw", "socket_inet_raw",
		"fork_exec", "execve", "kill_pid1", "kill_parent", "setuid_root", "unshare_user", "unshare_net",
		"write_etc",
		// MA6.2: the classes a red team named, denied by name.
		"io_uring_setup", "keyctl", "bpf", "perf_event_open", "userfaultfd", "ptrace", "clone_newns", "clone_newuser", "mount",
	}
	// deskCellMountProbes must fail with any errno when the worker has its
	// own mount namespace: the paths are not there, which is stronger than
	// a refusal.
	deskCellMountProbes = []string{"proc_meminfo", "proc_pid1_environ", "proc_parent_status", "sys_kernel", "dev_mem", "dev_extra_entries"}
	// deskCellLandlockProbes are denied only by Landlock (DAC allows them:
	// the allowed root is the guest's world-writable /tmp). Judged whenever
	// the kernel has Landlock, which required mode demands.
	deskCellLandlockProbes = []string{"write_allowed_root"}
)

// DeskCellSelfTestWorker is one desk worker's receipt.
type DeskCellSelfTestWorker struct {
	Role string `json:"role"`
	// User is the cell user the spawner planned (empty for a non-root
	// spawner in auto mode, which cannot switch identity).
	User    string `json:"user,omitempty"`
	UserUID string `json:"user_uid,omitempty"`
	PID     int    `json:"pid,omitempty"`
	// Status is the worker's /proc/<pid>/status as the spawner read it.
	Status map[string]string `json:"status,omitempty"`
	// NetNSIsolated / IPCNSIsolated: the worker's namespace differs from the
	// spawner's.
	NetNSIsolated bool `json:"net_ns_isolated"`
	IPCNSIsolated bool `json:"ipc_ns_isolated"`
	// MountNSIsolated: the worker's mount namespace differs from the
	// spawner's (MA6.2).
	MountNSIsolated bool `json:"mount_ns_isolated"`
	// Probes is the worker's own report, written before its first cell.
	Probes DeskCellSelfTestProbes `json:"probes,omitempty"`
	// ReadTargets are the paths the worker was asked to read (each exists).
	ReadTargets []string `json:"read_targets,omitempty"`
	// CellOutput is the check cell's stdout after the probes.
	CellOutput string `json:"cell_output,omitempty"`
	CellServes bool   `json:"cell_serves"`
	// FanOut is the interpreted goroutine fan-out probe (desk_cell_fanout.go).
	FanOut *DeskCellFanOut `json:"fanout,omitempty"`
	Error  string          `json:"error,omitempty"`
	// Profile is "gvisor" for a worker in its trust class's sandbox (MA6.4)
	// and "process" for one under process confinement. For a gVisor worker
	// Status was read inside the sandbox by the spawner's inspect role, and
	// GvisorMarker is whether that sandbox holds gVisor's marker file.
	Profile      string `json:"profile"`
	SandboxKey   string `json:"sandbox_key,omitempty"`
	SandboxID    string `json:"sandbox_id,omitempty"`
	GvisorMarker bool   `json:"gvisor_marker,omitempty"`
	// CellP50Micros is the median wall time of deskCellTimingCells trivial
	// cells after the check cell, as the spawner sees it.
	CellP50Micros int64 `json:"cell_p50_us,omitempty"`
	// QueuedMS is how long the worker's spawn waited for desks memory.
	QueuedMS int64 `json:"queued_ms"`
}

// deskCellTimingCells is how many trivial cells the self-test times per worker.
const deskCellTimingCells = 20

// DeskCellSelfTestReceipt is the whole self-test.
type DeskCellSelfTestReceipt struct {
	Kind        string                   `json:"kind"`
	StartedAt   string                   `json:"started_at"`
	Mode        string                   `json:"mode"`
	SpawnerUID  int                      `json:"spawner_uid"`
	LandlockABI int                      `json:"landlock_abi"`
	AllowedRoot string                   `json:"allowed_root"`
	Workers     []DeskCellSelfTestWorker `json:"workers"`
	// Gvisor is the spawner's gVisor state read while the workers were
	// live: one sandbox per trust class, with its host processes and their
	// resident memory (MA6.4).
	Gvisor   *GvisorStatus `json:"gvisor,omitempty"`
	OK       bool          `json:"ok"`
	Failures []string      `json:"failures,omitempty"`
}

// RunDeskCellSelfTestIfRequested is the worker half. The worker entrypoint
// calls it after ConfineDeskSessionWorker and before it serves; it does
// nothing unless the self-test spawner asked for it.
func RunDeskCellSelfTestIfRequested(getenv func(string) string, prefix string) {
	if getenv == nil {
		getenv = os.Getenv
	}
	prefix = sessionEnvPrefix(prefix)
	if getenv(prefix+cellSelfTestEnv) != "1" {
		return
	}
	tmp := getenv(prefix + cellTmpDirEnv)
	if tmp == "" {
		return
	}
	var reads []string
	for _, p := range filepath.SplitList(getenv(prefix + cellSelfTestReadEnv)) {
		if p = strings.TrimSpace(p); p != "" {
			reads = append(reads, p)
		}
	}
	probes := runDeskCellProbes(getenv(prefix+"ALLOWED_ROOT"), tmp, reads)
	raw, _ := json.Marshal(probes)
	// The worker's own tmp dir is the one place it may write.
	_ = os.WriteFile(filepath.Join(tmp, cellSelfTestFile), raw, 0o600)
}

// probeErr renders an error with its errno name, "" for success.
func probeErr(err error) string {
	if err == nil {
		return ""
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errnoName(errno) + ": " + err.Error()
	}
	return "ERROR: " + err.Error()
}

func errnoName(e syscall.Errno) string {
	switch e {
	case syscall.EPERM:
		return "EPERM"
	case syscall.EACCES:
		return "EACCES"
	case syscall.ENOENT:
		return "ENOENT"
	case syscall.ENOSYS:
		return "ENOSYS"
	case syscall.EROFS:
		return "EROFS"
	case syscall.EAFNOSUPPORT:
		return "EAFNOSUPPORT"
	case syscall.EINVAL:
		return "EINVAL"
	}
	return fmt.Sprintf("ERRNO%d", int(e))
}

// probeRead opens path and reads one byte or one directory entry.
func probeRead(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		_, err = f.Readdirnames(1)
	} else {
		_, err = f.Read(make([]byte, 1))
	}
	if err != nil && err.Error() == "EOF" {
		return nil
	}
	return err
}

// DeskCellSelfTestConfig is what the spawner needs beyond a production
// worker config.
type DeskCellSelfTestConfig struct {
	// Base is the production worker config (desk_go_eval's); Role is set per
	// worker.
	Base DeskSessionWorkerConfig
	// Roles defaults to DeskCellSelfTestRoles.
	Roles []string
	// ExtraReads are paths a desk cell must not read; only existing ones
	// are probed. /etc/passwd and the previous desk's canary are always
	// added.
	ExtraReads []string
	// CheckCell is the cell each worker runs after its probes; its stdout
	// must contain CheckWant.
	CheckCell string
	CheckWant string
}

// DeskCellSelfTest spawns one worker per desk through the production spawner
// and returns the receipt, judged. Workers are closed before it returns.
func DeskCellSelfTest(ctx context.Context, cfg DeskCellSelfTestConfig) DeskCellSelfTestReceipt {
	roles := cfg.Roles
	if len(roles) == 0 {
		roles = DeskCellSelfTestRoles
	}
	if cfg.CheckCell == "" {
		cfg.CheckCell = "import \"fmt\"\nfmt.Println(\"cell\", 6*7)"
		cfg.CheckWant = "cell 42"
	}
	rec := DeskCellSelfTestReceipt{
		Kind:        "desk_cell_selftest",
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
		Mode:        string(cfg.Base.Confinement.Mode),
		SpawnerUID:  os.Geteuid(),
		LandlockABI: landlockABI(),
		AllowedRoot: cfg.Base.Session.AllowedRoot,
	}
	if rec.Mode == "" {
		rec.Mode = "auto"
	}
	prefix := sessionEnvPrefix(cfg.Base.EnvPrefix)
	var live []*DeskSessionWorker
	defer func() {
		for _, w := range live {
			w.Close()
		}
	}()
	prevCanary := ""
	for _, role := range roles {
		out := DeskCellSelfTestWorker{Role: role, Profile: "process"}
		reads := existingPaths(append([]string{"/etc/passwd"}, cfg.ExtraReads...))
		if prevCanary != "" {
			// The previous desk's canary path: a class-mate of it sees the
			// sandbox's /tmp but not the 0700 dir; any other desk does not
			// see the path at all.
			reads = append(reads, prevCanary)
		}
		out.ReadTargets = reads
		wcfg := cfg.Base
		wcfg.Session.Role = role
		// The handler's deadline bounds the desks-memory wait too.
		wcfg.Context = ctx
		wcfg.ExtraEnv = append(append([]string(nil), cfg.Base.ExtraEnv...),
			prefix+cellSelfTestEnv+"=1",
			prefix+cellSelfTestReadEnv+"="+strings.Join(reads, string(os.PathListSeparator)),
		)
		w, err := SpawnDeskSessionWorker(wcfg)
		if err != nil {
			out.Error = "spawn: " + err.Error()
			rec.Workers = append(rec.Workers, out)
			prevCanary = ""
			continue
		}
		live = append(live, w)
		out.PID = w.PID()
		out.QueuedMS = w.AdmissionQueued().Milliseconds()
		if w.gvisor != nil {
			// Inside a sandbox the host's /proc shows only runsc's exec
			// client: the spawner reads the worker from inside.
			out.Profile = "gvisor"
			out.SandboxKey, out.SandboxID = w.gvisor.sandboxKeyID()
			if rep, err := w.gvisor.inspect(); err != nil {
				out.Error = "inspect in sandbox: " + err.Error()
			} else if rep.Error != "" {
				out.Error = "inspect in sandbox: " + rep.Error
			} else {
				out.Status = rep.Status
				out.GvisorMarker = rep.GvisorMarker
			}
		}
		if w.cellUID != 0 {
			out.User = fmt.Sprintf("activation uid %d (desk %s)", w.cellUID, role)
			out.UserUID = strconv.FormatUint(uint64(w.cellUID), 10)
		}
		if w.gvisor == nil {
			out.MountNSIsolated = nsDiffers(w.PID(), "mnt")
			out.Status = procStatusFields(w.PID())
			out.NetNSIsolated = nsDiffers(w.PID(), "net")
			out.IPCNSIsolated = nsDiffers(w.PID(), "ipc")
		}
		if raw, err := readCellSelfTestReport(w); err != nil {
			out.Error = "read worker report: " + err.Error()
		} else if err := json.Unmarshal(raw, &out.Probes); err != nil {
			out.Error = "decode worker report: " + err.Error()
		}
		cellCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		res, err := w.Eval(cellCtx, cfg.CheckCell)
		cancel()
		switch {
		case err != nil:
			out.CellOutput = "transport: " + err.Error()
		case res.Error != "":
			out.CellOutput = "cell error: " + res.Error
		default:
			out.CellOutput = strings.TrimSpace(res.Stdout)
			out.CellServes = strings.Contains(res.Stdout, cfg.CheckWant)
		}
		if out.CellServes {
			out.CellP50Micros = timeTrivialCells(ctx, w, deskCellTimingCells)
			out.FanOut = runDeskCellFanOut(ctx, w.Eval)
		}
		// The next desk tries to read this desk's live tmp dir.
		prevCanary = ""
		if w.gvisor != nil {
			if path, err := w.gvisor.writeCellCanary(deskCellSelfTestCanary); err == nil {
				prevCanary = path
			}
		} else if w.cellTmpDir != "" {
			canary := filepath.Join(w.cellTmpDir, deskCellSelfTestCanary)
			if err := os.WriteFile(canary, []byte("another desk's file\n"), 0o644); err == nil {
				_ = os.Chmod(canary, 0o644)
				prevCanary = canary
			}
		}
		rec.Workers = append(rec.Workers, out)
	}
	// Read back the sandboxes while every worker is live: one per class.
	gst := GvisorStatusNow()
	rec.Gvisor = &gst
	rec.Failures = JudgeDeskCellSelfTest(rec)
	rec.OK = len(rec.Failures) == 0
	return rec
}

// readCellSelfTestReport reads the worker's self-test report: from its host
// tmp dir, or for a gVisor worker from inside its sandbox.
func readCellSelfTestReport(w *DeskSessionWorker) ([]byte, error) {
	if w.gvisor != nil {
		return w.gvisor.readCellFile(cellSelfTestFile)
	}
	return os.ReadFile(filepath.Join(w.cellTmpDir, cellSelfTestFile))
}

// timeTrivialCells runs n trivial cells and returns their median wall time.
func timeTrivialCells(ctx context.Context, w *DeskSessionWorker, n int) int64 {
	var ds []time.Duration
	for i := 0; i < n; i++ {
		cellCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		start := time.Now()
		_, err := w.Eval(cellCtx, "x := 1 + 1\n_ = x")
		cancel()
		if err != nil {
			break
		}
		ds = append(ds, time.Since(start))
	}
	if len(ds) == 0 {
		return 0
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	return ds[len(ds)/2].Microseconds()
}

// JudgeDeskCellSelfTest returns every way the receipt falls short of the
// confinement contract; empty means it holds. A root spawner (the guest
// runtime) or required mode must switch identity and network namespace;
// required mode must have Landlock V5; whenever the kernel has Landlock, a
// world-readable file outside the grants must be unreadable.
func JudgeDeskCellSelfTest(rec DeskCellSelfTestReceipt) []string {
	var fails []string
	fail := func(format string, args ...any) { fails = append(fails, fmt.Sprintf(format, args...)) }
	identity := rec.SpawnerUID == 0 || rec.Mode == string(CellConfinementRequired)
	required := rec.Mode == string(CellConfinementRequired)
	processWorkers := 0
	for _, w := range rec.Workers {
		if w.Profile != "gvisor" {
			processWorkers++
		}
	}
	// MA6.4: required means gVisor-confined. A computer without gVisor
	// still runs its first-party desks under process confinement, but its
	// self-test says so.
	if required && (rec.Gvisor == nil || !rec.Gvisor.Available) {
		reason := "no gVisor state in the receipt"
		if rec.Gvisor != nil {
			reason = rec.Gvisor.Reason
		}
		fail("required mode without gVisor (%s): first-party desks run under process confinement and other desks are refused", reason)
	}
	// The fallback (process confinement) still needs Landlock V5; a receipt
	// with no gVisor worker is judged as the fallback.
	if required && (processWorkers > 0 || processWorkers == len(rec.Workers)) && rec.LandlockABI < 5 {
		fail("required mode on a kernel with Landlock ABI %d (want >= 5)", rec.LandlockABI)
	}
	if len(rec.Workers) == 0 {
		fail("no workers")
	}
	fails = append(fails, judgeSandboxSharing(rec)...)
	seenUID := map[string]bool{}
	for _, w := range rec.Workers {
		p := w.Role + ": "
		if w.Error != "" {
			fail(p+"%s", w.Error)
			continue
		}
		if required && w.Profile != "gvisor" {
			fail("%s", p+"runs under process confinement in required mode (want gVisor)")
		}
		if w.CellServes {
			if w.FanOut == nil {
				fail("%s", p+"fanout: the probe did not run")
			} else if !w.FanOut.OK {
				fail(p+"fanout: %s", w.FanOut.Error)
			}
		}
		if w.Profile == "gvisor" {
			fails = append(fails, judgeGvisorWorker(w, identity, seenUID)...)
			continue
		}
		st := w.Status
		if st["NoNewPrivs"] != "1" {
			fail(p+"NoNewPrivs=%q, want 1", st["NoNewPrivs"])
		}
		if st["Seccomp"] != "2" {
			fail(p+"Seccomp=%q, want 2", st["Seccomp"])
		}
		if identity {
			uids := strings.Fields(st["Uid"])
			if w.UserUID == "" || len(uids) != 4 || uids[0] != w.UserUID || uids[1] != w.UserUID || uids[2] != w.UserUID || uids[3] != w.UserUID {
				fail(p+"Uid=%q, want %s (%s) on all four", st["Uid"], w.User, w.UserUID)
			}
			if len(uids) > 0 && uids[0] == "0" {
				fail("%s", p+"runs as root")
			}
			for _, k := range []string{"CapEff", "CapPrm", "CapInh", "CapAmb"} {
				if strings.Trim(st[k], "0") != "" {
					fail(p+"%s=%s, want 0", k, st[k])
				}
			}
			if strings.TrimSpace(st["Groups"]) != "" {
				fail(p+"Groups=%q, want none", st["Groups"])
			}
			if !w.NetNSIsolated {
				fail("%s", p+"shares the runtime's network namespace")
			}
			if !w.IPCNSIsolated {
				fail("%s", p+"shares the runtime's IPC namespace")
			}
			if !w.MountNSIsolated {
				fail("%s", p+"shares the runtime's mount namespace")
			}
			if seenUID[w.UserUID] {
				fail(p+"uid %s is shared with another worker (want one uid per activation)", w.UserUID)
			}
			seenUID[w.UserUID] = true
		}
		if w.Probes == nil {
			fail("%s", p+"no probe report")
		} else {
			for _, name := range deskCellAllowedProbes {
				if got, ok := w.Probes[name]; !ok || got != "" {
					fail(p+"%s should succeed: %q", name, got)
				}
			}
			for _, name := range deskCellDeniedProbes {
				got, ok := w.Probes[name]
				switch {
				case !ok:
					fail(p+"%s did not run", name)
				case got == "":
					fail(p+"%s succeeded inside a confined worker", name)
				case name != "write_etc" && !strings.HasPrefix(got, "EPERM"):
					// The cell filter answers EPERM; anything else means the
					// call reached the kernel (unshare of a multithreaded
					// process is EINVAL, exec of a missing file ENOENT).
					fail(p+"%s = %q, want EPERM from the cell seccomp filter", name, got)
				}
			}
			if w.MountNSIsolated {
				for _, name := range deskCellMountProbes {
					got, ok := w.Probes[name]
					switch {
					case !ok:
						fail(p+"%s did not run", name)
					case got == "":
						fail(p+"%s succeeded: the worker sees the host's /proc, /sys or /dev", name)
					}
				}
			}
			if rec.LandlockABI > 0 {
				for _, name := range deskCellLandlockProbes {
					got, ok := w.Probes[name]
					if !ok {
						fail(p+"%s did not run", name)
					} else if got == "" {
						fail(p+"%s succeeded under Landlock", name)
					}
				}
			}
			for _, target := range w.ReadTargets {
				got, ok := w.Probes["read:"+target]
				switch {
				case !ok:
					fail(p+"read of %s did not run", target)
				case got == "":
					// Without Landlock a world-readable file outside the
					// grants stays readable; that is the gap Landlock closes.
					if rec.LandlockABI > 0 || identity && strings.Contains(target, deskCellSelfTestCanary) {
						fail(p+"read %s succeeded", target)
					}
				case strings.HasPrefix(got, "ENOENT") && !w.MountNSIsolated:
					// With its own mount namespace the path is not in the
					// worker's view at all, a stronger denial than Landlock's.
					fail(p+"read %s: %s (a missing path proves nothing)", target, got)
				}
			}
		}
		if !w.CellServes {
			fail(p+"check cell did not run after confinement: %s", w.CellOutput)
		}
	}
	return fails
}

// judgeSandboxSharing checks R40 on the receipt: workers of one trust class
// share one sandbox, workers of different classes never do, and the spawner
// read back exactly one live sandbox per class.
func judgeSandboxSharing(rec DeskCellSelfTestReceipt) []string {
	var fails []string
	byKey := map[string]string{}
	byID := map[string]string{}
	for _, w := range rec.Workers {
		if w.Profile != "gvisor" || w.Error != "" {
			continue
		}
		if id, ok := byKey[w.SandboxKey]; ok && id != w.SandboxID {
			fails = append(fails, fmt.Sprintf("%s: class %s runs in sandbox %s and %s (want one per class)", w.Role, w.SandboxKey, id, w.SandboxID))
		}
		if key, ok := byID[w.SandboxID]; ok && key != w.SandboxKey {
			fails = append(fails, fmt.Sprintf("%s: sandbox %s serves classes %s and %s", w.Role, w.SandboxID, key, w.SandboxKey))
		}
		byKey[w.SandboxKey], byID[w.SandboxID] = w.SandboxID, w.SandboxKey
	}
	if rec.Gvisor == nil || len(byKey) == 0 {
		return fails
	}
	live := map[string]int{}
	for _, sb := range rec.Gvisor.Sandboxes {
		live[sb.Key]++
		if sb.RSSBytes == 0 {
			fails = append(fails, fmt.Sprintf("sandbox %s: no resident memory read back", sb.ID))
		}
	}
	for key, id := range byKey {
		if live[key] != 1 {
			fails = append(fails, fmt.Sprintf("class %s: %d live sandboxes read back, want 1 (%s)", key, live[key], id))
		}
	}
	return fails
}

// judgeGvisorWorker judges one worker in a gVisor sandbox. Its status was
// read inside the sandbox; the namespaces are the sandbox's own (gVisor's
// kernel, --network=none), so the host namespace checks do not apply, and a
// host path being absent from its view is a stronger denial than Landlock's.
func judgeGvisorWorker(w DeskCellSelfTestWorker, identity bool, seenUID map[string]bool) []string {
	var fails []string
	p := w.Role + ": "
	fail := func(format string, args ...any) { fails = append(fails, p+fmt.Sprintf(format, args...)) }
	if !w.GvisorMarker {
		fail("the sandbox shows no gVisor marker")
	}
	st := w.Status
	if st["NoNewPrivs"] != "1" {
		fail("NoNewPrivs=%q, want 1", st["NoNewPrivs"])
	}
	if st["Seccomp"] != "2" {
		fail("Seccomp=%q, want 2", st["Seccomp"])
	}
	uids := strings.Fields(st["Uid"])
	if w.UserUID == "" || len(uids) != 4 || uids[0] != w.UserUID || uids[1] != w.UserUID || uids[2] != w.UserUID || uids[3] != w.UserUID {
		fail("Uid=%q, want %s on all four", st["Uid"], w.UserUID)
	}
	if len(uids) > 0 && uids[0] == "0" {
		fail("runs as root")
	}
	for _, k := range []string{"CapEff", "CapPrm", "CapInh"} {
		if strings.Trim(st[k], "0") != "" {
			fail("%s=%s, want 0", k, st[k])
		}
	}
	if seenUID[w.UserUID] {
		fail("uid %s is shared with another worker (want one uid per activation)", w.UserUID)
	}
	seenUID[w.UserUID] = true
	if w.Probes == nil {
		fail("no probe report")
	} else {
		for _, name := range deskCellAllowedProbes {
			if got, ok := w.Probes[name]; !ok || got != "" {
				fail("%s should succeed: %q", name, got)
			}
		}
		for _, name := range deskCellDeniedProbes {
			got, ok := w.Probes[name]
			switch {
			case !ok:
				fail("%s did not run", name)
			case got == "":
				fail("%s succeeded inside a confined worker", name)
			case name != "write_etc" && !strings.HasPrefix(got, "EPERM"):
				fail("%s = %q, want EPERM from the cell seccomp filter", name, got)
			}
		}
		// gVisor's /proc/meminfo and /dev are the sandbox kernel's own, not
		// the host's; the rest must be refused or absent.
		for _, name := range []string{"proc_pid1_environ", "proc_parent_status", "sys_kernel", "dev_mem", "write_allowed_root"} {
			got, ok := w.Probes[name]
			switch {
			case !ok:
				fail("%s did not run", name)
			case got == "":
				fail("%s succeeded inside the sandbox", name)
			}
		}
		for _, target := range w.ReadTargets {
			got, ok := w.Probes["read:"+target]
			switch {
			case !ok:
				fail("read of %s did not run", target)
			case got == "" && (!strings.Contains(target, deskCellSelfTestCanary) || identity):
				fail("read %s succeeded", target)
			}
		}
	}
	if !w.CellServes {
		fail("check cell did not run after confinement: %s", w.CellOutput)
	}
	return fails
}

func existingPaths(paths []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range paths {
		p = filepath.Clean(strings.TrimSpace(p))
		if p == "." || seen[p] || strings.ContainsRune(p, os.PathListSeparator) {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func procStatusFields(pid int) map[string]string {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return nil
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch k {
		case "Uid", "Gid", "Groups", "CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb", "NoNewPrivs", "Seccomp", "Seccomp_filters":
			out[k] = strings.TrimSpace(v)
		}
	}
	return out
}

func nsDiffers(pid int, ns string) bool {
	theirs, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/%s", pid, ns))
	if err != nil {
		return false
	}
	ours, err := os.Readlink("/proc/self/ns/" + ns)
	if err != nil {
		return false
	}
	return theirs != ours
}
