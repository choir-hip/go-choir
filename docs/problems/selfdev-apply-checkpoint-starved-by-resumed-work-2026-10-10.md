# Self-development apply: the checkpoint never sees a quiet computer (2026-10-10)

Found by the seventh Gate 2 reality rerun (M11 probe) on staging, build
2a16a6db, disposable `computer-520c29ba…` (VM `vm-1a9c51e0…`), operation
`selfdev-72caae58…`. Written 01:19Z, while the probe is still waiting.
Problem first; no fix in this commit. Mutation class of a fix: red
(checkpoint, run lifecycle after a planned restart).

## How far it got

For the first time since S2: implementation froze a candidate
(01:07:18Z), the independent verifier recorded a **pass** (the 3e69567b
mirror fix held), the operation reached `awaiting_approval`, the probe
approved it, and materialization began with a planned restart
(`restart kind=planned reason=self_development_apply`, 01:13:51Z).

## What happens next (guest console)

- The verifier run (`run:assignment-f59a636d…`) was still iterating
  after recording its verdict (as in rerun 5). The apply restart cut it;
  boot passivated it and its assignment was cancelled; the cancel report
  (`blocker`, 01:13:54Z) reached Texture (the 028446a5 binding fix lets
  it through) and a Texture supervision run (`280c9590…`) started and
  was still iterating at 01:18Z (iteration 34+).
- Every ~47 s from 01:14:41Z the materializer retries and fails:
  `self-development checkpoint: replay completeness: reconstruct event
  chain: computer event projection repair required`, with the replayed
  head 10–24 events behind the platform head (local seq 1368 → 1541,
  platform 1392 → 1551 across retries).

## Cause (code reading)

`ReplayCompleteness` (`agentcore/replay_completeness.go`) replays the
chain into a disposable workspace and then requires the platform head to
equal the replayed head (`computerevent/appender.go`
`ReconstructInto`), and it separately refuses if the live head moves
during the probe. It is designed for a quiet computer. After a planned
restart, pre-boot work resumes by the owner's rule ("if we're rebooting
to do an update … we can resume work"), and any desk that appends events
during the checkpoint starves it.

Two contributing facts: the verifier keeps working after its verdict
(so the apply restart always cuts a live run), and a cancel report now
wakes a Texture supervision turn.

## Fix directions (to decide; red)

- A: the materializer holds new desk activations (the actor dispatcher)
  from the planned-restart boot until the checkpoint and route
  projection finish, then releases them. Work still resumes after the
  update, as the owner's rule says; it just waits for the checkpoint.
- B: the checkpoint replays up to a head captured at its start and
  compares the live state at that head, tolerating later appends. Larger
  change to a protected verifier.
- Also: the verifier's run ends once it has recorded its verdict, so the
  apply restart does not cut a live verifier.

Leaning A (smallest, keeps the checkpoint's quiet-computer contract).

## After the computer went quiet (01:25Z)

The Texture supervision run hit its tool-loop budget at 01:19:40Z. The
next retries show the checkpoint stepping through three failures:

1. 01:20:29 `projection base refused: tail application is not
   exactly-once (1680 events, [1,1680], want (0,1673])`: the budget
   failure's own events landed during that replay.
2. 01:21:24 onward, with the head now stable: `replay is ineligible:
   reconstructed projection is not equivalent to live state`.

The replay-completeness report (read-only, owner-bound guest route,
01:24Z) names two differing keys: `dolt:texture:table:og_objects` and
the content root that hashes it. No missing tables, no schema drift, run
memory equal. So some object-graph rows on this computer were written
outside the event chain, and replay cannot reproduce them.

What is new on this computer compared with the September 29 pass: boot
passivated a run that was still working at the apply restart (the
verifier), its assignment was cancelled, and a Texture run failed on its
tool-loop budget. Hypotheses, not findings:

- H1: boot passivation's `store.UpdateRun` (`agentcore/runtime.go`)
  writes the run object without a reducer event.
- H2: the tool-loop budget failure path writes the run's terminal state
  directly.
- H3: the assignment cancel at restart writes outside the chain.

The report hashes whole tables, so it cannot say which rows differ.
Next: a row-level og_objects diff in the replay-completeness report (by
object kind and key), then name the writer. The operation stays in
`materializing`, retrying.

## Fix plan A (01:31Z; red, ceremony)

- Conjecture delta: work may resume after an update restart (owner
  rule), but not until the apply's checkpoint has read a still chain.
- Change: after a boot whose planned-restart marker is
  `self_development_apply`, the actor handler defers work-kind
  occurrences (`restartWorkKinds`: dispatches, coagent results, channel
  messages, owner revisions, assigned work, spawn and resume deadlines)
  with `actor.ErrDeferUnprocessed` while a self-development operation is
  `materializing`, for at most 10 minutes after boot. Cancels, fail-closed
  deadlines and `selfdev_materialization_retry` are never held. After
  the operation leaves `materializing`, or the 10 minutes pass, work
  proceeds as before. The bound keeps deferrals far below the
  dispatcher's 64-deferral poison limit, so a stalled apply delays work
  and never destroys it.
- Failure modes pinned in a handler test: held while materializing; not
  held when nothing is materializing; not held after the window; cancels
  and the materializer's retry never held; crash boots unchanged; a
  planned platform update boot never held.
- Protected surfaces: run lifecycle after a planned restart (red).
  Admissible evidence: the next M11 rerun shows no desk turn between the
  apply restart and the checkpoint. Rollback: git revert. Heresy delta:
  discovered "resume during apply"; repaired only on staging proof.

## Rerun 8 on 5704ace6: the hold works; the diagnostic names the rows (02:45Z)

Disposable `computer-14bec738…` (VM `vm-419eeec3…`), operation
`selfdev-87882911…`. Implementation and verification took 16 minutes;
the apply restart came at 02:43:13Z. From then the engineering and
Texture desks logged "waits for the self-development apply checkpoint"
(fix A, 7ea0f66d). The first checkpoint (02:44:02Z) read a quiet chain:
no head-moving or tail errors, straight to the equivalence test.

The equivalence test still fails, and now says why:

`og_objects live_only=26 replay_only=26 changed=3`, by kind:
`choir.lifecycle_command` 24, `choir.lifecycle_event` 24, `choir.event` 4,
`choir.co_super_assignment` 2 (content and body differ),
`choir.co_super_assignment_report` 1 (content and body differ).

The live-only rows were written at 02:27:58, 02:28:12, 02:40:18–24 and
02:43:15: the engineering assignment's open, its report and freeze, and
its cancel at the apply restart. The matching 26/26 counts mean replay
builds the same number of lifecycle commands and events under different
keys. **Finding:** the engineering assignment lifecycle (open, report,
cancel) is not replay-deterministic. Either the keys include a value the
replay does not see the same way (a sequence, a time, a run id), or the
live write and the reducer build different objects. H1–H3 above are
refuted as stated: the divergence is the assignment lifecycle, not run
passivation or the budget path.

Next: read how `store/engineering_assignments.go` mints lifecycle
command and event keys and bodies, and how the reducer replays the same
event, before any fix.
