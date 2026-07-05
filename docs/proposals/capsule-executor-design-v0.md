# CapsuleExecutor Design — v14

**Status:** Concrete design for the CapsuleExecutor integration layer.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Supersedes:** v13 (incorporates v13 consensus fixes: session_id
contradiction fixed, v12 comment updated to v13)

## Overview

The CapsuleExecutor is the component that bridges Choir's existing Go runtime
(tool registry, coagent spawning, run records) with the capsule container
runtime (namespaces, overlayfs, cgroups, seccomp). It enables cosupers to
execute bash commands inside isolated capsules and enables supers to capture
overlay diffs as MutationTransactions.

### Two-Plane Architecture (v5)

The runtime is split across the Firecracker host/guest boundary:

```
Firecracker HOST (outside guest kernel — trust anchor)
  └── HostAuthority (cmd/capsule-host)
        ├── Ed25519 private key (never enters guest)
        ├── Capability minting + revocation authority
        ├── Revocation tracking (per-CapabilityID, not global epoch)
        ├── Revocation persistence (append-only log on host disk)
        ├── Classifier + tape append (trust-bearing, runs on host)
        └── vsock listener (host↔guest control channel)

Firecracker GUEST (inside guest kernel — capsule management)
  └── Executor (internal/capsule)
        ├── Namespace/cgroup/overlay/broker lifecycle
        ├── Diagnostics (host-side reads, bypass broker)
        ├── Diff extraction (walks upperdir, sends manifest to host)
        └── vsock client (connects to HostAuthority)
  └── Broker (cmd/capsule-broker, per-capsule)
        ├── Ed25519 public key (injected at spawn)
        ├── Exec, file ops, session management
        └── Unix socket (executor↔broker, inside guest)
```

**Why split:** The Ed25519 private key must be outside the guest kernel
(so a guest-kernel LPE can't forge capabilities). But namespace/cgroup/
overlay management requires kernel syscalls inside the guest. A single
process can't be both outside and inside the guest. The HostAuthority
holds the key and signs; the Executor manages lifecycle inside the guest;
vsock connects them.

**Transport:** vsock (AF_VSOCK) between HostAuthority and Executor. Unix
domain socket between Executor and each Broker (inside guest, per-capsule
mount namespace).

## Threat Model (Updated from v0)

```
Firecracker microVM (PRIMARY security boundary — hardware virtualization)
  └── Capsule (SECONDARY security boundary — defense-in-depth)
        └── Cosuper bash commands
```

The capsule IS a security boundary — a defense-in-depth layer. The
seccomp/landlock/capabilities are load-bearing security controls, not
wasted effort. The microVM is the primary boundary (protects the host).
The capsule is the secondary boundary (protects work isolation inside the
VM: prevents cross-capsule data leakage, sabotage, forged edits, poisoned
diffs).

The framing matters: if engineers believe "the capsule is just a
convenience," they'll write less careful input validation, path traversal
checks, and symlink handling in the broker. The broker's JSON-RPC handler
must be written as if it's load-bearing, because it is.

## Library Stack (from capsule-runtime-implementation-v0.md)

| Component | Library |
|-----------|---------|
| Namespaces | `github.com/cpuguy83/gonso` |
| Overlayfs | Direct `golang.org/x/sys/unix.Mount()` |
| cgroups v2 | `github.com/containerd/cgroups/v3/cgroup2` |
| seccomp | `github.com/elastic/go-seccomp-bpf` |
| Landlock | `github.com/landlock-lsm/go-landlock/landlock` |
| Capabilities | `github.com/moby/sys/capability` |
| PTY | `github.com/creack/pty` |
| Overlay diff | `github.com/containerd/continuity/fs` |

All pure Go. No CGO. No daemon. No external binary dependencies.

## Package Layout

```
internal/capsule/
    executor.go       — Executor, Spawn, Destroy, lifecycle, GC (runs in GUEST)
    host_client.go    — vsock client to HostAuthority (mint, revoke, sync)
    capsule.go        — Capsule type, state, Quiesce, Diff, CommitManifest
    capability.go     — Ed25519 capability verification (public key only)
    roles.go          — AgentRole, VerbSet, RoleVerbSets
    broker_client.go  — BrokerClient (JSON-RPC over Unix socket, inside guest)
    seccomp.go        — seccomp filter setup
    landlock.go       — Landlock path restrictions
    capabilities.go   — capability dropping (Linux caps, not our caps)
    diagnostics.go    — Host-side diagnostic tools (bypass broker, openat2-safe)
    manifest.go       — FileManifest, walkUpperdir, diffManifests (snapshot diff)
    types.go          — SpawnSpec, ExecRequest, ExecResult, FileChange, etc.

cmd/capsule-host/
    main.go           — HostAuthority binary (runs on Firecracker HOST)
    authority.go      — Ed25519 signing, revocation tracking, vsock server
    revocation.go     — Per-CapabilityID revoked set + wildcard set (not global epoch)
    classifier.go     — CapsuleDiffClassifier (path → ledger mapping, trust-bearing)
    transaction.go    — BuildTransactionFromDiff (diff → MutationTransaction)

cmd/capsule-broker/
    main.go           — exec-broker binary (bind-mounted from host store)
    session.go        — Per-session shell management
    file_ops.go       — Typed RPC file operations (stat, mkdir, remove, etc.)

internal/runtime/
    tools_capsule.go  — NEW: spawn_capsule, commit_transaction, inspect tools
    tools_coding.go   — MODIFIED: bash tool rewired for capability-based routing
    tool_profiles.go  — MODIFIED: toolCtxCapsuleHandle context key
```

## Capability-Based Access Control (v2 — Ed25519 + role-based verbs)

Raw `capsule_id` is identity, not authority. The executor mints
unforgeable capability tokens that the broker verifies on every request.

### Trust Anchor: Ed25519 Asymmetric Signatures (v2 decision)

**Problem with HMAC (v1):** A single shared HMAC secret in the executor
(guest VM) means a guest-kernel LPE extracts the master key and can forge
capabilities for all capsules. Per-capsule HKDF derivation limits blast
radius to one capsule but the master secret is still in the attacked
kernel.

**Solution: Ed25519 asymmetric signatures.** The HostAuthority (on the
Firecracker host, outside the guest kernel) holds the Ed25519 private key.
The Executor (inside the guest) requests capability minting from the
HostAuthority via vsock. Each broker (inside the guest, per-capsule) holds
the Ed25519 public key, injected at spawn time. The broker verifies
capabilities locally — no per-RPC round-trip to the host.

- **Signing (minting):** Once per agent-run per capsule. Rare. Done by
  HostAuthority on the host (Executor requests via vsock).
- **Verification:** On every RPC. Done by broker locally with the public
  key. ~microseconds. No vsock round-trip.
- **Trust anchor:** Private key is outside the guest kernel. A guest-kernel
  LPE cannot read it.
- **Revocation (v5):** HostAuthority tracks revoked CapabilityIDs in a
  per-capsule set (not a global epoch bump). On revocation, HostAuthority
  sends `sync_revoked_caps` to the Executor via vsock, which forwards to
  the broker via `sync_revoked_caps` RPC. The broker rejects any capability
  whose CapabilityID is in the revoked set. This prevents collateral
  revocation of other agents sharing the same capsule. Wildcard researcher
  capabilities (TargetCapsule="*") are revoked globally — the CapabilityID
  is added to every capsule's revoked set, including capsules spawned
  after the revocation.
- **Revocation persistence (v6):** HostAuthority persists the revoked-
  capability set to an append-only log on host disk, fsynced before
  acknowledging the revocation to the Executor. On HostAuthority restart,
  the log is replayed to restore the revoked set. This ensures revocations
  survive HostAuthority crashes.
- **Broker restart (v5):** On `RestartBroker`, the Executor re-syncs the
  full revoked-capability set from the HostAuthority via vsock before the
  broker accepts any RPC. The HostAuthority is the authoritative source
  for revocation state. A broker crash does NOT reset revocation — the
  Executor holds the revoked set and re-injects it.
- **vsock auth (v6):** The vsock channel between Executor and HostAuthority
  is a guest-kernel facility. The two-plane architecture protects against
  **user-space compromise** of the Executor/broker/cosuper (they can't
  mint caps without the private key). It does NOT protect against
  **guest-kernel LPE** — an attacker with kernel access can impersonate
  the Executor via vsock. The microVM boundary remains the primary defense
  against guest-kernel compromise. To raise the bar: `/dev/vsock` is
  restricted to the Executor process: `/dev/vsock` is not mounted into
  capsule namespaces, and the Executor's cgroup v2 device controller
  (eBPF device filter program) denies vsock access to non-Executor
  processes. Defense in depth — kernel compromise bypasses this, which is
  why the microVM is the primary boundary.
- **Executor trust boundary (v14):** The Executor is **trusted guest TCB**.
  HostAuthority protects the private key and host-side tape/classifier from
  guest key extraction. The Executor is a privileged lifecycle manager inside
  the VM — if it is user-space-compromised, it can register arbitrary
  capsules/runs and request mints (bounded by mint-auth rules: no super role,
  TTL ≤ 24h). This is an explicit design choice: the Executor needs kernel
  syscalls for namespace/cgroup/overlay management, so it cannot be fully
  untrusted. The threat model is: HostAuthority trusted, Executor trusted
  guest TCB, broker/workload/cosuper less trusted.
- **Network enforcement (v10):** "Air-gapped" is enforced by two layers:
  (1) **CLONE_NEWNET per capsule** — creates an isolated network namespace
  with no interfaces, no loopback, no routing. This prevents cross-capsule
  communication via abstract Unix sockets (which are scoped by network
  namespace, not mount namespace). (2) **seccomp socket family filtering** —
  denies `socket(AF_INET)`, `socket(AF_INET6)`, `socket(AF_NETLINK)`, AND
  `socket(AF_VSOCK)` at creation time. AF_UNIX is allowed (needed for broker
  control plane), but CLONE_NEWNET ensures abstract Unix sockets are scoped
  to the capsule's network namespace. The workload's seccomp denylist also
  blocks these socket families. External access (researcher's dolt:write,
  message:send) goes through host-mediated RPCs via vsock (Executor-only),
  not direct network calls from broker/workload.
- **FD hygiene (v7):** Before exec'ing broker or workload, the Executor
  marks all non-stdio, non-control fds as close-on-exec via
  `close_range(3, ~0, CLOSE_RANGE_CLOEXEC)`. This prevents inherited
  vsock/inet fds from surviving exec into broker/workload, bypassing the
  seccomp socket creation block. (CLOSE_RANGE_CLOEXEC marks fds CLOEXEC
  rather than closing them immediately — the Executor's own vsock fd
  survives in the parent, but is not inherited by the child.) Broker
  allowed fds: stdio + AF_UNIX control socket only. Workload allowed fds:
  stdio/PTY only. Startup self-check walks `/proc/self/fd` and fails
  closed on any unexpected socket family.
- **Residual risk:** A guest-kernel LPE could replace the broker's in-memory
  public key with an attacker-controlled one, then forge tokens with the
  matching private key. Mitigated by: broker binary is content-hash verified
  at spawn (Decision 2), public key is injected by Executor and pinned in
  broker memory, broker runs with dropped capabilities and seccomp.

### Role-Based Verb Sets (v2 decision — not *nix permission tiers)

Agent roles delineate capabilities, not file permission tiers. A researcher
isn't "read-only" in the *nix sense — a researcher can read all capsules,
send messages, and write to Dolt. These are role-based capability grants,
not `AccessMode = ReadOnly`.

All capsules have all permissions. The capsule is an execution context, not
a permission boundary. The **agent's role** determines what verbs are
available.

```go
package capsule

import (
    "crypto/ed25519"
    "time"
)

// AgentRole defines what an agent can do, not what a capsule allows.
// Capsules are execution contexts with full VFS. Roles determine verbs.
type AgentRole string

const (
    RoleSuper      AgentRole = "super"      // spawn/destroy capsules, grant access, diagnostics
    RoleCosuper    AgentRole = "cosuper"    // exec, full VFS within granted capsules
    RoleResearcher AgentRole = "researcher" // read across all capsules, send messages, write to Dolt
)

// VerbSet is the set of broker RPC methods allowed for a role.
// Defined by role, not by *nix read/write permissions.
type VerbSet map[string]bool

// RoleVerbSets maps each role to its allowed broker verbs.
// Host control-plane verbs (spawn, destroy, mint, revoke, commit_transaction,
// inspect_capsule_raw, extract_diff, list_capsules) are NOT broker verbs —
// they are Executor/HostAuthority methods, not routed through a capsule's
// broker. Only in-capsule operations are broker verbs.
var RoleVerbSets = map[AgentRole]VerbSet{
    RoleSuper: {
        // Super has NO broker verbs. Super operates via Executor host methods:
        // spawn_capsule, destroy_capsule, pin_capsule, restart_broker,
        // force_destroy, mint_capability, revoke_capability, commit_transaction,
        // inspect_capsule_raw, extract_diff, list_capsules.
        // These bypass the broker entirely — they are host-side operations.
    },
    RoleCosuper: {
        "exec": true, "read_file": true, "write_file": true, "edit_file": true,
        "list_dir": true, "stat": true, "lstat": true, "readlink": true,
        "mkdir": true, "mkdir_all": true, "remove": true, "remove_all": true,
        "rename": true, "chmod": true, "symlink": true, "truncate": true,
        "file_hash": true, "kill_session": true,
    },
    RoleResearcher: {
        "read_file": true, "list_dir": true, "stat": true, "lstat": true,
        "readlink": true, "file_hash": true,
        // Researcher also gets external access (not broker verbs):
        // send_message, dolt_write — handled outside the broker
    },
}

// Capability is an Ed25519-signed token minted by HostAuthority.
// The cosuper never sees the raw capsule ID — it gets an opaque handle.
type Capability struct {
    CapabilityID   string    // stable unique ID (used in revocation + session binding)
    Handle         string    // opaque handle, e.g. "build-a" (agent-facing)
    CapsuleID      string    // real capsule UUID, or "" for wildcard (researcher)
    AgentRunID     string    // which agent run this cap is for
    AgentRole      AgentRole // determines verb set
    TargetCapsule  string    // capsule ID, or "*" for all (researcher)
    Verbs          VerbSet   // role-defined verb set
    ExternalAccess []string  // e.g. ["dolt:write", "message:send"] for researcher
    CommitEpoch    uint64    // audit metadata only (NOT enforced for exec/read/write)
    ExpiresAt      time.Time // capability expiry
    KeyID          string    // which signing key was used (for rotation)
    Signature      []byte    // Ed25519 signature over all fields
}

// HostAuthority (on host) mints a capability with the Ed25519 private key.
// For researcher role, capsuleID is "" and TargetCapsule is "*" — the
// executor expands "*" to concrete capsule IDs at call time (see below).
func (h *HostAuthority) MintCapability(agentRunID string, role AgentRole, capsuleID string, ttl time.Duration) (*Capability, error)
func (h *HostAuthority) RegisterCapsule(capsuleID string) error      // add to knownCapsules (called by Executor on spawn)
func (h *HostAuthority) RegisterActiveRun(agentRunID string) error   // add to activeRuns (called by Executor on agent spawn)
func (h *HostAuthority) UnregisterCapsule(capsuleID string) error    // remove from knownCapsules (called on destroy)
func (h *HostAuthority) UnregisterActiveRun(agentRunID string) error // remove from activeRuns (called on run completion)

// Broker (in guest) verifies a capability with the Ed25519 public key
func (c *Capability) Verify(publicKey ed25519.PublicKey) error

// Revocation (v5): HostAuthority adds CapabilityID to the capsule's revoked
// set and sends sync_revoked_caps to the Executor via vsock. The Executor
// forwards to the broker via sync_revoked_caps RPC. The broker rejects any
// capability whose CapabilityID is in the revoked set.
// Per-CapabilityID revocation prevents collateral revocation of other
// agents sharing the same capsule.
func (h *HostAuthority) RevokeCapability(agentRunID, capsuleID, capabilityID string) error

// Researcher wildcard routing: when a researcher calls a broker verb,
// the executor resolves TargetCapsule="*" by iterating all running
// capsules and issuing the RPC to each one individually. The researcher
// never talks to a broker directly — the executor fans out.
// For non-wildcard capabilities, the executor routes to the single
// capsule's broker as normal.
func (e *Executor) ResolveTarget(cap *Capability) ([]string, error)
// Returns []string of capsule IDs to route to.
// For TargetCapsule="*": returns all running capsule IDs.
// For TargetCapsule="<uuid>": returns []string{<uuid>}.

// The agent's tool receives the Handle, not the CapsuleID
// The executor maps Handle → Capability → CapsuleID internally
```

### How It Works

1. Super calls `spawn_cosuper(objective, capsule_access=[{handle: "build-a", capsule_id: "uuid-123"}])`
   or `spawn_researcher(objective)` (researcher gets `target_capsule="*"`)
2. Executor (guest) requests capability minting from HostAuthority (host) via vsock
3. HostAuthority mints Ed25519-signed capability, returns it to Executor
4. Agent's run metadata gets `capsule_handles: ["build-a"]` (not raw UUIDs)
5. Agent calls `bash(command, capsule="build-a")` — uses the handle, not UUID
6. Executor maps handle → capability → capsuleID, checks expiry + revoked set
7. Executor routes request to capsule's broker with the capability attached
8. Broker verifies Ed25519 signature with its public key, checks verb is in
   the capability's VerbSet, checks CapabilityID is NOT in revoked set.
   CommitEpoch is NOT checked for ordinary exec/read/write — capabilities
   remain valid across commits. Sessions survive commits. CommitEpoch is
   audit metadata only.

### Broker Socket Isolation

- Each capsule's broker socket lives inside its own mount namespace
- Sockets are NOT visible across capsules (mount namespace isolation)
- No abstract Unix sockets (would be visible across namespaces)
- Socket permissions: 0600, owned by executor process
- Only the executor can connect (filesystem permissions + capability verification)
- **Privilege separation (v2):** Broker runs as a per-capsule unprivileged
  UID in its own user+mount namespace, with seccomp, landlock scoped to
  capsule dirs, and a minimal Linux capability set. The broker retains
  `CAP_DAC_OVERRIDE` and `CAP_FOWNER` (required for overlayfs copy-up of
  base-owned files) but drops everything else (`CAP_SYS_ADMIN`,
  `CAP_NET_ADMIN`, `CAP_SYS_PTRACE`, `CAP_BPF`, etc.). The overlay is
  mounted with `userxattr` so user-namespace UID mappings work correctly.
  This makes the socket isolation and security boundary claims real, not
  marketing.

## Core Types

### executor.go

```go
package capsule

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "time"

    "github.com/containerd/cgroups/v3/cgroup2"
    "github.com/cpuguy83/gonso"
    "github.com/google/uuid"
    "golang.org/x/sys/unix"
)

// Executor manages capsule lifecycle. One instance per Runtime.
// Runs INSIDE the Firecracker guest VM (needs kernel syscalls for
// namespaces, cgroups, overlayfs). The Ed25519 private key is held by
// HostAuthority on the host — Executor requests minting via vsock.
type Executor struct {
    mu                sync.RWMutex
    capsules          map[string]*Capsule        // capsuleID → Capsule
    capabilities      map[capKey]*Capability     // (agentRunID, handle) → Capability
    revokedCaps       map[string]bool            // per-capsule revoked CapabilityIDs (synced from HostAuthority)
    globalRevokedCaps map[string]bool            // wildcard revoked CapabilityIDs (apply to all capsules)
    hostClient        *HostClient                // vsock client to HostAuthority
    stateDir          string                     // /var/lib/capsules
    erofsMount        string                     // shared EROFS mount point
    brokerStore       string                     // content-addressed broker binary store
    vmMemoryTotal     int64                      // total VM RAM for admission control
    vmMemoryUsed      int64                      // committed memory across all capsules
}

type capKey struct {
    AgentRunID string
    Handle     string
}

func NewExecutor(stateDir, erofsMount string, vmMemoryTotal int64) *Executor

type SpawnSpec struct {
    CapsuleID    string         // random UUID if empty
    MemoryMax    int64          // bytes (cgroup memory.max — total budget: RSS + tmpfs + kmem)
    CpuQuota     int64          // microseconds per period
    CpuPeriod    int64          // default 100000
    PidsMax      int64          // max processes
    DiskMax      int64          // bytes for tmpfs upperdir (MUST be <= MemoryMax - headroom)
    Env          []string       // environment variables for broker
    WorkingDir   string         // initial cwd
    OwnerRunID   string         // the super run that spawned this capsule
    Tier         ResourceTier   // small/medium/large preset
}

// ResourceTier presets. v2 fix: tmpfs (DiskMax) is a sub-budget of
// MemoryMax, not additive. tmpfs pages are charged to the capsule's
// cgroup memory.max, so DiskMax must be <= MemoryMax - process headroom.
type ResourceTier string

const (
    // MemoryMax is the total budget. DiskMax is a sub-budget within it.
    TierSmall  ResourceTier = "small"  // 1GB mem, 512MB disk, 0.5 CPU, 100 PIDs
    TierMedium ResourceTier = "medium" // 2GB mem, 1GB disk, 1 CPU, 200 PIDs
    TierLarge  ResourceTier = "large"  // 4GB mem, 2GB disk, 2 CPU, 500 PIDs
)

func (e *Executor) Spawn(ctx context.Context, spec SpawnSpec) (*Capsule, error)
func (e *Executor) GetCapsule(id string) (*Capsule, error)
func (e *Executor) Destroy(ctx context.Context, id string) error
func (e *Executor) ForceDestroy(ctx context.Context, id string) error // MNT_DETACH + force-kill
func (e *Executor) MintCapability(agentRunID string, role AgentRole, capsuleID string, ttl time.Duration) (*Capability, error)
func (e *Executor) ResolveCapability(agentRunID, handle string) (*Capability, error)
func (e *Executor) RevokeCapability(agentRunID, handle string) error  // v2: immediate revocation
func (e *Executor) GC(ctx context.Context) error
func (e *Executor) ListCapsules() []CapsuleSummary // for super's list_capsules tool
```

### capsule.go

```go
type CapsuleState string

const (
    CapsuleStateInitializing CapsuleState = "initializing"
    CapsuleStateRunning      CapsuleState = "running"
    CapsuleStateQuiescing    CapsuleState = "quiescing"
    CapsuleStateFrozen       CapsuleState = "frozen"
    CapsuleStateDestroying   CapsuleState = "destroying" // NEW: rejects new exec, waits for in-flight
    CapsuleStateDestroyed    CapsuleState = "destroyed"
)

type Capsule struct {
    ID          string
    PID         int
    UpperDir    string
    WorkDir     string
    MergedDir   string
    Cgroup      *cgroup2.Manager
    Namespace   gonso.Set
    State       CapsuleState
    CreatedAt   time.Time
    OwnerRunID  string
    Pinned      bool       // long-lived capsule (exempt from GC)
    PinExpiry   time.Time

    // Snapshot diff (v2 — replaces layer stacking)
    CommitEpoch   uint64         // increments on each commit (audit metadata)
    LastManifest  []FileManifest // upperdir state at last commit

    // Refcount for TOCTOU fix — covers ALL RPCs, not just exec
    inflightOps int64
    inflightMu  sync.Mutex

    mu sync.Mutex
}

func (c *Capsule) BindAgent(ctx context.Context, agentRunID string, role AgentRole) error
func (c *Capsule) UnbindAgent(agentRunID string)
func (c *Capsule) Exec(ctx context.Context, cap *Capability, req ExecRequest) (ExecResult, error)
func (c *Capsule) Quiesce(ctx context.Context) error
func (c *Capsule) Thaw(ctx context.Context) error
func (c *Capsule) Diff(ctx context.Context) ([]FileChange, error)     // snapshot diff vs manifest
func (c *Capsule) CommitManifest(ctx context.Context) error           // record manifest after tape append
func (c *Capsule) Destroy(ctx context.Context) error
func (c *Capsule) acquireOp() error   // refcount check (rejects if Destroying)
func (c *Capsule) releaseOp()         // decrements refcount
```

### Snapshot Diff for Incremental Diff (v2 decision — replaces layer stacking)

The v1 layer stacking approach (fold upperdir into lowerdir stack, remount)
is broken at the syscall level: EBUSY on unmount (frozen processes still
hold fds/cwd), workdir filesystem mismatch (EINVAL), no crash atomicity,
and kernel lowerdir stack limits (~500 layers, ~4KB mount option string).

**Solution: Snapshot diff (manifest-based).** Keep the overlay mount stable
for the capsule's entire lifetime. No remount, no unmount, no layer stacking.

At each commit:
1. Quiesce capsule (freeze cgroup, drain in-flight RPCs)
2. Walk the upperdir, recording a manifest: `{path, size, mtime, hash, mode, type}`
3. Compare against the previous commit's manifest → incremental diff
4. Append diff to tape
5. Write new manifest atomically (temp + fsync + rename)
6. Thaw capsule

The manifest is the diff baseline, not the lowerdir stack. The overlay
mount never changes. Session continuity is preserved (fds, cwd, background
jobs all survive commit). Crash-safe: the manifest is written after tape
append, so a crash leaves either the old manifest (re-do commit) or the
new manifest (commit already recorded).

**Whiteout/opaque dir handling:** The manifest walker must detect overlayfs
whiteouts (char device 0/0, or `trusted.overlay.whiteout` xattr) and opaque
dirs (`trusted.overlay.opaque` xattr). Whiteouts → tombstone entries in the
diff. Opaque dirs → directory-clear operations. Without this, the tape
records "file added" but never "file deleted."

**Walker safety (v3):** `walkUpperdir` uses the same defensive traversal
as `extract_diff`: `lstat` before every read, refuse non-regular non-
directory files (skip with note), `O_NONBLOCK` on opens, hard caps on
entries/bytes/depth/time. A guest `mkfifo` trap must not hang the host-side
manifest walk. The walker uses mtime+size as a **cache hint only** — if
mtime and size are unchanged since the last manifest, it skips the SHA-256
hash and reuses the cached value for **intermediate diffs** (e.g.
`capsule_diff_preview`). For **authoritative commit capture** (tape
append), the walker always hashes content — a workload with `CAP_FOWNER`
can restore mtime after a same-size edit, so mtime+size alone is not
trustworthy for the audit record.

```go
type FileManifest struct {
    Path   string
    Size   int64
    Mtime  time.Time
    Hash   string  // SHA-256 of content (regular files only)
    Mode   uint32
    Type   string  // "regular", "dir", "symlink", "whiteout", "opaque_dir"
}

// Diff compares current upperdir against the last commit's manifest.
// No remount, no layer stacking. The overlay mount is stable.
// walkUpperdir uses lstat + O_NONBLOCK (same safety as extract_diff).
// mtime+size fast-path is used here (preview/diff is non-authoritative).
func (c *Capsule) Diff(ctx context.Context) ([]FileChange, error) {
    current := walkUpperdir(c.UpperDir, c.LastManifest, true)  // fastPath=true
    return diffManifests(c.LastManifest, current)
}

// CommitManifest records the current upperdir state as the new baseline.
// Called after tape append, while capsule is frozen.
// fastPath=false: always hash content (authoritative commit capture).
// Crash recovery: tape append is idempotent by commit epoch — if a crash
// happens between tape append and manifest write, recovery re-does the
// commit but the tape dedups by (capsuleID, commitEpoch).
func (c *Capsule) CommitManifest(ctx context.Context) error {
    manifest := walkUpperdir(c.UpperDir, c.LastManifest, false)  // fastPath=false: always hash
    data := serializeManifest(manifest)
    tmpPath := filepath.Join(c.stateDir, "manifest.tmp")
    f, _ := os.Create(tmpPath)
    f.Write(data)
    f.Sync()  // fsync the temp file
    f.Close()
    os.Rename(tmpPath, filepath.Join(c.stateDir, "manifest.json"))
    // fsync the parent directory so the rename is durable
    dir, _ := os.Open(c.stateDir)
    dir.Sync()
    dir.Close()
    c.LastManifest = manifest
    c.CommitEpoch++
    return nil
}
```

### Capability Handle (NEW from review)

The cosuper's bash tool uses an opaque handle, not a raw capsule ID:

```go
// The cosuper sees:
bash(command="ls -la", capsule="build-a")

// The executor resolves "build-a" → Capability → CapsuleID
// The broker receives the Capability and verifies it
```

### Broker Protocol (Updated from v0 — typed RPCs, not exec wrappers)

The exec-broker is a small Go binary bind-mounted from a content-addressed
host store (v2 decision — not baked into EROFS). The executor mounts it
read-only into the capsule at spawn time and verifies its content hash
before exec. Runs as PID 1 inside each capsule. Listens on a Unix socket
(0600, executor-owned). Accepts JSON-RPC requests with Ed25519 capability
verification.

**Protocol: typed RPCs for all operations.** No exec wrappers for file
operations (heredoc-based wrappers reintroduce delimiter-collision bugs,
no binary safety, and non-atomic writes).

```go
// All requests include a Capability field for authorization
type BrokerRequest struct {
    Capability *Capability `json:"capability"`
    Method     string      `json:"method"`
    Params     json.RawMessage `json:"params"`
}

// Session-aware exec (broker maintains per-session shells)
// Session IDs are broker-minted random IDs, NOT agentRunIDs.
// Bound to {agentRunID, capsuleID, capabilityID, brokerIncarnationID}.
// commitEpoch is NOT part of the binding — sessions survive commits.
type ExecParams struct {
    SessionID string   `json:"session_id"`  // broker-minted random ID
    Command   string   `json:"command"`
    Cwd       string   `json:"cwd,omitempty"`     // overrides session cwd
    Env       []string `json:"env,omitempty"`     // overrides session env
    Stdin     string   `json:"stdin,omitempty"`
    TimeoutMS int      `json:"timeout_ms"`
    PTY       bool     `json:"pty,omitempty"`
}

type ExecResult struct {
    ExitCode  int    `json:"exit_code"`
    SessionID string `json:"session_id,omitempty"` // returned when broker creates new session
    Stdout    string `json:"stdout"`
    Stderr    string `json:"stderr"`
    Duration  int64  `json:"duration_ms"`
}

// Typed file operations (NOT exec wrappers)
type ReadFileParams struct {
    Path string `json:"path"`
    Offset int64 `json:"offset,omitempty"`  // for streaming large files
    Limit int64 `json:"limit,omitempty"`
}

type WriteFileParams struct {
    Path    string `json:"path"`
    Content []byte `json:"content"`    // binary-safe
    Mode    uint32 `json:"mode,omitempty"`
    Atomic  bool   `json:"atomic"`     // write to temp + rename
}

type EditFileParams struct {
    Path           string `json:"path"`
    ExpectedHash   string `json:"expected_hash"`  // precondition: fail if file changed
    OldString      string `json:"old_string"`
    NewString      string `json:"new_string"`
}

type StatParams struct {
    Path string `json:"path"`
}

type StatResult struct {
    Size  int64    `json:"size"`
    Mode  uint32   `json:"mode"`
    ModTime time.Time `json:"mod_time"`
    IsDir bool     `json:"is_dir"`
    IsSymlink bool `json:"is_symlink"`
}

// Full method list:
// exec, read_file, write_file, edit_file, list_dir,
// stat, lstat, readlink, mkdir, mkdir_all, remove, remove_all,
// rename, chmod, symlink, truncate, file_hash, kill_session,
// sync_revoked_caps (host→broker control message, not agent-callable)

// sync_revoked_caps: executor → broker revocation propagation
// Sends the full revoked-capability set (not a global epoch).
// On broker restart, executor sends the full set to re-initialize.
type SyncRevokedCapsParams struct {
    RevokedCapabilityIDs []string `json:"revoked_capability_ids"`
    Reason               string   `json:"reason"` // "revocation" or "broker_restart"
}
```

**Symlink policy:** Hard ban on symlinks crossing intended roots. The broker
rejects any path that resolves outside the capsule's merged dir via
`openat2(RESOLVE_BENEATH)`. Device nodes and privileged xattrs are rejected.

### Session Management (NEW from review)

The broker maintains per-session shell processes internally. Each cosuper
gets a real bash process with persistent cwd, env vars, and background jobs.

```go
// In the broker (cmd/capsule-broker/session.go)
type SessionManager struct {
    sessions map[string]*Session  // sessionID → Session
    mu       sync.Mutex
}

type Session struct {
    ID     string
    Shell  *exec.Cmd    // persistent bash process
    Stdin  io.WriteCloser
    Stdout io.Reader
    Cwd    string
    Env    []string
}

// The exec verb takes a session_id (broker-minted random ID, not agentRunID).
// If session doesn't exist, broker creates it and returns the new ID.
// The broker tracks cwd and env per session. Background jobs persist across calls.
// On broker restart, all sessions are invalidated (broker incarnation ID changes).
```

### Host-Side Diagnostic Tools (NEW from review)

The super needs tools that bypass the capsule layer entirely. These are Go
code in the executor reading kernel files directly — no broker needed.

**Critical: symlink protection.** These tools walk capsule-controlled paths
from host-privileged code. A malicious cosuper can plant a symlink pointing
outside the capsule before a crash. All path traversal uses
`openat2(RESOLVE_BENEATH)` or `O_NOFOLLOW` to prevent symlink-based escape.

```go
// diagnostics.go

// InspectCapsuleRaw reads cgroup state, broker PID, upperdir size — no broker needed
func (e *Executor) InspectCapsuleRaw(id string) (*CapsuleDiagnostics, error)

type CapsuleDiagnostics struct {
    ID            string
    State         CapsuleState
    BrokerPID     int
    BrokerAlive   bool
    BrokerLastLog string
    CgroupStats   CgroupStats
    UpperDirSize  int64
    UpperDirInodes int64
    ProcessList   []ProcessInfo
    OOMEvents     int64
    LastError     string
}

type CgroupStats struct {
    MemoryCurrent int64
    MemoryMax     int64
    MemoryEvents  MemoryEvents
    CPUUsage      int64
    PIDsCurrent   int64
}

type MemoryEvents struct {
    OOM      int64
    OOMKill  int64
    OOMPause int64
}

// ExtractDiff captures overlay diff even if broker is dead
// Uses openat2(RESOLVE_BENEATH) for all path traversal
func (e *Executor) ExtractDiff(id string) ([]FileChange, error)

// ListCapsules returns all capsules with resource usage (for cross-capsule contention)
func (e *Executor) ListCapsules() []CapsuleSummary

type CapsuleSummary struct {
    ID            string
    State         CapsuleState
    OwnerRunID    string
    MemoryUsed    int64
    MemoryMax     int64
    CPUUsage      int64
    PIDsCurrent   int64
    UpperDirSize  int64
    CosuperCount  int
    Pinned        bool
}

// RestartBroker restarts a crashed broker without destroying the capsule
func (e *Executor) RestartBroker(id string) error

// ForceDestroy uses MNT_DETACH + force-kill for stuck capsules
func (e *Executor) ForceDestroy(id string) error
```

### TOCTOU Fix (v2 — covers ALL RPCs, not just exec)

`GetCapsule` → `Exec` is TOCTOU. The super can call `destroy_capsule`
between a cosuper's lookup and its RPC call landing. v2 fix: the refcount
covers **every broker RPC** (exec, read_file, write_file, edit_file,
rename, remove, etc.), not just exec. A `write_file` mid-rename is exactly
the state you can't tear down.

```go
func (c *Capsule) acquireOp() error {
    c.inflightMu.Lock()
    defer c.inflightMu.Unlock()
    if c.State == CapsuleStateDestroying || c.State == CapsuleStateDestroyed {
        return fmt.Errorf("capsule %s is being destroyed", c.ID)
    }
    if c.State == CapsuleStateQuiescing || c.State == CapsuleStateFrozen {
        return fmt.Errorf("capsule %s is frozen for commit", c.ID)
    }
    c.inflightOps++
    return nil
}

func (c *Capsule) releaseOp() {
    c.inflightMu.Lock()
    defer c.inflightMu.Unlock()
    c.inflightOps--
}

func (c *Capsule) Destroy(ctx context.Context) error {
    c.mu.Lock()
    c.State = CapsuleStateDestroying
    c.mu.Unlock()
    
    // Wait for in-flight ops to complete (with timeout)
    deadline := time.Now().Add(30 * time.Second)
    for time.Now().Before(deadline) {
        c.inflightMu.Lock()
        if c.inflightOps == 0 {
            c.inflightMu.Unlock()
            break
        }
        c.inflightMu.Unlock()
        time.Sleep(100 * time.Millisecond)
    }
    
    // On timeout: SIGKILL all processes in cgroup, then proceed
    // (in-flight ops are killed at the syscall level)
    cgroup.KillAll(c.Cgroup)
    
    // Unmount, cleanup...
}
```

### VM Memory Admission Control (v2 — fixed double-counting)

v1 double-counted tmpfs: `MemoryMax + DiskMax` treated them as independent
budgets. But tmpfs pages are charged to the capsule's cgroup `memory.max`
(cgroup v2 unified accounting). The correct model: `memory.max` is the
total budget (RSS + tmpfs + kmem), and `DiskMax` is a sub-budget within it.

```go
func (e *Executor) Spawn(ctx context.Context, spec SpawnSpec) (*Capsule, error) {
    // v2: MemoryMax is the total budget. DiskMax is within it.
    // Admission checks MemoryMax only (tmpfs is already counted by cgroup).
    if spec.DiskMax > spec.MemoryMax - headroom {
        return nil, fmt.Errorf("disk max %d exceeds memory budget %d minus headroom %d",
            spec.DiskMax, spec.MemoryMax, headroom)
    }
    if e.vmMemoryUsed + spec.MemoryMax > e.vmMemoryTotal {
        return nil, fmt.Errorf("memory admission denied: need %d, available %d",
            spec.MemoryMax, e.vmMemoryTotal - e.vmMemoryUsed)
    }
    
    // ... spawn capsule ...
    
    e.vmMemoryUsed += spec.MemoryMax
    // ...
}

func (e *Executor) Destroy(ctx context.Context, id string) error {
    // ... destroy capsule ...
    e.vmMemoryUsed -= caps.MemoryMax
}
```

Tmpfs pages are charged to the cgroup of the task that faults them in. The
broker and cosuper processes are in the capsule's cgroup, so tmpfs writes
count against `memory.max`. The executor does NOT need to join the cgroup
before mounting — charging follows the writing task, not the mounting task.

## Integration with Existing Choir Code

### 1. Tool Context (tool_profiles.go)

Add a new context key for capsule handle (not raw capsule ID):

```go
// Add to the const block:
toolCtxCapsuleHandles toolContextKey = "capsule_handles"
```

Add to `WithToolExecutionContext`, inside the `if rec.Metadata != nil` block:

```go
if handles, ok := rec.Metadata["capsule_handles"].([]string); ok && len(handles) > 0 {
    ctx = context.WithValue(ctx, toolCtxCapsuleHandles, handles)
}
```

### 2. Runtime (runtime.go)

Add the executor to the Runtime struct:

```go
type Runtime struct {
    // ... existing fields ...
    toolProfiles     map[string]*ToolRegistry
    capsuleExecutor  *capsule.Executor // nil if capsules not configured
}
```

### 3. Tool Registry (tool_profiles.go)

In `InstallDefaultAgentTools`, add capsule tool registration:

```go
if rt.capsuleExecutor != nil {
    // Super gets capsule management + diagnostic tools
    registerCapsuleTools(superRegistry, rt.capsuleExecutor)
    // CoSuper gets capsule-aware bash tool (uses capability handles)
    registerCapsuleAwareBashTool(coSuperRegistry, rt.capsuleExecutor)
}
```

### 4. New Tools for Super (tools_capsule.go)

```go
func registerCapsuleTools(registry *ToolRegistry, exec *capsule.Executor) {
    // spawn_capsule — create a capsule
    // destroy_capsule — destroy a capsule
    // pin_capsule — mark as long-lived (exempt from GC)
    // inspect_capsule — diagnostic info (bypasses broker)
    // list_capsules — all capsules with resource usage
    // capsule_diff_preview — current overlay diff without committing
    // spawn_cosuper — spawn agent with capsule access (capability handles)
    // grant_access — grant capsule access mid-task (mints capability)
    // commit_transaction — quiesce, diff, classify, commit to tape
    // restart_broker — restart crashed broker
    // force_destroy — MNT_DETACH + force-kill for stuck capsules
}
```

### 5. Modified Bash Tool (tools_coding.go)

The cosuper's bash tool uses a capability handle, not a raw capsule ID:

```go
Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
    var in args
    if err := json.Unmarshal(raw, &in); err != nil {
        return "", fmt.Errorf("decode bash args: %w", err)
    }

    // Get capsule handle from tool args (not context)
    handle := in.Capsule // e.g. "build-a"
    if handle == "" {
        return "", fmt.Errorf("bash requires capsule handle")
    }

    // Resolve handle → capability → capsule (keyed by agentRunID + handle)
    agentRunID := stringFromToolContext(ctx, toolCtxRunID)
    cap, err := executor.ResolveCapability(agentRunID, handle)
    if err != nil {
        return "", fmt.Errorf("invalid capsule handle: %w", err)
    }

    // Verify capability is valid for this agent run
    if cap.AgentRunID != agentRunID {
        return "", fmt.Errorf("capability not valid for this run")
    }

    // Check that the verb is allowed for this role
    if !cap.Verbs["exec"] {
        return "", fmt.Errorf("role %s does not allow exec", cap.AgentRole)
    }

    // Route to capsule via capability
    caps, err := executor.GetCapsule(cap.CapsuleID)
    if err != nil {
        return "", fmt.Errorf("get capsule: %w", err)
    }

    result, err := caps.Exec(ctx, cap, capsule.ExecRequest{
        Command:   in.Command,
        SessionID: in.SessionID, // broker-minted random ID (not agentRunID)
        TimeoutMS: 30000,        // 30 seconds in milliseconds
    })
    if err != nil {
        return "", fmt.Errorf("capsule exec: %w", err)
    }

    return toolResultJSON(map[string]any{
        "command":    in.Command,
        "exit_code":  result.ExitCode,
        "output":     result.Stdout,
        "session_id": result.SessionID, // returned so agent can reuse session
    })
},
```

### 5b. Researcher Read-Path (tools_researcher.go)

The researcher's `read_file` tool uses `ResolveTarget` to fan out across
all capsules. The researcher never talks to a broker directly:

```go
Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
    var in struct {
        Path    string `json:"path"`
        Capsule string `json:"capsule"` // handle from spawn_researcher
    }
    json.Unmarshal(raw, &in)

    agentRunID := stringFromToolContext(ctx, toolCtxRunID)
    cap, err := executor.ResolveCapability(agentRunID, in.Capsule)
    if err != nil {
        return "", fmt.Errorf("invalid capsule handle: %w", err)
    }
    if !cap.Verbs["read_file"] {
        return "", fmt.Errorf("role %s does not allow read_file", cap.AgentRole)
    }

    // Resolve wildcard to concrete capsule IDs
    targetIDs, err := executor.ResolveTarget(cap)
    if err != nil {
        return "", fmt.Errorf("resolve target: %w", err)
    }

    // Fan out: issue read_file to each capsule's broker
    results := make(map[string]any)
    for _, cid := range targetIDs {
        caps, err := executor.GetCapsule(cid)
        if err != nil {
            results[cid] = map[string]any{"error": err.Error()}
            continue
        }
        content, err := caps.ReadFile(ctx, cap, in.Path)
        if err != nil {
            results[cid] = map[string]any{"error": err.Error()}
            continue
        }
        results[cid] = map[string]any{"content": content}
    }
    return toolResultJSON(results)
},
```

### 6. Coagent Spawning (tools_coagent.go)

When a super spawns a cosuper or researcher with capsule access, the
executor mints Ed25519-signed capabilities with role-based verb sets:

```go
// Spawning a cosuper with capsule access
if accessList, ok := constraints["capsule_access"].([]map[string]any); ok {
    for _, access := range accessList {
        capsuleID, _ := access["capsule_id"].(string)
        handle, _ := access["handle"].(string)

        cap, err := rt.capsuleExecutor.MintCapability(
            childRunID, capsule.RoleCosuper, capsuleID, 24*time.Hour,
        )
        if err != nil {
            return "", fmt.Errorf("mint capability: %w", err)
        }

        // Store handle in child metadata (not raw UUID)
        childMetadata["capsule_handles"] = append(
            childMetadata["capsule_handles"].([]string), handle,
        )
    }
}

// Spawning a researcher — gets read access to ALL capsules
// capsuleID is "" (wildcard); TargetCapsule is set to "*" by MintCapability
// for researcher role. Executor expands "*" at call time via ResolveTarget.
if isResearcher {
    cap, err := rt.capsuleExecutor.MintCapability(
        childRunID, capsule.RoleResearcher, "", 24*time.Hour,
    )
    if err != nil {
        return "", fmt.Errorf("mint researcher capability: %w", err)
    }
    childMetadata["capsule_handles"] = []string{cap.Handle}
    childMetadata["external_access"] = []string{"dolt:write", "message:send"}
}
```

## Capsule Lifecycle (v2 — bounded pins, quarantine, orphan recovery)

### Default: Ephemeral

Capsules are ephemeral by default, tied to the super's run record:
- On super run completion: all capsules destroyed (after diff extraction)
- "Has uncommitted overlay diff" exempts capsule from GC — but with a TTL
  (see Quarantine below)

### Long-Lived: Opt-in via Pin (v2 — bounded)

```go
// Super calls pin_capsule(capsule_id, timeout=24h)
// Hard max TTL: 24h, renewable
// Pin records owner run lineage
// Pin state persisted to stateDir/<id>/pin.json (survives executor restart)
// Orphaned pins (owner run dead) → GC-eligible
const MaxPinTTL = 24 * time.Hour
```

### GC Policy (v2)

- Running > 1hr with no agent binds AND no uncommitted diff → destroy
- Frozen > 30min with no thaw AND no uncommitted diff → destroy
- Pinned capsules: exempt until pin expires OR owner run is dead
- Owner run completed: destroy all non-pinned capsules (after diff extraction)
- Executor restart: scan stateDir, reattach or quarantine+destroy orphans

### Uncommitted Diff Quarantine (v2 — not immortal)

"Has uncommitted diff" prevents silent deletion but NOT indefinite retention.
After TTL (e.g., 4h idle with uncommitted diff):
1. Freeze capsule
2. Extract diff to durable quarantine artifact (tape-adjacent, auditable)
3. Mark capsule abandoned
4. Destroy runtime resources
5. Preserve diff artifact for audit

## Transaction Commit Invariants (v2 — expanded)

Before `Tape.Append`, capture:

```go
type CommitMetadata struct {
    CapsuleID           string
    ContributingAgentRuns []string    // from broker logs
    CommandLog          []byte        // v2: actual command log content (not just interval)
    CommandLogHash      string        // SHA-256 of command log for integrity
    QuiesceTimestamp    time.Time
    OverlayUpperDigest  string        // hash of upperdir manifest at commit
    PreviousEntryHash   string        // v2: hash chain for tamper-evidence
    ClassifierVersion   string
    ClassifierRulesetDigest string    // v2: content hash of ruleset (not just version)
    BrokerVersion       string
    BrokerBinaryHash    string        // v2: content hash of broker binary
    CapsuleSpec         SpawnSpec
    ResourceTier        ResourceTier
    UnauthorizedAttempts []UnauthorizedAttempt // v2: detailed, not just count
    BrokerRestarted      bool
    BrokerIncarnationID string         // v2: for restart tracking
    SharedCapsule        bool
    CapabilityGrants    []GrantRecord  // v2: who had access, what role, what verbs
    CapabilityRevocations []string      // v2: revoked handles during interval
    ResourceUsage       ResourceUsage  // v2: peak mem, CPU, IO, PID count
    ExtractionPath      string         // v2: "clean-quiesce" vs "force-extracted-post-crash"
}

type UnauthorizedAttempt struct {
    Timestamp time.Time
    AgentRunID string
    Method    string
    Path      string
    Reason    string  // "expired", "wrong_role", "invalid_signature", etc.
}

type GrantRecord struct {
    AgentRunID string
    AgentRole  AgentRole
    Handle     string
    Verbs      VerbSet
    GrantedAt  time.Time
    ExpiresAt  time.Time
}

type ResourceUsage struct {
    PeakMemoryBytes int64
    TotalCPUSeconds  int64
    TotalIOBytes     int64
    PeakPIDs         int64
    RPCCount         int64
}
```

## Failure Modes (Updated from v0)

| Scenario | Detection | Recovery |
|----------|-----------|----------|
| Namespace creation fails | gonso error | Abort, report to super |
| EROFS mount fails | unix.Mount error | Abort, report to super |
| Broker crashes | inspect_capsule_raw shows BrokerAlive=false | restart_broker(id) |
| Broker crashes (unrecoverable) | restart_broker fails | extract_diff(id), then destroy |
| Command hangs | Context timeout | Kill command, return timeout |
| Cosuper hangs at quiesce | 30s ack timeout | Force freeze, capture partial diff |
| Overlay fills (ENOSPC) | Write error | Report to cosuper, abort capsule |
| Host OOM-kills capsule | cgroup OOM event | Mark failed, bound cosupers get error on next exec |
| Destroy hangs (EBUSY) | Timeout | force_destroy: MNT_DETACH + force-kill |
| Executor crashes | Supervised process restart | Scan stateDir, reattach or extract+destroy orphans |
| Memory overcommit | Admission control check | spawn_capsule rejected |

## What Stays from v0/v1

- Decoupled agent/capsule model (orthogonal lifecycles)
- N agents per M capsules (super-controlled topology)
- Super without bash (but has diagnostic tools)
- EROFS sharing (mount once at boot)
- Air-gapped capsules (CLONE_NEWNET per capsule, no interfaces)
- Root in namespace (no user namespace for workload — v2 adds privilege
  separation via per-capsule unprivileged UID + user namespace for broker
  only. Workload shells retain CAP_DAC_OVERRIDE + CAP_FOWNER for overlayfs
  copy-up, drop all other caps. Seccomp: targeted denylist for workload
  (blocks setns/unshare/mount/bpf/ptrace), allowlist for broker)
- All prior consensus decisions (Q1-Q8)
- Library stack (all pure Go)
- Typed RPC broker protocol (all file operations)
- Broker-side session management
- Host-side diagnostic tools (openat2-safe, FIFO-safe)
- Defense-in-depth framing (microVM = primary, capsule = secondary)

## v6 Decisions (from v5 consensus fixes)

1. **HostAuthority revocation persistence** — append-only log on host disk,
   fsynced before ack. Survives HostAuthority crashes. Wildcard revocations
   stored in a separate globalRevokedCaps set, replayed on restart.
2. **vsock auth threat model clarified** — two-plane protects against
   user-space compromise, not guest-kernel LPE. MicroVM is primary
   boundary for kernel compromise. /dev/vsock not mounted into capsule
   namespaces; cgroup v2 device controller restricts access.
3. **Network enforcement via seccomp** — denies `socket(AF_INET)`,
   `socket(AF_INET6)`, `socket(AF_NETLINK)` at creation time (seccomp-BPF
   can filter socket()'s first arg, cannot dereference sockaddr pointers).
   AF_UNIX allowed. ~~No CLONE_NEWNET needed.~~ (v10: CLONE_NEWNET
   restored for abstract Unix socket isolation.)
4. **Wildcard researcher revocation** — CapabilityID added to globalRevokedCaps
   set, which is checked at every capsule spawn and synced to all brokers.
5. **sync_revoked_caps rename** — was sync_auth_epoch. Now reflects
   per-CapabilityID revoked set, not epoch. All references updated.
6. **Classifier runs in HostAuthority** — trust-bearing classification
   runs on host. Executor extracts diff, sends manifest to host for
   classification and tape append. Package layout updated.
7. **HostAuthority mint-request authorization** — rejects role=super from
   Executor, TTL > 24h, unknown capsuleID, unknown agentRunID. Prevents
   compromised Executor from minting arbitrary capabilities.
8. **Decision doc fully reconciled** — all stale v3/v4 entries (Q6, Q12-Q14,
   Q18-Q20) updated to v5/v6 positions. No contradictions.

## v8 Decisions (from v7 consensus fixes)

1. **AF_VSOCK seccomp block** — broker/workload seccomp denies
   `socket(AF_VSOCK)` at creation. Prevents compromised cosuper from
   dialing CID_HOST directly. CID is per-VM, not per-process, so CID
   filtering cannot distinguish Executor from cosuper.
2. **FD hygiene** — close_range(3, ~0, CLOSE_RANGE_CLOEXEC) before exec.
   Startup /proc/self/fd check. Prevents inherited vsock/inet fds from
   bypassing seccomp socket creation block.
3. **Classifier unknown-path policy** — unknown non-ephemeral paths
   rejected at commit time. Silently classifying as LedgerVM creates a
   trust-bearing catch-all.
4. **Package classifier fixed** — implementation doc sketch now says
   `package main` (cmd/capsule-host), matching package layout.
5. **io.device fixed** — replaced with eBPF device filter program
   (correct cgroup v2 terminology).
6. **RegisterCapsule/RegisterActiveRun RPCs** — added to HostAuthority
   for populating knownCapsules and activeRuns sets used by mint auth.
7. **Executor globalRevokedCaps field** — added to Executor struct for
   tracking wildcard revoked CapabilityIDs.
8. **Decision doc stale text fixed** — "needs re-run" replaced with
   "All three completed with rich findings."

## v9 Decisions (from v8 consensus fixes)

1. **globalRevokedCaps in design doc Executor struct** — added to match
   implementation doc. Both sketches now identical.
2. **Seccomp argument filtering code sketch** — implementation doc now
   shows concrete `socket()` arg filter blocking AF_INET(2), AF_INET6(10),
   AF_NETLINK(16), AF_VSOCK(40) while allowing AF_UNIX(1). Broker
   allowlist also documented.
3. **RegisterCapsule/RegisterActiveRun in design doc** — added to
   HostAuthority method list in design doc, matching implementation doc.
4. **CLOSE_RANGE_CLOEXEC wording clarified** — marks fds CLOEXEC (not
   closes them), so Executor's own vsock fd survives in parent but is
   not inherited by child processes.

## v10 Decisions (from v9 consensus fixes)

1. **CLONE_NEWNET restored** — abstract Unix sockets are scoped by network
   namespace, not mount namespace. Without CLONE_NEWNET, workloads could
   communicate across capsule boundaries via abstract sockets. CLONE_NEWNET
   + seccomp socket family filtering = defense in depth.
2. **Struct divergence resolved** — design doc is canonical for struct
   definitions. Implementation doc sketches are simplified views with
   explicit note.

## v11 Decisions (from v10 consensus fixes)

1. **Stale "no network namespace" references reconciled** — all 9 stale
   references across the three docs updated to reflect CLONE_NEWNET per
   capsule. Implementation doc Section 8 rewritten. Library stack table
   updated. Decision doc defense-in-depth list updated.
2. **Seccomp OpEq expanded to individual rules** — each denied socket
   family (AF_INET, AF_INET6, AF_NETLINK, AF_VSOCK) gets its own
   seccomp.Rule with single Value, matching elastic/go-seccomp-bpf API.
3. **CLONE_NEWUSER clarified** — namespace sketch now notes NEWUSER is
   for broker privilege separation; workload retains root for overlayfs.

## v12 Decisions (from v11 consensus fixes)

1. **Two wrapped stale references fixed** — decision doc Q3 and Q24 had
   "No network / namespace" wrapped across line breaks, missed by
   line-based grep. Now updated to CLONE_NEWNET per capsule. Multiline
   grep verification confirms zero live stale references.
2. **Seccomp AND/OR bug fixed** — each denied socket family now has its
   own SyscallGroup entry (separate entries are ORed at filter level).
   Previous single-Args-slice approach ANDed the rules, making the filter
   impossible to trigger (arg[0] can't equal 2 AND 10 AND 16 AND 40
   simultaneously).

## v13 Decisions (from v12 consensus fixes)

1. **Seccomp code sketch updated to match actual library API** — uses
   `NamesWithCondtions` (note: library has typo), `NameWithConditions`,
   `ArgumentConditions`, `Condition{Argument, Operation: Equal, Value}`
   matching the real `elastic/go-seccomp-bpf` API.
2. **ActionErrno includes EPERM errno payload** — `denyEPERM :=
   seccomp.ActionErrno | seccomp.Action(unix.EPERM)`. Without this,
   denied syscalls could appear to return success (errno=0).
3. **Stale "Rejected for MVP" gVisor text fixed** — now says "Rejected
   (production-only, no MVP)."

## v14 Decisions (from v13 consensus fixes)

1. **session_id contradiction fixed** — implementation doc corrected to
   describe session_id as broker-minted random ID (NOT agentRunID),
   bound to {agentRunID, capsuleID, capabilityID, brokerIncarnationID}.
   Matches design doc and decision doc Q12/Q30.
