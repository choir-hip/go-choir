# One tape: telemetry, recovery, backup and sync on one mechanism (2026-10-31)

Status 2026-10-31: design, building. Mutation class of this document: green.
The changes it orders are orange to red; each slice names its class.

Owner, 2026-10-31, approving the shape: "one append-only tape that stays the
single source of durability. Telemetry would ride it in compact segments. The
working store would keep only a window of it, which tonight's retirement and
trim already move toward. Sync to the Mac would then be shipping the tape and
snapshots, the same mechanism recovery already uses... hypervisor and sync work
belongs in the same design." And: "actually do build your design... if the
first version is 85% correct thats fine." And on boot: "boot should be light,
as light as possible. then essential services. compaction is its own heavy
operation."

Sources: [the data system frame](choir-data-system-frame-2026-10-30.md),
[object graph redesign §11](object-graph-redesign-2026-10-30.md),
[trace segment log (R7)](trace-segment-log-2026-10-22.md),
[hypervisor seam](../reports/vm-hypervisor-seam-design-2026-10-21.md),
[desktop sync and local VMs](../reports/desktop-sync-and-local-vms-2026-10-14.md),
[hypervisor, sync and storage research](../reports/hypervisor-sync-storage-research-2026-10-28.md).

## 1. The decision

There is one durable record of a computer: its tape (the hash-chained event
log, pinned on the host as content-addressed files and tape segments, with
heads and receipts in PostgreSQL) plus its sealed snapshots (a SQLite file
streamed as content-addressed chunks, with a witness). Everything else is
derived and disposable:

- the guest working store (state.db) is a window over the tape: semantic state
  in full, telemetry for days, the event index for a sequence window;
- a recovery is "newest snapshot, then replay the tail";
- a backup is "ship the snapshot chunks and the tape after it to another
  place";
- a sync, a move to a Mac, or a move to another host is "ship, then serve
  there", fenced so one writer serves at a time.

So recovery, off-host backup and sync are one mechanism, the **computer
bundle**: a snapshot root, its chunks, and the tape from the snapshot's head to
the current head (events, payloads, receipts). Recovery reads a bundle from the
local host; backup writes one to another place; sync reads one on the far side.

## 2. What this settles

| Mechanism | Before | Now |
|---|---|---|
| R7 trace log (`internal/tracelog`, `store.trace_log`) | designed to take trace lines off the tape; library and readers built, no writer | **deleted**. It would give traces a second durability path (its own sealing, backup, restore and replication), and the agent traces are the owner's data, sealed on the tape |
| Telemetry batching (`telemetry.batch`, the telemetry sealer) | built, off everywhere, noted "R7 replaces it" | **on by default**. It is the compact segment: up to 64 telemetry ops sealed into one ordinary tape event, behind the append barrier, so the tape carries telemetry at a fraction of the event and receipt count |
| Host tape segments (`PLATFORM_TAPE_SEGMENTS=write`) | on | unchanged: batches land in append-only segment files |
| Telemetry retirement (tape op) and the event index trim | on (deployed 2026-10-31) | unchanged: they are the working store's window |
| Event table F-c (`store.event_table`) | built, held by the owner pending R7 | the hold's question is answered (R7 is gone). It stays a working-store layout choice (fewer indexes per telemetry row), not a durability one; it may be turned on by ring later, and it is not needed for this design |
| Compaction | ran at Open | **never at boot**. It is its own scheduled heavy operation (slice S5) |

## 3. The computer bundle

A bundle is a directory (or an object-store prefix) with:

- `manifest.json`: computer id, owner id, the snapshot root record (as in
  `computer_snapshot_roots`), the tape range `(snapshot_head, head]` with the
  head digest, counts and byte totals, the producing host and time, and the
  bundle format version;
- `chunks/<digest>`: the snapshot's sealed chunks (content-addressed, so a
  later bundle to the same place copies only new chunks);
- `events/<digest>` and `payloads/<digest>`: the tape events in the range and
  the payloads they commit to (or the tape segment files that hold them);
- `receipts.jsonl`: the append receipts of the range, in sequence order, so
  the far side can check the chain without trusting the bundle's producer.

Export reads only sealed, immutable artifacts and PostgreSQL rows: it runs on
the host with the computer up or down, and never touches the guest. Verify
re-derives the chain (each event's digest and parent), checks every chunk and
payload digest, and checks the manifest's head. Import writes the artifacts
into a host's file store and the rows into its PostgreSQL, idempotently, and
refuses a range whose first event does not chain to the host's existing head
for that computer (a fork is a new computer, never a merge).

A restore drill is: export from Node B, import on a scratch host directory
and database, and run the recovery path there against the imported state.
The same bundle, written to Node A, is the off-host backup (L5) that replaces
the restic image backup: no guest images, only sealed artifacts.

## 4. Fencing (one writer per epoch)

Serving a computer from a bundle on a second machine while the first still
serves it would fork the tape. Until the writer epoch lands (object graph
redesign §5: the realization epoch in a lease row, carried in the append
capability, refused when stale), a move is **cold**: stop on A, export the
final range, import on B, start on B, and A refuses to start that computer
again (its ownership row is handed off). The warm handoff waits for the epoch.

## 4a. The Mac is the first off-host place

Owner, 2026-10-31: not as an ad hoc pull. The Mac copy comes after the Mac
app runs user computers under virtualization and keeps them in sync with
Node B; the scripted pull built on 2026-10-31 was removed. And the point is
syncing user computers (the owner's own personal computer first), not
replicating the paper on the Mac. Until then the off-host copy is the
nightly restic backup.

Owner, 2026-10-31: "getting vms running on my mac is a great way to get a
form of off-host backup and recovery 'for free'. obv we need that for real as
server infra, but i dont want to pay for that until the whole system is
functional and stable. and some of the key functionality is the workstation
mode."

So the first consumer of the bundle is the owner's Mac, not a paid backup
service: the Mac pulls bundles from Node B (incremental by chunk, event and
payload digest) into its own tape store, which is already an off-host copy,
and the same bundles let the Mac serve the computer in workstation mode. Node
A stays the second copy for the server fleet (S6) because it is already paid
for. Paid off-host infrastructure waits until the system is functional and
stable.

## 5. The Mac and the hypervisor

The bundle is hypervisor-neutral (a SQLite file and sealed artifacts, never a
disk image), so the hypervisor work is independent of it:

- the vmmanager seam (`Backend` + `Cage`, the 10-21 design) lets a second VMM
  launch; Firecracker stays the server default and Cloud Hypervisor joins on
  the test ring;
- on a Mac, a vfkit backend (Virtualization.framework) runs an aarch64 guest;
  the Mac host keeps the computer's key in the Keychain, serves its own small
  tape store, and imports a bundle to start;
- sync is then "export the new range on the serving side, import on the
  other": the same code as backup, run in both directions under the fence.
- the guest image itself becomes two read-only erofs layers: a slow base
  reused across deploys and a ~95 MB app layer per deploy, joined by an
  overlay in the guest. Both build per arch, so an aarch64 vfkit guest
  attaches the same pair
  ([layered guest image](layered-guest-image-2026-10-31.md), measured
  2026-10-31).

## 6. Slices, in build order

| Slice | What | Class | Proof |
|---|---|---|---|
| S0 | Boot is light: no compaction at Open, the event index catch-up two minutes after Open (53be139fe) | red | the paper boots and serves the deploy |
| S1 | Telemetry batching on d@e (R0) by stored setting, the default after d@e shows it; and a setting never stored on a computer reads as its declared default (it read as off, so `store.telemetry_retire`'s default-on of fe70432d2 never reached the paper) | orange | d@e runs a turn; batch events on the tape; a crash drill loses at most the open window; the paper's retirement job logs removals |
| S2 | Delete R7: `internal/tracelog`, `store/trace_log.go`, the merge halves in the event readers, the observability stats; `store.trace_log` stays a known retired key | red (event read path) | build, store and agentcore tests; deployed reads of run events |
| S5 | (moved up, review 2026-10-31) Off-boot compaction: see below. Until it runs, retirement and the trim only grow SQLite's freelist; the file, the snapshot and any bundle stay 32 GB | red (VM lifecycle) | the paper's state.db measured before and after |
| S3 | Bundle export and verify on the host (`cmd/choir-bundle export|verify`) | orange (read-only host tool) | export d@e and the paper; verify passes; byte totals reported |
| S4 | Bundle import, and the restore drill on a scratch directory and database | red (host store writes) | the imported state recovers through the guest's recovery path |
| S5 detail (superseded by 6b: compact snapshots, then a planned rebase) | Off-boot compaction: a planned maintenance op that compacts a stopped computer's state.db, inside the boot envelope budget or with a fresh envelope | red (VM lifecycle) | the paper's state.db measured before and after |
| S5a | Mac pull: the Mac fetches bundles from Node B over the authenticated host API into a local tape store (incremental), and verifies them. The first off-host copy, at no cost | orange | a verified bundle of the paper on the owner's Mac |
| S6 | L5 backup: nightly bundle to Node A (incremental by chunk and segment), retention, a restore drill; then delete the old restic repository | red (backup) | a verified bundle on Node A; a drill restores from it |
| S7 | Writer epoch (redesign §5) and the handoff | red | stale-epoch append refused on staging |
| S1b | Sealed batches compressed (zstd), with the format version on the tape | red (tape format) | bytes per telemetry line measured before and after on d@e |
| S9 | Tape retention: what of the tape, receipts and payloads may be dropped or cold-stored after a snapshot covers it, and on the Mac | red (tape, evidence) | design first; owner decides what is kept forever |
| S10 | Recovery inside its authority and time: replay takes a renewing capability source and renewal starts at the exchange; the bound restated in ops/bytes; the snapshot cadence stated against it | red (boot path, credentials) | a d@e recovery whose replay outlives one capability lifetime succeeds |
| S8 | Hypervisor seam, then vfkit on a Mac with a bundle import: workstation mode | red | a computer moved Node B to a Mac and back |
| S11 | Telemetry in per-day partition files, tool.result first (6c) | red (store schema, epoch 5) | d@e: partition writing; two snapshots hold a closed day's chunks; a whole-day retire deletes a file |

## 6a. Review of 2026-10-31 (Sonnet and Devin seats; DeepSeek pending)

Adjudicated:

- Accepted, blocking for the design's goal: nothing shrinks the paper's file
  until S5. `reclaimFreePages` works only on a file whose auto_vacuum is
  INCREMENTAL, which only a compacted copy is, and the snapshot streams every
  page. S5 moves ahead of S3/S4, and its live-size cap (4 GB, a boot bound)
  is lifted for the off-boot run; the disk needs 1.2x live free for the copy.
- Accepted: batching amortizes the per-event overhead (about 5.7 KB) over up
  to 64 lines but does not shrink the payload (about 4.7 KB): about 2.2x
  fewer bytes per line, 64x fewer receipts and index rows. Compressing a
  sealed batch (zstd; JSON telemetry compresses 5-10x) is the next factor.
  New slice S1b.
- Accepted: settings defaults now apply to never-stored settings fleet-wide
  at once. Retirement was meant fleet-wide (d@e ran it first). Batching has
  run nowhere, so it rings from d@e.
- Accepted, missing slices: tape retention and GC (the tape is the one
  durable record and only grows: S9), an explicit snapshot cadence against
  the 720,000-event bound and the 15-minute envelope (S10), and a bundle size
  gate (a drill that fails above N GB a day, with S3).
- Noted, not gating: while the event index catch-up runs, a replay-to-live
  comparison differs (the live index is between trim states); the snapshot's
  own witness is computed in one read transaction and stays self-consistent.
- Accepted, from Devin (verified in code): tail replay runs after the
  credential exchange (internal/autoputer/run.go:365 after :294), so the
  boot envelope is safe, but replay is handed the capability as a fixed
  string, and the capability expires with the envelope
  (internal/platform/credential_envelope.go:218); background renewal starts
  only after replay (run.go:491). A replay longer than the capability's
  lifetime (the paper's 720,000-event bound is about 40 minutes at 298
  events a second) would lose its read authority mid-way. Fix: replay takes
  a renewing token source, and renewal starts at the exchange (S10). Also:
  after batching the bound must be stated in ops or bytes, not events (one
  batch carries up to 64 ops).
- Accepted, minor, fixed: the trim floor was read at every trim point inside
  the finalize transaction; it is now written once per workspace.
- Noted: retired run memory has no read-through to the tape
  (`get_run_memory_entry` answers not found for a retired run).

Rough bounds from the review (assumptions in the review): about 1 M events a
day on the paper (12/s average, 298/s peak); steady-state working store 8-10
GB once compacted; a Mac pull of 3-10 GB a day before compression, about 1-5
GB a day of growth on the Mac without GC.

## 6a2. Review of the built bundle (2026-10-31, Sonnet seat; Devin and DeepSeek pending)

The bundle as built (internal/bundle, cmd/choir-bundle) is host-side
durability: a verified, importable copy of the tape and the snapshot chunks.
It is not yet recovery on another machine. Accepted, as slices:

- B1 keys: the snapshot manifest is sealed in the guest's snapshot_recorded
  event; a bundle carries no key material, so an imported copy is ciphertext
  until the computer's keys travel with it (redesign §11: Keychain on a Mac).
- B2 authenticity: verify checks chunk digests and the chain, not that the
  snapshot root derives from its chunks, nor the receipts' signatures; an
  imported root with a high head_sequence would be preferred by
  RecoveryBase. Verify must check signatures against a known host key set,
  and import must refuse a root the tape does not name.
- B3 host state: ownership, route, file-CAS roots and tombstones are not in
  the bundle.
- B4 import transaction: the whole range in one serializable transaction
  (about 720,000 receipts on the paper) holds the head row lock throughout;
  commit in chunks, advancing the head in the last.
- B5 physics: the paper's roots carry about 25 GB of unique chunks each (most
  page ranges change between snapshots), so a nightly pull is not an
  increment until compact snapshots (6b) land; and one file per event and pin
  is millions of small files a day: pack them into segments in the bundle
  format, and record the last pulled sequence for incremental pulls.
  Packing landed 2026-10-31 (bundle format 2, internal/bundle/objects.go):
  events, payloads and pin receipts in one append-only objects.pack with a
  line-per-object objects.idx; an interrupted export keeps the indexed part
  and truncates the rest. d@e: 33,641 files to 19, 258 MB to 172 MB on disk,
  export 78 s to 11 s; format 1 still verifies. Incremental pulls landed the
  same day: choir-bundle export --since <previous bundle> writes a delta (the
  events after that bundle's head, no snapshot), refused when the source
  does not hold the previous head, and imported only onto a target at it.
  The 25 GB root remains (compact snapshots, 6b).

Fixed at once: the vmctl remove_stale_route override took any route error
and any non-running state; it now needs the definite does-not-join refusal,
an idle ownership, no protected or always-on tier, and 30 days idle.

## 6b. What compaction is (owner question, 2026-10-31)

SQLite never returns deleted rows' pages to the disk; they go to the file's
freelist. Retirement and the trim make the working store emptier, not
smaller, and a snapshot streams every page. The design space, cheapest
physics first:

1. Make no garbage: telemetry in time-partitioned files (a day or a week
   each), so retiring a window is deleting a file. O(1), no rewrite, no
   fragmentation. The long-term answer for anything with a time window.
2. auto_vacuum=INCREMENTAL: cheap ongoing reclaim, but it needs one full
   rewrite to turn on and it fragments.
3. Compaction as a side effect: the working store is derived, so the
   snapshot producer writes only live pages (a compact snapshot), and a
   planned restart rebases onto the newest snapshot through the existing
   stage-and-swap recovery path. Compaction is "restore from your own compact
   snapshot", at the cost of one snapshot write that is paid anyway. Snapshots
   and bundles shrink to live size at once, before any rebase.
4. Rebuild from the tape: always correct, cost grows with history.

The tape has its own compaction (retention, compressed sealed batches,
segment repack: S9, S1b), which is where the information-theoretic savings
are.

Decision: S5 is option 3 (compact snapshots, then compaction is a planned
rebase onto them), and option 1 follows for telemetry. An in-place VACUUM of
a live computer is the old frame. Check first: the snapshot's content witness
is computed over a read transaction of the live file, so a compact snapshot
must give the same witness (it covers rows, not page layout).

## 6c. Telemetry in time-partitioned files (option 1, 2026-10-31)

Owner, 2026-10-31: "We need to reconsider how we do compaction if our current
plan has us rewriting gbs every day." Retirement by row delete (tool results
after 7 days, the trace kinds after 14) frees pages all over the 20 GB file,
so every snapshot's 4 MiB page ranges differ and nothing dedups. Option 1:
telemetry lives in small per-day files, and retiring a day is deleting one.
Built in internal/store/telemetry_partition.go; mutation class red (store
schema, projection, snapshot).

**Failure modes first** (each has a test; T in the store's
telemetry_partition_test.go, S in snapshot_partitions_test.go):

1. Replay puts a row in another file than the live apply did (the partition
   read from the wall clock or the binary's class table) (T1).
2. A reader answers differently over partitions (a missed file, a cut
   before the merge, a cursor that skips one); a seed misses the partitions
   and hands out a sequence or stream number twice (T2, T11).
3. Retirement keeps or drops other rows than the row delete did, the
   keep_cited rows in particular (T3).
4. A crash between the partition commit and the state commit leaves a
   phantom row, or loses a row (T4).
5. A crash mid-retire: a retired file is read again; a cut-short deletion
   leaves a WAL; a deletion runs while a compact copy whose head is below
   the retirement is staged, so the rebase replays a retire op whose file
   is gone and loses the cited rows (T5).
6. A late row of a retired day is invisible (T6). The dry run writes (T7).
7. The job retires before the window or never, or appends on an idle run
   (T8).
8. A snapshot misses a partition, a closed partition does not dedup, a
   restore keeps a stale partition of another lineage or loses the
   partitions silently, a partition path escapes its directory, the host
   sweeps partition chunks (S1-S6); a file is deleted under a capture (T9).
9. An older binary opens a store with partitions it cannot read or retire
   (T10).

**Decisions.**

- *Granularity and naming:* one file per class per UTC day,
  `<workspace>/telemetry/<class>-YYYYMMDD.db`, in the store's sidecar
  workspace so every path that moves the workspace (rematerialize's flip,
  quarantine) moves the files. A day, not a week: the retire op's longest
  slice is 24 hours, so one op retires one file; a week of tool results is
  eight files. First class: `tool_result` (tool.result), the largest
  retired kind and the only one with keep_cited. The trace kinds and
  loop.progress are the next classes (below).
- *The tape decides:* the writer stamps the op
  (`ProjectionOp.Table = "telemetry_partition"`, next to F-c's
  `choir_events`), and the projector routes on the stamp; the file is the
  class of the event kind and the UTC day of the object's own updated_at in
  the op body. A kind this binary does not partition, or a row whose day is
  already retired, lands in choir_events. Unstamped history replays as it
  did (og_objects).
- *Commit order, not attached databases:* SQLite commits attached WAL
  databases one file at a time, so a crash between them is possible either
  way. The partitions commit first (synchronous=FULL), then the state file,
  so a head implies its rows. Open deletes rows above the state file's head
  in every partition (one indexed delete per file); a restore, a compaction
  rebase and a crash all reduce to this, and the replay writes the rows
  again (an upsert on the tape position). This is not a resumer: it removes
  effects the state file never committed, and no work is reopened.
- *Readers fan out per query:* each choir_events half of a union reader,
  and each seed, runs its own SQL on choir_events and on every live
  partition, and the reader merges as it already merges the two tables. No
  ATTACH (per-connection state across a read pool, and a 10-database cap).
- *Retirement stays one tape op with the old semantics.* A retire op that
  covers a partition's whole day (every phase, all its kinds) retires it: in
  the state transaction, the rows a stored run acceptance cites are copied
  into choir_events and the partition is recorded in
  `telemetry_partitions_retired` (name, class, day, tape_seq). An op that
  covers part of a day deletes the matching uncited rows inside the
  partition, which is exactly the old row delete, in a file that is going
  away. The job walks the hours of og_objects and choir_events as before
  (the rows that predate partitions), then appends one whole-day op per
  partition whose day ended before the cutoff: a tool result stays at least
  7 days and less than 8.
- *The file deletion is not state.* `GCTelemetryPartitions` runs after the
  daily retirement and deletes the files the state file calls retired. It
  waits for a snapshot capture (the capture holds it from before its read
  transaction) and does nothing while a compact copy is staged.
- *Snapshot:* the record carries `partitions` (path, geometry, chunks),
  outside the manifest so an older reader installs the state file alone.
  Each partition streams from its own read transaction, begun after the
  state file's; rows above the head are dropped at the restore's Open. A
  closed partition's pages never change, so its chunks are the same digests
  in every snapshot and are never uploaded twice: that is the point. The
  root registration declares the partition chunks. Restore empties the
  partition directory, installs the snapshot's partitions, then the state
  file; a snapshot with partitions and no directory named is refused.
- *Content witness:* witness v3 covers the state file, including the
  retired set and the cited rows moved into choir_events; the partition
  rows are covered by their sealed, digest-addressed chunks, and by the
  replay-completeness fingerprints, not by the witness.
- *Floor:* store epoch 5. The writer writes floor 5 before it stamps, and
  the first partition file of a fresh workspace (a replay, a restore)
  writes it too.
- *Switch:* `store.telemetry_partitions`, a stored owner setting, default
  off for the ring walk (R0 d@e, then the paper, then the default), stamping
  only once the stream_seq lease is ready, like F-c.

**Migration.** None by copy. Rows already in og_objects stay there and age
out through the hourly row delete within 7 days of the switch (the paper's
retirement is already running); the cited ones stay in og_objects. Their
freed pages go with the compaction rebase (6b). From the switch on, every
new tool result lands in a partition.

**Rollback.** The setting off is the live rollback: new tool results go to
og_objects; the partitions stay readable and are retired by day as before.
A binary below epoch 5 refuses a floored store; going below it needs every
partition retired and deleted, then the floor lowered by hand. A git revert
of the build without the floor would leave tool results of the last week
unread by the older binary (telemetry only, never semantic state).

**What landed, what remains.** Landed: the tool_result class end to end
(writer stamp on both write paths, projector, dry run, readers and seeds,
fingerprints, retirement and keep_cited, the job's day ops, the file
deletion, Open's cleanup, the floor, snapshot capture and restore on both
install paths, observability). Remains, named:

- the next classes: desk.cell and desk.egress (14 days), phased and
  phaseless loop.progress (a phase filter splits a class: the class must be
  the retire group, kind and phase), tool.invoked and Texture progress
  (retired at once: they could simply not be written), run.delta and the
  other batched kinds the policy keeps (a forever class needs no file per
  day);
- the vocabulary fences do not read partitions (only epoch-5 V2 binaries
  stamp them; noted at the fence);
- a restore to a point below a later partial retire keeps that retire's
  effect inside the partition (the replay to the head converges; telemetry
  only);
- a partition witness (a digest of the rows at or below the head), if a
  replay comparison ever needs it.

**Rollout.** d@e: deploy, set `store.telemetry_partitions` on through
/api/computer-settings, run a turn, and check `telemetry_partitions` in
/api/runtime/observability (writing, today's file live), a snapshot whose
record lists the partition, and the next day's job appending one whole-day
op per closed file once the window passes. The paper: the same setting
after d@e shows it, then watch two consecutive snapshots: the closed days'
chunks must be held (not uploaded); the state file's churn falls as the
legacy tool results age out and the compaction rebase reclaims them.

## 7. What can go wrong

- Batching loses the open window (at most about a second of telemetry) on a
  crash; semantic events are never lost (the barrier seals first). Accepted by
  the 10-20 design.
- A bundle that omits a payload a replay needs fails at recovery, not at
  export. Verify checks every payload commitment in the range.
- An import onto a host that already serves the computer forks it. Import
  refuses unless the computer's ownership on that host is absent or handed
  off.
- Deleting R7's readers changes nothing observable today (no writer exists,
  the merged halves are empty), but a test or tool that expects the trace log
  directory will fail; those are deleted with it.
