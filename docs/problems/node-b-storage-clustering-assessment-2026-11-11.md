# Node B storage: clustering assessment and release plan (2026-11-11)

Status: open. Mutation class of this document: green. Nothing was changed or
deleted on Node B. Every plan item below is a proposal; items marked "owner"
wait for the owner.

Why this exists: the operating contract requires a clustering assessment
after three disk incidents on one subsystem (2026-10-09, 2026-10-22,
2026-11-11) before the next fix. Prior record of the third:
[node-b-deploy-refused-disk-headroom-2026-11-11.md](node-b-deploy-refused-disk-headroom-2026-11-11.md).
The owner said: "Let's clean up the disk and improve our mechanisms for
storage release."

Tags: **[M]** measured by me today (read-only, about 11:45 to 12:05Z).
**[C]** read in code or config at main `1abd0ebab`. **[E]** estimate or
inference. **[?]** not determined.

## 1. Summary

1. The disk is not filling from one thing. Between 2026-11-01 and 2026-11-11
   free space fell from 227 GiB to 51 GiB, 176 GiB in ten days, about
   17 GiB a day [M, growth meter]. Seven stores grow at once, and the only
   automatic collectors act on three of them (nix store, file-cas chunks,
   journals and guest releases).
2. The first two incidents were solved by freeing the biggest grower of that
   day (Dolt corpus 104 GiB; the paper's data image). Neither fix bounded the
   next grower. That is the cluster (section 2).
3. One large block is stranded and needs no data to be touched: btrfs has
   allocated 25.1 GiB of metadata (twice, 50.3 GiB raw) and uses 3.3 GiB
   [M]. About 38 GiB would return to free space with a metadata balance
   [E]. The metadata was filled by 10.9M loose tape files until they were
   packed into segments on 2026-11-01; the chunks were never given back. The
   existing `go-choir-btrfs-headroom` unit only balances data chunks.
4. Even a perfect clean-up buys days, not months. Legitimate-looking growth
   (tape, receipts, the paper's image) is about 7 to 8 GiB a day [E]. With
   roughly 75 GiB of one-time releases (section 7a) the host is back at the
   floor in about four days at today's total rate, and in about nine days if
   all waste is also stopped. The tape, the receipts table and the images
   need a sink: off-host tiering, or a bigger disk.
5. The 60 GiB floor is wrong in both directions (section 9): too high for the
   steady day, too low for the measured worst transient (the paper's image
   grew by 33 GiB for hours on 2026-11-09). Do not lower it to get a deploy
   through.

## 2. The cluster: what the three incidents share

| | 2026-10-09 | 2026-10-22 | 2026-11-11 |
| --- | --- | --- | --- |
| Free | 90 GiB | 72 to 85 GiB, falling 250 MiB a minute | 51 GiB |
| Floor | 80 GiB min-free | 90 GiB deploy floor | 60 GiB deploy floor |
| Top grower | Dolt corpus oldgen 104 GiB, 10 nix generations, quarantined images | the paper's `data.img` 80 GiB (no discard), Dolt oldgen +2 GiB/h | tape 83, images 114, receipts 26, chunks 54, capture 19 |
| Relief used | deleted by hand, owner approved | early compaction, deleted a backup | none yet |
| What was left unbounded | every other surface | every other surface | every surface the previous fix did not name |

Shared causes:

1. **Fixes bound the last grower, not the surface.** The O10 invariant
   ("declared bound per growth surface", archived record 2026-10-09) was
   written and never became a table. Each store got its own record and its
   own timing, so the next grower was always unbounded.
2. **Automatic release touches only regenerable data.** `go-choir-disk-gc`
   ran at 00:44Z today and reported `vms_reclaimed 0, retention_pruned 0`
   [M]; its useful work was nix GC (9.6 GiB), journals (165 MiB) and
   release dedupe. Nothing in the daily sweep can free state: tape, images,
   receipts, chunks of dead computers, capture bodies.
3. **Everything that frees state is owner-gated, and the gate is reached at
   the floor.** The owner is asked to delete under pressure, with the
   inventory built that day. Records F2, F3, F5, F7 (2026-10-22) were
   parked for that reason and never re-opened.
4. **The alarm fires at the floor.** `go-choir-host-disk-alarm` warns when
   free is under 60 GiB, the same number that refuses the deploy [C
   `nix/node-b.nix` alarm block]. Warning and refusal are simultaneous. The
   growth meter has the data to forecast and nobody reads it.
5. **Each record's numbers went stale.** The 2026-10-30 data-system cutover
   deleted Dolt, the host store compactor and the daily 04:00 sweep (commit
   `871ab6531`). The Oct-22 records still describe a host that no longer
   exists, and still say "no discard" and "lane not landed" (both now
   wrong; section 4).
6. **Evidence is short.** The journal is vacuumed to 181 MiB, about 30 hours
   [M]. Only the hourly growth ledger survives longer.

## 3. Evidence: what holds the disk now

Root: btrfs, 475 GiB, 380 GiB used, 52 to 56 GiB free (88%) [M]. Data
allocated 405.0 GiB with 372.6 used, metadata 25.1 GiB (DUP) with 3.3 GiB
used, unallocated 20.0 GiB [M `btrfs filesystem usage`]. Compression
`zstd:3` and `discard=async` are on.

Top level, allocated bytes (`du`; reflinks and compression make the sum
exceed `df`) [M]:

| Holder | GiB | Notes |
| --- | --- | --- |
| `platform-artifacts/tape-seg` | 82.8 | 101 computer dirs; paper 66.0 |
| `vm-state` data images | 113.9 | 12 state dirs; paper 44.7, d@e 25.8, b@c 19.2 |
| `sha256/file-cas-chunks` | 53.6 | paper 21.5; 13 registered dirs 26.7; 199 unregistered dirs 26.9 |
| `capture-bodies` | 19.5 | no collector |
| PostgreSQL | 30.5 | control 25.9, of which append receipts 25.8 |
| `/nix/store` | 35 to 39 | sawtooth, floor about 26 |
| `file-cas-roots` | 7.6 | collector retires after 7 days |
| `platform-update` | 7.2 | collector |
| guest releases, tools, bases | 12.5 + 2.4 + 1.2 | 5 releases kept |
| swap file | 16.0 | 0.22 GiB in use |
| `taperepack-work` | 1.8 | audit evidence |
| `/root` (go mod cache 3.4, playwright 0.6, npm 0.4, cutover 0.5) | 5.4 | |
| `computer-inputs` 0.7, `tape-sweep-archive` 0.3, `og` 1.0, gc-log 0.4 | 2.4 | |

## 4. Answers to the seven questions

### Q1. Per holder: what releases it, is it running, how fast does it grow

Growth rates are from the hourly ledger `/var/lib/go-choir/growth-meter/2026-11.jsonl`
(252 readings, 2026-11-01 to 2026-11-11) [M]. The ledger does not cover
`file-cas-chunks`, `capture-bodies` or `platform-update`; those rates are from
file mtimes.

| Holder | What releases it (rule) | Running on Node B? | Growth per day | Verdict |
| --- | --- | --- | --- | --- |
| nix store | `go-choir-nix-gc` every 4 h, enforce, keep 2 generations, builder keep 2; `go-choir-disk-gc` daily; min-free 24 GiB / max-free 40 GiB (`/etc/nix/nix.conf`) [M] | yes: 00:44Z freed 9.6 GiB, 04:39Z freed 0 ("0 store paths deleted"), 08:31Z freed 0.9 | sawtooth 25 to 40 GiB; +9.7 GiB across the ten overnight deploys | works; floor is the pinned set: generations 1979 and 1980, `/run/booted-system` (old guest image 3.6 GiB), `prebuild-results/nixos-result` (the refused deploy's closure) |
| tape segments | none. `tapestore.CompactSealed` rewrites sealed segments losslessly [C `compact.go:18-55`]. `taperepack` is manual only, no unit or timer [C]. The tape sweep removes destroyed computers' loose files on demand [C] | compaction yes; nothing deletes live tape | all computers +3.9 GiB/day (Nov 5 to 11); paper +2.4 (mtime histogram, 11-02 to 11-10 each day 1.8 to 3.4) | no sink at all |
| data images | guest `fstrim` hourly + Firecracker 1.17 discard on the data drive, fleet-wide (`VM_DATA_DISCARD=1`, `nix/node-b.nix:1723`, `nix/autoputer-vm.nix:937`) [C]. The paper's `fc-config.json` shows `"discard": true`, build 1.17.0 [M] | yes (images fall by 1.5 to 3 GiB at some hours, e.g. paper -2.8 at 08:00Z today [M]) | paper +2.6, d@e about +4 with spikes, four idle computers +3.4 each over 30 h then flat | works for steady state; transient peaks are not bounded (Q3) |
| file-cas chunks | artifact collector, `GRACE=168h`, `INTERVAL=1h`, 32 GiB/run cap; a chunk no root names is kept 24 h [C `artifact_gc.go:173`] | yes: deleted 4.0, 4.8, 5.0, 5.4 GiB on 11-05, 07, 08, 10; 5.85 GiB by 11:22Z today [M gc-log] | paper wrote 2.6 GiB on 11-10 and 18.7 GiB on 11-11 between 04Z and 07Z [M mtimes] | collects, but a root every 15 minutes kept 7 days holds old versions live; dead computers' chunks never collected |
| file-cas roots | F4 retires roots older than 168 h except the latest [C `fbc7df0b5`] | yes: 632 to 876 retired a day [M] | +0.7 GiB/day (0.9 to 7.6 over 10 days) | bounded at roughly 8 to 9 GiB |
| capture-bodies | none; F7 never landed [C] | n/a | 0.3 GiB/day (11-04) rising to 2.0 (11-09, 11-10); 168k to 260k files [M mtimes] | unbounded and accelerating |
| PostgreSQL receipts | receipt compaction on the paper, the owner's computer and d@e [commit `dc7471eca`] | yes for three computers | +553k rows, +1.0 GiB a day (11.7M rows, 25.8 GiB) [M] | grows with every projection batch |
| guest releases | `go-choir-guest-release-retention` every 30 min, keep current + previous + in-use, 5 now [M] | yes; 12.75 GiB listed, 10.1 GiB real | flat | works |
| VM state of dead test computers | vmctl `PruneRetention` every 2 min, active, with the test-class idle rule at 168 h (`nix/node-b.nix:1931-1947`) [C] | yes; `retention_pruned 0` now because only 13 computers remain | 0 | works; cannot touch `active` computers |
| journals | disk-gc vacuum | yes | 181 MiB held | works |
| swap | none | n/a | 0 | 16 GiB reserved, 0.22 used |

### Q2. Which proposed fixes landed

Checked with `git branch --contains` against main. None of the named lane
branches exists in `origin` or `mirror`; they were merged and deleted.

| Proposal (2026-10-22 records) | State | Evidence |
| --- | --- | --- |
| `lane/fp0-vm-retention` (test-class idle rule, quarantine age-out) | **Landed.** The record's "not landed" is stale. Active on Node B at 168 h. | `73ce6300c` is in main; `nix/node-b.nix:1944-1947` |
| `lane/fp0-disk-pressure` (host store compaction on pressure) | Landed in observe, then **deleted** by the data-system cutover. `VMCTL_STORE_COMPACT_*` in `nix/node-b.nix:1958-1964` is dead config | `2f6b76c1a`, deleted by `871ab6531` |
| RES-16 data-drive discard | **Landed**, now fleet-wide, Firecracker 1.17.0. The Oct-22 root cause "no discard" is fixed | `9aa45e644`, `nix/node-b.nix:1723`; paper `fc-config.json` [M] |
| fp0-nix-store-policy (enforce, 24/40 GiB band, alarm) | Landed; mode `enforce` seen in today's journal | `af4589eac`, journal [M] |
| F1 checkpoint-scratch sweep | Landed, then obsolete (hosts no longer rebuild checkpoints) | `8c84b3f03`, `871ab6531` |
| F4 retire old file-cas roots | Landed | `fbc7df0b5` |
| F6 compress the artifact GC log | Landed (files from 11-08 and earlier are `.zst`) | `11219965a`, `ls` [M] |
| F2 / F3 dead-computer bases and chunks | **Not landed** | no commit |
| F5 shorter file-CAS window | **Not landed** | no commit |
| F7 capture-bodies retention | **Not landed** (owner yes needed) | no commit |
| tape repack | Landed as a manual tool. Run by hand on 2026-11-01. No timer | `2c2d3515e`, `81b74974b`, `06731745d`; `taperepack-work` mtimes [M] |

Why the unlanded ones stalled: F2/F3/F7 delete owner-class or evidence data,
so each waited for an owner yes and a dead-computer proof; none had a
trigger that would ask. F5 waited for a measurement that nobody took. The
tape had no owner: the retention record called repack "another lane's", and
that lane finished the pack-and-delete of loose files and stopped.

### Q3. The paper's image

- **Compaction today is not the host compactor.** The host-side routine
  (`cp --reflink`, loop mount, `fstrim`, swap) was deleted on 2026-10-30
  [C `871ab6531`]. The attempts file holds one entry, from its last run:
  `file_bytes_before` 41.0e9, `file_bytes_after` 66.1e9,
  `store_bytes_after` 29.7e9 [M]. It is never written again. "After" larger
  than "before" is not explained by the file; the field is allocated bytes
  of the swapped-in file measured while a reflink clone still shared the
  extents [E]. Treat it as a dead record.
- **What runs now** is the in-guest compact copy: `VACUUM INTO` a staged
  copy of `state.db`, swapped in at a planned restart [C
  `internal/store/compact.go`]. It needs 1.2x the live size free *inside
  the guest* and a 2 GiB guest floor. It does not look at the host's free
  space [C].
- **That copy is visible on the host as a transient.** The paper's
  allocated image: +19.1 GiB at 11-01 04Z then -19.3 at 06Z; +20.7 at 11-02
  05Z, held 25 hours, then -20.8; on 11-09 +33.0 (37.6 to 70.5) at 05Z, -20.5,
  +21.2, -35.4 at 10Z [M hourly ledger]. These peaks are the real worst case
  for the floor.
- **Live versus reclaimable.** The image is 44.7 GiB allocated of 128 GiB
  apparent [M]. Since 11-09 10Z it rose from 36.3 to 44.7 GiB (+4.1 GiB/day
  over 49 h, +2.6/day over six days) while hourly guest `fstrim` and discard
  are active. I could not read the guest's own used figure (its `/health`
  needs the internal token). If discard is working, most of the 44.7 GiB is
  live guest data and little is reclaimable [E, p=0.7]. The way to settle it
  is the guest's `persistent_disk` field in `/health`, read by someone with
  the token.
- **Do guests discard?** Yes, by configuration: drive `discard: true`, hourly
  `fstrim` timer, Firecracker 1.17.0 [C, M]. I did not see an fstrim line in
  the console logs; the console does not carry it [M].
- **Four idle computers grew 3.4 to 3.7 GiB each between 11-10 00Z and
  11-11 05Z** (steady 0.09 to 0.27 GiB an hour), then stopped at the 04:31Z
  deploy [M]. Cause unknown [?]. One candidate: every deploy refresh restart
  rewrites state on each guest and the ten overnight deploys account for it.

### Q4. The paper's tape

- 3,411 files: 1,706 `.seg` (64.3 GiB, 64 MiB each) and 1,705 `.idx`
  (1.7 GiB), written at most 64 MiB or 50,000 records per segment [M, C].
  `du` equals apparent size, so btrfs compression gains nothing on them [M].
- By day (mtime) [M]: 2026-10-21 to 10-30 2.1, 5.5, 3.4, 2.3, 2.3, 2.5, 2.5,
  1.9, 2.6, 1.4; **2026-11-01 14.6 GiB** (1,197 files); then 1.9, 1.8, 2.4,
  2.2, 3.4, 3.2, 2.2, 2.7, 2.6 and 1.7 so far today.
- **The 11-01 block is the output of the loose-to-segment pack.** The growth
  meter shows the paper's tape going from 27.4 to 41.8 GiB that day. The
  inputs were 10.9M loose files that `taperepack delete` removed (the 1.77 GiB
  `delete-*.jsonl` manifest in `taperepack-work` is its log, 2026-11-01
  15:36) [C `06731745d`, M]. So the 14.6 GiB replaced files that no longer
  exist; it is not duplicated data. It is also why metadata is now nearly
  empty (section 1, point 3).
- **By kind, by count.** The paper's append receipts in the last 24 hours:
  555,628, of which `projection_batch_recorded` is 23,481 in the last hour
  against 179 `artifact_produced` and 3 `file_root_committed` [M ledger].
  About 98% of records by count are projection batches. I did not open any
  segment, so share by bytes is [?]. Payloads of non-public kinds are sealed
  to the computer and will not compress; version-2 records are already one
  zstd frame each [C].
- **What the design permits.** The tape is canonical: a lost disk was rebuilt
  from "sealed snapshot + tape tail + file root + escrowed key" on
  2026-10-30 [docs/computer-ontology.md:171]. A guest recovers from its own
  sealed snapshots plus the tape tail; snapshots are taken every 10,000
  events or hour [data-system frame §12]. So the segments older than the
  last verified snapshots are not needed to restore a running computer.
  They are needed for provenance, genesis rebuild and audit. That fits
  cold tiering, not deletion.
- **Already off-host.** The nightly restic backup to node-a includes all of
  `platform-artifacts`, so every sealed segment exists in the daily
  repository (114 GiB of 450 quota, last good run 02:50Z, added 40.9 GB)
  [M, C]. That repository keeps only two snapshot sets, so it is a backup,
  not an archive. A separate never-pruned archive repository exists in the
  script (`off-host-backup.sh archive TAG PATH`) [C]; its size is [?].
- Nothing in the design blesses thinning or dropping live computers' tape.
  Only a destroyed computer's tape may be deleted (owner notes section 26,
  quoted in the retention record).

### Q5. Dead and test computers

The registry the code uses is `vm-state/ownerships.json` (read directly;
no token needed), which holds 13 computers [M, C]. Route
`GET /internal/vmctl/list` and `retention-plan` give classes and per-test
verdicts but need the internal token even from a root shell [C
`handlers.go:1446`]; I did not use them.

| Store | Registered (13) | Unregistered dirs | Unregistered size |
| --- | --- | --- | --- |
| `file-cas-chunks` | 26.7 GiB | 199 | **26.9 GiB** (each 0.14 to 0.17 GiB) |
| `tape-seg` | 81.7 GiB (12 dirs) | 89 | 1.3 GiB |
| `file-cas-roots` | 7.5 GiB | 229 | 0.2 GiB |
| `vm-state` | 12 dirs | 0 orphan dirs | retention already removes them |

"Unregistered" means no VM in `ownerships.json`. Whether the control
database still holds an account for some of them [?]: that read needs
PostgreSQL and I did not run it. The collector keeps a dead computer's
latest root and so all its chunks forever [C `artifact_gc.go`, F3 unlanded].
At roughly 0.17 GiB per created computer, the dead share grows with every
test account.

The 13 registered computers by class, from `ownerships.json` metadata only
(allocated GiB, creation date, state) [M]:

- Never delete: paper (44.8), d@e (25.9, our test computer, but not for me
  to touch), owner (3.7), b@c (19.2, hibernated), `7cc96dc6` real user
  (3.1, stopped).
- Unclassified from outside, **33.7 GiB running**: `e80d229b` 7.9,
  `848b21d5` 10.2, `4d38420c` 7.4, `2bf3cab1` 8.2, all created 2026-11-03,
  state `active`, last activity at the deploy refresh. Plus hibernated
  `a875023a` 4.6 (last active 11-04, becomes idle-eligible at 168 h, about
  16:54Z today), `f1daccdb` 0.5, `4a297462` 0.5.
- The test-idle rule can never take an `active` computer [C
  `retention_test_class.go:176-221`]. If these four are test-class, they are
  protected by the deploy refresh that keeps waking them. Class [?].

### Q6. Where did 23 GiB go between 00:44Z and 11:40Z

Free fell 72.3 to 51.0 GiB between the 01:00Z and 11:00Z ledger readings
(-21.3), and 49 to 52 GiB at 11:40Z [M]. Used rose 21.6 GiB. Breakdown from
the ledger, 01:00Z to 11:00Z [M unless marked]:

| Item | GiB |
| --- | --- |
| `/nix/store` (ten deploys' closures; GC freed 0.9 at 08:31Z) | +9.7 |
| Data images: paper +1.5, d@e +1.0, four idle +0.7, other +0.1 | +3.3 |
| Tape segments (paper +1.35) | +1.7 |
| PostgreSQL (receipts +0.43, graph +0.17) | +0.6 |
| Not itemized by the ledger (chunks, capture bodies, transient files) | about +6 |

The paper wrote 18.7 GiB of new file-cas chunks between 04Z and 07Z [M
mtimes]; the collector deleted 5.85 GiB the same day. That net +12.9 GiB is
apparent bytes; on disk btrfs compresses them, which explains why the
ledger's un-itemized residual (+6) is smaller [E]. The 18.7 GiB burst begins
with the last deploy (04:31Z) and the paper's planned refresh restart at
07:15Z [M]; my hypothesis is a fresh state snapshot chunked after the
store's compact copy [E, p=0.6]. The earlier burst, 2.6 GiB at 01Z on 11-10,
fits the same pattern.

### Q7. The 60 GiB floor

See section 9.

## 5. What was tried and why it did not hold

| Attempt | Date | Why it did not hold |
| --- | --- | --- |
| Hand delete of probe mounts, GC roots, pre-compact images | 10-09 | One-time; no bound on any surface |
| Artifact collector made active | 10-20 | Covers 6 of 12 namespaces; no dead-computer rule |
| Early compaction of the paper, then a daily 04:00 sweep | 10-22 | The host compactor and the sweep were deleted with Dolt on 10-30 |
| Nix retention enforce, 24/40 band, deploy floor 90 to 60 | 10-22 to 10-23 | Works, but it only covers the nix store, 10 to 14 GiB of the swing |
| vmctl test-class retention | 10-22 | Works; has nothing left to remove because only active computers remain |
| Discard (RES-16) | 10-22 | Works for steady state; does not bound the in-guest compaction peak |
| Packing loose tape files into segments | 11-01 | Freed metadata inside btrfs but not back to the pool; added 14.6 GiB of segments |
| Growth meter | 11-01 | Measures; nothing reads it or alarms |
| Host disk alarm at 60 GiB | 10-22 | Same number as the refusal; no lead time |

## 6. Rules for the plan

- Contract classes apply: green docs; yellow tests, alarms, scripts; orange
  runtime; red tape and evidence, deploy routing; black irreversible
  destruction in production. Anything touching the paper, or irreversible
  deletion, waits for the owner.
- No switches: each mechanism ships on. A correctness refusal is not a
  switch.
- Prove on staging; local tests prove nothing for vmctl or live computers.
- Each mechanism names its bound in this table (section 8).

## 7. Ranked plan

### 7a. One-time releases the owner could approve today

Estimated free space after each (cumulative, from 52 GiB). Paths are exact
where I could read them. Nothing here touches the paper's, the owner's,
b@c's or the real user's data.

| # | Release | Size | Class | Owner? | Command and proof | Rollback |
| --- | --- | --- | --- | --- | --- | --- |
| A1 | **btrfs metadata balance**, returns over-allocated metadata chunks to the pool. Today 25.14 GiB allocated, 3.33 used [M] | +36 to 40 GiB [E] (free 52 to about 90) | orange | Go advised: not destructive, but online I/O on the host the paper serves from | `btrfs balance start -musage=30,limit=12 /` off-peak, run once, repeat until `Metadata ... Size` is about 6 GiB. Proof: `btrfs filesystem usage /` unallocated rises from 20 GiB to about 55 to 60 GiB and `df` free by the same; alarm line shows metadata % of size near 50% | `btrfs balance cancel /`. Metadata grows back by itself if needed. The 2026-10-16 exhaustion needed 84% use; now 13% |
| A2 | Chunks of the 199 unregistered computers, `sha256/file-cas-chunks/computer-*` | 26.9 GiB [M apparent] | black (one-time delete); red mechanism | **Owner** approves a reviewed list | First a read-only join of the 199 ids against the control database (accounts) and tape heads, written to a report; delete only ids with no account, no event after their creation, not owner-class. Proof: `du` of the dirs before and after, collector log | None after delete. Gate is the report. Test and fleet ids need no owner yes per the retention record F2/F3 gate; any id that ever held an owner account does |
| A3 | Swap file 16 GiB to 4 GiB (`/swap/swapfile`, `nix/disks.nix:35`; 0.22 GiB in use, 33 GiB RAM, MemAvailable 16 GiB) [M] | +12 GiB | orange | **Owner** (host memory safety) | `swapoff`, recreate with `btrfs filesystem mkswapfile --size 4G`, `swapon`; proof `swapon --show`. Do it only while the growth-meter memory ledger shows no pressure for 24 h | recreate at 16 GiB |
| A4 | Archive `taperepack-work/delete-1793531263412172392.jsonl` (1.77 GiB) and the `*.err` files (about 70 MiB) to the archive restic repository, verify, delete local | +1.8 GiB | red (audit evidence) | **Owner** | `off-host-backup.sh archive` then `restic check --read-data` (the script does this); delete local only after | restore from the archive repo |
| A5 | Four running `active` computers created 2026-11-03: `vm-ab6bcd58…`, `vm-209a583c…`, `vm-04a4e430…`, `vm-774a03f8…` | up to 33.7 GiB [M] | orange to hibernate; black to delete | **Owner**: classify first | The owner or an operator with the internal token reads `retention-plan` and `list`. If test-class: hibernate them (stops writes, allocation stays) and let the 168 h rule remove them; do not hand-delete | wake via vmctl |
| A6 | `/root/go/pkg` (3.4 GiB mod cache), `/root/.cache/ms-playwright` (0.6), `/root/.npm` (0.4), `/root/cutover` (0.5) | about 4 to 5 GiB | yellow if regenerable | owner glance | `go clean -modcache` on the host; check nothing builds there first | re-download |
| A7 | `capture-bodies` raw blobs older than N days (F7). Raw share is unmeasured [?] | up to 19.5 GiB | red | **Owner** (evidence) | Needs a size census and the access-log check from record F7 first | none; refetch is not the same bytes |

Totals if A1 to A4 are done: about +77 GiB, free about 129 GiB [E]. A1 alone
clears the floor.

Not recommended: deleting b@c's image (19.2 GiB; owner rule, never), touching
the owner's file chunks, the 5 retained guest releases, or any
sealed segment of a live computer.

### 7b. Mechanisms that keep the disk from refilling

Ranked by value per effort. Sizes are per day.

**B1. Give metadata chunks back, always (orange, small).** Change
`go-choir-btrfs-headroom` (`nix/node-b.nix:2086-2105`) to also run
`btrfs balance start -musage=40,limit=10 /` when metadata size exceeds three
times its used bytes. Releases: prevents the 38 GiB from being stranded again.
Proof: alarm line shows metadata % near 40 to 60 after the next run. Rollback:
revert. Effort: 1 hour. Add a test next to
`scripts/test-node-b-host-disk-alarm.sh`.

**B2. A forecast alarm with named growers (yellow, small).** Extend
`scripts/node-b-host-disk-alarm` (or `node-b-growth-meter --report`) to:
read the last 7 days of the ledger; compute net free-space slope with
transients removed (use the daily minimum); warn at "days to floor under
7", error under 3; print the top five 24-hour growers by ledger key. Today
the same number triggers warning and refusal. Proof: replay the 2026-11-01 to
11-11 ledger in a test and expect a warning by 11-06 (when avail was 147 GiB
and the slope since 11-01 was already -16 GiB a day). Rollback: revert. Effort:
half a day. Releases nothing; ends surprises.

**B3. Dead-computer reaper for artifact stores (red, medium).** In
`internal/platform/artifact_gc.go` add the F2/F3 rule: a computer with no
vmctl ownership, no control-database account, no tape event for 7 days is
collected (chunks, roots, tape-seg, per the tape sweep's existing destroyed
computer path), with a dry-run report first. Releases: 26.9 GiB now (item A2),
about 0.17 GiB per dead test computer after. Proof: collector log lines with
the computer ids and bytes; `du` falls. Rollback: revert; deleted data does
not come back, so the dry-run report is the gate. Effort: 2 to 3 days with
review (two seats).

**B4. Cap the file-CAS root history (orange, small to medium).** A root every
15 minutes kept for 7 days is 672 versions per computer. Thin roots: keep all
for 24 h, hourly to 7 days, the latest forever. This is F5 with a thinning
rule instead of a shorter window. Releases: the paper holds 21.5 GiB of
chunks, 18.7 written today; the share referenced only by old roots is [?]
and must be measured first (list roots older than 24 h and sum chunks only
they name). Class: orange. Owner to confirm that file history older than a
day is not a product promise. Effort: 1 day after measurement.

**B5. `capture-bodies` retention (red, owner).** 19.5 GiB, now +2 GiB a
day. Rule F7: retire raw blobs not cited by a publication after N days
(propose 30), keep block JSON and text. Proof: census before and after;
reader access log shows no fetch of retired hashes. Effort: 2 days. Rollback
none. Decide N soon: at 2 GiB a day this is the fastest-rising store.

**B6. Host gate on the in-guest compact copy (red, medium).** The copy peaks
at +19 to +35 GiB on the host and only checks guest free space [C]. Let
vmctl tell the guest whether the host can absorb the peak (a host free-space
check before `PrepareCompactCopy`, refuse and retry later), and stage it away
from deploy windows. This turns the worst transient into a bounded event.
Proof: staging run with host free forced low shows the refusal, no copy.
Files: `internal/store/compact.go`, vmctl host admission, a host-to-guest
signal. Effort: 2 to 3 days.

**B7. Tape tiering, not deletion (red, design first, owner).** Tape is
+3.9 GiB a day for all computers and has no sink. Proposal: sealed segments
older than the guest's last verified snapshot plus a margin (for example
14 days) are written to the archive restic repository, verified by
`restic check --read-data`, then dropped locally, with a lazy read path for
the genesis rebuild and audit that fetches a segment by name from the
archive. Releases: of the paper's 66 GiB, about 31 GiB would be cold (14 days
hot is about 35 GiB at 2.5 a day), and across all computers about 28 GiB of
83 [E]; after that the local tape stops growing. This is the only
item that bounds the largest legitimate grower. Open: the archive
repository's quota and size [?]; the read path does not exist; a design
record is first. Effort: 1 to 2 weeks.

**B8. Receipts table (red, design).** `computer_event_append_receipts`
11.7M rows, 25.8 GiB, +1.0 GiB a day, 98% of its rows projection batches
[M]. Extend the receipt compaction (active on three computers) to all, and
fold receipts older than the last checkpoint head into one row per segment.
Rollback is the existing "expand" setting. Effort: several days; the table
is the chain of acceptance, so two seats.

**B9. Hibernate idle test computers and cap their image (orange, small).**
The four 2026-11-03 computers show an `active` computer is never reaped. Add
to vmctl: a test-class computer idle for 24 h is hibernated, not left active;
the 168 h rule then applies. Cap test-class images at 16 GiB. Files:
`internal/vmctl/retention_test_class.go`, warmness policy. Proof: staging
plan shows them moving to `hibernated` and later removed.

**B10. Hygiene (green).** Delete the dead `VMCTL_STORE_COMPACT_*` env
(`nix/node-b.nix:1958-1964`) and the stale statements in the Oct-22 records;
write the per-surface bound table (section 8) into `docs/ACTIVE.md`.

**B11. Put a number on a bigger disk (owner).** Section 1, point 4: if B7
and B8 are not built, tape and receipts alone consume the current headroom
in weeks. A larger volume or a second device for `platform-artifacts` is the
fallback. Cost unknown [?].

Suggested order: A1, B1, B2 (a day); then A2 with B3 and B9; then B5; then
B4 and B6; then the B7 and B8 designs.

### 7c. What should never be deleted

- Every sealed or live tape segment of a registered computer, and its index
  files: the paper (66 GiB), the owner (5.6), d@e (2.9), b@c, the real user.
  The tape is canonical (ontology line 171).
- b@c's whole VM directory (19.2 GiB, `candidate-fleet-bf04bbc7…`, hibernated).
  Owner rule: never deleted.
- The paper's `data.img`, the owner's, the real user's (`vm-48bc0981…`, 3.1
  GiB, stopped), and their latest file roots and chunks.
- `platform-artifacts/signing-key`, `key-escrow-key`, `secrets`, `auth`,
  `auth-signing`, `mail`, `files`, `codex-auth*`, `unit-env`, deploy
  receipts, `vm-state/ownerships.json` and its backups, `vm-state/receipts`.
- `tape-sweep-*.manifest.jsonl` and `tape-sweep-archive` (a 7-day floor
  applies, and an apply names the manifest hash).
- The taperepack delete manifest until it is archived and verified (A4).
- The 5 retained guest releases and the ring-walk baseline.
- The last verified off-host backup snapshots.
- File chunks and roots of any computer that ever held an owner account,
  and `capture-bodies` not yet covered by an owner ruling (source evidence).

## 8. Per-surface bound table (the missing O10 table)

| Surface | Bound proposed | Mechanism | Today |
| --- | --- | --- | --- |
| nix store | 40 GiB | nix-gc (exists) | met |
| guest releases | 15 GiB | retention (exists) | met |
| file-cas roots | 10 GiB | F4 (exists) | met |
| file-cas chunks | live computers' latest + 24 h of history, no dead | B3, B4 | 54 GiB, unbounded for dead |
| capture-bodies | 30 days of raw | B5 | 19.5 GiB, none |
| data images | host admission + B6 + B9 | partly | transient +33 GiB |
| tape (all) | hot 14 days local | B7 | 83 GiB, none |
| PG receipts | one row per segment past checkpoint | B8 | 25.8 GiB, +1 a day |
| btrfs metadata | size at most 3x used | B1 | 7.5x |
| host free | alarm 7 days before floor | B2 | at floor |

## 9. The floor (Q7)

The floor is a sum: `60 >= 24 min-free + 8 deploy + 14 guest store compaction
peak + 10 paper regrowth + 4 margin` (`scripts/node-b-deploy-disk-preflight:4-12`).
Measured terms today:

| Term | Assumed | Measured |
| --- | --- | --- |
| nix `min-free` | 24 | 24 [M `/etc/nix/nix.conf`] |
| a deploy | 8 | nix store +9.7 across ten deploys (about 1 each net); a single peak not measured [?] |
| host store compaction peak | 14 | the host compactor is deleted. The in-guest copy peaked at +19, +21 and +33 GiB on the host (11-01, 11-02, 11-09) [M] |
| paper regrowth | 10 | +2.6 a day [M] |
| nightly off-host backup staging | 0 | pg dumps 19.3 GB written 02:42Z (a dip of about 4 to 6 GiB in alarm samples) [M] |
| unprotected-start rule | 0 | a start of an ordinary user's computer is refused unless free is at least 32 GiB plus the image's unallocated room (a 32 GiB image holding 8 GiB needs 56 GiB) [C `host_admission.go:672`]; none logged in 30 h [M] |

Conclusion: the floor is too high for a normal day (the 14 and the 10 never
add) and too low for the worst transient (24 + 9 + 33 + 5 = 71). Both are
true. A lower floor would hide the problem and would not survive a compact
copy on the paper; a higher one cannot be met by this disk. So: keep 60 for
now; do not lower it; make the preflight read the paper's current image
headroom instead of a constant once B6 bounds the peak; and warn at
floor + 30 (B2). The unprotected-start rule is a second effect of low
free space: at 52 GiB, waking an ordinary user's computer can be refused.
I found no occurrence in the 30 hours of journal.

## 10. Calibration (for the ledger)

Claims to enter before acting, with a test and settle time:

1. A1 returns at least 30 GiB of unallocated space. p=0.80. Test: `btrfs
   filesystem usage /` before and after one balance pass. Settle: same day.
2. After A1 and A2, free space stays above 60 GiB for at least seven days
   with no further action. p=0.10. Test: the alarm and ledger. Settle:
   2026-11-19. (My rate estimate says about four days.)
3. The four 2026-11-03 computers are test-class. p=0.6. Test: `retention-plan`.
4. The 18.7 GiB chunk burst on 11-11 is a state snapshot after the compact
   copy. p=0.6. Test: collector and store logs for the snapshot at 04Z to
   07Z.

## 11. Not determined

- The guest's own used bytes inside the paper's image (needs the internal
  token; settles Q3's reclaimable share).
- Retention-plan classes of the 13 computers (token). Control-database
  accounts for the 199 unregistered chunk directories.
- What the paper's tape holds by bytes, rather than by record count (needs
  a segment census, which reads record metadata only; I did not run one).
- Why four idle computers wrote 3.4 GiB each in 30 hours.
- What made the 18.7 GiB of chunks on 11-11 (hypothesis only).
- The share of `capture-bodies` that is raw, and who reads it.
- The size and quota of the archive restic repository.
- The footprint of a single deploy at its peak (only the ten-deploy total
  is known).
- Why the Oct-30 attempts record has "after" larger than "before" (the code
  that wrote it is deleted).

## 12. Method

Host reads only: `df`, `btrfs filesystem usage`, `du`, `find -printf` (name,
size, mtime), `journalctl`, `systemctl list-timers`, `nix path-info`, the
non-secret growth ledger (`growth-meter/2026-11.jsonl`), `ownerships.json`
(ids, states, dates only), `artifact-gc-log` (counts and sizes), `fc-config.json`.
No content of any tape segment, capture body, chunk or computer was read; no
secret, environment or credential file was opened. Heavy scans ran under
`nice` and `ionice`. Code facts came from reading main at `1abd0ebab`; file
and line references are above.
