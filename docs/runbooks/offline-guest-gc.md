# Runbook: Offline Guest Dolt GC (Node B, break-glass)

When the guest's `.choir-dolt-gc-disposition.json` reports
`outcome: skipped_size` and the store keeps growing, reclaim unreachable
chunk history from the host with the VM stopped. Proven 2026-09-03
(9.8 GB → 1.7 GB, heads/rows/branch history intact).

## Preconditions

- Computer held (`/internal/vmctl/hold`) and VM stopped. Never touch a
  running guest's block device.
- NVMe headroom ≥ 2× the store size in `/var/tmp` (backup + workspace).
- `dolt` CLI on Node B matches the guest's embedded major version
  (2026-09-03: `/nix/store/psvajwk2bdp5ddmr36ysmbz1qpgn2h3h-dolt-2.1.9/bin/dolt`).

## Procedure

1. Confirm no firecracker process for the VM slot:
   `ps -C firecracker -o args= | grep <vm-id>` must be empty.
2. Detach stale read-only loop devices for `data.img` first
   (`losetup -a`; `losetup -d`). The mount helper reuses a stale RO loop and
   mounts read-only with a "source write-protected" warning.
3. Attach explicitly and mount read-write:
   `losetup -f --show <slot>/data.img` → `/dev/loopN`;
   `mount -o rw /dev/loopN /mnt/guestlive`; `touch` probe to confirm RW.
4. Backup the live store (reversible surgery or stop):
   `cp -a /mnt/guestlive/state.texture/texture /var/tmp/texture-backup-pre-gc-<date>`.
5. Collect with default GC only (`--full` is not automated pending review;
   since 2026-09-04 Texture history resolves through the indexed parent chain
   rather than `dolt_history_*` + `AS OF`, branch history is no longer
   load-bearing for reads):
   `HOME=/tmp/guestq dolt gc` in the live texture dir.
6. Verify before unmount — all must hold:
   - `SELECT sequence, canonical_event_head FROM computer_event_projection_heads`
     matches the pre-GC head;
   - `SELECT COUNT(*) FROM computer_event_index` matches the pre-GC count;
   - `dolt log --oneline | wc -l` is nonzero (branch history preserved).
7. `sync`, `umount`, detach the loop device. Loose ends to check:
   `losetup -a` shows no attachment for the live `data.img`.
8. Boot via `/internal/vmctl/maintenance-serve` while held, confirm servable
   and no OOM loop, then unhold and boot normal. Note: a stale
   `MaintenanceHold` in manager instance config can re-fence boots after
   unhold; check `fc-config.json` boot args if the guest reports held.

## Rollback

Restore the backup over the live texture dir (VM stopped), reboot. The
backup is disposable only after the post-GC boot serves product traffic.

## Cleanup

Remove scratch copies, scripts, and `/tmp/guestq`; detach all helper loops.
Do not leave mounts under `/mnt/guest*`.
