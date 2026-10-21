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
