# Engineering admission cancels the live assignment (2026-10-21)

Status: **problem record, hypotheses from code reading, not trace-confirmed.**
Every claim below was read from the code at `f399a2e8c` (origin/main when the
lane branched). None has been confirmed against a staging ledger. Per
CLAUDE.md "Review the traces before attributing behavior", each is a
hypothesis until the orchestrator's staging drill (two Casts in a row, the
second while the first is live) shows the ledger agreeing. Mutation class of
this document: **green** (no code changed by it).

Items: SDC ARCH-10, launch half
([security-density-architecture-checklist-2026-10-21.md](../security-density-architecture-checklist-2026-10-21.md));
launch item L3-08 ([launch-checklist-2026-10-21.md](../launch-checklist-2026-10-21.md)).
Station: [choir-launch-fp4-selfdev-architecture-2026-10-21.md](../definitions/choir-launch-fp4-selfdev-architecture-2026-10-21.md).

## The claim

A new engineering assignment cancels the engineering assignment that is
running. Doctrine says the opposite. `docs/choir-doctrine.md` (the
management admission ledger) and `docs/computer-ontology.md:378-385`:
"one live engineering assignment per computer in the initial sequential
phase: each request carries a durable computer-scoped arrival ordinal,
selection is FIFO among non-expired requests, ... and admission refusal is
retryable with work left pending". The code supersedes instead of queueing.

The SDC checklist (ARCH-10) split the item: admission correctness (FIFO with a
retryable refusal, no cancellation of live work) is launch work, because the
launch checklist's L3-07 builds several app agents at once and a second
request must not destroy the first. N parallel capsules per assignment stays
P2.

## Where it happens (read from the code)

1. `openDelegatedCastAssignment` (`internal/agentcore/engineering_assignment_runtime.go:281`)
   commits the assignment row, its binding and the casting cell's inbox
   records in one transaction (C1e D1, D4). It checks nothing about other
   assignments. The same commit mints a
   `delegated_assignment_spawn_deadline` wake (`internal/store/lifecycle.go:645-650`).
2. The wake runs `HandleDelegatedAssignmentSpawnDeadline`
   (`internal/agentcore/continuation_schedule.go:157`), which takes
   `engineeringAssignmentOpenMu` and calls `resumeDelegatedCastAssignment`
   (`engineering_assignment_runtime.go:302`), which calls
   `spawnBindActivateAssignment` (`:426`). In kernel mode this is the only
   spawn path; the non-kernel inline path is `inbox_records.go:750-751`.
3. `spawnBindActivateAssignment` calls `reclaimSupersededAssignmentCapsules`
   (`:480`, the function at `:651-701`) before it spawns the new capsule. That
   function lists every assignment of the computer and, for each other one whose
   capsule is not `revoked` or `unbound`:
   - if the assignment is terminal, it revokes the leaked capsule (budget
     reclaim; correct, and unchanged by the fix);
   - otherwise it calls `cancelAssignedEngineering` with the reason
     "superseded by a fresh implementation assignment; capsule budget
     reclaimed". That cancels a **live** bound assignment: its run stops, its
     capsule is revoked, and a cancel Report goes to management.
4. `maxManagementActiveEngineeringSlots = 2` and `enforceEngineeringSlotBudget`
   (`internal/agentcore/runtime.go:1656-1676`) are reachable only from the
   legacy coagent spawn path (`runtime.go:1285, 1321`), not from the records
   Cast. They do not bound the records path. (SDC ARCH-10 calls them
   unreachable; they are reachable from the legacy spawn, which is not the
   Cast path.)
5. An assignment that is `open` and `unbound` holds no capsule, so the reclaim
   skips it (`engineering_assignment_reclaim_test.go` asserts this). Two
   Casts that arrive close together therefore do not cancel each other while
   both are unbound; the second Cast's spawn cancels the first only after the
   first has bound a capsule.

## Consequences (hypotheses)

- A second owner request that reaches management while an engineering
  capsule is working destroys that work. Texture and management cannot ask for
  the first to finish.
- The launch loop (L3-07, several app agents built at once) is unsafe on one
  computer: every new Cast kills the previous build.
- There is no refusal the caster can retry. The only outcome of a second Cast
  is the loss of the first.
- The doctrine's other clauses have no code: no durable arrival ordinal on the
  assignment (the row has `created_at` and the assignment id), and no expiry
  of a queued request.

## What is already right and must not change

- The assignment deadline (`assignedEngineeringDeadline`, six hours) fails a
  **bound, active** assignment closed. It is the bound that keeps a stuck live
  assignment from blocking the queue forever.
- A crash boot cancels every open assignment as `interrupted_by_restart` and
  never re-dispatches (`reconcileEngineeringAssignmentsForTrajectory`,
  `crashBoot`). `delegated_assignment_spawn_deadline` is in
  `RestartWorkKinds` (`planned_restart.go:171`), so a queued request's retry
  wake from before a crash is dropped as `interrupted_by_restart`. A queue
  must not become a boot-time resumer.
- Terminal assignments whose capsule was never revoked still have their
  capsule revoked before a fresh spawn (budget reclaim, not cancellation of
  live work).

## Substrate versus symptom

Symptom of an unfinished port. The doctrine is settled (the ledger paragraph
in the doctrine and the ontology), the reclaim was written for the "one live
capsule" rule of C1e D4 with supersede as the cheap way to keep it. The
replacement for supersede is admission in the same saga, before any cancel: no
new substrate. There is no existing unwired admission ledger to connect
(checked: no `admission` ordinal or queue for assignments in `internal/store`
or `internal/agentcore`; the "admission ledger" of I26 is doctrine text).

## Failure modes of the fix, written before the tests

1. **Head-of-line blocking forever.** A stranded `open` request (its wake lost
   or poisoned) blocks every younger one. Bound: a queued request older than
   the assignment deadline stops blocking, and its own retry cancels it with a
   visible reason.
2. **A stuck live assignment blocks the queue.** Previously a re-Cast
   superseded it. Now the queue waits for its deadline (six hours) or an
   explicit Cancel by management. Accepted and recorded as a residual: the
   doctrine chose waiting over silent destruction.
3. **Out-of-order admission.** Two queued requests retry at different times;
   the younger could reach the saga first. The admission predicate refuses a
   request while an older, non-expired `open` request exists, so order is by
   `(created_at, assignment_id)` regardless of retry timing.
4. **Hot polling.** The retry wake re-lists every assignment object of the
   computer (`ListEngineeringAssignmentsForComputer` scans the kind). The retry
   backs off (10s doubling to 60s) and stops when the request is admitted,
   cancelled or expires.
5. **A duplicate retry collapses.** The wake's update id is derived from kind
   and content (`actorDispatchUpdateID`), not from `not_before`; a retry with
   the same content would dedup into the consumed row. The retry content
   carries a retry count so each wake has its own id.
6. **The retry crosses a crash.** It must not. The kind is already a
   restart-work kind; a test asserts nothing re-arms at boot.
7. **Non-kernel mode strands a queued request.** The inline spawn
   (`inbox_records.go:750`) has no wake to retry. Production runs kernel mode
   (`internal/autoputer/run.go:529`). In non-kernel mode a busy refusal is
   returned to the cell as a typed retryable error and the request stays open;
   recorded as a residual, not a supported path.
8. **A cancel of live work by another path.** The fix removes the supersede
   cancel in the spawn saga. Other cancel paths (management's `Cancel`, the
   deadline, trajectory cancellation) are explicit fates and stay.

## Rollback

Revert the fix commit. The change adds no store schema, no new event kind and
no new persisted field: the queue state is the assignment rows already on the
tape and the retry wake is an ordinary scheduled actor occurrence. Reverting
restores supersede-on-spawn.

## Heresy delta

- discovered: the code deviates from the settled doctrine (supersede instead
  of FIFO); the SDC statement that the slot budget is "unreachable" is true of
  the Cast path and false of the legacy coagent spawn path.
- introduced: none.
- repaired: none until the fix lands.

## Review findings on the candidate (2026-10-21, b0f872bd6)

Two independent reviewers (Sol, Argon) sent the first candidate back. The
findings below are recorded before any fix. B1 and B2 were reproduced by Sol
with real-store overlay probes; B3 is from code reading (Argon) and is a
hypothesis until a test shows it. Mutation class of this section: green.

### B1: a planned restart cancels the queue

The queue made `open` + `unbound` a legitimate long-lived state. The restart
reconcile (`reconcileEngineeringAssignmentsForTrajectory`,
`engineering_assignment_fate.go`, the `BoundRunID == ""` branch) still reads
every such row created before this boot as an interrupted spawn strand: it
records a revoke intent, acknowledges the absent capsule, and cancels with
"restart acknowledged absent pre-bind assignment capsule". On a planned
restart (marker present, `crashBoot == false`) that destroys every queued
request, although the station's q7/q8 say a planned restart keeps the queue.
The same branch also runs during normal operation (the run-end kick and work
wakes call the reconcile with `crashBoot == false`), so after a planned
restart any later reconcile of the trajectory would cancel the request too.
The surviving retry wake then finds a terminal row and drops.

Sol's probe: `assignment=cancelled capsule=revoked reason="restart
acknowledged absent pre-bind assignment capsule"`.

The crash rule is not in question: a markerless boot cancels every open
assignment `interrupted_by_restart` and drops the queued wake; that stays.

### B2: expiry is checked only on a busy refusal

`engineeringRequestExpired` is applied to the request itself only in
`deferEngineeringAdmission`, which runs only when admission refuses. Two
failures follow:

1. A request whose deadline passes between retries, and whose blocker
   finishes before the next retry, is admitted and enters the spawn path.
   Sol's probe: `request_expired=true disposition=open ... assigned
   Engineering capsule authority unavailable` (the probe supplied no
   executor, so it stopped just past admission).
2. An older request that has expired but is still `open` (its own retry has
   not fired) no longer blocks a younger one, which is right ("FIFO among
   non-expired requests"), but its row stays open while the younger runs,
   and when its retry fires its cancel reason blames the younger request.
   The existing test `TestEngineeringAdmissionExpiredQueuedRequestDoesNotBlock`
   passes only because admission never notices that the admitted request is
   expired too.

### B3: queue wait counts against the bound execution deadline

`HandleAssignedEngineeringFateDeadline` fails a bound, active assignment
closed at `CreatedAt + deadline`. With a queue, `CreatedAt` is arrival, not
the start of execution: a request that waited 5h55m and then bound is killed
five minutes into its run. Separately, the deadline wake's content is
`{assignment_id, attempt}` both at open and at bind, and the wake's update id
is derived from its content, so the bind-time arm dedups into the open-time
wake's row; if that row was already consumed (it fired while the request was
still queued, a no-op), the bound assignment has no deadline wake at all.

### Non-blocking findings recorded with them

- ARCH-04's observe-mode counts (`objectgraph.WriterGateCounts`) have no
  reader outside tests; the guest log line is first-seen only. The counts are
  the oracle for the enforce flip and must be readable without SSH
  (standing question 9).
- Story opening writes `choir.texture_document` and
  `choir.texture_revision` from management's records commit
  (`internal/store/story_open.go`). Undeclared today, so observe mode passes
  it; before enforce, story seeding needs its own writer accounting.
- When the live assignment ends, the next queued request waits for its own
  retry (up to one minute); nothing wakes it.

### Repairs (lane/b10-selfdev-b1fix)

B1 7e24b0889, B2 d5055214a, B3 a46ed9a9b, writer-gate counts on
`/api/runtime/observability` bbd19192c, next-request wake 059924c7e. Story
seeding's writer accounting is left for the enforce flip (residual). Each
fix's test failed on b0f872bd6 before the fix. The staging drill is still the
only trace evidence.

## Round 2 review findings (2026-10-22, c494f5aac on deploy G)

Panel bunch 30: Gemini flash accept, Devin accept with nits, DeepSeek v4.1
flash accept with edits (`~/runs/fp4-r1-deepseek.md`, `~/runs/fp4-r1-devin.md`
on FP0's machine). FP0 asked for the six edits below before the lane lands as
deploy H's live red. Recorded before any fix; R1, R2, R4 and R5 are from code
reading and become findings when their tests fail on c494f5aac. Mutation
class of this section: green.

- **R1 (medium).** `admitEngineeringAssignment` reads the clock again after
  `expireOverdueEngineeringRequests`. If the request's deadline passes
  between the two readings, admission returns `errEngineeringRequestExpired`
  without cancelling, and `HandleDelegatedAssignmentSpawnDeadline` takes
  that error to mean "already cancelled" and returns nil. The row stays
  open and unbound, its wake is consumed, and no retry is armed.
- **R2 (medium; Devin finding 3 is the same hole).** The bound deadline is
  armed after the bind commit (`spawnBindActivateAssignment`). A planned
  restart, or a `scheduleActor` failure, between the commit and the arm
  leaves a bound assignment with no deadline wake, and the planned-boot
  reconcile leaves a bound, usable assignment alone. A stuck run is then
  unbounded until management cancels it.
- **R3 (medium, operational; interaction with deploy G's freeze close).**
  After assignment A completes with a frozen candidate, deploy G's close
  leaves A's operation open for management's Verify and the owner's
  decision, and selfdev allows one open operation per computer. The woken
  B binds, but every B freeze is refused `ErrOperationOpen` until A's
  operation is decided, and B can spend its six-hour deadline retrying.
  The one-open-operation rule stays; the refusal must be visible and the
  wait bounded.
- **R4 (low).** The nudge (`wakeNextQueuedEngineeringRequest`) picks the
  earliest open row without filtering expired ones, then returns because
  that row is expired: the first live request waits for its own retry.
- **R5 (low).** The queued-expiry and bound-deadline cancel reasons say
  "request remains pending and retryable" on a row that is cancelled.
- **R6 (low, forward wiring).** `RegisterKindChecked` works on a
  `*Registry`, but `DefaultRegistry()` returns a fresh copy and the
  registry `CheckWriteAs` reads (`writeRegistry`) is unexported. No package
  install can register a kind the gate reads.

### Round 2 dispositions (lane/fp4-on-g)

Findings 783cd578c, tests 6f1861830 (R1 to R5 each failed on c494f5aac),
fix 11dfea955.

- **R1: repaired.** Admission's own expiry check cancels the request with
  the sweep's cancel (`expireQueuedEngineeringRequest`) before it returns
  `errEngineeringRequestExpired`.
- **R2: repaired for a planned restart.** A planned boot re-arms the bound
  deadline of every bound, active assignment
  (`rearmBoundEngineeringDeadlinesAtPlannedBoot`); the wake carries the
  bind-time content, so a surviving wake dedups. It only arms a deadline
  and a crash boot never reaches it. Residual: a `scheduleActor` failure
  after the bind commit without a restart still leaves the run without a
  deadline wake until the next planned boot (the same post-commit shape the
  open-time arm had before this lane); minting the wake in the bind commit
  is the full fix and needs a store change.
- **R3: made visible and bounded, not removed.** Of the two options (keep B
  queued while an operation is open, or say why B's freeze is refused), the
  smaller and safer is the second: admission and selfdev's
  one-open-operation rule are unchanged, and an assignment that does not
  freeze is not held back. `openFreezeCandidate` turns `ErrOperationOpen`
  into a refusal on B's own run (the tool result in its transcript) that
  names the open candidate, says it waits for management's Verify and the
  owner's decision, and tells the run to report partial or failed instead
  of retrying; it logs the refusal too. The six-hour bound deadline still
  bounds the wait. **Residual for L3-07** (several app agents built at
  once): while one candidate waits for the owner, every other assignment on
  the computer can work but not freeze, so N app agents are built one
  owner decision at a time. Lifting that is a change to the
  one-open-operation rule (per-app operations or a candidate queue), an
  owner decision this lane does not make.
- **R4: repaired.** The nudge skips expired rows when it picks the next
  request.
- **R5: repaired.** The queued-expiry reason ends "the request is
  cancelled", the bound-deadline reason "the assignment is cancelled".
- **R6: recorded, no code.** Precondition for ARCH-03's package install and
  for the gate's enforce flip: the install path must register its kinds in
  the registry `CheckWriteAs` reads (export a registration on
  `writeRegistry`, or make `DefaultRegistry()` the shared instance), with a
  test that a kind registered at install is refused for another writer in
  enforce mode. Observe mode refuses nothing, so nothing is wrong today.
