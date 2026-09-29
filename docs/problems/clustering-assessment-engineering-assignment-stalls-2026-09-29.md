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

Fix: the assignment branch of the orphan fallback is removed once Merge 2
exists; orphan observation stays for non-assignment generic child runs.
Single fate authority.

### Deletion — write-only claim projections

`ogKindEngineeringRunClaim`, `ogKindEngineeringCapability`,
`ogKindEngineeringCapsule` are minted at Bind
(`engineering_assignments.go:1585-1604`) and read nowhere. They exist only
for not-exists uniqueness conditions, which the assignment/run objects can
carry directly. Delete the three projections + vocab migration entries +
their tests after preserving equivalent uniqueness checks. Net ~-100 LoC.

### Backstop sweep — narrow, second commit

Stall shapes a merge cannot close (because the trigger *is* a timer or a
policy, not an event):

- `frozen` op with completed implementation but no `candidate_id` (no desk
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
