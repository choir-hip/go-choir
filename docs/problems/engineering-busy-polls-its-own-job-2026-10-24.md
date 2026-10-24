# Engineering busy-polls its own capsule job, and a prompt rule has not stopped it (2026-10-24)

Date: 2026-10-24, 03:5x UTC. Found by: FP0, from the self-development drill
fp0-selfdev-1024a (evidence: [selfdev-drill-2026-10-24](../evidence/selfdev-drill-2026-10-24.md)).
Class of this record: green. Class of the fix it leads to: orange (the
engineering in-cell API; the reducer and the desk loop). Status: problem
recorded; not fixed.

## What happened

The drill (`--change go-served`, staging 0cbc52edc, a fresh jailed class-test
computer) passed all 19 checks. A full cycle works: a capsule change, Freeze,
verify, the pre-apply checkpoint, approval, the host build, apply, the new
route served, a restore, and the route gone. Its one failure was round 1 of
engineering:

- Engineering started the `bin/autoputer` Go build as a capsule job, then
  called `choir.Poll` on it 30 times in 73 seconds (03:08:53 to 03:10:06),
  each a model turn showing `go: downloading …` progress. The build was
  healthy. Module fetch through the host goproxy worked under host-only DNS.
- It tried `choir.Complete("blocked")`. The reducer refused it ("job … is
  still running; Complete waits until it ends … or choir.Stop it"; the
  10-20 fix). It then `choir.Stop`ped the healthy build and completed
  blocked. Management re-cast it, and round 2 passed.
- Round 1 cost 51 model turns and 1.68 M input tokens. Round 2 polled 57
  times over a build of about 7 minutes (71 turns, 1.54 M input tokens) and
  this time waited the job out. Polling is most of the cycle's model spend:
  about 3.2 M input tokens for one self-development change.

## The cluster

The same behaviour, three drills, five days, two models' worth of prompt
changes:

| Drill | Polls | Outcome |
|---|---|---|
| selfdev-jobs-0101 (10-15) | about 100 cells, 2 s apart, over 4 min | compaction, a refused second start, Complete blocked while the build ran |
| selfdev-jobs-0654 (10-15) | 126 cells every 2.5 s, about 4 KB tail each | compaction at 166 k tokens (emergency_fallback); the desk forgot its own build |
| fp0-selfdev-1024a (10-24) | 30 then 57 | Stop of a healthy build, round 1 lost; about 3.2 M input tokens |

What the desk is told today. **[V]**
- The `Poll` help (`internal/yaegikernel/surface.go:47`): "It never waits:
  end your turn and you are woken when the job ends."
- The engineering overlay (`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml:52`):
  "Do not poll a job in a loop … A running job is never a blocker: end your
  turn and wait to be woken; do not Complete "blocked" because a build or
  test is still running."

The model read the help in round 1 and polled anyway. The 10-20 cluster
record ([engineering-completion-path-cluster](engineering-completion-path-cluster-2026-10-20.md),
failure 23) already ruled that "a prompt change would only shift the odds. A
refusal closes the path for every model". It added the Complete refusal.
Tonight the model went around that refusal with Stop.

## Classification

Substrate, by this repo's rule: three or more symptoms in one subsystem, and
the prompt rule meant to prevent it exists and does not hold. Nothing in the
loop makes waiting the easy path. A cell that polls a running job returns at
once, and the tool loop offers the model another turn. Ending the turn is a
choice the model has to make against the grain of the loop, every time.

## What a fix has to do (not decided; failure modes first)

The fix should make "end the turn and be woken" what happens, not what is
asked. Candidate shape: a cell whose only job-related effect is a `Poll`
that sees the run's own job still running ends the turn after it reports.
The desk parks on the job wake, and the next activation starts from the
job's outcome. Failure modes any shape must answer:

1. The job wake never arrives (lost wake, restart): the desk must not sleep
   forever. Jobs carry a wall clock (at most 45 min). A park needs a
   deadline that re-wakes the desk, and the deadline must survive a planned
   restart and close on a crash, per the restart rule.
2. A desk that polls a job that is already done must see the outcome in the
   same turn (no park).
3. Stop as an escape: a Stop of a running job in the same activation that
   polled it, followed by Complete blocked, is today's escape. Decide
   whether Stop needs a reason the reducer records, or whether the park
   simply removes the pressure that leads to it.
4. Other work while a job runs (reading files, writing a note) must still be
   possible in the cells before the park, so engineering is not forced idle
   when it has real work.
5. Desks without capsule jobs (management, Texture, research) are
   unchanged.
6. The turn-commit ledger must record the park and the wake, so a trace
   shows the desk waited, and was not dropped or starved (the "Review the
   traces" rule).

## Also seen, not explained

- Right after the restore, the operation read back as `awaiting_approval`
  (its state at the restored sequence 930). The `restored` witness after the
  restart reads `applied`. One read was taken before the restart and one
  after. Not looked into.
- The host build took 17 min 10 s (go-served-1143 on 10-15: 13 min), the
  longest stage of the cycle. A warm build cache is the obvious lever. Not
  measured.

## Heresy delta

discovered: the overlay's no-poll rule and the 10-20 refusal do not prevent
a desk from stopping a healthy build to end its round. introduced: none.
repaired: none.
