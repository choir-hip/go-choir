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

**Real artifact:** `executeRun` goroutine closures replaced by actor activation
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
  as the lifetime of an `executeRun` closure;
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
`executeRun` closure owns lifecycle; `recoverInterruptedRuns` blanket-fails
active work; cancel graph still depends on parent/run trees; ParentRunID still
used as control input; passivation/rewarm does not drive real LLM loop memory;
warm update steering at step boundaries missing; cold wake/sweep proof missing;
kill -9 restart falsifier missing; acceptance evidence still points at run
continuation/lifecycle state instead of trajectory/work-item/activation
evidence. Current V=8. Expected first pass should decrease V by at least 1 by
inventorying and classifying the control reads before code motion.

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

**next move:** inventory lifecycle control reads and tests:
`executeRun`, `recoverInterruptedRuns`, `CancelRunGraph`, `ParentRunID`,
active-run graph queries, boot recovery, run-memory compaction, update-woken
delivery, and acceptance evidence. Classify each as delete, rename to
spawned_by provenance, port to trajectory/work item, or wrap in the activation
shim. Then update this paradoc with the first concrete construct batch and
expected Delta V.

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
