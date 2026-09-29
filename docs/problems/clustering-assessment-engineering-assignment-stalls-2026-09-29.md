# Clustering Assessment — Engineering Assignment / Selfdev Op Wedge Class

Date: 2026-09-29
Receipt class: this document IS the mandated clustering assessment (AGENTS.md,
"Root Cause Clustering": 3+ same-subsystem bugs within one week → stop
patching, write the assessment before the next fix).
Author: orchestrator. Status: proposal until owner ratifies.

## The cluster

Four defects in the engineering-assignment + selfdev-op substrate within ~24h
of each other, all discovered by the same M11 staged probe against staging:

| # | Receipt | Row kind | Dead-end state | Missing trigger |
|---|---|---|---|---|
| 1 | `docs/problems/engineering-verification-chain-dead-2026-09-26.md` | texture revision | doc headed by agent profile → chain never opens | opener gate excluded agent-headed docs |
| 2 | `docs/problems/capsule-subject-artifact-ephemeral-2026-09-28.md` + `engineering-subject-digest-paths-wrong-2026-09-28.md` | subject bytes on VM tmpfs | freeze can't open artifact → retries forever | no durable artifactDir; no sentinel→op-fail |
| 3 | (same M11 run, first `awaiting_approval`) | selfdev op | `verified` but no `awaiting_approval` → reconcile skipped | verification chain state set incomplete |
| 4 | `docs/problems/engineering-run-death-leaves-bound-assignment-2026-09-29.md` | engineering assignment | bound run died → capsule still `usable` → reconcile skips → op pinned `executing` | no bound-run liveness check; no reconcile trigger on run-state persist |

Fixes landed: `765f35a4` (chain opener), `90bcf657` (artifactDir + sentinel +
resume + busy signal), `dd48eb2c` (bound-run liveness + reconcile kick).

## Common cause

Every reconcile is **event-triggered only**:
`ReconcileEngineeringAssignmentsForTrajectory` fires on new occurrence, on
open/bind saga completion, on boot. `reconcileEngineeringVerification` fires
per occurrence per doc. Capsule freeze→verify→finalize is tool-call driven.
`selfdev` ops advance via assignment events.

A row that reaches a non-terminal state whose expected trigger never lands
(provider timeout, VM death, panic before the terminal write, state set gap)
**never recovers**. There is no time-based authority that re-derives desired
state from durable rows and acts on it. This is exactly the failure shape the
contract predicts when reconciliation is not total.

The boot sweep (`ReconcileEngineeringAssignmentsForTrajectory` at runtime
start) covers restart-recovery only; it does not run during a live session.

## What is NOT the fix

- Another occurrence-triggered edge patch. Each patch widens event coverage
  but cannot cover events that don't exist (provider timeouts, silent VM
  death, state-set omissions).
- A separate watchdog per row kind. The four wedges are the same bug; four
  watchdogs are four places to get the same thing wrong.
- A polling query per reconcile pass. The stalls are *table-wide*; the repair
  is one scan over the obligation tables with a staleness predicate.

## The substrate proposal

A single periodic **obligation stall sweep** that owns the invariant:

> For every durable row that represents an obligation (selfdev op in a
> non-terminal state, engineering assignment open/bound/executing, capsule
> active-but-bound-run-gone, texture verification pending past deadline), there
> must exist either a live driver or a pending reconcile trigger. Where
> neither exists, the sweep re-runs the existing reconcile or terminalizes
> with an explicit reason.

This is one authority, one pass, over all four tables — the *connect an
## The substrate proposal (post-scout, revised)

The scouts (StallSurfaceScout, ViaNegativaScout) enumerated 20 remaining stall
shapes and audited deletion candidates. Their joint conclusion changes the
plan: the cheapest substrate fix is **not** a generic periodic sweep. It is a
small set of merges that close the crash windows structurally, plus a narrow
backstop sweep for the residue a merge can't cover.

### Merge 1 — durable spawn obligation atomically inside Open commit

Owner-cast Open commits at `engineering_assignment_runtime.go:320-334`, then
spawn/mint/bind runs in the caller. A process death between them leaves
`open+unbound` with no wake (the six-hour assignment wake deliberately no-ops
on unbound rows — `continuation_schedule.go:176-184`). Delegated casts arm a
wake best-effort AFTER Open (`rlm_reduce.go:767-790`; `continuation_schedule.go:
270-279` is log-only on failure). Same window, two different code paths.

Fix: `OpenEngineeringAssignment` (or its delegated variant) writes a
`spawn_pending` obligation in the same object-graph command; a single
post-commit saga drains it. Owner and delegated share the path. Removes the
stranded-open class.

### Merge 2 — assignment fate intent inside the run-terminal reducer commit

`persistActivationStateAndEmit` writes the run terminal state and launches a
goroutine reconcile (`runtime.go:1681-1722`). A hard kill between the durable
write and the goroutine leaves the assignment bound but orphaned — the only
repair is the boot sweep.

Fix: the terminal reducer persists an assignment-fate obligation (or outbox
wake) in the same commit that marks the run terminal. Post-commit saga does
the external revoke/cancel/fail-op work. Removes the death-before-kick window.

### Merge 3 — retire the orphan-fallback authority for Engineering assignments

`ensurePersistedTerminalRunOutcome` diverts assignment-bearing terminal runs
to `RecordEngineeringOrphanObservation`
(`research_checkpoint_fallback.go:56-78`), which terminalizes the assignment
WITHOUT calling `failBoundSelfdevOperation` and WITHOUT capsule revocation.
This is a second fate authority that races the real one — an op can sit
`executing` with a terminal-failed assignment and an active capsule forever.
### Claim projections — deletion attempted, REVERTED (falsified)

The `ogKindEngineering{Run,Capability,Capsule}Claim` objects looked write-only
(no readers found), and the scout recommended deletion with uniqueness moved
to conditions. The attempt failed live: `ObjectCondition{CanonicalID,
Exists:false}` only blocks a bind when an object *exists* at that ID — the
claim objects ARE the uniqueness markers. Conditions alone pass vacuously.
`TestEngineeringAssignmentBindClaimsCapabilityAndCapsuleUniquely` caught the
regression; the deletion was reverted. Verdict: keep the three mints — ~30
lines buying durable per-run/per-capability/per-capsule collision fencing.

### Backstop sweep — narrow, second commit

Stall shapes a merge cannot close (because the trigger *is* a timer or a
policy, not an event):

- `frozen` op with completed implementation but no `candidate_id` (no desk

## Second harvest — 2026-09-29 M11 re-probe (deployed `c689e7fd`)

Two more members surfaced under live load; both are the same cluster shape:
a durable obligation whose only repair channel fires once and dies on the
first error.

### Wedge 4 — selfdev materializer drain is one-shot, no re-arm

`selfdevReconcileDrain` re-runs `reconcileSelfDevelopmentMaterialization`
only when (a) the post-commit observer sees a *new* event append, or (b) the
boot phase fires once, or (c) an owner POST explicitly triggers it. When the
drain's `materializeSelfDevelopmentOperation` errors post-`Apply` — verifier
cert, checkpoint publish, route projection — the drain swallows the error,
leaves the flag clear, and exits. The op stays `materializing`; the next
retry requires a new commit that may never arrive. Observed live: op
`selfdev-89d5e9ef` (computer `computer-ccb04d4a`) sat `materializing` for 50+
minutes after `materialization_applied` committed at 04:14:02Z; updater
journal `phase=completed`; no `checkpoint_published` or
`route_projection_updated` ever followed.

Fix direction: failure re-arms a durable wake (`delegated_` deadline or
outbox kind) instead of dying in the drain loop. The obligation is "op in
non-parked non-terminal state"; a scheduled retry IS the repair channel.

### Wedge 5 — actor-wake outbox mints wakes for terminal/exhausted updates

`migrateActorWakeOutbox` mints an outbox wake for every `ogKindWorkerUpdate`
row regardless of pending state. Rows whose activation already exhausted
delivery (`texture_activation_failed`, `delivery_attempts_exhausted`) mint
wakes that the projector then dispatches on a 500 ms tick and fails
deterministically: `Texture wake has no exact pending canonical occurrence`.
The wake is never marked projected (dispatch errors `continue` without a
mark), so the sweep retries the same dead wake every 500 ms forever.
Observed live on `computer-ccb04d4a`: `assignment-report:cancel-report:
sha256:76819f…` looped every ~500 ms post-restart at 04:20Z onward.

Fix direction: (a) skip minting wakes for worker updates with no pending
canonical row — the migration reads `pending`/`disposed` state already; (b)
when dispatch fails deterministically on a known-stale wake, mark the outbox
entry projected so it leaves the drain set.
  branch classifies this — `engineering_desk.go:296-320` returns nil).
- `frozen` op with terminal-failed/cancelled implementation and no
  verification assignment (no `frozen → failed` trigger).
- `verified` op between verifier-cell transition and desk reconcile; live
  sessions can skip it.
- `awaiting_approval` past mode-receipt expiry — policy decision needed
  (reject/fail/reopen), no repair transition exists.
- `bound` + `freeze_requested`/`revoke_requested`/`frozen` fate with no
  pending proposal — outbox emits nothing.
- `materializing`/`rollback_pending` with hung updater call — no deadline.

A bounded tick on the runtime supervisor (reuses the existing
`selfdev_materialization_reconcile` boot phase structure) scans these
predicates and re-dispatches to existing reconciles. Not a new state machine.

### Explicit non-goals

- No generic obligation framework (one framework for six predicates is
  over-engineered).
- No derivation of `selfdev.Operation` state from assignment state — they
  are different authorities (product vs execution) and Operation carries
  decision/bundle/materialization identity assignment lacks.
- No true single-transaction Open+Bind — external effects (spawn, mint,
  executor inspection) cannot live inside the object-graph transaction.
- No removal of `BoundRunID` — it is the exact join for tool-overlay and
  late-receipt auth.

## Risks / falsifiers

- **Merge 1 changes Open commit shape** — existing delegated Open callers
  (`openDelegatedCastAssignment`, `rlm_reduce.go:767-790`) and owner open
  must agree on the obligation payload; migration of existing `open+unbound`
  rows on staging needs a boot path.
- **Merge 2 ordering** — the terminal reducer must not write assignment fate
  intent before the run row is durable; the saga must be idempotent because
  both the wake handler and the async kick may fire.
- **Merge 3 removes evidence** — orphan-observation text is how "terminated
  without packet" is recorded; the merge must carry that reason into the
  fate path, not drop it.
- **Sweep double-fire** — reconcile functions are idempotent (command-conflict
  guards) but the sweep must not hold a mutex across dispatches.
- **Heresy** — a timer tick is a periodic driver, not an event consumer;
  acceptable only because it re-dispatches to event reconciles, never writes
  transitions itself.

## Decision (post-panel)

Panel verdict (`agentic-consensus-20260928-222842`, convergent): the wedge
class is **missing trigger edges into existing recovery authorities**, not a
missing sweep. The durable deadline continuations already ARE the sweep
substrate; a periodic scan would add a second writer fighting
`engineeringAssignmentOpenMu` and reintroduce mid-saga reaping.

Concrete edge closures (each ~10-30 LoC, reuses existing authorities):

1. **`failBoundSelfdevOperation` at every cancellation site.** Today it is
   called only from `cancelBoundEngineeringRun` + desk paths. Add it to the
   single cancellation choke point (`persistSystemEngineeringCancellation` or
   equivalent) so reconcile/deadline/direct cancellation all close the op.
2. **Desk wake after trajectory reconcile for document-bound trajectories.**
   `ReconcileEngineeringAssignmentsForTrajectory` repairs the assignment; a
   `ReconcileEngineeringDeskForTrajectory` call after repair recasts a
   frozen op's dead verification mid-flight (today only boot does this).
3. **Awaiting-approval regression fix** (codex): `engineering_desk.go:362`
   fails an op whenever `vFound && vLatest.Terminal()` without gating on
   `state==frozen`. A re-reconcile of an approved op spuriously fails it.
   Gate the failure branch to `StateFrozen`.
4. **Delegated-spawn mutex gap** (codex): `resumeDelegatedCastAssignment`
   runs spawn without `engineeringAssignmentOpenMu`; trajectory reconcile
   can reap a live admission. Re-read under the lock or route through the
   saga that takes it.
5. **Bind→activate gap** (codex): `BindEngineeringAssignment` (:769) then
   `rt.activate` (:776) as separate calls; a crash between leaves a bound
   run pending forever. Either activate-in-bind or an activate-failure path
   that cancels.
6. **Merge 1 (spawn obligation in Open commit)** stands — removes the
   owner-cast open+unbound window structurally.
7. **Merge 2+3** deferred: route terminal-run death through the fate path
   (not the orphan fallback) once edge 1 makes fate the single authority;
   keep orphan observation as evidence only.
8. **Claim-projection deletion** — three write-only claim rows; independent
   cleanup, follow-up commit.

Explicitly **rejected** by panel: generic `ReconcileStalledObligations`
owning four tables (mutex + second-writer hazard); periodic ticker writing
transitions (heresy: "database remembers, Go delivers"); deriving op state
from assignment state (different authorities); single-transaction Open+Bind
(external effects can't live in the object-graph transaction).

Ordering: land `dd48eb2c` (in-flight, done), then edges 1-5 + merge 1 in one
commit chain, then the claim deletion, then re-run M11 probe. A narrow
backstop tick MAY still land later for `awaiting_approval` expiry policy —
that one needs a decision (reject/fail/reopen), not a mechanism.

Panel + scouts consulted: `.agentic-consensus/agentic-consensus-20260928-222842/`
(excluding claude per owner token budget); scouts `StallSurfaceScout`,
`ViaNegativaScout` (agent:// refs).
## Third harvest — 2026-09-29 M11 re-probe (deployed `9f8c9866`)

The second-harvest fixes landed and the retry arm works. The re-run found
wedge 6: the materializer's `PublishCheckpoint` request is bound to the
wrong head.

### Wedge 6 — checkpoint request bound to the applied-event head, not the current effective head

`recordMaterializationApplied` (internal/agentcore/self_development_materializer.go:414)
posts `selfdevprotocol.CheckpointRequest{AcceptedEventHead: appliedEventHead, EffectiveEventHead: head.EffectiveEventHead}`
where `appliedEventHead` is the just-appended `materialization_applied`
digest and `head` is re-read *after* that append. `CheckpointAuthority.Publish`
(internal/platform/checkpoints.go:60) requires the request's
`AcceptedEventHead` to equal `computer_event_heads.canonical_event_head`
and `EffectiveEventHead` to equal `computer_event_heads.effective_event_head`
at publish time.

Between the applied event's append and the checkpoint publish, the
per-commit projection sweep mints more `projection_batch_recorded` events
on the same chain — the canonical/effective heads move forward. The request
is stale by the time it reaches corpusd; every retry carries the same stale
heads. Observed live 2026-09-29: op `selfdev-0af47efdf2983e4a5bad609be0cbd597`
(computer `computer-5352d5a8`) sat `materializing` from 06:41:58Z, the
`materialization_applied` event committed at seq 1155, and the drain's
60s retry wake fired ~30× each returning
`guest credential: checkpoint refused with status 400`
(corpusd journal shows no entries — the 400 body says only
`"checkpoint authority: current accepted/effective head does not match request"`).

Why the retry-arm didn't help: it re-fires *the same* `recordMaterializationApplied`
call, which recomputes `head` — but `appliedEventHead` comes from the
`EventReceiptByIdempotency` lookup (immutable `event_digest` of the already-
committed applied event), so `AcceptedEventHead` stays pinned to seq-1155's
digest even though the live head has moved. The op is deterministic-frozen.

Fix direction (not yet landed): the checkpoint request's
`AcceptedEventHead`/`EffectiveEventHead`/`EventHeadReceiptID` must be re-read
*at publish time* from the post-append head — i.e. bind to the head that
contains the applied event's *consequences* (projection batches + any
subsequent canonical events), not to the applied event's own digest. The
authority's strict equality is correct; the producer pins the wrong row.

Hermeticity note: this defect was invisible on the earlier pre-fix run
because the drain died before reaching `PublishCheckpoint`. The retry arm
exposed it — the trigger-edge fix moved the wedge one layer deeper.

Fix belongs in the same transaction family as the second-harvest commit:
re-derive `AcceptedEventHead`/`EffectiveEventHead`/`EventHeadReceiptID` from
`rt.store.Head(ctx, computerID)` *after* the applied event commits, inside
`recordMaterializationApplied`, immediately before `PublishCheckpoint`
(after line ~363's head re-read; the current code re-reads but then pins
`appliedEventHead` instead of `head.CanonicalEventHead`). Event-head receipt
must likewise be re-fetched for the *current* canonical head, not the
applied event's.

### Wedge 6 repair — landed in the dual-head checkpoint commit

The production data forced a wider repair than the paragraph above
proposed. Live state on `computer-5352d5a8`: canonical head seq 1204
(`0729ffe5`), effective head seq 1155 (`b9b4c7de` = the
`materialization_applied` digest) — the heads *permanently* diverge after
any post-apply event. Three coordinated changes:

1. **Producer rebind** (`self_development_materializer.go`,
   `platform_update.go`): `AcceptedEventHead`/`EventHeadReceiptID` now bind
   the fresh post-append `head.CanonicalEventHead` and its append receipt;
   `EffectiveEventHead`/`EffectiveStateCommitment` bind `head.Effective*`.
   New `store.EventReceiptByDigest` joins the canonical-head digest to its
   `event_head_receipt_id`. The `accepted` authorization payload is built
   verbatim from the *minted* checkpoint's request fields (the authority
   recomputes it byte-equal from `checkpoint.Request`), and the
   route-projection request takes a second fresh `Head` read after the
   `checkpoint_published` append — pinning the checkpoint event's digest
   wedges identically under drift.
2. **Protocol relaxation** (`selfdevprotocol/control.go`): the verifier
   class no longer requires `AcceptedEventHead == EffectiveEventHead`.
   That equality predates the append+project cutover (landed 2026-07-19);
   owner-recovery checkpoints already carry `accepted != effective`, and
   the live-head equality stays enforced by the authority's FOR UPDATE row
   check — this layer is shape validation only.
3. **Authority join shift** (`platform/checkpoints.go`): platform-follow's
   `event_kind == materialization_applied` check moved from
   `AcceptedEventHead` to `EffectiveEventHead` — accepted names the tip,
   effective names the applied event under the dual-head shape.

Residual race (accepted): a host append landing between the guest's head
read and the FOR UPDATE check still 400s — but the drain re-arms and each
retry carries a fresh head, so it converges in quiet windows instead of
wedge-permanently. `PublishRouteProjection`'s live-head pin is kept as the
freshness fence (certifies "transition at canonical head H").

### Wedge 7 — apply replay fenced by rotating realization ID

Found on the first drain pass after the dual-head checkpoint deploy
(2026-09-29, computer `computer-5352d5a8`, op `selfdev-0af47efdf2983e4a5bad609be0cbd597`):
the VM refreshed at deploy time (`epoch 12782 -> 12783`), and the guest's
retry of `updater.Apply` for the already-materialized release refused with
`updater client: updater refused request` — one shot at 07:46:45, then the
guest went silent (no armed retry re-fired visibly).

`realizationIDFor(vmID, epoch)` (internal/vmctl/ownership.go:2658) embeds
the epoch, so every boot changes the updater's `realizationID`.
`validateApplyRequest` required `request.RealizationID == updater's` and
folded `RealizationID` into `RequestCommitment`, so a replayed apply for a
journaled operation failed validation twice: mismatched fence and
mismatched commitment (`ErrIdempotencyConflict`). The journal on the VM's
persistent disk showed `phase: completed` — the result was already
durable; the caller just could not read it back.

Repair: `RequestCommitment` no longer includes `RealizationID`, and `Apply`
enforces the realization fence only for fresh requests (no journal). A
found journal is a self-describing resume: commitment equality alone
gates it. Same class as wedge 6 — a pre-cutover identity assumption
(realization as fencing token) colliding with restart-durable replay.
