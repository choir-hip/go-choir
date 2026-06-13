# Mission M3 - Lifecycle Cutover (cutover step 4) - v0

Source: `docs/mission-portfolio-2026-06-11.md` section M3. Program:
`docs/choir-rearchitecture-durable-actors-2026-06-11.md` (cutover step 4,
sections 2.1, 2.4, 3/R1/R2). Spec: `specs/actor_protocol.tla` (activate /
deliver / steer / passivate; especially atomic passivation and no lost wake).
Discipline: `skills/parallax/SKILL.md`. Predecessors: M1
(`docs/mission-trajectory-model-v0.md`) and M2
(`docs/mission-messaging-cutover-v0.md`) are settled.

## Source Form

**Kind:** spine.

**Real artifact:** run-shaped goroutine closures replaced by actor activation
loops; `recoverInterruptedRuns` blanket-fail deleted (boot = cold actors +
sweep); cancel-by-trajectory replaces `CancelRunGraph`; `ParentRunID` becomes
`spawned_by_run_id` provenance-only.

**Bridge conjecture (R1/R2):** activation/passivation/sweep semantics, already
proven at the protocol level (`actor_protocol.tla`) and package level
(`internal/actor` tests), survive contact with the real LLM loop.

**Falsifier:** kill -9 mid-activation under multi-agent load; on restart, sends
reactivate with correct memory and zero stranded messages.

**Edge (resource):** the LLM loop's streaming/tool machinery may resist the
clean step/loop/activation boundary; budget for a shim layer rather than
distorting the actor semantics.

**Settlement:** restart amnesia gone; the falsifier passes; ParentRunID control
sites migrated with their features; acceptance evidence re-pointed.

**Dependencies:** M2. **Size:** 2 overnight missions; the big one.

## Parallax State

status: working

**mission conjecture:** if runtime execution moves from run-shaped goroutine
closures to durable actor activation loops, and boot becomes cold actors plus a
wake/sweep instead of blanket-failing interrupted runs, then the deeper
rearchitecture goal advances: agents own lifecycles, old trajectories can
rewarm, restart amnesia is gone, and completion/parent-tree liveness no longer
controls coagent work.

**deeper goal (G):** durable actors, evidence-bearing promotion, and
self-development operational instead of documentary.

**witness/spec (A/S):** a runtime lifecycle cutover in which:
- live agent work is represented as an activation over an agent identity, not
  as the durable lifetime of a run row;
- incoming `update_coagent` records activate cold actors or steer warm actors
  at step boundaries;
- passivation compacts memory and releases residency without losing messages;
- boot treats actors as cold and reactivates from durable backlog/obligations;
- cancellation is by trajectory/work item rather than run graph;
- `ParentRunID` is no longer a control relationship and survives only as
  `spawned_by_run_id` provenance where needed for lineage/audit.

**invariants / qualities / domain ramp (I/Q/D):**
- I: exactly-once ledger effects remain in the runtime store transaction from
  M2; visibility remains at-least-once. No new wake/message primitive. No
  blanket fail-on-restart. No parent/child control reads.
- I: passivation's idle check and mailbox-empty check must be atomic enough
  that a message cannot arrive between "idle" and "safe to sleep" and recreate
  a lost wake.
- Q: the LLM loop may keep a compatibility shim temporarily, but the mission
  cannot settle while two permanent lifecycle models remain. Names should
  reflect actor terms: step, loop, activation, passivation, rewarm,
  trajectory, work item, spawned_by.
- D: start in-process with runtime tests over one computer. Grow to process
  restart proof with update-woken actors, then deployed staging smoke. Cross-VM
  outbox semantics remain explicitly out of scope unless the lifecycle cut
  touches super/vsuper across computers.

**variant (ranking function) V:** count remaining lifecycle blockers:
activation shim not yet wrapped around the real LLM loop and run-memory path;
interrupted activation passivation is only a compatibility state, not the final
actor residency model; residual `ParentRunID` control reads still need
`spawned_by_run_id` provenance-only semantics; active-run queries still stand
in for resident actor / trajectory obligation queries outside the vSuper
co-super slot guard; cold wake/sweep covers pending `update_coagent` backlog
and generic warm step-boundary steering, but not all live trajectory
obligations; restart falsifier missing; acceptance evidence still points at run
continuation/lifecycle state instead of trajectory/work-item/activation
evidence. Current V=1. Last Delta V: Batch F expected -1 and actual -1:
super-to-co-super skip-level blocking and vSuper exported-package cancel
protection now read co-super slot records instead of `ParentRunID` authority,
and regressions prove slot-owned same-trajectory co-supers are protected while
same-parent other-trajectory co-supers are ignored. Batch G then closed the
assigned-open-work silent-stall subclaim: boot now rewarms cold assigned work
items after passivating interrupted activations, even when there are zero
pending `update_coagent` rows. Batch H added an in-process resident-agent
index and renamed the hot path to `executeActivation`, so warm reuse no longer
depends first on persisted active-run rows. Full mission V remains 1 because
the activation body still records terminal run state as evidence, cancellation
keeps active-row compatibility fallbacks, and deployed restart acceptance has
not run. Batch I then narrowed the run-memory blocker: fresh tool-loop
activations seed their new `run_memory_entries` log with a deterministic
`actor_rewarm` compaction checkpoint from the latest prior inactive activation
for the same `(owner_id, agent_id)`, preserving prior compacted memory before
the wake input is appended. Commit
`a7b43100bf789480ee8da1a2ec4c78f0b0217e2b` then landed this bridge: CI run
`27462249760` and deploy job `81178185271` succeeded, public
`https://choir.news/health` reported proxy and sandbox commit/deployed commit
`a7b43100bf789480ee8da1a2ec4c78f0b0217e2b`, Playwright deployed lifecycle
smoke passed, and browser-public prompt-bar/VText/RunAcceptance smoke accepted
`runacc-cd78deed35b77e23cddd` at `staging-smoke-level` for trajectory/run
`d224018b-a651-40b1-8e1e-dd9287d94c28` and VText document
`e93fead9-2f1b-49ab-8b0f-b87e6f0c2f52`. This smoke proves the deployed
product path remains healthy after the bridge; it does not prove the
kill/restart actor-memory falsifier. Earlier landing proved the code reached staging, but
the deployed RunAcceptance smoke exposed the remaining acceptance-repointing blocker: a
prompt/VText trajectory at `https://choir.news` produced `staging-smoke-level`
evidence at deployed commit `a2252af27b5db087cbbb931e8d1b5dc04e402285`, while
the synthesized RunAcceptanceRecord `runacc-ffec1c9975f357724d29` stayed
`blocked` because `product_path_observed` still requires `super_requested` and
`worker_mutation_bounded` still requires worker/export/adoption evidence even
when no worker mutation was attempted. Commit
`25c498365221485cfe19bcb5d2a1992bb8bd6986` fixed that local semantics and its
push CI/deploy succeeded, but the first deployed acceptance rerun still saw the
old invariant semantics from an active interactive computer. A forced staging
workflow dispatch with `DEPLOY_ACTIVE_VM_REFRESH=true` and `DEPLOY_HOST_OS=true`
then failed during Node B activation: `go-choir-sandbox.service` exited during
the NixOS switch, and `/health` reported `status=degraded` with upstream 502s
while the proxy build identity stayed at `25c498365221485cfe19bcb5d2a1992bb8bd6986`.
Deploy diagnostic commit `68fd27e4dde77470a39c4b3071d937c9e63590ca` then
proved the sandbox startup root cause in workflow dispatch run `27461068327`,
deploy job `81174942714`: the sandbox journal repeated `runtime store:
bootstrap: apply schema: Error 1072: key column 'delivered_at' doesn't exist in
table`. Commit `a08076eda2ac6ca9ebcacb27e466d0399e6a1db2` fixed the local
runtime store bootstrap order, but the first staging deploy still ran the old
baked Nix sandbox package. Commit `05f9a1507f5060ec92e2ff173c006d4be8fbbf88`
fixed the host service execution contract so systemd service wrappers prefer
the `/var/lib/go-choir/services/<service>` pointer package and retain the baked
package as fallback. Main CI run `27461596479` and deploy job `81176429793`
succeeded, public `https://choir.news/health` reported proxy and sandbox
commit/deployed commit `05f9a1507f5060ec92e2ff173c006d4be8fbbf88`, and a
browser-public prompt-bar/VText/RunAcceptance smoke accepted
`runacc-e2a8723d1f297b9d8389` at `staging-smoke-level` for submission
`8502e863-ab64-41c7-836d-4c737a87e7cf` and VText document
`958f8575-60b8-48d5-ac03-a67ebf69e28b`.
The mission remains open on full lifecycle/restart falsifier proof; no
continuation-level, promotion-level, or final M3 settlement is claimed.
Batch J probe found a remaining active-run control fallback in `cancel_agent`:
after a vSuper failed to find a co-super slot for the requested agent in the
caller trajectory, it fell through to `GetLatestActiveRunByAgent`, allowing a
same-owner vSuper to cancel an agent activation in another trajectory. This is
documented before the fix because it is a real authority-boundary problem, not
just naming cleanup. The follow-up fix makes caller-trajectory co-super slot
ownership the only vSuper `cancel_agent` authority; non-vSuper compatibility
cancellation still retains its active-run fallback. Commit
`dd165ada20609f3dca0e2bd968f46e7796a83e5f` landed the fix: CI run
`27462568946` and deploy job `81179081886` succeeded, public
`https://choir.news/health` reported proxy and sandbox commit/deployed commit
`dd165ada20609f3dca0e2bd968f46e7796a83e5f`, deployed lifecycle smoke passed,
and browser-public prompt-bar/VText/RunAcceptance smoke accepted
`runacc-3326b96bd926f0ac5692` at `staging-smoke-level` for trajectory/run
`cd07ccc4-f35c-4855-9e06-bdb9d2df99cb` and VText document
`a84a3aed-c463-4380-925c-fb46ca800a0a`.

**budget:** 2 overnight missions. Solvency check: do not spend the first pass
rewriting the whole LLM loop. First buy the map: classify lifecycle reads and
separate control semantics from provenance/test fixtures. If the map shows the
activation loop cannot land in one batch, split into a shimmed in-process actor
activation step followed by deletion of the old lifecycle model; do not let the
shim become permanent.

**authority / bounds:** repo changes on the current mission branch; behavior
changes require focused tests, runtime shard/full touched-package tests, review,
then push/CI/staging proof before settlement. Follow Problem Documentation
First: any new staging or reliable evidence of a problem gets a doc checkpoint
before its fix.

**position / live conjectures / open edges:**
- C1 (R1 bridge): actor activation loops can wrap the current LLM/tool loop
  without weakening message/wake semantics. Test by replacing the lifecycle
  boundary while preserving M2 update delivery tests.
- C2 (R2 bridge): deleting completion as an agent concept improves correctness
  without losing user-visible run/accounting evidence, because work items and
  trajectories carry terminal state. Falsifier: a real product/control flow
  whose only correct representation is "agent completed."
- C3 (restart): boot as cold actors plus sweep preserves or reopens work better
  than `recoverInterruptedRuns`. Falsifier: kill -9 leaves messages or work
  items stranded with zero open obligations.
- C4 (provenance): `ParentRunID` can be renamed/re-scoped to spawned_by lineage
  without controlling cancellation, liveness, or slot ownership. Falsifier: a
  remaining parent read that is not a provenance query and cannot be expressed
  by trajectory/work item.
- Edge/resource: streaming/tool execution may not have a clean interruptible
  step boundary. Bound this with a shim; do not alter actor semantics to fit a
  legacy closure shape.
- Edge/missing_oracle: activation/passivation needs observability: resident
  actors, cold backlog, open obligations, and rewarm reasons should be
  queryable enough to diagnose stalls.

Inventory result plus constructs so far: current code starts
`executeActivation` goroutines from `startRunAsync` / `StartChildRun`, and
registers resident `(owner, agent)` entries for warm/cold decisions. Boot no
longer blanket-fails pending/running runs. It passivates interrupted activations with
`RunPassivated`, emits `activation.passivated`, clears stale VText mutation
blockers, and sweeps pending `update_coagent` backlog through the existing wake
logic. Batch C deleted the `CancelRunGraph` parent-tree cancellation entry
point; VText mutation cancellation now uses trajectory cancellation and its
regression proves spawned-by-only provenance does not control cancellation.
The Batch B probe found a sharper bug in the compatibility shim:
generic super/coagent `update_coagent` records are marked delivered by terminal
run update, including failed/cancelled activations, so an activation can consume
the only durable wake record without successfully incorporating the update.
That lost-wake failure mode is now fixed for the generic super/coagent shim:
only completed update-woken activations mark `worker_update_ids` delivered,
while failed/cancelled/blocked activations leave the backlog pending. Warm
generic super/coagent activations also poll pending updates through
`RunToolLoop`'s injection seam after tool turns and at final checkpoint; those
updates are appended as user turns, scoped to the addressed actor, and become
delivered only on successful activation completion. Batch D then moved vSuper
co-super slot budget and verifier sequencing to trajectory slot ownership:
active slot counts and implementation-slot history decide admission, while
direct parent-child ancestry is no longer a vSuper co-super liveness oracle.
Batch E then moved vSuper package reuse to the same trajectory slot authority:
the implementation slot's `publish_app_change_package` evidence is reused even
when the implementing co-super is not a direct child, and direct children on
other trajectories no longer select the package. Batch F then moved skip-level
co-super directive blocking and exported-package cancel protection to the
trajectory slot registry: direct parent-child ancestry no longer decides those
authority guards. Batch G added the missing assigned open-obligation boot
sweep: live/open work items with an assigned durable agent now rewarm a cold
generic actor after restart even without pending worker updates. Batch H added
the product runtime's volatile resident-agent index and switched warm rewarm
controllers to it; blocked/nonresident historical rows no longer suppress a
fresh coagent activation when durable backlog exists.
Batch I added the v0 actor-memory bridge: when a new tool-loop activation has
no current run-memory entries, initialization looks up the latest prior
completed/passivated activation for the same owner and agent, appends an
`actor_rewarm` compaction checkpoint to the new activation's memory log, then
appends the wake message. This keeps `loop_id` as activation evidence while
making rewarm context actor-scoped for the first provider call.
Batch J probe found that vSuper `cancel_agent` still uses `GetLatestActiveRunByAgent`
as a fallback after slot lookup misses, so an agent outside the caller
trajectory can be selected by latest active run. Batch J fixed that vSuper
path: a missing or inactive caller-trajectory slot now fails with `agent not
active in caller trajectory`, and the comprehensive regression keeps the
different-trajectory run running. Non-vSuper compatibility cancellation retains
its active-run fallback for now.
See "Lifecycle Inventory - 2026-06-13" below.

**next move:** keep M3 open as a lifecycle cutover mission, not a deployment
recovery mission. The service-pointer execution gap is fixed and staging is
healthy at `dd165ada20609f3dca0e2bd968f46e7796a83e5f`; public product-path
smoke accepted RunAcceptanceRecord `runacc-3326b96bd926f0ac5692` at
`staging-smoke-level` with `product_path_observed` and
`worker_mutation_bounded` passed. The next discriminator is the durable-actor
restart falsifier: kill/restart or equivalent deployed evidence that a cold
actor rewarms from durable backlog/open assigned obligations with zero stranded
messages or zero-obligation stalls. Cancellation's store-active fallback and
`executeActivation` terminal run rows are accepted for v0 as compatibility/audit
surfaces, not ordinary warm-residency or agent-liveness oracles.
Preserve historical `parent_loop_id` compatibility surfaces for trace/API
evidence until the rename cut is explicit, but do not let spawned-run
provenance decide liveness or authority.

**ledger file:** `docs/mission-lifecycle-cutover-v0.ledger.md`.

**version / lineage:** v0 compiled 2026-06-13 after M2 settled at
`794d28dd76ff00a2ae27c98a14dbce9e34834695`. Predecessors: M1 trajectory model,
M2 messaging cutover. Successors gated on this: M4 continuation deletion and
M5 Wire settlement falsifier.

**learning state:** retained here. M2 learning carries forward: no old/new
dual model may survive settlement; stale prompt/tool surfaces are blockers, not
cleanup; product surfaces are downstream falsifiers, not proof that the spine
stands.

**settlement:** not claimed. Required landing proof: commit, push `origin/main`,
CI, Node B deploy, staging health identity, and deployed acceptance proof. The
behavioral gate is the restart/amnesia falsifier plus no stranded messages or
zero-obligation stalls after rewarm.

## Lifecycle Inventory - 2026-06-13

Classification is against durable-actor sections 2.1/2.4/3/R1/R2 and
`specs/actor_protocol.tla`: send appends a durable update, warm actors steer at
a step boundary, cold actors activate from backlog, passivation requires an
atomic idle/backlog check, and boot recovery is sweep, not run failure.

| Surface | Current evidence | Classification | Cutover action |
|---|---|---|---|
| `executeActivation` | `Runtime.startRunAsync` and `StartChildRun` register `rt.running[runID]` plus `residentAgents[(owner, agent)]` and launch `go rt.executeActivation`; `executeActivation` transitions one activation's run-evidence row pending -> running -> terminal and owns `executeWithToolLoop` / `executeWithProvider`. | Compatibility activation body. | Keep the existing tool loop as the activation body for v0 while residency and wake decisions move to actor identity. Do not treat terminal run state as agent completion; it is activation evidence. |
| `recoverInterruptedRuns` | Deleted in this pass. `Runtime.Start` now calls `passivateInterruptedActivations` and `sweepPendingUpdateActors`; pending/running rows become `RunPassivated` instead of failed. | Delete. | Continue shrinking the compatibility passivation state into true actor residency: no resident actors on boot, pending backlog/open obligations are queryable and reactivated. Tests that asserted blanket failure were rewritten around passivation/rewarm. |
| `CancelRunGraph` | Deleted in Batch C. VText mutation cancel calls `CancelRunTrajectory`, which cancels open trajectory work items, marks the trajectory cancelled, and terminalizes active activations on that `trajectory_id`. Direct `CancelRun` remains activation termination evidence. | Delete / port to trajectory-work item. | Continue removing parent-tree cancellation assumptions from tests and callers. Cancellation reach is trajectory membership, not `parent_loop_id` recursion. |
| `ParentRunID` / `parent_loop_id` | Stored on runs and exposed as `parent_loop_id`. Batch D removed vSuper co-super budget and verifier sequencing dependence on direct children; Batch E removed package reuse's direct-child selector. Trace graphs, VText verifier parent checks, root trajectory refs, tool prompts, skip-level/cancel active-run checks, and many tests still read it. | Rename to `spawned_by_run_id` provenance, with control reads removed. | Keep historical `parent_loop_id` data frozen for compatibility during the cut, but new semantics are provenance-only. Authority/budget checks move to trajectory membership, co-super slot records, explicit requester metadata, or work items. |
| Active-run graph queries | vSuper co-super admission now counts active trajectory co-super slots, not active direct children. Persistent-super, coagent, assigned-work, and VText wake paths now use the volatile resident-agent index for warm reuse. Batch J removed the vSuper `cancel_agent` fallback from missing caller-trajectory slot to `GetLatestActiveRunByAgent`; vSuper cancellation is now limited to active co-super slots in the caller trajectory. `GetLatestActiveRunByAgent` otherwise remains for blocked-state evidence, requester provenance, and non-vSuper cancellation compatibility fallback. `RunningCount` / `RunningCountByProfile` still report running activation evidence for health/admission. | Port to trajectory/work item plus actor registry/residency. | Admission uses resident actor counts / per-owner caps and co-super slots; "waiting on work" uses trajectory obligations and pending update counts. Remaining active-run reads must stay audit/compatibility, not decide ordinary warm actor residency or cross-trajectory vSuper authority. |
| Boot recovery | Boot now passivates interrupted activations, sweeps pending `worker_updates`, and sweeps live/open work items that already name an assigned durable agent. `TrajectoryObligations` still exposes unassigned open obligations without selecting an actor. | Delete old recovery, port to sweep. | Continue shrinking boot recovery toward an explicit actor-residency/backlog table. Assigned work items are rewarm backlog; unassigned obligations need owner/supervisor routing and must stay observable rather than silently spawning an arbitrary actor. |
| Run-memory compaction | `run_memory_entries` are still physically keyed by `loop_id`, but new tool-loop activations now seed an `actor_rewarm` compaction checkpoint from the latest prior inactive activation for the same `(owner_id, agent_id)` before appending the wake message. `executeWithToolLoop` still initializes `runMemoryManager`, compacts on thresholds/overflow, and `CompactRunMemory` is manual per-run. | Wrap in activation shim, then port to actor memory snapshot. | The v0 bridge makes compacted context available across activations without a schema migration. The durable target remains an actor memory snapshot plus log tail; `loop_id` should stay activation/run evidence, not the long-lived memory identity. |
| Update-woken delivery | `update_coagent` writes `worker_updates`; `wakeUpdatedCoagent` calls `reconcilePersistentSuperActor` / `reconcileUpdatedCoagentActor`; those create or reuse active runs and mark update IDs delivered at terminal run update. | Wrap in activation shim. | Cold delivery should activate from backlog; warm delivery should inject steering input at a step boundary. Delivery/incorporation should be tied to processing the update, not to a whole run ending. |
| Acceptance evidence | `RunAcceptanceRecord` has `trajectory_id` but still carries `loop_id`; `continuation-level` is gated by a `continued` checkpoint after promotion, and AGENTS.md says this is transitional until M4. | Port to trajectory/work item / activation evidence. | M3 evidence should prove activation/sweep/rewarm and no stranded messages. M4 re-points `continuation-level` formally; this mission must avoid claiming continuation-level from old run continuation evidence. |
