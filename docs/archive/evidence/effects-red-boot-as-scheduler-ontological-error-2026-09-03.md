# Problem Receipt: Boot-as-Scheduler Ontological Error in Persistent Super Wake

- Date: 2026-09-03
- Mutation class of this receipt: green (documentation); the repair it directs is red
- Problem first named by owner, 2026-09-03, during Definition 1 review
- Status: documented before any code fix (problem-documentation-first)

## The Problem

The persistent Super wake path treats **boot and terminal-event backlog scans
as a normal scheduler tick**. `reconcilePersistentSuperActor`
(`internal/agentcore/super_controller.go:343`) selects pending undelivered
producer reports (`listPendingPersistentSuperAdmissibleReports`) and mints a
fresh privileged Super to drain them. This selection is reachable from:

1. Boot-time reconciliation sweep (`internal/agentcore/runtime.go:2580`);
2. Terminal continuation `maybeContinuePersistentSuperInbox`
   (`internal/agentcore/runtime.go:870`) — every Super terminal re-reconciles
   and can mint a successor from backlog;
3. Owner refresh (epoch advance), which restarts the guest process and re-enters
   the same reconciliation surface.

The steady-state product model is different and was never codified:

- **Paid tiers are always-on 24/7.** While the process is up, work arrives via
  live triggers (owner Texture action, delivered control packet, explicit
  product-path call). A Super runs to completion in-process.
- **Reboots are rare.** A reboot is a recovery event, not a scheduling event.
  On boot the computer restores state deterministically and resumes only the
  exact interrupted in-flight run (the `runtime_restarted` carve-out in
  `rewarmInterruptedPersistentSuperActors`, `runtime.go:2220`, whose own doc
  comment already states "this does not create a new Super from arbitrary
  backlog"). Pending backlog stays pending; the next **live** trigger executes
  it in FIFO order.
- **Lower tiers** may boot-warm the machine; warm-up executes nothing.

## Doctrine Conformance

`docs/computer-ontology.md:130-133`: "The normal computer is long-lived and
reconstructible. A realization may be replaced without changing ComputerID;
reconstruction verifies the immutable event chain and receipts, deterministically
rebuilds embedded state, **and never reruns a model, tool, or network
observation**." A reboot that spawns a 200-iteration model loop from backlog is
a model rerun during reconstruction — current behavior is the non-conformance.
The narrow in-flight resume of an exact passivated run is live-work continuation,
not reconstruction, and remains legitimate.

## Evidence (one disease, five symptoms)

| Date | Symptom | Receipt |
|---|---|---|
| 2026-08-19 | Super non-rewake after CoSuper cancel; repair minted continuation Super from pending cancel report | `docs/evidence/effects-red-super-non-rewake-after-cosuper-cancel-2026-08-19.md` |
| 2026-08-19 | Continuation storm: terminal -> reconcile -> same undelivered report -> new Super, ~11 min cycle | `docs/evidence/effects-red-super-continuation-storm-after-cosuper-cancel-2026-08-19.md` |
| 2026-08-19/20 | Replacement Super `e4141127` minted purely from backlog died at iteration 7 (max_tokens) with no CoSuper | turn receipts, `5e01ac3a` |
| 2026-08-28 | Six consecutive replacement Supers ("Prior implementation CoSuper assignment is terminal...") from the same nine cancel reports across refreshes | live `/api/runs`, 2026-09-03 query |
| 2026-09-03 | Definition 1 authored assuming reconcile-loop cycles drive FIFO observation ("observe persistent Super across >= 3 cycles") and requiring a fresh CoSuper binding to *consume* the nine cancel reports — the error baked into mission authority | `docs/definitions/choir-substrate-and-scheduling-readiness-2026-09-02.md` at `cb7804f9` |

The repair arc `9bc99f90` -> `3654d925` -> `5e01ac3a` progressively patched the
storm *around* the boot-as-scheduler assumption instead of deleting it.

## Root Cause Clustering Assessment

This is a substrate-level classification per the convergence-before-patching
rule: all five symptoms share one cause — reconcile-as-scheduler, i.e., backlog
selection as a wake source. The corrected substrate rule:

```text
Super wakes come from live triggers only.
Boot restores state; it resumes only the exact interrupted in-flight run.
Pending backlog is durable and waits.
Stale residue is settled as late evidence, never consumed-by-next-Super.
```

## Repair Direction (revised 2026-09-03 after 10-model consensus review)
1. **Kill ALL backlog-selection wakes — the full selection ladder, not one branch.**
   Consensus review (2026-09-03, 10-model panel) established this receipt's initial
   repair direction under-enumerated the wake sources. Boot-time reconcile can mint
   a Super through ALL of the following, each equally backlog-derived:
   - `listPendingPersistentSuperLifecycleControls` (`super_controller.go:381`) —
     fires FIRST; the durable ordinals (2, 3 on staging) are lifecycle controls,
     so deleting only the report branch would still boot-schedule;
   - `maybeRewakeSelfDevelopmentTextureAfterTerminalSuper` (`super_controller.go:392`)
     — legitimate as a Texture-agent wake, invalid as a boot-reachable Super mint;
   - `resumeSelfDevelopmentSuperForPendingInstruction` (`super_controller.go:405`)
     — a hidden scheduler if reachable from boot reconcile;
   - `listAndSettlePersistentSuperBacklog` (`super_controller.go:412`);
   - `listPendingPersistentSuperAdmissibleReports` (`super_controller.go:418`);
   - the boot work-item sweep (`runtime.go:2573-2580`) — selects by assigned work
     item, not by report, and calls full reconcile;
   - `maybeContinuePersistentSuperInbox` (`runtime.go:870`) — terminal continuation
     must narrow to controls actually delivered to the terminal run.
   Additionally, the narrow resume carve-out is **intended but not structurally
   isolated**: `rewarmInterruptedPersistentSuperActors` (`runtime.go:2267`) calls
   full `reconcilePersistentSuperActor`, which falls through the entire ladder when
   `reactivateRestartedPersistentSuperControlRun` misses. The repair must add a
   dedicated exact-run resume entry point that returns unconditionally without
   reaching any selection call, and the boot rewarm must call that.
   Deletion applies to the **wake**, never the **selection**: live-triggered FIFO
   drain of pending backlog (acceptance criterion 1) must survive.
2. **Keep the narrow resume**: exact-run resume (`runtime_restarted`, and
   injection-append failure only when it represents the same already-admitted run
   consuming only controls previously delivered to that run) is live-work
   continuation, not reconstruction — but only through the structurally isolated
   entry point from item 1.
3. **Settle stale residue at the store layer**: the nine 08-19 cancel producer
   reports (enumerable as pending producer reports with
   `requested_by_agent_id=co-super:assignment-97191e37-657c-5acf-af18-f1c80d09def2`)
   are tombstoned by a dedicated runtime/store lifecycle-reducer command with a CAS
   precondition (each report still pending and undelivered), a terminal semantic
   disposition (e.g. `UpdateLate`), and an idempotent settlement receipt. Exclusion
   is enforced at the store query layer (all pending selectors), and the fragile
   `claimedPersistentSuperProducerReportIDs` metadata claim-scan is retired
   (replaced, not coexisting). No replacement Super is minted from settled reports,
   before or after boot. This supersedes the `5e01ac3a` replacement-continuation
   direction, which is a patch on the wrong ontology.
4. **Contract flip in tests**: existing tests enforce the wrong ontology
   (backlog -> new Super as success case, e.g.
   `lifecycle_control_injection_test.go` fixtures seeded via
   `reconcilePersistentSuperActor`). They must be rewritten to the new contract.
   All wake entrypoints (boot, terminal, live) need deterministic tests proving
   only the authorized trigger class mints.


## Consequence for the Mission Sequence

Definition 1's remaining acceptance is reshaped: boot-does-not-schedule becomes
a positive assertion; FIFO selection is observed across live triggers; the
producer-report deliverable becomes settlement, not consumption. Definitions 2
and 3 are structurally unaffected; their Super wakes inherit the corrected
live-trigger-only rule.
