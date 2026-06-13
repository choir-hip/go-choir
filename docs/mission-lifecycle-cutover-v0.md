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
Batch K probe found the next restart falsifier gap: boot sweeps pending
`update_coagent` backlog before assigned open work items, and actor residency
is keyed by `(owner_id, agent_id)` rather than `(owner_id, agent_id,
trajectory_id)`. If a cold coagent has both pending updates and assigned open
work on the same trajectory after restart, the update sweep starts a replacement
activation with only `worker_update_ids`; the later work-item sweep sees the
actor resident and returns without attaching `work_item_ids` or the assigned
obligation prompt. The open work item remains observable through
`TrajectoryObligations`, so this is not a zero-obligation invisibility bug, but
it is a real rewarm incompleteness: the activation that was supposed to resume
all durable backlog for that actor may not be told about the assigned
obligation.
Independent review of the first Batch K fix candidate widened the problem:
`ListPendingWorkerUpdates` batches by target agent, not trajectory. A single
replacement activation may carry pending updates for trajectories A and B, but
the first candidate only attached assigned work from `updates[0].TrajectoryID`;
the work-item sweep would then skip both trajectory groups because the actor was
already resident. The fix must include assigned open work for every trajectory
represented in the pending update batch, not only the first trajectory.
Commit `63767a43673007aaca27e926c74dd6e9ee7093f3` landed that Batch K fix:
the cold update-backlog rewarm now also attaches assigned open work items for
every distinct pending-update trajectory in the actor backlog. CI run
`27462963675` and deploy job `81180171763` succeeded, FlakeHub publish run
`27462963683` succeeded, public `https://choir.news/health` reported `status=ok`,
`upstream=ok`, `vmctl_status=ok`, `vmctl_routing=enabled`, and proxy plus
sandbox build/deployed commit `63767a43673007aaca27e926c74dd6e9ee7093f3`.
The deployed lifecycle Playwright smoke passed, and browser-public
prompt-bar/VText/RunAcceptance smoke accepted RunAcceptanceRecord
`runacc-e6f3ae1cde0f9536c812` at `staging-smoke-level` for trajectory/run
`89ca3c23-3477-4e40-8ecb-1a738b3191ac` and VText document
`902ef3c2-e045-4acb-ad85-695ed0393e95`. Batch K narrows the restart-backlog
rewarm surface; it does not prove deployed kill/restart actor rewarm.
Batch L adds the first OS-process kill oracle for that gap:
`TestProcessRestartRewarmsCoagentAfterOSKill` starts a real child test process
with a running coagent activation against a persistent store, kills it with
`SIGKILL`, then boots a fresh child process over the same store. The recovery
process must passivate the killed activation and start a replacement activation
that carries both `worker_update_ids` and `work_item_ids`, while
`TrajectoryObligations` still reports the pending update and open work item.
This is local process evidence, not staging service-kill evidence, but it moves
the observer from seeded-row restart simulation to real process death and
fresh-process boot.
Batch M attempted to move from local process proof toward deployed restart
evidence and found a staging substrate blocker before running the intended
kill/restart probe. Public `https://choir.news/health` still reported
`status=ok`, `upstream=ok`, `vmctl_status=ok`, `vmctl_routing=enabled`, and
proxy/sandbox build plus deployed commit
`63767a43673007aaca27e926c74dd6e9ee7093f3`, but direct Node B service evidence
showed `go-choir-sandbox.service` had restarted 110 times. The journal repeated
`failed to listen on 127.0.0.1:8085: bind: address already in use`; `ss` showed
the port owned by a stray diagnostic process
`/var/lib/go-choir/services/sandbox/bin/sandbox -help` outside the active
systemd main process. This is a staging-proof blocker and a harness/ops
discipline finding: a binary help probe can accidentally start a second
sandbox, hold the runtime port, and force systemd restart loops while public
health may still look OK between restarts.
The immediate cleanup removed only the stray non-systemd process. A later
sample showed `go-choir-sandbox.service` active/running with main PID `42640`,
`NRestarts=117` unchanged through the watch window, and `ss` showing PID
`42640` as the sole listener on `127.0.0.1:8085`. Public health still reported
proxy/sandbox deployed commit `63767a43673007aaca27e926c74dd6e9ee7093f3`, and
the deployed adaptive lifecycle Playwright smoke passed. This restores staging
as a usable proof substrate, but it is not M3 restart-falsifier evidence.
Batch N then ran the first deployed SIGKILL probe against a live prompt/VText
trajectory, but the probe killed the host `go-choir-sandbox.service` rather
than the vmctl-routed user computer that owned the trajectory. The product path
created submission/trajectory `3e69d4ca-e629-450f-891f-ea3a21c795c3`, VText
document `ce4f4e4f-9cab-4d2f-a27b-c1b91d3db9ff`, owner
`9e4400f6-8101-4f71-a5d5-e18dcefe9155`, and initial loop
`cab77c12-c773-4784-a8c9-e529e48c71d4`; Trace observed conductor, super,
researcher, and VText before the kill. Host service PID `42640` was killed at
2026-06-13T10:11:17Z and systemd restarted it as PID `42992`
(`NRestarts` 117 -> 118); public health recovered and still reported deployed
commit `63767a43673007aaca27e926c74dd6e9ee7093f3`. The probe later failed
waiting for the trajectory to produce the expected `web_search`. Follow-up
route evidence explains why this cannot settle C3: vmctl listed an active
interactive computer for that owner at `http://10.200.9.2:8085`, while direct
host Dolt queries found zero rows for the owner, document, initial loop, and
trajectory in the host runtime store. This is host restart-under-load evidence
plus a route-target mismatch, not durable-actor rewarm proof for the user's
computer.
Batch O corrected the target by running a fresh prompt/VText trajectory inside
a throwaway vmctl-routed user computer, then calling internal vmctl `refresh`
for that exact owner/primary desktop. This force-killed and rebooted
Firecracker VM `vm-5d3ca0a2a3bdd8e8a402a598822fc4db`, moving the sandbox URL
from `http://10.200.10.2:8085` to `http://10.200.11.2:8085` and epoch 1 -> 2
while preserving the persistent data image. The guest boot log recorded
`runtime: passivated run 57ce8389-5482-4a25-aa85-419b1e6002d3 (was running)
after restart`, and public Compute Monitor reported the same current computer
active/reachable at epoch 2. The product proof still failed: VText revision
`a85d04bc-3032-4b02-8d67-74625bfc9151` had been written just before the refresh
with only the super update consumed; after restart the researcher appeared in
Trace as `passivated` at 10:28:26Z, but no researcher update reached VText, the
document stayed partial, and `TrajectoryObligations` showed zero pending updates
plus one unrelated open Wire publication work item. This is now a lifecycle
problem record: a killed spawned researcher activation can be passivated without
an associated durable assignment/backlog that reactivates it or keeps the
trajectory visibly waiting on that researcher result.
Batch P implements the spawned-work rewarm fix locally. `StartChildRun` now
records spawned researcher/super/vsuper/co-super objectives as live trajectory
work items assigned to the spawned durable agent, carries the `work_item_ids`
on the activation metadata, and marks those work items completed only when the
activation completes successfully. If the process dies first, the work item
stays open and the existing boot assigned-work sweep rewarms the actor. The
focused local proofs passed under the Nix dev shell:
`go test ./internal/runtime -run 'TestStartChildRunCompletesSpawnedWorkItem|TestProcessRestartRewarmsSpawnedChildWorkItemAfterOSKill|TestStartSweepsAssignedOpenWorkItemsAfterPassivation|TestProcessRestartRewarmsCoagentAfterOSKill' -count=1`
and
`go test ./internal/runtime -run 'TestSpawnMintsTrajectoryAndChildJoinsIt|TestVSuperCoSuperSlotReusedByTrajectorySlot|TestUpdateCoagentDeliveryRequiresSuccessfulActivation|TestUpdateCoagentDeliveryIgnoresStrayWorkerUpdateMetadata|TestStartRewarmsCoagentWithPendingUpdatesAndAssignedWork' -count=1`.
The broader runtime shard script also passed:
`nix develop -c scripts/go-test-runtime-shards`. This is not deployed
acceptance yet.
Batch Q reran the vmctl-routed deployed refresh proof after commit
`a9ef938d51d0f6a4d920393c0031d415d1709de8` reached staging. CI run
`27464702356`, deploy job `81184935254`, and FlakeHub publish run
`27464702377` succeeded; public health reported proxy and sandbox deployed at
`a9ef938d51d0f6a4d920393c0031d415d1709de8`. The vmctl target oracle was again
correct: owner `cdf41610-dfc5-4861-91a9-e7f293c65bf0` ran on VM
`vm-861e224b3619f023ec3b589d0fbe6af3`, refresh moved sandbox
`http://10.200.12.2:8085` to `http://10.200.13.2:8085`, and epoch 1 -> 2 on
the same deployed commit. The product predicate still failed. Direct
investigation against that VM showed the passivated researcher loop
`f8c76920-9dd0-46c8-afac-80dbae2a16a7` had `agent_profile=researcher` and
metadata `trajectory_id=bbe415d8-2a79-45a4-a692-7634d55dbf6b`, but no
`work_item_ids`; the trajectory's open obligations contained only a later Wire
publication work item and `pending_updates=0`. This narrows the problem:
the deployed VText `spawn_agent` surface did not persist the spawned researcher
work item that the direct `StartChildRun` regression expected.
Batch R adds a boot-time compatibility guard for that sharper shape. When boot
passivates an interrupted spawned researcher/super/vsuper/co-super activation,
it now synthesizes the missing assigned spawned-work item from the passivated
run metadata if one is absent, annotates the passivated activation with the
`work_item_ids`, and lets the existing assigned-work sweep rewarm the durable
agent. The focused local proofs passed under the Nix dev shell:
`go test ./internal/runtime -run 'TestStartSynthesizesSpawnedWorkItemForPassivatedChildWithoutBacklog|TestProcessRestartRewarmsSpawnedChildWorkItemAfterOSKill|TestStartChildRunCompletesSpawnedWorkItem|TestStartSweepsAssignedOpenWorkItemsAfterPassivation' -count=1`
and
`go test ./internal/runtime -run 'TestStartRewarmsCoagentWithPendingUpdatesAndAssignedWork|TestProcessRestartRewarmsCoagentAfterOSKill|TestSpawnMintsTrajectoryAndChildJoinsIt|TestVSuperCoSuperSlotReusedByTrajectorySlot|TestUpdateCoagentDeliveryRequiresSuccessfulActivation|TestUpdateCoagentDeliveryIgnoresStrayWorkerUpdateMetadata' -count=1`.
The broader runtime shard script also passed:
`nix develop -c scripts/go-test-runtime-shards`. This is not deployed
acceptance yet.
Batch S reran the vmctl-routed deployed refresh proof after commit
`2fe64f91c48f415ca48b484ca242be167765fe66` reached staging. CI run
`27465173151`, deploy job `81186246062`, and FlakeHub publish run
`27465173148` succeeded; public health reported proxy and sandbox deployed at
`2fe64f91c48f415ca48b484ca242be167765fe66`. The route-target oracle was again
correct: owner `1f77cd78-5a0d-440b-896e-e0031084f454` ran on VM
`vm-bfa54fb29cf43ce40fe79062955305e4`, refresh moved sandbox
`http://10.200.14.2:8085` to `http://10.200.15.2:8085`, and epoch 1 -> 2 on
the same deployed commit. The product predicate still failed. The final VText
revision text included a researcher-looking section, but its durable metadata
consumed only the super update; Trace showed the researcher activation
`3de3105b-6631-436b-ad6f-7dcb7612a6bd` passivated with no replacement. Direct
VM inspection showed the passivated researcher retained trajectory metadata and
`passivated_reason=runtime_restarted`, but still had no `work_item_ids`; the
trajectory had `pending_updates=0` and only the later Wire publication work
item open. This narrows the problem again: Batch R's boot-passivation synthesis
did not actually materialize a spawned-work item for the real deployed
VText-spawned researcher shape.
Batch T adds a second local recovery path for that sharper shape. Boot now
sweeps already-passivated spawned child activations after passivation and
before the normal pending-update/open-work sweeps, ensures a spawned child work
item exists, annotates the passivated run with `work_item_ids`, and hands the
item to the existing assigned-work reconciler. The `spawn_agent` tool-surface
test now also asserts that VText-spawned researchers carry a work item
immediately. Focused local proofs passed under the Nix dev shell:
`go test ./internal/runtime -run 'TestConductorCanSpawnVTextAndVTextCanSpawnResearcher|TestStartRewarmsAlreadyPassivatedSpawnedChildWithoutBacklog|TestStartSynthesizesSpawnedWorkItemForPassivatedChildWithoutBacklog|TestProcessRestartRewarmsSpawnedChildWorkItemAfterOSKill|TestStartChildRunCompletesSpawnedWorkItem|TestStartSweepsAssignedOpenWorkItemsAfterPassivation' -count=1`.
The broader runtime shard script also passed:
`nix develop -c scripts/go-test-runtime-shards`. This is not deployed
acceptance yet.
See "Lifecycle Inventory - 2026-06-13" below.

**next move:** commit and push Batch T, monitor CI/deploy, confirm staging build
identity, then rerun the vmctl-routed restart proof against `https://choir.news`.
Keep M3 open as a lifecycle cutover mission, not a deployment recovery mission.
The durable-actor restart falsifier remains the gate:
kill/restart or equivalent deployed evidence that cold actors rewarm from
durable backlog/open assigned obligations with zero stranded messages or
zero-obligation stalls. Batch T removes the local already-passivated spawned
child variant, but no continuation-level, promotion-level, or settlement claim
follows until the deployed vmctl-routed probe passes.
Cancellation's store-active fallback and `executeActivation` terminal run rows
are accepted for v0 as compatibility/audit surfaces, not ordinary
warm-residency or agent-liveness oracles.
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
| Boot recovery | Boot now passivates interrupted activations, sweeps already-passivated spawned child activations that still need assigned work, sweeps pending `worker_updates`, and sweeps live/open work items that already name an assigned durable agent. Batch K makes update-backlog cold rewarm also attach assigned open work items for every distinct pending-update trajectory in that actor's update batch, so the later assigned-work sweep cannot silently skip those obligations merely because the actor became resident. Batch L adds a local OS-kill oracle: a child process dies with a running activation in the store, and a fresh child process boots the same store and proves passivation plus rewarm. Batch P proves the same OS-kill shape for a spawned researcher whose only durable recovery input is the work item created by `StartChildRun`. Batch R adds a compatibility synthesis during boot passivation for interrupted spawned child activations that already carry trajectory metadata but missed `work_item_ids`; Batch S shows that synthesis still did not create a work item for the real deployed VText-spawned researcher. Batch T adds a post-passivation spawned-child sweep for that already-passivated no-work-item shape. `TrajectoryObligations` still exposes unassigned open obligations without selecting an actor. | Delete old recovery, port to sweep. | Continue shrinking boot recovery toward an explicit actor-residency/backlog table. Assigned work items are rewarm backlog; unassigned obligations need owner/supervisor routing and must stay observable rather than silently spawning an arbitrary actor. |
| Spawned researcher/coagent work | Batch O vmctl-refresh proof killed a user computer after a VText-requested child run started. Boot passivated the running child activation, but no pending update or assigned work item reactivated that researcher, and the trajectory stayed visibly waiting only on a later Wire publication work item. Batch P changes direct `StartChildRun` researcher/super/vsuper/co-super runs to create assigned trajectory work items up front and complete them only on successful activation completion; local process-kill proof shows boot rewarms a directly spawned researcher from that open work item with no pending updates. Batch Q shows the deployed VText-spawned researcher still lacked `work_item_ids` and no spawned researcher work item existed on the trajectory after vmctl refresh. Batch R locally proves boot can synthesize that missing assigned work item from the passivated spawned activation and rewarm the same durable researcher. Batch S falsifies the deployed version: the researcher run retained trajectory metadata and `passivated_reason`, but still lacked `work_item_ids`, and the trajectory exposed no spawned researcher work item. Batch T locally proves an already-passivated spawned researcher with no work item is swept, annotated, and rewarmed through the assigned-work reconciler; the VText `spawn_agent` tool-surface test now asserts immediate work-item creation. | Port spawned work to trajectory/work item plus activation shim. | Commit/push Batch T, then rerun deployed vmctl-routed refresh proof. |
| Run-memory compaction | `run_memory_entries` are still physically keyed by `loop_id`, but new tool-loop activations now seed an `actor_rewarm` compaction checkpoint from the latest prior inactive activation for the same `(owner_id, agent_id)` before appending the wake message. `executeWithToolLoop` still initializes `runMemoryManager`, compacts on thresholds/overflow, and `CompactRunMemory` is manual per-run. | Wrap in activation shim, then port to actor memory snapshot. | The v0 bridge makes compacted context available across activations without a schema migration. The durable target remains an actor memory snapshot plus log tail; `loop_id` should stay activation/run evidence, not the long-lived memory identity. |
| Update-woken delivery | `update_coagent` writes `worker_updates`; `wakeUpdatedCoagent` calls `reconcilePersistentSuperActor` / `reconcileUpdatedCoagentActor`; those create or reuse active runs and mark update IDs delivered at terminal run update. | Wrap in activation shim. | Cold delivery should activate from backlog; warm delivery should inject steering input at a step boundary. Delivery/incorporation should be tied to processing the update, not to a whole run ending. |
| Acceptance evidence | `RunAcceptanceRecord` has `trajectory_id` but still carries `loop_id`; `continuation-level` is gated by a `continued` checkpoint after promotion, and AGENTS.md says this is transitional until M4. | Port to trajectory/work item / activation evidence. | M3 evidence should prove activation/sweep/rewarm and no stranded messages. M4 re-points `continuation-level` formally; this mission must avoid claiming continuation-level from old run continuation evidence. |
