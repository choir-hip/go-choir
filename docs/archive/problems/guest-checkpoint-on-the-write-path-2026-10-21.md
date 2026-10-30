# The guest's Dolt checkpoint runs inside the write path every writer waits on

Date: 2026-10-21. Status: open; the first slice is built on
`lane/fp10-arch13-fanin` (station FP10, ARCH-13 in
[the security and density checklist](../security-density-architecture-checklist-2026-10-21.md)),
off by default, not deployed. Mutation class of the fix: red (the guest
store's write path; the checklist classes ARCH-13 red, the cited design
classes the cadence change orange; this station takes the stricter one).

## Problem

Every durable guest write is one tape append. The appender holds its own
mutex (`internal/computerevent/appender.go`, `Append`, `AppendNew`,
`AppendNewPayload`: `a.mu` held for the whole call) through
`Store.finalizeBatch`, which holds `Store.eventMu`
(`internal/store/computer_events.go:132`). Every 100th finalized event
(`defaultFinalizeCheckpointEvery`, `internal/store/store.go:104`) that
same call runs `CALL DOLT_COMMIT('-Am', ...)`
(`commitDoltCheckpoint`, `store.go:214`) before it returns.

So a wide tree of activations fans in to one queue, and one writer in a
hundred pays for a database-wide Dolt commit while every other writer on
the computer waits behind it on two mutexes. Nothing in the live path
needs that commit: the SQL transaction is already durable, the tape is the
authority, and Dolt commits exist only for `AS OF` reads and history
(`store.go:125-131`; [guest-store-history-bloat-and-memory-shape-2026-10-09](guest-store-history-bloat-and-memory-shape-2026-10-09.md)).

Three further facts, read from the code on main at e4b792b7f:

1. **A mark waits on any commit in flight.** `markDoltHistoryDirty`
   (`store.go:240`) takes `doltCommitMu`, which `commitDoltCheckpoint`
   holds across the SQL call. `GetHistory` (`texture.go:1254`) commits
   from the Texture read path; while it runs, the next `finalizeBatch`
   blocks in the mark, under `eventMu` and the appender mutex. So moving
   the commit off the write path is not enough by itself: the dirty mark
   must stop waiting on the commit.
2. **The cadence is counted in events, not ops.** With the telemetry
   sealer on (`CHOIR_TELEMETRY_BATCH`, commit 3f9c48bbc) one event holds up
   to 64 ops, so "every 100 events" silently becomes up to 6,400 ops per
   commit, and a quiet computer can sit uncommitted indefinitely
   ([projection-event-volume-design-2026-10-19](../reports/projection-event-volume-design-2026-10-19.md)
   §4(a), "an accidental form of (c)").
3. **Nothing measures it.** `EngineMutexStats` covers the object-graph
   engine lock (`internal/objectgraph/engine_mutex.go`), which the
   checkpoint does not take. No counter reports how often the guest
   commits, how long a commit holds the write path, or how many events
   and ops each commit carries. The checklist's "one engine mutex" line
   for ARCH-13 is therefore imprecise: the stall is `eventMu` plus the
   appender mutex, not `engineMu`.

## Evidence

Code reading only (cited lines above). No staging timing exists: the
counters to produce it are part of the fix. Per-commit history cost is
measured in [guest-dolt-storage-growth-2026-10-15](guest-dolt-storage-growth-2026-10-15.md)
(about 1.5 MB of pinned history per 100-event commit).

Hypothesis, not yet measured: a guest `DOLT_COMMIT` on a store of the
paper's size takes from tens to hundreds of milliseconds, so every
hundredth write on a busy computer carries a stall that every concurrent
writer shares. The observe counters below settle it.

## Fix directions

- (c) of the event-volume design: commit on a time debounce plus an ops
  bound, off the write path, like the platform's `internal/doltbatch`.
- A lock-free dirty mark (a generation counter), so a write never waits
  on a commit.
- Observe counters on every computer, whatever the mode: commits, where
  they ran (inline or background), durations, events and ops per commit,
  age of the last commit.
- Later and larger (SA slices 4 and 5): read cost, the store split, group
  commit of acts. Not this slice.

## Chosen first slice

See [the FP10 station file](../definitions/choir-launch-fp10-arch-13-2026-10-21.md).

## Review finding: Reopen after Close leaves the cadence closed (2026-10-22)

Found by FP0's first-pass reviewer on candidate 6d059b885 (blocking).
`Store.Close` runs `closeCheckpoint`, which sets `checkpointCadence.closed`.
`internal/autoputer/projection_base.go:174-196` closes the live store,
rebases it, then calls `Store.Reopen` on the same `*Store`. `Reopen` swaps
the handles but never clears `closed`, so with debounced mode on:
`checkpointDebounced` returns false and finalize falls back to inline
commits; `armCheckpoint` and the ops bound do nothing; and
`CheckpointStats().Mode` still says `debounced`. The writer stall comes
back without a word, and the mode report says it hasn't. Fix: `Reopen`
reopens the cadence and arms it for the adopted (dirty) workspace.

## Review findings: the background commit still stalls writers, and four smaller gaps (2026-10-22)

Found by FP0's review round (DeepSeek, bunch 45, accept with edits) on
f60daaa09 and checked against the code before any fix.

1. The background commit still holds the store's only connection. `Open`
   leaves `textureDB` nil, so `textureHandle()` is `s.db`, and
   `configureEmbeddedDoltDB` pins `s.db` to `MaxOpenConns(1)`
   (`internal/store/texture.go:352-355`). `finalizeBatch` takes `eventMu`
   and then calls `s.db.BeginTx` (`internal/store/computer_events.go`).
   While `DOLT_COMMIT` runs on that connection, the next finalize waits at
   `BeginTx` holding `eventMu`, and every other writer waits on `eventMu`
   behind it. Debounced mode bounds how often this happens (once per
   window, or per ops bound); it does not make it zero. The station's
   deliver line ("a guest write never waits on a Dolt history commit") is
   false as written, and the acceptance reads only `inline_commits` and
   `inline_ns`, which cannot see this wait. The real fix (a second
   connection, or the store split) is SA slice 5, excluded here.
2. `closeCheckpoint` runs the closing commit under
   `context.Background()` with `checkpointCommitTimeout` (2 minutes), so a
   wedged commit holds `Close` for two minutes on the planned-restart and
   projection-base rebase paths (`internal/autoputer/projection_base.go`).
3. `closeCheckpoint` does not wait for work in flight, and
   `CancelDerivedColumnMigration` only signals; a migration step already
   queued on the connection can land after the closing commit and stay
   outside Dolt history until the next commit.
4. A `DOLT_COMMIT` that answers "nothing to commit" is counted as a
   commit at its site, and `CheckpointStats.Mode` says `debounced` while
   the cadence is closed.
5. Assumption to state: with the checkpoint off the write path,
   durability rests on SQL `COMMIT` being durable in the embedded Dolt
   working set. A soak does not test that; a kill -9 of the guest store
   on a test computer does.

## Review round 3: the close cap and the wait counters at the edges (2026-10-22)

FP0's bunch 48 (DeepSeek v4.1 flash, accept with edits) on 2b964182e,
reproduced by the reviewer and then by the failing tests in 1a0fb4c13:

1. The 10 s close cap did not bound Close behind a background commit
   already in flight: the closing commit's context was spent waiting on
   the uninterruptible `doltCommitMu.Lock()` the background committer
   held for up to its 2-minute context (Close waited 3 s against a
   100 ms cap in the test).
2. The finalize wait counters could misattribute at the connection
   boundary: the commit was marked in flight before it took the
   connection and after it had returned it (a hook that did not hold
   the connection still counted a 0.3 ms wait).
3. The cap is a context deadline; DOLT_COMMIT does not check it
   mid-write.
4. The `Store.Close` comment said closing the pools waits for a step in
   flight; `database/sql` does not. A background commit that passed the
   closed check before Close could commit after the closing commit (the
   test reached the closed pool).
5. After a no-op, `last_commit_age_ms` can exceed the debounce while
   nothing is pending.

Fixed in b0ea5d1c0; dispositions in the station's now card.

## Review round 4: Close tore the engine down under a live commit (2026-10-22)

FP0's bunch 50 (DeepSeek v4.1 flash, accept with edits) on 40f1422a7.
Past the close cap, `Store.Close` went on to close `readDB`, whose
connector Close shuts the embedded engine, DoltDB and NomsBlockStore,
while a `DOLT_COMMIT` that ignored its cancel could still run on its
pinned connection (`database/sql` does not wait for a connection in use).
The on-disk state stayed safe in the reviewer's emulation, but the
ordering is no driver contract, a panic there would turn a planned
restart into a crash, and `NomsBlockStore.Close` can block on `nbs.mu`,
so the deadline was not a real bound. Also: a clean planned close counted
two failures (the cancelled background commit and the skipped closing
commit), the counters' "exact" claim was stronger than the code, the
rollback rationale was wrong for round 3, and one test synced on a sleep.
Fixed in cf2dc9d90 after failing tests in ed56fdc0e; dispositions in the
station's now card.

## Review round 5: the fail-closed state had fail-open doors (2026-10-22)

FP0's bunch 52 (DeepSeek v4.1 flash send back, reproduced; Gemini flash
accept with edits, same hole) on 75f3a9a46. A second Close while the
first waited, and a retried Close after a fail-closed Close while an
inline `FinalizeBatch` commit ran, both returned nil and closed the
pools, the connector and the engine under the live commit (probe: ping
after it, `sql: database is closed`; the inline commit then failed with
`*NomsBlockStore is closed`). Cause: `closeCheckpoint` set `closed`
before it knew no commit was live, and the `closeLive` guard existed
only after the first Close gave up. Count mode closed the pools under a
live inline commit the same way. One cut closing commit counted twice;
Reopen over an open store would orphan its handles; the record's
"callers stop on Close's error" was wrong for the planned restart, and
two test claims were broader than the tests. The failing tests are
d529f6a94; the fix is 4d42ba082 (Close and Reopen serialized, the
commit-lock fence on every Close). This is the fifth round on Close; the
station's now card carries the structural assessment.

## Round 6: round 5 is not inert; a failed Close left the store changed (2026-10-22)

Round 5 (83f7bc1f1) had three accepts. Two edits came with them. First,
round 5's Close (closeMu, the commit-lock fence, the Reopen refusal) is
unconditional, so it changes the default count mode on every computer:
online DOLT_GC holds the commit lock for minutes, and a planned restart
or rematerialize during a GC now waits up to about 10.25 s and gets
ErrCheckpointCommitLive (the restart ignores it and exits;
rematerialize returns it). Second, a Close that failed closed left the
cadence closed and the telemetry sealer stopped, so a store that kept
serving fell back to inline commits until exit. The second is fixed in
d36230506 (test 9d2523727); the first is a landing-class change recorded
in the station. Non-commit statements (a statement in flight, an open
write transaction) are still not fenced; that is an owner decision
between pinning the pools and never closing the engine in-process.

## Owner decision: pin the pools (2026-10-22)

The owner chose option (a) for the non-commit class: before teardown,
Close checks out each pool's single connection under the hard deadline,
so no statement, transaction, commit or GC is in flight when the engine
closes. Option (b), never closing the engine in-process, is not pursued.
It was built in round 7 (tests 1aaa5e7c9, fix 6dbdf815d); see below.

## Round 7: pin the pools, built (2026-10-22)

The owner's option (a) is built: tests 1aaa5e7c9 (Sonnet's round-5
probes - a SELECT SLEEP in flight on either pool and an open write
transaction - each made Close return nil on 3b220fa02), fix 6dbdf815d.
Close checks out each pool's single connection after the commit-lock
fence and fails closed when it cannot; the pools no longer close the
engine themselves, and the engine closes last with no connection open.

## Round 8: the arch13j mirror panic, Close over a removed workspace (2026-10-22)

FP0's Linux -race mirror of eb8185144 (arch13j, on main 0bf8a097d)
panicked in internal/store: TestComputerEventProjectionPersistsRevocationEpochAcrossRestart's
Open failed with "database not found: texture", Open called Close, and
closing the engine crashed the process in Dolt's NomsBlockStore.Close
("fatal error closing table persister": the journal could not write a
manifest temp file in the missing .dolt/noms; Dolt sets every SQL-engine
database to crash on that fatal I/O error).

The workspace had been removed under the open engine. Store test paths
were `os.TempDir()/go-choir-m3-store-test/<TestName>`, the same for every
test process of the package on the VM, so another process's cleanup of
the same test is the likely remover; the whole package passed under
-race in one process on this VM (1033 s), and no store code or test
removes another test's path. The crash itself is pre-existing: the same
test panics identically on 3b220fa02, and
[agentcore-store-open-tables-changed-under-load-2026-10-22](agentcore-store-open-tables-changed-under-load-2026-10-22.md)
records the same crash in Close after a failed open (its hypothesis 3).
Round 7's teardown order is not the cause.

Fixed in 7078b0561 (tests 949fb1467): Close checks the workspace's
.dolt/noms is on disk before closing the engine and otherwise leaves the
engine for process exit and returns an error; test paths are per process.

## Round 7 review findings, folded in as round 7c (2026-10-22)

DeepSeek v4.1 flash and Sonnet accepted round 7 with edits. Sonnet found
that the "engine closed once and last" claim had no behavioral test (six
mutants survived the focused set), that a failing Close stalls readers
for its whole wait (a read issued during the wait took 1.55 s; up to
about 10.25 s by default; reads recover within about 250 us afterwards,
because the pin holds the read pool's connection while it waits for the
write pool's), that a pin failure was reported as a running commit, and
that Open's failure paths dropped Close's error. DeepSeek found two
stale comments, this record's "built in the next round", and a log line
that blamed a statement when the deadline was already spent. Tests
564dc81ec, fix 8e68ece69; every one of the six mutants is now killed.

## Round 7d: the abandoned engine was outside the fail-closed contract (2026-10-22)

The reviews of round 7 complete (3b220fa02..92450f670) accepted it with
edits. DeepSeek (required) and Sonnet both found that a Close over a
removed workspace (round 8's fix) returned an error that was not an
ErrCloseLeftOpen, set nothing Reopen checks, and was followed by a
second Close returning nil and a Reopen that opened a new engine beside
the abandoned one (probe: "Reopen after that Close: <nil>"). Sonnet
found the round-7c test of Open's Close retries vacuous (a failed fresh
Open writes no marker, so the next Open deletes and recreates the
workspace whether or not the engine was closed; three mutants
survived), two untested workspace-check branches, and an untested
budget-spent label. DeepSeek found Reopen leaked the store it opened
when adopting it failed. Tests d6c81da3f and b7067604a, fix a6396a2fe.
