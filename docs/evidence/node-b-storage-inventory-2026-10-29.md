# Node B storage inventory (2026-10-29)

Status: read-only investigation, mutation class green (docs only). Nothing on Node B was deleted, moved, restarted or reconfigured. Input to [the data rearchitecture](../design/data-rearchitecture-2026-10-28.md).

Labels: **[V]** verified by a command on Node B today (or by reading the cited code); **[I]** inferred, with the basis stated. Sizes are `du` (allocated, GiB-ish) unless marked "apparent". Root fs: btrfs on md127, 475 GiB, 451 GiB used, 15 GiB free, device unallocated 1 MiB [V `btrfs filesystem usage /`]. Hardlinks/reflinks make per-path sums overstate shared extents; the paths below are not additive to the fs total.

## 1. platform-artifacts (178.2 GB du, 170 GiB) [V one `du --max-depth=2`]

Writer for all of it: the platform service (`internal/platform`, `StateDirectory go-choir/platform-artifacts`, `nix/node-b.nix:1226`). Only `sha256/<namespace>` content-addressed namespaces have a reachability GC (`internal/platform/artifact_gc.go`; `PLATFORM_ARTIFACT_GC_MODE=active`, grace 168h, hourly, `ACTIVE_NAMESPACES=projection-base,projection-base-parts,platform-update,file-cas-roots,file-cas-chunks,og`, `nix/node-b.nix:1285-1305`).

| Path | Size | Files | What it is | Class | Growth | Pruned? | Disposition |
|---|---|---|---|---|---|---|---|
| `sha256/projection-base` | **61.6 GB** | 425 | Materialized per-computer `runtime.db` replay bases (first bytes of the largest file read `runtime.db`) [V]; `internal/projectionbase`, platform event handlers. Read by wake/restore of a computer from the tape. | **derived-rebuildable** from tape + file CAS [I: a base is the replay result; descriptor seam in `projectionbase/types.go`] | Bursty. By file mtime [V]: 10-08/09 3 files 26 GB (8.7+8.8+...), 10-27 68 files 9.6 GB, **10-28 53 files 28.4 GB incl. one 26.5 GB file at 09:27** (the largest object in the store; allocated 25.8 GB, not sparse). | GC active, but 10-27..29 deleted only 0.73 + 0.07 + 0.12 GB [V gc log]; 7-day grace plus liveness (current/pinned/rollback) keeps almost all. | **Needs a decision**: the 26.5 GB base and the two 8.7 GB bases (10-08/09, beyond grace so still referenced by something) are 44 GB of rebuildable data. Find which computer/pin holds each (GC report lists eligible/live per namespace), then add a retention rule: latest base per computer plus pinned restore points. |
| `sha256/file-cas-chunks` | 27.9 GB | 197 per-computer dirs, 150-225 MB each for the large ones [V sample of 3] | Encrypted 4 MiB chunks of each computer's files tree (`internal/filecas`, `autoputer/file_sync.go`). Key per computer, so no cross-computer dedup. | canonical for the owner's files; **but** each of ~190 computers carries ~150 MB, likely the same seeded `files/Source` copy [I: host `/var/lib/go-choir/files/Source` is 289 MB; unverified] | Slow, ~1 dir per new computer | GC active (<1 MB/day deleted) | keep; redesign (shared base files, not a per-computer copy of seeded content) |
| `tape-seg/` | 27.2 GB (26.4 GB is one computer, `computer-1355b83b…`, 967 files) | 16 computers | Append-only segments of event + payload pins (A2, `PLATFORM_TAPE_SEGMENTS=write`, `internal/tapestore`). The chain of record. | **canonical** (the tape) | **~2.5 GB/day, steady** [V seg mtimes: 10-21 2.2, 10-22 5.8, 10-23 3.6, 10-24 2.4, 10-25 2.4, 10-26 2.6, 10-27 2.6, 10-28 2.0, 10-29 2.7 GB] | none (append-only; compaction/snapshot is the redesign) | keep; this is the fastest *steady* growth in the store |
| `sha256/pin-receipts` | 19.7 GB | **4.93 M** tiny `.json` files (~4 KB allocated each) [V `ls | wc`] | Signed PinReceipts for legacy (pre-segment) pins (`event_artifacts.go:121-125`) | canonical-ish evidence, but 4.9 M inodes | dir mtime = now [V], so still written (segment fallback or computers not on segments) [I] | **none** (not an active GC namespace) | **needs a decision**: repack into segments (the `taperepack` mission, `taperepack-work/` is its leftover); find out why legacy pins are still written today |
| `sha256/computer-event-payload` | 17.8 GB | 2.46 M | Legacy per-file private payload pins | canonical (tape), legacy layout | dir mtime = now [V] | none | same as above: repack |
| `sha256/computer-event` | 9.9 GB | 2.46 M | Legacy per-file canonical events | canonical (tape), legacy layout | dir mtime = now [V] | none | same: repack |
| `capture-bodies/` | 9.6 GB | many (256 prefix dirs) | Captured web bodies (`internal/platform/captures.go:145`) | cache / evidence of fetched pages [I] | dir mtime 2026-10-13, so **not growing** [V] | none | **needs a decision**: check whether the corpus (Store B captures) holds the same bodies; likely delete-by-rule (older than N days) |
| `sha256/platform-update` | 1.4 GB | few | Platform update payloads | derived/release | 4 objects (330 MB) deleted 10-27 | GC active, working | keep |
| `sha256/file-cas-roots` | 1.0 GB | ~thousands | Files-tree root manifests | canonical index | ~180-300 retired per day [V gc log], 83-138 MB/day deleted | GC active, working | keep |
| `computer-inputs/` | 0.7 GB | | Candidate inputs/blobs, last modified 2026-07-16 | dead-ish [I] | none | none | needs a decision (probably delete) |
| `sha256/og` (+ top-level `*.txt` ~12.5 K, 2026-05..10) | 0.5 GB | 12.5 K | Pre-teardown corpus bodies; `og` is deliberately outside the active GC set ("owner step") | hand-kept legacy | `.txt` files accrue ~1 K/day through 10-16, slower since | none by design | needs owner step |
| `artifact-gc-log/`, `tape-sweep-archive/` | 0.43 + 0.35 GB | 53 K archive files | Deletion intent logs (JSONL/day, ~0.4 MB) and the 10-20 tape-sweep manifests | hand-kept evidence | tiny | none | keep; archive can be compressed |
| `computer-checkpoints`, `checkpoint-scratch`, `projection-base-parts` | ~0.1 GB | | checkpoint JSON refs; scratch reaped by `checkpointd/scratch_sweep.go` | derived | tiny | scratch sweep working (last logged 10-23) | keep |

Answer to "what is in platform-artifacts and what grows fastest": 35% is `projection-base` (derived `runtime.db` replays), 15% each is the segmented tape (canonical) and per-computer file CAS, and 27% (47 GB) is the legacy one-file-per-pin layout (7.4 M inodes). Fastest *bytes*: projection-base bursts (28 GB in one day, 10-28); fastest steady growth: tape-seg at ~2.5 GB/day; fastest *inode* growth: legacy pins, which should have stopped after 10-21 but whose directories are still being touched [V mtime; rate not measured, a single full scan was the limit].

## 2. Dolt stores

| Path | Size | Writer / reader | Class | Growth | Retention | Disposition |
|---|---|---|---|---|---|---|
| `platform-dolt/platform` (Store A) | 56.6 GB du (~54 live) | platform/corpusd control store | canonical (control) | large; the squash copy that failed was already purged today (reported) | Dolt GC/squash manual | redesign (the rearchitecture target) |
| `platform-dolt/.choir-backup/platform` | 54.3 GB du (51.8 GiB exclusive per `btrfs filesystem du`) [V] | `dolt_sync` copy made by the nightly `scripts/off-host-backup.sh:233,436` | **backup copy** of Store A on the same disk | rewritten each night; exclusive extents, so a full second copy | none (replaced in place) | **needs a decision**: if the restic upload only needs a consistent point-in-time, a reflink/btrfs snapshot or streaming dump avoids a permanent 52 GB local copy. Reclaims ~52 GB. |
| `corpus-dolt/corpus` (Store B) | 15.0 GB (+1.1 GB `.dolt`) | corpusd | canonical (Autopaper object graph, captures, publications) | unmeasured today | none | keep, redesign with Store A |
| `offhost-backup/restic-cache` | 8.0 GB | restic | cache | grows with repo | restic-managed | safe to cap/prune; `status.json` shows each nightly run adds ~52.7 GB to the remote repo (raw 365 GB) [V], i.e. the Dolt backup copy is poorly deduplicated, which is a remote-cost finding |

## 3. vm-state (103.6 GB du) [V]

| Path | Size | What | Class | Notes |
|---|---|---|---|---|
| `vm-1669020a…` (paper) | 57.4 GB (btrfs exclusive 54.7 GiB of a 137 GB apparent sparse `data.img`) | the paper computer's data disk | canonical (the always-on computer) | the in-guest Dolt etc.; growth not measured today |
| `.jail/firecracker/*/root` | 28.7 GB (34.1 GB incl. shared) | firecracker chroots. `data.img` and `store.img`/`initrd`/`vmlinux` are **hardlinks** (link count 2-3) of the vm-state dir and guest-release files, so most of this is the same extents counted once [V `ls -l` nlink] | not extra | do not count it twice |
| `candidate-fleet-bf04bbc7…` | 8.4 GB + 8 GiB memory snapshot | test candidate; **the only release-bound memory snapshot on disk** (`snapshot/mem`, 8 GiB, 10-29 01:13) [V] | dead/test | delete on fleet retirement |
| `candidate-fleet-e15cb89f…` | 5.4 GB | candidate | dead/test | same |
| `vm-48bc0981…` | 3.2 GB | old VM (10-22) | dead | needs a decision (stopped since 10-22, console logs only) |
| `.data-image-templates` | 0.43 GB | one 32 GiB sparse template | cache | keep |

Hibernated computers: vmctl lists 145, but vm-state has only 16 entries, so **hibernated computers hold no local memory snapshot or data disk** [V ls]. Their state is the tape, file CAS and projection bases in platform-artifacts, which is what a wake rebuilds from [I]. So the epoch 2 to 3 refusal affects 1 snapshot (8 GiB, the candidate above), not 145; if those computers wake cold, the 61 GB projection-base set is what they depend on.

## 4. Other paths

| Path | Size | Class | Disposition |
|---|---|---|---|
| `/nix/store` | 49.3 GB | derived (system closures) | `nix-collect-garbage` after generation pruning; unmeasured how much is dead |
| `guest-releases` (3 releases x 3.9 GB) | 11.8 GB | derived; `store.img` hardlinked into the jails | keep current; older release dirs prune by rule (3 kept today) |
| `builder/releases` | 6.0 GB | derived build output | prune-by-rule candidate |
| `/swap` | 16.8 GB | swapfile | keep (resource decision) |
| `/root` (incl. `.cache` 1.1 GB) | 4.6 GB | cache/misc | needs a look |
| `/var/cache` | 3.3 GB | cache | prune |
| `services` (+`-previous` copies of each binary) | 2.5 GB | deployed binaries x2 | keep (rollback) |
| `retired-bases-20261028` | 0.45 GB | hand-kept evidence: one retired base (10-13) | delete after owner OK |
| `files/` | 0.31 GB (`Source` 289 MB) | host copy of seeded source | likely the seed duplicated into every computer's file CAS |
| `preflight-proof` 0.16, `archive` 0.10, `gateway` 0.09, `source-service` 0.02, `mail` 0.01 | small | evidence/canonical | keep |
| `taperepack-work` | ~2 MB now | dead scratch of the repack run | delete |
| `/data` (@data: March-April per-user subvolumes 4.1 GB + 62 snapshots 4.9 GB) | 9 GB | dead (old), hand-kept | needs owner OK, then delete |
| journald | 0.31 GB | log | fine |

## 5. Top growth drivers

1. **projection-base bursts.** One 26.5 GB `runtime.db` replay on 10-28 and 28.4 GB that day in total, 38 GB over 10-27/28. This alone explains most of the observed ~85 GB/day step. Derived data, with a 7-day grace plus liveness; GC removed only ~0.7 GB of it per day.
2. **Dolt backup copy on the same disk** (52 GB exclusive, rewritten nightly), plus Store A itself (54 GB) and a failed squash purged today: the Dolt family is 110 GB, 22% of the disk.
3. **The tape** (`tape-seg`, ~2.5 GB/day for the paper computer, no compaction). Canonical, so the long-term rate (~75 GB/month) needs a snapshot/compaction design rather than deletion.
4. **Legacy pin files** (7.4 M inodes, 47 GB) that are still being written and have no GC namespace.
5. **Paper VM `data.img`** (57 GB real in a 137 GB sparse file) and per-computer file-CAS copies (~28 GB across ~190 computers).

## 6. Safe-now deletions vs needs-a-decision

Nothing was deleted by this investigation.

**Safe now (low risk, rebuildable/dead; still confirm with the stated check):**

| Item | Size | Check before deleting |
|---|---|---|
| `retired-bases-20261028` | 0.45 GB | it is named retired; confirm no descriptor refs |
| `taperepack-work` | ~2 MB | no repack running |
| `/data` old subvolumes + 62 snapshots | 9 GB | owner confirms March-April data is obsolete |
| `candidate-fleet-*` dirs and the 8 GiB snapshot | ~14 GB + 8 GiB | vmctl says retired (the data is jailed; do through vmctl, not rm) |
| `vm-48bc0981…` | 3.2 GB | not in vmctl's running set |
| `/var/cache`, restic-cache, `/root/.cache` | ~12 GB | caches; restic cache refills (slower next backup) |
| `computer-inputs` | 0.7 GB | untouched since 2026-07-16 |
| `nix-collect-garbage` (old generations) | unknown | keep the rollback generation |

**Needs a decision (bigger, higher stakes):**

| Item | Size | Decision |
|---|---|---|
| The `.choir-backup` Dolt copy | 52 GB | Replace the permanent local copy with a transient snapshot; changes the backup procedure (red surface). |
| projection-base retention (26.5 GB + 2 x 8.7 GB + the long tail) | up to ~44-55 GB | Retention rule: latest base per computer plus pinned restore points; first identify who references each (GC dry-run report). Wake-time cost is a rebuild from tape. |
| Legacy pins (pin-receipts, computer-event, payload) | 47 GB | Finish the repack into segments, and find why they are still written; then they can leave the live tree. Red (tape/evidence). |
| `capture-bodies` | 9.6 GB | Verify the corpus has the bodies; then prune by age. |
| Per-computer file CAS seeds | up to ~25 GB | Share the seeded base content across computers instead of copying; design change. |
| `og` namespace and 12.5 K `.txt` | 0.5 GB | Owner step already named in `nix/node-b.nix`. |

## 7. Gaps and how to close them

- Growth rates for Store A, Store B, the paper `data.img` and the legacy pin directories were not measured (one full scan only). A `du`-per-day cron into a ledger, or the `btrfs filesystem du` per subvolume, would measure all in one pass.
- The identity of the computers behind `tape-seg/computer-1355b83b…`, the 26.5 GB base, and the per-computer file-CAS sizes were not mapped to vmctl ids; the GC report (`/internal/platform/artifact-gc`, dry-run) lists live/eligible per namespace and answers the "who references it" question without a tree scan.
- No btrfs quotas, so per-subvolume exclusive sizes need `btrfs filesystem du` (slow).

## Correction: capture bodies are growing (FP0, 2026-10-29 ~22:45 UTC)

Row 1's "dir mtime 2026-10-13, so not growing" is wrong: it read the 256
prefix directories' mtimes, which change only when a prefix is first made.
File mtimes [V `find -newermt`]: 154,332 of 154,487 files (9.37 GB) are
newer than 2026-10-13, i.e. the store refilled after the 10-09 corpus reset;
5.70 GB since 10-20, 1.05 GB since 10-27, 0.85 GB since 10-28 (18,979 files),
0.49 GB on 10-29 so far. Rate about 0.5-0.8 GB and 10-19 K files a day.

What the code does [V `internal/platform/captures.go:78-145,212-240`]: one
plaintext file per body, `capture-bodies/<2 hex>/<sha256>`, keyed by the
hash of the plaintext, truncated at 1 MiB, written with fsync and rename,
never rewritten, no GC namespace. `GET /internal/platform/captures/bodies/<sha>`
serves any body to any bound guest or internal caller ("captures are shared
content"), with no owner check and no encryption, and identical bodies dedup
across owners. That is correct for public web captures. It is a tenancy gap
for any capture of private content (a logged-in page, an owner's own
document), which the owner rules private: such a body would be readable by
every computer that learns its hash, and its hash confirms its presence.
Input to the object-storage design (public versus per-owner tiers); no
evidence yet that a private body is in the store.
