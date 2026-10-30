# Guest data image never trims: host allocation tracks every block ever written (2026-10-20)

Status 2026-10-30: the Dolt-specific numbers and fix are historical. The trim question carries over to the guest `data.img` holding SQLite `state.db`. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Status: problem record, before code. Mutation class of the fix: **red**
(VM lifecycle, guest disk). Related: `guest-store-compaction-scheduled-
2026-10-20.md`, `hostauth-enforce-flip-2026-10-20.md` ("Side observation").

## Problem

A guest's `data.img` is a sparse ext4 file on the host's btrfs. The host
allocates a block when the guest first writes it and frees it only when the
file is hole-punched. The guest never trims, so every block Dolt ever wrote
stays allocated on the host after the guest frees it.

Evidence (paper, 2026-10-20): the live Dolt store is about 8 GB. After the
offline compaction and `fstrim` at 01:50 UTC the image held about 9 GB on the
host; by 04:00 UTC it held 19.1 GB. Dolt rewrites journal and table files
constantly, so about 10 GB of host allocation appeared in about 2 hours with
no growth in live data. The 04:00 sweep threshold (32 GiB of `FileBytes`)
measures this allocation, not live data, so the compaction timer is driven by
the leak it cannot prevent: the schedule is a treadmill unless the guest
trims.

## Findings

### What is pinned

`pkgs.firecracker` from the locked nixpkgs (`flake.lock`, nixpkgs
`fe132422...`, flake nixos-unstable) is **Firecracker 1.16.1** (nixpkgs
commit "firecracker: 1.15.1 -> 1.16.1", 2026-07-10). `nix/node-b.nix` sets
`VM_FIRECRACKER_BIN` and `VM_JAILER_BIN` from it.

### Firecracker discard support

- **1.16.1 (pinned) has no virtio-blk discard.** Its release notes have no
  discard entry, and `docs/api_requests/block-discard.md` does not exist at
  tag `v1.16.1` (HTTP 404).
- **1.17.0 (released 2026-09-10) adds it, opt-in.** CHANGELOG 1.17.0, Added:
  "Add opt-in virtio-blk discard support for writable `Sync` IO engine drives
  through the `discard` drive configuration field" (PR #6142). Doc:
  `docs/api_requests/block-discard.md` on main.
  - Enable: `"discard": true` in the drive config (`PUT /drives/{id}`, or the
    `--config-file` drive object). Off by default.
  - **`io_engine` must be `Sync`** (the default). `Async` (io_uring) is not
    supported with discard. Read-only drives and vhost-user are not supported.
  - Backend: a regular backing file is hole-punched; a block device gets
    `BLKDISCARD`. Our `data.img` is a regular file, so a guest trim becomes a
    host hole punch, which btrfs honors.
  - Guest: a Linux guest triggers it with `fstrim`. Alignment is the
    advertised logical block size over 512.
- Firecracker's config parser denies unknown fields (the balloon code in
  `internal/vmmanager/balloon.go` already relies on that). Sending
  `"discard": true` to 1.16.1 fails every boot of that computer. So the flag
  must not be sent until the host binary is 1.17.0 or newer.
- 1.16.2 (same day as 1.17.0) is a fixes-only release and has no discard.
- nixpkgs master still packages 1.16.1 as of this record (the bump commit
  list shows no 1.17.x yet). The flake's weekly bump (`flake-bump.yml`,
  which already reports the firecracker version) will pick up 1.17.x when
  nixpkgs does; an `overrideAttrs` pin is the alternative and needs a
  `cargoHash` that cannot be computed without building on Node B.

### Drive config today

`internal/vmmanager/manager.go` `buildFirecrackerConfig` emits the `data`
drive as `{drive_id, path_on_host, is_root_device:false, is_read_only:false}`:
no `io_engine` (so Sync), no `cache_type` (so Unsafe, no flush), no
`discard`. The config is passed with `--config-file`; the jailer path
(`confinement.go`) rewrites `path_on_host` and passes other drive fields
through. The jailed writable drive is a hard link to the same inode, so a
hole punch through the VMM shrinks the real `data.img`.

### Guest mount today

`nix/autoputer-vm.nix` declares the data disk as a microvm.nix `volumes`
entry (`mountPoint = "/mnt/persistent"`). microvm.nix generates
`fileSystems."/mnt/persistent"` as `/dev/vdX`, ext4, with no `options`. No
`discard` mount option and no fstrim unit exist anywhere in `nix/` or
`flake.nix`. The guest kernel's virtio_blk driver advertises discard to the
block layer automatically when the device offers `VIRTIO_BLK_F_DISCARD`, so
no kernel change is needed.

## Decision

1. **Firecracker lacks discard at the pinned version.** Branch (c) applies
   today: enabling the drive flag now would break boots. The lever that
   works today is unchanged: the compaction schedule threshold, whose
   loop-mount `fstrim` is the only trim the image ever gets.
2. **Land branch (b) behind a gate that is off.** Both halves are inert until
   the host runs Firecracker >= 1.17:
   - vmmanager: `VM_DATA_DISCARD` (default off) adds `"discard": true` and
     `"io_engine": "Sync"` to the `data` drive only. At config load the flag is
     honored only if `VM_FIRECRACKER_BIN --version` reports >= 1.17.0;
     otherwise it is ignored with a log line, so the flag cannot break boots
     on 1.16.x. Only the `data` drive gets it (the store drive is read-only,
     the credential disk is tiny).
   - guest: a **periodic fstrim timer** on `/mnt/persistent`, not the
     `discard` mount option (continuous discard adds latency to every Dolt
     file delete). Hourly, because the observed churn is about 10 GB per 2 h;
     daily would let the image reach tens of GB between trims. The service
     is a no-op (exit 0, one log line) when the device advertises no discard,
     so guests refreshed onto this config but still on a no-discard VMM do not
     show a failed unit.
3. The firecracker bump itself is a separate, later change (host binary,
   red). This record names it as the prerequisite and the proof below as its
   acceptance.

## Failure modes, written before the code

1. `discard` sent to a Firecracker that does not know it: every boot of the
   affected computer fails. Guard: version gate at config load; flag off by
   default; unit test that off emits no `discard`/`io_engine` keys and that
   1.16.1 banner is refused.
2. `discard` with `Async` io_engine: Firecracker refuses the drive. Guard:
   the same code path sets `Sync` explicitly; test asserts it.
3. Discard on the read-only store drive or the credential disk: refused or
   pointless. Guard: only the `data` drive; test asserts the others.
4. Launch-config hash changes for every computer once the flag turns on, so
   hibernate snapshots refuse and cold boot. Already the rule on a Firecracker
   version change (`snapshotReasonFirecracker`); accepted, and it happens on
   the same deploy as the binary bump anyway.
5. Guest trim runs during a Dolt burst and competes for IO. Guard: hourly,
   `Nice`, idle IO class; ext4 skips already-trimmed groups within a mount.
6. A trim of a large, long-untrimmed image stalls the guest for a while on the
   first run. Mitigation: the offline compaction already trims the paper; the
   first online trim runs once after boot (`OnBootSec`) rather than at a
   scheduled load peak. Measure it (below).
7. Guest has an older unit and a newer VMM, or the reverse: both are inert
   alone (no discard feature offered -> unit exits 0; feature offered but no
   trim -> same as today).
8. Hole punching defeats a reflink backup's sharing: `data.img.pre-compact`
   (reflink) pins old extents; punching the live file only drops its own
   references. No loss. The backup exists only during compaction, with the VM
   stopped.
9. Restart rules: nothing here is a boot-time resumer. Guests pick the unit up
   at their next planned refresh; the VMM flag takes effect at the next cold
   boot of each computer.

## Ceremony

- Mutation class: **red** (guest disk, VM launch config). Protected surfaces:
  vmmanager launch config (data drive), guest NixOS config. Not touched: tape,
  Texture writes, auth, checkpoint/route projection, vmctl lifecycle logic.
- Admissible evidence: vmmanager unit tests of the drive config; a Nix
  evaluation of the guest timer/service; then on staging, after the Firecracker
  bump, the measurement below.
- Rollback: revert; or unset `VM_DATA_DISCARD` (VMM side) — the guest unit then
  exits 0 on "no discard". The unit alone is harmless.
- Heresy delta: discovered: the compaction threshold measures an allocation the
  guest inflates; introduced: none; repaired: none yet (inert until the bump).

## How to prove it on staging (after Firecracker >= 1.17 and the flag)

On the paper: `firecracker --version` on Node B is >= 1.17.0; the computer
cold-boots with `VM_DATA_DISCARD=1`; in the guest `cat
/sys/block/vdb/queue/discard_max_bytes` is non-zero; `systemctl status
choir-data-fstrim.timer` is active. Then measure, hourly for 6 hours, the host
allocation of the image (`DataImageStats.FileBytes`, or `du -k` of `data.img`)
against the live store size. Pass: allocation stays within about 1.5x of the
live Dolt store (about 12 GB for 8 GB live) instead of climbing ~5 GB/hour;
the journal line `choir-data-fstrim` reports bytes trimmed each hour; store
read/write latency and Dolt commit time show no hourly regression.

### Proof on one test computer (lane/fp0-discard-test, 2026-10-22)

The lane enables Firecracker 1.17 and discard only for
computer-9648031f (vm-80b173b9, FP2's test computer; coordinate with FP2
first). Every other computer keeps the pinned binary. FP0's steps, from the
bunch 72 reviews:

0. Before the deploy, on Node B: record `sha256sum` of
   `fc-config.json` for the paper and for an unnamed test computer
   (vm-c5c801f8), and `btrfs filesystem usage -b /` plus `df -k /`. Build
   `nix/firecracker-next.nix` once on Node B ahead of the deploy (done
   2026-10-22 17:47, `/nix/store/jbzvr6c965zyby7ldj73yi0zvibgd65b-firecracker-1.17.0`).
1. Negative control: after the deploy, the two recorded `fc-config.json`
   hashes are unchanged and the unnamed computer's snapshot still resumes
   (boot kind `snapshot_resume`).
2. The named computer cold boots (its 1.16 snapshot is refused with
   `firecracker_version_mismatch`). Its boot timeline names
   `Firecracker v1.17.0`; its `fc-config.json` data drive has
   `"io_engine": "Sync"` and `"discard": true`, and no other drive does.
3. In the guest: `/sys/block/vdb/queue/discard_max_bytes` is non-zero and
   `choir-data-fstrim.timer` is active. The store opens (`/health` ok) and
   one prompt gets a model answer.
4. The first fstrim, 10 minutes after the cold boot over a never-trimmed
   image: the Sync engine does the hole punch on Firecracker's VMM thread,
   which also serves the guest's net, vsock and balloon. Sample the guest
   `/health` latency every 10 s from 2 minutes before to 5 minutes after it.
   Record the unit's exit status and bytes trimmed. A failure (EIO from an
   ENOSPC hole punch on a near-full host) must show as a failed unit, not a
   silent one; check the host's btrfs metadata headroom at the same time.
5. Host effect, measured as host free space and btrfs exclusive bytes, not
   `du` or `st_blocks` alone: reflink copies (the off-host backup, the
   compaction copy) keep shared extents, and root is `compress=zstd`, so a
   partial punch of a compressed extent frees nothing. Read
   `btrfs filesystem usage -b /` and `df -k /` before the first fstrim,
   after it, and hourly for 6 hours, against the live store size.
6. Reverse switch: remove the id from both `_ONLY` lines, deploy, and wake
   the computer. Expect `firecracker_version_mismatch` (its 1.17 snapshot
   is refused under 1.16) and a cold boot, not a resume.
Pass: steps 1 to 3 and 6 as stated; step 4 with no health failure and a
visible unit result; step 5 shows host free space recovered by the trim,
or the record says why not.

## As built (same day)

- `internal/vmmanager/data_discard.go`: `ManagerConfig.DataDiscard`
  (`VM_DATA_DISCARD`), `firecrackerSupportsBlockDiscard`, and
  `applyDataDriveDiscard` (adds `io_engine: Sync` and `discard: true` to the
  `data` drive only). `LoadConfigFromEnv` honors the flag only when
  `firecracker --version` reports >= 1.17.0, and logs when it ignores it.
  Tests: `internal/vmmanager/data_discard_test.go` (off emits nothing; on
  touches only `data`, Sync; version table; env gate incl. unreadable binary;
  jail rewrite keeps the fields).
- `nix/autoputer-vm.nix`: `choir-data-fstrim.service` and `.timer` (first run
  10 min after boot, then hourly, idle IO class, no-op when the device
  advertises no discard). Check: `scripts/check-guest-data-fstrim.sh` (Nix
  evaluation of the guest config, no build).
- `nix/node-b.nix`: a comment only. `VM_DATA_DISCARD` is not set, because the
  pinned binary is 1.16.1.
- Nothing is live until `pkgs.firecracker` is >= 1.17.0 and
  `VM_DATA_DISCARD=1` is added to vmctl's environment. Guests pick up the
  timer at their next planned refresh; the flag at each computer's next cold
  boot. A Firecracker bump also invalidates hibernate snapshots
  (`firecracker_version_mismatch`), which cold boot.
- Until then the only trim is the compaction hook's loop-mount `fstrim`; the
  32 GiB sweep threshold remains the lever, and it is driven by host
  allocation the guest inflates (about 10 GB per 2 h on the paper), so it will
  fire more often than the store's real growth warrants.

### Every computer, the paper included (owner, 2026-10-22 22:40 UTC)

Owner: "Discard for the paper is important. Let's accelerate that." and
"Anything which improves resource utilization on a forward and ongoing
basis should be prioritized over stability, availability, or uptime in the
short term." So FP0 widens both selectors to `*` in deploy D4. It does this
before the trial's six hourly readings and its reverse switch (step 6), on
the trial's results so far (D3, 85a8115b1):

- computer-9648031f cold booted on Firecracker v1.17.0 (snapshot_fallback
  firecracker_version_mismatch);
- only its data drive got `io_engine` Sync and `discard` true;
- the store opened and Texture answered;
- /health answered 93 of 93 probes at 10 s across the first trim (slowest
  5.4 ms);
- the first trim punched 1.67 GiB out of its data image;
- a capsule build on the same VMM build (assignment-b5a05cc9…, 22:14-22:30)
  ended with `oom_kill=0` and a 748 ms spawn.

What it costs:

- Every computer cold boots once at its next launch, because its 1.16.x
  snapshot is refused. Idle computers do this at their next wake. The paper
  and the other busy critical or public computers do it through the
  deploy's planned, marker-backed refresh.
- Each computer's first trim runs 10 minutes after its boot, on a data image
  that has never been trimmed. On the paper that is the largest image on
  Node B.
- The Sync engine's hole punch runs on the VMM thread that also serves the
  network, so a slow /health window during the paper's first trim is
  expected and accepted.

Monitoring: FP0 samples the paper's /health every 10 s from 2 minutes
before its first trim to 10 minutes after it, and reads host free space and
the image's allocated size before and after.

Rollback: delete the four lines and redeploy. Every computer then cold boots
again on the pinned 1.16.2 with no discard (its 1.17 snapshot is refused
too). The pinned-build change that retires firecracker-next.nix is
lane/fp0-firecracker-117-fleet.
