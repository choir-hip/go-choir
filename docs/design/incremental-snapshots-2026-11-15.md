# Incremental snapshots: smaller page groups in the chunk store we already have, and one recovery-time trigger

Status: proposed, unreviewed. Written 2026-11-15 (19:10 UTC). Design only; no
code is changed by this document. Line numbers are from `origin/main` at
`7b186ba66`.

Readers: a builder and two reviewers who have seen none of the discussion
behind it. Everything needed is here or cited.

## Summary

The owner accepted this recommendation on 2026-11-15: page deltas over a
periodic full base, a re-base when the deltas grow to a fraction of the base,
and one recovery-time objective as the trigger, with a capture before any
planned restart. The tape stays canonical and whole.

Reading the code changes how that is best built. The snapshot format is
already a content-addressed store of fixed, page-aligned chunks, and a capture
already skips every chunk the host holds. A snapshot is therefore already a
delta. It is a poor one only because a chunk is 1,024 pages, so a chunk
survives a day of writes about one time in five.

So the chosen mechanism is to cut the state file into smaller page groups.
That is page deltas at a granularity of one group. It has no chain, so there
is no re-base rule to write, no new record type and no new restore path. The
restore code on `main` today already accepts such a record. Explicit delta
files (the form the owner accepted) remain the fallback, and section 3 states
the measurement that would send us there.

Two things this design does not deliver, said plainly:

- Smaller groups cut upload bytes. They do not cut capture duration, because
  the witness still reads the whole store (section 6).
- The trigger bounds the replay of the tail. A recovery also pays a fixed
  cost to install and witness the store, between about 8 and 25 minutes on
  the paper today, which no trigger changes (section 2).

Slices, in order: S1 a page-churn meter in the capture; S2 the trigger as one
recovery-time rule; S3 smaller page groups, gated on S1's numbers; S4 a
capture before a planned restart; S5 the host's root limits, only if S1 picks
a group size that needs them.

## 1. What exists

All of this is on `main`.

**Capture.** `TakeSnapshot` (`internal/snapshot/snapshot.go:350`) calls
`capture` (`:552`). Since `842e0fda6` the capture copies every page to a
scratch file under one short read transaction and hashes it as it goes
(`copyPages`, `:784`), ties the copy to the transaction's size, geometry and
head row (`crossCheckCopy`, `:882`), computes the witness over the copy by SQL
(`Config.Witness`, `:280`), then reads the copy back and uploads it
(`streamScratch`, `:935`). Telemetry partition files are captured beside it
(`capturePartition`, `:1178`).

**Chunking.** `ChunkSize = 4 << 20` (`:80`). The cut is a whole number of
pages: `pagesPerChunk = ChunkSize / pageSize` (`:937`), which is 1,024 pages
at the paper's 4 KiB page size. Chunk `i` holds pages
`[i*PagesPerChunk, (i+1)*PagesPerChunk)` (`:109-116`). Boundaries are fixed
page numbers, not content-defined.

**Sealing and the skip.** Each chunk is compressed with zstd and sealed with a
nonce derived from the key, the computer and the plaintext
(`filecas.SealChunk`, `internal/filecas/filecas.go:200`), so the same
plaintext always gives the same digest. The uploader (`handle`, `:502-535`)
seals the chunk, then asks the host whether it holds that digest
(`HasChunk`, `:513`; an HTTP `HEAD`, `internal/snapshot/http_chunks.go:77`)
and uploads only if it does not. Four workers do this (`uploadWorkers`,
`:87`).

**Record.** The `Record` (`:95-136`) carries the store head it captures,
`PageSize`, `PageCount`, `PagesPerChunk`, the witness root, and a manifest
that lists every chunk digest. It is sealed into one `snapshot_recorded` tape
event. Before that event is appended, the guest declares the digest list to
the host in plaintext (`RegisterSnapshotRoot`, `:402`), and the host refuses
the declaration if any chunk is missing
(`internal/platform/snapshot_roots.go`, `HandleSnapshotRoot`, about `:590`).

**Host limits and retention.** One declaration may name at most 65,536
chunks in a body of at most 8 MiB (`snapshot_roots.go:72-74`). The host keeps
the newest 4 roots per computer, or 2 when a root names more than 2,048
chunks (`:43-50`), plus up to 3 older roots that cover the 720,000-event
recovery bound (`:51-70`). A chunk no kept root names is collected after a
grace period.

**Restore.** `restoreOne` (`:1377`) opens the record, checks its geometry
(`:1411-1418`), fetches and opens each chunk in order (`assemble`, `:1554`),
and then recomputes the witness over the assembled file
(`VerifySnapshotFile`, `internal/agentcore/restore_snapshot.go:203`). The
candidates are every `snapshot_recorded` event at or below the target and
inside the recovery bound (`PlanCandidates`, `:1333`;
`restore_snapshot.go:132`). The tape tail above the snapshot is then
replayed.

The geometry check accepts any `PagesPerChunk` for which
`PageSize * PagesPerChunk <= 2 * ChunkSize` and the chunk count matches. A
record cut at 256 pages a chunk passes it on today's binary.

**Why about 19% is reused.** A chunk is reused only if none of its 1,024
pages changed. If a share `p` of pages change at random, a chunk survives
with probability `(1-p)^1024`, so 0.2% of pages changing (about 19,000 pages
of the paper's 9.47 million) already changes 87% of chunks. The store's
writes are scattered: 13 of the 16 indexes on `og_objects` end in a 136-byte
canonical id, so inserts land on random leaves; pointer-map pages are
rewritten on allocation; the daily `incremental_vacuum` moves pages; and a
compaction renumbers every page. The earlier design
`docs/design/snapshots-cost-the-change-2026-11-13.md` (sections 2 and 3)
derives this. Its meter (S4.1) and its smaller groups (S4.2) were never
built; a search for `pagehash` in `internal/` finds nothing.

**The trigger today.** `snapshotDue`
(`internal/agentcore/snapshot_producer.go:393-430`) fires on three cases:

1. the tail reaches 540,000 events (`snapshotDefaultTailEvents`, `:76`);
2. seven days have passed (`snapshotDefaultInterval`, `:71`; settable with
   `CHOIR_SNAPSHOT_INTERVAL`);
3. the tail would replay in more than 4 minutes (`snapshotReplayBound`,
   `:106`) at an assumed 113 events/s (`:113`), but only past a floor: the
   larger of ten times the last capture's duration and the last capture's
   sealed bytes spread over 8 GiB a day (`snapshotCaptureFloor`, `:381-388`).

When the last capture's cost is not recorded for the newest snapshot, case 3
is muted until case 1 or 2 fires (`:411-417`). That is the paper's state now:
its tail is about 441,000 events, the replay rule has been over its bound for
days, and nothing is due. The cost is kept in `snapshot-producer.json` beside
the store (`:126`, `:304`, `:344`). The measured replay rate is held in
memory only (`computerevent/appender.go:493`) and is lost at every restart.

A planned restart is refused while a snapshot is in flight
(`plannedRestartWork.SnapshotInFlight`, `planned_restart.go:516-519`, `:567`),
and the host retries.

## 2. Physical bounds

Inputs, with where each comes from:

| Quantity | Value | Source |
| --- | --- | --- |
| Store size `S` | 38.8 GB, 9.47 million 4 KiB pages | the paper after compaction, 2026-11-15 |
| Event rate `λ` | 2.8 to 9 events/s (10,080 to 32,400 an hour) | the paper's tape |
| Replay rate `μ` | 77 events/s on the paper; 113 to 380 on d@e | measured replays |
| Restore stream | about 15 GB/min | research report, section 2 |
| Capture, static copy | 7 min 30 s for 31.3 GB (copy 3 min 17 s, witness 4 min 12 s) | the paper's first capture |
| Capture, older | 25 min 51 s for 44.1 GB (witness 22 min 37 s) | the paper, before compaction |
| Compression | about 6x at 4 MiB chunks | 4.04 GB sealed for 5,858 chunks |

**Recovery time has three terms.** `install + witness + tail / μ`. Install is
`S` over the stream rate: 2.6 minutes. The witness at restore reads the whole
store again: 4 to 23 minutes on the measurements above (about 5 minutes if the
7 min 30 s capture scales with size). So the fixed part is about 8 to 25
minutes whatever the snapshot structure. Only the third term is the trigger's.
The objective in this document, `R`, is that third term: the replay of the
tail. An objective of "10 minutes" means about 18 to 35 minutes to a working
store on the paper today.

**Least delta bytes an hour.** A delta cannot be smaller than the pages that
changed. That number is not measured. Two bounds hold it:

- Lower: each event writes at least one page. 10,080 events an hour is at
  least 41 MB of raw pages an hour, 133 MB at 9 events/s.
- Upper: the write log bounds it, since each log frame is one page write.
  The static-copy design defines the measure (`last_pin_wal_growth_bytes`
  over `last_pin_events`, `docs/design/capture-static-copy-2026-11-15.md`
  lines 836-840), and no capture has run with it yet, so there is no
  reading. The code's old planning figure of 2 MiB/s would be 7.4 GB an
  hour.

The range is 41 MB to 7.4 GB an hour, a factor of 180. Nothing below can be
chosen well until S1 narrows it.

**What each structure reaches**, for one capture over an interval in which a
share `p` of pages changed, scattered at random (the worst case for groups):

| Structure | Raw bytes uploaded | At `p` = 0.2% |
| --- | --- | --- |
| 4 MiB chunks (today) | `S * (1-(1-p)^1024)` | 33.8 GB |
| 1 MiB groups (256 pages) | `S * (1-(1-p)^256)` | 15.6 GB |
| 512 KiB (128) | `S * (1-(1-p)^128)` | 8.8 GB |
| 128 KiB (32) | `S * (1-(1-p)^32)` | 2.4 GB |
| single pages, or a page-delta file | `S * p` | 78 MB |

If changes cluster (appends, a few hot tables), every row moves toward the
last one. The gap between the rows is the whole question, and it is a
property of the store's writes that only a measurement gives.

**Capture frequency the objective needs.** The trigger rule (section 5) gives
a start-to-start interval `T = R * μ / λ - d`, with `d` the capture
duration. With `μ` = 77 and `d` = 10 minutes:

| `R` | `λ` = 2.8 | `λ` = 9 |
| --- | --- | --- |
| 30 min | 13.6 h (1.8 a day) | 4.1 h (5.8 a day) |
| 10 min | 4.4 h (5.4 a day) | 76 min (19 a day) |

With today's 4 MiB chunks a capture uploads about 5 GB sealed, so 30 minutes
costs 9 to 29 GB a day and 10 minutes costs 27 to 95 GB a day. That is why
the owner's interim objective is 30 minutes on full captures and 10 minutes
once captures cost the change.

## 3. Options

**(a) Smaller fixed page groups in the existing chunk store.** Change one
number: the pages per chunk of the state file. Every unchanged group is
skipped by the existing probe. A capture is a new record that lists mostly
held chunks.

- Deletes or avoids: a delta record type, a chain, a re-base rule, a second
  restore path, any change to what a `snapshot_recorded` event means.
- Restore is unchanged, and an old binary restores the new records.
- Costs, all proportional to the chunk count `N = S / group` and paid by
  every capture even when nothing changed:

| Group | `N` at 38.8 GB | Record in the tape | Root declaration | Fits host limits |
| --- | --- | --- | --- | --- |
| 1,024 pages (today) | 9,470 | 0.7 MB | 0.6 MB | yes |
| 256 | 37,900 | 2.7 MB | 2.5 MB | yes, up to a 64 GiB store |
| 128 | 75,800 | 5.4 MB | 5.1 MB | no: over 65,536 chunks |
| 64 | 151,600 | 10.8 MB | 10.2 MB | no: both limits |
| 32 | 303,000 | 21.6 MB | 20.3 MB | no: both limits |

  The record is in the tape forever. At 32 pages and 19 captures a day that
  is 410 MB of tape a day for manifests alone, more than the paper's events.
  Each capture also seals every group (compress and encrypt the whole
  store), sends one `HEAD` per group, and the host stats every named chunk
  at registration and writes the digest list as one Postgres row. The host
  holds one file per chunk: 38,000 to 303,000 files a root's worth.
  Compression falls as groups shrink; the ratio below 4 MiB is unmeasured.
- A compaction or a large `incremental_vacuum` renumbers pages and makes the
  next capture a full upload at any group size. That is true today and stays
  true.

**(b) Explicit page-delta files chained on a base.** A delta lists changed
page numbers and their bytes against a named parent. This reaches the
`S * p` row of the table at any scatter.

- Adds: a delta record type, a chain the restore must walk in order, a
  re-base rule, host retention that understands parents, a way to know which
  pages changed (a page-hash sidecar the capture must trust, or a diff
  against the parent), and a restore path that applies deltas before the
  witness.
- A chain link that is lost loses every later snapshot until the next base.
- This is what the owner accepted, and it is the fallback here.

**(c) Continuous shipping of write-log frames.** The lowest bytes and the
shortest tail. It needs a hook on every checkpoint of the store, a second
ordered stream beside the tape, and a restore that applies frames. The tape
already is the ordered log of this system. A second log to keep consistent
with it is the kind of mechanism the restore-bases assessment
(`docs/problems/restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md`)
says not to add. Not chosen.

**(d) Copy-on-write clones at the filesystem.** A reflink of the store makes
the scratch copy free and instant. It does not move bytes to Node B, so it is
not a recovery base. It could later replace `copyPages`; whether the guest's
data volume supports reflinks is not verified. Not part of this design.

**Does (a) alone suffice?** It does if the store's changed pages cluster, or
if the scatter is mild enough that a group size the host can carry keeps the
daily upload inside the budget. It does not if the scatter is like the table
above: at `p` = 0.2% a capture, 256-page groups save only half and 32-page
groups still upload 30 times what the pages weigh. The evidence so far (19%
reuse at 1,024 pages) is what both cases predict. So the choice is made by
measurement, and the rule is fixed now, before the numbers exist:

> For each group size `g` in {256, 128, 64, 32}, let
> `U_g = (changed groups * group bytes) / c_g + record bytes + declaration bytes`,
> measured by S1 over at least three captures on the paper spaced by the
> trigger. Take the `g` with the smallest `U_g`. If `U_g` times the captures
> a day that the 10-minute objective needs (section 2) is at most the daily
> upload budget (owner decision 2), build (a) at that `g`. If no `g` meets
> it, do not build (a) below 256; write the design for (b) with S1's
> numbers, and hold the objective at the interim value.

**Default chosen now:** 256 pages (1 MiB). It never uploads more groups'
worth of bytes than today, needs no host change at the paper's size, and
keeps the record under 3 MB. Its one cost is a single full upload when the
cut changes (about 6.5 GB sealed).

## 4. The slices

Rules for every slice, from the assessment cited above: one mechanism per
slice; the failure list is reviewed before code; a repair round repairs and
adds no mechanism; three rounds without convergence stops the lane. Each
slice is its own lane and its own landing with `Ring-Expedite`. The builder
re-checks every anchor here against the code on the day.

Old snapshots stay restorable forever in every slice: no slice changes
`restoreOne`, the record's fields or their meaning.

### S1. A page-churn meter in the capture (orange; rollback: revert)

One mechanism: the capture measures how many pages and page groups changed
since the last capture, and logs it. It decides nothing.

`copyPages` already reads every page once. It keeps an 8-byte hash of each
page (76 MB for the paper) and, after the `snapshot_recorded` event is
appended, writes them to a sidecar beside the store with a header: the
store's inode and device, page size, page count, the snapshot's head
sequence and the capture's start time. The next capture reads the sidecar,
and if the header matches the same file, logs and reports in the producer's
status: pages changed; groups changed at 1, 8, 32, 64, 128, 256 and 1,024
pages; the zstd size of the changed groups at 32, 256 and 1,024 (sampled:
every 16th changed group, to bound the cost; the sampling is stated in the
log); the events and seconds between the two captures.

Nothing reads the sidecar except this log. A missing, unreadable or
mismatched sidecar logs "no baseline" and is replaced.

The synthetic bench on `origin/lane/bases-bench`
(`internal/store/page_churn_measure_test.go`, claim C97) measures the same
quantity on a generated store at units of 1, 32, 128 and 1,024. Run it on the
build VM for an early reading. It does not replace the meter: the gate in
section 3 reads the paper.

Protected surfaces: the capture path (`internal/snapshot`), which is a
guest-security path; run `scripts/check-guest-security-epoch`. The record,
the manifest and the upload are untouched.

Failure list:

1. The sidecar is from a different file (a compaction renamed a new store
   into place): the inode differs, the meter logs "no baseline".
2. The page count changed: compare the common prefix, and count added or
   removed pages as changed.
3. A crash between the append and the sidecar write: the old sidecar stays;
   its head sequence no longer matches the newest snapshot, so the next
   capture reports the interval it actually spans, from the header.
4. The sidecar write fails or the volume is full: the capture has already
   succeeded; log and continue. The meter never fails a capture.
5. The free-space check before a capture does not count the sidecar: add its
   size to the need, in the same expression (`snapshot_producer.go:581`).
6. The hash memory (8 bytes a page) on a small guest: it is 0.2% of the
   store; state the figure in the log.
7. A hash collision reports a changed page as unchanged: at 64 bits this
   skews a statistic by nothing visible, and the statistic gates no write.
8. The sampled compression misleads: log the sample count beside the ratio.
9. The scratch-copy crash wedge (a fixed scratch path left by a crash): the
   sidecar is written by rename from a temporary name and is never opened
   exclusively.

Proof: repo tests for each failure first; then two captures on d@e and the
meter's line read from its log; then the paper, read-only, after S2 has
caused captures there.

### S2. The trigger is one recovery-time rule (orange; rollback: revert)

Section 5 is this slice's design. It lands second because it is what makes
the paper capture at all, and those captures are S1's data.

### S3. Smaller page groups for the state file (red; rollback: revert)

One mechanism: `streamScratch` cuts the state file at `g` pages a chunk, with
`g` from the rule in section 3 (256 unless S1 says otherwise), where it now
computes `ChunkSize / pageSize` (`snapshot.go:937`). Partitions keep the
4 MiB cut (`capturePartition` is not touched): they are separate files with
their own write pattern, and changing them is a second mechanism.

The record already states `PagesPerChunk`; nothing else in it changes.

Conjecture delta: the upload of a capture becomes proportional to changed
groups of `g` pages; before, of 1,024. Evidence class: deployed captures and
a deployed restore. Heresy delta: none; the tape, the record's meaning and
the restore are unchanged.

Protected surfaces: tape and evidence (the snapshot record), the host's root
declaration, the guest-security epoch.

Old binaries and old snapshots: a record at 1,024 pages restores as before.
A record at `g` pages is restored correctly by today's binary, because
`restoreOne` reads the cut from the record (`:1411-1418`). The first test
written in this slice builds a `g`-page record by hand and restores it with
the unchanged restore code, before the capture is changed. A binary from the
VACUUM INTO era, which has no geometry fields, would mis-size the chunks;
those are refused by the record version check or fail the manifest size
check loudly. The builder confirms which, and if neither holds, stops and
reports: that is the case for bumping `Version` (`:73`), which makes old
binaries refuse.

Rollback: revert. Records written at `g` stay restorable by the reverted
binary. The next capture uploads in full once at the old cut.

Failure list:

1. The first capture after the change shares no chunk with the last root: a
   full upload, about 6.5 GB sealed on the paper. Expected once; the proof
   states it.
2. The chunk count exceeds the host's limit (a store over 64 GiB at 256
   pages): the guest computes the count before copying and refuses with the
   limit named, instead of failing at the declaration after the upload. The
   limit is a shared constant, not a second copy of the number.
3. The host's "large store" rule counts chunks (`largeSnapshotChunks = 2048`,
   `snapshot_roots.go:49`), so at 256 pages a store over 2 GiB is "large"
   and keeps 2 roots where it kept 4. State this in the change. Changing
   retention is owner decision 3 and not part of this slice.
4. The record grows fourfold. It must stay under the 64 MiB payload cap:
   at 256 pages that holds to about 900 GB. Test the refusal.
5. The uploader's buffers are sized by `ChunkSize` (`newChunkUploader`,
   `:938`): size them by the cut, or four workers hold 16 MiB for 1 MiB
   chunks. Not a correctness fault; fix it in the same change.
6. The last group is short: the existing short-chunk path handles it; test
   a page count that is not a multiple of `g`.
7. The registration's stat pass and Postgres write take four times as long
   under the host's deadline: measure on d@e at the paper's chunk count
   with a synthetic declaration before the paper takes it.
8. Four times the `HEAD` requests: measure the stream time; it must not
   raise the capture duration by more than the meter's reading predicts.
9. A restore fetches four times as many chunks in sequence: measure the
   d@e restore before and after.
10. The copy digest check (`ErrCopyDigest`, `:212`, checked in `streamScratch`) reads in chunk-sized
    cuts: the digest must not depend on the cut. Test both cuts give one
    digest.
11. Compression is worse at 1 MiB than S1's sample said: the proof compares
    sealed bytes, not raw.
12. A compaction between captures still forces a full upload: unchanged, and
    named so nobody reads it as a regression.

Proof: repo tests first (the old-binary restore test, each failure above);
then on d@e two captures and a restore that matches its witness, with the
second capture's uploaded bytes read from its log; then the paper,
read-only: the second capture after the deploy uploads at most what S1
predicted for 256 pages, within a factor of 1.5.

### S4. A capture before a planned restart (orange; rollback: revert)

Section 5 gives the rule. One mechanism: when the host asks for a planned
restart, a capture that would shorten the tail runs first, and the restart
waits for it through the refusal that already exists.

It lands after S3 so that these captures cost the change.

Failure list:

1. The capture fails or is refused: the restart must not wait again. One
   attempt per restart target, remembered in memory only.
2. The process crashes during the capture: nothing resumes; the next boot is
   a crash boot, and the next tick is new work. No marker is written before
   the capture finishes, because the marker is written only when the restart
   is granted (`planned_restart.go:737` onward).
3. The host stops retrying before the capture ends: read the host's retry
   schedule (`.github/scripts/node-b-ci-deploy` and the vmctl refresh) and
   state the bound; not verified here.
4. A self-development operation or live capsule also refuses the restart:
   the capture does not start while `Busy` reports one
   (`snapshot_producer.go:836-848`), as now.
5. Six deploys in a day: section 5.
6. The marker's 15-minute age limit (`plannedRestartMaxAge`,
   `planned_restart.go:28`) is not involved, because the capture precedes
   the marker. Test that a restart granted after a long capture writes a
   fresh marker.
7. A hibernate request during the capture: the existing fence order holds;
   test it.

Proof: repo tests; then on d@e a planned refresh with a tail present,
showing the refusal with `snapshot_in_flight`, the capture, then the granted
restart and a planned boot.

### S5. The host's root limits (red; rollback: revert), only if needed

Built only if the rule in section 3 picks a group under 256 pages, or the
paper's store nears 64 GiB. One mechanism: raise `maxSnapshotRootChunks` and
`maxSnapshotRootBody` (`snapshot_roots.go:72-74`) to cover a 256 GiB store at
the chosen group (the bound today's numbers were written for). The host
deploys before any guest that needs it. Its failure list starts from the
earlier design's (the stat pass, the Postgres row under the 30 s deadline,
the `LIKE` live check over a larger text, sequential restore fetches) and
each is benchmarked at the new size on d@e first.

If S1 points below 32 pages, stop: the per-capture record no longer belongs
in a tape event, and that is option (b)'s design, not a limit to raise.

## 5. The trigger (S2 and S4)

### The rule

One objective, `R`: the replay of the tail after a recovery takes at most `R`.

A capture started now is usable `d` later, and events keep arriving while it
runs. So a capture is due when the tail at that later moment would replay in
`R`:

```
(tail + λ * d) / μ >= R
```

- `tail` = the head sequence minus the newest snapshot's head sequence.
  Both are already read (`snapshot_producer.go:826-834`).
- `λ` = `tail / since`, where `since` is the time since the newest
  snapshot's capture began. This is the measured arrival rate over the
  interval in question, and it needs no window constant.
- `d` = the last capture's measured duration on this computer, from
  `snapshot-producer.json`. Unknown (no capture yet, or the file is lost):
  zero. The first capture then starts slightly late, once, and measures it.
- `μ` = the replay rate measured on this computer. It is in memory only
  today, so this slice stores it in the same file when a replay measures
  it, and reads it at boot. Never measured: 77 events/s, the slowest replay
  measured on any computer (the paper, 2026-11-15). A measurement above 77
  replaces it; this drops the present rule that a fast reading never
  relaxes the bound, because the measured value is the fact.

The threshold `R * μ` is capped at 540,000 events, three quarters of the
recovery bound (`recoveryplan.MaxRecoveryTailEvents`,
`internal/recoveryplan/recovery_plan.go:22`). Past the bound a boot refuses
to recover at all, so this is a correctness limit, and a fast replay rate
must not carry the threshold through it.

`R` is 30 minutes when this slice lands (owner decision 1). It is one
constant in the code with the owner's date beside it.

### What is deleted

- The event ceiling as its own case. It survives only as the cap above.
- The weekly interval and `CHOIR_SNAPSHOT_INTERVAL`. A quiet computer has a
  short tail, and the tape replays it inside `R`; a snapshot there buys
  nothing.
- The floor (`snapshotCaptureFloor`, `snapshotCaptureWallFraction`,
  `snapshotBytesPerDay`) and the "cost unknown" mute. The floor is why the
  paper's tail is 441,000 events. It was a spending rule placed inside a
  recovery rule. Spending is owner decision 2.
- The assumed rate of 113 and the 4-minute bound.
- The status fields `floor_limited` and `floor_why`. The status gains `λ`,
  `d`, `μ` with where each came from, the threshold, and the time the rule
  expects to fire.

What stays: the tick, the backoff after a failed capture, the 90-minute
deadline, the free-space refusal, the refusal while self-development or a
capsule is live, and `CHOIR_SNAPSHOT_DISABLE` as the emergency override.

### When the objective cannot be met

If `λ * d / μ >= R`, a capture cannot finish before the tail it leaves
already exceeds `R`. At the paper's worst figures (`λ` = 9, `μ` = 77, `d` =
26 minutes) that needs `R` under 3 minutes, so it does not arise at 10 or 30.
The rule still handles it without a setting: it is due at every tick, the
producer runs one capture after another, and the status says the objective
is not reachable and gives the three numbers.

### Capture before a planned restart (S4)

A planned restart keeps the data volume, so the snapshot is not needed for
the restart itself. It is taken because a deploy is the moment a new binary
is most likely to damage the store, and a short tail then makes the recovery
short.

Rule: when the host's request arrives (`noteRefreshAsked`,
`planned_restart.go:797`), a capture runs first if `tail > λ * d`. Below
that, a capture would end with as long a tail as it started with, so it
cannot help. The restart request is refused with `snapshot_in_flight` until
the capture ends, as it already is for any capture, and the host's retry
grants it. One attempt per target; a failed or refused capture lets the
restart go.

A crash never resumes any of this. No state is written for the pending
capture, and the marker is written only when the restart is granted.

**A six-deploy day.** Each deploy waits for one capture of duration `d` on
each busy computer: about 8 to 26 minutes on the paper, so 50 minutes to 2.6
hours of deploy delay for that computer across the day. Each capture resets
the tail, so the recovery-time rule fires that much less. In bytes, six
extra captures cost six deltas after S3 (unknown until S1) and six full
uploads, about 30 GB sealed, before it. That is the reason S4 follows S3.
If the delay is not acceptable, the alternative with no new mechanism is to
not build S4; the recovery-time rule alone still holds `R`.

### Node B cost a day

At `R` = 30 minutes with today's chunks: 1.8 to 5.8 captures a day at about
5 GB sealed, so 9 to 29 GB uploaded a day. Stored bytes do not grow at that
rate: the host keeps 2 roots (plus up to 3 for the recovery window) and
collects the rest after the grace period, so the stock is a few roots' worth
of unique chunks plus up to a day in grace. Node B had 192 GB free on
2026-11-15.

At `R` = 10 minutes after S3: 5.4 to 19 captures a day times `U_g`. `U_g` is
S1's output. At 256 pages each capture also adds 2.7 MB to the tape and 2.5
MB of declaration: 15 to 51 MB of tape a day.

## 6. The witness: whole store every capture

The witness is computed by SQL over the rows of the copy and recomputed over
the restored file. It is the one check that crosses from the page path (what
was copied and uploaded) to the row path (what the store means). The earlier
design (section 7 of `snapshots-cost-the-change-2026-11-13.md`) tested the
alternatives, a page root, the head row, `quick_check`, `integrity_check`,
and found each misses a mixed-state or wrong-content copy.

Can it become incremental? Not soundly, in either direction:

- A witness maintained on the write path (a running hash per table, updated
  by each write) never reads the copied pages. It would agree with the tape
  and say nothing about the copy, which is the thing being checked.
- A witness recomputed only for changed pages needs to know which rows live
  on which pages, which is the page path again, and a fault there is the
  fault the witness exists to catch.

So the witness stays whole, at capture and at restore. Consequences:

- **Capture duration floor.** `d` is at least copy plus witness plus the
  sealing pass: 7 min 30 s measured at 31.3 GB, so about 9 to 10 minutes at
  38.8 GB if it scales with size, and 26 minutes was measured on the larger
  uncompacted store. Smaller groups do not reduce it.
- **The objective.** `d` enters the rule only through `λ * d / μ`, which is
  0.3 to 3 minutes of replay on the paper. A 10-minute objective is
  reachable with whole-store witnesses. The floor does not block it.
- **The fixed recovery cost** of section 2 stands. Shortening it is a
  different problem (a faster witness, or a restore that serves before it
  finishes verifying) and is out of scope.

One saving is possible later without touching the witness: the sealing pass
compresses and encrypts every group to learn digests the last capture
already knew. The S1 sidecar could let a capture skip sealing unchanged
groups. That makes the sidecar a trusted input, which S1 deliberately does
not, so it is a separate design and is not proposed here.

## 7. What this needs from the forward restore event

The restore event is designed on `lane/forward-restore-design`. This design
needs four things from it and designs none of them:

1. The event names the snapshot it restored from and the target sequence, so
   a reader of the tape can tell which `snapshot_recorded` events are on the
   live line. A snapshot recorded after the target and before the restore
   event describes a state that was abandoned.
2. `PlanCandidates` must be able to exclude those abandoned snapshots. The
   rule for doing so belongs to the restore design; this design only
   requires that it exists before S2 raises the capture rate, since more
   captures means more snapshots inside any abandoned span.
3. Sequences stay increasing across a restore, so `head - snapshot head`
   remains the tail. If the restore design makes the effective tail
   something else (for example, the tail from the restored base), it states
   the function the trigger should call.
4. The host's root retention keeps the root a restore event names for as
   long as that event can be replayed through. Whether that needs a host
   change is the restore design's finding.

## 8. Owner decisions, each with a default

1. **The objective `R`** (replay of the tail). Default: 30 minutes when S2
   lands; 10 minutes when S3's proof on the paper passes. Both are the
   owner's interim figures of 2026-11-15. The fixed 8 to 25 minutes of
   install and witness are on top.
2. **Daily upload budget to Node B.** Default: 8 GiB a day, the figure
   already in the code (`snapshotBytesPerDay`), used here only as the gate
   in section 3, not as a runtime limit. The runtime has no cap; it logs
   bytes uploaded per capture and per day so a spike is seen. If the owner
   wants a runtime cap, it is an owner-set spending cap and a separate
   slice.
3. **Retention on Node B.** Default: unchanged (2 roots for a large store,
   plus the recovery-window chain). After S3 each extra root costs only its
   changed groups, so keeping more becomes cheap; the suggested later value
   is a day of roots, and it is invented.
4. **Whether a deploy waits for a capture (S4).** Default: yes, after S3.

Numbers in this document that are invented, not measured: the candidate
group sizes {256, 128, 64, 32}; the meter's "every 16th group" sampling;
"three captures" for the gate; the factor 1.5 in S3's proof; the "day of
roots" above. The 77 events/s fallback and the 540,000-event cap are
measured or derived, as stated where they appear.

## 9. Calibration claims to enter before each slice lands

Each is entered in `docs/calibration/claude-ledger.md` with a probability
after reading the scorecard; none is entered by this document.

- **S1.** On the paper, across three captures, the share of 256-page groups
  changed between consecutive captures is under half the share of
  1,024-page groups changed. Test: the meter's log lines. This is the claim
  that clustering exists.
- **S2.** Within 24 hours of the deploy the paper takes a capture with kind
  "recovery time", and for the following 48 hours its tail never exceeds
  `R * μ` plus the events of one capture, read from the producer's status
  every hour.
- **S3.** The paper's second capture after the deploy uploads less than half
  the sealed bytes of its last capture before the deploy, and a restore on
  d@e from a 256-page record matches its witness.
- **S4.** On d@e a planned refresh with a tail present is refused once with
  `snapshot_in_flight` and granted within one capture duration plus the
  host's retry interval.
- **S5.** If built: a declaration at the new limit registers on d@e inside
  the host's deadline.

## 10. Not verified

- The paper's page churn at any group size. Everything in section 3 waits on
  it.
- Compression below 4 MiB chunks.
- Write-log bytes per event: the measure exists in the static-copy design,
  and no capture has reported it.
- That restore streaming reaches 15 GB/min on Node B; it is the research
  report's figure.
- The host's retry schedule for a refused planned restart, and so the real
  deploy delay in S4.
- The host's grace period before an unnamed chunk is collected, and so the
  exact stock on Node B at a given upload rate.
- How a binary older than the page-cut record treats a 256-page record
  (S3, old binaries).
- Whether witness time scales with store size or with something else; the
  two readings (4 min 12 s at 31.3 GB, 22 min 37 s at 44.1 GB) do not fit a
  line, and the second was on a live file before the static copy.
- The synthetic bench (C97) has not reported.
