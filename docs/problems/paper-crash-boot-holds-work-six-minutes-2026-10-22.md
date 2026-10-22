# A crash boot holds the paper's work for six minutes (2026-10-22)

Date: 2026-10-22, 20:15 UTC. Found by: FP0, during the KF host reboot.
Status: open, problem record before any fix. Mutation class of a fix: red
(boot path, crash-restart closure, the paper's availability).

## Evidence

Node B rebooted at 20:05:19 for kernel 6.18.57. The paper
(vm-1669020a…, critical_protected) was busy, so it refused a planned-restart
marker and booted as a crash (`restart kind=crash_or_stop`, 20:07:08). Its
`/health` answered 200 from 20:08:33, but its boot phases ran in order, and
the two crash closures came before the source schedules:

| phase | start | duration |
|---|---|---|
| `close_pre_boot_runs_after_crash` | 20:07:12 | 1m50.4s |
| `close_open_rounds_after_crash` | 20:09:03 | 4m36.6s |
| `source_schedules_after_restart` | 20:13:39 | 3.3s |

So no source schedule was armed, and the gateway saw no inference call from
the paper, for about six and a half minutes after its boot while `/health`
said ok. During the second phase the console was silent; from the host the
Firecracker vCPUs used about 0.8 CPU, and the process had read 12 GB from
its disks by 20:13.

## Cause (read from the code)

`closeOpenRoundsAfterCrash` (`internal/agentcore/inbox_records.go:1133`)
calls `store.RebuildInboxHeads` (`internal/store/inbox.go:1088`), which
lists every inbox record of the computer (the record plane; the paper's
store held 46,381 `commitment_record` rows at 18:30, an upper bound), folds every inbox head and every
conversation, and only then picks the open conversations. The store already
keeps the conversation head as a cached object written with each record
(round, open flag, caster; `inbox.go:15-28`), so the open set is available
without the full fold. The fold's cost grows with the paper's whole
history, so every crash boot gets slower.

`close_pre_boot_runs_after_crash` closed each pre-boot run one at a time
(about 150 ms per run in the console).

## Why it matters

- Owner rule (2026-10-14): "we need paper publishing 24/7, even after
  crashes". A standing intent survives a crash, but here its next tick waits
  for a closure whose cost is unbounded.
- `/health` says ok through the whole window, so the runbook's "paper
  healthy" test (health ok AND schedules not overdue AND inference in the
  last 30 minutes) is the only check that catches it, and only after 30
  minutes.
- It is the same family as the post-refresh windows
  ([paper-post-refresh-health-windows-2026-10-22.md](paper-post-refresh-health-windows-2026-10-22.md))
  and the derived-keys validation pass: whole-history work on the boot path
  or right after it.

## Open

1. Can the crash closure read the open set from the cached conversation
   heads (and leave the full fold to `VerifyInboxHeads`, off the boot path)?
   What must hold for the cache to be trusted at a crash boot (the heads are
   written in the same transaction as their records?).
2. Should the source schedules arm before the crash closures? They are
   standing intents, independent of the closed runs, but a schedule tick
   must not race a closure of the same work.
3. The host reboot script never asks a busy critical computer for a
   planned restart with `allow_busy`, which the guest supports
   (`"allow_busy":false` in its refusals at 20:04:16-20:05:19). With it, the
   paper would have booted planned and skipped both closures.

## Findings (FP4 lane crash-boot-closure, 2026-10-22, from the code at 29954c14b)

**C1. What the 4m36s phase reads.** `closeOpenRoundsAfterCrash`
(`internal/agentcore/inbox_records.go:1133`, phase at
`internal/agentcore/runtime.go:889`) calls `RebuildInboxHeads`
(`internal/store/inbox.go:1088`), which lists every inbox-plane record via
`ogListAllByMetadata(choir.commitment_record, record_plane, inbox)`
(`inbox.go:1090`, `804`). No generated column or derived key covers
`record_plane`, so each page is a JSON_EXTRACT pass over the commitment
records after the page cursor, ordered by canonical id
(`internal/objectgraph/dolt_store.go:1271-1273`). The paging loop
(`internal/store/graph_store.go:134-160`) re-runs that pass once per page
of `ogMetadataPageSize` rows. So the total work is about (records / page size)
passes over the remaining records, each returning full bodies: quadratic
in the record count, under the engine lock. The phase then decodes every
record, folds every conversation, and keeps only the open ones. That
matches the 12 GB read from disk by 20:13.

**C2. The cached heads can be trusted at a crash boot.**
- The conversation head (`choir.conversation_head`) is written only by
  `planInboxRecords` (`inbox.go:600-611`). It is planned in the same
  object list as every inbox record that changes it (`inbox.go:530-544`),
  with a compare-and-set condition on the head's prior content hash
  (`inbox.go:119-133`).
- `planInboxRecords` is also the only writer of an inbox-plane record:
  `record_plane` is set only at `inbox.go:533`. The other writers of
  `choir.commitment_record` (`commitment.go:41`,
  `lifecycle_control_issue.go:384`, `lifecycle_commit_act.go:316,571`)
  write records outside the plane, and `RebuildInboxHeads` does not fold
  them.
- Every caller commits that list as one batch: `CommitInboxRecords`
  (`inbox.go:653-656`) and the composed plans in
  `engineering_assignments.go:1127,2437`,
  `story_research_request.go:173` and `texture_turn.go:688`.
- A batch is atomic on both write paths. Direct: one serializable
  transaction (`dolt_store.go:1147-1194`). Projection tape: the whole batch
  is one tape event (`internal/store/projection_tape.go:61-88`), finalized
  in one serializable transaction (`internal/store/computer_events.go:120-140`),
  and tape recovery replays whole events. A crash cannot leave a record
  without its head.
- Heads exist from the first inbox record: they arrived in the same
  commit as the record plane (dee099747, Cut 1 L3a).
- The fold was never the authority. A closure commits through
  `stepConversation` against the cached head, so a round the head calls
  closed refuses the closure anyway. The live run-end check already reads
  the heads (`OpenRoundsAddressedTo`, `inbox.go:1027-1081`).
- `VerifyInboxHeads` (`inbox.go:1117`) is the audit of heads against the
  fold. It has no production caller.

So the crash closure can read the open set from the heads. That costs one
header row per conversation of the owner (`ListAllObjectRefsByKindOwner`
over `choir.conversation_head`) and one primary-key read per head of the
computer. It grows with conversations, not with records.

**C3. Should schedules arm first?** Not needed once C1 is cheap:
`engineering_assignment_capsules` and `source_schedules_after_restart`
run after the closure, and nothing in them reads rounds. The order stays
as it is.

**C4. The 150 ms per run of `close_pre_boot_runs_after_crash`.**
`closePreBootRunsAfterCrash` (`runtime.go:2608`) makes, per run:
- one owner-wide header list, shared by all runs (`forEachPreBootRun`);
- `terminalizeRunCanonical`: a trajectory point read, then either
  `TerminalizeRun` or `UpdateRun`, which is one write transaction;
- `MarkAgentMutationStale` for Texture runs (the texture DB);
- `emitEvent`, which is one more write. Its first append for a run in a
  new process seeds the run's sequence by reading every earlier event of
  that run (`internal/store/store.go:2181-2214`, indexed on `run_id`, up
  to 10,000 bodies).

A local probe (20 parked runs with 300 events each, a cold cache from a
reopened store) took 22 ms per run. Most of it was the two writes (40
`PutObject`, 265 ms), then the seed reads (20 calls, 70 ms). There is no
scan that grows with the computer's history. The cost is about two write
transactions per closed run, and the input is the set of runs open or
parked at the crash. That set is emptied by the crash close itself, so a
second crash boot has nothing to close. Hypothesis, unconfirmed: 1m50s at
about 150 ms is about 730 runs. That would be the parked runs that
accumulated since the last crash boot (every planned restart parks the
in-flight runs, and parked runs that are never resumed stay passivated).
The 20:05 console's candidate count will tell. Batching closures into
fewer transactions would change the event append path (sequence
allocation, telemetry sealer), so it is not a small fix. Recorded as
residual R-C1, not fixed here.

### Fix shape (red, boot path)

`closeOpenRoundsAfterCrash` reads the open rounds from the conversation
heads (a new `Store.OpenConversationHeads`, sharing its loop with
`OpenRoundsAddressedTo`), in conversation-id order. Each closure is
committed exactly as before (same record, same command id `boot-closure:`,
same fate). The fold stays in `VerifyInboxHeads`, off the boot path.

### Fix and proof steps (lane/fp4-crash-boot-closure)

Fix: 44793b5f0 (red). Test: 85a046294 (yellow). Before the fix, the test
failed on `ListObjectsByMetadataPage choir.commitment_record
$.record_plane json ran 2 times`. Not fixed: C4, the per-run close
(residual R-C1), and Open 3, the host reboot script's `allow_busy` (a host
change, not this lane).

Proof steps (FP0, at a crash boot of a test computer: stop it through
vmctl with no planned-restart marker):

1. Before the stop, make sure the test computer has at least one open
   round: a Texture request whose research desk has not reported yet.
   Note its conversation id, from the guest's obligations surface or the
   console's "Cast" line.
2. After the boot, from the console:
   `grep -E 'restart kind=|boot phase (begin|end)|closed round|closed run|crash boot'`.
   Pass:
   - `restart kind=crash_or_stop`;
   - `close_open_rounds_after_crash dur=` is a few seconds at most;
   - one "closed round N of conv:… after a crash restart" line per open
     round, and none for closed ones;
   - `source_schedules_after_restart` begins right after the closure;
   - no "crash boot round closure: open rounds:" error.
3. Crash rule: every pre-boot run reads cancelled with error
   `interrupted_by_restart` ("closed run … after a crash restart"). The
   round's caster inbox holds a final system Report with stop reason
   `interrupted_by_restart`. No "activation wait" line for a closed run.
4. Second boot writes nothing: crash-stop the same computer again. Pass:
   `close_open_rounds_after_crash` logs no "closed round" line, and
   `close_pre_boot_runs_after_crash` closes only runs started between the
   two boots.
5. On the paper at its next crash boot, if one happens: the same grep. The
   `close_pre_boot_runs_after_crash` candidate count (the "owner-scoped …
   candidates=N" line) settles the C4 hypothesis (about 730 parked runs).
   Record it here.

Residuals:
- R-C1: about 150 ms per closed run on the paper: two write transactions
  plus the event-sequence seed. Bounded by the open and parked runs at the
  crash, not by history. Batching would change the event append path.
- R-C2: `VerifyInboxHeads` has no production caller. An off-boot periodic
  or owner-triggered audit would catch head drift if it ever appeared.
- R-C3: the header list over all conversation heads of the owner grows
  with conversations, not records. If it ever matters, an `open` field in
  the head's metadata lets closed heads skip the point read.
