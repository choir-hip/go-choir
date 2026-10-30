# Captures a failed management run read are never disposed (2026-10-14)

Status: cause confirmed from the run trace; fixed on branch
`captures-undisposed-after-failed-runs` (not pushed, not deployed). See
"Fix" below.

Found by the S0 probe of management delegation
(`docs/archive/problems/management-serial-turn-blocks-paper-2026-10-14.md`, section
"S0 baseline", on branch `management-delegation-s0-s1`): on the live paper,
45 captures that arrived 06:51-09:12 UTC had no disposition at probe time,
33 of them more than 15 minutes after arrival. 28 arrived in the 08:22 UTC
burst, while two management runs failed with the opencode-go circuit open.

## Contract

The L2a design (`docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md`,
R2-5) and management's prompt: every capture gets a disposition: a story
opened from it, a feed into a story, or a decline to its source.

## Evidence (read-only, paper computer, product routes, pulled ~10:30 UTC)

Computer `computer-1355b83bced1613eb048ee28d66688a8`, inbox
`management:e5962310-…`. Analysis scripts and JSON are kept outside the repo
(scratchpad); probe output:
`~/.config/choir-qa/traces/management/s0-paper-20261014T091207Z.json`.

Management's inbox now: `head 541, cursor 541`. Every record has been
marked read.

The 45 undisposed captures, by inbox record:

| records | captures | what happened |
|---|---|---|
| 508-514 (08:22:12-37) | 28 | **read by failed run a8081845, cursor advanced, never disposed** (below) |
| 525-528 (09:08:52-53) | 12 | still in flight at probe end (run 0768cfa8 ran 09:07:52-09:15:07); not judged |
| 460, 471 | 3 | disposed: probe join miss. Feeds exist in `texture:ef04dfc6…` seq 3 (07:05:48) and `texture:f9a21f8a…` seq 7 (07:35:17), story inboxes the probe did not read |
| 517, 518 | 2 | completed run e82ddd26: `key-6b105ac6…` was fed into `texture:6ca766e8…` (seq 4) under a **mangled id** (`key-6b105ac6d0d79b57df043cc` + the tail of another capture's id), which `choir.Feed` accepted; `key-0f433530…` was named by no act |

33 = 28 + 3 + 2 are the ones older than 15 minutes.

### The 28: the trace of the 08:22 burst

- 08:14:30 run f485d510 completes; it read record 507. Cursor 507.
- 08:22:12 record 508 (bbc_world, 1 capture) arrives and wakes run
  **e20d19a2** (created 08:22:12.840).
- 08:22:29-37 records 509-514 (27 captures) arrive while e20d19a2 is busy.
- e20d19a2 events 1-12: no cell; five model calls, failover deepseek ->
  cat-jam-free, all `circuit open (upstream unhealthy)`; event 12
  `loop.failed` 08:23:28. No cell committed, so the cursor stays 507.
- The post-turn re-check (`recheckRecordsInbox`) finds records 509-514
  committed after e20d19a2 began and wakes management: run **a8081845**
  (created 08:23:43.734).
- a8081845 event 5-7 (08:24:12-13): its first cell runs
  `for _, r := range choir.Records() {…}`. The output (event 7, continued
  through `choir.CellOut` in event 12) shows `seq=508` through `seq=514`,
  all `kind=captures`. The cell stages **0 intents** (event 6
  `desk.cell intents 0`).
- Events 11-48: eight more cells, all `intents 0` (reading cell output and
  `choir.Stories`).
- Events 51-59 (08:25:15-08:26:29): the provider fails; event 59
  `loop.failed`: `tool loop iteration 13: gateway call failed: … circuit
  open`.
- 08:37:35 record 515 arrives and wakes run 920f6721. Its first cell
  (events 5-7) prints `records 1`: only record 515. Records 508-514 are
  behind the cursor; no later run is shown them.

### Mechanism (from code)

`commitRecordsRoute` (`internal/agentcore/inbox_records.go`) commits each
cell's records **and** a cursor advance over what the run's
`choir.Records()` calls showed (`capToShown`). A cell that stages no
records still commits the cursor advance alone (`len(records) == 0 &&
cursor == nil` is the only early return). So a8081845's first cell, which
only read, moved management's cursor 507 -> 514 at 08:24:13. The run then
failed. Nothing undoes the advance (the store's cursor moves only forward),
and nothing re-shows the records: the re-check after a failed run wakes
only for records committed after that run began, and those were all
read.

So of the three hypotheses:

- **Confirmed:** a failed run's records were marked read (cursor advanced)
  without dispositions. The mark is made by a read-only cell, before the
  run fails.
- Not the cause of the 28: "nothing re-wakes management after a failed
  run". e20d19a2's failure did re-wake (a8081845). But after a8081845 there
  was nothing unread to wake for. If the read mark is fixed, the second
  half matters: a failed turn's unread records wait for the next arrival
  (here 14 minutes), and the last poll before a quiet period would wait
  longer.
- Partly: the probe's join missed 3 dispositions (story inboxes it did not
  find); the 12 of 09:08 were in flight.

Captures records are the only records in management's inbox with no
round. A Texture's Cast to management opens a round, and a failed run's
open rounds are closed by `closeRoundsOfEndedRun` (reason `run_failed`),
which wakes the Texture. A captures record has no such fate: once a failed
turn has marked it read, it is gone.

## Belief state

- The read mark belongs to a turn that finished, or to a cell that acts on
  what it read (its acts and the mark commit together). A cell that only
  reads, in a run that then fails, must not consume the records.
- A failed turn whose records stay unread needs a reader. Re-arming at once
  can hot-loop through a provider outage or a record that breaks the desk;
  re-arming with backoff and a bound cannot.
- A crash closes occurrences and never resumes work. Unread records are
  not an occurrence; a crash-closed run marks nothing read and re-arms
  nothing, as now.

## Remaining error field

- **Partial turns.** A cell that acts commits the mark over everything the
  run was shown. If a run disposes some captures in one cell and then
  fails, the captures it was shown and did not dispose are still marked
  read. Fixing this needs a per-capture disposition index.
- **Completed-turn omissions.** A completed turn can leave a capture with
  no act naming it (`key-0f433530…`, run e82ddd26). The read mark is
  correct there; the obligation is not enforced.
- **Unvalidated capture ids.** `choir.Feed` accepted a capture id that
  names no capture (`key-6b105ac6…`, mangled). The disposition looks
  structured and disposes nothing.
- **The probe's join** misses feeds into stories it does not discover
  (receipts of Feed and Note name no inbox).

## Fix

The read mark moves from management's read-only cells to the end of the
turn, the way Texture's activation end already answers a trigger that no
cell acted on (`tools_desk.go`, "rerun 12"):

- `commitRecordsRoute`: a management cell that stages no records commits
  no cursor advance. A cell that acts still commits its acts and the mark
  over what the run was shown, in one transaction, as before.
- `markTurnRecordsRead` (first step of `recheckRecordsInbox`): a
  management turn that **completed or parked** marks read what its
  `choir.Records()` calls showed it. A failed or cancelled turn marks
  nothing; a crash-closed run is never completed and marks nothing.
- `scheduleFailedTurnRetry`: after a **failed** management turn that left
  records unread (and no record arrived during the turn, which wakes at
  once as before), one durable `inbox_record` wake is scheduled with
  `not_before` = now + 2 min x 2^(n-1), n the failed turns in a row
  (reset by a completed turn), for n <= 4 (2, 4, 8, 16 min). Past that the
  records wait for the next arrival. The counter is in memory; a restart
  resets it. A crash boot interrupts a pre-boot `inbox_record` wake
  (`RestartWorkKinds`), so the retry never resumes work after a crash.
- Research, Texture and engineering are unchanged: their records carry
  rounds, which a failed run's round closure already gives a fate.

On the paper's 08:22 burst this would have left the cursor at 507 after
a8081845 failed, scheduled a wake for 08:28:29, and shown records 508-514
to the next turn.

Tests: `TestManagementRecordsSurviveFailedTurn` (a real management run
through the tool loop: a cell reads a captures record, then the scripted
provider fails; before the fix the cursor was 1 after the failed turn, the
live failure) and `TestManagementFailedTurnRetryIsBoundedAndBacksOff`
(backoff, bound, reset, crash, research unaffected), in
`internal/agentcore/records_failed_turn_test.go`;
`TestCellMarksReadOnlyRecordsItShowed` and
`TestLegacySupportPairsRecordIsShownClosedAndJudged` updated to the new
mark (read at the turn's end, not at a read-only cell).

The remaining error field above is unchanged by this fix.
