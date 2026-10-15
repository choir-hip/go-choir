# Runbook: Offline Guest Dolt GC (Node B, break-glass)

Use this runbook when the guest's history compaction record (`/health` →
`dolt_compaction`, the same content as `.choir-dolt-compaction.json`) reports
`deferred_space`, `refused`, `error` or `degraded`, or when the GC record (`/health` →
`dolt_gc`, `.choir-dolt-gc-disposition.json`) reports `deferred_online` and
the online GC cannot finish. Run it with the VM
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

Since 2026-10-15 history can be bounded online too
([`guest-dolt-history-bound-2026-10-15`](../problems/guest-dolt-history-bound-2026-10-15.md)).
It is **off for the fleet**: only computers listed in
`CHOIR_DOLT_COMPACT_COMPUTERS` (guest runtime environment,
`nix/autoputer-vm.nix`) compact. Today that is the paper,
`computer-1355b83bced1613eb048ee28d66688a8`. Every other computer records
`not_enabled`. To enable or disable one, edit the list, deploy, and let the
computer boot on the new image.

On an enabled computer, after each journal GC tick, the guest compacts once
the store has grown by the history budget since its last compaction:
- never on the first tick after boot, and not before a random 0–30 minutes
  after boot (`boot_wait`);
- the budget is `RUNTIME_DOLT_HISTORY_BUDGET_MIB` (default 2,048 MiB),
  lowered to a quarter of free space, with a 512 MiB floor;
- the compaction is a squash to one commit over the root (the parentless
  commit), then `DOLT_GC('--full')`, on the open store's write connection;
- it is checked like step 6 below (the projection heads and the event index
  count, plus every table's row count), and by a full content witness on
  the read pool before and after;
- the record is `in_progress` (with `last_failure_at` set) while it runs. A
  guest killed mid-run therefore backs off 6 h after it reboots, and so does
  any failure.

A witness mismatch after the GC records `degraded`, with the changed
table(s), and logs `DOLT COMPACTION DEGRADED`. The store can no longer be
shown equal to its tape projection. Rebuild it from the tape (the
rematerialize / projection-base path) or restore the pre-compaction backup
if one exists. Do not re-enable compaction on that computer until it is
rebuilt.

**Acceptance on the paper after a deploy** (read-only, from Node B: `curl -s
http://<guest tap ip>:8085/health`, the guest IP from `fc-config.json` boot
args; counts and sizes only):
- `dolt_compaction.outcome` goes `boot_wait`, then `noop` or `ran` within
  about 35 minutes of boot;
- after a run: `outcome = ran`, `witness_match = true`,
  `commits_after <= 2`;
- `gc_ms`, `witness_ms` and `peak_rss_mib` are recorded (the write stall
  and the memory high-water mark);
- `content_root`, `heads_digest` and `event_index_count` are present;
- free space after GC is flat to within live growth over 24 h;
- no `degraded`.

It needs free space of at least store + journal + 512 MiB; with less, it
records `deferred_space`. That case, a refusal (extra branch) or an error
is what this runbook's offline path (`projection-compact`) is for.
`compaction_needed` on `dolt_gc` still means free space stayed under 25%
after a GC; with online compaction running, that is live data, not history.
Kill switches: `RUNTIME_DOLT_COMPACT_DISABLED` (compaction only) or
`RUNTIME_DOLT_GC_DISABLED` (both).

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
