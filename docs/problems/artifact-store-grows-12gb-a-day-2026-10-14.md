# Node B's Artifact Store Grows About 12.6 GB a Day; Garbage Collection Never Runs

Date: 2026-10-14, about 12:00 UTC. Found while sizing the performance
report. Status: diagnosed; fix on branch `fix/artifact-store-gc`, not
deployed (see "Fix and owner steps" at the end). Owner: "That much storage growth?
Another storage leak to fix huh."

## Evidence (read-only, Node B)

- Disk: `/dev/md127` 476 GB, 374 GB used (80%), 98 GB free. It was 67% on
  the morning of 10-14 per the cost-track investigation.
- Largest homes: `/nix/store` 168 GB; `/var/lib/go-choir/platform-artifacts`
  125 GB (of which `sha256/` 119 GB, `checkpoint-scratch/` 6.6 GB,
  `capture-bodies/` 1.2 GB); `vm-state` 110 GB (guest data disks and
  snapshots); `platform-dolt` 23 GB; `corpus-dolt` 22 GB (the corpus was
  torn down on 10-09 by owner decision, but its data remains).
- `platform-artifacts/sha256`: files written in the last 24 h: 1,182,580
  files, 12.6 GB; last 7 days: 1,944,690 files, 61.5 GB. The largest objects
  are `projection-base/*` at 8.7-15.9 GB each (10-07, 10-08, 10-09, 09-09).
- Artifact GC runs in dry-run mode only (`nix/node-b.nix`,
  `PLATFORM_ARTIFACT_GC_MODE`, noted in the cost-track investigation).
- At about 12.6 GB a day, the 98 GB left lasts roughly 1-2 weeks, less on
  heavy deploy days.

## Also found

- Snapshot memory files are compressed by btrfs (zstd:3): about 560 MB on
  disk per 8 GiB snapshot (compsize: 6%). The snapshot budget
  (`VM_SNAPSHOT_BUDGET_GIB=48`) counts logical bytes, so it admits about 6
  where real disk would allow about 85.
- Snapshots of computers that were later stopped stay on disk
  (`vm-0f24d3b8...`, `vm-db4bb41c...`: both computers stopped at 08:30).

## To determine before fixing

What writes 1.2 million objects a day (checkpoints? per-boot projection
bases? capture bodies?), which of them anything still references, the
retention each kind needs (restore points, rollback refs), and why GC was
left in dry-run. Old NixOS generations and the corpus database are
separate, simpler cleanups.

## Findings (2026-10-14, about 12:30 UTC; read-only on Node B)

Method: `find -printf` census of every `sha256/` namespace (count, bytes,
mtime), `ls`/`stat` of projection-base descriptors (computer id and
sequence only), chunk mtime histograms against vmctl boot lines in
`journalctl`, the corpusd `artifact gc:` log lines, and a read-only
estimate of the file-CAS live set computed from the on-disk root
manifests (chunk digests only; no file names printed). Nothing was
deleted or restarted; no database or env file was read. Disk at the time:
380 GB used (81%), 92 GB free.

### What the store holds and what wrote the last 24 hours

| Namespace | Files | GB | 24 h files | 24 h GB | 7 d GB | Swept by GC |
|---|---:|---:|---:|---:|---:|---|
| `file-cas-chunks` | 762,397 | 45.98 | 111,341 | **6.17** | 24.58 | yes |
| `projection-base` | 204 | 47.30 | 64 | **4.03** | 31.41 | yes |
| `computer-event-payload` | 1,063,982 | 6.71 | 268,459 | 1.42 | 2.56 | no (tape) |
| `pin-receipts` | 2,114,024 | 1.89 | 537,033 | 0.48 | 0.68 | no (tape) |
| `computer-event` | 1,046,512 | 1.58 | 268,450 | 0.41 | 0.57 | no (tape) |
| `file-cas-roots` | 6,616 | 2.06 | 245 | 0.11 | 0.61 | yes (every row live) |
| `platform-update` | 18 | 3.40 | 0 | 0 | 1.10 | yes |
| `og` | 571,988 | 1.13 | 1,384 | 0.03 | 0.03 | yes |
| `proposals`, top-level `*.txt` | 1,612 | 0.004 | | | | no |

The 24 h sums are 1,186,912 files and 12.65 GB, matching the report
above. Bytes: file CAS chunks 49%, projection bases 32%, event tape 18%.
Files: the event tape is 90% (each event is one event file, one payload
file and two pin receipts).

**1. `file-cas-chunks`: every boot re-uploads the whole Files tree
(6.17 GB/day).** The guest file sync (`internal/autoputer/file_sync.go`)
keeps its "already uploaded" cache in memory only, and
`filecas.SealChunk` seals every chunk under a fresh random nonce, so the
ciphertext digest is new each time. The first sync after a boot (15
minutes in, `defaultFileSyncInterval`) therefore uploads every file as
new chunks; the platform's immutable PUT cannot deduplicate them. Receipt:
computer `computer-7cc96dc6...` wrote 19 bursts of exactly 2,375 chunks
(167 MB) in 24 h; each burst is 15 minutes after a `vmmanager: booted VM
vm-48bc0981...` line (25 boots that day, most of them deploy restarts;
boots followed by another boot within 15 minutes leave no burst). The
same tree is stored 19 times a day for that one computer.

**2. `projection-base`: one full base per checkpoint (4.03 GB/day).**
Written by `checkpointd` (cadence: tail of 2,500 events,
`checkpointDue`); every base is a full snapshot, so a busy computer
writes a growing series (computer-1355b83b: 13 bases, 15 MB to 462 MB,
10-13 15:29 to 18:43). These are one base per head, not per boot, so the
writer is working as designed and the leak is that GC never collects
them. The four giants all belong to `computer-03335285...` (42.1 GB in 4
bases): 09-09 seq 148,431, 15.9 GB (vocabulary v1); 10-07 seq 572,151,
8.7 GB; 10-08 seq 574,653, 8.7 GB; 10-09 seq 577,162, 8.8 GB.
Side finding (separate problem): no base has been published for that
computer since 10-09, and `checkpointd` runs today failed with `og
migrate: ... has migrated identity fields but no verified identity
formula` (04:14 computer-84cc92fc, 10:41 computer-1355b83b).

**3. Event tape: 2.31 GB and 1.07 million files a day.** Retained by
design: its live set is the append-receipt chain, excluded from the
refcount sweep (`artifact_gc.go`). Not addressed here; the 268,000
events a day rate is its own question.

### Why GC has never collected anything

1. `PLATFORM_ARTIFACT_GC_MODE=dry-run` in `nix/node-b.nix`.
2. The per-run cap (`MAX_DELETES=10000`) also counts dry-run candidates,
   and `file-cas-chunks` is swept first. Every logged run (9 in the last
   26 h) reads `mode=dry-run deleted=10000 bytes≈563 MB`, so the sweep
   stops inside the chunks and never reports `projection-base`,
   `platform-update` or `og`. The log line carries totals only.
3. The runner first fires one interval (1 h) after start, and corpusd
   restarts on every deploy, so it ran 9 times in 26 h.

Code defects that would make an active run unsafe (found while reading
the sweep; none has fired, because nothing ran active):

- **Fail-open live set.** `platformUpdateLiveDigests` swallows query
  errors and returns a partial or empty set as a warning; the sweep goes
  on. A database error would delete platform-update payloads that route
  transition receipts name, which are the rollback refs.
- **Unreadable manifest drops its chunks.** The per-computer chunk sweep
  skips an unreadable root manifest as "conservative", but skipping
  removes that root's chunks from the reachable set, so they would be
  deleted.
- **Reuse race.** `og` externalization (`objectgraph_store.go`) returns an
  existing body file without refreshing it. An old unreferenced body that
  is reused between the sweep's live-set read and the new row's commit is
  deleted although the new row names it. The same race applies to file
  CAS chunks once their digests repeat.
- **Torn chunk files.** `writeFileCASImmutable` writes in place under the
  final name. A crash mid-write leaves a truncated file, and every later
  PUT of that digest fails with "size mismatch". Today a retry escapes
  with a new random digest; with repeatable digests it would be stuck.
- **On-demand endpoint.** `POST /internal/platform/artifact-gc` accepts
  `{"mode":"active"}` with the 30-minute default grace from any internal
  caller.

### What is referenced (the retention each kind needs)

- `projection-base`: each computer's watermark `base_ref`; explicit pins
  (`computer_projection_base_pins`, written by restore points through
  `agentcore/restore_base.go` `pinCheckpointBase`); the latest two bases
  per computer; and the seeds of queued, running or retrying jobs.
- `file-cas-roots`: every `computer_file_roots` row (rows are never
  deleted).
- `file-cas-chunks`: the latest root of every computer plus roots recorded
  within the grace window. Nothing restores a historical file root:
  hydration uses only the latest root (`HydrateIfNeeded`). Old roots stay
  cited on the event tape.
- `platform-update`: current route slots, every transition receipt's old
  and new refs, and authorization evidence (the deploy rollback refs).
- `og`: Store B `og_objects.body_ref`. The roughly 570,000 pre-10-09
  bodies are unreferenced since the corpus teardown. The 10-09 doc made
  their collection an explicit owner step, and the corpus dump below may
  still name them.

### Expected reclaim on the first active run (7-day window)

- `file-cas-chunks`: **7.74 GB** (130,057 chunks) eligible now; 38.1 GB
  live (latest roots plus a week of roots, most of it per-boot copies).
  With a 1-day window it would be 18.9 GB. Once the re-upload stops, the
  copies age out over the following week.
- `projection-base`: **0 to 15.9 GB.** Only the 09-09 base is older than 7
  days, and whether it is pinned or a watermark is in the database (not
  read). After 7 days, older non-latest bases follow unless pinned.
- `og`: about 1.1 GB, held back by default (owner step).
- `platform-update`: small; depends on route receipts.
- Total for the first run: **about 7.7 to 23.6 GB.** The exact figure is
  the first uncapped dry-run report after the fix deploys.

### Nix store (168 GB) and the corpus database (22 GB)

- Nix store: 30 `go-choir-guest-image` outputs at 3.26 GB each (98 GB
  apparent) and 25 system generations (1281 to 1305, 21 of them since the
  daily `go-choir-disk-gc` sweep at 00:17). Each deploy adds a generation
  whose closure roots a new 3.26 GB guest image. The sweep keeps `+4`
  generations and ran `nix store gc` at 00:17 (freed 30.8 GiB, disk 62%).
  By 12:00 the disk was at 81%: 92 GB in 12 h, mostly new guest images,
  because the daily cadence is far slower than about 20 deploys a day.
  `min-free = 80 GiB` in `nix.conf` is the only other bound.
- Corpus database: the 22 GB is not the live corpus. The live Store B
  repo (`corpus-dolt/corpus`, 469 MB) **is in use**: `corpus-dsn.env`
  exists (since 09-19), corpusd holds two connections to :13307, the repo
  was written today, and 27 code sites read Store B (`store.corpus()`: og
  bodies, publications, the Autopaper front). The 22 GB is
  `corpus-dolt/rebuild-20261009/corpus.sql` (22.9 GB, the history-free
  dump of the torn-down repo, written 10-09 12:57 to 13:14) plus
  `dump.sh`. Nothing has it open, it has not changed since, and only two
  docs name it (`docs/archive/evidence/world-wire-store-b-inventory-2026-10-09.md`,
  `docs/archive/problems/node-b-disk-free-space-2026-10-09.md`, which left it as
  "compress and copy to node-a, or delete, owner's call").

### Failure modes the fix must hold (written before the tests)

1. Deleting a referenced object: watermark, pinned, latest-two or job-seed
   base; a root's chunks; a route-receipt payload.
2. A live set that fails to load, or loads partially, is treated as
   empty. Includes an unreadable manifest and a namespace whose live set
   is empty while old files exist (the 10-05 wrong-store mass delete).
3. Race with a writer: a reused (deduplicated) old object is deleted
   between the sweep's live-set read and the writer's commit of the
   reference.
4. Crash mid-GC: an unlink with no record, or a half-deleted pair (base
   blob without its descriptor).
5. Crash mid-write: a torn object under its final name blocks later
   writes of that digest.
6. Runaway deletion: one run deletes far more than intended (cap
   exceeded, or the on-demand endpoint bypasses the configured policy).
7. Dry-run hides the eligible set (caps applied to the report), so the
   owner approves blind.
8. Repeatable chunk digests change the confidentiality property beyond
   what the platform already sees.

## Fix (branch `fix/artifact-store-gc`, not pushed) and owner steps

Commits after this doc commit, in order:

1. `fix(platform)`: the artifact GC is safe to run active. Every live-set
   query fails closed. A computer whose liveness cannot be proven keeps its
   chunks. A namespace with an empty live set is reported, never swept.
   Writers refresh a reused object under a reuse lock, and the GC re-checks
   each candidate under it before the unlink. CAS writes go through a temp
   file and rename. Each run is capped at 50,000 objects and 32 GiB. An
   intent record is fsynced to `artifact-gc-log/<day>.jsonl` before every
   unlink. Reports show the uncapped eligible set. The on-demand endpoint
   is clamped to the configured policy. og is held back. `node-b.nix`
   stays dry-run with the production policy (7-day window).
2. `fix(filecas)`: chunk nonces are derived from the chunk (HMAC of the
   plaintext under a key derived from the DEK), so a rebooted guest
   re-seals unchanged chunks to the same digest and the platform stores
   them once.
3. `feat(node-b)`: Nix generation pruning (keep 4) and `nix store gc`
   every 4 hours.
4. `feat(node-b)`: `PLATFORM_ARTIFACT_GC_MODE=active` (the activation
   gate).

Owner steps:

- **A. Push 1 to 3 and read the dry-run report.** After corpusd has been
  up for 10 minutes, `journalctl -u go-choir-corpusd | grep 'artifact
  gc:'` shows the per-namespace report. `eligible_bytes` and `deleted`
  are the first active run's figures; the warnings show any unproven
  computers. The expectation is about 7.7 GB of chunks plus 0 to 15.9 GB
  of bases.
- **B. Push 4 (activation)** once A looks right. Each hourly run then
  deletes at most 50,000 objects or 32 GiB and logs every deletion. To
  roll back, set `PLATFORM_ARTIFACT_GC_MODE=dry-run` (or `off`).
- **C. Corpus dump.** Delete `/var/lib/go-choir/corpus-dolt/rebuild-20261009/`
  (22.9 GB), or compress it and copy it to node-a first. Do not stop
  `go-choir-corpus-dolt`: live Store B runs on it.
- **D. Pre-teardown og bodies (about 1.1 GB, 570,000 files)**, only after
  C: add `og` to `PLATFORM_ARTIFACT_GC_ACTIVE_NAMESPACES`.
- **E. Review of the nonce change (optional).** Deterministic chunk
  sealing lets the platform see when two chunks of one computer are
  identical. The platform already stores that computer's plaintext
  manifests. A review panel can confirm this trade before step 2 deploys.

Still open: the event tape (2.31 GB and 1.07 million files a day, kept by
design) and its 268,000 events a day; full-snapshot projection bases (one
per 2,500 events); `checkpointd` failing on `og migrate ... no verified
identity formula`; and the guest still re-sending (not re-storing) its tree
after each boot (network only).

## Step D enabled (2026-10-20 06:25 UTC)

Precondition C is met: the corpus dump was deleted on 2026-10-20 (owner
notes §27 item 1). The hourly GC report at 06:10 UTC for `og`, still in
dry-run: 577,773 scanned, 7,173 live, 570,600 unreachable and eligible
(1,101,002,048 bytes), 0 in grace, 0 unproven. That matches the plan's
570,000 files and 1.1 GB. `og` joins
`PLATFORM_ARTIFACT_GC_ACTIVE_NAMESPACES`. Each hourly run deletes at most
50,000 objects, so the backlog drains over about twelve runs, every
deletion logged to `artifact-gc-log/`. Rollback: drop `og` from the list.
