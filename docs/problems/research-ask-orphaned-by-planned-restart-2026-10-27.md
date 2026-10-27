# A research ask waiting at a planned restart is never answered, and its open round blocks every later ask (2026-10-27)

Status: open; documented before any fix. Found while proving the
related-review wall bound (docs/problems/related-review-act-outlives-its-request-2026-10-27.md).

## Evidence (read-only, FP0, 17:3x to 17:4x)

- Related-review batches on 052c75484 and later: b41 scanned 150 stories,
  busy 84; b42 scanned 76, busy 75; neither asked anything. At the same
  time the paper's dispatcher logged `slots=4/12 due_agents=4`, so the
  busy stories are not running research desks.
- One busy story, 24133f0f-5263-5d22-a3af-20bb07faa4fb ("Trump scheduled
  for Rock Hill campaign stop Thursday"): trajectory `live`; agents
  `research:<doc>` and `texture:<doc>` with no active run; no work items;
  events only `trajectory_started`, two `texture_turn_committed`,
  `run_reactivated`; no research run ever. Its research inbox
  (`GET /api/inboxes/research:<doc>/records`) holds one record, the cast
  `related-review:rr-2026-10-26-11:<doc>` committed 2026-10-26T20:28:12Z,
  and `open_rounds` shows round 1 open.
- The paper took a planned deploy refresh at 20:33:24 (vmctl `destructive
  kill … cause=refresh`, then `deploy refresh caller=deploy`), and three
  more by 21:54.
- Control: the 40 stories asked by b13 to b16 on 10-27 (receipts
  returned) have no open research round: all were answered.

## Why (hypothesis, not yet traced)

- `CommitStoryResearchRequest` refuses while the texture↔research round is
  open (`internal/store/story_research_request.go`, `conversation.Open` →
  `ErrStoryResearchBusy`); the backfill counts that as Busy.
- Rounds close on a final Report, when a run ends or parks
  (`closeRoundsOfEndedRun`, `internal/agentcore/inbox_records.go`), or at a
  crash boot (`closeOpenRoundsAfterCrash`). A planned restart resumes
  runs but this ask had no run yet: if its wake lived only in memory, the
  planned restart dropped it, and nothing closes the round.
- Consequence: every story asked shortly before a planned restart is
  never analysed and is refused by every later ask, silently, as Busy. The
  same path would orphan any cast to a desk that had not started when a
  deploy landed (hypothesis on scope).

## Next

Trace how a cast to a research desk becomes a dispatch (durable work item
or in-memory wake) and what a marker boot does with an undispatched cast;
count open rounds addressed to research desks on the paper.
