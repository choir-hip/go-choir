# Effects Super non-rewake after CoSuper 200-iteration cancel — 2026-08-19

**Boundary:** diagnose. Not freeze. Not promote. No live send.
No restore. No Super-start-from-scratch. No `maxToolLoopIterations` patch.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Host:** `https://choir.news/health` `deployed_commit`
`d33f245c3e9bf0ec9bfb72451eb275b07acddbaa` (`deployed_at` 2026-08-19T17:43:06Z)

**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **326**
`propose_only` generation 1. Effects remain OFF.

**Pre-A fence:** checkpoint
`99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7`

## Live observation (2026-08-19T18:15Z)

GET `/api/computers/{id}/self-development/operations/selfdev-ccf0f1ec0e851750f253fe5f5ed97974`
still `executing`. Empty `bundle_digest`. `updated_at` frozen at 17:45:05Z.

| Actor | ID | State | Times |
|---|---|---|---|
| Super | `f009f383-c31c-41ca-8d87-6ea2e6deb581` | `completed` | 17:45:04Z–17:45:27Z |
| CoSuper | `run:assignment-97191e37-657c-5acf-af18-f1c80d09def2` | `cancelled` | 17:45:16Z–17:57:48Z |
| Texture caller | `d0502969-a820-5540-932d-6088b74bb8dd` | `running` | created=updated 17:45:04Z; no tokens |

CoSuper error remains:

```text
tool loop: exceeded 200 iterations without end_turn
```

No later Super run in `/api/runs`. Texture document
`c273a57b-a253-5234-888d-6139024a6cf1` still `current_version_number=0`.

Super result (verbatim summary of its own protocol) then **completed**:

1. Consumed Texture `execution_request` for operation `selfdev-ccf0f1ec…`.
2. Opened `assignment-97191e37` via `assign_co_super`.
3. `report_to_texture` with `work_disposition=open`.
4. “I'll await its result packet on the durable lifecycle control run and
   incorporate it when it returns.”

`request_source=lifecycle_texture_control`. Bound control
`15ce0662-8d98-5f00-b7d9-441d98aed845` on Super work
`fd43ecca-cb82-53cf-91b5-dbe6f2412f97`. Parent work for the assignment is
that same Super work item. Super metadata has no `actor_park_on_idle`.

Capsule `sh -c` is not this failure. Guest exec on this assignment already
ran without `getpgrp` (see
`docs/evidence/effects-red-capsule-broker-job-control-2026-08-19.md`).

## Causal chain

CoSuper 200-iteration abort is `handleExecutionError` for an assigned CoSuper
(`internal/agentcore/runtime.go`). That calls `terminalizeRun` →
`cancelBoundCoSuperRun` → `persistSystemCoSuperCancellation`.

`CancelCoSuperAssignment` still builds a pending producer report
(`kind=blocker`, `role=co-super`,
`Direction=producer_report`) via `buildCoSuperReturnPacket`. Because parent
Super `f009f383` is already terminal, `DeliveredToRunID` is left empty so the
packet stays pending in Super's mailbox (`internal/store/cosuper_assignments.go`).
That mailbox write is the 2026-08-18 cancellation-delivery repair and is not
this miss.

`persistSystemCoSuperCancellation` returns that result and **does not call
`wakeUpdatedCoagent`**. Contrast:

- `newCancelAssignedCoSuperTool` wakes on the queued update.
- `ReconcileCoSuperAssignmentsForTrajectory` wakes after restart cancel.

Tool-loop fate therefore records the blocker and never sends the actor
message.

Even if the actor message were sent, Super would not start. Completing Super
clears actor resume memory (`memoryFromRunState`). The next
`coagent_result` with empty memory calls `ReconcileCoagentWake` →
`reconcilePersistentSuperActor`.

`reconcilePersistentSuperActor` starts Super only when:

1. `listPendingPersistentSuperLifecycleControls` finds a pending Texture
   `Direction=Control` `kind=execution_request` joined to **open** Super work,
   or
2. mailbox leftovers pass `filterPersistentSuperExecutionUpdates`.

That filter is `persistentSuperExecutableUpdate`: Texture control +
`execution_request` only. A CoSuper `blocker` is
`persistentSuperAdmissibleReport` — injectable into an **already running**
`lifecycle_texture_control` Super (`pendingCoagentUpdatesForRun`, the
2026-08-18 injection repair) — but it is not a wake key.
`settlePersistentSuperNonExecutionUpdates` leaves admissible reports pending,
so the mailbox holds a packet no reconciler will start Super to drain.

The original Texture opener is already delivered to `f009f383`. HTTP
operations POST is the only production mint of
`turn:selfdev-texture-rewake:` (`ensureSelfDevelopmentTextureJoin` when latest
Super is terminal). That path was forbidden until this receipt. Texture
`d0502969` never left `created_at` and never authored a rewake control.

`TestSelfDevelopmentTextureJoinRewakesTerminalPersistentSuper` only covers
that HTTP rewake after an explicit unbind. No test covers Super continuation
on a system CoSuper cancel.

## What this is not

- Not the 2026-08-18 “dead parent delivery stamp” bug. `DeliveredToRunID` is
  empty on this terminal parent.
- Not “Super never learned the assignment existed.” Super opened it, reported
  it, then ended the run.
- Not Texture failing to wake Super via a new `execution_request`. Texture
  issued the first control; Super consumed it. Texture has not issued a second.
- Not a reason to raise `maxToolLoopIterations` first. A longer loop would
  still complete Super before CoSuper returns.

## Required join (unpaid)

A terminal CoSuper producer report must continue persistent Super **without**
an owner HTTP operations POST:

1. `persistSystemCoSuperCancellation` must `wakeUpdatedCoagent` on the
   cancellation update, matching the tool and restart-reconcile paths.
2. `reconcilePersistentSuperActor` must treat pending
   `persistentSuperAdmissibleReport` packets as wake-eligible when Super is
   terminal and no Texture `execution_request` is pending, **or** Super must
   park on the same `lifecycle_texture_control` run after `assign_co_super`
   with open work so the 2026-08-18 injector can fire.

Do not patch in this docs receipt.

## Residual (still unpaid)

- `maxToolLoopIterations = 200` aborts capsule authorship before freeze.
- `deploy-impact` still classifies last push, not last-deployed SHA.
- Texture supervision revision while work is open remains unpaid (document
  version 0).

## Forbidden until the Super-continuation repair deploys

- freeze / propose / promote
- live send
- restore
- HTTP Super-start / operations POST as a substitute for automatic
  continuation
- raising `maxToolLoopIterations`
- SQL-empty or replace the retained computer

## Rollback

No product-state rollback in this receipt. Checkpoint `99949fe2` remains the
pre-A fence. This file is docs-only.
