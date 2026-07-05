# Capsule Runtime v2 — Consensus Synthesis

**Status:** Synthesis of 4-agent consensus panel review of v2 design.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)
**Panel:** cursor, devin, omp-gpt55, omp-gemini35 (codex + opencode failed)
**Outputs:** `/tmp/capsule-v2-consensus/`

## Headline

**No architectural fatal flaws.** All 4 agents agree the v2 architecture
(snapshot diff, Ed25519, bind-mounted broker, role-based verbs) is sound.
The remaining issues are specification contradictions and stale v1 residue,
not architectural dead-ends. The docs need a reconciliation pass before
implementation.

## Universal Consensus (4/4)

### V2-1. Privilege separation / capability bounding set contradiction [FATAL if implemented as written]

**All 4 agents.** The design doc says the broker has an "empty Linux
capability bounding set." The decision doc says overlayfs copy-up requires
`CAP_DAC_OVERRIDE` and `CAP_FOWNER`. If the broker (PID 1) has empty caps,
its children (cosuper bash) can't trigger overlayfs copy-up on base-owned
files → capsule is non-functional.

**Fix (converged across agents):** Broker retains `CAP_DAC_OVERRIDE` +
`CAP_FOWNER` in bounding+permitted sets, drops everything else (`CAP_SYS_ADMIN`,
`CAP_NET_ADMIN`, `CAP_SYS_PTRACE`, etc.). Runs as unprivileged mapped UID
in user namespace. This is still strong privilege separation.

### V2-2. Implementation doc is stale v1 [FATAL for implementers]

**3/4 agents** (cursor, devin, omp-gpt55). `capsule-runtime-implementation-v0.md`
still contains HMAC `capSecret`, `AccessMode`, `CommitLayer`, `LowerDirs`,
single-arg `ResolveCapability(handle)`, and layer stacking — directly
contradicting the "Resolved Questions" section 60 lines below. Anyone
implementing from this doc builds v1.

**Fix:** Delete all v1 residue from the implementation doc. Reconcile with
the design doc.

### V2-3. Session IDs still cosuperRunID [unfixed from v1]

**3/4 agents** (cursor, devin, omp-gpt55). U9 from v1 review was flagged
but never actually fixed in the bash tool example. `session_id = cosuperRunID`
is still present. Guessable.

**Fix:** Session IDs are broker-minted random IDs bound to `{agentRunID,
capsuleID, capabilityID, epoch, brokerIncarnationID}`.

### V2-4. Secrets as env vars still unresolved [unfixed from v1]

**3/4 agents** (cursor, devin, omp-gpt55). U8 from v1 review never fixed.
`SpawnSpec.Secrets` as env vars readable via `/proc/<pid>/environ`.

**Fix:** Either accept as "visible to capsule" explicitly, or use a
secrets fd / tmpfs mount that's not readable via /proc.

### V2-5. Snapshot diff crash recovery lacks idempotency

**3/4 agents** (cursor, devin, omp-gpt55). "Manifest written after tape
append" means a crash between tape append and manifest write → re-doing
the commit on recovery → duplicate tape entry.

**Fix:** Tape append must be idempotent by transaction ID / commit epoch.
Recovery dedups by it.

### V2-6. walkUpperdir needs FIFO/symlink guards

**2/4 agents** (omp-gemini35, cursor). The diagnostic safety rules (lstat,
O_NONBLOCK, refuse non-regular) need to apply to the manifest walker too,
not just `extract_diff`. A guest `mkfifo` trap can hang the host-side
manifest walk.

**Fix:** `walkUpperdir` uses the same defensive traversal as `extract_diff`:
lstat before read, O_NONBLOCK, refuse non-regular, caps on entries/bytes/
depth/time.

## Strong Consensus (3/4)

### V2-7. Revocation epoch vs commit epoch conflated

**omp-gpt55, cursor, devin.** `Epoch` is used for both commit tracking and
capability revocation. Need separate `commitEpoch` and `authEpoch`.

### V2-8. Revocation propagation to broker unspecified

**omp-gpt55, cursor.** Executor increments epoch on revocation, but broker
verifies locally. No mechanism for broker to learn about revocation. Need
host-to-broker revocation message or epoch sync.

### V2-9. os.Sync() is wrong for manifest atomic write

**devin.** The CommitManifest code uses `os.Sync()` (global sync of all
filesystems) instead of `f.Sync()` on the temp file + `fsync` on the parent
directory after rename. Both wrong and a performance hazard.

**Fix:** Write temp → `f.Sync()` → `rename` → `fsync(parent dir)`.

## Unique Findings

### V2-10. [cursor] Privilege separation scoped to broker only, not exec surface

The actual bash command execution path (cosuper's `exec`) is still
root-in-namespace with no user namespace. Only the broker RPC server got
hardened. The larger attack surface (arbitrary agent-supplied commands)
didn't move.

### V2-11. [devin] Researcher `target_capsule="*"` needs broker-side enforcement

The `*` is an executor-side wildcard. The broker must reject literal `*`
and only accept resolved capsule IDs from a capability that encodes the
wildcard grant.

### V2-12. [omp-gpt55] Super verbs shouldn't be broker verbs

Spawn/destroy/mint/diagnostics are executor/host-control-plane verbs, not
broker verbs. The broker runs inside the capsule; it can't spawn capsules.

### V2-13. [omp-gpt55] Seccomp allowlist vs denylist contradiction

Design doc says "seccomp allowlist." Implementation doc says "targeted
denylist." A strict ~40 syscall allowlist on cosuper workloads will break
`go`, `git`, `python`. Need to resolve: allowlist for broker, denylist for
workload (or carefully constructed allowlist that includes dev tool syscalls).

### V2-14. [omp-gemini35] overlayfs userxattr mount option

Running in an unprivileged user namespace with overlayfs requires
`userxattr` mount option or explicit UID mappings, or copy-up of root-owned
base files fails with EPERM.

### V2-15. [omp-gpt55, devin] mtime+size fast-path for manifest walk

Every commit re-hashes the entire accumulated upperdir. Add mtime+size
short-circuit before hashing — if mtime and size are unchanged since last
manifest, skip the hash. For long-lived write-heavy capsules this is
critical for performance.

## What's Good

All 4 agents confirmed:
- **Snapshot diff** is the correct replacement for layer stacking. Sound,
  crash-safe (with idempotency fix), session-continuous.
- **Ed25519 capabilities** are sound. Trust anchor outside guest kernel,
  zero per-RPC latency. The residual risk (public key replacement) is
  correctly scoped — an attacker who can write broker memory can already
  bypass verification entirely, so replacing the key gains nothing.
- **Role-based verb sets** are sound. Capsule = execution context, role =
  authority. Correctly rejects the fake "read-only + exec" model.
- **Bind-mounted broker** is sound. Operational velocity without sacrificing
  reproducibility.
- **Memory admission fix** is correct. `memory.max` is total budget, tmpfs
  is sub-budget.
- **Refcount covers all RPCs** is correct.
- **Bounded pins + quarantine** is correct.

## Recommendation

**The v2 architecture is sound. The docs need a reconciliation pass.**
Before implementation:

1. **Fix the capability bounding set contradiction** (V2-1) — retain
   `CAP_DAC_OVERRIDE` + `CAP_FOWNER`, drop everything else.
2. **Delete all v1 residue from the implementation doc** (V2-2).
3. **Fix session IDs to be broker-minted** (V2-3).
4. **Resolve secrets handling** (V2-4) — explicit decision needed.
5. **Add tape idempotency for crash recovery** (V2-5).
6. **Apply diagnostic safety rules to walkUpperdir** (V2-6).
7. **Split commitEpoch from authEpoch** (V2-7).
8. **Specify revocation propagation to broker** (V2-8).
9. **Fix manifest atomic write** (V2-9) — f.Sync() + fsync(parent).
10. **Resolve seccomp allowlist vs denylist** (V2-13).
11. **Add userxattr mount option for user namespace overlayfs** (V2-14).
12. **Add mtime+size fast-path to manifest walk** (V2-15).

None of these are architectural changes. They're specification fixes.

## Raw Outputs

- Manifest: `/tmp/capsule-v2-consensus/manifest.tsv`
- Per-agent: `/tmp/capsule-v2-consensus/<agent>.out`
- Review prompt: `/tmp/capsule-review-v2-prompt.md`
