# SL obligation inventory (define slice, 2026-10-09)

Station [SL](../definitions/choir-appdev-sl-obligation-terminality-2026-10-08.md),
operational invariant O1: every durable obligation has exactly one live
driver and reaches a recorded terminal fate within bounded attempts.
Read-only code survey at main 3d2247da. Not yet measured on a computer;
"unknown" marks what the code read did not settle.

## Finding 1: one periodic driver, about thirty event or boot drivers

The guest has exactly one periodic obligation driver: the actor wake
outbox sweep (`agentcore/runtime.go:2525`, 500 ms ticker at `:2651`). It
dispatches unprojected wakes, at most 64 per tick and 4 per target desk,
and marks each projected, or disposed when its occurrence is gone.

Every other re-drive is a named function called at boot or from an event
handler. There are about thirty of them in `internal/agentcore`
(`reconcile*`, `resume*`, `redrive*`, `recover*`, `drain*`): for example
`reconcileTerminalRunOutcomes`, `ReconcileEngineeringDesk`,
`ReconcileParkedLifecycleCoagentWake`, `redriveStrandedFreshMintManagement`,
`resumePendingPlatformUpdate`, `reconcileSelfDevelopmentMaterialization`,
`reconcileEngineeringAssignmentCapsulesAfterRestart`. An obligation that
strands between boots, with no further event touching it, has no driver
until the next boot. This is the "fragmented enforcer" the register names.

## Finding 2: attempt budgets exist for four paths only

| Path | Budget | Exhausted fate |
|---|---|---|
| lifecycle update delivery (`store/lifecycle_update_delivery.go:49`) | 3 | `delivery_attempts_exhausted`, emitted fail kind |
| lifecycle control delivery (`agentcore/management_controller.go:1141`) | 3 | same path |
| engineering restart recasts (`agentcore/engineering_desk.go:51`) | 3 | assignment fails with "recast attempts exhausted" |
| terminal settlement CAS (`store/store.go:1296`) | 8 retries | error returned |

The actor wake sweep has no attempt count. A wake whose dispatch keeps
failing for any reason other than `ErrNoPendingActorOccurrence` is logged
and retried every 500 ms forever, invisibly (`runtime.go:2585`).

## Obligation kinds

Obligations moved from SQL tables into object-graph kinds; the SQL tables
of the same names are legacy.

| Kind (object graph) | What is owed | Driver(s) | Terminal fates | Attempts | Restart |
|---|---|---|---|---|---|
| `choir.actor_wake_outbox` | wake a desk/run | the sweep (only periodic driver) | projected; disposed (occurrence gone) | none | boot migration re-mints from worker updates (`MigrateActorWakeOutbox`; storm class sa1) |
| `choir.worker_update`, direction `control` | deliver a control to the bound run | management controller delivery; expiry `ExpireStalePendingLifecyclePacket` (4 call sites) | delivered → incorporated; rejected; revoked; expired; exhausted | 3 | re-derived from packets; unknown whether every bound-but-dead case is caught |
| `choir.worker_update`, `directive` | deliver a directive to a desk | same delivery path | as control | 3 | as control |
| `choir.worker_update`, `producer_report` | report reaches its consumer and settles | `commitLifecycleProducerReportAct`; settlement | incorporated; late; rejected | 3 (delivery) | unknown |
| `choir.work_item` | a unit of desk work | `ReconcileLifecycleWorkAssignment`, `reconcileAssignedWorkItemActor*` | completed; cancelled; refused; finalized | none | boot reconcile |
| `choir.run` (parked/passivated) | a run resumes on its wake | `ReconcileCoagentWake` (actor handler), `reconcileTerminalRunOutcomes` | terminal run status | none | boot reconcile; owner rule: Texture work never resumes on boot ("Interrupted by a restart") |
| `choir.trajectory` | settle when its rule is met | settlement on run terminal; `drainCancelledTrajectoryActivations` | settled; finalized | none | unknown |
| `choir.lifecycle_cancel_intent` | a cancel takes effect | projector loop (`runtime.go:2694`) | applied | unknown | resumed (cd7e8cfe) |
| `choir.engineering_assignment` (+ run claim, capsule) | engineering work reaches a verdict | `ReconcileEngineeringDesk*`, `resumeDelegatedCastAssignment`, `resumeStranded*Assignment*`, capsules-after-restart | completed; failed; cancelled; frozen | 3 recasts | boot reconcile |
| `choir.run_continuation` | the next goal starts | run acceptance events | selected → started; blocked; finalized | none | unknown |
| `self_development_operations` (SQL) | a self-development op ends | `reconcileSelfDevelopmentMaterialization`, `recoverSelfDevelopmentDecision` | applied; rejected; failed; rolled_back; degraded | none | boot reconcile |
| platform update pending transition | an update applies or refuses | `resumePendingPlatformUpdate*`, `resumeStrandedPlatformUpdateTail` | unknown | none | resumed after ready |
| `choir.commitment_record` | a precommitment is kept or broken | none found | unknown | none | unknown |
| `choir.inbox_delivery`, `choir.coagent_mailbox`, `co_super_slots` | legacy | none found in the read | n/a | n/a | replay "empty until supported" |

## O1 problem docs by kind

From the 41 `liveness` docs in
[`problem-doc-triage-2026-10-08.jsonl`](problem-doc-triage-2026-10-08.jsonl)
(statuses there are stale; reconciliation is the next step):

- **Actor wake outbox (7):** actor-wake-outbox-createdat-drift,
  kernel-cutover-wake-gap-analysis, s0m-desk-run-dispatch-stall,
  sa1-wake-outbox-rearm-storm, s0m-management-live-occurrence-storm,
  passivated-run-wake-consumed-before-reactivation,
  selfdev-wake-passivated-super-silent-noop.
- **Lifecycle packets (11):** s0m-bound-control-not-marked-incorporated,
  s0m-freed-control-never-rebinds, s0m-freed-control-stale-activerunid-blocks-rebind,
  s0m-stranded-bound-control-deadlock, s0m-directive-engineering-desk,
  sa-delegated-report-poisons-management-listing,
  document-cast-stray-complete-settlement, document-cast-terminal-report-strand,
  texture-incorporate-deadlock-settled-producer,
  coagent-result-parked-run-not-reactivated, s0m-channel-mail-texture-dead-letter.
- **Engineering assignment (5):** engineering-run-death-leaves-bound-assignment,
  engineering-verification-chain-dead, m11-engineering-desk-deferral,
  m11-restart-passivation-cancels-desk-strands-op,
  clustering-assessment-engineering-assignment-stalls.
- **Desk activation / run (9):** texture-desk-activation-contract-respawn-loop,
  sa-management-mint-no-start-slot-deadlock, smg-management-open-invalid-transition,
  s0m-postboot-deskmint-dispatch-starvation,
  m11-texture-required-write-loop-starves-selfdev,
  smg-disposable-texture-budget-loop, owner-revision-supervision-desk-target-409,
  texture-research-hollow-revisions, m11-supervision-blind-desk-progress.
- **Platform update / self-development op (2):**
  s2-refused-apply-wedges-pending-transition,
  s2-stranded-retired-realization-permanent-wedge.
- **Not O1 (7):** vmctl-idle-sweep-hibernates-busy-guest and
  vmctl-pressure-reclaim-mid-respawn-gap (host lifecycle),
  s0-fetch-guest-timeline-deadlock (O12 lock scope),
  root-cause-wrong-path-cluster (O2), three CI/test flakes.

## What this suggests (for the panel, not a decision)

- The outbox is already the one periodic driver and already the re-drive
  authority for wakes. Routing every kind's re-drive through an outbox
  wake, with an attempt count and an exhausted fate on the wake itself,
  gives one driver and one budget without a new poller. (Owner direction
  below replaces any boot re-drive: boot closes work instead.)
- The registry is then a table of kinds, each declaring its terminal
  fates and its wake source. The fate surface reads unprojected wakes and
  non-terminal objects per kind.
- Deletion candidates: the legacy SQL obligation tables, and any
  `reconcile*`/`resume*` function whose case a wake-based re-drive covers.
  Each needs a citer check first.
- Open questions: whether commitment records are obligations here or a
  separate protocol (Gate 3); where the platform update pending
  transition lives and what its fates are; whether actor wakes and
  lifecycle packets need different fate semantics (the definition's
  stated uncertainty).

## Owner direction (2026-10-09, after this inventory)

- No panel. Build the conservative design.
- **Event-driven, not polling.** The outbox driver is signalled when an
  obligation is written, keeps one timer for the earliest due retry, and
  does no fixed-interval scan. Today it queries the store every 500 ms
  even when idle, on a guest store with one connection.
- **Crash restarts end work; planned update restarts resume it**
  (AGENTS.md "Restarts End Work (Crash) Or Resume It"). The host writes a
  durable planned-restart marker before an update restart; boot consumes
  it once. With a marker, the boot-time resumers in Finding 1 run;
  without one, boot closes every open obligation as "interrupted by a
  restart". The resumers are therefore gated on the marker, not deleted.
- A transition whose completion is a restart (a self-development release
  apply, a platform update) is a planned restart: it writes the marker and
  records its own outcome after the boot.

## Next

1. Measure: non-terminal counts per kind and unprojected wakes on a fresh
   disposable and, as counts only, on the owner computer.
2. Reconcile the 34 O1 docs' status against commits.
3. Agentic-consensus review of the SL definition plus this inventory, then
   decide registry shape.
