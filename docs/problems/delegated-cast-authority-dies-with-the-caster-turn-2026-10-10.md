# A management cast dies when the management turn ends, and a second cast is refused (2026-10-10)

Status: documented, not fixed. Mutation class of the fix: `red` (engineering
assignment authority).

## Evidence

Minesweeper demo rerun on staging at dc9a1cba (test computer
computer-abd38983…, trajectory 8fd69969…, all-agents traces in the session
scratchpad under traces/ms3; guest console log on Node B for
vm-8cac78e5…). The two earlier fixes held: management's cast cell ran once
(no duplicate tool calls), and the assignment was not reaped by the work
wake. But no engineering run ever started.

1. **The first cast is cancelled at bind.** Management run f9be6e42 staged
   `choir.Cast("engineering", …)` at 19:07:08 (receipt `rlm:cast:1`),
   reported to Texture, and ended its turn, as the management prompt tells
   it to. The run's state is `completed`. At 19:07:24 the deferred spawn
   wake reached bind, and the assignment was cancelled with:
   `bind assigned Engineering activation: delegated cast: caster run is not
   live: co-super assignment invalid transition`. The cancel report reached
   management as a `blocker` update.
2. **Every later cast is refused at admission.** Management runs baa55da4
   (19:15) and 8d6408d7 (19:24) each staged a new cast and got
   `delegated cast admission: delegated cast: caster holds multiple open
   work items`. Each Texture `open_persistent_super` control opens a fresh
   management work item, and nothing closes the earlier ones.
3. **The model then drew a wrong conclusion.** Management told Texture "the
   execution request is already covered by an existing open Engineering work
   item", which was not true. The belief came from the two errors above, not
   from disobedience.

## Cause (from source)

- `requireEngineeringDelegatedParentAuthority`
  (`internal/store/engineering_assignments.go`) requires, for every non-historical
  transition (open, bind, activation, progress), that the caster's run is in
  `pending`, `running` or `passivated`, and that the caster agent's
  `ActiveRunID` is empty or equal to the casting run. Management runs end in
  `completed` within seconds of the cast. The spawn is deliberately deferred
  out of the cell reducer, so it always runs after the casting turn has
  ended. A delegated assignment can therefore only progress while the
  casting turn is still alive, and a later management turn breaks the
  `ActiveRunID` check as well.
- `openDelegatedCastAssignment`
  (`internal/agentcore/engineering_assignment_runtime.go`) derives the parent
  work item by scanning for the caster's open work items on the trajectory,
  and refuses when there is more than one. The casting run already names its
  exact work item in its metadata (`lifecycle_work_item_id`, also
  `lifecycle_control_bindings[].target_work_item_id`).

## Clustering assessment (CLAUDE.md, Root Cause Clustering)

This is the third delegated-cast failure documented today, after
`delegated-cast-reaped-before-spawn-2026-10-10.md` and the duplicate cast
from `responses-stream-duplicates-tool-calls-2026-10-10.md`. The first and
this one share one cause: **delegated-cast authority is anchored on
turn-scoped facts (a live run, a unique open work item, a reconciler's view
of "unbound") instead of the durable objects that outlive a turn.** The
persistent management desk is long-lived, but its runs are short turns, and
each Texture request adds a work item. Any check that needs "the caster's
turn is still running" fails for a desk whose turn ends before the deferred
spawn.

Substrate-level direction, not a symptom patch:

- Authority for every post-open transition rests on durable objects: a live
  trajectory, the persistent management agent (non-lifecycle, management
  profile), the commitment control the cast minted and that agent authored,
  and the **open parent work item**. The casting run must exist and be bound
  to the trajectory, but its liveness is checked only at admission, when it
  is the turn that is staging the cast.
- The caster agent's `ActiveRunID` must not have to equal the casting run
  after admission. A new management turn is not a revocation.
  Revocation stays explicit: management's `CancelAssignment`, the parent work
  item closing, or the trajectory ending.
- Parent work is the casting run's own `lifecycle_work_item_id`, not a scan.

No unwired replacement exists. The document-cast and run-parent authority
paths have the same run-liveness shape, but their parents (owner revisions,
engineering desk runs) have not failed this way on staging, so they stay out
of scope.

## Belief state

Before: the delegated cast path was blocked only by the reaper. After: the
reaper fix holds, and delegated casts still cannot reach a capsule because of
the authority anchor. Remaining error field: whether the persistent
management run should passivate instead of completing (it would satisfy the
current check, but it hides the turn/authority mismatch rather than
removing it).
