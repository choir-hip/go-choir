# Capsule Runtime Implementation Plan — v14

**Status:** Synthesis of four parallel research subagents + 8+6+4+5+5+5+5 agent consensus.
Updated with v14 fixes: session_id contradiction fixed (now correctly
described as broker-minted random ID, not agentRunID).
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Supersedes:** v13 (session_id fix)

## Executive Summary

No single open source project does exactly what Choir's capsule runtime needs.
But the problem is well-solved by assembling focused, production-tested Go
libraries. The key finding is that we can build the entire runtime in pure Go
(no CGO) using libraries that are actively maintained and production-tested.

**The paint-by-numbers stack:**

| Component | Library | CGO | Why |
|-----------|---------|-----|-----|
| Namespaces | `cpuguy83/gonso` | No | Handles LockOSThread correctly |
| Overlayfs | Direct `unix.Mount()` | No | Simplest, no dependency |
| cgroups v2 | `containerd/cgroups/v3` | No | OCI-compatible, well-maintained |
| seccomp | `elastic/go-seccomp-bpf` | No | Pure Go BPF filter generation |
| Landlock | `landlock-lsm/go-landlock` | No | Official, maintained by LSM author |
| Capabilities | `moby/sys/capability` | No | Active fork of syndtr/gocapability |
| PTY | `creack/pty` | No | Standard Go PTY library |
| Network policy | (none — air-gapped) | N/A | CLONE_NEWNET per capsule (no interfaces), host-mediated I/O |
| Overlay diff | `containerd/continuity` | No | DiffDirChanges fast path |

**All pure Go. No CGO. No daemon. No external binary dependencies.**

## What We Rejected and Why

### runc/libcontainer

**Verdict: Rejected.**

- runc maintainers explicitly state libcontainer is "not intended for external
  consumption" with an "unstable API" not covered by SemVer.
- Ongoing effort to move packages into `internal/` (runc issue #3028).
- Heavy dependency tree (cgroups, runtime-spec, selinux, moby/sys, moby/term).
- Requires CGO for the nsenter C constructor pattern.
- Docker, containerd, and Kubernetes are all actively removing libcontainer
  dependencies.

### containerd

**Verdict: Rejected.**

- Requires a daemon. The Go client only communicates via gRPC to a running
  containerd daemon. No in-process mode exists.
- 100+ direct dependencies, 30%+ binary size impact.
- Designed for image distribution, orchestration, plugin architecture — all
  irrelevant to lightweight ephemeral capsules.
- The sandbox API is for Kubernetes pods, not single-container capsules.

### Nucleus

**Verdict: Rejected** (per capsule-runtime-decision-v0.md).

- No programmatic API (CLI-only Rust tool).
- Overlay mode requires CAP_DAC_OVERRIDE + CAP_FOWNER (kernel requirement,
  same as any overlayfs approach).
- Nix store integration is incidental to Choir's needs.
- HMAC key in VM is insufficient trust anchor → v3: Ed25519, private key on host.

### gVisor (runsc)

**Verdict: Rejected (production-only, no MVP). Reconsider for high-security capsules.**

- Userspace kernel approach provides strongest isolation.
- But reimplements Linux in Go — heavyweight, complex integration.
- Performance overhead per syscall.
- Overkill for bash tool isolation where the threat is capsule escape, not
  syscall-level attack.

## The Library Stack (Detailed)

### 1. Namespaces: `github.com/cpuguy83/gonso`

**Why:** Go's threading model makes namespace manipulation dangerous —
namespaces are per-thread, but Go's scheduler moves goroutines between OS
threads. gonso handles `runtime.LockOSThread` correctly internally.

**API:**
```go
current, _ := gonso.Current()
newSet, _ := current.Unshare(
    unix.CLONE_NEWNS | unix.CLONE_NEWPID | unix.CLONE_NEWNET | // CLONE_NEWNET for abstract Unix socket isolation
    unix.CLONE_NEWUTS | unix.CLONE_NEWIPC | unix.CLONE_NEWUSER, // NEWUSER for broker privilege separation (workload retains root for overlayfs)
)
newSet.Do(false, func() bool {
    // Code runs in new namespace
    return false
})
```

**Alternative:** Direct `golang.org/x/sys/unix` calls with manual
`runtime.LockOSThread` management. More flexible but error-prone. Use gonso
for safety, fall back to direct syscalls if gonso's API is too restrictive.

**Status:** 35 stars, active, used by Moby/buildkit. Pure Go.

### 2. Overlayfs: Direct `unix.Mount()` calls

**Why:** No library needed. Overlayfs mounting is a single syscall with
options string.

**API:**
```go
// Mount EROFS base as read-only lower
unix.Mount("/dev/vd0", "/capsule/base", "erofs", unix.MS_RDONLY, "")

// Mount tmpfs for upper/work
unix.Mount("tmpfs", "/capsule/upper", "tmpfs", 0, "size=512M")
unix.Mount("tmpfs", "/capsule/work", "tmpfs", 0, "size=64M")

// Mount overlay
opts := "lowerdir=/capsule/base,upperdir=/capsule/upper,workdir=/capsule/work"
unix.Mount("overlay", "/capsule/merged", "overlay", 0, opts)
```

**EROFS as lowerdir:** Fully supported by the kernel. No special options
needed. EROFS is just another read-only filesystem to overlayfs.

**Gotcha:** Page size limit on mount options (4096 bytes). Not an issue for
single lowerdir. Kernel >= 6.13 recommended (6.12.63 had an EROFS+overlay
regression).

### 3. cgroups v2: `github.com/containerd/cgroups/v3/cgroup2`

**Why:** Production-tested, OCI runtime-spec compatible, supports unified
hierarchy. Used by containerd.

**API:**
```go
import "github.com/containerd/cgroups/v3/cgroup2"

res := cgroup2.Resources{
    CPU: &cgroup2.CPU{
        Max: cgroup2.NewCPUMax(&quota, &period),
    },
    Memory: &cgroup2.Memory{
        Max: pointer.Int64(512 * 1024 * 1024),
    },
    Pids: &cgroup2.Pids{
        Max: pointer.Int64(100),
    },
}
m, err := cgroup2.NewSystemd("/", "capsule-abc.slice", -1, &res)
// Add process to cgroup
m.AddProc(pid)
```

**Alternative:** Direct filesystem writes to `/sys/fs/cgroup/`. Simpler but
error-prone. Use containerd/cgroups for correctness.

### 4. seccomp: `github.com/elastic/go-seccomp-bpf`

**Why:** Pure Go! No CGO dependency. This is a significant finding —
`libseccomp-golang` requires CGO, but `elastic/go-seccomp-bpf` generates BPF
filters in pure Go. Supports `FilterFlagTSync` which is critical for Go's
threading model (applies filter to all threads).

**API:**
```go
import (
    "github.com/elastic/go-seccomp-bpf"
    "golang.org/x/sys/unix"
)

// Workload seccomp: default-allow with targeted denylist + socket family filtering.
// The socket family filter is the v8 network enforcement mechanism.
// NOTE: ActionErrno's low 16 bits carry the errno value. Use ActionErrno | Action(unix.EPERM)
// to ensure denied syscalls return EPERM, not success (errno=0).
denyEPERM := seccomp.ActionErrno | seccomp.Action(unix.EPERM)

filter := seccomp.Filter{
    NoNewPrivs: true,
    Flag:       seccomp.FilterFlagTSync,
    Policy: seccomp.Policy{
        DefaultAction: seccomp.ActionAllow,
        Syscalls: []seccomp.SyscallGroup{
            {
                Action: denyEPERM,
                Names: []string{
                    "keyctl", "add_key", "request_key",
                    "ptrace", "process_vm_readv", "process_vm_writev",
                    "mount", "umount2", "pivot_root", "swapon", "swapoff",
                    "reboot", "init_module", "finit_module", "delete_module",
                    "kexec_load", "kexec_file_load",
                    "perf_event_open", "fanotify_init",
                    "bpf", "lookup_bpf_cookie",
                    "unshare", "setns", // prevent namespace escape
                },
            },
            // v13: Block socket() for network/vsock families by arg filtering.
            // seccomp-BPF can inspect socket()'s first arg (domain), but
            // cannot dereference sockaddr pointers in connect/bind/sendto.
            // AF_UNIX (1) is allowed for broker control plane.
            // Each denied family is its own SyscallGroup with NamesWithCondtions
            // (note: library has typo "Condtions"). Separate groups are ORed.
            {
                Action: denyEPERM,
                NamesWithCondtions: []seccomp.NameWithConditions{{
                    Name: "socket",
                    Conditions: seccomp.ArgumentConditions{{
                        Argument:  0, // socket(domain, type, protocol) — filter on domain
                        Operation: seccomp.Equal,
                        Value:     uint64(unix.AF_INET), // 2
                    }},
                }},
            },
            {
                Action: denyEPERM,
                NamesWithCondtions: []seccomp.NameWithConditions{{
                    Name: "socket",
                    Conditions: seccomp.ArgumentConditions{{
                        Argument:  0,
                        Operation: seccomp.Equal,
                        Value:     uint64(unix.AF_INET6), // 10
                    }},
                }},
            },
            {
                Action: denyEPERM,
                NamesWithCondtions: []seccomp.NameWithConditions{{
                    Name: "socket",
                    Conditions: seccomp.ArgumentConditions{{
                        Argument:  0,
                        Operation: seccomp.Equal,
                        Value:     uint64(unix.AF_NETLINK), // 16
                    }},
                }},
            },
            {
                Action: denyEPERM,
                NamesWithCondtions: []seccomp.NameWithConditions{{
                    Name: "socket",
                    Conditions: seccomp.ArgumentConditions{{
                        Argument:  0,
                        Operation: seccomp.Equal,
                        Value:     uint64(unix.AF_VSOCK), // 40
                    }},
                }},
            },
        },
    },
}
seccomp.LoadFilter(filter)

// Broker seccomp: default-deny allowlist (stricter than workload).
// Broker only needs: read, write, open, openat, close, fstat, lstat,
// stat, readlink, mkdir, mkdirat, unlink, unlinkat, rename, renameat,
// chmod, fchmod, symlink, symlinkat, truncate, ftruncate, fcntl,
// dup, dup2, dup3, pipe, pipe2, select, poll, epoll_wait, wait4,
// kill, tgkill, rt_sigaction, rt_sigprocmask, sigreturn, exit, exit_group,
// brk, mmap, munmap, mprotect, mremap, madvise, futex, getpid, gettid,
// getuid, geteuid, getgid, getegid, setuid, setgid, setgroups,
// clock_gettime, nanosleep, ioctl (for PTY), socket(AF_UNIX only),
// bind, listen, accept, connect, read, write (for AF_UNIX control socket),
// close_range, getdents64, readlinkat, faccessat, faccessat2, newfstatat,
// getrandom, prlimit64, getrlimit, setrlimit, arch_prctl.
// socket(AF_INET/AF_INET6/AF_NETLINK/AF_VSOCK) denied via arg filter (same as workload).
// NOTE: Sketch verified against elastic/go-seccomp-bpf upstream in v13.
// Pin library version and compile-test the filter in the first implementation PR.
```

**Design decision:** Default-allow with explicit denylist (rather than
default-deny with allowlist). A default-deny seccomp profile requires
enumerating every syscall the workload needs, which is fragile for arbitrary
bash commands. Default-allow with a targeted denylist of dangerous syscalls
is more practical for agent workloads.

**Status:** 93 stars, active, pure Go. Used by Elastic's auditbeat.

### 5. Landlock: `github.com/landlock-lsm/go-landlock`

**Why:** Official library, maintained by the Landlock LSM author (Mickaël
Salaün). Supports best-effort mode for graceful degradation on older kernels.

**API:**
```go
import "github.com/landlock-lsm/go-landlock/landlock"

// For overlayfs, we need read/write/execute on / (the merged overlay)
// But we can restrict access to host paths that shouldn't be visible
err := landlock.V9.BestEffort().RestrictPaths(
    landlock.RWDirs("/capsule/merged"),
    landlock.RODirs("/capsule/base"), // EROFS is read-only anyway
)
```

**Note:** With overlayfs, we must grant read/write/execute on `/` (the merged
mount point). Landlock cannot be default-deny in this configuration. This is
the trade-off documented in capsule-runtime-decision-v0.md — the elevated
capabilities (CAP_DAC_OVERRIDE, CAP_FOWNER) and broad Landlock access are the
cost of transparent overlayfs writes.

**Kernel requirement:** Linux 5.13+ with `CONFIG_SECURITY_LANDLOCK=y`.

**Status:** 298 stars, very active, maintained by LSM author. Pure Go.

### 6. Capabilities: `github.com/moby/sys/capability`

**Why:** Active fork of `syndtr/gocapability` (which is unmaintained).
Maintained by the Moby project. Pure Go.

**API:**
```go
import "github.com/moby/sys/capability"

caps, _ := capability.NewPid(0)
caps.Clear(capability.CAPS | capability.BOUNDS)
caps.Set(capability.CAPS, capability.CAP_DAC_OVERRIDE, capability.CAP_FOWNER)
caps.Apply(capability.CAPS | capability.BOUNDS)
```

**Design:** Drop ALL capabilities except CAP_DAC_OVERRIDE and CAP_FOWNER
(required for overlayfs copy-up). This matches the decision in
capsule-runtime-decision-v0.md.

**Status:** Part of moby/sys, very active. Pure Go.

### 7. PTY: `github.com/creack/pty`

**Why:** Standard Go PTY library, ~2K stars, actively maintained. Used
everywhere in the Go ecosystem.

**API:**
```go
import "github.com/creack/pty"

c := exec.Command("/bin/bash", "--norc", "-i")
ptmx, err := pty.Start(c)
// ptmx is *os.File — read stdout/stderr, write stdin
defer ptmx.Close()
```

**Status:** ~2K stars, active (v1.1.24). Pure Go.

### 8. Network Policy: Air-gapped (CLONE_NEWNET + seccomp)

**Design:** Capsules are air-gapped via two layers: (1) CLONE_NEWNET per
capsule creates an isolated network namespace with no interfaces, no
loopback, no routing — this prevents cross-capsule communication via
abstract Unix sockets (which are scoped by network namespace, not mount
namespace). (2) seccomp denies `socket(AF_INET)`, `socket(AF_INET6)`,
`socket(AF_NETLINK)`, and `socket(AF_VSOCK)` at creation time. AF_UNIX
is allowed for broker control plane (scoped by CLONE_NEWNET). All network
I/O is mediated by the host (executor). The broker has no network access
— it communicates only via the Unix domain socket to the executor. Agents
that need external access (e.g. researcher's dolt:write, message:send) do
so through host-mediated RPCs, not direct network calls.

### 9. Overlay Diff: `github.com/containerd/continuity/fs`

**Why:** `DiffDirChanges` is a fast single-walk diff that detects overlay
mounts and extracts the upperdir automatically. ~9x faster than double-walk.
Production-tested in containerd's overlay snapshotter.

**API:**
```go
import "github.com/containerd/continuity/fs"

changes, err := fs.DiffDirChanges(ctx, upperdir, lowerdir, true)
// changes is []fs.Change with Path, Kind (Add/Modify/Delete), etc.
```

**Whiteout handling:** The library handles overlayfs whiteouts (character
device 0/0 or xattr `trusted.overlay.whiteout`) and opaque directories
(xattr `trusted.overlay.opaque`).

**Alternative reference:** Dagger's `internal/buildkit/util/overlay` has a
Go implementation that explicitly checks `trusted.overlay.opaque` and
`user.overlay.opaque` xattrs. Useful as a reference for edge cases.

**Status:** Part of containerd org, active. Pure Go.

## The Persistent Shell Problem

The hardest design question is how to execute bash commands inside a capsule
from a Go goroutine. The research found several approaches:

### Option A: Per-command exec (simple, stateless)

Each bash tool invocation creates a new process inside the namespace via
`setns`. No state persists between commands (cwd, env vars, background
processes are lost).

**Latency:** ~1-5ms per namespace switch + fork/exec overhead.
**State:** None (each command starts fresh).
**Implementation:** Use gonso's `Do()` to enter namespace, exec command,
capture output.

### Option B: Persistent shell with sentinel framing (stateful, fast)

Start a long-lived `bash --norc -i` process inside the namespace with a PTY.
Communicate via stdin/stdout. Use a sentinel marker to detect command
completion.

**Latency:** ~0.1ms per command (no fork/exec, just pipe write).
**State:** Full shell state (cwd, env vars, background processes persist).
**Implementation:**
```go
// Start persistent bash
cmd := exec.Command("/bin/bash", "--norc", "-i")
cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags: unix.CLONE_NEWNS | unix.CLONE_NEWPID | ...,
}
ptmx, _ := pty.Start(cmd)

// Send command with sentinel
sentinel := fmt.Sprintf("__CHOIR_CMD_DONE_%d__", time.Now().UnixNano())
fmt.Fprintf(ptmx, "%s; echo %s\n", command, sentinel)

// Read until sentinel appears
scanner := bufio.NewScanner(ptmx)
for scanner.Scan() {
    line := scanner.Text()
    if strings.Contains(line, sentinel) {
        break
    }
    output += line + "\n"
}
```

**Problem:** Sentinel can appear in command output (e.g., `echo $sentinel`).
Mitigation: Use a randomized sentinel and check for it on its own line. Or
use a more robust framing protocol.

### Option C: Exec-broker (stateful, clean, recommended)

PID 1 inside the capsule is a small Go program that listens on a Unix socket.
The host sends JSON-RPC commands; the broker execs them and streams output.

**Latency:** ~0.5ms per command (Unix socket + fork/exec inside namespace).
**State:** Broker maintains state (cwd, env) across commands.
**Implementation:**
```go
// Broker (runs as PID 1 inside capsule)
type ExecRequest struct {
    Cmd     string   `json:"cmd"`
    Args    []string `json:"args"`
    Cwd     string   `json:"cwd"`
    Env     []string `json:"env"`
    Timeout int      `json:"timeout"`
}

type ExecResponse struct {
    ExitCode int    `json:"exit_code"`
    Stdout   string `json:"stdout"`
    Stderr   string `json:"stderr"`
}

// Listen on Unix socket (bind-mounted from host)
listener, _ := net.ListenUnix("unix", &net.UnixAddr{
    Name: "/capsule/broker.sock",
})
for {
    conn, _ := listener.AcceptUnix()
    // Read JSON request, exec command, write JSON response
}
```

**Advantages:**
- Clean framing (no sentinel parsing)
- Structured I/O (separate stdout/stderr/exit-code)
- Timeout support
- Multiple concurrent commands (each is a separate connection)
- Broker can enforce resource limits per command

**Disadvantages:**
- Need a small Go binary in the capsule rootfs
- Socket setup adds ~10ms to capsule creation
- Broker is a new attack surface (but it runs inside the namespace)

### Recommendation: Option C (exec-broker) with typed RPCs

**Production-only. No MVP, no sentinel framing, no phased delivery.**

The exec-broker is the only option. Sentinel framing (Option B) reintroduces
delimiter-collision bugs, no binary safety, and non-atomic writes — exactly
the fragility the broker was built to eliminate. The broker uses **typed
RPCs** for all operations, not exec wrappers via heredocs.

The broker is bind-mounted from a content-addressed host store (v2 decision).
The executor mounts it read-only into the capsule at spawn time and verifies
its content hash before exec. This enables minutes-scale broker hotfixes
without EROFS rebuilds. The broker version and binary hash are recorded in
commit metadata for auditability.

**Broker protocol (typed RPCs, not exec wrappers):**

| Method | Purpose | Notes |
|--------|---------|-------|
| `exec` | Run command | Session-aware (session_id for persistent cwd/env) |
| `read_file` | Read file | Binary-safe, streaming for large files |
| `write_file` | Write file | Atomic (temp + rename), binary-safe |
| `edit_file` | Edit file | Expected-hash precondition (fail if file changed) |
| `list_dir` | List directory | With metadata |
| `stat` / `lstat` | File metadata | Size, mode, modtime, is_dir, is_symlink |
| `readlink` | Read symlink | |
| `mkdir` / `mkdir_all` | Create directories | |
| `remove` / `remove_all` | Delete files/dirs | |
| `rename` | Move/rename | |
| `chmod` | Change permissions | |
| `symlink` | Create symlink | Path traversal policy enforced |
| `truncate` | Truncate file | |
| `file_hash` | Compute file hash | For edit preconditions |
| `kill_session` | Kill running exec | For hung commands |

**Symlink policy:** Hard ban on symlinks crossing intended roots. The broker
rejects any path that resolves outside the capsule's merged dir via
`openat2(RESOLVE_BENEATH)`. Device nodes and privileged xattrs are rejected.

**Session management:** The broker maintains per-session shell processes
internally. Each cosuper gets a real bash process with persistent cwd, env
vars, and background jobs. The `exec` verb takes a `session_id` (broker-minted
random ID, NOT agentRunID — bound to {agentRunID, capsuleID, capabilityID,
brokerIncarnationID} for session invalidation on broker restart).

**Capability verification:** Every request includes a Capability field. The
broker verifies the Ed25519 signature with its public key (injected at spawn,
never enters guest as a secret) and checks the requested verb is in the
capability's role-based VerbSet. See `capsule-executor-design-v0.md` for the
Capability structure.

**Reference projects:**
- `ptyrelay` (FanBB2333/ptyrelay) — session framing (historical reference)
- `go-agent-sessions` (hollis-labs) — long-lived PTY with Wait/Stop/SendInput
- Docker exec implementation — reference for exec API design

## Implementation (Production-Only, No Phases)

**Hard cutover from the current non-capsule architecture. No MVP, no
fallbacks, no phased delivery.**

All components are built together and shipped as one production system:

1. Namespace creation via gonso (mount, PID, UTS, IPC, NET)
2. EROFS mount (shared, once at boot) + overlayfs mount via `unix.Mount()`
   with `userxattr` option for user-namespace compatibility
3. Exec-broker (typed RPCs, session-aware, Ed25519 capability-verified,
   bind-mounted from content-addressed host store)
4. cgroups v2 (memory + CPU + PID limits) via `containerd/cgroups/v3`
5. seccomp via `elastic/go-seccomp-bpf` (targeted denylist for workload:
   blocks setns/unshare/mount/bpf/ptrace; allowlist for broker)
6. Landlock via `landlock-lsm/go-landlock` (restrict to capsule paths)
7. Capabilities dropping via `moby/sys/capability` (keep CAP_DAC_OVERRIDE
   + CAP_FOWNER for overlayfs copy-up, drop all others)
8. Snapshot diff via manifest walk (lstat-safe, mtime+size fast-path,
   whiteout/opaque dir detection)
9. CapsuleDiffClassifier (host-side, versioned, ruleset digest recorded)
10. Ed25519 capability-based access control (executor-minted on host,
    broker-verified in guest, role-based verb sets)
11. Host-side diagnostic tools (bypass broker, openat2-safe, FIFO-safe)
12. VM memory admission control (memory.max is total budget, tmpfs is sub-budget)
13. Capsule lifecycle management (GC, bounded pins, quarantine, OOM handling)
14. Integration with Choir's tool registry and agent goroutines
15. Integration with MutationTransaction builder

**Deliverable:** A Go package that can:
```go
capsule, _ := executor.Spawn(ctx, SpawnSpec{
    Tier:       capsule.TierMedium,
    OwnerRunID: superRunID,
})
cap, _ := executor.MintCapability(agentRunID, capsule.RoleCosuper, capsule.ID, 24*time.Hour)
result, _ := capsule.Exec(ctx, cap, ExecRequest{
    Command:   "ls -la /",
    SessionID: brokerMintedSessionID, // broker-minted random ID, not agentRunID
})
diff, _ := capsule.Diff(ctx)              // snapshot diff vs last manifest
capsule.CommitManifest(ctx)               // record manifest after tape append
capsule.Destroy(ctx)
```

## The CapsuleExecutor API (Sketch — v14)

**Note:** The design doc (`capsule-executor-design-v0.md`) is canonical for
struct definitions. The sketches below are simplified views omitting
concurrency fields (`mu`, `inflightMu`, `inflightOps`) for readability.
Always reference the design doc for the complete struct definition.

```go
package capsule

// HostAuthority runs on the Firecracker HOST (outside guest kernel).
// Holds the Ed25519 private key. Communicates with Executor via vsock.
type HostAuthority struct {
    signKey          ed25519.PrivateKey
    keyID            string
    revokedCaps      map[string]map[string]bool // capsuleID → set of revoked CapabilityIDs
    globalRevokedCaps map[string]bool           // wildcard revoked CapabilityIDs (apply to all capsules)
    revocationLog    *os.File                   // append-only log on host disk (fsynced before ack)
    vsockListener    net.Listener               // vsock listener for Executor connections
    knownCapsules    map[string]bool            // capsuleIDs that have been spawned (for mint auth)
    activeRuns       map[string]bool            // agentRunIDs that are active (for mint auth)
}

// MintCapability authorization policy (v6):
// - Rejects role=super from Executor (super caps are host-local only)
// - Rejects TTL > 24h
// - Rejects capsuleID not in knownCapsules (unless TargetCapsule="*")
// - Rejects agentRunID not in activeRuns
func (h *HostAuthority) MintCapability(agentRunID string, role AgentRole, capsuleID string, ttl time.Duration) (*Capability, error)
func (h *HostAuthority) RevokeCapability(agentRunID, capsuleID, capabilityID string) error
func (h *HostAuthority) GetRevokedCaps(capsuleID string) ([]string, error) // per-capsule + global wildcard set
func (h *HostAuthority) ReloadRevocationLog() error // replay on restart
func (h *HostAuthority) RegisterCapsule(capsuleID string) error  // add to knownCapsules (called by Executor on spawn)
func (h *HostAuthority) RegisterActiveRun(agentRunID string) error // add to activeRuns (called by Executor on agent spawn)
func (h *HostAuthority) UnregisterCapsule(capsuleID string) error // remove from knownCapsules (called on destroy)
func (h *HostAuthority) UnregisterActiveRun(agentRunID string) error // remove from activeRuns (called on run completion)

// Executor runs INSIDE the Firecracker guest VM.
// Manages namespace/cgroup/overlay/broker lifecycle.
// Requests capability minting from HostAuthority via vsock.
type Executor struct {
    ErofsMount    string
    StateDir      string
    BrokerStore   string
    VmMemoryTotal int64
    hostClient    *HostClient                // vsock client to HostAuthority
    capsules      map[string]*Capsule
    capabilities  map[capKey]*Capability
    revokedCaps   map[string]bool            // per-capsule revoked CapabilityIDs (synced from HostAuthority)
    globalRevokedCaps map[string]bool        // wildcard revoked CapabilityIDs (apply to all capsules)
}

type SpawnSpec struct {
    CapsuleID    string
    MemoryMax    int64          // total budget: RSS + tmpfs + kmem
    CpuQuota     int64
    CpuPeriod    int64          // default 100000
    PidsMax      int64
    DiskMax      int64          // tmpfs sub-budget of MemoryMax
    Env          []string
    WorkingDir   string
    OwnerRunID   string
    Tier         ResourceTier
}

type Capsule struct {
    ID          string
    PID         int
    UpperDir    string
    WorkDir     string
    MergedDir   string
    Cgroup      *cgroup2.Manager
    Namespace   gonso.Set
    State       CapsuleState
    CommitEpoch uint64          // audit metadata (not enforced for exec/read/write)
    LastManifest []FileManifest
    Pinned      bool
}

type Capability struct {
    CapabilityID   string
    Handle         string
    CapsuleID      string    // "" for wildcard (researcher)
    AgentRunID     string
    AgentRole      AgentRole
    TargetCapsule  string    // "*" for researcher
    Verbs          VerbSet
    ExternalAccess []string
    CommitEpoch    uint64    // audit metadata only
    ExpiresAt      time.Time
    KeyID          string
    Signature      []byte    // Ed25519 signature
}

type ExecRequest struct {
    SessionID string  // broker-minted random ID
    Command   string
    Cwd       string
    Env       []string
    Stdin     string
    TimeoutMS int
    PTY       bool
}

type ExecResult struct {
    ExitCode  int
    SessionID string  // returned when broker creates new session
    Stdout    string
    Stderr    string
    Duration  time.Duration
}

type FileChange struct {
    Path string
    Kind ChangeKind
    Mode os.FileMode
}

type FileManifest struct {
    Path   string
    Size   int64
    Mtime  time.Time
    Hash   string
    Mode   uint32
    Type   string
}

func (e *Executor) Spawn(ctx context.Context, spec SpawnSpec) (*Capsule, error)
func (e *Executor) Destroy(ctx context.Context, id string) error
func (e *Executor) ForceDestroy(ctx context.Context, id string) error
func (e *Executor) MintCapability(agentRunID string, role AgentRole, capsuleID string, ttl time.Duration) (*Capability, error) // via vsock to HostAuthority
func (e *Executor) ResolveCapability(agentRunID, handle string) (*Capability, error)
func (e *Executor) RevokeCapability(agentRunID, handle string) error  // via vsock to HostAuthority
func (e *Executor) ResolveTarget(cap *Capability) ([]string, error)
func (e *Executor) InspectCapsuleRaw(id string) (*CapsuleDiagnostics, error)  // host-side, bypasses broker
func (e *Executor) ExtractDiff(id string) ([]FileChange, error)              // host-side, bypasses broker
func (e *Executor) ListCapsules() []CapsuleSummary                            // host-side, bypasses broker
func (e *Executor) RestartBroker(id string) error                             // re-syncs revoked caps from HostAuthority

func (c *Capsule) Exec(ctx context.Context, cap *Capability, req ExecRequest) (ExecResult, error)
func (c *Capsule) Quiesce(ctx context.Context) error
func (c *Capsule) Thaw(ctx context.Context) error
func (c *Capsule) Diff(ctx context.Context) ([]FileChange, error)       // snapshot diff (fastPath=true)
func (c *Capsule) CommitManifest(ctx context.Context) error             // record manifest (fastPath=false)
func (c *Capsule) Destroy(ctx context.Context) error
```

## The CapsuleDiffClassifier (Sketch)

```go
package main // cmd/capsule-host — classifier runs in HostAuthority

type Classifier struct {
    Version string                    // "v1"
    Rules   map[LedgerKind][]PathPattern
    Ignore  []PathPattern             // /tmp, /run, /var/log, .cache
}

type LedgerKind int
const (
    LedgerVM LedgerKind = iota    // V: /boot, /lib/modules, /etc/systemd
    LedgerDolt LedgerKind = iota  // D: /var/lib/dolt
    LedgerSource LedgerKind = iota // S: /home/user/src, /workspace
    LedgerBlob LedgerKind = iota   // B: /var/lib/blob
    LedgerArtifact LedgerKind = iota // A: /var/lib/artifact
    LedgerRoute LedgerKind = iota  // R: /etc/choir/route
    LedgerUnknown LedgerKind = iota
)

func (c *Classifier) Classify(changes []FileChange) map[LedgerKind][]FileChange {
    // Group changes by ledger based on path patterns
    // Ignore ephemeral paths (/tmp, /run, /var/log, .cache)
    // Unknown paths: reject at commit time (v7 decision). Silently
    // classifying as LedgerVM creates a trust-bearing catch-all.
}
```

**Runs on the host (trusted zone).** The VM exports the overlay diff; the
host classifies it. This keeps the classifier in the trusted zone, consistent
with the MutationTransaction design.

## Kernel Version Floor

The capsule runtime requires the following minimum kernel versions:

| Feature | Minimum | Recommended |
|---------|---------|-------------|
| Landlock | 5.13 | 6.13+ |
| `close_range(CLOSE_RANGE_CLOEXEC)` | 5.9 | 6.13+ |
| `openat2(RESOLVE_BENEATH)` | 5.6 | 6.13+ |
| `seccomp` `NamesWithCondtions` arg filtering | 3.5 | 6.13+ |
| cgroup v2 | 4.18 | 6.13+ |
| overlayfs `userxattr` | 5.8 | 6.13+ |
| EROFS | 5.4 | 6.13+ |

**Single source of truth: Linux 6.13+** is the recommended guest kernel.
This avoids the EROFS+overlay regression risk noted in earlier kernels
and provides all features with mature stability. The minimum supported
is 5.13 (Landlock requirement), but 5.13–6.12 kernels may encounter
EROFS+overlay edge cases.

## Reference Projects to Study

These projects solve adjacent problems and are worth studying before
implementation:

1. **MiniContainer** (`hwang-fu/minicontainer`) — Clean Go implementation of
   namespaces + cgroups + overlayfs. Best reference for the core loop.

2. **containerd/continuity** (`containerd/continuity/fs`) — DiffDirChanges
   for overlayfs diff. Use directly.

3. **ptyrelay** (`FanBB2333/ptyrelay`) — Session framing with BEG/END
   markers. Reference for sentinel-based command completion.

4. **go-agent-sessions** (`hollis-labs/go-agent-sessions`) — Long-lived PTY
   with Wait/Stop/SendInput/Resize. Reference for session management.

5. **Dagger's overlay util** (`dagger/dagger`) — Go overlayfs diff with
   opaque directory handling. Reference for edge cases.

6. **mattolson/agent-sandbox** (`mattolson/agent-sandbox`) — Go library for
   agent sandboxing with network egress policy. Reference for nftables
   integration.

7. **go-sandbox** (`hollis-labs/go-sandbox`) — Bubblewrap wrapper for
   OS-level sandboxing. Reference for namespace setup patterns (even though
   we use gonso instead of bubblewrap).

## Resolved Questions (from v1 + v2 review rounds)

1. **Exec-broker binary:** Bind-mounted from content-addressed host store
   (v2). Content hash verified at spawn. Not baked into EROFS.

2. **Agent sharing model:** N agents per M capsules. Super controls
   topology. Multiple agents can share a capsule (shared upperdir, separate
   shells via session_id). Role-based capability access.

3. **EROFS base sharing:** Mount once at VM boot, share across all capsules.
   Page cache efficiency.

4. **Network:** Air-gapped capsules. CLONE_NEWNET per capsule (no interfaces)
   + seccomp socket family filtering. All network I/O mediated by host.

5. **User namespace:** Root inside namespace. v2 adds privilege separation:
   broker runs as per-capsule unprivileged UID in user+mount namespace.

6. **Incremental diff:** Snapshot diff (v2). Manifest-based, no remount.
   Walk upperdir, compare against last commit's manifest. Crash-safe.

7. **Session management:** Broker-side sessions. Each agent gets a real
   bash process with persistent cwd/env. The `exec` verb takes a session_id.

8. **Broker protocol:** Typed RPCs, not exec wrappers. All file operations
   are typed (stat, mkdir, remove, rename, chmod, symlink, etc.).

9. **Capsule access:** Ed25519 asymmetric capabilities (v5). HostAuthority
   (on Firecracker host) holds private key, Executor (in guest) requests
   minting via vsock, broker (guest) holds public key. Role-based verb sets
   (super/cosuper/researcher). Agent sees opaque handles, not raw UUIDs.
   Per-CapabilityID revocation (not global epoch).

10. **Capsule lifecycle:** Ephemeral by default (tied to super's run record).
    Long-lived via explicit `pin_capsule(id, timeout)` with 24h max TTL.
    Uncommitted diff quarantined after 4h idle (not immortal).

11. **Memory admission:** `memory.max` is total budget (RSS + tmpfs + kmem).
    DiskMax is sub-budget within MemoryMax. No double-counting (v2).
