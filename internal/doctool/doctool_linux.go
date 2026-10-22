//go:build linux

package doctool

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	seccomp "github.com/elastic/go-seccomp-bpf"
	"github.com/elastic/go-seccomp-bpf/arch"
	"github.com/landlock-lsm/go-landlock/landlock"
	ll "github.com/landlock-lsm/go-landlock/landlock/syscall"
	"golang.org/x/sys/unix"
)

// The helper's environment, set by the spawner only. Without the level the
// helper does nothing (D8).
const (
	helperLevelEnv     = "CHOIR_DOCTOOL_LEVEL"
	helperInputDirEnv  = "CHOIR_DOCTOOL_INPUT_DIR"
	helperExecRootsEnv = "CHOIR_DOCTOOL_EXEC_ROOTS"
	helperArgv0Env     = "CHOIR_DOCTOOL_ARGV0"
	helperLevel        = "required"
)

// defaultExecRoots are where a tool and its libraries live: the Nix store on
// the guest; the usual directories elsewhere. Missing ones are skipped. The
// tool's own directory is added per run.
var defaultExecRoots = []string{"/nix/store", "/usr", "/lib", "/lib64", "/bin"}

// passedEnv are the daemon's variables a tool may see; nothing else crosses.
// TZ crosses: pdfinfo prints dates in it, and observe compares outputs
// (review B3).
var passedEnv = []string{"LANG", "LC_ALL", "LOCALE_ARCHIVE", "TZ", "TZDIR"}

// runSandboxed runs the tool through the helper. budget puts it in a cgroup
// of its own under the document tools' root (required, and observe's shadow
// since review E2); only the isolation tests run without one.
// maxRSS is the tool's peak RSS. setupErr is non-nil when the sandbox could
// not be set up and the tool did not run.
func runSandboxed(ctx context.Context, opts runOptions, budget bool, tool, toolPath string, prefixArgs []string, ext string, raw []byte, suffixArgs []string) (out string, warnings []string, maxRSS int64, setupErr error) {
	resolved, err := filepath.EvalSymlinks(toolPath)
	if err != nil {
		return "", nil, 0, setupErrorf("resolve %s: %v", tool, err)
	}
	// D1: the identity switch needs a root spawner. Never run as root
	// instead.
	root := os.Geteuid() == 0
	if !root && !opts.testUserNamespace {
		return "", nil, 0, setupErrorf("running tools as uid %d needs a root spawner (running as uid %d)", SandboxUID, os.Geteuid())
	}
	helperBin := opts.helperBin
	if helperBin == "" {
		if helperBin, err = os.Executable(); err != nil {
			return "", nil, 0, setupErrorf("helper binary: %v", err)
		}
	}

	// D4: the memory and process budget, part of the sandbox.
	var cg *toolCgroup
	if budget && !opts.testNoCgroup {
		cgRoot := os.Getenv(cgroupRootEnv)
		if cgRoot == "" {
			return "", nil, 0, setupErrorf("no memory budget: %s is unset (no document tools cgroup)", cgroupRootEnv)
		}
		if cg, err = newToolCgroup(cgRoot); err != nil {
			return "", nil, 0, setupErrorf("%v", err)
		}
		defer cg.remove()
	}

	// The input: a 0400 file in a 0700 directory the sandbox user owns.
	dir, err := os.MkdirTemp("", "choir-doctool-")
	if err != nil {
		return "", nil, 0, setupErrorf("input dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	input := filepath.Join(dir, "input"+safeExt(ext))
	if err := os.WriteFile(input, raw, 0o400); err != nil {
		return "", nil, 0, setupErrorf("input file: %v", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", nil, 0, setupErrorf("input dir mode: %v", err)
	}
	if root && !opts.testUserNamespace {
		for _, p := range []string{dir, input} {
			if err := os.Lchown(p, SandboxUID, SandboxUID); err != nil {
				return "", nil, 0, setupErrorf("input owner: %v", err)
			}
		}
	}

	toolCtx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	limit := opts.outputCap
	if limit <= 0 {
		limit = defaultOutputCap
	}
	stdout := &cappedBuffer{limit: limit, onOverflow: cancel}
	stderr := &cappedBuffer{limit: 64 << 10}
	argv := append([]string{HelperArg, resolved}, toolArgs(prefixArgs, input, suffixArgs)...)
	cmd := exec.CommandContext(toolCtx, helperBin, argv...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = helperEnv(tool, dir, resolved)
	attr := &syscall.SysProcAttr{
		Setpgid:    true,
		Pdeathsig:  syscall.SIGKILL,
		Cloneflags: syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	if opts.testUserNamespace {
		attr.Cloneflags |= syscall.CLONE_NEWUSER
		attr.UidMappings = []syscall.SysProcIDMap{{ContainerID: os.Getuid(), HostID: os.Getuid(), Size: 1}}
		attr.GidMappings = []syscall.SysProcIDMap{{ContainerID: os.Getgid(), HostID: os.Getgid(), Size: 1}}
		attr.GidMappingsEnableSetgroups = false
	} else {
		attr.Credential = &syscall.Credential{Uid: SandboxUID, Gid: SandboxUID, Groups: []uint32{}}
	}
	if cg != nil {
		attr.UseCgroupFD = true
		attr.CgroupFD = int(cg.dir.Fd())
	}
	cmd.SysProcAttr = attr
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.WaitDelay = 2 * time.Second

	if err := cmd.Start(); err != nil {
		return "", nil, 0, setupErrorf("start: %v", err)
	}
	err = cmd.Wait()
	if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok && ru != nil {
		maxRSS = ru.Maxrss << 10 // KiB
	}
	if stdout.overflow {
		return "", []string{fmt.Sprintf("%s_output_exceeds_cap: %d MiB", tool, limit>>20)}, maxRSS, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == helperSetupExit {
		if msg, ok := strings.CutPrefix(stderr.buf.String(), helperSetupPrefix); ok {
			return "", nil, maxRSS, setupErrorf("%s", strings.TrimSpace(truncate(msg, 500)))
		}
	}
	if err != nil {
		w := toolFailure(tool, stderr.buf.String(), err)
		if cg != nil {
			if n := cg.oomKills(); n > 0 {
				w += fmt.Sprintf(" (%s, memory.max %d MiB)", killedByBudget, toolMemoryMax>>20)
			}
		}
		return "", []string{w}, maxRSS, nil
	}
	return stdout.buf.String(), nil, maxRSS, nil
}

func helperEnv(tool, dir, resolved string) []string {
	roots := append(append([]string{}, defaultExecRoots...), filepath.Dir(resolved))
	env := []string{
		helperLevelEnv + "=" + helperLevel,
		helperInputDirEnv + "=" + dir,
		helperExecRootsEnv + "=" + strings.Join(roots, ":"),
		helperArgv0Env + "=" + filepath.Base(tool),
		"HOME=" + dir,
		"TMPDIR=" + dir,
		"PATH=" + filepath.Dir(resolved),
	}
	for _, k := range passedEnv {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func safeExt(ext string) string {
	var b strings.Builder
	for _, r := range ext {
		if r == '.' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// EnsureToolCgroupRoot makes the document tools' cgroup root at daemon
// start: the memory and pids controllers for its children, its aggregate
// memory.max, and the removal of tool cgroups a crashed daemon left (t-*,
// in this root only; tools die with the daemon, nothing resumes). It lists
// one directory.
func EnsureToolCgroupRoot(root string) error {
	if root == "" {
		return fmt.Errorf("document tools cgroup root is not set")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("document tools cgroup %s: %w", root, err)
	}
	enable := func(dir string) error {
		err := os.WriteFile(filepath.Join(dir, "cgroup.subtree_control"), []byte("+memory +pids"), 0o644)
		if err != nil && !strings.Contains(err.Error(), "device or resource busy") {
			return fmt.Errorf("document tools cgroup: enable controllers in %s: %w", dir, err)
		}
		return nil
	}
	if err := enable(filepath.Dir(root)); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("document tools cgroup: list %s: %w", root, err)
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), toolCgroupPrefix) {
			// Review E5: kill, then rmdir as the per-run removal does; never
			// a recursive unlink.
			if p := filepath.Join(root, e.Name()); !killAndRemoveCgroup(p) {
				log.Printf("doctool: leftover tool cgroup %s not removed", p)
			}
		}
	}
	if err := enable(root); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "memory.max"), []byte(strconv.Itoa(toolAggregateMax)), 0o644); err != nil {
		return fmt.Errorf("document tools cgroup: memory.max: %w", err)
	}
	return nil
}

// toolCgroup is one run's cgroup under the document tools' root.
type toolCgroup struct {
	path string
	dir  *os.File
}

// writeToolCgroupFile writes one control file of a tool cgroup (tests
// replace it).
var writeToolCgroupFile = func(dir, name, value string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(value), 0o644)
}

func newToolCgroup(root string) (*toolCgroup, error) {
	id := make([]byte, 6)
	if _, err := rand.Read(id); err != nil {
		return nil, fmt.Errorf("tool cgroup name: %w", err)
	}
	path := filepath.Join(root, toolCgroupPrefix+hex.EncodeToString(id))
	if err := os.Mkdir(path, 0o755); err != nil {
		return nil, fmt.Errorf("tool cgroup %s: %w", path, err)
	}
	for _, s := range []struct{ file, value string }{
		{"memory.swap.max", "0"},
		{"memory.max", strconv.Itoa(toolMemoryMax)},
		{"memory.oom.group", "1"},
		{"pids.max", strconv.Itoa(toolPidsMax)},
	} {
		err := writeToolCgroupFile(path, s.file, s.value)
		if err != nil && s.file == "memory.swap.max" && errors.Is(err, os.ErrNotExist) {
			// No swap controller means no swap to bound (review F1; as the
			// desk worker cgroup does).
			continue
		}
		if err != nil {
			_ = os.Remove(path)
			return nil, fmt.Errorf("tool cgroup %s: %s: %w", path, s.file, err)
		}
	}
	dir, err := os.Open(path)
	if err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("tool cgroup %s: open: %w", path, err)
	}
	return &toolCgroup{path: path, dir: dir}, nil
}

func (c *toolCgroup) oomKills() int64 {
	raw, err := os.ReadFile(filepath.Join(c.path, "memory.events"))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "oom_kill" {
			n, _ := strconv.ParseInt(f[1], 10, 64)
			return n
		}
	}
	return 0
}

// remove kills anything left in the cgroup and removes it; a leftover is
// swept at the next boot (D7).
func (c *toolCgroup) remove() {
	_ = c.dir.Close()
	_ = killAndRemoveCgroup(c.path)
}

// killAndRemoveCgroup kills every process in a cgroup and removes its
// directory with rmdir, retrying briefly while the killed processes drain.
// It reports whether the directory is gone.
func killAndRemoveCgroup(path string) bool {
	// cgroup.kill exists in every cgroup; it is written, never created.
	if f, err := os.OpenFile(filepath.Join(path, "cgroup.kill"), os.O_WRONLY, 0); err == nil {
		_, _ = f.WriteString("1")
		_ = f.Close()
	}
	for i := 0; i < 50; i++ {
		if err := unix.Rmdir(path); err == nil || errors.Is(err, unix.ENOENT) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// RunHelper is `autoputer doc-tool-sandbox <tool> <args...>`: started by the
// spawner as the sandbox user in its namespaces and cgroup, it confines
// itself and execs the tool. It returns only on failure, with
// helperSetupExit and a helperSetupPrefix line on stderr.
func RunHelper(args []string) int {
	fail := func(format string, a ...any) int {
		fmt.Fprintf(os.Stderr, helperSetupPrefix+format+"\n", a...)
		return helperSetupExit
	}
	if os.Getenv(helperLevelEnv) != helperLevel {
		return fail("not started by the document-tool spawner")
	}
	if len(args) < 1 || !filepath.IsAbs(args[0]) {
		return fail("no absolute tool path")
	}
	// D10.
	if os.Geteuid() == 0 || os.Getuid() == 0 {
		return fail("refusing to run a document tool as root")
	}
	for _, l := range []struct {
		res int
		val uint64
	}{{unix.RLIMIT_CORE, 0}, {unix.RLIMIT_NOFILE, 64}, {unix.RLIMIT_CPU, 60}} {
		if err := unix.Setrlimit(l.res, &unix.Rlimit{Cur: l.val, Max: l.val}); err != nil {
			return fail("rlimit %d: %v", l.res, err)
		}
	}
	if err := restrictToolFilesystem(os.Getenv(helperExecRootsEnv), os.Getenv(helperInputDirEnv)); err != nil {
		return fail("%v", err)
	}
	if err := seccomp.LoadFilter(ToolSeccompFilter()); err != nil {
		return fail("seccomp: %v", err)
	}
	argv0 := os.Getenv(helperArgv0Env)
	if argv0 == "" {
		argv0 = filepath.Base(args[0])
	}
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "CHOIR_DOCTOOL_") {
			env = append(env, kv)
		}
	}
	// Review E8: the tool inherits no fd above stderr, whatever the helper
	// was started with; close-on-exec leaves the helper's runtime alone
	// until the exec.
	if err := unix.CloseRange(3, ^uint(0), unix.CLOSE_RANGE_CLOEXEC); err != nil {
		return fail("close_range: %v", err)
	}
	err := syscall.Exec(args[0], append([]string{argv0}, args[1:]...), env)
	return fail("exec %s: %v", args[0], err)
}

// restrictToolFilesystem applies Landlock V5 (D2, D3, D9): read and execute
// the exec roots; read the input directory and this process's own /proc
// entry; read and write /dev/null; read a few shared files; nothing else,
// and no TCP. A kernel without V5 is refused.
func restrictToolFilesystem(execRoots, inputDir string) error {
	if inputDir == "" {
		return fmt.Errorf("no input dir")
	}
	if v, err := ll.LandlockGetABIVersion(); err != nil || v < 5 {
		return fmt.Errorf("Landlock V5 required, kernel offers ABI %d (%v)", v, err)
	}
	rx := landlock.AccessFSSet(ll.AccessFSExecute | ll.AccessFSReadFile | ll.AccessFSReadDir)
	read := landlock.AccessFSSet(ll.AccessFSReadFile | ll.AccessFSReadDir)
	rules := []landlock.Rule{
		landlock.PathAccess(read, inputDir),
		landlock.PathAccess(read, "/proc/"+strconv.Itoa(os.Getpid())),
		landlock.RWFiles("/dev/null"),
		landlock.ROFiles("/dev/urandom", "/etc/localtime", "/etc/ld.so.cache").IgnoreIfMissing(),
		landlock.RODirs("/etc/fonts", "/etc/zoneinfo").IgnoreIfMissing(),
	}
	for _, root := range strings.Split(execRoots, ":") {
		if root = strings.TrimSpace(root); root != "" {
			rules = append(rules, landlock.PathAccess(rx, root).IgnoreIfMissing())
		}
	}
	if err := landlock.V5.Restrict(rules...); err != nil {
		return fmt.Errorf("Landlock: %w", err)
	}
	// Scoped IPC (V6) where the kernel has it.
	_ = landlock.V6.BestEffort().RestrictScoped()
	return nil
}

var (
	toolDeny   = seccomp.ActionErrno // EPERM
	toolENOSYS = seccomp.Action(uint32(seccomp.ActionErrno) | uint32(unix.ENOSYS))
)

// toolDeniedSyscalls are what no document tool needs and an exploit of one
// reaches for: tracing, mounts and namespaces, kernel code and keys, eBPF and
// perf, io_uring, handle-based opens, the clock, and sockets (the network
// namespace is empty; this also closes unix sockets to the guest's daemons).
var toolDeniedSyscalls = []string{
	"ptrace", "process_vm_readv", "process_vm_writev", "kcmp",
	"mount", "umount2", "pivot_root", "chroot", "unshare", "setns",
	"fsopen", "fsconfig", "fsmount", "fspick", "move_mount", "open_tree", "mount_setattr",
	"bpf", "perf_event_open", "userfaultfd",
	"keyctl", "add_key", "request_key",
	"kexec_load", "kexec_file_load", "init_module", "finit_module", "delete_module",
	"io_uring_setup", "io_uring_enter", "io_uring_register",
	"open_by_handle_at", "name_to_handle_at", "quotactl", "quotactl_fd",
	"swapon", "swapoff", "reboot", "acct", "syslog", "vhangup", "iopl", "ioperm",
	"settimeofday", "clock_settime", "clock_adjtime", "adjtimex",
	"socket", "connect", "bind", "listen", "accept", "accept4",
}

// cloneNamespaceFlags are the CLONE_NEW* bits: clone with any of them is
// refused (D5).
const cloneNamespaceFlags = unix.CLONE_NEWNS | unix.CLONE_NEWUTS | unix.CLONE_NEWIPC | unix.CLONE_NEWUSER |
	unix.CLONE_NEWPID | unix.CLONE_NEWNET | unix.CLONE_NEWCGROUP | unix.CLONE_NEWTIME

// ToolSeccompFilter is the tool's filter: allow by default (tools differ in
// what they need), deny the list above, refuse clone into any namespace, and
// answer clone3 ENOSYS so libc falls back to clone, whose flags it can read.
func ToolSeccompFilter() seccomp.Filter {
	names := toolDeniedSyscalls
	if info, err := arch.GetInfo(""); err == nil {
		names = make([]string, 0, len(toolDeniedSyscalls))
		for _, n := range toolDeniedSyscalls {
			if _, ok := info.SyscallNames[n]; ok {
				names = append(names, n)
			}
		}
	}
	return seccomp.Filter{
		NoNewPrivs: true,
		Flag:       seccomp.FilterFlagTSync,
		Policy: seccomp.Policy{
			DefaultAction: seccomp.ActionAllow,
			Syscalls: []seccomp.SyscallGroup{
				{Action: toolDeny, Names: names},
				{Action: toolDeny, NamesWithCondtions: []seccomp.NameWithConditions{
					{Name: "clone", Conditions: seccomp.ArgumentConditions{{Argument: 0, Operation: seccomp.BitsSet, Value: cloneNamespaceFlags}}},
				}},
				{Action: toolENOSYS, Names: []string{"clone3"}},
			},
		},
	}
}
