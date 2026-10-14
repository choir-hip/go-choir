//go:build linux

package yaegikernel

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	seccomp "github.com/elastic/go-seccomp-bpf"
	"github.com/elastic/go-seccomp-bpf/arch"
	"github.com/landlock-lsm/go-landlock/landlock"
	ll "github.com/landlock-lsm/go-landlock/landlock/syscall"
	"golang.org/x/sys/unix"
)

// cellSpawnPlan is what SpawnDeskSessionWorker applies to the worker's
// exec.Cmd.
type cellSpawnPlan struct {
	attr   *syscall.SysProcAttr
	env    []string
	dir    string
	tmpDir string
}

// planCellConfinement resolves the spawner half (see desk_cell_confinement.go).
// It refuses rather than weakens: a root spawner without a cell identity for
// the role, or required mode where the identity switch is impossible, is an
// error and no worker starts.
func planCellConfinement(cfg DeskSessionWorkerConfig) (cellSpawnPlan, error) {
	attr := workerSysProcAttr(cfg.ProcessGroup)
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}
	plan := cellSpawnPlan{attr: attr, dir: cfg.Session.AllowedRoot}
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
		name := cfg.Confinement.User
		if name == "" {
			name = DeskCellUser(cfg.Session.Role)
		}
		if name == "" {
			return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: no cell user for desk role %q", cfg.Session.Role)
		}
		if !root {
			return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: required mode needs a root spawner to run cells as %s (running as uid %d)", name, os.Geteuid())
		}
		u, err := user.Lookup(name)
		if err != nil {
			return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: cell user %s: %w", name, err)
		}
		uid, err = strconv.Atoi(u.Uid)
		if err != nil {
			return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: cell user %s uid %q: %w", name, u.Uid, err)
		}
		gid, err = strconv.Atoi(u.Gid)
		if err != nil {
			return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: cell user %s gid %q: %w", name, u.Gid, err)
		}
		if uid == 0 || gid == 0 {
			return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: cell user %s resolves to uid %d gid %d", name, uid, gid)
		}
		// Go sets Pdeathsig after the credential change, so the kernel's
		// reset of the parent-death signal on setuid does not clear it.
		attr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: []uint32{}}
		// An empty network namespace: the worker's only channel is fd 3.
		// Abstract unix sockets live in the network namespace too.
		attr.Cloneflags |= syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC
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
	if err := restrictCellFilesystem(level, getenv(prefix+"ALLOWED_ROOT"), tmp); err != nil {
		return err
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
	"getrlimit", "prlimit64", "getrusage", "sysinfo", "uname", "getrandom", "arch_prctl",
}

// DeskCellSeccompFilter is the worker's deny-by-default filter. pid is the
// worker's own pid: kill and tgkill reach only it (the Go runtime signals its
// own threads for preemption). clone is admitted only with CLONE_THREAD, so
// the worker can grow threads but never a process or a namespace.
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
	self := uint64(pid)
	return seccomp.Filter{
		NoNewPrivs: true,
		Flag:       seccomp.FilterFlagTSync,
		Policy: seccomp.Policy{
			DefaultAction: cellDeny,
			Syscalls: []seccomp.SyscallGroup{
				{Action: cellAllow, Names: names},
				{Action: cellAllow, NamesWithCondtions: []seccomp.NameWithConditions{
					{Name: "clone", Conditions: seccomp.ArgumentConditions{{Argument: 0, Operation: seccomp.BitsSet, Value: unix.CLONE_THREAD}}},
					{Name: "kill", Conditions: seccomp.ArgumentConditions{{Argument: 0, Operation: seccomp.Equal, Value: self}}},
					{Name: "tgkill", Conditions: seccomp.ArgumentConditions{{Argument: 0, Operation: seccomp.Equal, Value: self}}},
				}},
				{Action: cellENOSYS, Names: []string{"clone3"}},
			},
		},
	}
}
