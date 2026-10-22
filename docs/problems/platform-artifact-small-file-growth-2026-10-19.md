# The platform artifact store keeps one file per tape object; metadata pressure is structural (2026-10-19)

Date: 2026-10-19, measured 20:56 to 21:05 UTC on Node B. Docs only: no code
changed. Follows
[`node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16`](node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md)
(the incident, mitigated by a balance) and the daily
`go-choir-btrfs-headroom` timer (a guard, not a cure). Owner request: the
cause and a permanent solution.

Classification: **substrate** (data persistence: the artifact store's
object layout). Mutation class of this doc: green. Mutation class of every
fix below: **red** (the event tape write path and the evidence store;
protected surfaces: computer events, pin receipts, checkpoint and replay).

Related records: the tape rate is also a guest cost
([`guest-dolt-storage-growth-2026-10-15`](guest-dolt-storage-growth-2026-10-15.md):
about 240 KB of guest Dolt journal per tape event) and the tape was left
open in
[`artifact-store-grows-12gb-a-day-2026-10-14`](artifact-store-grows-12gb-a-day-2026-10-14.md)
("Still open: the event tape"). This is the third record that names the same
rate. Root-cause clustering applies, and the lever it points at is events per
write, not tuning.

## Summary

- **Cause.** Every tape event costs four files: an event file, a payload
  file and two signed pin receipts (one per pinned artifact). Each file is
  under 2 KiB for 87% of tape files, so btrfs stores it inline in metadata.
  Nothing ever deletes a tape file. Metadata is a one-way ratchet.
- **Multiplier.** The tape carries **all** durable guest state. A
  `projection_batch_recorded` event is not a cursor. Its payload is the
  object-graph write itself. The paper computer appends one event per
  runtime event and one per run-memory entry, each with one op.
- **Fix, in order.** (1) Group commit: put many ops into one
  `projection_batch_recorded` event (the format already allows it). (2)
  Pack tape objects into per-computer append-only segments with a sealed
  index, behind a dual-read migration. (3) Then decide retention.
  Retention alone removes at most 14% of today's tape.
- **Not recommended as the fix:** a database for the bytes, `max_inline=0`
  (a break-glass lever only), or deleting tape files.

## Evidence (read-only, Node B, 2026-10-19 20:56 to 21:05 UTC)

Method: one `find -printf '%s %T'` pass per namespace with awk totals (counts
and sizes only, no file content). `SELECT` counts on the platform Dolt
`computer_event_heads` table. `jq` counts on vmctl `ownerships.json`. A
250-file sample of event headers for `computer_id` prefix and `event_kind`
tallies. Nothing was modified.

### Filesystem

| | |
|---|---|
| Mount | `btrfs rw,noatime,compress=zstd:3,ssd,discard=async,space_cache=v2`, subvol `/@` |
| `max_inline` | not set (btrfs default 2048 bytes) |
| Sector / node size | 4096 / 16384 |
| Metadata (DUP) | 18.64 GiB allocated, **15.07 GiB used** (37.29 GiB on disk) |
| Data | 408 GiB allocated, 333.55 GiB used |
| Unallocated | 30.00 GiB (the guard's threshold) |

### The tape namespaces (`platform-artifacts/sha256/`)

| Namespace | Files | Bytes | Avg bytes | Files at most 2048 B |
|---|---:|---:|---:|---:|
| `computer-event` | 2,259,058 | 3.41 GB | 1,511 | 100% |
| `computer-event-payload` | 2,276,449 | 12.42 GB | 5,457 | 48% (78% at most 4096) |
| `pin-receipts` | 4,538,908 | 4.06 GB | 895 | 100% |
| **tape total** | **9,074,415** | **19.90 GB** | | **87% (7,882,662)** |
| `og` (not tape) | 577,600 | 1.26 GB | 2,179 | 76% |

The platform Dolt `computer_event_heads` sums to 2,258,489 events across 228
computers. That is the event-file count within 569 files. The tape is
complete and nothing was ever deleted.

### Measured cost per event

Ratios from the table: 1.008 payload files per event, 2.009 receipts per
event.

| Per event | Count | Bytes |
|---|---:|---:|
| event file | 1 | 1.51 KB |
| payload file | 1.008 | 5.50 KB |
| pin receipts | 2.009 | 1.80 KB |
| **files / bytes** | **4.02 files** | **8.81 KB** |
| platform Dolt row (`computer_event_append_receipts`, incl. the signed head receipt) | 1 | not measured |

Each file is also written through a temporary name and renamed
(`internal/platform/service.go:882-898`), so the filesystem sees about
twice the directory churn. About 0.8% of the pins are orphans: 17,391
payload files and 20,792 receipts exceed what 2,259,058 events need. Pins
land before the CAS (`appender.go:657` then `:682`), so a refused append
leaves them behind, and no sweep collects them.

### Metadata cost

- Per-file items (inode, name ref, two dir entries, extent item) are about
  0.6 KB, plus the inline data. **Estimate**, from btrfs item sizes, not
  measured.
- Cross-check from growth: from 10-16 23:00 to now the four namespaces grew
  from 8,340,518 to 9,652,015 files (+1,311,497 in 2.92 days). Metadata used
  grew from 13.61 to 15.07 GiB (+1.46 GiB). That is **at most 1.17 KB per
  new file** (an upper bound: other writers share the metadata).
- So the 9.65M files are most of the 15 GiB: **about 8 to 11 GiB logical,
  16 to 22 GiB on disk** (DUP), for 21 GB of data. That is an estimate.
- Metadata chunks come only from unallocated space, two physical GiB per
  logical GiB. The daily balance returns sparse data chunks to unallocated;
  it cannot shrink metadata. At the 10-16 rate (1.5M files a day) metadata
  grows 1.7 GiB a day (3.4 GiB on disk).

### Rate

Events per day (tape file mtimes, `computer-event`):

| Day (UTC) | Events |
|---|---:|
| 10-14 | 337,668 |
| 10-15 | 364,940 |
| 10-16 | 351,656 |
| 10-17 | 101,925 |
| 10-18 | 89,859 |
| 10-19 (to 21:00) | 126,449 |

The rate fell 3.4x after 10-16. Files follow at 4.02 per event, about
0.45M a day now against 1.5M. **Hypothesis, not traced:** the 10-16 to
10-17 fixes changed producer churn (run-end retry `c8e1dc259`, Hacker News
live counters dropped from feed bodies `67201a94c`). The run ledgers were
not read. Treat the lower rate as unproven and as reversible with product
behavior.

Burstiness, last 12 h: 95,747 events in 14,411 active seconds (max 27 in one
second). Events by arrival window, as served today:

| Window | Batches | Events per batch |
|---|---:|---:|
| 0.25 s | 32,479 | 2.95 |
| 1 s | 14,411 | 6.64 |
| 2 s | 9,038 | 10.6 |
| 5 s | 4,322 | 22.2 |

Of the 250-file sample, 248 were `projection_batch_recorded` (99.2%; the
10-15 count on the paper was 99.6%) and all came from one computer.

### Who owns the tape

Join of `computer_event_heads` with vmctl's ownership registry (151
ownerships):

| Group | Computers | Events | Share |
|---|---:|---:|---:|
| `computer-1355b83b` (registry: stopped at 20:30; the paper, per the 10-15 storage record) | 1 | 1,356,556 | 60.1% |
| `computer-03335285` (registry: active; created 08-13) | 1 | 593,049 | 26.3% |
| two other registry computers | 2 | 663 | 0.03% |
| **no ownership record today** | **224** | **308,221** | **13.6%** |

The 224 computers with no ownership record are destroyed, pruned or never
registered (test, tracer and candidate computers). Five of them hold at least
10,000 events (89,711 in all). Their 308,221 events are about 1.24M files and
2.7 GB. The 146 hibernated registry computers have no tape.

The paper (`computer-1355b83b`) is `stopped` in the registry as of 20:57.
The 16:30 plan to compact its history
(`paper-computer-disk-full-again-history-growth-2026-10-19.md`) would
explain that. Not checked.

## Mechanism: the write path

One tape event, guest to host. Guest code in `internal/computerevent` and
`internal/store`; host code in `internal/platform`.

| Step | Code | Writes on Node B |
|---|---|---|
| A store write (an object, an edge, a run-memory entry, a desktop state) becomes a `ProjectionOp` | `internal/store/projection_tape.go:56-84` (`interceptOG`), `run_memory.go:52-88`, `desktop_live.go:73` | none |
| The op list becomes one `ProjectionBatch` payload and one `projection_batch_recorded` event | `projection_tape.go:97-135` (`appendOps`) | none |
| Dry-run the batch, pin the payload (`PinNonPrivatePayload`) | `appender.go:338-397` (`AppendNewPayload`), `http_client.go` `pinPayload` to `event_handlers.go:199` `HandleComputerEventPin` | payload file + receipt file |
| Pin the event | `appender.go:657`, `event_artifacts.go:31-44` | event file + receipt file |
| Head CAS | `appender.go:682`, `computer_events.go:275` | one SQL row in platform Dolt |

`EventArtifactService.pin` (`event_artifacts.go:62-90`) writes the artifact
with `writeImmutable` (`:223-239`: read to probe, then temp file and rename),
signs a `PinReceipt` (`NewSignedReceipt`, `receipt.go:53`), and writes the
receipt the same way. Receipts carry a random UUIDv7 id and an issue time,
so they cannot be re-derived; they are real, signed evidence.

`og` is a separate writer: `objectgraph_store.go:48-80` externalizes
object bodies of at least 1 KiB. It is now nearly idle (about 600 to 1,700
files a day). 570,000 of its files are the pre-10-09 corpus bodies,
unreferenced since the teardown.

## What `projection_batch_recorded` is

It is the **sole durable write path** for guest state when the tape is bound
(`docs/learn/01-ledger-and-store.md` §2.5; `BindProjectionTape`,
`projection_tape.go:23`). `DoltStore.PutObject`, `PutBatch` and `PutEdge` do
not write SQL; the interceptor wraps the ops in a batch, appends it, and
finalization applies it to `og_objects` (`internal/store/computer_events.go:153-258`, the
guest store). The payload **is** the write. It is not a projection cursor
and it cannot become a local cursor: restore and rebuild are replay of the
payloads (`appender.go:1008-1056` `resolveProjectionBatch`).

Who emits it: every `Store.AppendEvent` (`store.go:1929` to
`AppendEventOG`, `graph_store.go:1168`, one `choir.event` object), every
`AppendRunMemoryEntry` (`run_memory.go:20`, one op), every desktop state
write, and the commitment, wake and source-ref writes. From the 10-15 paper
counts: `choir.event` objects 207,658 and run-memory entries 111,636 are
89% of the 358,503 batch events. Almost every batch carries **one op**,
because each caller writes alone. `ProjectionBatch.Ops` is a list with no
cap (`projection_batch.go:159-215`), and the replay path
(`finalizeBatch`, `projectBatch`) applies a list in order. A multi-op batch
needs **no format change and no replay change**.

Readers of the batch: the guest finalizer, boot replay, the offline
rebuilder (`internal/projectionbase/source.go`) and the ledger witness
(`internal/ledgerwitness/walk.go:93`).

### Can it be cut?

| Lever | Effect | Verdict |
|---|---|---|
| Make it a local cursor | n/a | No. The event carries the data. |
| **Group commit**: coalesce concurrent and near-in-time ops into one batch event | 3x at 250 ms, 6.6x at 1 s, 10x at 2 s (table above). Same factor on files, receipts, platform Dolt rows and guest journal. | **Yes. Do first.** |
| Demote pure telemetry (`choir.event`, 58% of events) to a non-replayed local store | removes about 58% of events | Owner decision. It breaks "all durable state is on the tape" and the state-completeness probe. Only if group commit is not enough. |
| Fewer producers (wakes, retries, churn) | the 3.4x drop on 10-17 shows the size of this lever | Product work; keep doing it. Not a storage fix. |

Group commit keeps the per-event costs fixed and cuts the number of events.
The fixed part is large: event file plus two receipts is 3.3 KB of 8.8 KB
and three of four files. Payload bytes do not shrink.

## Readers (access pattern)

| Reader | Code | Pattern |
|---|---|---|
| Event CAS validation | `event_artifacts.go:92-177` `ValidateEventPins`: reads 1 + n receipts and their artifacts | by digest, once, within seconds of the write |
| Replay page to the guest | `event_replay.go:36-125` (SQL index by sequence, event file by digest) | by digest, in sequence order |
| Payload fetch | `event_artifacts.go:194-221` `FetchPayload` | by digest, random |
| Checkpoint authority | `checkpoints.go:222`, `:247`, `:266` | by digest, rare |
| Offline rebuilder (checkpointd, `choir-rebuild-base`) | `projectionbase/source.go:51-79` walks the chain from a head back to **genesis**; without a head it `ReadDir`s the whole flat directory (`:80-107`) and reads every event of every computer | sequential per computer today, but via 1.36M random file opens |
| Ledger witness | `ledgerwitness/walk.go:71-93` (chain walk, then payloads) | by digest along the chain |
| Pin receipts | `event_artifacts.go:116` only | **read once, at append**; never again. Their digests live on in the signed head receipt (`computer_events.go:346`). |

Every reader already knows the computer id. Reads are digest-verified
everywhere. The tape is never scanned by time or by content. These fit a
per-computer, append-only layout well.

## Retention

- **No code deletes a tape file.** `artifact_gc.go:97-112` excludes
  `computer-event`, `computer-event-payload` and `pin-receipts` ("a
  chain-retention policy, not a refcount sweep"), and the only
  `os.Remove` in the platform GC is `artifact_gc.go:777`. The 2026-10-14
  fix closed the other namespaces (`projection-base`, `file-cas-*`,
  `platform-update`); `og` is held back.
- **Destroying a computer leaves its tape.** vmctl's retention prune destroys
  only the VM directory (`retention_prune.go:516`, `vmmanager/manager.go:1367`
  `DestroyVMState`). Nothing in the platform deletes artifacts, head rows or
  append receipts by computer.
- **Measured:** 13.6% of tape events (308,221; about 1.24M files, 2.7 GB)
  belong to computers with no ownership record. The two live computers hold
  86.4%, and the paper alone holds 60.1%. Retention of dead computers is
  worth doing, but it cannot be the fix.
- **Other retention gaps:** about 38,000 orphan pins (above); the 577,600
  `og` files (5.8% of all files, 1.26 GB; its bodies may still be named by the
  corpus dump at `corpus-dolt/rebuild-20261009`, which is still on disk).
- **Truncating a live computer's old tape is not safe today.** The chain
  links back to genesis, the witness walks to a record, genesis replay is the
  state-completeness check, and restore replays from a base to a target. A
  retention policy must keep the chain addressable. Archiving is safe;
  deleting is not.

## Options

Evidence integrity: the tape is signed evidence (receipts bind
`request_commitment`; the head receipt binds the receipt digests). Any option
must keep every object's bytes and digest exactly as written and keep
digest verification on read.

| Option | Removes | Evidence integrity | Migration | Read path | Verdict |
|---|---|---|---|---|---|
| **(a) Fewer events: group commit** in `appendOps` | files, receipts, Dolt rows and guest journal per op: 3x to 7x (6.6x at a 1 s window); payload bytes unchanged | Unchanged. Each batch is a normal event with a normal payload; replay applies ops in order. | none (new events only) | none | **Do first** |
| **(b) Per-computer segment logs** (git-pack style) with sealed index | file count: growth to about zero, the 9.07M legacy files after backfill; metadata back to near the pre-tape base | Bytes and digests unchanged. Reads still verify the digest. Adds fsync before ack (today's `writeBlob` does not fsync). Optional signed segment manifest. | dual-read, backfill, soak, delete (see below) | by digest through one library; replay becomes sequential | **The substrate fix** |
| (c) Small objects in a database | file count, as (b) | Same bytes. Weaker for a COW filesystem. | similar to (b) | SQL by key | Reject Dolt; SQLite is second choice to (b) |
| (d) Retention of dead computers' tape | at most 13.6% of tape files now; `og` 5.8% | Deleting evidence of a destroyed computer is an owner decision; archive, then delete | per-computer once (b) exists | none | After (b), as a segment-level policy |
| (e) `max_inline=0` | about half the metadata per new file; nothing existing | Untouched | none; new files only; remount | none | Break-glass only |

### (a) Group commit

Where: `projectionTape.appendOps` (`projection_tape.go:97`) and the
appender mutex (`appender.go:338`). A leader takes every op queued while one
append is in flight and sends one batch. A first caller on an idle queue
flushes at once, so idle latency does not change. An optional short timer
could apply to telemetry ops only.

Failure modes to hold (write the tests first):

1. A bad op in a merged batch refuses everyone's write (the dry-run,
   `appender.go:364`). On a batch failure, split and retry the callers one by
   one, and return per-caller errors.
2. Conditional writes. `putBatch` evaluates conditions, then calls the
   interceptor **outside** the engine lock (`dolt_store.go:841-849`). That is
   already check-then-act. A queue delay widens the window. Keep conditional
   batches un-coalesced, or re-check inside finalize.
3. Ordering of ops and sequence marks: batch order must be arrival order;
   `AppendEvent` already releases `eventMu` before the append when the tape is
   bound (`store.go:2009-2011`), so callers do queue today.
4. A crash with ops queued: callers have not returned, so no acknowledged
   write is lost. A crash restart closes open runs ("Restarts End Work").
5. Batch size: cap ops and bytes; the pin handler body limit is unchecked.
6. Proof: file count per 1,000 guest writes before and after on staging, and
   a genesis-replay content-witness equality (`projectionbase/witness.go`).

Expected: new tape files fall from 4.02 per op to about 4.02 / k. At 0.45M
files a day today that is about 0.07M to 0.15M a day. The guest journal
(about 240 KB per event) should fall by a similar factor. That is a
hypothesis from the 10-15 measurement: fixed per-transaction cost
dominates, but it is untested.

### (b) Segment store

Design sketch, per computer:

- `platform-artifacts/tape/<computer_id>/seg-NNNNNN.seg`. Append-only.
  Records are framed: namespace, digest, length, crc32c, bytes. Rotate at
  64 MiB or 50k records.
- Sealing writes `seg-NNNNNN.idx` (digest to offset). The open segment's
  index lives in memory and is rebuilt by scanning at start. A torn tail is
  truncated by the crc scan.
- One library, `Put(computer, namespace, digest, bytes)` and
  `Get(computer, namespace, digest)`. Replace `writeImmutable` and the
  read sites above, including the offline tools (`cmd/ledger-witness`,
  `cmd/choir-rebuild-base`, `cmd/checkpointd`), which read the directory
  directly today.
- Group the fsync across the four records of an event, then ack.
- Sealed segments are immutable. They can be zstd-compressed, copied off node
  and verified by hash. This is also the retention mechanism for (d).
- Receipts could be stored compactly later (id, time, signature, with the
  fields re-derived and the digest checked). Not needed once they are packed.

Migration (red; all steps reversible until the legacy delete):

1. Ship the library with dual read: segments first, legacy file second.
2. Backfill per computer in sequence order from the SQL index
   (`computer_event_append_receipts` already lists the event, receipt and
   payload digests). Verify each digest on copy.
3. Flip writes behind a flag. Soak. Run the verification census: every legacy
   digest resolves from a segment with equal bytes.
4. Delete legacy files in bounded, idle-priority batches. Balance after.
   Keep a manifest of deleted digests.

Failure modes: a corrupt segment has a larger blast radius than one file
(sealed immutability, crc, and an off-node copy of sealed segments answer
this); an index/segment skew after a crash; a partial backfill treated as
complete; deleting legacy files before the census; offline tools missed in
the cutover; unbounded dual-read fallback hiding a missing record. Orphan
pins need a rule (keep in an orphan segment, or drop with a logged manifest).

### (c) Database

- **Dolt: reject.** The guest measured about 240 KB of journal per tape
  event, the platform Dolt is already a large store, and
  `content_store.go:18` records the rule "never in Dolt rows (the captures
  104 G lesson)". Dolt GC needs headroom.
- **SQLite** (`modernc.org/sqlite` is already a dependency): transactional,
  idempotent insert, one file. Failure modes: a single 20 GB-plus file to
  back up and vacuum; random-key inserts rewrite pages on a COW filesystem;
  deleting a computer needs `VACUUM`; the offline tools need the driver;
  corruption blast radius. Workable, but an append-only log is the better
  match for an append-only tape.

### (e) `max_inline=0`

Remount option, reversible, no code, evidence untouched. New files then use
a 4 KiB data extent each instead of inline metadata. Estimate: metadata per
new file falls from about 1.2 KB to about 0.6 KB; data rises by about 4 KiB
per file. At 0.45M files a day that is +1.8 GiB data and -0.26 GiB logical
metadata per day. It trades the scarce resource for the plentiful one and
increases total disk use. It does nothing for the 9M existing files, whose
inline extents stay. Keep it as a break-glass lever if unallocated space
falls below about 10 GiB before (a) ships.

## Recommendation

1. **Now (no tape change).**
   - Add the alert the incident named: unallocated below 20 GiB, and metadata
     chunk free below 2 GiB (usage in `df` does not show this failure).
     Orange, `nix/node-b.nix`.
   - Leave the daily headroom guard as is.
   - Record `max_inline=0` as the break-glass lever above.
   - Owner step, separate: collect the `og` namespace (577,600 files) once
     the corpus-dump decision is made (add it to
     `PLATFORM_ARTIFACT_GC_ACTIVE_NAMESPACES`). Removes 5.8% of files.
2. **Substrate step A: group commit** (red, small, behind a flag, no format
   change). Expected: 3x to 7x fewer new tape files, receipts, platform
   Dolt rows and guest journal writes. This also cuts the guest storage
   growth that the 10-15 record measured, so it pays twice.
3. **Substrate step B: the segment store** (red, larger, dual-read
   migration). Expected: file growth to about zero and the 9.07M legacy
   files gone, which returns an estimated 8 to 11 GiB of logical metadata
   (16 to 22 GiB on disk) after the delete and a balance. The tape bytes
   stay at about 20 GB and compress better in segments. Do B even after A:
   A still grows files with events, and a busy day (350k events) returns
   the 10-16 rate divided by k.
4. **Retention (d)** as a policy on sealed segments after B: archive dead
   computers' segments off node first, delete only on an owner decision.
   Never truncate a live computer's chain.

Mutation class: red for A and B. Before either: a mission goal file via the
throughline skill, with a conjecture, the failure modes above as the
pre-flight answers, and full red ceremony (rollback path: the flag for A;
dual-read and the unchanged legacy files until the census for B).

## Not verified

- The 10-17 drop in rate: cause not traced (hypothesis above).
- The current op mix: the 58% and 31% shares are the paper's 10-15 counts.
  Today's batches were not decoded (payloads are owner data).
- Metadata attributed per namespace: estimated from item sizes and the growth
  cross-check, not measured with a tree dump.
- Whether group commit cuts guest journal bytes in proportion.
- Append latency and queue depth: the arrival windows come from file mtimes
  of events already serialized by the appender, so true concurrent arrival
  is likely higher than shown (a lower bound on the batching gain).
- Whether the paper's `stopped` state is the planned compaction.

## Heresy delta

- discovered: the tape namespaces have no retention, no orphan collection
  and no per-computer deletion path (about 38,000 orphan pins; 13.6% of events
  belong to computers with no ownership record).
- discovered: the offline rebuilder reads the whole chain, or the whole flat
  directory, per run (`projectionbase/source.go:51-107`).
- discovered: `writeBlob` and `writeImmutable` never fsync
  (`service.go:882-898`). A power loss between rename and flush could leave a
  torn tape file under its final name. Unverified consequence.
- discovered: conditional OG writes check, then append, outside the engine
  lock (`dolt_store.go:841-849`).
- introduced: none (docs only).
- repaired: none.

## Repack tool v2: plan and failure modes, written before the code (FP10, 2026-10-22)

Why a second tool. `cmd/taperepack` and `internal/taperepack` (FP9, `plan`,
`copy`, `census` over a manifest) are on main. They were proven on test
computers, and they do not hold at the size Node B measured at 23:15 UTC on
2026-10-22: 4.88 million pin receipts, 2.44 million events, 2.44 million
payloads, about 46 GB by `du`, in three flat directories. Read from the
code, as hypotheses until the synthetic run below measures them:

- `Plan` keeps every manifest entry in memory (`entries []Entry`, about 200
  bytes each, two per pin) and, with `--count-unreceipted`, a 32 byte key
  per receipted digest; `ReadManifest` loads the whole file. Millions of
  entries are gigabytes. The host's memory line is the reason the backup
  verify was moved to weekly.
- `Copy` and `Census` call `Reader.ReadSegment` per record. Each call lists
  the computer's segment directory and, for an absent digest, opens and
  searches every sealed index of that computer (hundreds). At 10 million
  records that is billions of opens.
- The census counts records in a manifest. It does not count the legacy
  files that no manifest line names, and it has no per-namespace or
  per-computer "no verified copy" answer a delete step could consume. A
  later delete step could treat "not in the manifest" or "missing" as
  something other than "not deletable". That is the r4 gap.
- `refuseLiveWriter` refuses when the writer holds the lock at that instant;
  on a live host that makes a run fail at random instead of waiting its turn
  on the lock the appender already takes.

Classification: orange to red. The tool writes only through the production
`tapestore.Appender` into `tape-seg/` (Trace/evidence, red) and never touches
a legacy file. Mutation class red; protected surfaces: computer events, event
payloads, pin receipts, replay, checkpointd, ledger witness. Rollback: delete
nothing; `tape-seg/<computer>` can be dropped to restore today (the reader
falls back to the legacy file). Heresy delta: discovered above; introduced
none; repaired: the tool's memory and r4 gap.

### Design

One streaming engine, three phases, bounded memory. The work directory is
outside the artifacts root and holds spill files only.

1. **List.** Stream the three legacy directories with `ReadDir(1024)` and
   write `(namespace, digest)` records into 256 spill files keyed by the first
   byte of the digest. No file is opened. Names that are not a digest (or
   digest plus `.json` for receipts), directories and links count as
   `other_files`.
2. **Edges.** Read each pin receipt once. The signed receipt is the only
   authority for which computer pinned an artifact and under which media
   type (a payload's own bytes can name any computer; a guest can pin them).
   Write one edge `(artifact digest, namespace, computer, length, eligible)`
   per receipt into 256 spill files keyed by the artifact's first digest
   byte. Eligible is the production rule of `pin` in `event_artifacts.go`: an
   event, or a payload whose media type is the projection batch type.
3. **Join per bucket.** For bucket b, load that bucket's edges and, for every
   computer that has segments, the records of its sealed indexes (by the
   index's fanout range for byte b, so no index is read whole twice) and its
   open segment. The bucket holds about 1/256 of the records. Then, for each
   legacy file of the bucket: read it, check that it hashes to its name,
   decide the computers that need it (a receipt: the computer it names; an
   artifact: every eligible edge's computer, after the length and event
   scope checks), and decide a verdict from the segments of each. Copy mode
   appends what is missing, in bounded batches, through `Appender.AppendFunc`;
   it re-checks presence inside the build function, under the computer's
   lock, then reads each appended record back from the segments.

Verdict per (file, computer), and per file (the worst across computers):
`covered` (every segment copy of that record verifies and equals the legacy
bytes), `uncovered` (eligible, no copy), `corrupt` (a copy fails its crc,
hash or index, or differs from the legacy bytes), `blocked` (the computer's
segment store cannot be read: a bad index, a bad header, a non-final
unsealed segment), `legacy_bad` (the file does not hash to its name),
`too_large`, `ineligible` (an edge exists and none is eligible, or the event
is another kind or names another computer), `unreceipted` (no receipt names
it: an orphan the tape sweep owns), `vanished` (gone between list and read).
Only `covered` is deletable.

**r4, enforced by the type, not by a count.** `Verdict` has unexported fields;
its zero value is not deletable; only the engine builds one; `Deletable()` is
true only for `covered`, which the engine sets only after reading the
segment bytes and comparing them with the legacy bytes in the same bucket
pass. Any later delete step must obtain its verdicts by running the engine
(the verdict is recomputed at delete time) and cannot construct one. A file
with several copies is covered only if every copy verifies, because
`Reader.readSegment` answers with the newest copy and stops at the first
error (residual r2). A computer with any segment-level error blocks every one
of its files. A guard test fails if the package calls `os.Remove`,
`os.RemoveAll`, `syscall.Unlink` or `os.Rename` outside `workdir.go`, which
refuses any path under the artifacts root.

Dry run is the default. `copy` writes only with `--apply`, and only for
computers named with `--computer` (the owner's and the paper's computers are
never copied without the owner naming them). `census` and a dry-run `copy`
write nothing outside the work directory.

Resumable and idempotent. Copy state (`state.json`: root, computers, phase
done flags, the computer table, buckets done, one result file per bucket)
lives in the work directory. A rerun with the same root and computers
resumes at the first undone bucket. A bucket interrupted mid-way is redone,
and the presence check (under the lock, against the live segments) finds the
records already written, so no record is written twice, and a second full run
writes nothing. `--fresh` discards the state. `census` and dry run always
list afresh.

Live writer. The repack takes no lock of its own on a computer's directory; it
uses the appender's: a blocking exclusive `flock` per computer, held for one
append. The platform's appender validates its cached segment under that lock
before every append; when the repack's call has sealed the platform's open
segment (the repack is "another writer" and seals an unsealed segment it did
not write before it starts its own), the platform's next pin re-establishes
(seals the repack's segment, starts a new one). The cost is two small files
per handoff, bounded by the live pin rate (about 4 events an hour on Node B at
23:15 UTC). The repack seals its own open segment per computer when it
finishes (`Appender.Seal`, new, same lock and fail-stop rules), so a computer
nobody writes again does not keep an unsealed tail. A failed write or fsync
poisons the repack's appender: the run stops, the state says where, nothing is
retried in that process.

Memory. The resident set is the bucket's edge and index slice, the
computers' open-segment key maps (at most one segment each), at most 64 MiB
of pending append bodies, and one legacy file at a time (a legacy file larger
than `tapestore.MaxRecordBytes` is `too_large`). It does not depend on the
number of files. Measured on a 200,000+ file synthetic tree (below).

### Failure modes (each gets a test that fails on the base)

- **P1** A legacy file whose segment copy fails verification (crc, hash,
  index, or bytes different from the legacy bytes) is counted `corrupt` and is
  not deletable. A legacy file with one good and one corrupt copy is `corrupt`.
- **P2** A computer with an unreadable segment (bad index, header, or a
  non-final unsealed segment) blocks all its files; none is deletable.
- **P3** An absent copy is `uncovered`, never `covered`; a legacy file nobody
  receipted is `unreceipted`, never `covered`.
- **P4** `Verdict` cannot be forged outside the package and its zero value is
  not deletable; the package has no unlink or rename outside `workdir.go`.
- **P5** A payload whose own bytes name another computer is attributed by its
  receipt, not by its bytes; its copy lands under the receipt's computer.
- **P6** A legacy file that no longer hashes to its name is `legacy_bad`, is
  never copied, and is never covered.
- **P7** An event of another kind, or whose computer differs from its
  receipt's, is `ineligible`; a payload with another media type is
  `ineligible`; neither is copied.
- **P8** Dry run writes nothing: no segment, index, lock file or change under
  the artifacts root (tree snapshot, mtimes included), and counts what it
  would copy. `copy` with no `--computer` or with no `--apply` writes nothing.
- **P9** A second `copy --apply` writes zero records; a run killed after a
  bucket's append and before its result file is rerun and writes none of that
  bucket's records again; a resumed run reports the same totals as an
  uninterrupted one.
- **P10** A run for computer A writes nothing under computer B.
- **P11** A legacy file removed between list and read is `vanished`, not an
  error and not covered.
- **P12** A live writer appending during a copy loses no acknowledged record and
  the copy neither fails nor duplicates (race test, two goroutines on one
  root, `-race`).
- **P13** A poisoned appender (injected fsync failure) stops the run with an
  error; records already acknowledged still read back.
- **P14** Copied records read back through the production `Reader.Read` and
  pass `Reader.Scan`; the copy is byte-identical.
- **P15** A bucket's results sum to the whole: the report's per-computer and
  global tallies equal the files on disk (counted independently in the test).
- **P16** Memory: the resident set on 200,000+ files stays flat (measured, not
  asserted in `go test`).
- **P17** A receipt that is not a signed-shape `PinReceipt` by `corpusd`, or whose
  length is implausible, makes no edge; its artifact is `unreceipted` for
  that receipt.
- **P18** An empty census is not ok (nothing was proven).

Residuals named now. (1) Event-namespace receipts are copied whatever the
event's kind (the receipt does not say), which writes a few receipt records
the production path would leave as files; harmless, covered the same way. (2)
Orphan pins are copied like any other receipted file; the orphan sweep and
this tool are independent. (3) The platform's `reuse` rule: a pin answered by
a record the repack wrote writes one synced legacy copy (counted as
`reuse_legacy_writes`), for a digest that is pinned again with no legacy file.
That cannot happen for a digest whose legacy file is still present.

### Owner ruling on the delete (2026-10-22)

FP0, 23:20 UTC, relaying the owner in session: "4. yes", answering "deleting
the 46 GB of legacy tape files once the repack census shows every one has a
verified copy". The ruling is for the legacy delete of files that the census
marks deletable; it gates on the census, not on a date. It is a follow-on
command (`taperepack delete`), built in its own commit after the census, dry
run by default, and run by FP0 only after the census is read.

### Delete command design (follow-on, red, its own commit)

`taperepack delete --root R --work-dir W --census-receipt <digest>
[--computer ids] [--apply]`. The census writes its report to
`W/census-<digest>.json`, where the digest is the SHA-256 of the report's
canonical JSON (the census receipt digest, printed with the report). The
delete command:

1. refuses unless the receipt file's bytes hash to the digest given, the
   report names the same root, the report is a complete census (every bucket
   done, `ok` for store errors: no `corrupt`, `blocked` or segment error),
   and it is not older than 24 hours;
2. does not trust the receipt for any file: it runs the engine again, and
   each legacy file is unlinked only when the freshly computed `Verdict` is
   `covered` (`Deletable()`), checked once more immediately before the
   unlink on the same bytes it just compared;
3. dry run by default: counts what it would unlink and unlinks nothing;
   `--apply` and the digest together unlink; the digest alone, or `--apply`
   alone, do nothing;
4. unlinks in bounded batches with a pause between batches, writes a deletion
   log in the work directory (namespace, digest, computer, bytes), stops at the
   first unlink error, and never removes a directory.

Failure modes (tests first): D1 no digest or a wrong digest refuses; D2
`--apply` without a digest does nothing; D3 a corrupt copy, a missing copy, a
blocked computer, a `legacy_bad` file, an `unreceipted` file and an
`ineligible` file are never unlinked, even when the receipt lists a clean
census; D4 a stale receipt (older than 24 hours) or one for another root
refuses; D5 a copy that turns corrupt between the census and the delete is
caught by the recomputed verdict; D6 the dry run unlinks nothing; D7 an
unlinked file is gone and its record still reads through `Reader.Read` from
the segment; D8 files outside the three tape namespaces are untouched; D9 a
rerun after a partial delete deletes the rest and no more; D10 the deletion
log has one line per unlinked file.
