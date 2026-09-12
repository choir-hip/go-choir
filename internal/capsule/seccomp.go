//go:build linux

package capsule

import (
	"fmt"

	"github.com/elastic/go-seccomp-bpf"
	"golang.org/x/sys/unix"
)

var denyEPERM = seccomp.ActionErrno

var capsuleAllowedSyscalls = []string{
	"read", "write", "readv", "writev", "pread64", "pwrite64", "close", "close_range",
	"open", "openat", "openat2", "creat", "stat", "fstat", "lstat", "newfstatat", "statx",
	"access", "faccessat", "faccessat2", "lseek", "getdents", "getdents64", "getcwd", "chdir", "fchdir",
	"mkdir", "mkdirat", "rmdir", "rename", "renameat", "renameat2", "link", "linkat", "unlink", "unlinkat",
	"symlink", "symlinkat", "readlink", "readlinkat", "chmod", "fchmod", "fchmodat", "chown", "fchown", "lchown", "fchownat",
	"truncate", "ftruncate", "umask", "utime", "utimes", "futimesat", "utimensat", "fsync", "fdatasync", "syncfs",
	"mmap", "mprotect", "munmap", "mremap", "madvise", "brk", "mlock", "munlock", "mincore",
	"rt_sigaction", "rt_sigprocmask", "rt_sigreturn", "sigaltstack", "restart_syscall",
	"ioctl", "fcntl", "flock", "dup", "dup2", "dup3", "pipe", "pipe2", "poll", "ppoll", "select", "pselect6",
	"epoll_create", "epoll_create1", "epoll_ctl", "epoll_wait", "epoll_pwait", "eventfd", "eventfd2",
	"inotify_init", "inotify_init1", "inotify_add_watch", "inotify_rm_watch",
	"socketpair", "connect", "bind", "listen", "accept", "accept4", "shutdown", "sendto", "recvfrom", "sendmsg", "recvmsg",
	"getsockname", "getpeername", "setsockopt", "getsockopt",
	"clone", "clone3", "fork", "vfork", "execve", "execveat", "exit", "exit_group", "wait4", "waitid",
	"getpid", "getppid", "gettid", "getuid", "geteuid", "getgid", "getegid", "getresuid", "getresgid", "getgroups",
	"setuid", "setgid", "setreuid", "setregid", "setresuid", "setresgid", "setgroups", "setfsuid", "setfsgid",
	"kill", "tkill", "tgkill", "prctl", "capget", "capset", "set_tid_address", "set_robust_list", "rseq",
	"futex", "sched_yield", "sched_getaffinity", "sched_setaffinity", "nanosleep", "clock_nanosleep",
	"clock_gettime", "clock_getres", "gettimeofday", "time", "times", "getitimer", "setitimer", "alarm",
	"getrlimit", "setrlimit", "prlimit64", "getrusage", "sysinfo", "uname", "getrandom",
	"sendfile", "copy_file_range", "splice", "tee", "vmsplice", "memfd_create", "arch_prctl",
}

// WorkloadSeccompFilter is default-deny. It admits the file/process/memory
// substrate needed by the broker and offline build tools plus AF_UNIX only.
func WorkloadSeccompFilter() seccomp.Filter {
	return seccomp.Filter{
		NoNewPrivs: true,
		Flag:       seccomp.FilterFlagTSync,
		Policy: seccomp.Policy{
			DefaultAction: denyEPERM,
			Syscalls: []seccomp.SyscallGroup{
				{Action: seccomp.ActionAllow, Names: capsuleAllowedSyscalls},
				{Action: seccomp.ActionAllow, NamesWithCondtions: []seccomp.NameWithConditions{{
					Name: "socket", Conditions: seccomp.ArgumentConditions{{Argument: 0, Operation: seccomp.Equal, Value: uint64(unix.AF_UNIX)}},
				}}},
			},
		},
	}
}

func BrokerSeccompFilter() seccomp.Filter { return WorkloadSeccompFilter() }

func LoadWorkloadFilter() error {
	if err := seccomp.LoadFilter(WorkloadSeccompFilter()); err != nil {
		return fmt.Errorf("failed to load workload seccomp filter: %w", err)
	}
	return nil
}

func LoadBrokerFilter() error {
	if err := seccomp.LoadFilter(BrokerSeccompFilter()); err != nil {
		return fmt.Errorf("failed to load broker seccomp filter: %w", err)
	}
	return nil
}
