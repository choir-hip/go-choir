# Guest data image never trims: host allocation tracks every block ever written (2026-10-20)

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
