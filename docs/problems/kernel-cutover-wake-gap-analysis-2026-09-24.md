# Problem Receipt: Kernel Cutover Wake Gaps (G1–G11)

Date: 2026-09-24
Status: documented before fix, per the problem-documentation-first invariant.
Source: `SubclassDGapScout` read-only gap analysis 2026-09-24, against the
outbox fold landed in `12a3521d` and sub-class (b) migration in `f18cd4bf`.
Mutation class of this document: green (analysis; no runtime change).

**Update 2026-09-24 (post-implementation):** G1/G3/G5/G8/G10 closed by the
generalized outbox (`bf40aa1c`); G9 closed by the pending-fate wake
(`fcfc025a`). **Reclassified on re-analysis:** G2 and G6 are **non-gaps
under the kernel model** — an interrupted activation's triggering event
stays unprocessed (crash before `Commit`), so `PendingAgents` re-fires it
directly; `handleInitialDispatch` accepts `RunPending`/`RunRunning` and
re-executes. A cleanly-parked run is woken by its next event. The boot
passivation sweep exists only to mark runs for the old rewarm path — it is
deleted at cutover, not replaced by a wake. The residual concern is handler
idempotency under re-execution (panel finding B5), a correctness property,
not a wake edge. **G11 (self-dev materializer) is the only genuine remaining
gap** — operation-state transitions mint no event; it may be a boundary
exception (self-dev operations as a separate control plane) pending an
authority decision. G4/G7 fold into R3's dual-path cutover.
## The finding

The kernel's pending projection (`PendingAgents` = due unprocessed
`actor_updates`) is **not sound for cutover**. It correctly reproduces every
*due* continuation the legacy `Sweep` would activate, and it covers the
lifecycle `worker_update` class via the outbox fold — but the outbox only
fires on `choir.worker_update` objects committed through
`commitLifecycleTransition`. Many canonical transitions commit
continuation-bearing state (an open work item, a passivated run, a pending
assignment, an owner revision, a cancellation intent, an operation-state row)
**without** a `worker_update` — so no actor wake is minted, and the
continuation strands the moment its legacy sweep/timer is deleted.

**Root cause of the gap class:** the outbox derivation watches one object
kind (`worker_update`) instead of the general rule — *any canonical
transition that creates a continuation obligation must mint a wake.* The
fold is correct; its trigger set is too narrow.

## The 11 gaps

| Gap | Canonical transition that creates the obligation | Missing wake |
|---|---|---|
| G1 | `StartLifecycle` initial work/agent (`lifecycle.go:719-720`; callers `texture_lifecycle_create.go:104-123`, `selfdev_texture_join.go:90-118`) | initial-dispatch wake is direct-only |
| G2 | passivation/replacement run state (`runtime.go:2080`, `:2202`; `lifecycle.go:2045-2046`) | rewarm wake not derivable |
| G3 | `OpenLifecycleWork` work item (`lifecycle.go:2027-2051`) | work-discovery wake absent |
| G4 | legacy `CreateWorkItem` claim/spawn (`trajectory.go:144-193`; wire caller `wire_publication.go:608-636`) | claim wake absent |
| G5 | owner revision/head commit (`lifecycle.go:3433-3437`, `:3515-3518`) | owner-revision wake absent |
| G6 | Texture mutation/run reactivation (`texture_controller.go:920-930`; `texture.go:2020-2088`) | reactivation wake absent |
| G7 | synthetic legacy terminal outcome (`research_checkpoint_fallback.go:145-184`; `store.go:2862-2930`) | bypasses outbox (DispatchWorkerUpdate, not commitLifecycleTransition) |
| G8 | cancellation intent commit (`lifecycle.go:4490-4517`) | cancel-drain occurrence absent |
| G9 | capsule disposition/fate state (`engineering_assignments.go:2237-2240`) | fate wake absent before watchdog arm |
| G10 | bound-assignment 6h deadline (`engineering_assignments.go:1114-1132`; sweep `engineering_assignment_fate.go:477-525`) | no durable scheduled event |
| G11 | self-dev operation-state materializer (`selfdev/operations.go:369-458`; scanner `self_development_materializer.go:36-67`) | materialization wake absent |

## What is already covered (not gaps)

- Lifecycle `worker_update` producer/control packets → outbox fold (`12a3521d`).
- The four sub-class (b) watchdogs → durable `not_before` events (`f18cd4bf`).
- Parked actor mailbox snapshots → covered when pending lifecycle controls
  exist (`adapter.go:536-586` consumes the outbox-covered control class).
- `CancelEngineeringAssignment` terminal report → folds a worker update.

## The fix direction (design decision, not yet implemented)

Two candidate shapes, to be adjudicated before implementation:

1. **Generalize the outbox trigger** — derive a wake not only from
   `worker_update` objects but from any canonical transition object that
   carries a continuation obligation (work item, run state, assignment,
   revision, cancellation intent, operation state). Requires each transition
   to declare its continuation target; the outbox body already carries
   target/kind/content, so the derivation widens, not the schema.
2. **Fold over `choir.lifecycle_event` instead of `worker_update`** — every
   lifecycle event that implies a continuation mints a wake. Cleaner
   conceptually (the event *is* the obligation) but `lifecycle_event`
   objects don't uniformly carry `target_agent_id`, so a target-resolution
   step is needed.

Either way, the cutover gate stays closed until G1–G11 are closed or
explicitly moved outside the kernel's authority boundary (like the three
`frame_lock` exceptions). The write-fence cutover must not proceed on the
current projection.

## Consequence for the stack

This is the evidence that the kernel is build-then-migrate, not
delete-only — and that the migration's hard part is not the substrate
(landed) but the completeness of the wake edge set. R3/R4 and the
self-development gate queue behind closing these gaps.

## Update 2026-09-24 (consensus panel): a second gap class — state repair, not wake edges

A 6-agent convergent panel (codex, claude-opus, gpt6-sol, cursor-grok, cursor,
gemini; `.agentic-consensus/agentic-consensus-20260924-171648/`) reviewed the
three cutover fixes (idempotent outbox re-mint, post-drain trajectory return,
ineligible lifecycle passivation). All three fixes were confirmed correct for
their immediate failures, but the panel named a **second, systematic gap class**
distinct from the G1–G11 wake-edge set:

> **The outbox re-fires a committed continuation obligation. It cannot perform
> a boot-time state correction that has no canonical event to fold — and a
> process restart is not a canonical event.** (claude)

The deleted sweeps did two jobs: (1) re-fire an obligation, (2) mutate local
state with no corresponding canonical event. The outbox replaces only (1).
Corrections still at risk:

| Correction | Old sweep | Kernel status |
|---|---|---|
| Eligible interrupted lifecycle run that crashes *after* its trigger commits | `rewarmInterruptedLifecycleActivations` → `rt.activate` | **zombie**: stays `running`, holds `ActiveRunID`, no unprocessed tape row, `MigrateActorWakeOutbox` won't re-arm a projected wake |
| `ensureSpawnedCoagentWorkItem` minted on passivation | `sweepPassivatedSpawnedCoagentWork` / generic `passivateBatch` | **absent** from `passivateInterruptedLifecycleActivation` — passivated coagent work has nothing for the fold to project |
| Open work items at `LifecycleVersion > 1` | `sweepOpenWorkItemActors` | `MigrateActorWakeOutbox` filters `LifecycleVersion == 1` — multi-version open work stranded |
| Transient-failure retry runs (admission/injection) | `reactivateRetryableLifecycleInjectionRuns` | producer deleted; recovery-occurrence handler branches now dead code |
| Interrupted persistent-management resume | `rewarmInterruptedPersistentManagementActors` | producer deleted; `MigrateActorWakeOutbox` does not index `RunRecord` |
| Terminal-outcome binding repair | `reconcileTerminalRunOutcomes` | **still a boot sweep** — not folded |
| `coagent_result` re-arm of a consumed wake | n/a | deterministic `update_id` → tape `ON CONFLICT DO NOTHING` drops the re-drive |

**Identity disconnect (must not be "fixed" naively):** the outbox `UpdateID` is
*obligation* identity (deterministic from the wake key); the tape `update_id`
is *occurrence* identity (random per drain). Wiring the outbox `UpdateID` into
the tape append without a re-arm generation discriminator would make a consumed
row silently drop every re-drive. The correct shape is `key + generation`
(e.g. source `LifecycleVersion`) for the tape id.

**Gate consequence:** the design's "delete the sweep, rely on the outbox"
coverage claim is falsified at the state-repair seam. Before the cluster recount
can reach zero, each remaining boot mutation must be either (a) kept as an
explicit named exception, or (b) given a canonical event so the fold sees it.
The eligible-run zombie and the spawned-work-item mint are the two highest-risk
instances.

## Update 2026-09-24 (resolution): sweeps restored as explicit state-repair passes

The panel's gate consequence was adjudicated by restoring the deleted sweeps
rather than inventing canonical events for restart. The seven functions
(`rewarmInterruptedLifecycleActivations`,
`rewarmInterruptedPersistentManagementActors`,
`reactivateRetryableLifecycleInjectionRuns`,
`enqueueLifecycleResearchAdmissionRecoveryOccurrence`,
`sweepPendingUpdateActors`, `sweepOpenWorkItemActors`,
`sweepPassivatedSpawnedCoagentWork`) plus the Management helpers
(`ResumeInterruptedPersistentManagementControlRun`,
`resumeInterruptedPersistentManagementControlRunLocked`,
`rewarmReactivatedManagementResumeWatchdogs`, `errResumeWatchdogScanCap`) were
restored verbatim into `internal/agentcore/boot_recovery.go` /
`management_controller.go` and wired into `Start` unconditionally (no
`kernelMode` gate). They run alongside the projector: the outbox re-fires
committed obligations, the sweeps perform the state repairs that have no
canonical event. The reconcile functions they call are idempotent
(`activeRunByAgent` resident checks), so the two paths do not double-activate.

Verified: the seven restart/recovery tests that failed under the delete-only
cutover now pass (`TestProcessRestartRewarmsCoagentAfterOSKill`,
`TestStartRewarmsCoagentWithPendingUpdatesAndAssignedWork`,
`TestRuntimeInjectionAppendFailurePassivatesAndRestartReactivatesExactResearchRun`,
`TestStartSynthesizesSpawnedWorkItemForPassivatedChildWithoutBacklog`,
`TestStartRewarmsAlreadyPassivatedSpawnedChildWithoutBacklog`,
`TestProcessRestartRewarmsSpawnedChildWorkItemAfterOSKill`,
`TestPersistentManagementLifecycleControlsStayTrajectoryIsolatedThenReconcile`),
and the three earlier fixes still pass. The eligible-run zombie is resolved by
`rewarmInterruptedLifecycleActivations` re-dispatching eligible runs via
`rt.activate`; the spawned-work-item mint by `sweepPassivatedSpawnedCoagentWork`
+ `ensureSpawnedCoagentWorkItem`.

**Residual:** the obligation-vs-occurrence `update_id` disconnect (the outbox
`UpdateID` is never wired to the tape) remains a latent trap — documented in the
`actorWakeOutbox` comment. `coagent_result` re-arm of a consumed wake still
drops on the tape dedup. These are recorded for the next boundary, not fixed
here.

## Update 2026-09-24 (recount consensus): SEND BACK — the sweeps re-fire, not just repair

A convergent review panel (codex, gpt6-sol, claude, cursor;
`.agentic-consensus/agentic-consensus-20260924-231458/`) reviewed the landed
recount candidate `ba4f1881` and returned SEND BACK. The non-kernel actor path
deletion was confirmed real (`internal/actor` is clean: no `Sweep`,
`AgentsWithBacklog`, `NewRuntime`, or coalescer). Three defects were fixed in
`40bda8dd` (emission `drain` not clearing → cumulative emissions; the new
panic-recovery path skipping `RecordAttempt` → a panicking handler tight-loops
past poison accounting; `Drain` ignoring its timeout).

The blocking findings, and the adjudication they force:

1. **Boot sweeps exceed the state-repair exception.** The sweeps do not only
   mutate durable state — they mint wakes (`rt.activate` → `dispatchActor`,
   `wakeUpdatedCoagent`, `reconcile*`) that cause actors to run. The panel's
   position: a scan that *decides* a continuation occurs is a second
   continuation authority even though the dispatcher is the sole *deliverer*.
   The counter (this session's analysis): the sweeps are wake *producers* for
   restart-recovery obligations the pending projection cannot express (a
   restart mints no event), and every wake they mint is a canonical
   `actor_update` the dispatcher delivers — so there is one delivery
   authority. The two readings diverge on whether "produces a canonical event
   by scanning" is inside or outside the one-authority invariant.
2. **Wire debounce `time.AfterFunc` is a real lost continuation.** The batch
   (`pendingDocIDs`/`pendingRevisionIDs`) lives only in
   `wirePublishDebouncer` memory; a restart inside the 300s window drops the
   reconciler run. `recoverOpenWirePublicationClaims` cancels open claims but
   does not rebuild the batch. Fix direction: persist the batch + mint a
   durable `not_before` event at the window deadline (the (b) pattern).
3. **`context.AfterFunc` progress deadline — split verdict.** claude/cursor:
   legitimate per-activation preemption (the durable `activation_budget_deadline`
   is the backstop; serial-per-actor means the durable wake cannot preempt an
   in-flight activation, so the in-process timer is the only preempting path).
   codex/gpt6-sol: a second route to `terminalizeRun`. The durable backstop
   exists; the question is whether in-process preemption is a violation.
4. **`reconcileSelfDevelopmentMaterialization` (G11)** runs via `go` +
   `ListByStates` on accept/rollback — unresolved; needs a tape wake or a
   named exception.

**Gate consequence:** the cluster recount cannot reach zero under the current
charter text without either (a) a canonical `process_boot` event that the
dispatcher folds into the repair scans (making restart recovery
dispatcher-driven), or (b) an owner-ratified charter amendment naming the
state-repair sweeps + the debounce/deadline timers as exceptions. This is an
authority decision, not an implementation detail — recorded for the owner.

---

## Landed fixes 2026-09-25 (incident: accounts fail to boot)

Three red-class commits pushed to `main` to bound the delivery authority and
unblock boot:

- `f481e622` — **boot stall**: the embedded-Dolt `engineMu` no longer spans the
  network CAS (validate+intercept moved out of the lock; `PutObject`/`PutBatch`
  narrowed to the direct-SQL tx). `MigrateActorWakeOutbox` moved from synchronous
  `actorruntime.New` to an async post-`Start` goroutine — it minted N sequential
  network appends before the guest listened, the confirmed boot hang. Also:
  `RecordAttempt` now runs *before* the handler + a pre-handler poison check, so
  a process-fatal crash mid-handler still increments the durable attempt count.
- `41f1303f` — **bound delivery**: `adapter.New` wires `MaxAttempts=8` +
  `ErrorSink=<scoped delivery-poison mailbox>` (was dead code: empty
  `DispatcherOptions`). `ErrDeferUnprocessed` now backs off via `DeferUpdate`
  (sets `not_before`, rolls back the pre-recorded attempt) instead of
  hot-looping and burning the poison budget.
- `b575b2fb` — **stable wake identity**: `actorDispatchUpdateID` was a fresh
  uuid for most kinds, so a recovery-sweep re-mint got a new `update_id`/budget
  each boot and escaped the poison bound. Now a deterministic hash of the
  logical wake `{owner,computer,to,kind,content,trajectory,from}` — a re-minted
  wake lands on the same durable row and the same attempt count.

**Panel adjudication adopted:** the boot sweeps are *kept* as wake producers for
restart-recovery obligations (the pending projection cannot express a restart);
the doom loop is bounded by `MaxAttempts`+`ErrorSink`+deterministic wake id
rather than by deleting the sweeps. User directive "recovery is separate from
auto-continue" is satisfied: sweeps mint wakes (recovery), the dispatcher
delivers them (one delivery authority); a stuck wake dead-letters instead of
auto-continuing forever.

## Owner ruling 2026-09-25 — sweep authority settled

The pending "do the boot sweeps count as a continuation authority" question is
answered by the owner:

> At boot time work should not be restarted. Paid users' VMs are always-on; a
> reboot means a crash or an update, in which case no work starts without user
> input. Cold-booting non-paid users and burst VMs likewise start work only on
> a prompt (owner or agent). **State recovery is a different thing entirely.**

Consequence: the boot sweeps that *resume in-flight work* — `rewarm_*`,
`sweep_open_work_item_actors`, `sweep_pending_update_actors`,
`sweep_passivated_spawned_work` — are wrong-path under this ruling and are
deleted in K's (d) pass. The sweeps that *repair durable state* —
`passivate_interrupted_activations`, `reconcile_terminal_run_outcomes`,
`recover_wire_publication_claims` — are state-recovery and stay. The
re-listener wake loop (`actor.go:192`, `dispatcher.go:256`) is not a boot
restart and is unaffected.

This overrides the consensus panel's keep-the-sweeps adjudication: a panel
reading "restart recovery needs producers" is superseded by the owner's
"restart causes no work" authority.
