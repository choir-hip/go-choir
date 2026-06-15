# Mission M3 - Lifecycle Cutover (cutover step 4) - v0

Source: `docs/mission-portfolio-2026-06-11.md` section M3. Program:
`docs/choir-rearchitecture-durable-actors-2026-06-11.md` (cutover step 4,
sections 2.1, 2.4, 3/R1/R2). Spec: `specs/actor_protocol.tla` (activate /
deliver / steer / passivate; especially atomic passivation and no lost wake).
Discipline: `skills/parallax/SKILL.md`. Predecessors: M1
(`docs/mission-trajectory-model-v0.md`) and M2
(`docs/mission-messaging-cutover-v0.md`) are settled.

Doctrine note (2026-06-13): legacy continuation acceptance-level references in
this cutover mission are transitional acceptance-language residue, not target
doctrine.

Recovery gates: M3.1 settled on 2026-06-14 and M3.2 settled on 2026-06-15.
M3 must resume as lifecycle cutover, not deterministic VText researcher
continuation and not direct-super ingress. Preserve the M3.1/M3.2 invariants:
VText remains the artifact control plane, semantic delegation is VText's choice,
decision rationale stays off-document, and prompt/source/article/mission ingress
enters VText-owned artifact state before downstream super work.

## Source Form

**Kind:** spine.

**Real artifact:** run-shaped goroutine closures replaced by actor activation
loops; `recoverInterruptedRuns` blanket-fail deleted (boot = cold actors +
sweep); cancel-by-trajectory replaces `CancelRunGraph`; legacy parent-run
fields become `spawned_by_run_id` provenance-only.

**Bridge conjecture (R1/R2):** activation/passivation/sweep semantics, already
proven at the protocol level (`actor_protocol.tla`) and package level
(`internal/actor` tests), survive contact with the real LLM loop.

**Falsifier:** kill -9 mid-activation under multi-agent load; on restart, sends
reactivate with correct memory and zero stranded messages.

**Edge (resource):** the LLM loop's streaming/tool machinery may resist the
clean step/loop/activation boundary; budget for a shim layer rather than
distorting the actor semantics.

**Settlement:** restart amnesia gone; the falsifier passes; legacy parent-run
control sites migrated with their features; acceptance evidence re-pointed.

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

**witness/spec (A/S):** live work is an activation over a durable agent
identity; `update_coagent` and assigned work activate cold actors or steer warm
actors at step boundaries; passivation drops residency without losing backlog;
boot reactivates from durable updates/open obligations; cancellation is by
trajectory/work item; legacy parent-run fields are provenance-only and
eventually become `spawned_by_run_id`.

**invariants / qualities / domain ramp (I/Q/D):**
- I: no blanket fail-on-restart, no parent/child control reads, no new wake
  primitive, and no direct-super ingress for ordinary VText-centered work.
- I: VText owns artifact control. M3 must not force VText to spawn researcher or
  super as a lifecycle proof precondition.
- I: passivation must not lose a wake or open assigned obligation. If an actor
  sleeps, backlog and open work must remain observable and rewarmable.
- I: prompt/VText smoke and `staging-smoke-level` acceptance are not M3 proof.
  M3 proof is lifecycle evidence: passivation, rewarm, delivered updates, open
  obligations, and no stranded messages after real restart/refresh.
- Q: compatibility shims may exist for v0, but settlement must name which run
  rows, active-run queries, and legacy parent-run fields remain audit-only.
- D: use focused local process-kill tests only to shape. Settlement requires
  deployed vmctl-routed user-computer restart/refresh proof on staging.

**variant (ranking function) V:** current V=3:
1. compile a fresh lifecycle proof harness/predicate over existing
   actor/backlog/work-item evidence, not forced researcher sequencing;
2. run the deployed vmctl-routed restart/refresh falsifier against
   `https://choir.news`, with correct target VM identity before/after refresh;
3. either settle from receipts or record the next real lifecycle substrate
   blocker, plus acceptance packet, rollback refs, heresy delta, and residual
   compatibility surfaces.

Prior M3 batches landed real lifecycle substrate: `executeActivation`,
passivation instead of blanket failure, pending-update and assigned-work sweeps,
trajectory-owned co-super slots, vSuper cancel authority limited by trajectory
slot, actor rewarm memory seeding, spawned-child work items, boot synthesis for
missing spawned work, and explicit VText requester metadata on reactivated
spawned work. The last M3 proof attempts got trapped in a false proxy: requiring
an explicit researcher branch before vmctl refresh. M3.1 and M3.2 settled that
trap. Do not resurrect it.

**budget:** one finishing pass for M3 proof and settlement if the harness can
use existing product/control evidence. If the proof reveals a new lifecycle
failure, document that first and fix only the lifecycle substrate, not VText
delegation policy.

**authority / bounds:** mutation class for the next implementation is `red`
because it touches vmctl, VText lifecycle evidence, run acceptance, and actor
restart semantics. Before code, name conjecture delta, protected surfaces,
admissible evidence, rollback path, and heresy delta. Apply Problem
Documentation First for any new staging/lifecycle failure.

**evidence packet:** focused tests for any touched runtime path;
`nix develop -c scripts/go-test-runtime-shards`; independent review before
settlement; push to `origin/main`; CI; Node B deploy with staging health
identity; deployed vmctl-routed restart/refresh proof; browser-public/product
control evidence of trajectory/work-item obligations before refresh and
passivation, rewarm/delivery/no-stranding after refresh; run acceptance
synthesis that does not claim old continuation or promotion acceptance levels.

**heresy delta:** discovered: late M3 turned a proof precondition into forced
appagent sequencing; M3.1/M3.2 repaired that. Introduced: none accepted.
Current M3 must repair only lifecycle heresy: restart amnesia, parent-tree
authority, or active-run liveness if the fresh proof exposes it.

**position / live conjectures / open edges:**
- C1 supported locally: activation loops can wrap the current LLM/tool loop
  without weakening M2 message/wake semantics.
- C2 supported locally: boot passivation plus sweeps can rewarm pending updates
  and assigned work after OS-process death.
- C3 testing: deployed vmctl-routed user-computer refresh must prove cold actors
  rewarm from durable backlog/open assigned obligations with no stranded updates
  or zero-obligation stalls.
- C4 active: legacy parent-run fields remain compatibility/audit lineage. They
  must not decide liveness, cancellation, slot ownership, or authority.
- Edge/compatibility: non-vSuper cancellation active-run fallback, terminal run
  rows, and physical `run_memory_entries.loop_id` remain accepted v0 shims only
  if final settlement names them and proves they are not warm-residency or
  authority oracles.
- Edge/successor: M3.2 accepted source/news/article route proof as residual
  product-surface debt. It does not block M3 lifecycle settlement, but do not
  claim source/news/article route proof from M3.

**next move:** build or update a deployed M3 lifecycle proof harness around the
current invariant: create/observe durable assigned work or pending update
backlog for a VText-owned trajectory, refresh the correct vmctl-routed user
computer, then prove passivation, rewarm, delivery, and no stranded obligations.
Do not require an exact researcher branch, exact tool order, direct-super
ingress, or deterministic VText continuation. If no current product path can
expose the required lifecycle evidence, record the exact proof-surface blocker
and stop before runtime changes.

**ledger file:** `docs/mission-lifecycle-cutover-v0.ledger.md`.

**version / lineage:** v0 compiled 2026-06-13 after M2 settled at
`794d28dd76ff00a2ae27c98a14dbce9e34834695`. M3.1 and M3.2 are now settled
gates. Successors gated on M3: M4 continuation deletion and M5 Wire settlement
falsifier.

**learning state:** M2 learning carries forward: no old/new dual model may
survive settlement; stale prompt/tool surfaces are blockers, not cleanup;
product surfaces are downstream falsifiers, not proof that the spine stands.
M3.1/M3.2 learning carries forward: VText is artifact control plane, not a
workflow runner.

**settlement:** not claimed. Required landing proof: commit, push, CI, Node B
deploy, staging health identity, deployed vmctl-routed restart proof, and an
acceptance/evidence packet whose lifecycle claim does not outrun the receipts.

## Suggested Goal String

```text
Use Parallax on docs/mission-lifecycle-cutover-v0.md. Treat it as M3 proper
after settled M3.1 and M3.2. Current status is working with V=3. Preserve Choir
Doctrine, AGENTS.md, docs/vtext-agentic-invariants-2026-06-13.md, and the
settled M3.1/M3.2 invariants: VText is Choir's artifact control plane, semantic
delegation is VText's choice, decision rationale stays off-document, ordinary
prompt/source/article/mission ingress enters VText-owned artifact state, and
super is downstream of VText request. Do not resurrect deterministic
edit_vtext -> spawn_agent, exact researcher branch requirements, prompt-bar
researcher routing, direct-super ingress, or prompt/VText smoke as M3 proof.

The live M3 objective is lifecycle cutover settlement: prove on deployed staging
that a vmctl-routed user computer can be refreshed/restarted while actor work
rewarms from durable backlog/open assigned obligations, updates are delivered,
and no stranded messages or zero-obligation stalls remain. Current V=3: 1)
compile a fresh lifecycle proof harness/predicate over existing
actor/backlog/work-item evidence, not forced researcher sequencing; 2) run the
deployed vmctl-routed restart/refresh falsifier against https://choir.news with
correct target VM identity before/after refresh; 3) settle from receipts or
record the next real lifecycle substrate blocker with acceptance packet,
rollback refs, heresy delta, and residual compatibility surfaces.

Start by reading the compact Parallax State and the Lifecycle Inventory, then
inspect current runtime proof surfaces. If a code change is needed, first record
the lifecycle problem in this paradoc/ledger. Mutation class is red: protected
surfaces include vmctl refresh, actor passivation/rewarm, trajectory/work-item
obligations, VText lifecycle evidence, run acceptance, and deployment routing.
Required verification for settlement: focused tests for touched runtime paths,
nix develop -c scripts/go-test-runtime-shards, independent review, push to
origin/main, CI, Node B deploy, staging health identity, deployed vmctl-routed
restart proof, and acceptance synthesis that does not claim old continuation or
promotion acceptance levels. If no browser-public/product-control path can
expose the needed lifecycle evidence, record the exact proof-surface blocker
and stop without weakening the invariant.
```

## Historical Parallax State - Through 2026-06-13

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
Batch U reran the vmctl-routed deployed refresh proof after commit
`aba8674961ca4c9f9f557bb713c323874253437f` reached staging. CI run
`27465708076`, deploy job `81187644952`, and FlakeHub publish run
`27465708082` succeeded; public health reported `status=ok` with proxy and
sandbox deployed at `aba8674961ca4c9f9f557bb713c323874253437f`. The
route-target oracle was again correct: owner
`80beba93-9a74-4ecb-bc70-f3a21e7005d2` ran on VM
`vm-f7aea0d3796d4f367539a0ba8011f955`, refresh moved sandbox
`http://10.200.16.2:8085` to `http://10.200.17.2:8085`, and epoch 1 -> 2 on
the same deployed commit. The product predicate still failed. The final VText
revision `858d2821-077d-486e-96b9-f4c4f970a2a6` was written after refresh, but
it consumed only the super worker update and explicitly left the researcher
finding pending. Trace showed researcher agent
`17a9b254-fc2c-429e-829a-acc3a119c362` in `passivated` state with no researcher
update consumed by VText by the probe deadline. Direct VM reads confirmed the
super activation `48fcee1d-0a0a-4b89-9eb8-61c230ece891` completed with
`request_source=update_coagent` and one `worker_update_ids` entry, and the
post-refresh VText activation `f82ce157-607b-4d7c-93d0-c547cbf04948` completed
from the same trajectory/channel after consuming only the super update. This
narrows the problem from "route target wrong" and "Batch T not deployed" to:
the deployed vmctl refresh still leaves the VText-spawned researcher stranded
as passivated or otherwise non-delivering, so the local already-passivated
spawned-child sweep is still missing some real product-path shape.
Batch V closes the local requester-route half of Batch U. VText-spawned
researcher/super/vsuper/co-super work now records explicit requester metadata
when the parent is VText (`requested_by_profile=vtext`,
`requested_by_agent_id=vtext:<doc>`, and `requested_by_run_id=<vtext loop>`),
persists that metadata into the spawned work item, and restores it on
trajectory-work-item-sweep reactivation. This avoids relying on `ParentRunID`
as the only delivery target after a cold work-item rewarm creates a fresh
activation with no parent row. The already-passivated and boot-passivated
spawned-child regressions now assert the replacement activation carries the
VText requester route. Focused local tests passed under the Nix dev shell:
`go test ./internal/runtime -run 'TestStartRewarmsAlreadyPassivatedSpawnedChildWithoutBacklog|TestStartSynthesizesSpawnedWorkItemForPassivatedChildWithoutBacklog|TestProcessRestartRewarmsSpawnedChildWorkItemAfterOSKill|TestStartChildRunCompletesSpawnedWorkItem|TestStartSweepsAssignedOpenWorkItemsAfterPassivation|TestConductorCanSpawnVTextAndVTextCanSpawnResearcher' -count=1`.
The adjacent coagent/spawn/slot/delivery sweep also passed:
`go test ./internal/runtime -run 'TestStartRewarmsCoagentWithPendingUpdatesAndAssignedWork|TestProcessRestartRewarmsCoagentAfterOSKill|TestSpawnMintsTrajectoryAndChildJoinsIt|TestVSuperCoSuperSlotReusedByTrajectorySlot|TestUpdateCoagentDeliveryRequiresSuccessfulActivation|TestUpdateCoagentDeliveryIgnoresStrayWorkerUpdateMetadata' -count=1`.
The broader runtime shard script passed:
`nix develop -c scripts/go-test-runtime-shards`. Batch V was then committed as
`17b70e70b03740f2502a27e1c8694c1925ba618c`, pushed to `origin/main`, and
deployed. CI run `27466179291`, deploy job `81188874173`, and FlakeHub publish
run `27466179269` all succeeded; public health reported `status=ok` with both
proxy and sandbox deployed at
`17b70e70b03740f2502a27e1c8694c1925ba618c`.

Batch W reran the deployed vmctl proof against `17b70e70`, but it did not reach
the restart/recovery phase. Probe output
`/tmp/m3_vmctl_refresh_probe.17b70e70.out.json` used marker
`M3_VMCTL_REFRESH_1781352215222`, owner
`09b7a13c-edea-4aef-a732-2de4690316be`, submission/trajectory
`4f1dffd9-d4f8-4c5b-b5f5-7961baad0ea7`, VText document
`c84b154f-006c-4211-b539-952c0038775b`, and VM
`vm-8b2fc425bf1933fdc178e0ac1ea8ad62` at
`http://10.200.18.2:8085`, epoch 1. The probe timed out waiting for Trace to
show conductor, VText, researcher, and super before refresh; Trace showed only
conductor, super, and VText. Direct trace evidence showed conductor created the
document, sent the assignment to `super`, `super` wrote the verification note
and called `update_coagent`, and VText edited revision
`8dea92b7-0267-4f3a-8697-853744863319`. No researcher actor, researcher
delegation, or researcher worker update existed on that trajectory by the
240-second pre-refresh deadline. Because no vmctl refresh was issued, this is
not a falsification of Batch V's requester-route rewarm fix; it is a new
product-path orchestration problem for the acceptance prompt: a request that
explicitly asks for researcher evidence can complete the VText document through
super-only work and never spawn the researcher branch.
Batch X closes that local pre-refresh orchestration hole. The deterministic
prompt-bar VText route still sends broad operational/code/product-path prompts
to persistent super first, but explicit researcher requests now bypass that
shortcut and start an initial VText revision run instead. The initial VText
tool choice likewise stays on `edit_vtext` for explicit researcher requests so
VText can write a brief working revision and open researcher work; the VText
prompt now names the obligation not to satisfy an explicit researcher request
by asking only super. New regressions prove that a prompt shaped like the
vmctl probe starts VText, not persistent super, while the existing operational
proof prompt still routes to persistent super. Focused prompt-bar/VText tests,
focused lifecycle/restart tests, and the runtime shard script passed under the
Nix dev shell. This is local construct evidence only until the commit is
pushed, deployed, and the vmctl proof reaches the refresh phase.
Batch Y reran the deployed proof after Batch X reached staging at commit
`2f64ac09807052833bd0be1b5008ebaa25931db7`. CI run `27466513026`, deploy job
`81189760069`, and FlakeHub run `27466513005` succeeded; health reported both
proxy and sandbox at `2f64ac09807052833bd0be1b5008ebaa25931db7`. The deployed
probe still failed before vmctl refresh. Probe output
`/tmp/m3_vmctl_refresh_probe.2f64ac09.out.json` used marker
`M3_VMCTL_REFRESH_1781353148536`, owner
`fea954a8-cd64-4266-9297-a948707f3143`, submission/trajectory
`fc7ae24a-62ae-49bf-b0dd-d740dfc202ba`, VText document
`2edcb9fd-0f88-468e-ba10-5fd8a68b86cb`, and VM
`vm-8bef0f8e83c462e8b4479b86226e42ce` at
`http://10.200.19.2:8085`, epoch 1. Trace again contained only conductor,
super, and VText by the pre-refresh deadline. Direct VM reads showed
`initial_loop_id=e354a0f4-f065-40b9-93ef-9317e9e528bc` was still a `super`
activation with `request_source=update_coagent`, and the later VText activation
`f2ec5260-596f-4aad-9fda-0856ceb63c18` came from super's worker update. No
researcher actor or researcher update appeared. This falsifies the Batch X
local assumption that text-pattern detection alone would reliably bypass the
persistent-super initial handoff in staging.
Batch Z made that route signal durable at the prompt-bar boundary. Commit
`93fc3ada07e4a5e3c94169cb92c6daaee4ac46d4` stamped
`explicit_researcher_request=true` on explicit researcher prompt-bar
submissions, copied it into the initial user VText revision, preserved it
through VText revision metadata, and used it in the VText initial tool-choice
and prompt guidance paths. Local focused prompt-bar/VText tests, focused
lifecycle/restart tests, and `scripts/go-test-runtime-shards` passed. CI run
`27466853268`, deploy job `81190664423`, and FlakeHub run `27466853276`
succeeded; staging `/health` reported proxy deployed commit
`93fc3ada07e4a5e3c94169cb92c6daaee4ac46d4` with `vmctl_status=ok` and
`vmctl_routing=enabled`.
The deployed proof still failed before vmctl refresh, but with a different
shape. Probe output `/tmp/m3_vmctl_refresh_probe.93fc3ada.out.json` used
marker `M3_VMCTL_REFRESH_1781354046143`, owner
`476df81b-1b02-4864-b75d-779316bbbe3f`, submission/trajectory
`b83b80b0-f81c-4c12-9fd2-3e16f4a42b32`, VText document
`788b2f9b-687b-4718-a665-42d2d6c75ae1`, and VM
`vm-516253a97cfa407b1424533676f1b349` at `http://10.200.20.2:8085`, epoch 1.
This time `decision.initial_loop_id=acf35ca3-b81e-4485-b744-4761160413ff`
was the initial VText route, not a persistent-super route. The sandbox was
healthy at commit `93fc3ada07e4a5e3c94169cb92c6daaee4ac46d4`, with no active
runs after failure. A diagnostic trace read from the owner-routed sandbox
showed only conductor, super, and VText agents; `finding_count=0`; VText
invoked `edit_vtext` and completed; super invoked file/search/read/write tools
and `update_coagent`; no researcher agent, `spawn_agent` moment, researcher
finding, or researcher worker update appeared by the 240-second pre-refresh
deadline. This falsifies the new local assumption that getting the prompt onto
the VText initial route is enough for an explicit researcher request to open a
researcher branch before the super branch completes the document.
Batch AA tried to turn that obligation into a tool-result continuation hint.
Commit `d74e60617db0b4d48daadbb6286b72f7fa326504` made `edit_vtext` return
`next_required_tool=spawn_agent` with researcher arguments when an initial
user-authored VText revision carries explicit researcher intent. Local focused
VText, prompt-bar, lifecycle, restart, and runtime-shard checks passed. CI run
`27467234086`, deploy job `81191719843`, and FlakeHub run `27467234071`
succeeded; staging `/health` reported proxy build and deployed commit
`d74e60617db0b4d48daadbb6286b72f7fa326504`, `vmctl_status=ok`, and
`vmctl_routing=enabled`.
The deployed proof still failed before vmctl refresh. Probe output
`/tmp/m3_vmctl_refresh_probe.d74e6061.out.json` used marker
`M3_VMCTL_REFRESH_1781355099704`, owner
`6bc028a5-ad5e-4810-aef6-9400c39c71cb`, submission/trajectory
`535c54c2-cd4f-4541-9e26-1c8149fb20aa`, VText document
`76468684-39bf-4e85-b3ab-8fbc46c8bc87`, and VM
`vm-d25cfca4740e90695d25b7802302afc6` at `http://10.200.21.2:8085`, epoch 1.
The sandbox was healthy at commit
`d74e60617db0b4d48daadbb6286b72f7fa326504`. The trace contained conductor,
super, and VText only; `delegation_count=0`, `finding_count=0`, and
`message_count=0`. Super completed the artifact path and updated VText; VText
then invoked `edit_vtext`, created revision `53543c7c`, received the tool
result, made one more provider call, and completed without `spawn_agent`. This
falsifies the assumption that a required-continuation hint in the tool result is
strong enough to enforce an explicit researcher obligation in deployed product
traffic.
Batch AB moved the obligation predicate from a user-authored-base-only check to
explicit researcher intent plus no existing researcher participation on the same
document trajectory. Commit `15301fd00f59085d9b277e893cd4ae32cd19d555`
passed focused VText continuation tests, prompt-bar/lifecycle tests, tool-loop
checks, and `scripts/go-test-runtime-shards`; CI run `27467612158`, deploy job
`81192724078`, and FlakeHub run `27467612166` succeeded; staging `/health`
reported proxy build and deployed commit
`15301fd00f59085d9b277e893cd4ae32cd19d555`, `vmctl_status=ok`, and
`vmctl_routing=enabled`.
The deployed proof still failed before vmctl refresh. Probe output
`/tmp/m3_vmctl_refresh_probe.15301fd0.out.json` used marker
`M3_VMCTL_REFRESH_1781356093374`, owner
`b2b5bdb4-a6e8-486e-8a24-7d44ce87f33a`, submission/trajectory
`4355083f-28da-489e-92be-4b078ba00102`, VText document
`579d11f1-5bbd-4127-881f-420d4a0d27ce`, and VM
`vm-5ecdc6656b5fd9e1f4e58e73d99a996b` at `http://10.200.22.2:8085`, epoch 1.
The route was again not the desired initial VText edit route:
`decision.initial_loop_id=d9c01209-82e2-4e0b-8af4-527db3cdae16` was a super
run. The only VText run, `70149a0c-dbb6-4698-8182-0477d8d7ce6e`, was spawned
from that super run after `update_coagent`. Trace contained conductor, super,
and VText only; `delegation_count=0`, `finding_count=0`, and
`message_count=0`. Moment detail for VText `edit_vtext` result
`19eec8ba-a1c3-4fe2-bb2a-8b46f5c467bd` showed output with only
`base_revision_id`, `doc_id`, `revision_id`, and `status=stored`, with no
`next_required_tool`. A duplicate `edit_vtext` call was also returned in the
same VText turn and was skipped by the existing duplicate-edit guard. This
falsifies the Batch AB assumption that the explicit researcher signal is
available to the worker-woken VText run that follows the super update.
Batch AC derived explicit researcher intent from durable VText base revision
content/metadata instead of only the worker-woken VText run prompt. Commit
`5f38c437f0a9541feaf03a00c41e76e3ae2f0852` passed focused VText
continuation tests, prompt-bar/lifecycle tests, required-tool tests,
`scripts/go-test-runtime-shards`, CI run `27468043199`, deploy job
`81193874119`, and FlakeHub run `27468043198`; staging `/health` reported
proxy build and deployed commit
`5f38c437f0a9541feaf03a00c41e76e3ae2f0852`, `vmctl_status=ok`, and
`vmctl_routing=enabled`. The deployed proof did not reach the prior researcher
or vmctl-refresh predicate. Probe output
`/tmp/m3_vmctl_refresh_probe.5f38c437.out.json` used marker
`M3_VMCTL_REFRESH_1781357266995` and failed waiting 180 seconds for
`[data-desktop][data-authenticated="true"][data-desktop-ready="true"]`. Compute
Monitor reported the current primary computer active, but runtime health was
unavailable and the persistent data image was 100% full
(`used_bytes=17179869184`, `avail_bytes=0`, `warning=true`, `critical=true`).
This is a new staging substrate blocker, not evidence for or against the Batch
AC researcher-intent fix: the owner desktop did not become product-ready enough
to submit the proof prompt.
Follow-up VM inspection showed the reported full image was host-side sparse
file size, not guest filesystem saturation: direct guest `/health` for
`vm-5e6d1e851839401dc3230e849623940b` later reported `status=ready`,
`runtime_health=ready`, and guest persistent disk `used_percent=6.88`.
Rerunning the same deployed proof against `5f38c437` reached the product path
and falsified the Batch AC code assumption. Probe output
`/tmp/m3_vmctl_refresh_probe.5f38c437.retry1.out.json` used marker
`M3_VMCTL_REFRESH_1781357892473`, owner
`498e1996-5760-4f92-a5ea-00cc768f3529`, submission/trajectory
`09dbcd65-3f14-4502-8a9e-14e42f531984`, VText document
`e7d7119d-40ee-4db3-b109-a2f72807ef2e`, and VM
`vm-a5e22e5bd9662584b90c8372bcf609fc` at `http://10.200.39.2:8085`,
epoch 1. Compute Monitor and direct VM health were ready, guest disk was
healthy (`used_percent` about 6.07), and the prompt submitted. The proof failed
before vmctl refresh waiting for roles `conductor`, `vtext`, `researcher`, and
`super`; Trace contained only conductor, super, and VText. The VText run
consumed only the super worker update. The final VText revision preserved
`Researcher finding: pending`, and both the user base revision and appagent
revision metadata still carried the original `seed_prompt` with `Ask
researcher...`. Trace logs showed the first `edit_vtext` result returned only
`base_revision_id`, `doc_id`, `revision_id`, and `status=stored`, with no
`next_required_tool`; the duplicate `edit_vtext` was skipped by the existing
duplicate-edit guard. This means the durable-base researcher intent existed in
the deployed document/revision data, but the tool result still did not attach
the deterministic researcher continuation.
See "Lifecycle Inventory - 2026-06-13" below.

**next move:** remain blocked behind
`docs/mission-lifecycle-cutover-m3.1-v0.md` until M3.1 settles or names
accepted successor edges. Do not debug or harden deterministic VText researcher
continuation. After the recovery gate clears, rerun the deployed vmctl-routed
restart proof against `https://choir.news` with a lifecycle evidence predicate:
kill/restart or equivalent deployed evidence that cold actors rewarm from
durable backlog/open assigned obligations with zero stranded messages or
zero-obligation stalls. Batch V removes the local requester-route hole for
work-item reactivated VText-spawned children, but Batch AC did not remove the
deployed explicit-researcher continuation hole. No continuation-level,
promotion-level, or settlement claim follows until the deployed vmctl-routed
probe passes.
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
| Boot recovery | Boot now passivates interrupted activations, sweeps already-passivated spawned child activations that still need assigned work, sweeps pending `worker_updates`, and sweeps live/open work items that already name an assigned durable agent. Batch K makes update-backlog cold rewarm also attach assigned open work items for every distinct pending-update trajectory in that actor's update batch, so the later assigned-work sweep cannot silently skip those obligations merely because the actor became resident. Batch L adds a local OS-kill oracle: a child process dies with a running activation in the store, and a fresh child process boots the same store and proves passivation plus rewarm. Batch P proves the same OS-kill shape for a spawned researcher whose only durable recovery input is the work item created by `StartChildRun`. Batch R adds a compatibility synthesis during boot passivation for interrupted spawned child activations that already carry trajectory metadata but missed `work_item_ids`; Batch S shows that synthesis still did not create a work item for the real deployed VText-spawned researcher. Batch T adds a post-passivation spawned-child sweep for that already-passivated no-work-item shape. Batch U shows the deployed product path still leaves a VText-spawned researcher passivated/non-delivering after correct vmctl refresh. Batch V restores explicit VText requester metadata on work-item-sweep activations. `TrajectoryObligations` still exposes unassigned open obligations without selecting an actor. | Delete old recovery, port to sweep. | Continue shrinking boot recovery toward an explicit actor-residency/backlog table. Assigned work items are rewarm backlog; unassigned obligations need owner/supervisor routing and must stay observable rather than silently spawning an arbitrary actor. |
| Spawned researcher/coagent work | Batch O vmctl-refresh proof killed a user computer after a VText-requested child run started. Boot passivated the running child activation, but no pending update or assigned work item reactivated that researcher, and the trajectory stayed visibly waiting only on a later Wire publication work item. Batch P changes direct `StartChildRun` researcher/super/vsuper/co-super runs to create assigned trajectory work items up front and complete them only on successful activation completion; local process-kill proof shows boot rewarms a directly spawned researcher from that open work item with no pending updates. Batch Q shows the deployed VText-spawned researcher still lacked `work_item_ids` and no spawned researcher work item existed on the trajectory after vmctl refresh. Batch R locally proves boot can synthesize that missing assigned work item from the passivated spawned activation and rewarm the same durable researcher. Batch S falsifies the deployed version: the researcher run retained trajectory metadata and `passivated_reason`, but still lacked `work_item_ids`, and the trajectory exposed no spawned researcher work item. Batch T locally proves an already-passivated spawned researcher with no work item is swept, annotated, and rewarmed through the assigned-work reconciler; the VText `spawn_agent` tool-surface test now asserts immediate work-item creation. Batch U falsifies the deployed version again: after the local sweep reached staging, VText still consumed only super, while Trace showed the researcher agent passivated. Batch V adds explicit requester-route metadata so a reactivated researcher can report back to VText without relying on `ParentRunID`. Batch W shows the deployed proof prompt can complete through conductor -> super -> VText without spawning researcher at all, so the vmctl restart oracle did not execute. Batch X locally makes explicit researcher prompts bypass the persistent-super initial shortcut, but Batch Y shows staging still set `initial_loop_id` to super and skipped researcher. Batch Z persists explicit researcher intent at prompt-bar submission and staging now sets `initial_loop_id` to VText, but the VText route still completes through super-only work with no researcher agent or finding. Batch AA adds a model-visible required-continuation hint to `edit_vtext`, but staging still lets VText complete after the edit without `spawn_agent`; the trace has no delegation, finding, or worker update. Batch AB allows explicit researcher obligations after non-user bases when no researcher exists on the trajectory, but staging still routes through initial super and the worker-woken VText `edit_vtext` result lacks `next_required_tool`, indicating the explicit researcher signal did not reach that run. Batch AC locally derives researcher intent from durable VText base revision content/metadata. The first deployed Batch AC probe hit a desktop-readiness substrate blocker before prompt submission, and later VM inspection showed the "full" image was a host sparse-file metric rather than guest disk saturation. The rerun reached VText and still produced an `edit_vtext` result without `next_required_tool` even though the durable base revision and revision metadata contained `Ask researcher...`; Trace had conductor, super, and VText only. M3.1 reclassified this exact-role precondition as proxy capture rather than a lifecycle requirement. | Port spawned work to trajectory/work item plus activation shim. | Do not force VText researcher delegation. After M3.1 clears the regression, prove spawned/coagent lifecycle through trajectory/work-item evidence: open assigned work before refresh, passivation/rewarm after refresh, delivered updates, and no stranded obligations. |
| Run-memory compaction | `run_memory_entries` are still physically keyed by `loop_id`, but new tool-loop activations now seed an `actor_rewarm` compaction checkpoint from the latest prior inactive activation for the same `(owner_id, agent_id)` before appending the wake message. `executeWithToolLoop` still initializes `runMemoryManager`, compacts on thresholds/overflow, and `CompactRunMemory` is manual per-run. | Wrap in activation shim, then port to actor memory snapshot. | The v0 bridge makes compacted context available across activations without a schema migration. The durable target remains an actor memory snapshot plus log tail; `loop_id` should stay activation/run evidence, not the long-lived memory identity. |
| Update-woken delivery | `update_coagent` writes `worker_updates`; `wakeUpdatedCoagent` calls `reconcilePersistentSuperActor` / `reconcileUpdatedCoagentActor`; those create or reuse active runs and mark update IDs delivered at terminal run update. | Wrap in activation shim. | Cold delivery should activate from backlog; warm delivery should inject steering input at a step boundary. Delivery/incorporation should be tied to processing the update, not to a whole run ending. |
| Acceptance evidence | `RunAcceptanceRecord` has `trajectory_id` but still carries `loop_id`; `continuation-level` is gated by a `continued` checkpoint after promotion, and AGENTS.md says this is transitional until M4. | Port to trajectory/work item / activation evidence. | M3 evidence should prove activation/sweep/rewarm and no stranded messages. M4 re-points `continuation-level` formally; this mission must avoid claiming continuation-level from old run continuation evidence. |
