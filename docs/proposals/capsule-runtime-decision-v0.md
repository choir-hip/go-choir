# Capsule Runtime Decision — v14

**Status:** Synthesis of three parallel deep-dive research threads + 8+6+4+5+5+5+5
agent consensus. Production-only, no MVP. All open decisions resolved.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Supersedes:** v13 (adds v13 consensus fix: session_id contradiction resolved)

## Context

Three blocking tensions were identified during review of Nucleus as the capsule
runtime for Choir's autoputer self-transition architecture:

1. EROFS vs Nix store rootfs (integrity model mismatch)
2. CapsuleExecutor + CapsuleDiffClassifier (missing integration layer)
3. Cosuper sharing model within capsules

Three parallel research subagents were launched. All three completed with
rich findings, which were synthesized into the v7 design.

## The Critical Finding

**OverlayFS copy-up requires `CAP_DAC_OVERRIDE` and `CAP_FOWNER` in the
workload process. This is a Linux kernel requirement, not a Nucleus design
choice.**

Source: Nucleus documentation states:

> Overlay rootfs mode is a writable development snapshot mode, not the strict
> production posture. To support overlayfs copy-up, Nucleus retains
> CAP_DAC_OVERRIDE and CAP_FOWNER in the workload and grants native Landlock
> read/write/execute access to /. Use bind rootfs mode for the default-deny
> Landlock and all-capabilities-dropped posture.

The kernel raises these capabilities internally for overlayfs copy-up
operations:
- `CAP_DAC_OVERRIDE`: creating files in workdir, rename operations, bypassing
  DAC checks during copy-up
- `CAP_FOWNER`: chmod, timestamp updates, removing whiteouts from sticky
  directories

**Implication:** Any approach using kernel overlayfs — Nucleus, custom Go
runtime, or raw `mount -t overlay` — cannot achieve the "all caps dropped,
default-deny Landlock" posture. The elevated capabilities are unavoidable for
transparent copy-up.

This is not an impedance mismatch with Nucleus. It is an impedance mismatch
with the Linux kernel's overlayfs implementation.

## The Design Space

| Approach | Transparent writes | Capabilities | Diff extraction | Perf |
|----------|-------------------|-------------|-----------------|------|
| **A. Kernel overlayfs** | Yes (copy-up) | Elevated (CAP_DAC_OVERRIDE, CAP_FOWNER) | Walk upperdir (trivial) | Native |
| **B. Bind-mount + writable dirs** | No (explicit paths) | All dropped | Compare writable vs base (moderate) | Native |
| **C. FUSE userspace overlay** | Yes (userspace copy-up) | All dropped | Custom (complex) | ~2-10x slower |

### Approach A: Kernel overlayfs (accept elevated caps)

The capsule sees a unified filesystem. Reads come from the EROFS base, writes
go to the upper layer transparently. The workload retains CAP_DAC_OVERRIDE and
CAP_FOWNER but drops all other capabilities.

**Pros:**
- Transparent writes (workload doesn't need to know which paths are writable)
- Trivial diff extraction (walk upperdir, handle whiteouts/opaque dirs)
- Native performance
- Simplest implementation

**Cons:**
- CAP_DAC_OVERRIDE bypasses discretionary access control (DAC) checks
- CAP_FOWNER allows operations on files regardless of ownership
- Landlock must grant read/write/execute on / (not default-deny)
- Weaker isolation than Choir's stated goal

**Security assessment:**
- CAP_DAC_OVERRIDE + CAP_FOWNER are filesystem capabilities, not system-wide
- They allow the workload to bypass file permission checks within its namespace
- They do NOT grant: root access, ability to load kernel modules, network
  raw sockets, ptrace, or namespace escape
- The workload is still confined to its mount namespace, PID namespace, network
  namespace, etc.
- seccomp can still restrict syscalls
- The threat model is: a compromised cosuper can read/write any file in the
  capsule's overlay, but cannot escape the capsule

**Key question:** Is "can read/write any file in the capsule" acceptable? The
capsule is already a single workspace for the cosuper. The cosuper's bash tool
already has full access to the capsule's filesystem. The elevated caps don't
grant access beyond the capsule's namespace.

### Approach B: Bind-mount + writable dirs (strict isolation)

The capsule gets a read-only bind mount of the EROFS base. Specific paths are
bind-mounted as writable directories (e.g., /home/user/src, /tmp, /var/lib/dolt).
All capabilities are dropped. Landlock is default-deny with explicit allowlist.

**Pros:**
- Strict isolation (all caps dropped, default-deny Landlock)
- Matches Choir's stated security posture
- No overlayfs capability issue

**Cons:**
- Not transparent (workload must know which paths are writable)
- Diff extraction requires comparing writable dirs against base (moderate)
- Must pre-declare writable paths (inflexible)
- If workload writes to a read-only path, it gets EROFS (read-only error)
- May need many bind mounts for different ledger paths

**Security assessment:**
- Strongest isolation
- Workload cannot bypass DAC (no CAP_DAC_OVERRIDE)
- Landlock can be default-deny with explicit path allowlist
- A compromised cosuper is confined to explicitly writable paths

**Key question:** Can we pre-declare all writable paths? The ontology defines
ledger paths:
- V (VM/runtime): /boot, /lib/modules, /etc/systemd, /nix/store
- D (Dolt): /var/lib/dolt, /mnt/persistent/dolt
- S (Source): /home/user/src, /workspace
- B (Blobs): /var/lib/blob, /mnt/blob
- A (Artifacts): /var/lib/artifact
- R (Route): /etc/choir/route

But capsules may also write to:
- /tmp (ephemeral)
- /root/.cache, /home/user/.cache (build caches)
- /run (runtime state)
- Arbitrary paths the cosuper chooses

The pre-declaration requirement is restrictive. A cosuper running `go build`
writes to $GOCACHE, $GOPATH, the source directory, /tmp, and potentially
arbitrary paths. Pre-declaring all of these is fragile.

### Approach C: FUSE userspace overlay (strict + transparent)

A FUSE filesystem implements copy-up in userspace. The workload sees a unified
filesystem. All capabilities are dropped. FUSE handles the overlay logic without
kernel capabilities.

**Pros:**
- Transparent writes (like Approach A)
- Strict isolation (like Approach B)
- All caps dropped, default-deny Landlock possible

**Cons:**
- ~2-10x performance overhead (FUSE context switches)
- Complex implementation (FUSE daemon, copy-up logic, whiteout handling)
- FUSE daemon itself needs capabilities (but outside the workload namespace)
- Larger attack surface (FUSE daemon is a userspace program handling all FS ops)
- Go FUSE libraries exist but are not trivial to integrate

**Security assessment:**
- Strong isolation for the workload
- FUSE daemon runs outside the workload namespace (in the capsule manager)
- FUSE daemon needs access to both base and upper dirs
- Attack surface: FUSE daemon bugs could leak data or corrupt state

**Key question:** Is the performance overhead acceptable? For agent workloads
(bash commands, file edits, builds), 2-10x FS overhead may be tolerable for
most operations but painful for large builds (go build, npm install).

## Recommendation: Approach A (kernel overlayfs, accept elevated caps)

### Rationale

1. **The elevated capabilities are filesystem-scoped, not system-scoped.**
   CAP_DAC_OVERRIDE and CAP_FOWNER allow bypassing file permission checks
   within the capsule's mount namespace. They do not grant namespace escape,
   kernel module loading, network raw sockets, or ptrace.

2. **The capsule is already a single-tenant workspace.** The cosuper's bash
   tool already has full access to the capsule's filesystem. The elevated caps
   don't grant access beyond what the cosuper already has — they just allow
   overlayfs copy-up to work.

3. **The threat model is two-layer.** The microVM is the primary security
   boundary (protects the host). The capsule is the secondary security
   boundary (defense-in-depth: prevents cross-capsule data leakage, sabotage,
   forged edits, poisoned diffs). Both are security boundaries with different
   threat models. The namespace isolation + seccomp + capability-based access
   control achieve intra-VM work isolation.

4. **Transparency is essential for agent workloads.** Cosupers run arbitrary
   bash commands that write to arbitrary paths. Pre-declaring writable paths
   (Approach B) is fragile and would break common workflows. FUSE (Approach C)
   adds complexity and performance overhead that isn't justified.

5. **Diff extraction is trivial.** Walking the overlay upperdir is simple,
   deterministic, and fast. With snapshot diff (v2 decision), the overlay
   mount is stable for the capsule's lifetime. At commit, walk upperdir,
   compare against last commit's manifest → incremental diff. No remount,
   no layer stacking.

6. **Implementation is simplest.** We mount EROFS as lowerdir, tmpfs as
   upperdir, overlayfs as merged. No FUSE daemon, no bind-mount gymnastics.

### Defense-in-Depth Framing (Updated from review)

The capsule IS a security boundary — a defense-in-depth layer, not a
"convenience." The seccomp/landlock/capabilities are load-bearing security
controls. The framing matters: if engineers believe "the capsule is just a
convenience," they'll write less careful input validation, path traversal
checks, and symlink handling in the broker. The broker's JSON-RPC handler
must be written as if it's load-bearing, because it is.

- **MicroVM** = primary boundary (hardware virtualization, protects host)
- **Capsule** = secondary boundary (defense-in-depth: namespaces, seccomp,
  landlock, capabilities, cgroups, capability-based access control)
- Both are security boundaries with different threat models

### What we still get from strict isolation

Even with Approach A, we retain:
- PID namespace isolation (capsule processes invisible to host)
- Mount namespace isolation (capsule sees only its overlay)
- UTS/IPC namespace isolation
- seccomp syscall filtering (deny dangerous syscalls)
- cgroups v2 resource limits (CPU, memory, PIDs, disk)
- All capabilities dropped EXCEPT CAP_DAC_OVERRIDE and CAP_FOWNER
- Landlock with read/write/execute on / (broader than default-deny, but still
  confines to the capsule's mount namespace)
- **Capability-based access control** (executor-minted tokens, broker-verified)
- **Broker socket isolation** (sockets not visible across capsule namespaces)
- **Air-gapped network** (CLONE_NEWNET per capsule + seccomp socket family filter, all I/O via host)

### What we lose

- Default-deny Landlock (must grant / access)
- CAP_DAC_OVERRIDE and CAP_FOWNER are retained (DAC bypass within capsule)
- A compromised cosuper can read/write any file in the capsule (but it already
  could via bash — this is not a regression)

## The Runtime Choice: Custom Go vs Nucleus

Given Approach A (kernel overlayfs), the runtime choice simplifies:

### Nucleus (with patches)

**Required patches:**
1. Remove `/nix/store` path validation for rootfs (moderate)
2. Accept EROFS mount point as lowerdir (moderate)
3. Skip `.nucleus-rootfs-sha256` when using EROFS (trivial)
4. Replace HMAC with Ed25519 or ignore Nucleus signing (moderate)

**Remaining issues:**
- No programmatic API (CLI-only, must shell out per command or use nsenter)
- Rust dependency in a Go codebase
- `image commit --freeze` blocks (must quiesce for diff capture)
- No live diff streaming

**Effort:** 2-4 weeks integration + patches

### Custom Go runtime

**Required components (v5 — all pure Go, no CGO):**
1. Namespace creation (gonso) — handles LockOSThread
2. Overlayfs mounting (direct unix.Mount with userxattr)
3. cgroups v2 (containerd/cgroups/v3)
4. seccomp (elastic/go-seccomp-bpf — pure Go, no CGO)
5. Landlock (go-landlock, Linux 5.13+)
6. Capabilities dropping (moby/sys/capability)
7. Air-gapped (CLONE_NEWNET per capsule + seccomp socket family filter, host-mediated I/O)
8. Persistent shell process (long-lived bash with pipes)
9. Snapshot diff extraction (manifest walk, handle whiteouts/opaque dirs)
10. HostAuthority (Ed25519 signing on host, vsock transport)

**Effort:** Production-only, no MVP, no phased delivery. Hard cutover.

### Recommendation: Custom Go runtime

**Rationale:**

1. **The exec transport problem is fatal for Nucleus.** Nucleus has no
   programmatic API. Every bash command would require either:
   - Shelling out to `nucleus attach` (50-150ms overhead per command)
   - Using `nsenter` from Go (bypasses Nucleus entirely for exec)
   - Adding a daemon mode to Nucleus (fundamental architecture change)

   The persistent-shell approach (long-lived bash process inside the namespace,
   communicating via pipes) is the right design for Choir's high-frequency
   command execution. Nucleus doesn't support this.

2. **The Nix integration is incidental, not core.** Nucleus's Nix integration
   is for production NixOS services. Choir uses EROFS + tape-derived base. The
   Nix integration would need to be patched out or worked around.

3. **The overlay mode is the same either way.** Both Nucleus and a custom Go
   runtime use kernel overlayfs, which has the same capability requirements.
   Nucleus doesn't provide a security advantage here.

4. **Go-native integration is valuable.** The CapsuleExecutor needs to be
   deeply integrated with Choir's Go runtime (tool registry, cosuper
   goroutines, MutationTransaction builder). A Go-native implementation
   avoids the impedance of shelling out to a Rust CLI.

5. **The libraries exist.** gonso (namespaces), containerd/cgroups (cgroups),
   elastic/go-seccomp-bpf (seccomp, pure Go), go-landlock (Landlock),
   moby/sys/capability (capabilities). The implementation is assembly of
   existing, tested libraries, not greenfield development.

6. **Production-only, no MVP.** Hard cutover from current architecture.
   All components built together and shipped as one production system.

## The EROFS Question

The second research thread evaluated whether to change Choir's integrity model
to fit Nucleus's Nix store closure model.

**Finding:** Replacing EROFS+dm-verity with Nix store closures loses
kernel-enforced integrity. Nix store attestation is build-time verification,
not runtime enforcement. An attacker with root access can modify /nix/store
without detection. dm-verity rejects bad blocks at read time — this is a
fundamentally stronger security property.

**Decision:** Keep EROFS+dm-verity as the integrity root. Do not adopt Nix
store closures as the base integrity model.

**Implication for Nucleus:** Since we're keeping EROFS and building a custom
Go runtime, Nucleus's Nix integration is irrelevant. We mount EROFS as the
overlay lowerdir directly.

## The CapsuleDiffClassifier

Regardless of runtime choice, we need to classify overlay diffs into ledger
deltas. The classifier should:

1. **Run on the host (trusted zone).** The VM exports the overlay diff; the
   host classifies it. This keeps the classifier in the trusted zone.

2. **Be deterministic and versioned.** The classifier is part of the trust
   chain. If it changes, old commits may not verify. Version it explicitly.

3. **Use path patterns from the ontology.** Map paths to ledgers:
   - V: /boot, /lib/modules, /etc/systemd, /nix/store
   - D: /var/lib/dolt, /mnt/persistent/dolt
   - S: /home/user/src, /workspace
   - B: /var/lib/blob, /mnt/blob
   - A: /var/lib/artifact
   - R: /etc/choir/route

4. **Handle unknown paths.** Options: reject (fail commit), classify as V
   (catch-all for runtime state), or ignore (ephemeral paths like /tmp, /run).

5. **Handle ephemeral paths.** /tmp, /run, /var/log should be ignored (not
   ledger state). Build caches (/root/.cache, /home/user/.cache) should be
   classified as V (runtime state) or ignored.

## The HMAC/Signing Question

The third research thread (Nucleus deep-dive) confirmed that Nucleus's HMAC
key lives in the VM, which is the untrusted zone. This is insufficient as a
trust anchor.

**Decision:** Ignore Nucleus's signing entirely. Compute our own content
hashes (SHA-256) of the overlay diff on the host. The host is the trusted
zone. The diff hash is content-addressed and substrate-independent.

This is consistent with the MutationTransaction design: the transaction
records content-addressed ledger hashes, not HMAC signatures.

## Open Questions (Resolved from v1 + v2 + v3 + v4 + v5 + v6 + v7 + v8 + v9 + v10 + v11 + v12 + v13 + v14 Review Rounds)

1. **CapsuleExecutor design** — Resolved. See `capsule-executor-design-v0.md`
   (v14). Two-plane architecture: HostAuthority (host) + Executor (guest) +
   vsock. Exec-broker with typed RPCs, session-aware, Ed25519 capability-verified.

2. **Agent sharing model** — Resolved. N agents per M capsules.
   Super-controlled topology. Role-based capability access. Shared
   upperdir for collaboration, separate shells via session_id.

3. **Network egress policy** — Resolved. Air-gapped capsules. CLONE_NEWNET
   per capsule (no interfaces) + seccomp socket family filter. All network
   I/O mediated by host.

4. **Resource limits** — Resolved. Tiered presets (small/medium/large).
   VM memory admission control. v2 fix: `memory.max` is total budget,
   tmpfs is sub-budget (no double-counting).

5. **Incremental diff** — Resolved (v2). Snapshot diff: manifest-based,
   no remount. Walk upperdir, compare against last commit's manifest.
   Crash-safe, session-continuous.

6. **Capsule access control** — Resolved (v6). Ed25519 asymmetric
   capabilities. HostAuthority (on Firecracker host) holds private key,
   Executor (in guest) requests minting via vsock, broker (guest) holds
   public key. Role-based verb sets (super/cosuper/researcher). Agent sees
   opaque handles, not raw UUIDs. Per-CapabilityID revocation (not global
   epoch).

7. **Capsule lifecycle** — Resolved (v2). Ephemeral by default. Long-lived
   via `pin_capsule(id, timeout)` with 24h max TTL. Uncommitted diff
   quarantined after 4h idle. Orphan recovery on executor restart.

8. **Broker binary placement** — Resolved (v2). Bind-mounted from
   content-addressed host store. Content hash verified at spawn. Not
   baked into EROFS. Enables minutes-scale hotfixes.

9. **Trust anchor placement** — Resolved (v2). Ed25519 private key on
   Firecracker host (outside guest kernel). Public key injected into
   broker at spawn. No per-RPC vsock round-trip. Residual risk: public
   key replacement in broker memory (mitigated by binary hash verification
   + privilege separation).

10. **Capsule classes** — Resolved (v2). Single capsule type with
    role-based capabilities. All capsules have all permissions. Agent
    roles (super/cosuper/researcher) determine verb sets. Explicit
    doctrine delta documenting departure from strict-agent default.

11. **Privilege separation** — Resolved (v3). Broker runs as per-capsule
    unprivileged UID in user+mount namespace, with seccomp (allowlist for
    broker, targeted denylist for workload), landlock scoped to capsule
    dirs. Retains CAP_DAC_OVERRIDE + CAP_FOWNER for overlayfs copy-up,
    drops all other caps. Overlay mounted with userxattr.

12. **Session IDs** — Resolved (v5). Broker-minted random IDs bound to
    {agentRunID, capsuleID, capabilityID, brokerIncarnationID}.
    commitEpoch is NOT part of the binding (audit-only). Not agentRunIDs.

13. **Epoch model** — Resolved (v5). commitEpoch is audit metadata only
    (not enforced for exec/read/write). AuthEpoch was replaced by
    per-CapabilityID revoked sets in v5 — no global epoch counter.

14. **Revocation propagation** — Resolved (v5). HostAuthority adds
    CapabilityID to per-capsule revoked set. Sends sync_revoked_caps to
    Executor via vsock, which forwards to broker. Broker rejects any
    capability whose CapabilityID is in the revoked set.

15. **Manifest walker safety** — Resolved (v3). walkUpperdir uses lstat,
    O_NONBLOCK, refuses non-regular files, caps on entries/bytes/depth/time.
    mtime+size fast-path skips hashing unchanged files.

16. **Tape idempotency** — Resolved (v3). Tape append dedups by
    (capsuleID, commitEpoch). Crash recovery re-does commit, tape dedups.

17. **Secrets** — Resolved (v3). Secrets are managed by gateway and other
    services, not injected as env vars in SpawnSpec. If an agent acquires a
    secret at runtime, that's the agent's business.

18. **CommitEpoch check scope** — Resolved (v5). CommitEpoch is audit
    metadata only, NOT enforced for any verb. commit_transaction and
    extract_diff are host control-plane methods, not broker verbs, so
    there is no broker-side CommitEpoch gate at all.

19. **Researcher wildcard routing** — Resolved (v5). Executor expands
    TargetCapsule="*" to concrete capsule IDs at call time via
    ResolveTarget. Researcher never talks to a broker directly; executor
    fans out to each capsule individually.

20. **Super verbs vs broker verbs** — Resolved (v5). Super has NO broker
    verbs. All super operations (spawn, destroy, mint, revoke, commit,
    inspect, extract_diff, list_capsules) are Executor/HostAuthority host
    methods that bypass the broker entirely.

21. **ExecResult.SessionID** — Resolved (v4). Broker returns newly minted
    session ID in ExecResult so caller can use it for subsequent commands.

22. **mtime+size fast-path** — Resolved (v4). Cache hint only for
    non-authoritative preview diffs. Authoritative commit capture always
    hashes content (workload with CAP_FOWNER can restore mtime).

23. **acquireOp state checks** — Resolved (v4). Rejects RPCs while capsule
    is Quiescing or Frozen, not just Destroying/Destroyed.

24. **Network policy** — Resolved (v4, updated v11). Air-gapped capsules.
    CLONE_NEWNET per capsule (no interfaces) + seccomp socket family filter.
    All network I/O mediated by host. nftables section deleted.

25. **Host/guest authority boundary** — Resolved (v5). Two-plane architecture:
    HostAuthority (on Firecracker host, holds Ed25519 private key) +
    Executor (in guest, manages lifecycle) + vsock transport. Resolves the
    contradiction of a single process being both outside and inside the guest.

26. **Per-capability revocation** — Resolved (v5). HostAuthority tracks
    revoked CapabilityIDs in a per-capsule set, not a global AuthEpoch bump.
    Prevents collateral revocation of other agents sharing the capsule.

27. **AuthEpoch restore on broker restart** — Resolved (v5). Executor
    re-syncs full revoked-capability set from HostAuthority via vsock before
    broker accepts RPCs. Broker crash does NOT reset revocation.

28. **Super has no broker verbs** — Resolved (v5). All super operations
    (spawn, destroy, mint, revoke, commit, inspect, extract_diff,
    list_capsules) are Executor/HostAuthority host methods that bypass
    the broker entirely.

29. **commit_transaction is not a broker verb** — Resolved (v5). It's a
    host control-plane method. Removed from broker CommitEpoch gate.

30. **Session commitEpoch is audit-only** — Resolved (v5). Not part of
    session binding, not enforced. Sessions survive commits. Binding is
    {agentRunID, capsuleID, capabilityID, brokerIncarnationID}.

31. **Bash tool returns session_id** — Resolved (v5). Agent can reuse
    sessions across calls.

32. **Researcher read-path** — Resolved (v5). ResolveTarget fans out to
    all capsules, aggregates results, handles partial failures.

33. **HostAuthority revocation persistence** — Resolved (v6). Append-only
    log on host disk, fsynced before ack. Survives HostAuthority crashes.

34. **vsock auth threat model** — Resolved (v6). Two-plane protects against
    user-space compromise, not guest-kernel LPE. MicroVM is primary boundary
    for kernel compromise. /dev/vsock restricted via device cgroup.

35. **Network enforcement** — Resolved (v9). Two layers: (1) CLONE_NEWNET
    per capsule (isolated network namespace, no interfaces, prevents
    abstract Unix socket cross-capsule communication). (2) Seccomp denies
    `socket(AF_INET)`, `socket(AF_INET6)`, `socket(AF_NETLINK)`, AND
    `socket(AF_VSOCK)` at creation time. AF_UNIX allowed for broker
    control plane (scoped by CLONE_NEWNET). FD hygiene: close_range before
    exec, startup /proc/self/fd check.

36. **Wildcard researcher revocation** — Resolved (v6). CapabilityID added
    to every capsule's revoked set, including capsules spawned after
    revocation.

37. **Classifier placement** — Resolved (v6). Classifier runs in
    HostAuthority on host (trust-bearing). Executor extracts diff, sends
    manifest to host for classification and tape append.

38. **HostAuthority mint-request authorization** — Resolved (v6).
    HostAuthority rejects: role=super from Executor (super caps are
    host-local only), TTL > 24h, capsuleID not in known-spawned set
    (unless wildcard), agentRunID not in active-runs set. This prevents
    a user-space-compromised Executor from minting arbitrary capabilities.

39. **AF_VSOCK seccomp block** — Resolved (v7). Broker/workload seccomp
    denies `socket(AF_VSOCK)` at creation. Without this, a compromised
    cosuper could dial CID_HOST directly, bypassing the two-plane model.

40. **FD hygiene** — Resolved (v7). close_range(3, ~0, CLOSE_RANGE_CLOEXEC)
    before exec'ing broker/workload. Startup /proc/self/fd check. Prevents
    inherited vsock/inet fds from bypassing seccomp.

41. **Classifier unknown-path policy** — Resolved (v7). Unknown non-ephemeral
    paths are rejected at commit time. Silently classifying as LedgerVM
    creates a trust-bearing catch-all.

42. **Abstract Unix socket isolation** — Resolved (v10). CLONE_NEWNET
    restored per capsule. Abstract Unix sockets are scoped by network
    namespace, not mount namespace — without CLONE_NEWNET, workloads could
    communicate across capsule boundaries via abstract sockets. CLONE_NEWNET
    + seccomp socket family filtering = defense in depth.

43. **Struct divergence** — Resolved (v10). Design doc is canonical for
    struct definitions. Implementation doc sketches are simplified views
    with explicit note referencing the design doc.

44. **Stale "no network namespace" references** — Resolved (v11). All 9
    stale references across the three docs updated to reflect CLONE_NEWNET
    per capsule. Implementation doc Section 8 rewritten. Grep-verified.

45. **Seccomp OpEq multi-value semantics** — Resolved (v11). Expanded to
    individual seccomp.Rule entries with single Value per denied socket
    family, matching elastic/go-seccomp-bpf API.

46. **CLONE_NEWUSER scope** — Resolved (v11). NEWUSER is for broker
    privilege separation only; workload retains root for overlayfs copy-up.
    Namespace sketch now has explicit comment.

47. **Seccomp AND/OR semantics** — Resolved (v12). Each denied socket
    family gets its own SyscallGroup entry. Rules within a single Args
    slice are ANDed; separate SyscallGroups are ORed. Previous approach
    (all rules in one Args slice) was impossible to trigger.

48. **Wrapped stale references** — Resolved (v12). Two "No network /
    namespace" references in decision doc Q3/Q24 wrapped across line
    breaks, missed by line-based grep. Fixed with multiline grep
    verification.

49. **Seccomp API sketch accuracy** — Resolved (v13). Code sketch updated
    to match actual `elastic/go-seccomp-bpf` API: `NamesWithCondtions`,
    `NameWithConditions`, `ArgumentConditions`, `Condition{Argument,
    Operation: Equal, Value}`.

50. **ActionErrno errno payload** — Resolved (v13). `ActionErrno` without
    errno value can make denied syscalls appear to return success (errno=0).
    Fixed with `denyEPERM := seccomp.ActionErrno | seccomp.Action(unix.EPERM)`.

51. **Stale "Rejected for MVP" text** — Resolved (v13). gVisor verdict
    updated to "Rejected (production-only, no MVP)."

52. **session_id contradiction** — Resolved (v14). Implementation doc
    corrected: session_id is a broker-minted random ID, NOT agentRunID.
    Bound to {agentRunID, capsuleID, capabilityID, brokerIncarnationID}
    for session invalidation on broker restart. Matches design doc and
    decision doc Q12/Q30.

## Next Steps

1. Implement `internal/capsule/` package (executor, capsule, capability, roles, manifest)
2. Implement `cmd/capsule-broker/` binary (typed RPCs, session management, Ed25519 verify)
3. Implement CapsuleDiffClassifier (host-side, versioned, ruleset digest)
4. Wire CapsuleExecutor into Choir's tool registry
5. End-to-end self-transition: spawn capsule → agent works → commit → tape
6. Hard cutover from non-capsule architecture (no fallbacks)
