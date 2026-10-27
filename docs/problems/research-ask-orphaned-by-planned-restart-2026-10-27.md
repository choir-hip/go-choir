# A research ask whose wake ends without a run leaves its round open, and the open round blocks every later ask (2026-10-27)

(Retitled 18:0x: the planned-restart hypothesis below is refuted by the code; the file name keeps the first title.)

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

## Trace (Sonnet, read-only, 18:0x; file:line in the agent report, summarized)

- **Refuted:** the wake is not in memory. `CommitStoryResearchRequest`
  writes the cast, the round-opening conversation head and an
  `actor_wake_outbox` row of kind `inbox_record` in one conditional batch
  (`story_research_request.go:210-224`, `inbox.go:541-563`). The projector
  sweeps the whole outbox after any restart (`runtime.go:3000`,
  `actor_wake_projector.go:88`), and a planned (marker) boot delivers a
  pre-boot `inbox_record` wake (`planned_restart.go:232-237`,
  `handler.go:112`). A crash boot interrupts it and
  `closeOpenRoundsAfterCrash` closes the round (`inbox_records.go:1227`).
- **What can leave the round open (hypothesis among these):** the wake
  reaches a terminal fate with no run and nothing closes the round:
  dead letter after 64 deferrals (`WakeRecordsInbox` errors →
  `ErrDeferUnprocessed`, `handler.go:527-529`; `delivery_failed` only
  logs); `MarkActorWakeExhausted` after 5 attempts (`runtime.go:2942`); a
  silent `nil, nil` from `WakeRecordsInbox` when the desk record is
  missing or a run with a live loop exists (`inbox_records.go:990`); or a
  crash-boot closure that was skipped or failed to commit
  (`inbox_records.go:1233-1237`).
- **Not decidable now:** the paper's console from 10-26 has rotated away
  (oldest kept: 10-27 12:10), so this story's wake fate is unread. Its
  outbox row's `projected`/`fate` metadata would decide it; no product
  read route for that row is known.
- **Scope:** not restart-specific. Any round whose recipient desk never
  runs is stranded once its wake goes terminal; owner casts to research
  have no rescue (Texture-to-management casts have `recheckRecordsInbox`).

## Fix candidates (not chosen; red: inbox rounds, wake outbox)

1. Close the round wherever a wake reaches a terminal fate without a run
   (dead letter, exhausted, failed crash closure, the `nil, nil` no-op),
   reusing `NewRoundClosureRecord` (`inbox_records.go:1308-1318`).
2. A reconcile for open rounds addressed to a desk with no active run, no
   unprojected wake and unread records: re-mint the wake on a planned
   boot, close it with a restart fate on a crash boot.
3. `WakeRecordsInbox` returns an error, not `nil, nil`, when the desk is
   missing, so the wake defers and dead-letters visibly.
4. Symptom only: the backfill treats an old open round with no research
   run as stale. Weakest; hides the cause.

First observation either way: count open `research:*` rounds with no
active run on the paper, and log each wake's terminal fate with its
inbox so the next case is readable.
