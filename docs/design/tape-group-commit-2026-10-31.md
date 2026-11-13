# Tape group commit: fewer tape events for the same writes (2026-10-31)

Status: design and build, mutation class red (tape write path, evidence).
Rollback: the computer setting `tape.group_commit` off (live, no restart),
or revert. Inputs: [telemetry batching design](../reports/telemetry-batching-design-2026-10-20.md),
[paper tape composition](../evidence/paper-tape-composition-2026-10-30.md),
[one tape](one-tape-durability-2026-10-31.md), `internal/store/telemetry_sealer.go`,
`internal/store/projection_tape.go`, `internal/computerevent/appender.go`.

Labels: **[V]** measured or read in this note; **[I]** inferred, basis stated.

## 0. Why

Every tape event costs a fixed ~5-6 KB of host state whatever it carries: a
PostgreSQL append receipt row (~1.9 KB plus indexes), two pin receipts, the
event record and the payload record in a segment. The paper writes ~12.4k
events an hour with telemetry batching on (since 14:48 today), down from
~18.4k. The aim is fewer events for the same ops, without losing an
acknowledged write.

## 1. What was measured (paper, 2026-10-31 14:50-16:51 UTC) [V]

Read-only, metadata only: the four newest tape segments of
`computer-1355b83b...d66688a8` on Node B, event envelopes only (`occurred_at`,
`authority_ref`, `event_kind`); payload records were skipped by their header
length and never decoded. Host receipt counts from `computer_event_append_receipts`.

| | per hour | share |
|---|---:|---:|
| ordinary-authority `projection_batch_recorded` (acknowledged writes) | 10,288 | 83% |
| telemetry-authority batches (the sealer) | 2,075 | 17% |
| total tape events | 12,485 | |

- **The sealer is already one per computer** (one `telemetrySealer` per
  `Store`, `telemetry_sealer.go`); there are no per-stream sealers. The
  "several streams each sealing once a second" reading was wrong.
- **Telemetry batches are sealed by the barrier, not the window.** Of 4,238
  telemetry batches, 4,175 are followed directly by an acknowledged event and
  only 63 by another telemetry batch (transitions T->A 4,175, T->T 63).
  Inter-arrival of telemetry batches p50 0.92 s. Every acknowledged write
  first seals pending telemetry (the ordering barrier, design 4.3), and on the
  paper an acknowledged write comes every ~0.35 s, so the batch closes at the
  next acknowledged write whatever the window is. A longer window alone buys
  almost nothing [I: barrier seals dominate].
- **Ops per batch** could not be counted without decoding payloads (not
  done). The 10-22 staging gate measured k = 5.16 on a test computer.
- **The remaining 83% is acknowledged writes**, mostly single-op (10-29: 92%
  of all events one op; run memory alone 37% of events). The append is
  serial under the appender mutex; consecutive events cluster at 60-80 ms
  apart (3,498 + 2,439 gaps in the 60-80 ms bins of 25k), the service time of
  one append (pin payload, pin event, CAS, finalize) [I].
- **The "1 s window = 62k/day, 5 s = 14.6k/day" claim checks out as
  arithmetic**: today's data gives 2,531 non-empty 1 s windows an hour
  (60.7k/day) and 651 non-empty 5 s windows (15.6k/day); the 10-30
  composition note found 61,827 and 14,643 on 10-29. **It is not reachable by
  group commit of acknowledged writes**, because a window that long would hold
  every acknowledged write for up to that long, and most writes are serial
  chains (one goroutine: write, wait for the ack, write again), which no
  window can merge. A simulation over today's arrivals, service time 65 ms:

| linger L | ack events/h | mean ack latency |
|---|---:|---:|
| 0 (pure group commit) | 10,010 | 72 ms |
| 10 ms | 9,686 | 78 ms |
| 50 ms | 9,238 | 114 ms |
| 100 ms+ | (not credible: the simulation merges serial chains it cannot) | |

  The simulation's arrivals are `occurred_at`, stamped inside
  `appendOpsAs` after the caller's own locks. Run memory takes the global
  `runMemoryMu` before that stamp, so concurrent run-memory writers convoy
  there and arrive one service time apart: the simulation cannot see them as
  concurrent. Group commit merges a convoy; how much of the 60-80 ms cluster
  is convoy rather than a serial chain is the open number. **Predicted gain
  of group commit on acknowledged writes: 3% to ~35%, settled only by the d@e
  and paper counters (section 6).**

## 2. Failure modes (first)

Each has a test in `internal/store/tape_group_commit_test.go`.

| # | Failure | Rule that prevents it | Test |
|---|---|---|---|
| F1 | An acknowledged write returns before it is durable | a member returns only after the shared event's CAS and local finalize returned; its error is the event's error | `TestGroupCommitMembersAckAfterSharedCommit` |
| F2 | One member's refusal fails the others | a refused group (`ErrBatchRefused`, nothing pinned) is split: pending telemetry is sealed first, then each member is written alone in arrival order and gets its own result | `TestGroupCommitRefusalSplitsToSingles` |
| F3 | Unknown outcome (pin or CAS failed after something may have landed) appended twice | never retried, as today: every member gets the error; `RecoverPrepared` settles the prepared event at boot | `TestGroupCommitUnknownOutcomeIsNotRetried` |
| F4 | A guarded op (expected state, create-only) fails inside a group and drags others | only unconditional ops group: `object_recorded`, `object_edge_recorded`, `run_memory_entry_recorded` (upserts by key). Every other op kind takes the direct path | `TestGroupCommitOnlyUnconditionalOpsGroup` |
| F5 | A semantic write lands on the tape ahead of earlier telemetry | the group leader holds the sealer's flush lock and puts every queued telemetry record in the same event, ahead of the members' ops; older chunks beyond the batch cap are sealed first in order | `TestGroupCommitTelemetryRidesAheadOfMembers` |
| F6 | Ordering across streams | ops in the event are telemetry (enqueue order), then members (arrival order under the group lock). A write that causally follows another (issued after its ack) arrives after the earlier group is done, so it lands in a later event | `TestGroupCommitCausalOrderAcrossGoroutines` |
| F7 | Deadlock: leader and sealer, or the sealer's own class-violation fallback | lock order is always flushMu -> appender mutex; the leader appends with the barrier exemption (it is the barrier); the sealer's internal singles bypass grouping | `TestGroupCommitNoDeadlockUnderLoad` (race, 32 writers, telemetry on) |
| F8 | Replay or witness diverges | the event is an ordinary `projection_batch_recorded` batch; live finalize and replay apply the same ops in the same order; positions (`pos`, op index) of stamped ops are fixed by the tape. No reducer, payload or witness change | `TestGroupCommitReplayMatchesLive` |
| F9 | Memory grows without bound | a member's ops are memory its caller already held; a group is capped at 256 ops and 1 MiB; a larger single write goes alone | `TestGroupCommitCapsSplitGroups` |
| F10 | A cancelled caller's write is half in a group | a member cancelled before a leader took it is removed and gets ctx.Err() (nothing written); once taken it waits for the outcome | `TestGroupCommitCancelledBeforeTakenWritesNothing` |
| F11 | Crash mid-group | nothing was acknowledged (every member was still waiting), so this is today's crash of one in-flight append; boot closes work as "interrupted by a restart" (CLAUDE.md), nothing re-dispatched; queued telemetry is the tail the note already names | crash-close tests unchanged |
| F12 | Planned-restart drain | members are synchronous: nothing to drain. Telemetry drain and synchronous mode are unchanged (the leader takes nothing from a sealer in synchronous mode: its queue is empty) | existing planned-restart tests |
| F13 | Idempotency keys | a projection batch's key is `projection-batch:<event id>`, minted per event; no caller supplies a key on this path. Callers with their own keys (`AppendNew*`) never group | n/a (by construction) |
| F14 | Paper publishing latency | no linger: a lone writer commits at once, as today; a writer that would have queued on the appender mutex rides the event ahead of it instead of waiting for its own. Latency is equal or lower | counters: `group_commit.wait_ms` |
| F15 | Rollback strands data | nothing new is written: the events are the format every release reads. Off returns every write to the direct path at once | `TestGroupCommitOffIsTheDirectPath` |

## 3. Telemetry (part a)

1. One sealer per computer: already so (section 1). No change.
2. **Telemetry rides the next acknowledged write.** With `tape.group_commit`
   on, a grouped acknowledged write carries the pending telemetry in its own
   event instead of first sealing it as a separate telemetry event. This is
   the barrier, done without an extra event. Measured cost removed: 2,075
   telemetry events an hour on the paper, less the batches still sealed alone
   (by the window when no acknowledged write comes, by the direct-path
   barriers of `AppendNew*` writers, by cap). Riding telemetry is in an
   ordinary-authority event; the ops are still `choir.event` objects of a
   batched kind and are found by kind, not authority (no reader keys on the
   telemetry authority: checked by grep).
3. **Window 10 s.** The clamp maximum rises from 2 s to 10 s, and with
   `tape.group_commit` on the window is 10 s. It now matters only when the
   computer writes no acknowledged event for that long. Crash loss is at most
   the time to the next acknowledged write, at most 10 s, and is already
   named by `telemetryMissingWindowNote` on every crash-closed run. The queue
   cap (256) and the reseed gap (512) are unchanged; at the paper's ~1.3
   telemetry records a second a 10 s window holds ~13.

## 4. Group commit (part b)

Leader and followers on `projectionTape.appendOps`, no timer:

- A write whose ops are all unconditional joins the pending list and then
  either finds its result already set (a leader took it) or takes the leader
  token.
- The leader takes the pending list (up to the caps), takes the sealer's
  flush lock and its queue, builds one batch (telemetry ops, then members'
  ops), and appends it once with the barrier exemption. It then sets every
  member's result and hands the token on.
- On `ErrBatchRefused` it splits (F2). On any other error every member gets
  it (F3).
- Run memory: the global `runMemoryMu` becomes 64 stripes keyed by run, so
  run-memory writers of different runs can share an event; one run's entries
  stay serial (their seq and parent come from the previous entry).

Not built, named as the next levers (they change who waits, so they need the
owner):

- **Call-site batching** of serial chains: `initialize` and
  `carryActorConversation` append a run's opening messages one acknowledged
  event each (`internal/agentcore/run_memory.go`); one batch per call would
  remove them with no change of semantics.
- **Run memory as a log, not an acknowledged write** (37% of events on
  10-29): it is already outside the telemetry barrier. A crash closes the run
  and never resumes it, so its memory after a crash is record only. Making it
  window-durable like telemetry is a durability policy change.

## 5. Switch and ring

`tape.group_commit` (computer setting, default off, registry ring R0): on for
d@e first by stored setting after the deploy
(`POST /api/computer-settings {"key":"tape.group_commit","value":true}` with
d@e's key), then the paper, then the default. Applied at boot and live on
change. Off is the rollback.

## 6. Proof

On d@e, then the paper: `GET /api/runtime/observability` -> `telemetry_batching.group_commit`
(`groups`, `members`, `members_per_group`, `telemetry_ridden`,
`refused_splits`, `unknown_outcome`, `wait_ms_mean/max`) and `tape_events`
per hour against the host receipt count; the replay completeness probe
`equivalent` at a head with grouped events; zero `records_dropped`; no
duplicate `seq`/`stream_seq` across a crash boot.

## 7. Prediction for the paper

From 12,485 events an hour (299.6k a day) [V]:

- telemetry riding: -1,700 to -2,000 an hour [I: the telemetry batches that
  directly precede a grouped acknowledged write];
- group commit: -300 (3%) to -3,600 (35%) an hour of the 10,288 [I, section 1].

So about **8.5k to 10.6k an hour, 205k to 255k a day** (-15% to -32%). The
62k/day the windowing arithmetic shows needs the two levers in section 4's
"not built" list, chiefly run memory, not a longer window.

## 8. As built (2026-10-31, lane/tape-group-commit)

- `internal/store/tape_group_commit.go` (members, leader, `sealRiding`,
  counters), `projection_tape.go` (`appendOpsAs` routes, `appendDirect` is the
  old body), `telemetry_sealer.go` (clamp max 10 s, `timerWindowLocked`, the
  class-violation singles go direct), `run_memory.go` (64 stripes),
  `computer_setting.go` + `internal/agentcore/tape_group_commit.go` (the
  setting at boot and on change), registry entry `tape_group_commit` (R0).
- The barrier exemption has a third caller, the leader; the source-scan test
  names it.
- Load test (`TestGroupCommitNoDeadlockUnderLoad`, 32 writers, telemetry on
  at 5 ms, race detector): 960 writes, 43 tape events, mean 14.6 members a
  group, every telemetry record rode, nothing dropped, run-memory seq dense
  per run. It shows the mechanism, not the paper's gain (its writers are all
  concurrent).
- With group commit forced on for the whole store, agentcore and
  textureowner suites (a temporary local switch, not committed): the only
  store failures are tests that assert one tape event per write, the old
  telemetry shape or the 1 s window, all expected; textureowner passed whole;
  agentcore's one failure is `TestBootConfiguresBatchingFromSettingAndEnv`
  reading the effective 10 s window instead of the 500 ms it set, expected.

Open risks: the gain on acknowledged writes is unmeasured (section 1); the
lease and every other unconditional object write now may share an event,
which changes nothing a reader sees but makes the restore granularity
coarser by one group; telemetry that rides is in an ordinary-authority event,
so the authority alone no longer finds all telemetry (no reader does that
today); Consistent reads still seal telemetry as its own event.

## 9. The run-memory window (owner, 2026-10-31)

Owner decision, relayed: run memory may join telemetry's crash-lossy window,
except `rlm_inbox_cursor` and anything else whose loss would cause duplicate or
wrong work after a restart. Built behind the same `tape.group_commit` setting
(`internal/store/run_memory_batch.go`).

**Kind audit.** Five kinds are written today:

| Kind | Read after the write by | If lost in a crash | Window? |
|---|---|---|---|
| `message` | the run itself (context); the next run of the actor via `LatestActorRunMemoryEntries` (completed or passivated runs only); compaction | the newest turns of a run the crash boot closes and never resumes | **yes** |
| `rlm_inbox_cursor` | the next desk cell's inbox read (`rlm_reduce.go`) | the inbox is read again: duplicate work | no |
| `cell_fate` | the reducer's cell disposition (`rlm_reduce.go`) | a cell is dispatched again | no |
| `compaction` | the next run's checkpoint (`latestRunMemoryCheckpointAndTail`) | a wrong seed for the next run | no |
| `texture_cell_author_receipt` | Texture's idempotency check (`texture_cell_author.go`) | a committed turn is authored twice | no |
| any later kind | | | no (closed list) |

**Failure modes and tests** (`internal/store/run_memory_batch_test.go`,
`internal/agentcore/tape_group_commit_test.go`):

| # | Failure | Rule | Test |
|---|---|---|---|
| W1 | a kind above waits in the window | `runMemoryKindBatched` is a closed list (message only); the record constructor and the seal-time recheck both refuse others | `TestRunMemoryWindowHoldsOnlyMessages` |
| W2 | a live run cannot read its own queued messages, or reads them differently | the run's list, latest entry and lookup by id merge the queue (taken first) with the table (read second), converted by the same scanner as a row | `TestRunMemoryWindowReadsAndNumbersQueuedEntries` |
| W3 | seq or parent reused or broken | a per-run tail cache numbers queued entries; on a miss the run's queued entries are sealed before the table is read; the cache is cleared whenever the switch changes | same test |
| W4 | a crash leaves a hole | every acknowledged write (any kind, any run) seals or carries the queue first, so what survives is a prefix | `TestRunMemoryWindowCrashLosesOnlyATail` |
| W5 | a finished run seeds the next with a partial log | a run becomes completed or passivated only by an acknowledged write, after which its queued memory is on the tape; a crash-closed run is cancelled and `LatestActorRunMemoryEntries` skips it | same test |
| W6 | a queued entry that cannot be sealed is lost silently | the run is marked damaged: its next append and read fail with `ErrRunMemoryLost`; other runs go on | `TestRunMemoryWindowLostEntryFailsTheRun` |
| W7 | a planned restart loses memory | the planned-restart drain and synchronous mode are the telemetry ones, unchanged; a planned boot whose drain failed with the window on closes runs as after a crash instead of parking them for a resume | `TestRunMemoryWindowPlannedDrainLosesNothing`, `TestRunMemoryWindowRuntimeRules` |
| W8 | a read across runs misses queued entries | the agent-wide read seals first | `TestRunMemoryWindowCrossRunReadSeals` |
| W9 | the opening and carried messages are one event each | `AppendRunMemoryEntries` writes a run's entries as one write; used by `initialize` and `carryActorConversation` with the switch on | `TestRunMemoryBatchedAppendIsOneWrite`, `TestRunMemoryWindowRuntimeRules` |

The crash-close record's note becomes "the last window of this run's trace
and run memory may be missing" while the window is on.

**Prediction for the paper, revised** (events a day; today 299.6k [V]):

| Item | Now | After | Basis |
|---|---:|---:|---|
| telemetry batches | 49.8k | ~0-5k | they ride acknowledged writes; deadline seals only in quiet stretches [I] |
| run-memory messages (own events today) | 75k-113k | ~0 | run memory was 125.9k single-op events on 10-29 [V]; messages 60-90% of it [I: kinds not decoded] |
| other acknowledged writes | 134k-172k | 87k-167k | group commit -3% to -35% (section 1) [I] |
| **total** | **299.6k** | **~90k-170k** | -45% to -70% |

The batched opening messages add little on top of the window (the window
already queues them); they cut events on computers without telemetry
batching and for compaction rows.

**As built, run-memory window.** Default-off suites pass (store, computerevent,
agentcore, textureowner). With the switch forced on for every store (a local,
uncommitted switch), textureowner passes whole and agentcore fails only two
tests that assume the switch off or the 1 s window; the store failures are the
section 8 set, all shape assertions.

## As built after owner ruling D8 (2026-11-13, lane/s5-coalesce)

Everything above about the setting, the 10 s telemetry window and the
run-memory window is superseded: coalescing is always on, there is no setting,
and run-memory messages are acknowledged after their CAS. What the build adds
beyond the design:

- **Leader panic.** The leader token is freed by a defer. If a leader panics,
  every member it took gets an error (never an acknowledgement; the append may
  or may not have landed) and the panic goes on.
- **Cancelled callers.** A caller whose context is cancelled at entry, before it
  tries for the token, or right after it wins the token (the token is handed
  back) returns `ctx.Err()` and writes nothing. A member already taken returns
  its group's outcome.
- **Refusal split.** Each single gets its own deadline, not what is left of the
  group's.
- **`telemetry_ridden`** is a lower bound: it counts records that rode a
  committed event only; a ride whose commit failed or whose outcome is unknown
  is counted under the seal failure counters instead.
- **A stale batch never reaches the CAS (the head check).** A batch is
  numbered from, and dry-run against, local state; an append whose outcome was
  unknown can leave the host ahead of the local projection, and any other
  writer's finalize or recovery moves the local head between a writer's dry run
  and its turn under the appender mutex. A retried run-memory write that reused
  a landed entry's (run, seq) reached the CAS and wedged the tape on
  UNIQUE(loop_id, seq). Three mechanisms that tried to detect the event that
  makes a batch stale (a latch, a settle, "this append recovered") each missed
  another such event
  (`docs/problems/stale-batch-after-unclear-append-structural-assessment-2026-11-13.md`).
  The rule now asks whether the state is still the state the batch was
  validated against: the store's dry run reports the projection head sequence
  it validated against (`DryRunProjectionBatchAt`; the store carries it into the
  append in the context, `computerevent.WithValidatedHead`). Under the appender
  mutex, before the event is pinned or prepared, and again after any in-place
  recovery at either site, `batchGuard.check` compares it with the current local
  head. Equal: nothing more. Different, for any reason: the dry run is repeated
  against the current state (`Revalidations()`) and the batch is refused with
  `ErrBatchRefused` if the state no longer accepts it (counted in
  `tape_batches_refused`). A projection that cannot report a head is
  re-validated after a recovery only (the earlier rule). If a recovery cannot
  level the heads the append already refuses, and `recoveryFailedAt` keeps
  further writes refusing for up to 30 s after a failed recovery with
  `ErrNeedsProjectionRepair`; the tape is not poisoned.
  Why equal head means unchanged state: the head row and the projected rows are
  written by one transaction (`finalizeBatch`), and the dry run reads the head
  in the same transaction that applies the batch to roll it back. The only
  writers of run-memory or object rows outside `finalizeBatch` are the unbound
  test seam (`s.projectionTape == nil`) and run-memory retirement, which is a
  tape op that goes through `finalizeBatch` too.
  Where the dry run runs: a group leader dry-runs outside the mutex (it holds
  the leader token, so its dry run and its turn are adjacent); every other
  append is dry-run once, under the mutex, by the appender (writers queue on
  the mutex after validating, so an outside dry run would be repeated nearly
  every time: measured 0.995 revalidations per direct append). Measured on the
  sealed harness, 8 writers: grouped 0.000 revalidations per tape event
  (50 events); direct 0 (200 events, one dry run each under the mutex); half the
  writers direct, half grouped: 0.17 per tape event (26 in 150), about half of
  the grouped events.
  The run-memory writer has no latch and no internal retry: after an unclear
  outcome its next call may be refused once (`ErrBatchRefused`, an error every
  caller already handles as a failed append) and numbers from the caught-up
  table on the attempt after. A refused group splits to singles, each through
  the same check. After a prepare-failure recovery that moved the local head
  past the event's bound sequence the append is refused cleanly (not counted as
  a wedge) and the next append binds to the head as it is now.
