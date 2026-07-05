# Capsule Runtime v1 — Consensus Synthesis

**Status:** Synthesis of 8-agent consensus panel review of v1 design.
All open decisions resolved in v2 (see below).
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Panel:** codex (failed, output recovered), claude, devin, opencode, cursor, omp-gpt55, omp-gemini35, omp-glm52
**Outputs:** `/tmp/capsule-v1-consensus/`
**Decisions round:** `/tmp/capsule-v1-decisions/` (6 agents, excluding claude + glm52)

## Panel

| Agent | Status | Notes |
|-------|--------|-------|
| codex (default) | failed (exit 2) | Output recovered from manual run; full review captured |
| claude (default) | ok | Strongest on CommitLayer syscall-level analysis |
| devin (default) | ok | Strongest on HMAC key management, session ID security |
| opencode (mimo-v2.5) | ok | Most lenient — "no fatal flaws" |
| cursor (default) | ok | Strongest on trust anchor placement, memory tier table |
| omp-gpt55 (high) | ok | **Caught the lowerdir ordering bug no one else did** |
| omp-gemini35 (high) | ok | Strongest on memory double-counting, PID-1 subreaper |
| omp-glm52 (high) | ok | Strongest on privilege separation, doctrinal tension |

## Universal Consensus (8/8 agents)

These are agreed by every agent that produced output. They are not optional.

### C1. Capability scope is too coarse — needs verb set, not just AccessMode

**All 8 agents.** `AccessMode (ReadOnly | ReadWrite)` is decorative. `exec` can
write files (`echo x > f`) regardless of mode. The capability must enumerate
allowed RPC verbs, or `exec` must be unconditionally excluded from read-only
grants.

Recommended verb tiers (converged across agents):
```
ReadOnly:  read_file, list_dir, stat, lstat, readlink, file_hash
WriteFiles: ReadOnly + write_file, edit_file, mkdir, rename, truncate
Delete:    remove, remove_all
Exec:      exec, kill_session
Admin:     chmod, symlink (separate from WriteFiles for blast radius)
```

### C2. CommitLayer is broken at the syscall level

**All 8 agents.** Three independent fatal problems:

1. **`unix.Unmount` returns EBUSY.** Broker shell sessions have cwd and open
   fds inside the merged dir. Cgroup freeze pauses tasks but does NOT close
   fds or clear cwd. `MNT_DETACH` is worse — old mount stays alive for frozen
   processes, who keep writing to the old (now "immutable") upperdir after
   thaw. Silent divergence.

2. **Workdir filesystem mismatch.** New upperdir is a fresh tmpfs, but
   `c.WorkDir` is reused from the old tmpfs. Overlayfs requires workdir and
   upperdir on the **same filesystem** — the mount call returns EINVAL every
   time. The snippet has never been run.

3. **No crash atomicity.** A VM crash between unmount and mount leaves no
   merged view and no record of which epoch. Need a durable commit
   manifest/WAL with states: `PREPARED → TAPE_APPENDED → MOUNT_SWITCHED →
   COMMITTED`, plus recovery logic.

### C3. Memory admission formula double-counts tmpfs

**All 8 agents.** tmpfs pages are charged to the cgroup of the task that
faults them in (cgroup v2 unified memory accounting). If the broker and
cosuper are in the capsule's cgroup, tmpfs usage counts against
`memory.max`. The formula `MemoryMax + DiskMax` double-counts the same
budget.

The tier table is internally inconsistent:
| Tier | Memory (cgroup) | Disk (tmpfs) |
|------|------|------|
| small | 512MB | 1GB |
| medium | 1GB | 2GB |
| large | 2GB | 4GB |

**Every tier promises more tmpfs disk than the memory limit that governs
it.** A workload using the advertised disk quota gets OOM-killed before
reaching the tmpfs cap.

Correct model:
```
memory.max = total capsule memory budget (RSS + tmpfs + kmem)
tmpfs size <= memory.max - process_headroom - broker_headroom
```

### C4. Host diagnostic FIFO/device DoS

**All 8 agents.** `openat2(RESOLVE_BENEATH)` prevents symlink escape but
does NOT prevent FIFO/socket/device DoS. A cosuper does `mkfifo trap`;
`extract_diff` calls `os.ReadFile("trap")` → blocks forever waiting for a
writer.

Mandatory fix: `lstat` before every read, refuse non-regular non-directory
files, open with `O_NONBLOCK`, cap entries/bytes/depth/time on every
diagnostic operation.

## Strong Consensus (6-7 agents)

### C5. Layer stacking needs compaction/squash (7 agents)

Kernel overlayfs limits: `OVL_MAX_STACK` = 500 lowerdirs, but the mount
option string is capped at one page (~4KB) — long paths blow the limit at
~80-120 commits. Lookup cost grows linearly with stack depth.

Need a squash policy: periodically (every N commits, or when stack
approaches limit) merge several committed layers into a single new lower
via file copy, collapsing the stack. Cap at 32-64 layers, compact at 75%.

### C6. Pin needs max TTL + orphan recovery (7 agents)

Unbounded pins leak forever. Pin must have:
- Hard max TTL (24h recommended, renewable)
- Owner run lineage recorded
- Orphan detection: if owner run is dead/expired, pin is invalid → capsule
  becomes GC-eligible
- Durable state (`stateDir/<id>/pin.json`) so executor restart can
  reconstruct pin registry

### C7. Refcount must cover ALL RPCs, not just exec (6 agents)

`write_file`, `edit_file`, `rename`, `remove` — all mutate the overlay. If
`Destroy` unmounts while a `write_file` is in progress, you get corruption.
Every broker operation that touches the overlay must increment the
refcount on entry and decrement on exit.

Background jobs are a separate gap (3 agents): a backgrounded `long_build &`
returns from its initiating exec RPC immediately — it holds no refcount
once the RPC returns, even though the process keeps running. `Destroy` can
tear down a capsule with live background processes without noticing.

### C8. Uncommitted diff exemption is weaponizable (6 agents)

"Has uncommitted overlay diff exempts from GC" → a cosuper writes 1 byte
and becomes immortal. Need escalation: after TTL (e.g., 4h idle), freeze
capsule, extract diff to durable quarantine artifact, mark abandoned,
destroy runtime. Preserves data-safety without immortal capsules.

### C9. Command log must be IN the tape, not just an interval (6 agents)

Broker logs are volatile (broker can restart, logs lost). An interval
reference into volatile logs is a dangling reference. The tape needs either
the full command log segment embedded, or a content-addressed blob
reference with hash.

### C10. Classifier ruleset digest, not just version (6 agents)

Version strings get fat-fingered. Record a content hash of the actual
ruleset. Versions are immutable references; digests verify the reference
resolves to what was in effect.

## Converged High-Value Findings (4-6 agents)

### C11. HMAC key management is broken (6 agents)

**Single shared secret across all brokers** = a compromised broker extracts
the secret and can forge capabilities for ANY capsule. This defeats the
entire capsule boundary — the exact threat model.

Two fixes proposed:
- **Per-capsule derived keys** via `HKDF(masterSecret, capsuleID)` —
  compromised broker can only forge for its own capsule (codex, claude,
  omp-gpt55, omp-glm52, cursor)
- **Asymmetric signatures (Ed25519)** — executor holds private key, brokers
  hold public key. Compromised broker can verify but cannot forge (devin)

Also: key rotation, key ID in token, core dumps disabled, `getrandom`
generation, never logged.

### C12. Capability revocation is missing (6 agents)

Once minted, valid until expiry. If a cosuper goes rogue mid-session, you
wait for expiry. Need `revoke_capability(handle)` which increments the
capsule's epoch, invalidating outstanding capabilities. The broker rejects
capabilities with epoch < current.

### C13. Handle map must be keyed by (cosuperRunID, handle) (4 agents)

A flat global map keyed by bare `Handle` means two supers independently
choosing `"build-a"` overwrite each other's capability. Key by
`(cosuperRunID, handle)`.

### C14. Broker socket isolation is illusory without privilege separation (4 agents)

Mount namespace isolation defeats non-root processes. A process with
`CAP_SYS_ADMIN` (root) can `nsenter` any mount namespace. If a broker
compromise yields root-in-VM, socket isolation is gone.

The missing specification: **the broker must run as a per-capsule
unprivileged UID, with a seccomp allowlist, landlock restrictions, dropped
Linux capabilities, inside its own user+mount namespace.** Without this,
the "security boundary" claim is unfalsifiable.

## Unique High-Value Findings (1-2 agents)

### U1. [omp-gpt55, FATAL] Lowerdir order is backwards

**Only omp-gpt55 caught this.** The v1 example says:
```
After commit: lowerdir=[erofs_base, committed_upper_1], upperdir=[fresh_empty_tmpfs]
```

Linux overlayfs gives priority to the **leftmost** lower layer. The correct
order is:
```
lowerdir=[committed_upper_N, ..., committed_upper_1, erofs_base]
```

With the wrong order:
```
base/foo      = "old"
upper/foo     = "new"
commit upper
remount lowerdir=base:committed_upper_1
merged/foo    = "old"   // committed edit hidden by base
```

**Committed edits disappear. Deletes are worse — whiteouts from the
committed layer are shadowed incorrectly, so deleted base files reappear.**

This is fatal until fixed. Acceptance tests:
```
commit 1 modifies existing base file → commit 2 reads modified file
commit 1 deletes base file → commit 2 verifies base file stays deleted
commit 1 creates opaque directory → commit 2 verifies lower entries stay hidden
```

### U2. [claude] Alternative to remount — snapshot diff at diff layer

Keep the mount stable for the capsule's whole life. At commit time, record
a manifest (path, size, mtime, hash) of the upperdir. The next `Diff()`
compares current upperdir against the last manifest. No remount, no
freeze-window races, no stack limit, crash-safe (manifest is written
atomically after tape append). You lose "lowerdir is physically immutable"
but you gain a design that works.

### U3. [cursor] Move capability trust anchor outside the guest kernel

The entire capability system is one guest-kernel privilege-escalation bug
away from total collapse, because the thing holding the keys (Executor)
sits right next to the thing under attack (capsules). Move capSecret,
minting, and verification to the Firecracker host process over vsock.

### U4. [omp-glm52] Privilege separation is the load-bearing missing spec

Three "fixes" (#7 socket isolation, #6 security boundary, #5 diagnostic
safety) silently depend on privilege separation that is never specified.
Specify: broker UID (per-capsule, non-zero), capability bounding set
(empty), seccomp allowlist (~40 syscalls), landlock scoped to capsule
dirs, user namespace so broker "root" maps to unprivileged host UID.

### U5. [omp-glm52] Doctrinal tension — capsule drifting from strict-agent default

The v1 capsule has persistent bash sessions, shared capsules, capability-
granted write/exec, full VFS RPC API. The doctrinal default is a strict-
agent effect chamber: network none, copy-in-out, no secrets, tmpfs home.
The design never mentions this tension. Needs an explicit conjecture delta
naming capsule classes (strict-agent verifier vs build vs disposable-
preview) with per-class policy.

### U6. [omp-glm52] Whiteout/opaque dir handling in Diff() is unspecified

When a cosuper deletes a file present in the base, overlayfs writes a
whiteout (char device 0/0, or `trusted.overlay.whiteout` xattr). When a
cosuper deletes an entire directory, it's an opaque dir
(`trusted.overlay.opaque` xattr). Naive `filepath.Walk(upperdir)` misses
whiteouts. `Diff()` must detect whiteouts → emit tombstones, detect opaque
dirs → emit directory-clear operations. Without this, the tape records
"file added" but never "file deleted," and replay reconstructs a wrong
filesystem.

### U7. [opencode] Broker should NOT be in EROFS — bind-mount from host

Baking the broker into EROFS creates a tight coupling: a broker security
fix requires EROFS rebuild + dm-verity re-seal + VM image redeploy + reboot
(multi-hour cycle). Better: bind-mount the broker from a content-addressed
host store, verify content hash at startup. Broker hotfixes take minutes.
The "broker is part of the reproducibility contract" argument still holds —
you record which version was used.

### U8. [cursor] Secrets as env vars are readable via /proc/<pid>/environ

`SpawnSpec.Secrets` as env vars are readable by anything sharing the PID
namespace via `/proc/<pid>/environ`, `env`, `ps eww`, or accidental
inclusion in diagnostic dumps. Not addressed.

### U9. [omp-gpt55] Session ID = cosuperRunID is guessable

`exec` takes `session_id`, typically `cosuperRunID`. That's guessable. A
cosuper that learns another's runID can call `exec(session_id=B)` and
inherit B's cwd/env/background jobs. Session IDs should be broker-minted
random IDs bound to `{capsuleID, cosuperRunID, capabilityTokenID, epoch,
brokerIncarnationID}`.

### U10. [omp-gpt55] Center the design on a CapsuleManifest, not mutable Capsule

Replace the mutable `Capsule{LowerDirs, UpperDir, WorkDir, CommitEpoch}`
with an append-only `CapsuleManifest` that records every transition
(spawn, grant, prepare_commit, append_tape, switch_mount, commit_complete,
broker_restart, destroy, quarantine). Recovery is "read manifest, verify
digests, complete or quarantine" — not "inspect whatever dirs exist and
guess."

## Dissent / Disagreements

### D1. Is the v1 design fundamentally sound or fatally flawed?

- **opencode:** "No fatal flaws. The v1 design is solid. Remaining items
  are production hardening, not architectural flaws."
- **omp-gpt55:** "Do not implement this as written. Fix the overlay order
  and commit atomicity first. Those are structural."
- **claude:** "`CommitLayer` must be redesigned (it fails at the syscall
  level, not just in edge cases)."
- **omp-glm52:** "Two fatal flaws must be resolved before any
  implementation."

**Resolution:** The majority is correct. `CommitLayer` as written fails at
the syscall level (EBUSY, EINVAL on workdir, no crash atomicity). The
lowerdir ordering bug (U1) makes committed edits disappear. These are not
hardening items — they are structural failures.

### D2. Snapshot diff vs layer stacking

- **claude:** Abandon remount entirely. Use snapshot diff (manifest-based).
- **omp-gpt55:** Keep layer stacking but fix ordering, workdir, crash
  atomicity, compaction.
- **cursor:** Mount new overlay on top of existing (Linux permits stacking
  over busy mount) — no unmount needed.
- **omp-gemini35:** Abandon live remounting. Use snapshot diffing.

**Resolution:** This needs a decision. See Open Decisions below.

### D3. Broker in EROFS vs bind-mounted from host

- **v1 design:** Baked into EROFS (content-addressed, dm-verity sealed).
- **opencode:** Bind-mount from host with content hash verification.
  Faster hotfix cycle.

**Resolution:** Needs a decision. The EROFS approach is more
tamper-resistant but operationally rigid. The bind-mount approach is
flexible but requires careful content hash verification.

## Low-Confidence / Unverified Claims

- **cursor's vsock trust anchor (U3):** Sound in principle but requires
  significant architectural change. Not verified against Firecracker's
  vsock capabilities for this use case.
- **claude's "mount new overlay on top of busy mount":** Linux permits
  this, but the behavior of open fds spanning the mount boundary after
  the new mount is in place needs verification.
- **omp-glm52's doctrinal tension (U5):** Correct that the design doesn't
  address the strict-agent default, but whether this is a drift or an
  intentional evolution is a doctrine-level decision for the user.

## Recommendation

**Do not implement v1 as written.** Resolve these before any code:

### Must-fix before implementation (structural failures):

1. **Fix lowerdir ordering** (U1) — newest committed first, EROFS base
   last. This is a one-line fix but it's fatal.

2. **Redesign CommitLayer** (C2) — choose one:
   - **Option A (snapshot diff):** Keep mount stable, record manifest,
     compare upperdir vs manifest. No remount. (claude, omp-gemini35)
   - **Option B (stack-on-top):** Mount new overlay over existing, no
     unmount. Old fds keep old view, new lookups use new overlay. (cursor)
   - **Option C (fix remount):** Quiesce ALL sessions (kill background
     jobs, close fds, chdir out), fresh workdir per epoch, durable WAL.
     (omp-gpt55)

3. **Fix memory admission** (C3) — `memory.max` is the total budget;
   tmpfs is a sub-budget. Fix the tier table so `DiskMax <= MemoryMax -
   headroom`.

4. **Add capability verb set** (C1) — replace `AccessMode` with explicit
   verb allowlist per capability.

5. **Fix HMAC key management** (C11) — per-capsule derived keys or
   Ed25519 asymmetric. No shared secret.

6. **Add crash recovery** (C2, U10) — durable commit manifest/WAL with
   states and recovery logic. Every transition recoverable.

### Must-fix before production (security holes):

7. **Privilege separation spec** (U4, C14) — broker as unprivileged UID
   in user+mount namespace, seccomp, landlock, no caps.

8. **Diagnostic safety** (C4) — lstat before read, refuse non-regular,
   O_NONBLOCK, caps on entries/bytes/depth/time.

9. **Capability revocation** (C12) — `revoke_capability`, epoch bump,
   broker rejects stale epoch.

10. **Refcount covers ALL RPCs + background jobs** (C7) — not just exec.

11. **Layer compaction** (C5) — cap at 32-64 layers, squash at 75%.

12. **Pin TTL + orphan recovery** (C6) — max 24h, owner lineage,
    durable state, orphan sweep.

13. **Uncommitted diff quarantine** (C8) — TTL-based escalation, not
    immortality.

14. **Command log in tape** (C9) — content or content-addressed blob.

15. **Classifier ruleset digest** (C10) — not just version.

16. **Whiteout/opaque dir handling in Diff()** (U6) — tombstones and
    directory-clear operations.

17. **Session IDs are broker-minted** (U9) — not cosuperRunID.

### Open decisions — RESOLVED in v2

A second consensus round (6 agents: cursor, devin, opencode, omp-gpt55,
omp-gemini35; excluding claude and glm52 per user request) was run to
resolve the 4 open decisions. Results:

- **D1 (diff strategy):** Option A (snapshot diff) — 6/6 unanimous.
  Manifest-based, no remount. Eliminates EBUSY, workdir mismatch, lowerdir
  ordering, crash atomicity, stack limits.

- **D2 (broker placement):** Option B (bind-mounted from host) — 6/6
  unanimous. Content-hash verified at spawn. Minutes-scale hotfixes.

- **D3 (trust anchor):** Resolved as Ed25519 asymmetric (not in original
  options). Executor (host) holds private key, broker (guest) holds public
  key. Zero per-RPC latency. Trust anchor outside guest kernel. Residual
  risk: public key replacement in broker memory (mitigated by binary hash
  verification + privilege separation). This emerged from the discussion
  as a fourth option that combines the best of HKDF (zero latency) and
  vsock (trust anchor outside guest).

- **D4 (capsule classes):** Resolved as single capsule type with role-based
  capabilities (user decision, overriding panel majority). All capsules
  have all permissions. Agent roles (super/cosuper/researcher) determine
  verb sets. The capsule is an execution context, not a permission boundary.
  Explicit doctrine delta documents departure from strict-agent default.

All decisions incorporated into v2 design docs.

## Raw Outputs

- Manifest: `/tmp/capsule-v1-consensus/manifest.tsv`
- Per-agent: `/tmp/capsule-v1-consensus/<agent>.out`
- Earlier codex run: `/tmp/codex-v1-review.txt`
- Decisions manifest: `/tmp/capsule-v1-decisions/manifest.tsv`
- Decisions per-agent: `/tmp/capsule-v1-decisions/<agent>.out`
- Review prompt: `/tmp/capsule-review-v1-prompt.md`
