# Runbook: Offline Guest Dolt GC (Node B, break-glass)

Use this runbook when the guest's GC record (`/health` → `dolt_gc`, the same
content as `.choir-dolt-gc-disposition.json`) reports `compaction_needed: true`,
or `deferred_online` and the online GC cannot finish. Run it with the VM
stopped to reclaim Dolt history or a huge journal from the host. Proven
2026-09-03 (9.8 GB → 1.7 GB), 2026-09-11 (18.6 GB journal → 2.1 GB store,
heads/rows/branch history intact) and 2026-10-03.

Since 2026-10-15 the in-guest GC has no store-size guard. The old 5 GiB guard
measured the old generation, which every default GC grows, so once history
passed 5 GiB it switched GC off for good; `skipped_size` no longer exists.
The GC now works like this:
- an online GC runs on the open store every 5 minutes;
- the boot GC runs only on journals up to 4 GiB;
- either runs once the journal reaches min(1 GiB, free/4)
  (`RUNTIME_DOLT_GC_JOURNAL_GIB`).

That bounds the journal. A default GC never reclaims history or live data.
When free space stays under 25% after a GC, the record says
`compaction_needed`: squash plus `dolt gc --full` with `projection-compact`,
which is this runbook's offline path.

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
Do not leave mounts under `/mnt/guest*`. Note: mounts made inside a guest's
mount namespace (e.g. via a running firecracker process's `/tmp`) are
unreachable from the host and pin deleted files until that VM exits —
prefer mounting helper images from the init namespace only.
