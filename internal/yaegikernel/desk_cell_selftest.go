package yaegikernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
var DeskCellSelfTestRoles = []string{"management", "texture", "research"}

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
	}
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
	// Probes is the worker's own report, written before its first cell.
	Probes DeskCellSelfTestProbes `json:"probes,omitempty"`
	// ReadTargets are the paths the worker was asked to read (each exists).
	ReadTargets []string `json:"read_targets,omitempty"`
	// CellOutput is the check cell's stdout after the probes.
	CellOutput string `json:"cell_output,omitempty"`
	CellServes bool   `json:"cell_serves"`
	Error      string `json:"error,omitempty"`
}

// DeskCellSelfTestReceipt is the whole self-test.
type DeskCellSelfTestReceipt struct {
	Kind        string                   `json:"kind"`
	StartedAt   string                   `json:"started_at"`
	Mode        string                   `json:"mode"`
	SpawnerUID  int                      `json:"spawner_uid"`
	LandlockABI int                      `json:"landlock_abi"`
	AllowedRoot string                   `json:"allowed_root"`
	Workers     []DeskCellSelfTestWorker `json:"workers"`
	OK          bool                     `json:"ok"`
	Failures    []string                 `json:"failures,omitempty"`
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
		out := DeskCellSelfTestWorker{Role: role}
		reads := existingPaths(append([]string{"/etc/passwd"}, cfg.ExtraReads...))
		if prevCanary != "" {
			reads = append(reads, prevCanary)
		}
		out.ReadTargets = reads
		wcfg := cfg.Base
		wcfg.Session.Role = role
		wcfg.ExtraEnv = append(append([]string(nil), cfg.Base.ExtraEnv...),
			prefix+cellSelfTestEnv+"=1",
			prefix+cellSelfTestReadEnv+"="+strings.Join(reads, string(os.PathListSeparator)),
		)
		if os.Geteuid() == 0 || wcfg.Confinement.Mode == CellConfinementRequired {
			id := wcfg.Confinement.UID
			if id == 0 {
				id, _ = DeskCellIdentity(role)
			}
			out.User = fmt.Sprintf("manifest uid %d", id)
			out.UserUID = strconv.FormatUint(uint64(id), 10)
		}
		w, err := SpawnDeskSessionWorker(wcfg)
		if err != nil {
			out.Error = "spawn: " + err.Error()
			rec.Workers = append(rec.Workers, out)
			prevCanary = ""
			continue
		}
		live = append(live, w)
		out.PID = w.PID()
		out.Status = procStatusFields(w.PID())
		out.NetNSIsolated = nsDiffers(w.PID(), "net")
		out.IPCNSIsolated = nsDiffers(w.PID(), "ipc")
		if raw, err := os.ReadFile(filepath.Join(w.cellTmpDir, cellSelfTestFile)); err != nil {
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
		// The next desk tries to read this desk's live tmp dir.
		prevCanary = ""
		if w.cellTmpDir != "" {
			canary := filepath.Join(w.cellTmpDir, deskCellSelfTestCanary)
			if err := os.WriteFile(canary, []byte("another desk's file\n"), 0o644); err == nil {
				_ = os.Chmod(canary, 0o644)
				prevCanary = canary
			}
		}
		rec.Workers = append(rec.Workers, out)
	}
	rec.Failures = JudgeDeskCellSelfTest(rec)
	rec.OK = len(rec.Failures) == 0
	return rec
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
	if rec.Mode == string(CellConfinementRequired) && rec.LandlockABI < 5 {
		fail("required mode on a kernel with Landlock ABI %d (want >= 5)", rec.LandlockABI)
	}
	if len(rec.Workers) == 0 {
		fail("no workers")
	}
	for _, w := range rec.Workers {
		p := w.Role + ": "
		if w.Error != "" {
			fail(p+"%s", w.Error)
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
				case strings.HasPrefix(got, "ENOENT"):
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
