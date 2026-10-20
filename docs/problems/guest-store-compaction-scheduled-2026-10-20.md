# Guest store compaction on a schedule, as a planned restart (2026-10-20)

Status: plan, before code. Owner ruling: owner notes §26 ("Yes schedule
compaction"). Prior record: docs/problems/paper-disk-full-again-history-
growth-2026-10-19.md (the manual compaction, 52.0 GB to 8.06 GB, root hash
identical, and its fstrim).

## Problem

The paper's guest Dolt store grows about 6 GB a day, mostly history
(13,700 commits in about three weeks). Without a bound it filled its disk
twice on 2026-10-19. The manual remedy works: hold, stop, reflink backup,
bounded `projection-compact`, fstrim, serve, unhold, refresh. But it is a
crash restart: every open obligation is closed "interrupted by a restart",
and it needs an operator.

## Design

Run the compaction inside vmctl's existing planned-restart path, so the
guest's work resumes after it, and let a daily timer pick computers by the
host storage their data image holds.

1. **vmmanager:** `VMConfig.OfflineMaintenance func(dataImagePath string)
   error`, run by `RefreshVMWithConfig` after the Firecracker process is
   killed and before the boot, under the per-VM operation lock. It is never
   persisted into the instance config. If it returns an error the boot
   still happens: the hook must leave a bootable image (below).
2. **vmctl compaction endpoint** `POST /internal/vmctl/compact-store
   {user_id, desktop_id}`: the deploy-refresh flow with the hook attached.
   It inherits deploy refresh's guards unchanged: one refresh per computer
   at a time, skip held, not active, or divergent computers, the guest's
   planned-restart mark with `allow_busy` only for always-on classes, and
   the guest's own refusal during a self-development apply or while an
   engineering capsule is live (CLAUDE.md restart rules).
3. **The hook (host side):** reflink-copy `data.img` to
   `data.img.pre-compact`; loop-mount the image; run `projection-compact`
   under `systemd-run --wait` with `MemoryMax=10G` and `MemorySwapMax=0`;
   `fstrim` the mount; unmount. On any failure: unmount, put the backup back
   in place of `data.img`, and return the error (the computer boots its
   original store). On success: remove the backup (the tool refuses unless
   the Dolt root hash is identical before and after).
4. **Sweep** `POST /internal/vmctl/compact-sweep {min_file_bytes, dry_run}`:
   for each active, unheld, non-test computer whose data image allocates at
   least `min_file_bytes` on the host (`DataImageStats.FileBytes`; the guest
   never trims, so this only shrinks after a compaction), compact one at a
   time. A busy refusal is reported as deferred and is retried the next day.
5. **Timer** in `nix/node-b.nix`: daily at 04:00 UTC, threshold 32 GiB.
   The paper reaches it in about three to four days; the owner computer
   (3 GB) never does.

## Failure modes, written before the code

1. The hook runs while the VM still writes the image: it runs only after
   `killFirecrackerProcess`, under `lockVMOperation(vmID)`.
2. The hook fails half way (mount, tool, trim): the backup is restored
   before the boot; a failed restore fails the refresh loudly, and the
   computer is not booted on a partial image.
3. The compaction outlasts the guest's 15 min planned-restart marker
   window (`internal/agentcore/planned_restart.go:27`): the boot is then a
   crash boot, which closes open work. Correct per the restart rule, only
   less gentle. The 32 GiB threshold keeps runs well under it (11 min at
   52 GB tonight).
4. Host memory: the tool runs capped at 10 GB with no swap; a cap overrun
   fails the run (failure mode 2) instead of thrashing Node B.
5. Two compactions or a compaction and a deploy refresh at once: deploy
   refresh's per-computer in-flight guard covers both; the sweep runs
   computers one at a time.
6. Disk space for the backup: the reflink copy costs nothing at first; it
   pins the old blocks only until it is removed after success.
7. Wrong computer kind: test computers and non-active ones are skipped;
   a held computer is skipped.
8. Hook persisted into the instance config and run on a later ordinary
   refresh: it is taken from the overrides only, never stored.

## Ceremony

- Mutation class: **red** (vmctl lifecycle, VM restart routing).
- Protected surfaces: vmctl refresh path, the guest's persistent data image.
  Not touched: the tape, Texture canonical writes, auth, checkpoints.
- Admissible evidence: unit tests of the hook ordering and failure restore
  in vmmanager and vmctl; then on staging the endpoint run by hand on the
  paper (planned restart consumed, store compacted, root hash identical,
  runs resume), then the timer's first run.
- Rollback: revert the commits; the timer and endpoints go away, the manual
  procedure remains.
- Heresy delta: discovered: none new; introduced: none intended (no
  boot-time resumer; the planned-restart marker is still consumed once);
  repaired: an operator-only crash-restart procedure becomes a planned one.
- Conjecture: a store compacted every few days stays under about 30 GB,
  and the compaction fits in the planned-restart window.

## Staging finding: the sweep sees no data image stats (01:03 UTC)

Deployed d8ea02b07 (CI 39349824902). A dry-run sweep at a 1 GiB threshold
skipped both active computers, the paper included, as "no data image
stats". `listOwnershipResponses` does not fill `DataImage`; only the
single-computer lookup does (`internal/vmctl/handlers.go:471-473`). So the
04:00 sweep would never select anything. Fix: the sweep reads
`DataImageStatsForVM` for each active interactive computer itself.

## Staging finding: the sweep would squash a second computer (01:20 UTC)

Deployed 15b471e44 (CI 39351328741). A dry-run sweep at 1 GiB now selects
the paper (`vm-1669…`, critical_protected, 12.6 GB on the host), so the
stats fix works. It also selects `candidate-fleet-e15c…`, an account's
premium_always_on computer created 2026-07-17, at 42.75 GB host
allocation. That is over the timer's 32 GiB threshold, so the 04:00 UTC
run would squash that computer's Dolt history to two commits.

The plan assumed only the paper would qualify. It measured the wrong
thing: `FileBytes` is the host's allocation for the image, and an image
the guest never trims keeps every block it ever wrote, so a computer with
a small store can still sit far above the threshold. History squashing was
ruled for the paper's growth (owner notes §26), not for an account's own
computer.

Fix: the sweep selects only the computer-level platform classes
(critical_protected, public_platform), the same set that may take a planned
restart while busy (`deployRefreshAllowsBusy`). Other computers are listed
as skipped with the reason "not a platform class". Compacting an account's
computer stays a by-hand `compact-store` call, made only when the owner asks.
