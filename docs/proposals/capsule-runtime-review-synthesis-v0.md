# Refined Model Review Synthesis — v0

**Status:** Synthesis of 5-review-thread feedback on the refined capsule model.
**Date:** 2026-07-05
**Mutation class:** green (design documentation, no runtime change)

## Review Threads

| Thread | Perspective | Status |
|--------|-------------|--------|
| Subagent 1 (4f4594e0) | Architecture & security | Complete |
| Subagent 2 (d65fd90d) | Integration & production | Complete |
| Codex (o3) | Architecture & security | Complete |
| Cursor (gpt-5.5-high) | Isolation & authority | Complete |
| Cursor (sonnet-5-thinking-high) | Correctness & production | Complete |

OMP gpt-5.5, gpt-5.4, and gemini-3.5-flash either produced unusable output
(web search mode, no output file, or file-listing mode) or crashed. The 5
threads above all produced substantive reviews.

## Convergence: The #1 Change Across All Reviews

**All 5 reviews independently identified the same core weakness: raw
`capsule_id` as a tool argument is identity, not authority.**

The fix, stated by codex and cursor-gpt55 most directly:

> Replace `capsule_id` as a tool argument with run-scoped capability handles.
> A capability should bind `{cosuperRunID, capsuleID, access mode, epoch,
> expiry, audit context}`. The executor mints it; the broker verifies it
> before doing anything.

This is not cryptographic capsule ID obfuscation. It's capability-based
access control: the executor issues unforgeable tokens that the broker
verifies on every request. The cosuper never sees the raw capsule ID — it
gets an opaque handle (`workspace="build-a"`) that the executor maps to the
real capsule.

This aligns the architecture with its actual needs: the VM remains the host
security boundary, while capsule access becomes a real integrity boundary
for work products and commits.

## NEW Fatal Flaw (Found Only by Sonnet-5)

### The Overlayfs Incremental Diff Gap

`Capsule.Diff()` walks `upperdir` vs the EROFS `lowerdir` via
`containerd/continuity/fs.DiffDirChanges`. That comparison is always
upper-vs-original-base — it has no concept of "since the last commit."

For the model's own flagship scenario (cosuper A commits, capsule persists,
cosuper B continues later, super calls `commit_transaction` again): the
second `Diff()` call will re-report cosuper A's already-committed changes
PLUS cosuper B's new changes, as one flat diff against the pristine base.

**Every subsequent commit on a reused capsule re-diffs the entire history
of that capsule, not just the delta.**

This means either:
- The tape gets duplicate/overlapping `FileChange` entries every time a
  long-lived capsule commits more than once (breaks the append-only,
  non-duplicating ledger model), or
- Someone has silently assumed capsules are always destroyed after exactly
  one commit — which contradicts the "capsule persists, cosuper B continues
  the work" narrative that justifies decoupling lifecycles.

**The fix (two options):**
- **(a) Layer stacking:** After each `commit_transaction`, fold the current
  upperdir into a new lowerdir layer and mount a fresh empty tmpfs upper on
  top. Overlayfs supports stacked `lowerdir=upper_N:...:upper_1:erofs_base`.
  The next diff is naturally incremental again.
- **(b) Snapshot diff:** Snapshot/hash the upperdir tree at each commit and
  diff the current upperdir against that snapshot instead of against the
  EROFS base.

**This blocks "capsule reuse across commits" from shipping until designed.**
If not resolved, capsules must be single-commit-then-destroy for v1, with
long-lived workspaces deferred.

## Consensus on Required Changes (All 5 Reviews)

### P0: Must Fix Before Implementation

**1. Capability-based capsule access (not ID-based)**
Replace raw `capsule_id` tool arguments with executor-minted, unforgeable
capability handles scoped to `{cosuperRunID, capsuleID, access mode, epoch,
expiry}`. The broker verifies capabilities on every request. Broker sockets
must not be visible across capsule mount namespaces.

**2. Super needs host-side diagnostic tools**
The super cannot depend on the broken capsule being able to execute commands.
Add out-of-band tools that bypass the capsule layer entirely:
- `inspect_capsule_raw(id)` — read cgroup files, check broker PID, walk
  upperdir directly from Go (no broker needed)
- `extract_diff(id)` — capture overlay diff even if broker is dead
- `list_capsules()` — all capsules with resource usage (for cross-capsule
  contention diagnosis)
- `restart_broker(id)` — restart a crashed broker without destroying capsule
- `force_destroy(id)` — MNT_DETACH + force-kill for stuck capsules

These tools walk capsule-controlled paths from host-privileged code, so they
need `O_NOFOLLOW`/`openat2(RESOLVE_BENEATH)` to prevent symlink-based escape.

**3. Overlayfs incremental diff gap (see above)**
Either implement layer stacking or snapshot diff, or scope capsule reuse to
single-commit-then-destroy until solved.

**4. Broker socket isolation**
Broker sockets must not be reachable from other capsules. Each capsule's
broker socket lives in its own mount namespace. No abstract Unix sockets.
Socket permissions 0600, owned by executor.

### P1: Must Design Before Shipping

**5. Broker protocol: typed RPCs, not exec wrappers**
Sonnet-5 strongly disagrees with the earlier synthesis's "Option B" (exec as
primary, file tools as wrappers via heredocs). That reintroduces
delimiter-collision bugs, no binary safety, and non-atomic writes — exactly
the fragility the broker was built to eliminate. Use typed RPCs:

- `exec` — run command (with session_id, cwd, env, stdin, timeout, pty, kill)
- `read_file` — read file (with streaming for large files)
- `write_file` — write file (atomic, binary-safe)
- `edit_file` — edit file (with expected-hash precondition)
- `list_dir` — list directory
- `stat` / `lstat` — file metadata
- `readlink` — read symlink
- `mkdir` / `mkdir_all` — create directories
- `remove` / `remove_all` — delete files/directories
- `rename` — move/rename
- `chmod` — change permissions
- `symlink` — create symlink (with path traversal policy)
- `truncate` — truncate file
- `file_hash` — compute file hash
- `kill_session` — kill running exec session

Symlink handling needs a written policy: hard ban on symlinks crossing
intended roots, device nodes, privileged xattrs.

**6. Session management: explicit session_id**
The broker must track per-cosuperRunID session state (cwd, env) explicitly.
Either the broker's `exec` verb takes a `session_id` and maintains
per-session shell processes internally, or the tool layer threads cwd/env
through every call. The current design contradicts itself on this point.

**7. Capsule lifecycle: ephemeral by default, leased for long-lived**
- Default: capsules are ephemeral, tied to the super's run record
- On super run completion: all capsules destroyed (after diff extraction)
- Long-lived capsules are opt-in via `pin_capsule(id, timeout)`
- "Has uncommitted overlay diff" exempts capsule from GC (never destroy
  state that hasn't been committed)
- TTL, idle eviction, max upperdir size, max inode count
- Commit-before-evict behavior

**8. Transaction commit invariants**
Before `Tape.Append`, capture:
- Frozen capsule state
- Complete diff
- Command log interval (from broker logs)
- Contributing cosuper run IDs
- Classifier version
- Lower/base digest
- Broker version
- Capsule spec and resource tier
- Whether any unauthorized access attempts occurred
- Whether broker restarted during the interval

**9. Reframe capsule as defense-in-depth**
The capsule IS a security boundary — a defense-in-depth layer, not a
"convenience." The seccomp/landlock/capabilities are load-bearing security
controls. The microVM is primary, the capsule is secondary. Both are
security boundaries with different threat models. The framing matters
because it affects how carefully engineers write the broker's input
validation, path traversal checks, and symlink handling.

### P2: Production Hardening

**10. VM memory admission control**
Tiered presets commit cgroup memory + tmpfs disk (which is RAM). Four
"large" capsules = 8GB cgroup + 16GB tmpfs = 24GB against what might be an
8GB VM. `spawn_capsule` must reject if total committed would overcommit.
Also: tmpfs must be mounted from a process already in the capsule's cgroup
for correct memcg accounting.

**11. TOCTOU race: destroy vs in-flight exec**
`GetCapsule` → `Exec` is TOCTOU. Need `CapsuleStateDestroying` that rejects
new exec calls, and in-flight exec calls hold a refcount that `Destroy`
waits on with a timeout.

**12. Broker crash recovery with at-most-once semantics**
Restarting a broker and blindly retrying non-idempotent commands (rm -rf,
git commit) is dangerous. Need at-most-once semantics or explicit "unknown
outcome" reporting to the cosuper.

**13. Cheap hardenings for capsule IDs**
- Use random UUIDs, not sequential `cap-1`, `cap-2`
- Uniform error responses: don't distinguish "not found" from "not
  authorized" (prevents enumeration oracle)

**14. Quiesce/Destroy fallback for EBUSY**
If a zombie process holds the merged mountpoint open, `Destroy` can hang.
Need documented fallback: `MNT_DETACH`/lazy unmount + force-kill remaining
PIDs in the cgroup.

**15. What happens to bound cosupers when capsule OOMs**
The failure-mode table says "mark failed, notify super" but doesn't say what
bound cosupers see. Do their next `bash` calls get an error? Are they
killed? Is partial work salvageable? This needs to be specified.

## Dismissed Concerns (User Already Decided)

**Attribution in shared capsules:** User explicitly chose N cosupers per M
capsules. The MutationTransaction records the capsule's final state (like a
git squash merge). Capsule-level attribution is the primary unit. Broker
logs provide cosuper-level attribution. The transaction must record
contributing cosuper run IDs (per codex and sonnet-5), but single-author
attribution is not required for shared capsules.

**MicroVM as sufficient boundary for adversarial cosupers:** User explicitly
accepted: "maybe a determined cosuper could escape containment but thats
what we have microvms for." The capability-based access control (P0 #1)
addresses the practical concern of cross-capsule interference in normal
operation. For truly adversarial cosupers, separate microVMs are needed
(per cursor-gpt55).

## Summary of Required Changes

| Change | Priority | Source |
|--------|----------|--------|
| Capability-based capsule access | P0 | codex, cursor-gpt55, cursor-sonnet5 |
| Super host-side diagnostic tools | P0 | all 5 |
| Overlayfs incremental diff gap | P0 | cursor-sonnet5 |
| Broker socket isolation | P0 | codex, cursor-gpt55 |
| Typed RPC broker protocol (not exec wrappers) | P1 | cursor-sonnet5 |
| Explicit session_id in broker | P1 | cursor-sonnet5 |
| Capsule lifecycle: ephemeral + pin | P1 | all 5 |
| Transaction commit invariants | P1 | codex, cursor-gpt55, cursor-sonnet5 |
| Reframe as defense-in-depth | P1 | all 5 |
| VM memory admission control | P2 | cursor-sonnet5 |
| TOCTOU race: destroy vs exec | P2 | cursor-sonnet5 |
| Broker crash: at-most-once semantics | P2 | cursor-sonnet5 |
| Random UUIDs + uniform errors | P2 | cursor-sonnet5 |
| Destroy fallback for EBUSY | P2 | cursor-sonnet5 |
| Bound cosuper behavior on OOM | P2 | cursor-sonnet5 |

## What Stays Unchanged

- Two-layer boundary model (microVM + capsule) — sound, reframe as
  defense-in-depth
- Decoupled cosuper/capsule model — correct abstraction
- N cosupers per M capsules — user's explicit choice
- Super without bash — but gains host diagnostic tools
- All prior consensus decisions (Q1-Q8 from previous round)
- Library stack (all pure Go)
