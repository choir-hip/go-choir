# Restart/Lifecycle Re-arming Cluster — Structural Assessment

**Date:** 2026-09-30 · **Station:** M0 residual → M-SUB design input · **Build:** `4c279162`

Three distinct defects surfaced on the owner computer today share one substrate:
**a lifecycle obligation that survives a restart (or a second owner revision)
has no re-arming authority.** Each symptom is real; the cluster is the signal.

## The three symptoms

1. **Texture desk passivated `runtime_restarted`, never re-woken** —
   `passivateInterruptedActivations` (runtime.go:2302) marks the run +
   `MarkAgentMutationStale`, but no `actor_wake_outbox` row is re-minted for
   the open `owner_revision` obligation. `sweepActorWakeOutbox` hits
   `ErrNoPendingActorOccurrence` and disposes the consumed wake as dead. The
   `textureowner.Handler.Start` reconcile that *does* re-dispatch
   `coagent_result` runs only at process boot, not on the in-process
   `Runtime.Start` restart path. Evidence: texture run `654acaec` passivated
   03:48, doc `485859ad` still at v0.

2. **Engineering assignment cancelled on restart — not replayed** —
   `ReconcileEngineeringAssignmentsForTrajectory` revokes an absent executor
   capsule and cancels the assignment (`restart revoked absent assignment
   capsule`). Unlike Management's same-ID resume (`ReconcileCoagentWake`), an
   engineering cast killed mid-flight is dropped, not re-driven. The owner
   revision obligation stays open but the implementation work is gone.
   Evidence: `run:assignment-6234ff62` cancelled 04:35:40, doc `f939b0f9`.

3. **Second owner revise opens a duplicate desk work item** — `revise` on a
   desk-bound doc with an already-open cast seeded a second desk-target work
   item instead of joining the open one; the next `revise` returned 409
   `lifecycle has multiple open desk target work items`. Owner revision →
   desk-work-item is not deduplicated against an in-flight obligation.

## Common cause

The canonical event commit (owner revision, trajectory start) mints delivery
obligations — `actor_wake_outbox` rows and desk work items — but **the set of
armed obligations is not reconciled against the set of live deliverables after
a discontinuity** (runtime restart, VM reboot, or a second owner input while a
cast is open). Three different code paths (texture wake, engineering capsule,
desk work item) each handle their own happy-path minting; none owns the
"obligation armed but deliverable lost/duped" reconciliation. This is a
substrate gap, not three independent bugs.

## Why this is the M-SUB design input

M-SUB's `cell_fate` + armed-terminal-deadline work already owns "a cell exits →
record fate". This cluster adds the missing dual: **an obligation that was
armed but whose deliverable was lost across a restart must be re-armed** —
the projector must not be the only mint source, and each obligation type needs
a reconciliation that survives restart. Fixing any one symptom in isolation
leaves the class open; the durable fix is a single "armed-obligation re-drive"
pass over `actor_wake_outbox` + desk work items at `Runtime.Start` (not just
process boot), plus dedup of desk-target work items against open casts.
## Post-fix receipts — same substrate, new symptoms (2026-09-30, later)

The two M0-window fixes (`d1d875a0` passivation re-wake; `2404e7d2`
texture-delivery content-derived CommandID) landed and cleared their exact
defects — but each immediately exposed the next discontinuity-handling gap,
confirming this is a substrate class, not three isolated bugs:

4. **`lifecycle receipt join unavailable` (corpusd CAS 400)** — texture audit
   appends (`trajectory_started`, `revision_committed`) are refused because the
   `computer_event` CAS join expects the `lifecycle_observed` event's
   `ProposedEffectRef` to match exactly one *unjoined* lifecycle receipt. The
   crash loop minted 140+ pending `refresh`/`start`/`restart` receipts; the
   join finds ≠1 or a digest that no longer matches, and the append fails.
   Effect so far: audit-log loss (desk commits still land), not a hard block —
   but it is the same "armed obligation left un-reconciled" shape at the CAS
   boundary. (`internal/platform/computer_events.go:264-275`)

5. **`Texture activation returned without disposing exact trigger`** — the desk
   wake fires (`texture:86422cad...408c5b5e` for the M1 prompt doc) but the
   activation returns without marking the wake obligation consumed, so the
   dispatcher defers and re-arms it in a loop. Detected at
   `internal/actorruntime/handler.go:593`.

The invariant the substrate is missing remains: **every minted obligation
(wake, work item, lifecycle receipt, delivery claim) needs a re-drive/reconcile
pass that survives restart and bounds retry.** M-SUB's cell_fate+deadline is the
desk-cell half; this cluster now adds the corpusd CAS join and the
texture-trigger dispose as obligation types the reconcile must cover. The fix is
still substrate-level, not five patches.

## Mutation class

`red` — restart/recovery authority + lifecycle delivery substrate.
