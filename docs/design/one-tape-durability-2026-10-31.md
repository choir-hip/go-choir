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
