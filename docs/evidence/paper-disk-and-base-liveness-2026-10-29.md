# Paper data disk fill and projection-base liveness (2026-10-29)

Read-only investigation on Node B, 2026-10-29 ~22:00 UTC. [V] verified by
command, [I] inferred. Nothing on the host was changed. Context:
`node-b-storage-inventory-2026-10-29.md`, `docs/problems/paper-never-checkpointed-2026-10-27.md`.

## A. The Autopaper data disk (computer-1355b83bced1613eb048ee28d66688a8)

### A0. Urgent: the paper's persistent fs is read-only and the runtime store will not open

- [V] `console.log` 2026-10-29T19:42:34Z: Firecracker "Failed to execute Out
  virtio block request ... No space left on device" (host ENOSPC on the sparse
  write). Guest then logs `I/O error, dev vdb`, `EXT4-fs error (device vdb):
  Detected aborted journal`, `EXT4-fs (vdb): Remounting filesystem read-only`.
  Superblock says "clean with errors", last write 19:56:09Z.
- [V] At 22:01:48Z (last console lines) the autoputer runtime restarts and fails
  `runtime store: open unified Dolt workspace ... openat LOCK: read-only file system`
  then "starting server" anyway. The paper is up as a process but cannot write.
- [I] The host was full at 19:42 (host `/` is now 386G/476G, 82G free, after the
  cleanup in the storage audit). The guest fs itself had ~72 GiB free; the
  fault is the host sparse file running out of backing blocks, not the guest.
  The guest needs a restart (fs remount + journal recovery, e2fsck advisable
  given "clean with errors" and `needs_recovery`); not done here.

### A1. Size and sources

- [V] `btrfs filesystem du` of data.img: 58.76 GB exclusive (54.7 GiB), logical 137.4 GB sparse.
  At last boot (10-28 21:57) allocated was 40.2 GB (boot-timeline
  `data_image_allocated_bytes`); guest then reported 135.0 GB total, 92.0 GB free.
- [V] Guest ext4 superblock (debugfs -c, stale but journal abort was late): 33,554,432 blocks
  of 4 KiB (128 GiB), 19,062,888 free, so 55.3 GiB used. This matches the host's
  54.7 GiB exclusive within ~1%, so the host holds almost no freed-but-unreleased blocks
  now [I].
- [V] Directory sums (debugfs `ls -l` walk, apparent sizes; slightly stale, read-only):

| path under /mnt/persistent | size |
|---|---|
| state.texture/texture/.dolt/noms/oldgen (179 files) | 51.02 GiB |
| state.texture/texture/.dolt/noms top (journal `vvvv...` 0.83 GiB, idx, manifests) | 0.84 GiB |
| state-actor.db (+wal 11 MB) | 0.61 GiB |
| capsule-artifacts (4,848 files) | 0.18 GiB |
| choir-updater | 0.15 GiB |
| files (2,611 files) | 0.11 GiB |
| .dolt/stats | ~2 KB, temptf/tmp/capsule-state/image-blobs-like dirs: empty or absent |

  There is no `image-blobs` or `logs` dir at the top level. The store is 98% of use.
- [V] oldgen age histogram: 72 files / 24.7 GiB were written 10-28 04:00-04:13 (the
  compaction output; one 5.12 GiB file); everything after is steady 3-4 files per hour of
  0.2-0.35 GiB (about 0.75-1.1 GiB/h), 10-28 05:xx through 10-29 19:xx: ~26.3 GiB
  in ~39.7 h.
- [V] vmctl `store-compact-attempts.json`: last attempt 2026-10-28T04:00:00Z,
  `file_bytes_before` 64.67 GB, `file_bytes_after` 89.23 GB (the compaction
  needs headroom: the file grew 24.6 GB during it), `store_bytes_after` 26.30 GB.

### A2. What writes it

- [V] Growth since compaction is ~0.66 GiB/h = ~16 GiB/day, all as new oldgen table files
  (journal flush every ~15-20 min), not Dolt noms top-level, not capsule/file state.
- [V] Guest Dolt maintenance disposition file (`.choir-dolt-gc-disposition.json`, 10-29 19:52Z):
  `outcome: noop, mode online, used_gib 59, live_gib 51, journal_mib 662, threshold_mib 1024`
  (internal/store/dolt_maintenance.go). So the routine GC did nothing because the journal (662 MiB)
  was under the 1024 MiB threshold; `live_gib` is an on-disk measure, not a proof of live data.
- [I] Of the 51 GiB, 26 GiB was the post-compaction store and ~26 GiB is 40 h of new
  tablefiles. Whether those are genuine new chunks or mostly dead (rewritten) chunks that a
  `dolt gc` would reclaim is unknown; the first compaction cut 64.7 GB to 26.3 GB, so the
  prior is that most is garbage. A read-only dolt size check was not possible (fs is ro, store won't open).
  The writer is the agent runtime churn in the texture Dolt store, ~16 GiB/day.

### A3. Discard / trim

- [V] nix/autoputer-vm.nix has no `discard` mount option; it uses an hourly
  `choir-data-fstrim` timer (lines ~852-895), inert unless the device exposes
  `discard_max_bytes` (Firecracker >= 1.17.0 with VM_DATA_DISCARD on).
  Running VMM is Firecracker v1.17.0 (boot-timeline, vmm-build.json).
- [I] Host allocation fell from 89.2 GB (after compaction) to 40.2 GB at the next
  boot, which suggests trim does release blocks. I did not confirm VM_DATA_DISCARD
  in vmctl's environment (not visible via systemctl show). Current host-vs-guest
  gap is ~1%, so trim is not what fills the disk.

## B. Projection bases

### B1. Liveness rule (code)

`internal/platform/artifact_gc.go` `artifactGCLiveSets` plus
`internal/platform/projection_jobs.go:570` (`projectionBasePins`). A base is live if it is:
1. the `base_ref` of any `computer_replay_watermarks` row (current), or
2. in `computer_projection_base_pins` (restore points), or
3. one of each computer's **latest two** bases (hardcoded `COUNT(newer) < 2`), or
4. the `seed_base_ref` of a queued/running/retry projection job.

Everything else is unreachable and is deleted only when older than
`PLATFORM_ARTIFACT_GC_GRACE` (mtime based). There is no separate rollback/grace
class beyond these; "grace" is the age gate on unreachable ones.

### B2. Observed

- [V] Dry-run via POST `127.0.0.1:8086/internal/platform/artifact-gc` (corpusd hosts the
  platform route; 8082/8083/8084 return 404) with the internal header: projection-base
  scanned 426 entries (blob + descriptor), live 254, unreachable 172, all 172 in grace,
  eligible 0. Config in nix/node-b.nix:1302: MODE=active, GRACE=168h, INTERVAL=1h,
  MAX_BYTES=32 GiB, MAX_DELETES=50000.
- [V] 214 blob files, 63.04 GB, 96 computers (+1 group without a descriptor). By
  descriptor computer_id:

| computer (last 8) | bases | total | latest only | note |
|---|---|---|---|---|
| ...d66688a8 (paper) | 1 | 26.46 GB | 26.46 GB | seq 4526210, 10-28 09:27 |
| ...6879f9e6 | 2 | 17.53 GB | 8.79 GB | 8.75 GB seq 574653 (10-08 22:06), 8.79 GB seq 577162 (10-09 02:43); 21 days old |
| ...aa60e24d | 21 | 10.91 GB | 1.17 GB | five >1 GB: seq 68784 1.03, 71298 1.08, 73808 1.13, 74432 1.14 (10-27), 75734 1.17 (10-28 15:17) |
| ...57164e72 | 80 | 4.45 GB | ~0 | tiny bases |
| 93 others | ~110 | ~3.7 GB | | <0.5 GB each |

  Bases over 1 GB: 7 files (48.5 GB) plus the 26.5 GB paper base is the biggest
  single one.
- [V] Totals by rule: all 63.04 GB; latest two per computer 49.89 GB; latest one per
  computer 38.96 GB. Bases beyond the latest two: 98 files, 13.15 GB, ages 0.1-7.0 days,
  all inside the 168 h grace.

### B3. What a tighter rule frees

- [V/I] Already scheduled: the 13.15 GB beyond-latest-two frees itself as each file
  passes 7 days (all within the next 7 days), with no change.
- [I] "Latest per computer plus pins": frees a further ~10.9 GB (63.04 - 38.96 = 24.1 GB
  minus the 13.15 GB already unreachable), chiefly the older 8.75 GB 6879f9e6 base and
  ~1.1 GB of aa60e24d, assuming no pin or watermark on them (pins are in the DB; I did
  not query it). Retaining the paper's single 26.5 GB base is correct and not freeable.
  Computer 6879f9e6 appears dormant (bases frozen since 10-09); if it is a test
  computer, deleting the computer, not changing the rule, frees 17.5 GB.
- Config without code: only `PLATFORM_ARTIFACT_GC_GRACE` (e.g. 24h) would release
  the 13.15 GB beyond-latest-two bases sooner; but grace is a floor in
  `clampArtifactGCRequest`, shared across namespaces (file-cas, platform-update),
  so lowering it affects them. The keep-count of 2 is hardcoded in
  `projectionBasePins`; going to "latest 1" requires a code change (`<2` to `<1`) and
  loses the fallback base. MAX_BYTES (32 GiB) per run is not binding.
