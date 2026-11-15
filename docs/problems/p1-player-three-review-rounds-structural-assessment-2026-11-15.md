# The P1 phone player after three review rounds: a structural assessment

Date: 2026-11-15. Lane `lane/arl-p1`. Class orange. One seat (Sonnet), three
rounds, each REPAIR. The rule is to stop patching at three and change the
approach.

## What happened

| Round | Tip | Blocking findings | Where they were |
| --- | --- | --- | --- |
| 1 | 681ae12d9 | failed then finished for one segment; replay after running dry; an append lost in the loop's tail | the first build |
| 2 | d2bf58f61 | `resumeAt` outlived its document | state added by the round 1 repair |
| 3 | 1222503a7 | a paused loop survives `handOver` and plays the wrong blocks; a stale follower's `handOver` replaces a newer document | the hand-over added in the round 2 repair |

Round 3 confirmed every earlier finding closed. Both of its blocking findings
are in code that did not exist at round 2.

## Belief

Two causes, neither of them the reviewer.

1. Each repair added per-document state to the player (`resumeAt`, the closed
   marker, `pendingDocument`) and each new field was cleared on some of the
   paths that replace a document and not on others. The player has four ways
   to replace a document (`setDocument`, `start(at:)`, `speakText`, the queue)
   and no single transition that every one of them goes through.
2. I widened the scope inside a repair round. Round 1's reviewer said the
   missing first-segment deadline did not block. I asked for the deadline
   anyway, then in round 2 asked for a hand-over from the article to a late
   programme. That hand-over is a feature, not a repair, and it was built
   without a design, in the one file that was already failing review.

## Change of approach

- The hand-over is removed from P1. At the deadline the article plays and the
  stretch is let go, which is the round 2 behaviour round 2's reviewer
  accepted. The listener gets the programme at the next load. This is written
  down as a residual, with the measured reason it does not bite today (request
  plus first segment took 0.01 s in the simulator; a real computer's latency
  is unmeasured until L1 deploys the route).
- P1 lands as round 2's code plus the four round 2 repairs that round 3
  confirmed, with one delta review that the removal is clean.
- The hand-over becomes its own later piece, after L1 gives a real latency
  number, and starts from a design: one document-replacement transition that
  bumps the epoch and clears every per-document field in one place, and a
  follower bound to the document id it was opened for.
- No scope is added inside a repair round again. A reviewer's non-blocking
  note becomes a residual or a new lane.

## Remaining error

A computer that is slow to wake leaves the listener on the article for that
load. The player's per-document state is still spread over several fields;
the table in `mobile/README.md` says where each is cleared, and round 3
checked it against the code.
