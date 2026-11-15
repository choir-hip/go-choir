# Incremental snapshots: smaller page groups in the chunk store we already have, and one recovery-time trigger

Status: proposed; review rounds 1 and 2 (two seats each, all REPAIR) repaired,
see "Review round 1" and "Review round 2" at the end. Written 2026-11-15 (19:10 UTC).
Design only; no code is changed by this document. Line numbers are from
`origin/main` at `6263d0b25` (the same code as `7b186ba66` for every file cited).

Readers: a builder and two reviewers who have seen none of the discussion
behind it. Everything needed is here or cited.

## Summary

The owner accepted this recommendation on 2026-11-15: page deltas over a
periodic full base, a re-base when the deltas grow to a fraction of the base,
and one recovery-time objective as the trigger. The tape stays canonical and
whole. The owner's fourth element, a capture before a planned restart, is not
in this design: a planned restart keeps the data volume, so the restart does
not need the snapshot, and the request handler writes the marker and answers
before any capture could run (review round 1).

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
recovery-time rule; S3 smaller page groups, gated on S1's numbers; S4 the
host's root limits, only if S1 picks a group size that needs them.

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
is muted until case 1 or 2 fires (`:411-417`; the branch
`origin/lane/capture-trigger` removes that mute). That is the paper's state
now: its tail is about 441,000 events, the replay rule has been over its bound
for days, and nothing is due. The cost is kept in `snapshot-producer.json`
beside the store (`:126`, `:304`, `:344`), together with the capture's head
sequence, which seeds the tail's start after a boot (`:489`). The measured
replay rate is the slowest of the replays in this process
(`computerevent/appender.go:466-500`, only replays of at least 1,000 events
count) and is lost at every restart; `snapshotDue` keeps the lower of it and
the assumed 113 (`:406-409`), so a fast reading never relaxes the bound.


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

**Capture frequency the objective needs.** The trigger (section 5) is due
when the tail reaches `R * mu_min` and the existing floor allows it. The floor
is ten times the last successful capture's duration, start to start. With
`mu_min` = 77 (the paper's measured rate) and `d` = 26 minutes (the longest
measured capture), the floor is 4.3 hours. The paper's present rate is 9,400
events an hour (2.6 events/s, the owner's figure of 2026-11-15); the busy end
is 32,400 an hour (9 events/s). Hours between capture starts are
`max(threshold / λ, floor)`:

| `R` | Threshold | 9,400 an hour | 32,400 an hour |
| --- | --- | --- | --- |
| 30 min | 138,600 events | 14.7 h (1.6 a day); floor does not bind | 4.3 h (the threshold is 4.28 h, the floor 4.33 h) |
| 10 min | 46,200 events | 4.9 h (4.9 a day); floor does not bind | 4.3 h, the floor binds (the threshold would be 86 min) |

At 32,400 an hour and `R` = 10 the floor holds a capture the rule wants, so
the tail at the start is about 140,000 events (30 minutes of replay at 77)
and `R` is not met; the status says so (section 5). The floor binds at `R` =
10 above 10,700 events an hour (46,200 over 4.33 h), and at `R` = 30 above
32,000 an hour. With `d` = 10 minutes the floor is 100 minutes and binds only
above 27,700 an hour at `R` = 10. Before the paper has measured a replay of at
least 1,000 events, `mu_min` is the code's 113: 203,400 events at `R` = 30
and 67,800 at `R` = 10 (21.6 h and 7.2 h apart at 9,400 an hour).

With today's 4 MiB chunks a capture uploads about 5 GB sealed. At the paper's
present rate that is about 8 GB a day at `R` = 30 (1.6 captures) and 24 GB a
day at `R` = 10 (4.9); at the busy end about 27 GB at either `R` (the floor caps
captures near 5.5 a day). The owner's interim objective is 30 minutes
on full captures and 10 minutes once captures cost the change.

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

  The limits and counts in this table are per declaration, and the declaration
  and the record also carry the telemetry partitions' chunks
  (`snapshot.go:402-406`), so the partitions share them.

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
measurement, and the rule is fixed now, before the numbers exist.

The gate considers only the group sizes S1 measures, 256 and 32 pages, each
against today's 1,024. S1 logs the zstd size of changed groups at exactly
those three sizes; 128 and 64 pages are not sampled, so they are not
candidates.

> For `g` in {256, 32}, let
> `U_g = (changed groups * group bytes) / c_g + record bytes + declaration bytes`,
> with `c_g` the compression ratio S1 sampled at `g` and the changed groups
> read from the meter over the intervals S2's rule produced on the paper: at
> least three, not counting the first interval after S2 deploys, which spans
> the paper's 441,000-event backlog. Multiply `U_g` by the captures a day the
> 10-minute objective needs (section 2) and compare it with the daily upload
> budget (owner decision 2). Build (a) at 256 if that passes; otherwise at 32
> if that passes and S4 (the host's limits) is built; otherwise S3 is not
> built: the objective stays at 30 minutes, and page-delta files (option (b))
> get their own design from S1's numbers.

**How the interval is scaled.** The meter measures churn over one interval
between two captures. Under S2's rule that interval is `R * mu_min / λ` at
the 30-minute objective, 14.7 hours on the paper. The gate multiplies that
`U_g` by the captures a day at the 10-minute objective, whose intervals are
about a third as long. The groups changed over a shorter interval are a
subset of those changed over a longer one, so `U_g` measured over the long
interval is at least `U_g` over the short one. The gate is therefore
conservative: it may reject a size that would have passed at 10 minutes and
cannot accept one that would fail there. No scaling rule is used and none is
invented. If the gate fails only at the 10-minute objective, taking `R` = 30
with S3 anyway is the owner's call under decision 1.

**Default if S1 does not separate the sizes:** 256 pages (1 MiB). It never
uploads more groups' worth of bytes than today, needs no host change at the
paper's size, and keeps the record under 3 MB. Its one cost is a single full
upload when the cut changes (about 6.5 GB sealed).

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
status: pages changed; groups changed at 32, 256 and 1,024 pages; the zstd
size of the changed groups at those same three sizes (sampled: every 16th
changed group, to bound the cost; the sampling is stated in the log); the
events and seconds between the two captures.

Why S1 stays (review round 1, DeepSeek should-fix 10): the gate chooses
between 256 and 32 pages from the paper's own writes over several real
intervals, and one trial size at S3's first capture (a full upload anyway,
in a red slice) cannot compare two sizes. The 76 MB hash is 0.2% of the
store and the pass already reads every page.

Nothing reads the sidecar except this log. A missing, unreadable or
mismatched sidecar logs "no baseline" and is replaced.

The synthetic bench on `origin/lane/bases-bench`
(`internal/store/page_churn_measure_test.go`, claim C97) measures the same
quantity on a generated store. Run it on the
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

### S2. The trigger states the recovery-time objective (orange; rollback: revert)

Section 5 is this slice's design. Stated truthfully, it is today's rule with
three changes: `R` is the stated objective in place of the 4-minute bound
(`snapshotReplayBound`), the weekly interval is deleted, and `mu_min` is
persisted. It also drops the bytes part of the floor (section 5). It lands
second because it is what makes the paper capture at all, and those captures
are S1's data. S2 and S1 do not depend on each other in code.

### S3. Smaller page groups for the state file (red; rollback: revert)

One mechanism: `streamScratch` cuts the state file at `g` pages a chunk,
where it now computes `ChunkSize / pageSize` (`snapshot.go:937`). The group
is `g = min(chosen, ChunkSize / pageSize)`, with `chosen` from the gate in
section 3 (256 unless S1 says otherwise). The clamp matters because
`storeGeometry` accepts any page size that divides 4 MiB up to 65,536
(`snapshot.go:769`) and the store's page size is not pinned: at 64 KiB pages
today's cut is 64 pages, and 256 pages would be a 16 MiB chunk that
`restoreOne` refuses (`PageSize*PagesPerChunk > 2*ChunkSize`, `:1414`) and
the host refuses to store (`maxFileCASChunkBytes = 8 << 20`,
`internal/platform/file_cas_http.go:17`). Partitions keep the 4 MiB cut
(`capturePartition` is not touched): they are separate files with their own
write pattern, and changing them is a second mechanism.

The record already states `PagesPerChunk`; nothing else in it changes.

Conjecture delta: the upload of a capture becomes proportional to changed
groups of `g` pages; before, of 1,024. Evidence class: deployed captures and
a deployed restore. Heresy delta: none; the tape, the record's meaning and
the restore are unchanged.

Protected surfaces: tape and evidence (the snapshot record), the host's root
declaration, the guest-security epoch.

Old binaries and old snapshots: a record at 1,024 pages restores as before.
A record at `g` pages is restored correctly by today's binary, because
`restoreOne` reads the cut from the record (`:1411-1418`), and the host
inspects digests and counts only. A binary that ignores the geometry fields
cuts at `ChunkSize`, but it concatenates the chunks in order, so the bytes are
the same file; the worst case is a loud size refusal. The first test written
in this slice builds a `g`-page record by hand and restores it with the
unchanged restore code, before the capture is changed. There is no `Version`
bump: `restoreOne` accepts only `Version` 1 (`:1390`), so a bump would make
the new binary refuse every existing record unless it accepted both, which
is a restore-side change this design does not make. If the builder finds an
older binary that would restore a `g`-page record wrongly and silently, S3
stops and reports.

Rollback: revert. Records written at `g` stay restorable by the reverted
binary. The next capture uploads in full once at the old cut.

Failure list:

1. The first capture after the change shares no chunk with the last root: a
   full upload, about 6.5 GB sealed on the paper. Expected once; the proof
   states it.
2. A store with large pages: the group is clamped (above), so no chunk
   exceeds `ChunkSize`. Test a 64 KiB-page store (the existing page-size test
   at `snapshot_pages_test.go:633` already includes 65,536) and a 16 KiB one,
   and that each record restores and passes the geometry check.
3. The chunk count exceeds the host's limit (65,536 per declaration, shared
   with the partitions; a store over about 64 GiB at 256 pages): the guest
   computes the count of state and partition chunks before copying and
   refuses with the limit named, instead of failing at the declaration after
   the upload. The limit is a shared constant, not a second copy of the
   number.
4. Retention. The host keeps 2 roots instead of 4 when a root names more than
   `largeSnapshotChunks = 2048` chunks (`snapshot_roots.go:49`), and the
   comment calls that 8 GiB, which is true only at 4 MiB chunks. At 1 MiB
   chunks it is 2 GiB: a store between 2 and 8 GiB goes from four kept roots
   to two (the paper already keeps two, at 9,470 chunks), and at 32 pages the
   limit is 256 MiB. State this in the change and correct the comment. Changing
   the constant is owner decision 3 and not part of this slice.
5. The record grows fourfold. It must stay under the 64 MiB payload cap
   (`computerevent/privacy.go:51`): at 256 pages that holds to about 900 GB.
   Test the refusal.
6. The uploader's buffers are sized by `ChunkSize` (`newChunkUploader`,
   `:938`): size them by the cut, or four workers hold 16 MiB for 1 MiB
   chunks. Not a correctness fault; fix it in the same change.
7. The last group is short: the existing short-chunk path handles it; test
   a page count that is not a multiple of `g`.
8. The registration's stat pass and Postgres write take four times as long
   under the host's 30-second deadline (`snapshotRootDBDeadline`,
   `snapshot_roots.go:348`): measure on d@e at the paper's chunk count with a
   synthetic declaration before the paper takes it.
9. Four times the `HEAD` requests: measure the stream time; it must not
   raise the capture duration by more than the meter's reading predicts.
10. The host's live-check recheck. `chunkProtectedBySnapshotRoot`
    (`snapshot_roots.go:524-546`) runs `chunks LIKE '%"digest"%'` once per
    candidate chunk, with the 30-second deadline per query, over root rows
    that grow about fourfold to about 2.5 MB of text. When a root cut at 4 MiB
    ages out, about 9,470 candidates are rechecked (the query is skipped only
    while the root generation is unchanged). Measure on d@e before S3 lands:
    seed five roots of 37,900 chunks and recheck 9,470 candidates. Stop S3 if
    a single query takes more than 10 seconds (a third of the deadline; this
    number is invented) or the pass takes longer than the host's GC interval
    allows, which the builder reads from the collector and states.
11. A restore fetches four times as many chunks in sequence. `assemble`
    fetches one chunk at a time (`snapshot.go:1554-1580`); only the existence
    probe is eight wide (`:1515`). At 256 pages that is 37,900 requests
    instead of 9,470. The restore request is one blocking request of ten
    minutes with about one minute of margin (`snapshot_producer.go:100-111`).
    Measure on d@e with a synthetic record of the paper's chunk count, before
    and after. Stop S3 at that size if the restore takes more than 60 seconds
    longer (the margin); a bounded parallel fetch would then be its own slice.
12. The copy digest check (`ErrCopyDigest`, `:212`, checked in
    `streamScratch`) reads in chunk-sized cuts: the digest must not depend on
    the cut. Test both cuts give one digest.
13. Compression is worse at 1 MiB than S1's sample said: the proof compares
    sealed bytes, not raw.
14. A compaction between captures still forces a full upload: unchanged, and
    named so nobody reads it as a regression.
15. Tests and a comment that assert the 1,024 cut change with the code:
    `snapshot_pages_test.go:88-91` (page size 4,096, 1,024 pages, the chunk
    count), `:476-477` (a page range is `ChunkSize` bytes), `:667`
    (`PagesPerChunk == ChunkSize/pageSize`), and the package comment at
    `snapshot.go:24-31`. No other test file names `PagesPerChunk`. The
    old-record test keeps the 1,024 assertions as its fixture.
16. The restore geometry check has a ceiling and no floor (`:1414`): a corrupt
    record can name tiny chunks, bounded by the manifest count. Not a
    regression; not changed here.

Proof: repo tests first (the old-binary restore test, each failure above);
then on d@e two captures and a restore that matches its witness, with the
second capture's uploaded bytes read from its log; then the paper,
read-only: the second capture after the deploy uploads at most what S1
predicted for the chosen size, within a factor of 1.5.

### S4. The host's root limits (red; rollback: revert), only if needed

Built only if the gate in section 3 picks 32 pages, or the paper's store
nears 64 GiB. One mechanism: raise `maxSnapshotRootChunks` and
`maxSnapshotRootBody` (`snapshot_roots.go:72-74`) to cover the store at
the chosen group, up to what the record can carry: the tape record is about
70 bytes a chunk against the 64 MiB payload cap (`computerevent/privacy.go:51`),
so about 958,000 chunks, which is about 120 GiB at 32 pages (S3 failure 5). The
bound today's numbers were written for, 256 GiB, is not reachable at 32 pages.
The host
deploys before any guest that needs it. Its failure list is S3's items 8, 10
and 11 at the new size, benchmarked on d@e first, and the host's per-chunk
file count (303,000 files a root at 32 pages).

If S1 points below 32 pages, stop: the per-capture record no longer belongs
in a tape event, and that is option (b)'s design, not a limit to raise.

## 5. The trigger (S2)

### The rule

One objective, `R`: the replay of the tail after a recovery takes at most
about `R`. A capture is due when

```
tail >= min(R * mu_min, 540000)   AND   the existing floor allows it
```

- `tail` = the head sequence minus the newest snapshot's head sequence.
  Both are already read (`snapshot_producer.go:826-834`).
- `mu_min` = the slowest replay rate ever measured on this computer, in
  events a second. Today `ReplayRate()` is the slowest of the replays in this
  process, and `snapshotDue` keeps the lower of it and 113 (`:406-409`). The
  slice keeps that ratchet and makes it durable, as below. It is never
  raised. The start value is the existing 113 (`snapshotAssumedReplayRate`).
  The existing guard stays: only a replay that applied at least 1,000 events
  counts (`minReplayRateEvents`, `appender.go:466`).
- The ceiling 540,000 events (`snapshotDefaultTailEvents`, three quarters of
  `recoveryplan.MaxRecoveryTailEvents`, `recovery_plan.go:22`) caps the
  threshold. Past the recovery bound a boot refuses to recover, so this is a
  correctness limit. With `mu_min` never above 113 the cap cannot bind while
  `R` is under 80 minutes; it stays so that changing `R` cannot carry the
  threshold through the bound.
- **The floor** is the existing one (`snapshotCaptureFloor`, `:381-388`,
  `snapshotCaptureWallFraction` = 10): no capture starts before ten times the
  last successful capture's duration has passed since the last one began.
  Its inputs are the duration of the last successful capture, kept in
  `snapshot-producer.json`. A failed capture records no duration
  (`:640`, on success only), so the floor keeps using the last successful
  one, and the failure backoff (below) applies on top. With the smaller
  lane's change (`origin/lane/capture-trigger`, an unknown duration does not
  mute) a capture with no recorded duration is due at once, and it measures
  the duration.
- **The bytes part of the floor goes** (sealed bytes spread over 8 GiB a day,
  `snapshotBytesPerDay`). At today's chunks a capture seals about 5 GB, so
  that part alone is 14.6 hours, longer than the 4.9-hour interval `R` = 10
  asks for, and it is the reason the paper's tail is 441,000 events. It is a
  spending rule inside a recovery rule, and spending is owner decision 2. The
  deletion is the smaller change: the function loses one block and the
  constant goes. Bytes stay logged.

`R` is 30 minutes when this slice lands (owner decision 1). It is one
constant in the code with the owner's date beside it.

The rule has no `λ` and no `since` division, so a burst cannot trigger it
unless the tail is really at the threshold.

**The duty bound.** Because the floor is ten times the last successful
capture's duration, start to start, captures take at most about one tenth of
wall time, whatever `mu_min` is, including a corrupt or a legitimately very
low one. There is no continuous capture loop, and a deploy refresh (refused
while a capture is in flight, `planned_restart.go:566-568`) finds a capture
running at most a tenth of the time.

**The honest consequence: when the floor binds, `R` is not met.** The floor
holds back a capture the threshold wants whenever `λ` is high enough that the
threshold is reached before the floor ends (the 32,400-an-hour, `R` = 10
case in section 2). Then the tail at the start is `λ * 10 * d`, the worst tail
a recovery faces is `λ * 11 * d`, and the replay is that over the recovery's
rate: about 33 minutes at 9 events/s, `d` = 26 minutes and 77 events/s. The
status says so: it reports the tail, the threshold, `mu_min`, the estimated
replay of the tail (`tail / mu_min`), whether the rule wants a capture, and,
when the floor holds one, "the objective is not being met" with the time
until the floor ends (the existing `floor_limited` and `floor_why` fields
carry it).

**Where `R` holds.** `R` bounds the tail only if the recovery's own replay is
no slower than the slowest past replay. The first replay after an install is
measured during that recovery (`appender.go:1466-1472`), too late to move
the trigger.

### The persisted `mu_min`

- It is a small file beside the store, with the computer's id. It is loaded
  and checked like the capture record (`snapshot_producer.go:294-302`,
  `:332`, `:476`): a file whose `ComputerID` is another computer's (a fork
  or a restored volume) is discarded, and so is a value that is not finite
  and above zero; the start value 113 is then used.
- It is written by the existing temp-and-rename, with the file and
  directory sync (`saveSnapshotCapture`, `:312-339`): the slice extracts that
  body so both files use it, and adds no second writer. It is written from
  the top of `evaluate`, before the `Ready` return (`:457`), whenever the
  process's `ReplayRate()` is below the stored value. A crash before the
  first evaluation loses only a reading the next boot measures again.
- Every lowering is logged with the applied events, the elapsed time and the
  old and new value, and the status shows `mu_min`. Deleting the file resets
  it. No clamp is added: the floor bounds what a low value can cost.

### What the rule does, stated from the code

- **The replay time a recovery sees** is `R` plus the events appended while
  the capture ran: the new snapshot's head is the store head when the copy
  was taken (`ref.Record.StoreHeadSequence`, `:624`), so the capture leaves a
  tail of `λ * d` behind it, and the worst moment is just before a capture
  completes, at `R * mu_min + λ * d` events. At the paper's 9,400 an hour and
  `d` of 10 minutes that is 1,570 events, about 20 seconds at 77. At the
  busy end (9 events/s) and `d` of 26 minutes it is 14,040 events, about 3
  minutes. After a boot with no matching capture record (the file absent or
  `belongsTo` failing), the tail starts at the snapshot event's sequence, not
  the capture's head (`:476-485`), so it is understated by `λ * d` and the
  worst realized tail is `R + 2 * λ * d / μ`. `R` is chosen with that in
  mind.
- **A low `mu_min`.** One slow replay of 1,000 events lowers `mu_min` for
  good, and the threshold falls with it. The bound, in events and hours apart
  at 9,400 an hour, without the floor:

  | `mu_min` | `R` = 30 | `R` = 10 |
  | --- | --- | --- |
  | 113 (start value) | 203,400 events, 21.6 h | 67,800 events, 7.2 h |
  | 77 (paper's measured) | 138,600 events, 14.7 h | 46,200 events, 4.9 h |
  | 20 (pessimistic) | 36,000 events, 3.8 h | 12,000 events, 1.3 h |
  | 10 (very pessimistic) | 18,000 events, 1.9 h | 6,000 events, 38 min |

  The floor then spaces the starts by ten times `d`: with `d` = 10 minutes,
  100 minutes, so the 1.3-hour and 38-minute rows are held to 100 minutes
  and the capture takes a tenth of wall time. The tail at the start is then
  about 15,700 events at 2.6 events/s, which at 10 events/s replay is 26
  minutes: `R` = 10 is not met in that row, and the status says so. That is
  a residual to watch with the logged `mu_min`, not a loop.
- **Right after boot.** The first evaluation is after `snapshotInitialDelay`
  (3 minutes) and once `Ready` (`:457`). The tail starts at the persisted
  head sequence; `mu_min` is read from its file; the boot's own replay may
  lower it. Nothing resumes: a crash boot is a crash boot.
- **After a failed capture.** The existing backoff holds: 20 minutes, doubling
  to 3 hours, reset by a success (`snapshotBackoffWait`, `:433`; `retryAt`,
  `:531`). The tail grows meanwhile, by about 97,000 events in 3 hours at 9
  events/s, which the code's own comment accepts. The floor uses the last
  successful capture's duration.
- **With no measurement yet.** `mu_min` is 113, and an unknown capture
  duration does not hold a capture.
- **Back to back.** A ticker holds one pending tick (`snapshotTickEvery`,
  `:130`; the loop at `:996-1009`), so the evaluation after a capture longer
  than ten minutes is immediate. The floor, not the tick, spaces successful
  captures.

### What is deleted

- The event ceiling as its own case. It survives only as the cap above.
- The weekly interval and `CHOIR_SNAPSHOT_INTERVAL`. A quiet computer has a
  short tail, and the tape replays it inside `R`; a snapshot there buys
  nothing.
- The 4-minute bound (`snapshotReplayBound`) and the "cost unknown" mute
  (the smaller lane removes the mute first); the bytes part of the floor,
  `snapshotBytesPerDay`.
- The status gains the tail, the threshold, `R`, `mu_min` with where it came
  from (start value or measured), and whether the objective is being met.
  `floor_limited` and `floor_why` stay.

What stays: the floor on duration, the tick, the backoff after a failed
capture, the 90-minute deadline, the free-space refusal, the refusal while
self-development or a capsule is live, and `CHOIR_SNAPSHOT_DISABLE` as the
emergency override.

**The capture cost record.** `snapshot-producer.json` stays, with its duration
(the floor's input), its sealed bytes (logged only now) and its head
sequence (the tail's start at boot, `:489`).

**Why no capture before a planned restart.** The owner's accepted shape had
one; it is not in this design. The restart keeps the data volume, so it does
not need a snapshot, and the request handler writes the marker and answers
in one call, after which the host kills the VM
(`planned_restart.go:834-846`), so no capture could run first without a new
refusal state. The recovery rule alone holds `R`.

### Node B cost a day

At `R` = 30 minutes with today's chunks and the paper's present rate: about
1.6 captures a day at 5 GB sealed, 8 GB (7.6 GiB) a day, under the 8 GiB
budget but not by much; at the busy end the floor caps it near 5.5 captures,
about 27 GB (25 GiB). That already exceeds the 8 GiB a day design budget of
owner decision 2, so the budget is a gate on S3's group size only and S2's
upload is today's upload plus what the lower threshold adds. Today's
upload on the paper is none: its capture is muted. Had the rule run, the
13.4-hour floor would allow about 1.8 captures, 9 GB a day. What S2 adds at
the paper's present rate is therefore close to nothing past that cadence
(8 GB against 9 GB), and at the busy end up to about 18 GB a day more.

Stored bytes do not grow at the upload rate: the host keeps the newest 2
roots plus up to 3 anchors for the recovery window and collects the rest
after the grace period. By the code's own comments that is about 25 GB of
unique chunks for each of the 2 newest and about 4.8 GB for each anchor
(`snapshot_roots.go:41-70`), about 64 GB, plus up to a day in grace. Node B
had 192 GB free on 2026-11-15.

At `R` = 10 minutes after S3: 4.9 to 5.5 captures a day times `U_g`. `U_g` is
S1's output. At 256 pages each capture also adds 2.7 MB to the tape and 2.5
MB of declaration: about 13 to 15 MB of tape a day.

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
- **The objective.** `d` enters the rule through the floor (ten times `d`
  between starts) and adds `λ * d / μ` to the realized tail (section 5), 0.3
  to 3 minutes of replay on the paper. At the paper's present rate a
  10-minute objective is reachable with whole-store witnesses and the floor
  does not bind; at the busy end it does (section 2).
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
   owner's interim figures of 2026-11-15. The realized tail is `R` plus up to
   about 3 minutes (section 5), and the fixed 8 to 25 minutes of install and
   witness are on top.
2. **Daily upload budget to Node B.** Default: 8 GiB a day, the figure
   already in the code (`snapshotBytesPerDay`), used here only as the gate
   in section 3, not as a runtime limit. S2 at `R` = 30 already uploads
   about 8 GB a day at the paper's present rate and about 27 GB at the busy
   end (section 5), so the budget is a gate on S3's group size only. The runtime has no cap; it logs
   bytes uploaded per capture and per day so a spike is seen. If the owner
   wants a runtime cap, it is an owner-set spending cap and a separate
   slice.
3. **Retention on Node B.** Default: unchanged (2 roots for a large store,
   plus the recovery-window chain). At 1 MiB chunks the host's "large store"
   line moves from 8 GiB to 2 GiB (S3, failure 4). After S3 each extra root
   costs only its changed groups, so keeping more becomes cheap; the
   suggested later value is a day of roots, and it is invented.

Numbers in this document that are invented, not measured: the candidate
group sizes {256, 32}; the meter's "every 16th group" sampling; "three
captures" for the gate; the factor 1.5 in S3's proof; the 10-second query
stop in S3 failure 10; the "day of roots" above. The 113 start value, the 77
events/s paper reading and the 540,000-event cap are measured or taken from
the code, as stated where they appear; the 60-second stop in S3 failure 11 is
the restore request's margin in the code.

## 9. Calibration claims to enter before each slice lands

Each is entered in `docs/calibration/claude-ledger.md` with a probability
after reading the scorecard; none is entered by this document.

- **S1.** On the paper, across three captures, the share of 256-page groups
  changed between consecutive captures is under half the share of
  1,024-page groups changed. Test: the meter's log lines. This is the claim
  that clustering exists.
- **S2.** Within 24 hours of the deploy the paper takes a capture with kind
  "replay", and for the following 48 hours its tail never exceeds
  `R * mu_min` plus the events of one capture, or the status says the floor
  holds one, read from the producer's status every hour.
- **S3.** The paper's second capture after the deploy uploads less than half
  the sealed bytes of its last capture before the deploy, and a restore on
  d@e from a 256-page record matches its witness.
- **S4.** If built: a declaration at the new limit registers on d@e inside
  the host's deadline.

## 10. Not verified

- The paper's page churn at any group size. Everything in section 3 waits on
  it.
- Compression below 4 MiB chunks.
- Write-log bytes per event: the measure exists in the static-copy design,
  and no capture has reported it.
- That restore streaming reaches 15 GB/min on Node B; it is the research
  report's figure.
- The host's grace period before an unnamed chunk is collected, and so the
  exact stock on Node B at a given upload rate.
- How a binary older than the page-cut record treats a 256-page record
  (S3, old binaries).
- Whether a persisted `mu_min` is driven low in practice by short replays;
  the logged lowerings answer it.
- Whether witness time scales with store size or with something else; the
  two readings (4 min 12 s at 31.3 GB, 22 min 37 s at 44.1 GB) do not fit a
  line, and the second was on a live file before the static copy.
- The synthetic bench (C97) has not reported.

## Review round 1

Two seats reviewed the first version: Sonnet (the file `claude.out`) and
DeepSeek v4.1 flash (`omp-deepseek-v41-flash.out`), both REPAIR. Each finding
was checked against the code before it was taken. The round repairs by
subtraction; no mechanism was added.

| Seat | Finding | Disposition |
| --- | --- | --- |
| both | S4 (capture before a planned restart) does not work: marker written and VM killed in one call; vacuous gate; delays deploys; stated reason false | Deleted with S4 and owner decision 4. Code confirms: `planned_restart.go:834-846`, `unsafe()` at `:567`. |
| Sonnet B1 | Burst makes `λ` huge; `since` can be zero or negative; no brake between captures; "one after another" case stated as acceptable | Adopted by deleting `λ` and `since` from the rule. The first version also rejected a spacing between successful captures; that was wrong (round 2): the existing floor stays and spaces them. |
| Sonnet B1 | No clamp on a persisted `mu` | Rejected as a clamp; the floor bounds what a low value costs and the file is validated (round 2). |
| Sonnet B1 | The unreachable condition is `2λd/μ >= R`, not `λd/μ >= R` | Correct for the old rule; moot under the new one, where it is `λ * d >= R * mu_min`. |
| Sonnet B2 | S4 contradicts expedite | Deleted with S4. |
| Sonnet B3 / DeepSeek should-fix 1 | The gate is not decidable: `c_g` sampled only at 32, 256, 1,024; interval scaling; fallback contradicts the default | Adopted: candidates {256, 32}; scaling stated (conservative); S3 not built if the gate fails; the default applies only if the sizes are not separated. |
| Sonnet should-fix | `largeSnapshotChunks` means 2 GiB at 1 MiB chunks | Adopted (S3 failure 4). |
| Sonnet should-fix | Sequential restore fetch, 37,900 requests | Adopted with a stop number (S3 failure 11). |
| Sonnet should-fix / DeepSeek should-fix 3 | `LIKE` recheck cost | Adopted into S3's list (failure 10). |
| Sonnet should-fix | Restore geometry check is a ceiling only | Noted, not changed (S3 failure 16). |
| Sonnet should-fix | A failed capture does not record `d` | Moot: `d` is not a trigger input. |
| Sonnet note 2 | The 540,000 cap must apply to the comparison | Adopted (the rule). |
| Sonnet note 4 | A one-off diff on the build VM instead of the sidecar | Rejected: the gate needs the paper's writes over several real intervals; the sidecar is read only by a log. |
| DeepSeek B1 | S4 cannot work | Deleted with S4. |
| DeepSeek B2 | Dropping the `mu` ratchet makes `R` not a bound | Adopted: the minimum ever measured, persisted, start 113. |
| DeepSeek B3 | `g` as a raw count breaks large-page stores | Adopted: `g = min(chosen, ChunkSize/pageSize)`, 64 KiB test. Checked: `restoreOne` `:1414`, `file_cas_http.go:17`. |
| DeepSeek should-fix 2 | `λ` uses the capture's end as its start | Moot: `λ` and `since` are deleted. |
| DeepSeek should-fix 4 | Restore fetch not bounded | Adopted (S3 failure 11). |
| DeepSeek should-fix 5 | `refreshHoldMax` shorter than a capture | Moot with S4. |
| DeepSeek should-fix 6 | Give the stock a number | Adopted (about 64 GB, section 5). |
| DeepSeek should-fix 7 | Partitions share the chunk-count limits | Adopted (section 3, S3 failure 3). |
| DeepSeek should-fix 8 | Tests asserting the 1,024 cut | Adopted (S3 failure 15). |
| DeepSeek should-fix 9 | A `Version` bump is not a fallback | Adopted: no bump; stop and report. |
| DeepSeek should-fix 10 | S1's hash and sidecar are deletable | Considered; S1 kept, one line why (S1). |
| DeepSeek Q2 | One capture per ten-minute tick rules out back-to-back runs | Wrong against the code: the ticker holds one pending tick, so after a capture longer than ten minutes the next evaluation is immediate. Section 5 says so. |
| DeepSeek Q1, Q3, Q5, Q6 | Verified positives (restore reads the cut from the record; deploys cannot be failed by refusals; record size under the cap; retention) | Confirmed or moot; no change. |

Recount after the round: S1 has 9 failures, S3 has 16, S4 (the host's
limits) shares S3's items; the owner decisions are three: the objective `R`,
the upload budget (as the design gate), and retention.

## Review round 2

Two seats reviewed the round 1 repair, both REPAIR. They converged on one
defect that round 1's adjudication caused: with `λ`, `d` and the floor
deleted, nothing spaced two successful captures, so a low `mu_min` (a corrupt
file, or one slow replay made permanent) gave continuous capturing that also
refused every deploy refresh (`planned_restart.go:566-568`). The fix uses what
the code has: the existing floor stays. This is the last repair round.

| Seat | Finding | Disposition |
| --- | --- | --- |
| both | No spacing between successful captures; a low `mu_min` makes a 100% duty loop (DeepSeek B1, Sonnet blocking) | Adopted: the floor (ten times the last successful capture's duration) stays; duty is at most a tenth of wall time (section 5). The round 1 rejection of a spacing is corrected. |
| DeepSeek B1 | Even `mu_min` = 20 at `R` = 10 loops at the longest `d` and busy `λ` | Same fix; the floor binds, `R` is not met, and the status says so. |
| DeepSeek B1 | "A quarter of wall time at 10 events/s" is 4 times wrong | Corrected: at 10 events/s the 6,000-event threshold arrives in 10 minutes; the quarter belongs to 2.6 events/s. Section 5 reworked. |
| DeepSeek B1 | `CHOIR_SNAPSHOT_DISABLE` is not a safe exit; deploy refresh refused during the loop | Moot with the duty bound. |
| both | The `mu_min` file has no validation or `ComputerID` | Adopted: id checked like the capture record, non-finite or non-positive values discarded to 113. |
| Sonnet | Writer placement: lowering must run before the `Ready` return | Adopted: from the top of `evaluate`. |
| Sonnet | Temp-file leak on a crash | Same as the existing writer; not changed. |
| Sonnet | A clamp is wrong for a file-loaded value, defensible for a measurement | The validation covers the file; the floor bounds the measured case. No clamp. |
| DeepSeek should-fix 2 | `R` bounds the tail only if the recovery replay is no slower than the slowest past replay | Stated (section 5). |
| DeepSeek should-fix 3 | After a boot fallback the realized tail is `R + 2 * λ * d / μ` | Adopted (section 5). |
| DeepSeek should-fix 4 | S4's 256 GiB claim is unreachable at 32 pages against the 64 MiB record cap | Adopted: about 958,000 chunks, about 120 GiB at 32 pages (S4). |
| DeepSeek should-fix 5 | The 8 GiB budget is not applied to S2 | Stated: the budget gates S3's group size only; S2 at `R` = 30 is about 8 GB a day at the paper's rate (section 5, decision 2). |
| DeepSeek should-fix 6 | A failed capture records no duration | Stated: the floor uses the last successful one, and the backoff applies. |
| DeepSeek note | `minReplayRateEvents` is `appender.go:466` | Corrected. |
| DeepSeek | Round 1 closures (S4 deleted, gate decidable, group clamp, no `Version` bump, ticker, cap, host limits) | Confirmed; no change. |
| Sonnet | After-boot behavior at the start value 113, and the failed-capture backoff | Confirmed; no change. |
