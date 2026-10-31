# Compaction that reaches the paper (2026-10-31)

Mutation class: red (guest store file, snapshot layout, the boot path of a
planned restart; the paper is the protected computer). Built and tested, not
deployed. Rollback: turn `store.telemetry_retire` off on the computer (the
loop that runs the copy stops; a staged copy is dropped at the next planned
boot only if it has gone stale), or revert the commit; nothing on disk
changes format (the new `state.db.layout.json` is a sidecar nothing else
reads).

This closes open risk 1 of the
[append-clustered store note](append-clustered-store-2026-10-31.md) (section
7, item 5 of section 0) using the compact copy and rebase of
[one-tape durability](one-tape-durability-2026-10-31.md) section 6b. It adds
no mechanism: the copy (`store/compact.go`), the sidecar, the planned-restart
swap and the rollback copy exist and are tested. It changes when a copy is
made, how long a copy stays useful, and what the copy may cost.

## 1. Why the copy never reached the paper

1. **Trigger.** `PrepareCompactCopy` runs only when the freelist is over 10%
   of the file or 2 GiB. The paper's file is grown by inserts and its
   freelist is small, so no copy is made, though a grown file is the reason
   every 4 MiB snapshot chunk changes within an hour.
2. **Bound.** The planned boot drops a copy whose tail is over 50,000 events.
   At the paper's ~14,000 events an hour that is 3.5 hours; a planned restart
   comes later than that more often than not, so a copy that was made is
   dropped.
3. **Staleness.** Nothing refreshes a staged copy, so (2) is permanent once
   the copy is a few hours old.

## 2. Failure modes first

Each has a test in `store/compact_cadence_test.go` (P1 to P8) or an existing
one in `compact_test.go` (C1 to C11).

| | Failure | Answer |
|---|---|---|
| P1 | a grown file with no freelist is never copied; or a small or just-compacted file is copied again and again | second trigger: **layout age**. A file of at least 1 GiB with no rewrite for 7 days (or never) is due. `state.db.layout.json` records the last rebase |
| P2 | the copy goes stale and the boot drops it; the bound is an event count that means nothing | the bound is a **replay time**, 30 minutes at a planned 250 events a second (measured 206 to 350 on d@e), which is 450,000 events, about 32 hours of the paper's tape. Replay reads the tape in 1,024-event pages, so memory does not depend on the tail |
| P3 | a fresh copy is rewritten every day; a stale one is kept and dropped at the boot | the daily run keeps a staged copy whose tail is under half the bound and replaces one over half: a copy at the boot is at most about a day old plus its own write time, inside the bound, and a computer that never restarts writes one copy a day at worst, not one every run |
| P4 | the restage holds the original, the old copy and the new one: three times the file | free space is checked first (1.2 x live size); if short and a stale staged copy exists, **it is dropped first** and the check repeated; if still short, refuse. Peak is the original plus one copy |
| P5 | the copy starts with no memory to give | refuse below 256 MiB `MemAvailable` (`/proc/meminfo`; unknown off linux is allowed). The copy connection has a 16 MiB page cache and no mmap; `fadvise(DONTNEED)` drops the finished copy's pages from the kernel cache |
| P6 | a failed or cut-short copy leaves a staging file, a sidecar, or alters the original | the copy writes only `state.db.compact-staging`, removed on any failure and at the next attempt; the staged name and its sidecar change only after integrity_check, the chain check and an fsync; the original is a read-only source throughout (one read transaction). A crash mid-copy leaves the old store authoritative and a staging file nobody trusts |
| P7 | the copy's witness differs from the original's | `VACUUM INTO` preserves rows and the schema text; test compares `SQLiteStateExtractor` content root and schema hash of the original and the copy at the same head. After the swap the boot's ordinary replay brings the copy to the canonical head, and the replay-eligibility gate runs on that |
| P8 | the layout clock restarts on a discard or a crash boot, or never | written only after a swap succeeds (planned boot, or the finish of an interrupted swap) |

Kept from before: a crash boot never swaps (the planned-restart marker
decides); the swap distrusts the stream_seq lease durably before it; the
original and its WAL are kept as `state.db.precompact` until the replay is
done; an interrupted swap is finished or undone before anything opens the
file. A planned restart may resume work and a crash closes it, as in
CLAUDE.md: nothing here resumes anything.

## 3. The change

- `compactReason`: `freelist` (as before) or `layout` (big file, no rewrite in
  7 days).
- `compactRebaseMaxTail` derives from `compactRebaseMaxReplay` x
  `compactReplayEventsPerSecond`.
- `PrepareCompactCopy`: keep a fresh staged copy; drop an unusable one; memory
  check; space check that frees a stale copy before refusing; the copy itself
  is `vacuumIntoCopy` (bounded cache, no mmap).
- `RebaseOntoCompactCopy`: writes the layout marker after a swap.
- Rides the existing daily `StartTelemetryRetirement` loop, which already
  calls `PrepareCompactCopy` after retirement.

**Switch.** None added: the registry is at its cap of 64. The copy runs under
the existing computer setting `store.telemetry_retire` (default on, ring all),
so it is on for d@e, and for every computer on the next guest release;
it cannot be held back from the paper alone without setting that off there,
which would also stop retirement. If the owner wants d@e first, that is a
release-ring decision (the guest release reaches d@e before the paper), not a
code one.

## 4. Open and not done

- The paper's own copy time is unmeasured. The budget is 90 minutes
  (`compactCopyBudget`); a 21 GB file at the synthetic rate in section 5 needs
  that or more, and a copy cut short leaves nothing and is retried the next
  day. The first deployed run on d@e, then a clone of the paper's disk (the
  existing `TestCompactDrillOnFile`), should set the budget.
- The WAL grows for the copy's whole read transaction (`journal_size_limit`
  trims it after). The 1.2 x margin on live size (about 4 GB on the paper)
  covers it at the measured rates; not proven.
- The CPU of the copy and its integrity_check is not paced. Both run as one
  statement each; their memory is bounded, their CPU is one core for their
  duration.
- A decay faster than the layout age (weekly) would want a shorter age; the
  decay past 24 hours is still unmeasured.

## 5. Measurements

`TestCompactMeasure` (env-gated; `compact_measure_test.go`) on the synthetic
store of the churn bench in today's layout, **scaled to 1/5.6 of the paper**:
600,000 og rows and 205,000 run-memory rows, 3.38 GB (the paper: 3.34 M rows,
21 GB). The writes are the paper's full hourly volume (10,640 og, 5,320
run-memory), so against a file this small they touch a larger share of the
chunks than on the paper; read the percentages as a floor for both rows and
the absolute decay as steeper than the paper's. M-series Mac, SSD, not the
guest.

| | grown file (today) | after the compact copy |
|---|---:|---:|
| 4 MiB snapshot chunks unchanged after 1 h | 0 of 809 (0%) | 648 of 777 (83.4%) |
| after 12 h | 0 (0%) | 626 (80.6%) |
| after 24 h | 0 (0%) | 381 (49.0%) |
| file size | 3,224 MB | 3,092 MB (-4%) |

The grown file holds 0 chunks for the whole 24 hours, as on the paper [P].
The compact copy holds above 80% for 12 hours and then loses 20 points at
hour 13 and again at hour 19 (the drops are steps, not a slope: a split
cascade rewrites a region at a time; the cause is not isolated). Whether the
paper, 5.6 times larger, decays at the same hours or at the same percentage
is not measured; the 3.34 M-row bench reports 83% after 24 h. Either way a
daily snapshot after a rebase re-sends about half the chunks or fewer instead
of all, and the layout age of 7 days is to be tuned from a measurement of
the real file, not from this.

Copy cost (`vacuumIntoCopy`, the production path, one run):

- duration 42.7 s for 3.2 GB, 75 MB/s on this disk. The guest's disk is
  slower and the file 6.5 times larger: assume 20 to 30 times the time, 15 to
  20 minutes, against the 90-minute budget. Unmeasured on the guest.
- peak resident memory 102 MB against 17 MB before: **+84 MB**, flat; the
  copy connection's cache is 16 MiB and the rest is the Go runtime and
  SQLite's working set. It does not grow with the file (the copy streams each
  B-tree).
- disk: the original plus one copy (3.09 GB beside 3.22 GB).
