# An engineering capsule takes 17-18 s from admission to ready: most of it is one git process per source file, run three times (2026-10-22)

Investigation lane FP4 (lane/fp4-capsule-coldstart, base origin/main
273d139c9). Problem documentation first: this commit has no code. The timing
logs come in the next commit, and no optimization is built. Each finding is
marked **confirmed** (by code read and a local measurement, or by arithmetic
on the logged timestamps) or **hypothesis**.

## Evidence

- FP4 drill on deploy Q
  (docs/evidence/fp4-admission-queue-deployed-proof-2026-10-22.md, step 4):
  - A terminal at 14:01:29.732.
  - B's admission nudge waited 2 ms.
  - B's capsule broker ready and B bound at 14:01:46.890: **17.16 s**.
- SEC-06 run on deploy S, test computer 71afbf14 (guest excerpt
  ~/runs/coldstart-71afbf14-161230.log). The computer booted at 16:09:39.

  ```
  16:12:43 actorruntime: engineering admission wake latency mailbox=management:c7f51f0f-… update=7543d433-… waited=13.479s content={"assignment_id":"assignment-da297e3be55ce27dfdf377a0","attempt":1}
  16:13:01 capsule-broker: session worker ready=true
  16:13:01 runtime: actor wake latency kind=initial_dispatch target=engineering:assignment-da297e3be55ce27dfdf377a0 waited=31.224s dispatch=1ms
  16:13:34 runtime: actor wake latency kind=assigned_engineering_fate_deadline target=management:c7f51f0f-… waited=1m4.1s dispatch=1ms
  16:13:34 (and 16:13:35) three more of these, waited 1m4.564s, 1m5.131s, 1m5.327s
  16:13:34-35 seven "dispatcher: handle c7f51f0f-…" lines, then "dispatcher: poison c7f51f0f-…"
  ```

  There are **18 s** between the admission wake (16:12:43) and the broker
  line (16:13:01), with no log line in between.

## The path from admission to "session worker ready" (code read, origin/main 273d139c9)

`capsule-broker: session worker ready=true` is a constant. The broker prints
it once it has hardened itself (cmd/capsule-broker/main.go:306), and no
session worker is started there. So the 18 s all comes before the broker
process runs.

The steps, in order. "Seconds?" says whether a step can take seconds and why.

| # | Step (code) | What it does | Seconds? |
|---|---|---|---|
| 0 | `planEngineeringCast` (internal/agentcore/engineering_assignment_runtime.go:174, the preflight at :227), **at cast time, inside management's activation, before the open commits** | `PreflightSourceSnapshot("")`. See 0a-0c. | **Yes, ~7 s (0c).** |
| 0a | `immutableGitCommitIdentity` (internal/capsule/source_snapshot.go) | `git diff --quiet`, `git diff --cached --quiet`, `git rev-parse` | ~10 ms (measured) |
| 0b | `immutableCommitInventory` | `git ls-tree -r` of the commit | ~6 ms |
| 0c | `canonicalImmutableCommitDigest` | One `git cat-file blob <oid>` process **per tracked file**, hashing each blob | **Yes: N fork/execs of git.** |
| 1 | `HandleDelegatedAssignmentSpawnDeadline` (continuation_schedule.go:177) | Delivered on the casting agent's mailbox (management) for the open-time wake, or on `engineering-admission` for retries and nudges. Takes `engineeringAssignmentOpenMu` and re-reads the row. | Waits for the mailbox (see "other waits") |
| 2 | `resumeDelegatedCastAssignment` | `expireOverdueEngineeringRequests`, `admitEngineeringAssignment` (store reads) | ms |
| 3 | `PreflightSourceSnapshot(capsule-source-git:<commit>:<digest>)` (executor.go:310) | **`canonicalImmutableCommitDigest` again**, to recheck the pinned digest | **Yes, ~7 s.** |
| 4 | `adoptionInputDir` | Only on a release-adoption trajectory: re-verifies and copies the adoption patch | Small (one patch) |
| 5 | `offerCarriedPatchForAssignment` → `DeriveCarriedPatch` + `CheckCarriedPatchApplies` (internal/capsule/carried_patch_linux.go) | Only when an earlier round on the trajectory failed with a candidate: `withSourceRefTree` materializes the git base with **`copyImmutableCommitTree`, once for derive and once for the apply check** | **Yes, ~14 s each (step 7's cost) when it runs.** Not in the 18 s, which had no prior round. |
| 6 | `reclaimTerminalAssignmentCapsules` | Lists assignments; may `ForceDestroy` a terminal capsule (unmount, cgroup delete) | Usually ms; a teardown waits up to `capsuleProcessWaitTimeout` |
| 7 | `Executor.Spawn` (executor.go:357), **holding `e.mu` (write) for the whole spawn** | See 7a-7l | |
| 7a | `planCapsuleMemory`, `admitCapsuleMemoryLocked` | Arithmetic | ms |
| 7b | `digestRegularFile(brokerPath)` | sha256 of the broker binary (33.8 MB local build) | ~0.1-0.2 s |
| 7c | Stat lower/source, `stateFreeBytes`, `MkdirAll` state dir | | ms |
| 7d | `copyImmutableCommitTree` → inventory | `git ls-tree -r` | ms |
| 7e | → blob copy | **One `git cat-file blob` per tracked file**, written to `source-lower/workspace/platform` | **Yes, ~7 s.** |
| 7f | → `digestCanonicalSubjectTree(target)` | Walks and hashes the 53 MiB copy in-process | ~0.2 s |
| 7g | → `canonicalImmutableCommitDigest` | **The per-file `git cat-file` pass a third time**, to check the copy | **Yes, ~7 s.** |
| 7h | → `makeSubjectTreeReadOnly`, then `requireFrozenComputerSurfaceSource` and `makeSubjectTreeReadOnly` once more in Spawn | chmod walk, twice | ~20 ms each |
| 7i | Verifier bundle, adoption and carried copies (`copyCanonicalSubjectTree`) | Only when present; small trees | ms |
| 7j | `prepareCapsuleReleaseDir`, `MountOverlayFS` (namespace.go:213), `prepareCapsuleRoot`, `installBrokerMount`, the read-only bind mounts | mount(2) and mkdir calls; `waitForCommandStart` runs each mount once under a 10 s watchdog (`capsuleBrokerStartTimeout`, internal/capsule/start_command.go), with no retry | ms (not measured locally: needs root) |
| 7k | `createCapsuleCgroup`, `ensureCapsuleAggregateLocked` | cgroupfs writes | ms (not measured locally) |
| 7l | `startEgressLocked` (ecosystem_proxy) | Listens on a unix socket in the merged root and starts the proxy goroutine. **No network I/O at spawn.** | ms |
| 7m | `startBrokerLocked` | One fork/exec of `/run/capsule/broker` into new namespaces. The broker mounts /proc, starts the egress forwarder, applies Landlock, drops caps and loads seccomp. The executor polls readiness (connect plus a signed `Stat`) **every 25 ms, for up to 10 s**. | ms to ~100 ms in the normal case; 10 s only on failure |
| 8 | Mint the capability, attest the policy, `BindEngineeringAssignment` (store commit), arm the deadline, `rt.activate` | Store writes | ms to 100s of ms |

The session worker (`exec-go-session`) starts later, lazily, on the first
go_eval (cmd/capsule-broker/session_worker.go). It is not in the 18 s.

## Measurement (local, this VM, 8 vCPU, ext4; repo at 273d139c9: 3416 tracked files, 52.7 MiB)

A throwaway test (deleted, not committed) timed each source function
against this repo:

```
commit identity (2 git diff + rev-parse):            9.7 ms
canonicalImmutableCommitDigest (preflight, 3416 git procs): 7.15 s
ls-tree inventory (3416 files):                       6.4 ms
copyImmutableCommitTree total (6832 git procs):      14.24 s
  of which digestCanonicalSubjectTree of the copy:    0.21 s
makeSubjectTreeReadOnly:                              19 ms
```

- One `git cat-file blob` costs about 2.1 ms: 300 in a shell loop took
  0.69 s.
- The whole tree through **one** `git cat-file --batch` took **0.16 s**.
- `git archive HEAD | tar -x` of the whole tree took **0.21 s**.

On the path from admission to ready (steps 3 + 7), the per-file processes
cost about 7.15 + 14.24 = 21.4 s here. Staging measured 17-18 s. That is the
same order, on a guest whose fork/exec costs a little less, or whose platform
tree differs slightly in file count. Not confirmed per phase on staging:
that is what the next commit's log lines are for.

**Confirmed (code + local measurement): about 95% of the cold start is three
passes of one git process per tracked file.**

- Pass 1, `resume_preflight`: re-proves the digest the cast already proved
  at step 0.
- Pass 2, `source_blobs`: the copy.
- Pass 3, `source_digest_commit`: re-proves the digest a third time, to
  check a copy that was also hashed in-process in 0.2 s.

The cast-time pass (step 0) is a fourth, about 7 s, and runs inside
management's activation.

## The other waits

### Admission wake waited 13.479 s after a fresh boot

**Confirmed (code + timestamps).** The boot is incidental: the computer had
been up 2.8 min.

- The wait comes from the residual already named in
  docs/problems/fp4-nudge-waits-behind-management-activation-2026-10-22.md
  ("Disposition"). The **open-time spawn wake is minted by the store in the
  open commit** (internal/store/lifecycle.go, `appendWake(":spawn",
  assignment.Binding.ParentAgentID, …)`) and goes to management's mailbox.
- The dispatcher runs one activation per mailbox. The wake therefore waits
  until the activation that cast it has finished its tool loop.
- Here that was the 16:12:30 to 16:12:43 loop, ending with "round 1 … stays
  open" at 16:12:43.
- 16:12:43 minus 13.479 s is 16:12:29.5, the open commit.

**The cast preflight is not in those 13.5 s** (corrected in round 2, after
review bunch 77, DeepSeek F1; round 1 claimed it was, as a hypothesis).

- `waited` is `time.Since(due)` (internal/actorruntime/handler.go), and
  `due` is the actor update's `CreatedAt`, because the `:spawn` wake has no
  `NotBefore`. That `CreatedAt` is set when the wake is appended to the actor
  log (internal/actorruntime/adapter.go), after the open commits.
- The cast preflight (step 0) runs in `planEngineeringCast`, before
  `openDelegatedCastAssignment`. It therefore delays the open, and so the
  whole start, but it lies outside the logged wait.
- Likewise, the tool-loop iterations at 16:12:30 and 16:12:38 come after
  the open at about 16:12:29.5, so their 8 s gap is not the cast preflight.
- The 13.5 s is the tail of the casting activation after the open. The
  preflight is a separate pre-open cost of about 7 s, which the
  `cast_preflight` line (next commit) will show.
- Repair in flight elsewhere, not duplicated here: lane/fp4-actor-retry-backoff
  (docs/problems/fp4-actor-retry-no-backoff-wake-latency-spawn-mailbox-2026-10-22.md,
  disposition at 4f003dccb) moves the open-time `:spawn` wake to the
  `engineering-admission` mailbox (its F3).

### initial_dispatch "waited 31.224 s"

**Confirmed (code + arithmetic): this is a measurement artefact, not a
queue.**

- The initial_dispatch wake is minted at the first bind commit
  (lifecycle.go, `appendWake(":dispatch", …)` at `LifecycleVersion == 2`).
- Its `CreatedAt` comes from `actorWakeOutbox(source…)`, which uses
  `source.CreatedAt`: the **assignment object's** CreatedAt.
- Every later version keeps the open time (`assignmentObj.CreatedAt` in each
  `lifecycleObject` rewrite, internal/store/engineering_assignments.go).
- 16:13:01 minus 31.224 s is 16:12:29.8, the open again.
- 31.2 s matches 13.5 s (the admission wait) plus 18 s (the cold start)
  within about 0.3 s. That is the error the second-granular log
  timestamps allow: it is about 0.25 s below the sum, not above it.
- `dispatch=1ms`: the dispatch itself went out within a millisecond of the
  bind.

### assigned_engineering_fate_deadline wakes "waited ~1 m 4 s"

**Confirmed (same arithmetic): also the assignment's age.**

- 16:13:34 minus 1m4.1s is 16:12:29.9.
- These `:fate` wakes are minted by fate commits (freeze or revoke
  requested, revoked; lifecycle.go G9) when engineering ended at about
  16:13:34. They went out at once (`dispatch=1ms`).
- **Hypothesis:** the four lines are four fate commits of the one assignment.

The `actor wake latency` line therefore cannot tell a slow queue from an old
assignment, for any wake whose source object outlives its first version.
lane/fp4-actor-retry-backoff (its F2) now measures that line from the
outbox row's mint time (`MintedAt`).

### The poison at 16:13:35

**Confirmed mechanism (code):**

- After a handler error the dispatcher logs `dispatcher: handle
  <agentID>/<updateID>: <err>` and ends the activation
  (internal/actor/dispatcher.go:516, `break`).
- The deferred `d.Notify()` then re-reads the projection at once
  (dispatchPending's goroutine), so the update is redelivered **with no
  backoff**.
- `RecordAttempt` counts each delivery, and the kernel's `MaxAttempts` is 8
  (internal/actorruntime/adapter.go:159). The eighth delivery is poisoned
  (`attempts >= MaxAttempts`): `delivery_failed` goes to the error sink, and
  the update is incorporated (dropped).
- Seven "handle" lines and one "poison" line inside about one second is
  exactly that: a transient error burns the whole 8-attempt budget in about
  1 s.
- Only `ErrDeferUnprocessed` gets a backoff (`deferralBackoff`).

**Not confirmed: which update was poisoned, and its error.**

- The excerpt cuts each line after the first segment of the scoped agent id
  (`c7f51f0f-…`, the owner id that also names `management:<owner>`). The
  update id and error text are not in it.
- **Hypothesis:** one of the `assigned_engineering_fate_deadline` updates on
  management. `HandleAssignedEngineeringFateDeadline` →
  `resumeStrandedFateAssignmentIfPending` races the engineering activation's
  own fate and report commits on the assignment's `LifecycleVersion` CAS.
  The earlier record saw the same race at 10:11:20: "terminal report attempt
  failed … lifecycle command digest conflict".
- A conflict that clears within a second is exactly what a no-backoff retry
  turns into a dead letter.
- **Probe:** the full guest console lines at 16:13:34-35, which carry
  `<agentID>/<updateID>: <err>`.

## Adjacent findings (confirmed by code read; harm partly hypothesis)

- **`Executor.Spawn` holds `e.mu` (write) for all ~14 s of materialization.**
  Every executor call waits behind it: `GoEval`/`Exec`/`ReadFile` through
  `resolveOne`, `HasCapsule`, `MemoryBudget`, `ClaudeDelegateStatus` (which
  `planEngineeringCast` calls for the network mode,
  engineering_assignment_runtime.go:264), and the job verbs.
  - With one live assignment capsule at a time the harm is small today.
  - With N capsules per assignment (ARCH-10's second half) every live
    capsule stalls while a sibling spawns.
- **The spawn saga runs inside a mailbox activation.** It runs on
  management's mailbox for the open-time wake, or on `engineering-admission`
  otherwise, and holds `engineeringAssignmentOpenMu` for the whole ~18-21 s.
  - On the open-time path, management's next activation (for example the
    Texture round's next update) waits for the capsule's cold start.
- **The carried-patch path materializes the full tree twice more**
  (step 5): about +28 s on a retry trajectory, measured from step 7's cost.
  It is not in this run.

## Options to cut the cold start (none built here)

Ordered by value over risk. Costs and risks are the lane's estimates.

1. **One git process per pass instead of one per file:** `git cat-file
   --batch` streams every blob of the inventory through one process (0.16 s
   measured, against 7.15 s).
   - Cost: about 60 lines in source_snapshot.go (a batch reader that parses
     `<oid> blob <size>\n<bytes>\n`).
   - Risk: low. Same bytes, same digest domain, and the existing
     digest-equality tests hold.
   - A mid-stream parse error must fail closed. Red-adjacent: it is the
     source pinning behind the subject digest, so it needs a panel.
   - Gain: about 20 s becomes under 1 s on every spawn, and also the cast's
     7 s.
2. **Do not re-prove a digest the same process just proved.** Pass 3 checks
   a copy that `digestCanonicalSubjectTree` already hashed. Comparing the
   copy's digest with the binding's `SubjectDigest` gives the same
   guarantee, since the binding's digest came from the commit.
   - The resume preflight (pass 1) can be cached per commit for the life of
     the process (commit → digest, keyed by the immutable commit id).
   - Cost: small.
   - Risk: low to medium. It drops a defence-in-depth check against
     corruption of the object store between calls. The copy-side check
     keeps the binding honest, so admit it only with that check in place.
   - Gain: about 14 s (passes 1 and 3) even without option 1.
3. **Cache the materialized read-only source per commit.** Keep one
   `source-cache/<commit>/workspace/platform`, verified once, and use it as
   the overlay's lower layer for every capsule on that commit. It is
   already read-only, and overlayfs never writes a lower layer.
   - Cost: a cache directory with an eviction policy (a commit no live
     capsule references), and a decision about where it lives
     (state volume, `StateFloorBytes`).
   - Risk: medium. It is shared lower state across capsules. A corrupted
     cache poisons every capsule until evicted, so re-verify on first use
     after a boot. It also changes the per-capsule teardown (no
     source-lower to delete).
   - Gain: about 0 s for every spawn after the first on a commit. The
     cast's preflight remains unless option 1 or 2 also lands.
4. **Prepare the next capsule while the current one finishes** (pre-warm).
   Materialize, mount and start the broker for the queued request before A
   ends, and bind it on admission.
   - Cost: large. Admission semantics (a capsule that exists before its
     request is admitted), the memory admission budget (two capsules
     counted), and cleanup of a pre-warm whose request is cancelled.
   - Risk: medium to high. It touches the one-slot rule (ARCH-10) and the
     planned-restart "live capsule" refusal.
   - Gain: hides the cold start entirely, but only for queued requests.
     Options 1-3 make it unnecessary.
5. **Reflink or hardlink copies.** Reflinks need XFS or btrfs. On ext4,
   `cp --reflink=auto` falls back to a full copy. This lane did not check
   the guest data disk's filesystem.
   - The copy is not the cost anyway: the 53 MiB in-process walk takes
     0.2 s. The cost is the git processes, so reflinks only help after
     option 3.
   - Hardlinking from a per-commit cache is option 3 with more risk
     (shared inodes and their chmods).
   - Not recommended.
6. **Move the spawn out of `e.mu`** (materialize unlocked, lock only to
   register the capsule). It does not shorten the cold start; it stops one
   spawn stalling every other capsule.
   - Cost: moderate. Two spawns of the same id must still refuse, so the
     id needs to be reserved under the lock.
   - Risk: medium (red: capsule lifecycle). Needed before N capsules per
     assignment.

Not cold-start, but found here:

- (a) A backoff for a handler error in the dispatcher, as deferrals have,
  so a one-second CAS race does not dead-letter.
- (b) `actor wake latency` measured from the wake's own mint time, not
  from the source object's CreatedAt.
- (c) The open-time `:spawn` wake moved to the `engineering-admission`
  mailbox, the residual of the nudge record.

All three are F1 to F3 of lane/fp4-actor-retry-backoff
(docs/problems/fp4-actor-retry-no-backoff-wake-latency-spawn-mailbox-2026-10-22.md;
tests 94f829bfc, fix b7c5d7c97, disposition 4f003dccb). This lane does not
duplicate them.

## Timing logs (next commit, orange: log lines only)

Every line carries the capsule id, so one grep shows a spawn:

```
grep 'capsule capsule-<id> ' guest.log
capsule <id> saga phase=cast_preflight took=…      assignment=…   (management's cast cell)
capsule <id> saga phase=admission took=…           assignment=…
capsule <id> saga phase=resume_preflight took=…    assignment=…
capsule <id> saga phase=adoption_input|carried_patch|reclaim took=… assignment=…
capsule <id> spawn phase=executor_lock took=…
capsule <id> spawn phase=admit_checks took=…
capsule <id> spawn phase=state_dir took=… preflight=false
capsule <id> spawn phase=source_inventory took=… files=N git_procs=1        (git ls-tree)
capsule <id> spawn phase=source_blobs took=… files=N git_procs=N            (one git cat-file per file)
capsule <id> spawn phase=source_digest_copy took=…                          (in-process)
capsule <id> spawn phase=source_digest_commit took=… files=N git_procs=N+1  (its own ls-tree, then N cat-file)
capsule <id> spawn phase=source_read_only took=…
capsule <id> spawn phase=source_lower_ready|selfdev_inputs|overlay_mount|root_prepare|selfdev_binds|cgroup|egress took=…
capsule <id> spawn phase=broker_start took=…
capsule <id> spawn phase=broker_ready took=… probes=K
capsule <id> spawn total=… ok=true|false [err=…]   (ok=false err=spawn did not return (panic) on a panic)
capsule <id> saga phase=spawn|bind|activate took=… assignment=…
```

**Locks (corrected in round 2, review bunch 77 F5).** The phase lines are
written while locks are held:

- the spawn lines under the executor's `e.mu` (write);
- the saga lines under `engineeringAssignmentOpenMu`.

Each line is one `log.Printf`, about 20 per spawn. That is a few
microseconds against seconds of git inside the same critical section, and
the same practice as the existing teardown memory line, which logs under
`e.mu`. Still, the critical sections are longer by those writes, so "no
behaviour change" means no change to what the spawn does and decides, not
zero added time under the lock. The `total` line is written after the
unlock: its deferred call is registered before the unlock's.

The lines were not moved after the unlock. That would mean buffering them
until the spawn ends, so a spawn that hangs inside a phase would no longer
show the last phase it finished, and the hang is what the lines are for. No
other work was added under either lock. A nil timer (the carried-patch
path) formats nothing.

Prediction for the next deploy, against which the conjecture is judged:

- `resume_preflight`, `source_blobs` and `source_digest_commit` are each
  about 5-7 s.
- Every other phase is under 0.3 s.
- `cast_preflight` is about the same as one of those three.

If `broker_ready` or `overlay_mount` is seconds instead, this record is
wrong about where the time goes, and the next probe is there.
