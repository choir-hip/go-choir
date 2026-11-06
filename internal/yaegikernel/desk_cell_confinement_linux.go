//go:build linux

package yaegikernel

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	seccomp "github.com/elastic/go-seccomp-bpf"
	"github.com/elastic/go-seccomp-bpf/arch"
	"github.com/landlock-lsm/go-landlock/landlock"
	ll "github.com/landlock-lsm/go-landlock/landlock/syscall"
	"golang.org/x/sys/unix"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// cellSpawnPlan is what SpawnDeskSessionWorker applies to the worker's
// exec.Cmd.
type cellSpawnPlan struct {
	attr   *syscall.SysProcAttr
	env    []string
	dir    string
	tmpDir string
	// uid is the activation's own uid (0 when the identity is not switched);
	// release frees it for reuse at reap.
	uid     uint32
	release func()
	// gvisor is the worker's cell in its trust class's sandbox (MA6.4), nil
	// under process confinement; wrap turns the worker's command into the
	// runsc exec that starts it there.
	gvisor *gvisorCell
	wrap   func(*exec.Cmd) error
	// budgetCap caps a process-confined desk worker's cgroup memory.max at
	// its claim in the desks memory ledger (0: the manifest budget); queued
	// is how long its spawn waited for that claim (or the sandboxed
	// worker's, for its cell).
	budgetCap uint64
	queued    time.Duration
}

// planCellConfinement resolves the spawner half (see desk_cell_confinement.go).
// It refuses rather than weakens: a root spawner without a cell identity for
// the role, or required mode where the identity switch is impossible, is an
// error and no worker starts.
func planCellConfinement(cfg DeskSessionWorkerConfig) (out cellSpawnPlan, err error) {
	attr := workerSysProcAttr(cfg.ProcessGroup)
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}
	// MA6.4: every desk worker runs under gVisor in its trust class's
	// sandbox. Where gVisor is unavailable, a first-party desk runs under the
	// process confinement below and any other desk is refused.
	if cls, isDesk := classifyDesk(cfg.Session.Role); isDesk {
		ok, reason := gvisorAvailable()
		if ok {
			if cfg.Confinement.UID == 0 {
				if _, known := DeskCellIdentity(cfg.Session.Role); !known {
					return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: the manifest gives desk role %q no cell identity", cfg.Session.Role)
				}
			}
			bin := cfg.Bin
			if bin == "" {
				if bin, err = os.Executable(); err != nil {
					return cellSpawnPlan{}, fmt.Errorf("desk worker binary: %w", err)
				}
			}
			gplan, gerr := planGvisorCell(cfg, cls, bin, attr)
			if gerr == nil {
				return gplan, nil
			}
			if errors.Is(gerr, errDesksMemoryCommitted) || errors.Is(gerr, errDesksMemoryNeverFits) {
				// A capacity refusal, not an unavailable gVisor: no desk
				// runs outside its sandbox for want of memory.
				return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: desk %q: %w", cfg.Session.Role, gerr)
			}
			reason = gerr.Error()
		}
		if !cls.FirstParty {
			return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: desk %q is not first-party and runs only under gVisor, which is unavailable: %s", cfg.Session.Role, reason)
		}
		log.Printf("yaegikernel: desk %q (class %s) runs under process confinement, not gVisor: %s", cfg.Session.Role, cls.Key, reason)
		// The fallback worker is in the same desks memory ledger as the
		// sandboxes (it waits like them), and its cgroup is capped at its
		// claim.
		root := cfg.CgroupRoot
		if root == "" {
			root = os.Getenv(DeskCgroupRootEnv)
		}
		var budget uint64
		if d, ok := deskmanifest.Lookup(cfg.Session.Role); ok {
			budget = d.Memory.WorkerMaxBytes
		}
		mem, merr := deskGvisor.admitProcess(cfg.Context, cls, cfg.Session.Role, budget, root)
		if merr != nil {
			return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: desk %q: %w", cfg.Session.Role, merr)
		}
		if mem != nil {
			defer func() {
				if err != nil {
					mem.release()
					return
				}
				out.budgetCap, out.queued = mem.bytes, mem.queued
				uidRelease := out.release
				out.release = func() {
					if uidRelease != nil {
						uidRelease()
					}
					mem.release()
				}
			}()
		}
	}
	plan := cellSpawnPlan{attr: attr, dir: cfg.Session.AllowedRoot}
	defer func() {
		if err != nil && plan.release != nil {
			plan.release()
		}
	}()
	prefix := sessionEnvPrefix(cfg.EnvPrefix)
	root := os.Geteuid() == 0
	required := cfg.Confinement.Mode == CellConfinementRequired
	level := cellSandboxBestEffort
	if required {
		level = cellSandboxRequired
	}

	var uid, gid int
	switchIdentity := root || required
	if switchIdentity {
		id := cfg.Confinement.UID
		if id == 0 {
			// The manifest names the desk and gates the spawn; the uid the
			// worker runs as is its activation's own (desk_cell_uid.go).
			if _, known := DeskCellIdentity(cfg.Session.Role); !known {
				return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: the manifest gives desk role %q no cell identity", cfg.Session.Role)
			}
		}
		if !root {
			return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: required mode needs a root spawner to run cells as a desk cell uid (running as uid %d)", os.Geteuid())
		}
		if id == 0 {
			if id, plan.release, err = activationUIDs.acquire(); err != nil {
				return cellSpawnPlan{}, err
			}
		}
		plan.uid = id
		uid, gid = int(id), int(id)
		// Go sets Pdeathsig after the credential change, so the kernel's
		// reset of the parent-death signal on setuid does not clear it.
		attr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: []uint32{}}
		// An empty network namespace: the worker's only channel is fd 3.
		// Abstract unix sockets live in the network namespace too.
		attr.Cloneflags |= syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC
		// A mount namespace of its own, built into a minimal root by the
		// worker (isolateCellMounts) with CAP_SYS_ADMIN as an ambient
		// capability it drops, on every thread, before it confines itself.
		attr.Cloneflags |= syscall.CLONE_NEWNS
		attr.AmbientCaps = []uintptr{capSysAdmin}
		plan.env = append(plan.env, prefix+cellMountNSEnv+"=1")
	}

	tmp, err := os.MkdirTemp("", "choir-cell-"+safeCellRole(cfg.Session.Role)+"-")
	if err != nil {
		return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: tmp dir: %w", err)
	}
	if err := os.Chmod(tmp, 0o700); err != nil {
		_ = os.RemoveAll(tmp)
		return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: tmp dir mode: %w", err)
	}
	if switchIdentity {
		if err := os.Lchown(tmp, uid, gid); err != nil {
			_ = os.RemoveAll(tmp)
			return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: tmp dir owner: %w", err)
		}
		// The cell user can always enter its own tmp dir; it may not be
		// able to search the allowed root's parents. Reads resolve
		// against AllowedRoot, never the cwd.
		plan.dir = tmp
		plan.env = append(plan.env, prefix+cellUIDEnv+"="+strconv.Itoa(uid))
	}
	plan.tmpDir = tmp
	plan.env = append(plan.env,
		prefix+cellSandboxEnv+"="+string(level),
		prefix+cellTmpDirEnv+"="+tmp,
		"TMPDIR="+tmp,
	)
	return plan, nil
}

func safeCellRole(role string) string {
	var b strings.Builder
	for _, r := range role {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "desk"
	}
	return b.String()
}

// ConfineDeskSessionWorker is the worker half: called by the worker entry
// after it holds its session socket and memory limits, before it serves a
// cell. With no sandbox level in the environment (an older spawner, a test
// helper) it does nothing. Any failure is fatal to the worker: it must not
// serve cells unconfined.
func ConfineDeskSessionWorker(getenv func(string) string, prefix string) error {
	if getenv == nil {
		getenv = os.Getenv
	}
	prefix = sessionEnvPrefix(prefix)
	level := cellSandboxLevel(getenv(prefix + cellSandboxEnv))
	if level == "" {
		return nil
	}
	if level == cellSandboxGvisor {
		return confineGvisorWorker(getenv, prefix)
	}
	if level != cellSandboxRequired && level != cellSandboxBestEffort {
		level = cellSandboxRequired
	}
	if want := getenv(prefix + cellUIDEnv); want != "" {
		if got := strconv.Itoa(os.Getuid()); got != want || os.Geteuid() == 0 {
			return fmt.Errorf("desk cell worker: running as uid %s (euid %d), want the cell user's uid %s", got, os.Geteuid(), want)
		}
	}
	if level == cellSandboxRequired && (os.Geteuid() == 0 || os.Getuid() == 0) {
		return fmt.Errorf("desk cell worker: refusing to serve cells as root")
	}
	tmp := getenv(prefix + cellTmpDirEnv)
	if tmp == "" {
		return fmt.Errorf("desk cell worker: no tmp dir")
	}
	if cellMountNSWanted(getenv, prefix) {
		// Build the minimal root while the ambient capability is still held,
		// then drop every capability on every thread: nothing below needs one.
		if err := isolateCellMounts(getenv(prefix+"ALLOWED_ROOT"), tmp); err != nil {
			return fmt.Errorf("desk cell worker: %w", err)
		}
		if err := dropAllCapabilities(); err != nil {
			return fmt.Errorf("desk cell worker: %w", err)
		}
	}
	if err := applyCellRlimits(); err != nil {
		return fmt.Errorf("desk cell worker: %w", err)
	}
	if err := restrictCellFilesystem(level, getenv(prefix+"ALLOWED_ROOT"), tmp); err != nil {
		return err
	}
	if err := seccomp.LoadFilter(DeskCellSeccompFilter(os.Getpid())); err != nil {
		return fmt.Errorf("desk cell worker: seccomp: %w", err)
	}
	return nil
}

// confineGvisorWorker is the worker half inside a gVisor sandbox: it must be
// in one (the marker only gVisor's procfs holds), never as root and as the
// uid its spawner planned, with no capability; then the rlimits and the cell
// seccomp filter, exactly as outside. gVisor's filesystem view replaces
// Landlock and the MA6.2 mount namespace (Landlock is ENOSYS inside gVisor).
func confineGvisorWorker(getenv func(string) string, prefix string) error {
	if _, err := os.Stat(gvisorMarkerFile); err != nil {
		return fmt.Errorf("desk cell worker: gvisor level outside a gVisor sandbox (%v)", err)
	}
	want := getenv(prefix + cellUIDEnv)
	if want == "" || strconv.Itoa(os.Getuid()) != want || strconv.Itoa(os.Geteuid()) != want || os.Geteuid() == 0 {
		return fmt.Errorf("desk cell worker: running as uid %d (euid %d), want the cell user's uid %q", os.Getuid(), os.Geteuid(), want)
	}
	tmp := getenv(prefix + cellTmpDirEnv)
	if tmp == "" {
		return fmt.Errorf("desk cell worker: no tmp dir")
	}
	// The worker makes its own tmp dir in the sandbox's memory /tmp; a dir
	// that already exists was made by someone else and is refused.
	if err := os.Mkdir(tmp, 0o700); err != nil {
		return fmt.Errorf("desk cell worker: tmp dir: %w", err)
	}
	if st, err := os.Lstat(tmp); err != nil || !st.IsDir() || st.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) {
		return fmt.Errorf("desk cell worker: tmp dir %s is not the worker's own", tmp)
	}
	if err := applyCellRlimits(); err != nil {
		return fmt.Errorf("desk cell worker: %w", err)
	}
	// Landlock's no_new_privs; the filter's own NoNewPrivs sets it too.
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("desk cell worker: no_new_privs: %w", err)
	}
	if err := seccomp.LoadFilter(DeskCellSeccompFilter(os.Getpid())); err != nil {
		return fmt.Errorf("desk cell worker: seccomp: %w", err)
	}
	return nil
}

// restrictCellFilesystem applies Landlock: read on the allowed root, read and
// write on the tmp dir, /dev/null and /dev/urandom, the time zone database;
// nothing else, and (V4+) no TCP bind or connect at all. Required mode
// demands the V5 contract (as capsules do); best effort degrades on older
// kernels. Landlock sets no_new_privs on every thread first.
func restrictCellFilesystem(level cellSandboxLevel, allowedRoot, tmp string) error {
	read := landlock.AccessFSSet(ll.AccessFSReadFile | ll.AccessFSReadDir)
	rules := []landlock.Rule{
		landlock.RWDirs(tmp),
		landlock.RWFiles("/dev/null"),
		landlock.ROFiles("/dev/urandom", "/etc/localtime").IgnoreIfMissing(),
		landlock.PathAccess(read, "/etc/zoneinfo").IgnoreIfMissing(),
	}
	if allowedRoot != "" && cellCanOpen(allowedRoot) {
		rules = append(rules, landlock.PathAccess(read, allowedRoot))
	}
	cfg := landlock.V5
	if level != cellSandboxRequired {
		cfg = cfg.BestEffort()
	} else if v, err := ll.LandlockGetABIVersion(); err != nil || v < 5 {
		// Report it in words a reader recognizes before the library's
		// error does: required mode never runs without Landlock.
		return fmt.Errorf("desk cell worker: Landlock V5 required, kernel offers ABI %d (%v)", v, err)
	}
	if err := cfg.Restrict(rules...); err != nil {
		return fmt.Errorf("desk cell worker: Landlock: %w", err)
	}
	// Scoped IPC (V6): no signals or abstract unix sockets outside this
	// worker's Landlock domain. Best effort on every level; the private
	// network namespace and the seccomp filter already cover both.
	if err := landlock.V6.BestEffort().RestrictScoped(); err != nil && level == cellSandboxRequired {
		return fmt.Errorf("desk cell worker: Landlock scope: %w", err)
	}
	return nil
}

// cellCanOpen reports whether this worker can open path at all. A root the
// cell user cannot reach (a 0700 directory of the runtime's) gets no grant
// instead of failing the whole ruleset: skipping a grant only removes access.
func cellCanOpen(path string) bool {
	fd, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return false
	}
	_ = unix.Close(fd)
	return true
}

var (
	cellAllow = seccomp.ActionAllow
	cellDeny  = seccomp.ActionErrno // EPERM
	// cellENOSYS answers clone3 so glibc (the binary is cgo: Dolt/ICU) falls
	// back to clone, whose flags the filter can inspect.
	cellENOSYS = seccomp.Action(uint32(seccomp.ActionErrno) | uint32(unix.ENOSYS))
)

// deskCellSyscalls is what a confined worker may call. It starts from the
// capsule workload list (proven for the same Yaegi session worker) and
// removes everything a worker never needs once it serves cells: socket,
// socketpair, connect, bind, listen and accept (it only uses fd 3), exec and
// fork (threads go through the conditioned clone below), the set*id family,
// chown, chmod, links, prctl, capset, memfd_create, splice and vmsplice,
// sendfile, sched_setaffinity, mlock. Names this architecture lacks (open,
// stat, fork, ... on arm64) are dropped at build time.
var deskCellSyscalls = []string{
	"read", "write", "readv", "writev", "pread64", "pwrite64", "close", "close_range",
	"open", "openat", "openat2", "stat", "fstat", "lstat", "newfstatat", "statx", "statfs", "fstatfs",
	"access", "faccessat", "faccessat2", "lseek", "getdents", "getdents64", "getcwd", "chdir", "fchdir",
	"mkdir", "mkdirat", "rmdir", "rename", "renameat", "renameat2", "unlink", "unlinkat",
	"readlink", "readlinkat", "truncate", "ftruncate", "umask", "fsync", "fdatasync",
	"mmap", "mprotect", "munmap", "mremap", "madvise", "brk", "mincore",
	"rt_sigaction", "rt_sigprocmask", "rt_sigreturn", "sigaltstack", "restart_syscall",
	"ioctl", "fcntl", "flock", "dup", "dup2", "dup3", "pipe", "pipe2", "poll", "ppoll", "select", "pselect6",
	"epoll_create", "epoll_create1", "epoll_ctl", "epoll_wait", "epoll_pwait", "epoll_pwait2", "eventfd", "eventfd2",
	"shutdown", "sendto", "recvfrom", "sendmsg", "recvmsg", "getsockname", "getpeername", "setsockopt", "getsockopt",
	"exit", "exit_group",
	"getpid", "getppid", "gettid", "getuid", "geteuid", "getgid", "getegid", "getresuid", "getresgid", "getgroups",
	"set_tid_address", "set_robust_list", "rseq",
	"futex", "sched_yield", "sched_getaffinity", "nanosleep", "clock_nanosleep",
	"clock_gettime", "clock_getres", "gettimeofday", "time", "times", "getitimer", "setitimer",
	"getrlimit", "getrusage", "sysinfo", "uname", "getrandom", "arch_prctl",
}

// deskCellDeniedSyscalls are the classes a red team named (items 3 and 5),
// denied by name with EPERM. The profile is deny by default, so these are
// already outside the allow list; naming them keeps them out when someone
// adds to the list (the profile test fails if any name is both), and puts
// the intent where a reader of the filter finds it. Names this architecture
// lacks are dropped at build time. Namespace clones are denied by clone's
// flag condition below, not by name.
var deskCellDeniedSyscalls = []string{
	// io_uring: its own syscall surface, repeatedly the source of local
	// privilege escalations and a way around seccomp on the calls it queues.
	"io_uring_setup", "io_uring_enter", "io_uring_register",
	// kernel keyrings.
	"keyctl", "add_key", "request_key",
	// eBPF and perf events: kernel attack surface and side channels.
	"bpf", "perf_event_open",
	// userfaultfd: stalls the kernel on a fault the attacker controls, the
	// classic race-widening primitive.
	"userfaultfd",
	// tracing and cross-process memory.
	"ptrace", "process_vm_readv", "process_vm_writev", "kcmp", "pidfd_open", "pidfd_getfd", "pidfd_send_signal",
	// namespaces, mounts, root.
	"unshare", "setns", "mount", "umount2", "pivot_root", "chroot", "open_tree", "move_mount", "fsopen", "fsconfig", "fsmount", "fspick", "mount_setattr",
	// process creation and replacement.
	"execve", "execveat", "fork", "vfork",
	// sockets: the worker's only channel is fd 3.
	"socket", "socketpair", "connect", "bind", "listen", "accept", "accept4",
	// credentials and capabilities.
	"setuid", "setgid", "setresuid", "setresgid", "setreuid", "setregid", "setgroups", "setfsuid", "setfsgid", "capset",
	// loading code into the kernel, reboot, clocks, swap.
	"init_module", "finit_module", "delete_module", "kexec_load", "kexec_file_load", "reboot", "settimeofday", "clock_settime", "clock_adjtime", "adjtimex", "swapon", "swapoff",
	// other fd factories and kernel interfaces that were never needed.
	"memfd_create", "memfd_secret", "inotify_init", "inotify_init1", "fanotify_init", "name_to_handle_at", "open_by_handle_at",
	"personality", "prctl", "seccomp", "modify_ldt", "iopl", "ioperm", "acct", "quotactl", "lookup_dcookie", "vhangup", "sysfs", "syslog", "_sysctl", "uselib", "ustat",
	"landlock_create_ruleset", "landlock_add_rule", "landlock_restrict_self",
}

// cloneNamespaceFlags are the CLONE_NEW* flags: a thread clone never carries
// one. (The kernel already refuses CLONE_NEWUSER and CLONE_NEWPID together
// with CLONE_THREAD; the filter does not rely on that.)
const cloneNamespaceFlags = unix.CLONE_NEWNS | unix.CLONE_NEWCGROUP | unix.CLONE_NEWUTS | unix.CLONE_NEWIPC |
	unix.CLONE_NEWUSER | unix.CLONE_NEWPID | unix.CLONE_NEWNET | unix.CLONE_NEWTIME

// DeskCellSeccompFilter is the worker's deny-by-default filter. pid is the
// worker's own pid: kill and tgkill reach only it, prlimit64 only reads it (the Go runtime signals its
// own threads for preemption). clone is admitted only with CLONE_THREAD and
// no CLONE_NEW* flag, so the worker can grow threads but never a process or
// a namespace.
func DeskCellSeccompFilter(pid int) seccomp.Filter {
	names := deskCellSyscalls
	if info, err := arch.GetInfo(""); err == nil {
		names = make([]string, 0, len(deskCellSyscalls))
		for _, n := range deskCellSyscalls {
			if _, ok := info.SyscallNames[n]; ok {
				names = append(names, n)
			}
		}
	}
	denied := make([]string, 0, len(deskCellDeniedSyscalls))
	if info, err := arch.GetInfo(""); err == nil {
		for _, n := range deskCellDeniedSyscalls {
			if _, ok := info.SyscallNames[n]; ok {
				denied = append(denied, n)
			}
		}
	} else {
		denied = deskCellDeniedSyscalls
	}
	self := uint64(pid)
	return seccomp.Filter{
		NoNewPrivs: true,
		Flag:       seccomp.FilterFlagTSync,
		Policy: seccomp.Policy{
			DefaultAction: cellDeny,
			Syscalls: []seccomp.SyscallGroup{
				{Action: cellDeny, Names: denied},
				{Action: cellAllow, Names: names},
				{Action: cellAllow, NamesWithCondtions: []seccomp.NameWithConditions{
					{Name: "clone", Conditions: seccomp.ArgumentConditions{
						{Argument: 0, Operation: seccomp.BitsSet, Value: unix.CLONE_THREAD},
						{Argument: 0, Operation: seccomp.BitsNotSet, Value: cloneNamespaceFlags}}},
					{Name: "kill", Conditions: seccomp.ArgumentConditions{{Argument: 0, Operation: seccomp.Equal, Value: self}}},
					{Name: "tgkill", Conditions: seccomp.ArgumentConditions{{Argument: 0, Operation: seccomp.Equal, Value: self}}},
					// prlimit64 reads only: the target is the worker itself (pid 0
					// or its own pid) and new_limit is null. Same-UID siblings
					// share credentials, so an unconditioned prlimit64 could lower
					// another worker's limits.
					{Name: "prlimit64", Conditions: seccomp.ArgumentConditions{
						{Argument: 0, Operation: seccomp.Equal, Value: 0},
						{Argument: 2, Operation: seccomp.Equal, Value: 0}}},
					{Name: "prlimit64", Conditions: seccomp.ArgumentConditions{
						{Argument: 0, Operation: seccomp.Equal, Value: self},
						{Argument: 2, Operation: seccomp.Equal, Value: 0}}},
				}},
				{Action: cellENOSYS, Names: []string{"clone3"}},
			},
		},
	}
}

// discard undoes a plan whose worker did not start: its tmp dir and uid.
func (p cellSpawnPlan) discard() {
	removeCellTmpDir(p.tmpDir)
	if p.release != nil {
		p.release()
	}
}
