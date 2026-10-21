//go:build linux

package yaegikernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	seccomp "github.com/elastic/go-seccomp-bpf"
	ll "github.com/landlock-lsm/go-landlock/landlock/syscall"
	"golang.org/x/sys/unix"
)

// Desk cell confinement, the OS half (F4). A cell is model-authored Go;
// these tests assume the Yaegi floor has already failed and ask what the
// worker process itself can still do.
//
// Ways this could fail, written first:
//  1. the worker still runs as root (the identity switch is dropped, or the
//     cell user is missing and the spawn falls back to root);
//  2. it runs as another uid but keeps capabilities or can regain them
//     (no no_new_privs; a setuid binary);
//  3. seccomp is not loaded, or is loaded on one thread only;
//  4. the worker can write outside its own tmp dir (Landlock missing, or
//     the allowed root granted read-write);
//  5. the worker can open a socket of its own: raw, inet or unix, and so
//     reach a host port or another local service;
//  6. the worker can exec or fork a process;
//  7. the confinement breaks a required path: the ready handshake, a
//     plain cell, choir.ReadFile under the allowed root, an egress frame
//     answered by the host, a goroutine-heavy cell (new threads);
//  8. the per-worker tmp dir outlives the worker;
//  9. required mode on a host that cannot switch identity runs the cell
//     unconfined instead of refusing.

// TestDeskCellConfinedHelperProcess is the re-executed worker for these
// tests: the production worker entry (confine, then serve), plus probes
// that act as an escaped cell would.
func TestDeskCellConfinedHelperProcess(t *testing.T) {
	if os.Getenv("CHOIR_DESK_CELL_CHILD") != "1" {
		return
	}
	cfg := SessionWorkerConfigFromEnv(nil, "")
	fd, conn, err := SessionWorkerSockFD(nil, "")
	if err != nil || fd < 0 || conn == nil {
		fmt.Fprintf(os.Stderr, "helper: no session socket: %v\n", err)
		os.Exit(2)
	}
	// The self-test's mutation check runs a worker that skips confinement.
	if os.Getenv("CHOIR_DESK_CELL_SKIP_CONFINE") != "1" {
		if err := ConfineDeskSessionWorker(nil, ""); err != nil {
			fmt.Fprintf(os.Stderr, "helper: confine: %v\n", err)
			os.Exit(3)
		}
	}
	RunDeskCellSelfTestIfRequested(nil, "")
	if os.Getenv("CHOIR_DESK_CELL_PROBE") == "1" {
		raw, _ := json.Marshal(runEscapeProbes(cfg))
		fmt.Fprintf(os.Stdout, "PROBES %s\n", raw)
	}
	ExecuteWorkerSessionConn(conn, cfg)
	os.Exit(0)
}

// probeResult is "" when the operation succeeded, else its error.
type probeResults map[string]string

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func runEscapeProbes(cfg SessionWorkerConfig) probeResults {
	out := probeResults{}
	tmp := os.Getenv("TMPDIR")
	out["write_own_tmp"] = errString(os.WriteFile(filepath.Join(tmp, "probe"), []byte("x"), 0o600))
	out["write_allowed_root"] = errString(os.WriteFile(filepath.Join(cfg.AllowedRoot, "probe-write"), []byte("x"), 0o600))
	out["write_etc"] = errString(os.WriteFile("/etc/choir-cell-probe", []byte("x"), 0o600))
	_, err := os.ReadFile(filepath.Join(cfg.AllowedRoot, "visible.txt"))
	out["read_allowed_root"] = errString(err)
	_, err = os.ReadFile("/etc/passwd")
	out["read_etc_passwd"] = errString(err)
	_, err = os.ReadFile("/proc/1/environ")
	out["read_pid1_environ"] = errString(err)
	socket := func(domain, typ, proto int) error {
		fd, err := unix.Socket(domain, typ, proto)
		if err == nil {
			unix.Close(fd)
		}
		return err
	}
	out["socket_packet_raw"] = errString(socket(unix.AF_PACKET, unix.SOCK_RAW, 0))
	out["socket_inet_raw"] = errString(socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_ICMP))
	out["socket_inet_stream"] = errString(socket(unix.AF_INET, unix.SOCK_STREAM, 0))
	out["socket_inet6_dgram"] = errString(socket(unix.AF_INET6, unix.SOCK_DGRAM, 0))
	out["socket_unix"] = errString(socket(unix.AF_UNIX, unix.SOCK_STREAM, 0))
	out["socket_netlink"] = errString(socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_ROUTE))
	if addr := os.Getenv("CHOIR_DESK_CELL_PROBE_ADDR"); addr != "" {
		c, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			c.Close()
		}
		out["dial_host_port"] = errString(err)
	}
	// A path that does not exist: an unconfined worker fails with ENOENT
	// (and survives the probe); a confined one never gets that far (EPERM).
	_, err = syscall.ForkExec("/nonexistent/choir-cell-probe", []string{"probe"}, &syscall.ProcAttr{})
	out["fork_exec"] = errString(err)
	out["execve"] = errString(unix.Exec("/nonexistent/choir-cell-probe", []string{"probe"}, nil))
	out["kill_pid1"] = errString(unix.Kill(1, 0))
	out["kill_parent"] = errString(unix.Kill(os.Getppid(), 0))
	out["ptrace_parent"] = errString(unix.PtraceSeize(os.Getppid()))
	// prlimit64 is read-only once confined: a same-UID sibling's limits and
	// the worker's own can be read (pid 0) but never set.
	var lim unix.Rlimit
	out["prlimit_self_read"] = errString(unix.Prlimit(0, unix.RLIMIT_NOFILE, nil, &lim))
	out["prlimit_self_set"] = errString(unix.Prlimit(0, unix.RLIMIT_NOFILE, &lim, nil))
	out["prlimit_parent_read"] = errString(unix.Prlimit(os.Getppid(), unix.RLIMIT_NOFILE, nil, &lim))
	out["prlimit_parent_set"] = errString(unix.Prlimit(os.Getppid(), unix.RLIMIT_NOFILE, &lim, nil))
	out["setuid_root"] = errString(unix.Setresuid(0, 0, 0))
	out["unshare_user"] = errString(unix.Unshare(unix.CLONE_NEWUSER))
	return out
}

// confinedTestEnv prepares a worker binary, allowed root and listener the
// cell user can see, and returns a config for role.
type confinedTestEnv struct {
	cfg      DeskSessionWorkerConfig
	root     string
	listener net.Listener
}

func newConfinedTestEnv(t *testing.T, role string, mode CellConfinementMode) confinedTestEnv {
	t.Helper()
	// A worker running as the cell user must be able to exec the binary and
	// read the allowed root; go test's own build dir is 0700 root.
	shared, err := os.MkdirTemp("", "choir-cell-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(shared) })
	if err := os.Chmod(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(shared, "worker.test")
	copyExecutable(t, os.Args[0], bin)
	root := filepath.Join(shared, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), []byte("cells can read this"), 0o644); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	return confinedTestEnv{
		root:     root,
		listener: ln,
		cfg: DeskSessionWorkerConfig{
			Bin:       bin,
			WorkerArg: "-test.run=TestDeskCellConfinedHelperProcess",
			Session: SessionWorkerConfig{
				AllowedPackages: DefaultSafeStdlibPackagesList(),
				ComputerID:      "host-test",
				ActivationID:    "act-cell-1",
				Epoch:           1,
				AllowedRoot:     root,
				Role:            role,
			},
			ProcessGroup: true,
			ExtraEnv: []string{
				"CHOIR_DESK_CELL_CHILD=1",
				"CHOIR_DESK_CELL_PROBE=1",
				"CHOIR_DESK_CELL_PROBE_ADDR=" + ln.Addr().String(),
			},
			Confinement: CellConfinement{Mode: mode},
		},
	}
}

func copyExecutable(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

// procStatus reads the named fields of /proc/<pid>/status.
func procStatus(t *testing.T, pid int) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		t.Fatalf("read worker status: %v", err)
	}
	fields := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok {
			fields[k] = strings.TrimSpace(v)
		}
	}
	return fields
}

func workerProbes(t *testing.T, w *DeskSessionWorker) probeResults {
	t.Helper()
	buf, _ := w.cmd.Stdout.(*bytes.Buffer)
	if buf == nil {
		t.Fatal("worker stdout not captured")
	}
	for _, line := range strings.Split(buf.String(), "\n") {
		if raw, ok := strings.CutPrefix(line, "PROBES "); ok {
			var out probeResults
			if err := json.Unmarshal([]byte(raw), &out); err != nil {
				t.Fatalf("decode probes: %v", err)
			}
			return out
		}
	}
	t.Fatalf("worker printed no probes; stdout=%q stderr=%q", buf.String(), w.stderr.String())
	return nil
}

// requiredPathsWork runs the cells a desk needs through the confined worker.
func requiredPathsWork(t *testing.T, w *DeskSessionWorker) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cells := []struct{ name, src, want string }{
		{"plain", `import "fmt"
fmt.Println("sum", 1+1)`, "sum 2"},
		{"read_file", `import ("fmt"; "choir")
s, err := choir.ReadFile("visible.txt")
fmt.Println(s, err)`, "cells can read this"},
		{"list_dir", `import ("fmt"; "choir")
names, err := choir.ListDir(".")
fmt.Println(names, err)`, "visible.txt"},
		// Busy goroutines make the runtime start new OS threads after the
		// seccomp filter is in place (clone with CLONE_THREAD).
		{"goroutines", `import "fmt"
done := make(chan int, 64)
for i := 0; i < 64; i++ {
	go func(i int) {
		x := 0
		for j := 0; j < 200000; j++ { x += j % 7 }
		done <- i*i + x*0
	}(i)
}
sum := 0
for i := 0; i < 64; i++ { sum += <-done }
fmt.Println("sum", sum)`, "sum 85344"},
		{"time_json_rand", `import ("fmt"; "time"; "encoding/json"; "encoding/hex"; "math/rand/v2"; "regexp")
b, _ := json.Marshal(map[string]int{"a": 1})
loc, lerr := time.LoadLocation("UTC")
fmt.Println(string(b), hex.EncodeToString(b)[:4], rand.IntN(10) < 10, time.Now().In(loc).Year() > 2000, lerr, regexp.MustCompile("a+").MatchString("caab"))`, `{"a":1} 7b22 true true <nil> true`},
	}
	for _, c := range cells {
		res, err := w.EvalCell(ctx, c.src, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("cell %s: transport: %v (stderr=%q)", c.name, err, w.stderr.String())
		}
		if res.Error != "" || !strings.Contains(res.Stdout, c.want) {
			t.Fatalf("cell %s: error=%q stdout=%q, want stdout containing %q", c.name, res.Error, res.Stdout, c.want)
		}
	}
}

func requireDenied(t *testing.T, probes probeResults, names ...string) {
	t.Helper()
	for _, name := range names {
		got, ok := probes[name]
		if !ok {
			t.Errorf("probe %s did not run", name)
			continue
		}
		if got == "" {
			t.Errorf("probe %s succeeded inside a confined worker", name)
		}
	}
}

func requireAllowed(t *testing.T, probes probeResults, names ...string) {
	t.Helper()
	for _, name := range names {
		if got := probes[name]; got != "" {
			t.Errorf("probe %s failed inside a confined worker: %s", name, got)
		}
	}
}

// requireEPERM: refused by the seccomp filter, not by anything later.
func requireEPERM(t *testing.T, probes probeResults, names ...string) {
	t.Helper()
	for _, name := range names {
		if got := probes[name]; got != unix.EPERM.Error() {
			t.Errorf("probe %s = %q, want %q (the cell seccomp filter)", name, got, unix.EPERM.Error())
		}
	}
}

func landlockEnforced(probes probeResults) bool {
	// With Landlock a world-readable file outside the grants is unreadable;
	// without it only DAC applies and /etc/passwd stays readable.
	return probes["read_etc_passwd"] != ""
}

func kernelHasLandlock() bool {
	v, err := ll.LandlockGetABIVersion()
	return err == nil && v >= 5
}

// TestDeskCellWorkerRunsAsTheDeskUser is the production shape: a root
// runtime spawns the worker. A root spawner always switches identity and
// network namespace, in auto mode too; required mode (the guest image)
// adds mandatory Landlock, covered by TestDeskCellRequiredModeNeedsLandlock.
// It needs root; the cell identity is a numeric uid from the desk manifest,
// so no user has to exist on the test host.
func TestDeskCellWorkerRunsAsTheDeskUser(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs a root spawner (the guest runtime is root)")
	}
	cellUID, known := DeskCellIdentity("research")
	if !known {
		t.Fatal("the manifest gives research no cell identity")
	}
	env := newConfinedTestEnv(t, "research", CellConfinementAuto)
	var egressCalls int
	env.cfg.Egress = func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
		egressCalls++
		return json.Marshal(map[string]any{"results": []string{"host answered"}})
	}
	w, err := SpawnDeskSessionWorker(env.cfg)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.Close()

	status := procStatus(t, w.PID())
	uid := strings.Fields(status["Uid"])
	if len(uid) != 4 {
		t.Fatalf("status Uid = %q", status["Uid"])
	}
	for _, id := range uid {
		if id != strconv.Itoa(int(cellUID)) {
			t.Fatalf("worker Uid = %q, want all four = %d (research's manifest uid)", status["Uid"], cellUID)
		}
	}
	if status["Groups"] != "" {
		t.Errorf("worker has supplementary groups %q", status["Groups"])
	}
	for _, field := range []string{"CapInh", "CapPrm", "CapEff", "CapAmb"} {
		if strings.Trim(status[field], "0") != "" {
			t.Errorf("worker %s = %s, want 0", field, status[field])
		}
	}
	if status["NoNewPrivs"] != "1" {
		t.Errorf("worker NoNewPrivs = %q, want 1", status["NoNewPrivs"])
	}
	if status["Seccomp"] != "2" {
		t.Errorf("worker Seccomp = %q, want 2 (filter)", status["Seccomp"])
	}
	self, _ := os.Readlink("/proc/self/ns/net")
	theirs, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/net", w.PID()))
	if err != nil || theirs == self {
		t.Errorf("worker network namespace %q, want a private one (runtime's is %q)", theirs, self)
	}

	requiredPathsWork(t, w)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := w.EvalCell(ctx, `import ("fmt"; "choir")
r, err := choir.SearchWireCorpus("choir", 2)
fmt.Println(r, err)`, nil, nil, nil, nil, nil, nil)
	if err != nil || res.Error != "" || !strings.Contains(res.Stdout, "host answered") || egressCalls != 1 {
		t.Fatalf("egress frame from a confined worker: err=%v res=%+v calls=%d", err, res, egressCalls)
	}

	probes := workerProbes(t, w)
	requireAllowed(t, probes, "write_own_tmp", "read_allowed_root")
	// write_allowed_root and read_pid1_environ are refused by DAC alone
	// (the root is 0755 root, environ 0400 root); Landlock adds the rest.
	requireDenied(t, probes,
		"write_allowed_root", "write_etc", "read_pid1_environ",
		"socket_packet_raw", "socket_inet_raw", "socket_inet_stream", "socket_inet6_dgram",
		"socket_unix", "socket_netlink", "dial_host_port",
		"fork_exec", "execve", "kill_pid1", "kill_parent", "ptrace_parent",
		"setuid_root", "unshare_user")
	requireEPERM(t, probes, "socket_inet_stream", "socket_unix", "socket_packet_raw", "fork_exec", "execve", "ptrace_parent", "kill_parent", "prlimit_self_set", "prlimit_parent_set")
	if probes["prlimit_self_read"] != "" {
		t.Errorf("confined worker cannot read its own rlimit: %s", probes["prlimit_self_read"])
	}
	if kernelHasLandlock() != landlockEnforced(probes) {
		t.Errorf("kernel Landlock=%v but worker file confinement enforced=%v", kernelHasLandlock(), landlockEnforced(probes))
	}

	tmp := w.cellTmpDir
	if tmp == "" {
		t.Fatal("worker has no private tmp dir")
	}
	info, err := os.Stat(tmp)
	if err != nil {
		t.Fatalf("worker tmp: %v", err)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); !ok || fmt.Sprint(st.Uid) != fmt.Sprint(cellUID) || info.Mode().Perm() != 0o700 {
		t.Errorf("worker tmp %s: mode %v owner %v, want 0700 owned by the cell user", tmp, info.Mode().Perm(), info.Sys())
	}
	w.Close()
	if _, err := os.Stat(tmp); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("worker tmp %s survives the worker: %v", tmp, err)
	}
}

// TestDeskCellRequiredModeNeedsLandlock: the guest image runs required
// mode; on a kernel without Landlock V5 the worker refuses to start rather
// than run with DAC alone, and with it the file confinement is enforced.
func TestDeskCellRequiredModeNeedsLandlock(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("required mode needs a root spawner")
	}
	env := newConfinedTestEnv(t, "texture", CellConfinementRequired)
	// Required mode also demands the memory budget; give it a real desks
	// cgroup (skips where cgroups are not writable) so the Landlock refusal
	// is what is tested.
	env.cfg.CgroupRoot = deskCgroupTestRoot(t)
	w, err := SpawnDeskSessionWorker(env.cfg)
	if !kernelHasLandlock() {
		if err == nil {
			w.Close()
			t.Fatal("required mode started a worker on a kernel without Landlock V5")
		}
		if !strings.Contains(err.Error(), "Landlock") {
			t.Fatalf("required mode without Landlock: err = %v, want a Landlock refusal", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.Close()
	requiredPathsWork(t, w)
	probes := workerProbes(t, w)
	requireDenied(t, probes, "read_etc_passwd", "write_allowed_root", "write_etc")
}

// TestDeskCellWorkerSandboxedWithoutRoot is the CI and dev shape: a
// non-root spawner cannot switch identity or create a network namespace,
// but the worker still loads no_new_privs, seccomp and (where the kernel
// has it) Landlock, and every required path keeps working under them.
func TestDeskCellWorkerSandboxedWithoutRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("covers the non-root spawner")
	}
	env := newConfinedTestEnv(t, "management", CellConfinementAuto)
	w, err := SpawnDeskSessionWorker(env.cfg)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.Close()
	status := procStatus(t, w.PID())
	if status["NoNewPrivs"] != "1" || status["Seccomp"] != "2" {
		t.Fatalf("worker NoNewPrivs=%q Seccomp=%q, want 1 and 2", status["NoNewPrivs"], status["Seccomp"])
	}
	requiredPathsWork(t, w)
	probes := workerProbes(t, w)
	requireAllowed(t, probes, "write_own_tmp", "read_allowed_root")
	requireDenied(t, probes,
		"socket_packet_raw", "socket_inet_raw", "socket_inet_stream", "socket_inet6_dgram",
		"socket_unix", "socket_netlink", "dial_host_port",
		"fork_exec", "execve", "kill_pid1", "kill_parent", "ptrace_parent", "unshare_user")
	requireEPERM(t, probes, "socket_inet_stream", "socket_unix", "socket_packet_raw", "fork_exec", "execve", "ptrace_parent", "kill_parent", "prlimit_self_set", "prlimit_parent_set")
	if probes["prlimit_self_read"] != "" {
		t.Errorf("confined worker cannot read its own rlimit: %s", probes["prlimit_self_read"])
	}
	if landlockEnforced(probes) {
		requireDenied(t, probes, "write_allowed_root", "read_pid1_environ")
	} else {
		t.Log("kernel without Landlock: file confinement is DAC only on this host")
	}
}

// TestDeskCellRequiredModeRefusesWithoutIdentity: required mode never runs
// a cell it cannot confine.
func TestDeskCellRequiredModeRefusesWithoutIdentity(t *testing.T) {
	if os.Geteuid() != 0 {
		// A non-root spawner cannot switch identity, so required mode
		// refuses outright.
		env := newConfinedTestEnv(t, "management", CellConfinementRequired)
		if w, err := SpawnDeskSessionWorker(env.cfg); err == nil {
			w.Close()
			t.Fatal("required confinement spawned a worker it could not confine")
		}
	}
	// A role the manifest gives no cell identity (engineering runs in a
	// capsule; the others are not desks) is refused in every mode, root or
	// not: there is no unconfined fallback and no default identity.
	for _, role := range []string{"engineering", "", "conductor", "root", "app:never-registered"} {
		env := newConfinedTestEnv(t, role, CellConfinementRequired)
		if w, err := SpawnDeskSessionWorker(env.cfg); err == nil {
			w.Close()
			t.Fatalf("required confinement spawned a worker for role %q with no cell identity", role)
		}
	}
}

// TestDeskCellWorkerRefusesToServeAsRoot: if the identity switch is ever
// lost, the worker itself refuses to serve as root in required mode.
func TestDeskCellWorkerRefusesToServeAsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	err := ConfineDeskSessionWorker(func(k string) string {
		switch k {
		case "CHOIR_DESK_SESSION_SANDBOX":
			return string(cellSandboxRequired)
		case "CHOIR_DESK_SESSION_ALLOWED_ROOT", "CHOIR_DESK_SESSION_CELL_TMPDIR":
			return os.TempDir()
		}
		return ""
	}, "")
	if err == nil || !strings.Contains(err.Error(), "root") {
		t.Fatalf("confining a root worker in required mode: err = %v, want a refusal naming root", err)
	}
}

// TestDeskCellSeccompFilterAssembles: the cell filter assembles on this
// architecture (Node B is amd64, dev hosts arm64) and admits threads but
// not processes.
func TestDeskCellSeccompFilterAssembles(t *testing.T) {
	filter := DeskCellSeccompFilter(os.Getpid())
	if _, err := filter.Policy.Assemble(); err != nil {
		t.Fatalf("assemble: %v", err)
	}
	names := map[string]bool{}
	for _, group := range filter.Policy.Syscalls {
		for _, n := range group.Names {
			names[n] = group.Action == cellAllow
		}
		for _, n := range group.NamesWithCondtions {
			if group.Action == cellAllow {
				names[n.Name+"?"] = true
			}
		}
	}
	for _, banned := range []string{"socket", "connect", "bind", "execve", "execveat", "fork", "vfork", "ptrace", "process_vm_readv", "setuid", "setresuid", "unshare", "setns", "mount", "bpf", "io_uring_setup", "memfd_create"} {
		if names[banned] {
			t.Errorf("cell filter allows %s", banned)
		}
	}
	for _, conditioned := range []string{"clone?", "kill?", "tgkill?", "prlimit64?"} {
		if !names[conditioned] {
			t.Errorf("cell filter does not admit %s under a condition", strings.TrimSuffix(conditioned, "?"))
		}
	}
}

// TestDeskCellSeccompPrlimitIsReadOnly: prlimit64 is never in the
// unconditioned allow list, and every conditioned entry pins both the target
// (pid 0 or the worker's own pid) and a null new_limit.
func TestDeskCellSeccompPrlimitIsReadOnly(t *testing.T) {
	self := os.Getpid()
	filter := DeskCellSeccompFilter(self)
	if _, err := filter.Policy.Assemble(); err != nil {
		t.Fatalf("assemble: %v", err)
	}
	targets := map[uint64]bool{}
	for _, group := range filter.Policy.Syscalls {
		for _, n := range group.Names {
			if n == "prlimit64" && group.Action == cellAllow {
				t.Fatal("prlimit64 is allowed unconditionally")
			}
		}
		for _, n := range group.NamesWithCondtions {
			if n.Name != "prlimit64" || group.Action != cellAllow {
				continue
			}
			var pid uint64
			pinned, nullNew := false, false
			for _, c := range n.Conditions {
				if c.Argument == 0 && c.Operation == seccomp.Equal {
					pid, pinned = c.Value, true
				}
				if c.Argument == 2 && c.Operation == seccomp.Equal && c.Value == 0 {
					nullNew = true
				}
			}
			if !pinned || !nullNew {
				t.Errorf("prlimit64 entry %+v does not pin pid and null new_limit", n.Conditions)
			}
			targets[pid] = true
		}
	}
	if !targets[0] || !targets[uint64(self)] || len(targets) != 2 {
		t.Errorf("prlimit64 targets = %v, want exactly pid 0 and %d", targets, self)
	}
}
