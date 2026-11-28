# A replay-time capture is not muted by an unknown capture cost

Design and failure list, 2026-11-15. Class: green (this file). The change it
proposes is orange (runtime behavior of the snapshot producer; rollback: revert).
Evidence tags: [C] read in code, [M] measured and written in a repo document,
[E] arithmetic from those, [I] invented, [?] not verified.

Files read: `internal/agentcore/snapshot_producer.go` (line numbers below are at
718ea5ed4), `docs/design/capture-static-copy-2026-11-15.md`,
`docs/archive/design/restore-bases-store-once-2026-11-15.md`,
`docs/definitions/choir-storage-solutions-2-2026-11-12.ledger.md`,
`docs/calibration/claude-ledger.md` (C99, read only).

## 0. The defect in one paragraph

The largest computer (the paper) reads tail 441,187 events, estimated replay
65 minutes at the assumed 113 events/s (441,187 / 113 = 3,904 s) against the
240 s bound, `due: false`, `floor_limited: true`, `capture_known: false`. The
replay-time trigger is muted by `case !in.HaveCapture` (`snapshot_producer.go:411-417`),
so only the 540,000-event ceiling (`:51-52`, three quarters of
`recoveryplan.MaxRecoveryTailEvents` = 720,000, `internal/recoveryplan/recovery_plan.go:22`)
or the 7-day interval (`:70`) can fire. The paper's tail grew 441,187 events in
44 h, 2.79 events/s [E]; the ceiling is 98,813 events away, about 9.9 h [E].

## 1. What the floor protects against, and why the cost is unknown

**What it protects.** The floor (`snapshotCaptureFloor`, `:381-390`) is the least
time between two replay-time captures: the larger of ten times the last
capture's duration (a tenth of wall time) and the last capture's sealed bytes
spread over 8 GiB a day (`snapshotCaptureWallFraction`, `snapshotBytesPerDay`,
`:121-122`). It exists because a capture then held a read transaction for the
whole witness (22 to 37 minutes on the paper, `:113-117` comment) and rewrote
gigabytes on the host (owner, 2026-10-31, quoted at `:71-74`). The rule is not
a comparison of capture cost with replay saving; it is a rate limit built from
the last capture's own duration and bytes. The unknown branch exists because
the rule needs those two inputs and, without them, chose to wait for the ceiling
or interval to take "one capture and measure it" (`:411-415`).

**When the cost becomes known, and where it is stored.** Two places, both
written by the same code path after a successful capture: in memory
(`p.haveCapture`, `p.lastCaptureDuration`, `p.lastCaptureBytes`, set at `:632`)
and durably in `snapshot-producer.json` beside the store on the data volume
(`saveSnapshotCapture`, `:300-340`, called at `:640`; file name `:127`). The
record carries `computer_id`, `started_at`, `duration_seconds`, `sealed_bytes`,
`head_sequence`.

**Does a restart lose it?** No, when the record exists. On the first
evaluation after boot `evaluate` loads it (`:462-486`) and keeps it only if it
is this computer's and `belongsTo` the newest snapshot on the tape (`:358-363`:
the snapshot event lies between one minute before the capture's start and its
duration plus 30 minutes). A restart (and a planned restart: the host waits for
`inFlight`, `planned_restart.go:516-519`, `:567`) therefore does not lose a
known cost. `forget()` (`:653`, called by `rematerialize.go:269` when a restore
replaces the store) clears it on purpose and re-reads it.

**Why it is unknown on the paper [C, E; the last step is [?]].** The persistence
was introduced by commit `c7eabe010` on 2026-11-14 22:31 (found with
`git log -S saveSnapshotCapture`). The paper's newest snapshot is the one at
head 11,522,779, taken 2026-11-13T22:36Z by an older binary (C99 baseline text,
`docs/calibration/claude-ledger.md`). That binary wrote no record, so there is
nothing to load: the cost was never recorded for a snapshot taken before
`c7eabe010`. No capture has run on a binary that writes the record. I did not
read the paper's data directory, so "no `snapshot-producer.json` exists" is
inferred from the dates, not seen [?].

**Is there a durable record to read instead (candidate b)?** No. The
`snapshot.Record` on the tape (`internal/snapshot/snapshot.go:99-`) holds the
head, size, page cut, manifest, witness root and `CreatedAt`; it has no capture
duration and no uploaded-bytes count. `Stats.BytesUploaded` and the timings
exist only in the in-process `ref.Stats` and the one log line. The two numbers
the floor needs are not recoverable from any durable record of an old snapshot.
The log line of the paper's newest old capture has rotated (C99 baseline). The
figures in documents are for other captures (25 min 51 s, 4.04 GB sealed, head
10,978,774, `choir-storage-solutions-2-2026-11-12.ledger.md:61`; 37 min 40 s,
4.8 GB, `restore-bases-store-once` doc line 123) and are not in the computer.
Reading cost from a record cannot fix this computer, and a restart does not lose
the cost on a computer whose newest snapshot was taken after `c7eabe010`. So the
defect is not "lost at restart"; it is "never measured for the snapshot in
hand".

## 2. After the first capture: does the floor mute replay captures forever?

No. The floor is a duration, not a verdict. Once the cost is known the rule is
`in.Since < floor` (`:418`) and otherwise `Due, Kind=replay` (`:424`). With the
paper's measured old figures:

- wall term: 37 min 40 s x 10 = 6.3 h [E, from `:121` and doc line 123];
- bytes term: 4.8 GB / 8 GiB x 24 h = 13.4 h (the code comment at `:123`
  states the same figure) [E]. The larger binds: 13.4 h.

The replay-time trigger therefore fires once the tail's estimated replay is over
240 s (27,120 events at 113/s, `:105-108`; already true) and 13.4 h have passed
since the capture. At the paper's 2.79 events/s the tail at 13.4 h is about
134,000 events [E]: 29 minutes of replay at 77 events/s, 20 minutes at 113.
The ceiling needs 540,000 / 2.79 = 54 h [E], which agrees with the 53 h
cadence in `restore-bases-store-once` doc line 204. So on the paper the replay
trigger is the one that fires, about every 13.4 h (1.8 a day), 4.8 GB sealed
each, 8.6 GB a day [E], which is the 8 GiB a day the constant was set to
allow. The ceiling only wins when the tail grows faster than 540,000 events in
the floor, above about 11 events/s [E].

Plainly: on a 38 GB store the 4-minute bound in `snapshotReplayBound` is not
reachable, because a capture costs 4.8 GB sealed and the budget is 8 GiB a day.
What the replay-time trigger is for there is a rate-limited cadence of roughly
one capture per floor. The recovery bound the paper actually has in the steady
state is about 134,000 events plus the events during a capture (at 2.79/s a
40-minute capture adds about 6,700 [E, duration assumed]), about 29 to 31 minutes
at the measured 77 events/s (`restore-bases-store-once` doc lines 67, 114) on a
quiet day; on a busier day (6 events/s, the code's own figure, `:80`) it is
about 290,000 events, 63 minutes; and the hard bound is the ceiling, 540,000
events, 117 minutes at 77/s (156 minutes at the 720,000 recovery plan limit)
[E]. The restore request is a ten-minute request (`:101-103`), so the 4-minute
figure is a target the paper does not meet at this cadence; the
`restore-bases-store-once` design (a base chain) is the work that changes that,
not this change.

What the present muting costs: until the first capture the paper has at most
the ceiling cadence, 98,813 events away, 9.9 h at the current rate [E]. So this
change moves the paper's first capture earlier by at most 9.9 h and moves nothing
in the steady state. The benefit is small; the reason to do it is that the
rule's own text says an unknown cost must be measured, and today it is measured
only when something else fires.

## 3. The candidates

**Pick (a): an unknown capture cost does not mute a replay-time capture.** One
mechanism: the replay-time case `ev.Est > snapshotReplayBound` is due when
`!in.HaveCapture || in.Since >= snapshotCaptureFloor(...)`. Delete the
`case !in.HaveCapture` block (`:411-417`, 7 lines) and make the floor test
`in.HaveCapture && in.Since < floor`, so an unknown cost is due explicitly (not
through `floor = 0`, which a negative `Since` after a clock step would leave
floor-limited and never due). The capture it takes measures the cost, and the record at
`:640` makes it known for every later evaluation and every restart. Change the
comments at `:120-127` and `:728-736` and the `FloorWhy`/`CaptureKnown` text so
they no longer promise waiting. No new constant, setting or field.

Why the others lose:

- **(b) read the cost from a durable record.** There is none for an old
  snapshot (section 1), and a record that is present is already read
  (`:462-486`). A restart does not lose it. Deriving a cost from the tape (for
  example from the manifest's chunk count) would be an estimate invented for a
  one-time gap, new code to delete later, and it does not apply to the computer
  in question.
- **(c) delete the floor rule.** The pin is short, but the capture still reads
  and writes the whole store (copy 39 GB, witness 22 to 35 minutes of IO, 4.8 GB
  sealed). At the 4-minute bound the paper would capture every 27,120 / 2.79 =
  2.7 h, about 9 a day, about 43 GB a day sealed, the 43 to 86 GB figure in
  `restore-bases-store-once` doc line 199 [E]; that is the daily GBs the owner
  objected to (`:71-74`). The pin shrinking removed one reason for the floor, not
  the Node B cost reason. Deleting is right only with a base chain or a cheaper
  capture.
- **(d) leave the rule.** The paper then waits for the ceiling (about 9.9 h
  more), and any computer whose newest snapshot predates the record file waits
  for the ceiling or a week. It is safe, and it leaves a trigger whose own text
  says "wait", with the first measurement of the new pin (C99) delayed with it.

## 4. Failure list

1. **A capture every tick.** An unknown-cost capture that succeeds writes the
   record at `:640` and sets `p.haveCapture` at `:632`, so the next evaluation
   uses the floor (at least 10 x duration: about 110 s for d@e's 11 s capture,
   hours for the paper). A capture that fails does not set it: `failures` goes
   up and `retryAt` is `ended + snapshotBackoffWait(failures)` (`:605-613`),
   20 minutes, then 40, 80, 160, capped at 3 h (`snapshotBackoffFloor` `:93`,
   `snapshotBackoffCeiling` `:94`); the gate is at `:531`. A refusal (token held,
   free space, busy) does not touch the backoff and repeats every 10-minute tick
   (`:130`) at the cost of reading the store size and free space, with a log line
   only when the text changes (`:938-945`). So an attempt that costs scratch IO
   is at most every 20 minutes and, after repeated failures, every 3 h; an
   attempt that costs nothing is every 10 minutes.
2. **Back-to-back captures because each tail is over the bound again.** After a
   success the tail starts at the capture's head (`:631`). The paper regains
   27,120 events in 2.7 h [E], but the known floor then holds the next one for
   13.4 h. For this to fail the floor would have to be shorter than the capture
   itself, which `10 x duration` forbids. d@e is floor-bound the same way. No new
   spacing is needed.
3. **The record write fails.** The write failure is only logged (`:640-642`):
   in memory the cost is known and the process keeps the floor, but a restart
   makes it unknown again and the unmuted trigger takes another capture. This
   needs a full or read-only data volume, where a capture's own free-space
   check (`:567-576`) would already refuse. Residual, accepted.
4. **Free-space band on a 38.8 GB store.** `need = S + 2 GiB` (`:75`, `:567`):
   about 41 GB against 85.9 GB free at the last reading (doc line 108,
   2026-11-15T05:49Z), so it admits unless a staged compact copy (about 42 GB)
   is on the volume; then 43.9 GB free against 41.7 GB needed is admitted by
   about 2 GB (`capture-static-copy` design, lines 440-461), and a copy that runs
   past about 18 minutes at 2 MiB/s of WAL growth is aborted by the 64 MiB guard
   with nothing uploaded. A refusal is retried at every tick (no backoff, no IO
   beyond a stat); an abort goes through the failure backoff (item 1), so a
   permanent abort band costs one 39 GB scratch copy at most every 3 h at the
   cap, never a tight loop. Today's ceiling would hit the same band; the change
   only moves the first attempt earlier. `consecutive_refusals` rises while a
   refusal lasts and ends with the first `taken` or `not_due`
   (`recordOutcome`, `:688-704`); it is the field to read for a stuck trigger.
5. **A capture starting right after boot.** The producer starts three minutes
   after boot (`:129`), and `Ready` is false while `Runtime.Start` runs
   (`bootInProgress`, `runtime.go:1272`, used at `:852`) or under a maintenance
   hold. Replay runs before `Runtime.Start` (`run.go:786-808`) and the rebase earlier in
   `autoputer.Run`, so both are done before the first tick [C]. The
   Busy gate then refuses for live self-development or a live capsule
   (`:845-847` region). An unknown-cost capture is no earlier than a known one.
6. **d@e and small computers.** The unknown state is a one-time condition per
   old snapshot: the change gives one extra capture per such computer, then the
   known rule is what it is today. d@e: 1.1 GB store, an 11 s capture
   (`:121-123` comment), so the wall term is 110 s, and its bytes term is 0.28 h per 0.1 GB sealed
   at 8 GiB a day [E], so the bytes term likely binds on d@e; its steady cadence
   is unchanged. Whether d@e's
   tail now exceeds 27,120 events is not measured here [?].
7. **Deploy restarts interrupt a capture.** A planned restart waits for
   `inFlight` (`planned_restart.go:516-519`, `:567`), bounded by the 90-minute
   time budget (`:99-104`), so a deploy waits up to 90 minutes rather than
   cutting the capture. A crash mid-capture costs one of three attempt-counter
   starts (`spendGuestAttempt`, `:586`; `snapshotAttemptLimit` `:133`) and the
   leftovers are removed at boot and at the next attempt (`:546-551`;
   `capture-static-copy` design 2.8 table, lines 547-554). After each boot the
   in-memory failure counter is gone, and an interrupted or failed capture leaves
   no record, so a computer whose newest snapshot has no record starts a new
   unknown-cost capture about three minutes after boot. On a busy landing day
   (a deploy every 2 hours) the paper would start a capture, hold the deploy up
   to 90 minutes, and finish it; the record is then written and the unknown state
   is over. The loop continues only if captures fail, and then each failure resets
   with the restart; non-crash aborts are not bounded by the attempt counter, so
   the cost is one 39 GB read per boot at worst. I add no durable backoff (a
   second mechanism); see open question 1.
8. **Node B cost.** A capture on the paper uploaded 5,858 of 10,524 chunks, 4.04
   GB sealed (11-12), and 4.8 GB (11-13) (ledger line 61; doc line 123);
   `ChunksUploaded` and `BytesUploaded` are in the log line at `:645-648`. The
   change adds one such capture per old-snapshot computer; then the steady state
   is the floor's about 8.6 GB a day on the paper [E], against about 2.2 GB a day
   when the ceiling alone fired (doc line 204). That is Node B writes the
   owner's 8 GiB constant already allows, and it is the behavior of any computer
   that already has a record, not something this change introduces.
9. **Restart rules (CLAUDE.md "Restarts").** No boot-time resumer is added: the
   trigger is the same standing intent re-evaluated at the tick. A crash restart
   never resumes a capture (the next boot removes the scratch) and a planned
   restart does not cut one (`inFlight`). The change reads and writes no marker.
10. **A record that does not match.** `belongsTo` fails (a copied data
    directory, a record left by an earlier snapshot, a clock step over 30
    minutes) and the cost reads unknown; with the change that costs one capture,
    which then writes a matching record. Before it, it cost a wait to the
    ceiling.
11. **Compaction holds the store-copy token.** A capture is refused for a
    compaction's whole copy (up to its 90 minutes, `capture-static-copy` design
    lines 466-476) and retried at the tick. Unchanged by this design; the
    refusal shows in `refusal` with `consecutive_refusals`, and the unmuted
    trigger makes it visible earlier.
13. **After a restore.** `forget()` (`:653`, `rematerialize.go:269`) clears the
    state and re-reads a record that the flip does not move
    (`rematerialize.go:249-253` moves only the marker, workspace, state, `-wal`
    and `-shm`); the restored base's snapshot event is re-finalized by the replay,
    so `belongsTo` fails and the cost is unknown. With a tail over the bound the
    first tick after the restore takes a capture. That is intended (a restored
    computer with a long tail has the same recovery problem). The store-sized
    quarantine is on the volume (`rematerialize.go:232-244`) so the expected
    outcome on a large store is `refused` for free space until it is removed; a
    refusal is not a failure.
14. **An understated cost record after a failure in an earlier process.** Only an
    in-memory failure is compensated (`sealed = max(sealed, p.lastCaptureBytes)`,
    `:625-631`). If boot 1's capture uploads 4 GB and fails, then the process
    restarts, boot 2's capture finds those chunks held, uploads little in a shorter
    run, and the record holds that smaller cost, so the floor is shorter than the
    true cost of getting a snapshot. Not a storm (the floor is still at least 10 x
    the duration). Residual, stated, no code change: the host keeps the failed
    attempt's chunks until its grace passes.
15. **The fleet at the deploy.** Every computer whose newest snapshot has no
    matching record and whose tail is over 27,120 events captures a few minutes
    after its runtime serves; the deploy's idle refresh restarts them together,
    and the code adds no spacing between computers and none is proposed. The count
    is not in the repo or the docs I read (it needs the host's list of snapshot
    roots against each computer's record), so it is unknown [?]. The size of the
    wave is the number of such computers times the changed chunks since each one's
    last snapshot; for the paper alone, 4.04 to 4.8 GB sealed (item 8).
16. **Deploy latency.** A planned restart waits for a capture in flight
    (`planned_restart.go:567`), up to the 90-minute budget (`:83-87`, comment
    `:231-237`), once per capture. A planned restart never interrupts a capture;
    only a crash does. On a busy landing day the cost is a deploy of the paper
    that waits up to 90 minutes after the unknown-cost capture starts, the reverse
    of an interruption, and it matters under the expedite policy.
17. **A planned restart between the end of `Take` and the record write.**
    `p.inFlight` is cleared inside the `Take` closure (`:595-599`) and the record
    is written after it (`:640`): a planned restart landing in that gap loses the
    record and costs a second full capture on the next boot. Decision: the code
    change keeps `inFlight` true through the record write (`defer
    p.inFlight.Store(false)` at the function level instead of in the closure; two
    lines, and the other reader, `memory_cache_return.go:296`, waits a few
    milliseconds longer), with a test that observes `inFlight` after `Take`
    returns.
18. **Status text.** `FloorLimited` with `FloorKey == "unknown"` stops appearing;
    a test pins that `capture_known: false` and `due: true` can hold at once
    (status at `:728-736`, `:762`).

## 5. Proof

**Repo tests, written before the code, table tests on `snapshotDue`** (inputs
`snapshotDueInput`, `:365-377`):

| Case | Expect |
| --- | --- |
| tail 441,187, rate unmeasured, `HaveCapture` false, `Since` 44 h | `Due`, kind `replay`, `FloorLimited` false |
| same with `HaveCapture` true, duration 37m40s, bytes 4.8 GB, `Since` 44 h | `Due`, `replay` (past the 13.4 h floor) |
| same with `Since` 5 h | not due, floor-limited, `Floor` 13.4 h |
| tail 10,000, `HaveCapture` false | not due, no floor text (under the bound) |
| tail 540,000, `HaveCapture` false | `ceiling`, as before |
| `Since` over the interval, `HaveCapture` false | `interval`, as before |
| measured rate faster than assumed | the assumed rate is used, as before |
| tail 27,120 then 27,121 at 113/s | boundary on `Est > bound` |

Added case: `HaveCapture` false with a negative `Since`: due. Two existing
tests pin the old behaviour and are replaced:
`TestUnknownCaptureCostBindsReplayTimeCaptures` (asserts `not_due` and
`FloorLimited` for this state) and `TestPersistedCostSkewAndIdentity` (asserts
that a 150,000-event tail with another computer's record stays unknown; after the
change that run takes a capture. Its intent, that another computer's record is
not trusted, is kept by asserting the loaded cost before the capture and the
record's cost after it). Plus an `evaluate` test with no record file and a stub `Take`: one capture is
taken, `haveCapture` becomes true, the record file is written, and the next
tick is `not_due` (floor); a failing `Take` sets the backoff and writes no
record. Run the whole uncached suite of `internal/agentcore` by shards, per
CLAUDE.md.

**Deployed proof.**

- Test computer (d@e, drills allowed). Setup, because d@e may have captured
  since `c7eabe010` and then reads `capture_known: true`: remove
  `snapshot-producer.json` beside d@e's store and restart the runtime (a planned
  restart; the producer reads the record on its first evaluation), then confirm
  the block reads `capture_known: false`. The tail must be over 27,120 events
  (not measured here [?]; if smaller, write to it until it is). Then the
  producer block shows `due: true`, `due_why` naming the
  240 s bound, `floor_limited: false`; then `last_outcome: taken` with kind
  `replay` in `last_detail`, `capture_known: true`, and `snapshot-producer.json`
  present beside the store. Then `not_due` with a floor until it passes.
- The paper, read-only from the console. The window counts from the runtime
  serving (the producer's first `evaluated_at` in the block; the boot marks
  `replay_begin` and `runtime_serving`), not from the VM boot: the tape replay
  runs before `Runtime.Start` (`internal/autoputer/run.go:786-808` calls
  `rt.Start` as the replay's completion callback and the producer starts inside
  it), and the paper's replay is long [C]. Pass: `last_outcome: taken` within 60
  minutes of the first `evaluated_at` (60 minutes is [I]: the three-minute
  initial delay, one 10-minute tick, one cleared refusal, one more tick, slack),
  or, if refused, `refused` with a reason that clears (the store-copy token held
  by a compaction, or free space) and a capture after it clears. Fail:
  `consecutive_refusals` above 6 with a reason that is neither of those, or a
  `failed` outcome that repeats past its backoff. A planned restart waits for a
  capture already running.
- C99 (`docs/calibration/claude-ledger.md:181`, not edited here): this design
  makes the paper's first capture happen at the deploy rather than up to 9.9 h
  later. Read `last_pin_seconds` and `last_scratch_bytes` from that capture and
  compare with the 775 s threshold (half of the 1,551 s baseline). A refusal or
  an abort in the free-space band is a false C99 by its own wording and is not a
  failure of this change.

**Rollback.** Revert the commit. The producer returns to waiting on the ceiling
or interval when the cost is unknown; a record written in the meantime is read
by the old code (same format, no new field). **Rollback trigger** (all [I], from
this document's arithmetic): more than one `taken` with kind `replay` on one
computer inside its floor, except the one capture after a restore (item 13);
`consecutive_refusals` above 6 (an hour) whose reason does not name the
store-copy token holder or free space (a compaction holds the token up to 90
minutes, item 11, and is normal); or the owner's stop. A capture started while
`Ready` is false is a code guarantee (`:852`), not observable from status, so it
is not a trigger.

## Round 1

| Seat | Verdict | Blocking | Should-fix taken |
| --- | --- | --- | --- |
| omp-deepseek-v41-flash | ACCEPT | none | proof window from runtime serving (proof); the two tests named as replaced (section 5); rollback trigger excludes token and free-space refusals (proof); post-restore state (item 13); fleet (item 15); d@e bytes term corrected (item 6); the explicit `!in.HaveCapture` due branch with a `Since < 0` table case (section 3, section 5); d@e setup (proof); understated cost (item 14); inFlight through the record write (item 17) |
| claude seat | ACCEPT | none | understated cost (item 14); a planned restart never interrupts a capture and the deploy-latency cost (item 16); fleet (item 15); d@e setup (proof); paper proof is pass or fail (proof); restore exception to the rollback trigger (proof, item 13) |

Both seats noted dissent for (d): the benefit is at most 9.9 h on one computer
and the first static-copy capture at 39 GB lands at the deploy. The owner has
accepted the design; the dissent is recorded and not acted on.

## 6. Open questions and the conservative default

1. Should a failed unknown-cost attempt keep its backoff across a restart?
   Default: no (a second mechanism; one extra 39 GB read per boot at worst,
   item 7).
2. Is Node B's about 8.6 GB a day on the paper (item 8) acceptable, or is the
   8 GiB constant to be re-owned? Default: unchanged; this change does not alter
   it.
3. The paper's replay rate: 77 events/s is a 2026-11-15 measurement (doc line
   114), and the producer uses the slower of the assumed 113 and a measured one;
   the paper's `replay_rate_source` is not read here. Default: no change.
4. Whether the 4-minute bound should be restated for stores whose capture
   cannot meet it (section 2). Default: leave `snapshotReplayBound`; the
   base-chain design addresses it.
5. The 60-minute window and the rollback thresholds are [I]; the owner may set
   them.

## 7. Not verified

The paper's data directory (no record file, inferred from commit dates), the
paper's tail rate beyond the numbers in the task report, d@e's tail and event
rate, that replay and the rebase complete before `Ready` in `Runtime.Start`
(stated by the static-copy design, not read), and the new capture's real
duration, since none has run on the new code.
