//go:build linux

package capsule

import (
	"fmt"
	"runtime"

	"github.com/elastic/go-seccomp-bpf"
	"github.com/elastic/go-seccomp-bpf/arch"
	"golang.org/x/sys/unix"
)

var denyEPERM = seccomp.ActionErrno

var capsuleAllowedSyscalls = []string{
	"read", "write", "readv", "writev", "pread64", "pwrite64", "close", "close_range",
	"open", "openat", "openat2", "creat", "stat", "fstat", "lstat", "newfstatat", "statx", "statfs", "fstatfs",
	"access", "faccessat", "faccessat2", "lseek", "getdents", "getdents64", "getcwd", "chdir", "fchdir",
	"mkdir", "mkdirat", "rmdir", "rename", "renameat", "renameat2", "link", "linkat", "unlink", "unlinkat",
	"symlink", "symlinkat", "readlink", "readlinkat", "chmod", "fchmod", "fchmodat", "fchmodat2", "chown", "fchown", "lchown", "fchownat",
	"truncate", "ftruncate", "umask", "utime", "utimes", "futimesat", "utimensat", "fsync", "fdatasync", "syncfs",
	"mmap", "mprotect", "munmap", "mremap", "madvise", "brk", "mlock", "mlock2", "munlock", "mincore",
	"rt_sigaction", "rt_sigprocmask", "rt_sigreturn", "sigaltstack", "restart_syscall",
	"ioctl", "fcntl", "flock", "dup", "dup2", "dup3", "pipe", "pipe2", "poll", "ppoll", "select", "pselect6",
	"epoll_create", "epoll_create1", "epoll_ctl", "epoll_wait", "epoll_pwait", "epoll_pwait2", "eventfd", "eventfd2",
	"inotify_init", "inotify_init1", "inotify_add_watch", "inotify_rm_watch",
	"socketpair", "connect", "bind", "listen", "accept", "accept4", "shutdown", "sendto", "recvfrom", "sendmsg", "recvmsg",
	"getsockname", "getpeername", "setsockopt", "getsockopt",
	"fork", "vfork", "setpgid", "getpgid", "getpgrp", "setsid", "getsid", "execve", "execveat", "exit", "exit_group", "wait4", "waitid",
	"getpid", "getppid", "gettid", "getuid", "geteuid", "getgid", "getegid", "getresuid", "getresgid", "getgroups",
	"setuid", "setgid", "setreuid", "setregid", "setresuid", "setresgid", "setgroups", "setfsuid", "setfsgid",
	"kill", "tkill", "tgkill", "prctl", "capget", "capset", "set_tid_address", "set_robust_list", "rseq",
	"futex", "sched_yield", "sched_getaffinity", "sched_setaffinity", "nanosleep", "clock_nanosleep",
	// Read-only scheduling queries: pthread_getschedparam (every Node process)
	// calls them, and refused it prints "pthread_getschedparam failed: 1" in a
	// flood (docs/problems/engineering-completion-path-cluster-2026-10-20.md
	// failure 5). The setters stay denied.
	"sched_getparam", "sched_getscheduler",
	"clock_gettime", "clock_getres", "gettimeofday", "time", "times", "getitimer", "setitimer", "alarm",
	"getrlimit", "setrlimit", "prlimit64", "getrusage", "sysinfo", "uname", "getrandom",
	"sendfile", "copy_file_range", "splice", "tee", "vmsplice", "memfd_create", "arch_prctl",
}

// cloneNewUser is CLONE_NEWUSER, the flag a workload must never pass to clone.
const cloneNewUser = 0x10000000

// refuseENOSYS answers a syscall as if the kernel lacked it. clone3 gets it
// (SEC-13): its flags are in a struct seccomp cannot read, so allowing it
// would admit any CLONE_NEW* flag unconditionally. libc and the Go runtime
// fall back to clone, whose flags the filter reads.
var refuseENOSYS = seccomp.ActionErrno | seccomp.Action(unix.ENOSYS)

// archSyscalls returns names as this architecture knows them. The lists are
// written for amd64, the guest's architecture, where nothing is dropped and a
// misspelt name still fails the load; arm64 (a dev host) lacks the legacy
// names (open, fork, dup2, poll, ...) and drops them, since its libc reaches
// the same calls through their *at and newer forms, which the lists carry.
func archSyscalls(names []string) []string {
	if runtime.GOARCH != "arm64" {
		return names
	}
	info, err := arch.GetInfo("")
	if err != nil {
		return names
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if _, ok := info.SyscallNames[n]; ok {
			out = append(out, n)
		}
	}
	return out
}

// capsuleXattrSyscalls stay refused, but with EOPNOTSUPP rather than the
// default EPERM. Tools read EOPNOTSUPP as "this filesystem has no xattrs"
// and fall back (cp sets permissions with chmod instead of an ACL); EPERM
// made every first release copy fail with "setting permissions"
// (docs/problems/engineering-completion-path-cluster-2026-10-20.md failure
// 24). Only the errno differs: nothing here is allowed.
var capsuleXattrSyscalls = []string{
	"setxattr", "lsetxattr", "fsetxattr", "setxattrat",
	"getxattr", "lgetxattr", "fgetxattr", "getxattrat",
	"listxattr", "llistxattr", "flistxattr", "listxattrat",
	"removexattr", "lremovexattr", "fremovexattr", "removexattrat",
}

var refuseEOPNOTSUPP = seccomp.ActionErrno | seccomp.Action(unix.EOPNOTSUPP)

// WorkloadSeccompFilter is default-deny. It admits the file/process/memory
// substrate needed by the broker and offline build tools plus AF_UNIX only.
func WorkloadSeccompFilter() seccomp.Filter {
	return seccompFilterAllowing(capsuleAllowedSyscalls, false)
}

// WorkloadInetSeccompFilter also admits AF_INET and AF_INET6 sockets. It is
// loaded only in an ecosystem_proxy (L2) capsule, whose network namespace
// has loopback as its one reachable address, so TCP reaches nothing but the
// broker's egress forwarder (engineering-network-grants-2026-10-10.md
// §4.0b). Netlink, packet and every other family stay refused.
func WorkloadInetSeccompFilter() seccomp.Filter {
	return seccompFilterAllowing(capsuleAllowedSyscalls, true)
}

func seccompFilterAllowing(allowed []string, inet bool) seccomp.Filter {
	families := []uint64{unix.AF_UNIX}
	if inet {
		families = append(families, unix.AF_INET, unix.AF_INET6)
	}
	socketRules := make([]seccomp.NameWithConditions, 0, len(families))
	for _, family := range families {
		socketRules = append(socketRules, seccomp.NameWithConditions{
			Name: "socket", Conditions: seccomp.ArgumentConditions{{Argument: 0, Operation: seccomp.Equal, Value: family}},
		})
	}
	return seccomp.Filter{
		NoNewPrivs: true,
		Flag:       seccomp.FilterFlagTSync,
		Policy: seccomp.Policy{
			DefaultAction: denyEPERM,
			Syscalls: []seccomp.SyscallGroup{
				{Action: seccomp.ActionAllow, Names: archSyscalls(allowed)},
				{Action: seccomp.ActionAllow, NamesWithCondtions: socketRules},
				// SEC-13: clone, but never into a new user namespace (a
				// nested user namespace hands the workload a full capability
				// set inside it); CLONE_NEWPID and the other flags the broker
				// uses to start its children are unchanged. clone3 answers
				// ENOSYS so libc falls back to clone.
				{Action: seccomp.ActionAllow, NamesWithCondtions: []seccomp.NameWithConditions{
					{Name: "clone", Conditions: seccomp.ArgumentConditions{{Argument: 0, Operation: seccomp.BitsNotSet, Value: cloneNewUser}}},
				}},
				{Action: refuseENOSYS, Names: []string{"clone3"}},
				{Action: refuseEOPNOTSUPP, Names: archSyscalls(capsuleXattrSyscalls)},
			},
		},
	}
}

// brokerExtraSyscalls are allowed to the broker but not to workloads. The
// session worker inherits the broker's filter, applies its own Landlock,
// then stacks the workload filter with seccomp(); refusing these made every
// worker die before ready (capsule-session-worker-dies-at-start-2026-10-09).
// Both can only remove access.
var brokerExtraSyscalls = []string{"landlock_create_ruleset", "landlock_add_rule", "landlock_restrict_self", "seccomp"}

func BrokerSeccompFilter() seccomp.Filter {
	allowed := append(append([]string(nil), capsuleAllowedSyscalls...), brokerExtraSyscalls...)
	return seccompFilterAllowing(allowed, false)
}

// BrokerInetSeccompFilter is the L2 broker filter: the broker's loopback
// forwarder and its children (direct exec, session workers) need inet
// sockets, and a child cannot gain what its parent's filter refuses.
func BrokerInetSeccompFilter() seccomp.Filter {
	allowed := append(append([]string(nil), capsuleAllowedSyscalls...), brokerExtraSyscalls...)
	return seccompFilterAllowing(allowed, true)
}

// delegateExtraSyscalls are what the Claude delegate's binary (Claude Code,
// built on Bun) needs beyond the workload allowlist
// (docs/archive/problems/claude-delegate-bun-timer-seccomp-2026-10-15.md).
// uSockets arms its loop timer through a timerfd; refused, Bun panics at
// start ("us_create_timer returned null"). JavaScriptCore parks a thread for
// the collector with SIGPWR and rt_sigsuspend; refused, sigsuspend returns
// at once and the thread runs on while its stack is scanned. Only the
// delegate stage and the broker it inherits from get them; cells and jobs
// keep stacking the workload filter.
var delegateExtraSyscalls = []string{"timerfd_create", "timerfd_settime", "timerfd_gettime", "rt_sigsuspend"}

// DelegateSeccompFilter is the delegate stage's filter: the L2 workload
// filter (the delegate needs the model API through the egress forwarder)
// plus the Bun runtime calls above.
func DelegateSeccompFilter() seccomp.Filter {
	allowed := append(append([]string(nil), capsuleAllowedSyscalls...), delegateExtraSyscalls...)
	return seccompFilterAllowing(allowed, true)
}

// BrokerDelegateSeccompFilter is the L2 broker filter of a capsule that
// serves the Claude delegate. The delegate stage inherits the broker's
// filter and seccomp answers with the strictest filter of the stack, so the
// broker must admit the delegate's calls too. Its session workers and jobs
// still stack the workload filter, which refuses them.
func BrokerDelegateSeccompFilter() seccomp.Filter {
	allowed := append(append(append([]string(nil), capsuleAllowedSyscalls...), brokerExtraSyscalls...), delegateExtraSyscalls...)
	return seccompFilterAllowing(allowed, true)
}

func LoadDelegateFilter() error {
	if err := seccomp.LoadFilter(DelegateSeccompFilter()); err != nil {
		return fmt.Errorf("failed to load claude delegate seccomp filter: %w", err)
	}
	return nil
}

func LoadBrokerDelegateFilter() error {
	if err := seccomp.LoadFilter(BrokerDelegateSeccompFilter()); err != nil {
		return fmt.Errorf("failed to load claude delegate broker seccomp filter: %w", err)
	}
	return nil
}

func LoadWorkloadFilter() error {
	if err := seccomp.LoadFilter(WorkloadSeccompFilter()); err != nil {
		return fmt.Errorf("failed to load workload seccomp filter: %w", err)
	}
	return nil
}

func LoadWorkloadInetFilter() error {
	if err := seccomp.LoadFilter(WorkloadInetSeccompFilter()); err != nil {
		return fmt.Errorf("failed to load L2 workload seccomp filter: %w", err)
	}
	return nil
}

func LoadBrokerInetFilter() error {
	if err := seccomp.LoadFilter(BrokerInetSeccompFilter()); err != nil {
		return fmt.Errorf("failed to load L2 broker seccomp filter: %w", err)
	}
	return nil
}

func LoadBrokerFilter() error {
	if err := seccomp.LoadFilter(BrokerSeccompFilter()); err != nil {
		return fmt.Errorf("failed to load broker seccomp filter: %w", err)
	}
	return nil
}
