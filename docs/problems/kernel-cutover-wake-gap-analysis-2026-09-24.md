# Problem Receipt: Kernel Cutover Wake Gaps (G1–G11)

Date: 2026-09-24
Status: documented before fix, per the problem-documentation-first invariant.
Source: `SubclassDGapScout` read-only gap analysis 2026-09-24, against the
outbox fold landed in `12a3521d` and sub-class (b) migration in `f18cd4bf`.
Mutation class of this document: green (analysis; no runtime change).

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
