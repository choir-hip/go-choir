# Every hibernate writes the whole 8 GiB guest, idle computers stay up 30 minutes, and every guest-artifact deploy voids every snapshot (2026-10-22)

Status: documented, not fixed. Checklist item RES-13 (P2, red) in
`docs/security-density-architecture-checklist-2026-10-21.md`; levers L3,
L4 and L20 of `docs/reports/vm-density-limits-2026-10-14.md`. Design:
`docs/design/fp10-snapshot-levers-and-host-slab-2026-10-22.md`.

This record gathers three problems the density report found on
2026-10-14 that had no problem record. No new staging reading was taken
for it; every number below cites the reading it came from.

## 1. Full snapshots (L3)

`internal/vmmanager/fcapi.go:126` asks Firecracker for
`"snapshot_type": "Full"`. Firecracker writes every guest page, resident
or not, so an 8 GiB guest writes an 8 GiB memory file.

Evidence (density report, Node B, 2026-10-14):
- a hibernate took 7.6 to 8.1 s and pushed host memory PSI to 2.5, which
  set off the reclaim cascade that day
  (`docs/problems/pressure-reclaim-wedged-a-user-computer-2026-10-14.md`);
- btrfs zstd stores the file in about 560 MB (compsize 6%), so disk is
  not the cost; the write and the page-cache churn are;
- hibernates run one at a time, about 0.1 a second safely.

The S3 design rejected Diff snapshots (design note D3) because Node B has
swap and `mincore` does not see swapped pages. vmctl's unit sets
`MemorySwapMax=0` (`nix/node-b.nix`, the vmctl unit), so a guest page in
vmctl's cgroup never swaps; whether that still holds for a Firecracker
process the jailer moves into another cgroup (FP8, SEC-04) is open.

## 2. Thirty idle minutes before hibernate (L4)

`VMCTL_IDLE_TIMEOUT=30m` and `VMCTL_PRIMARY_KEEPALIVE_MODE=under-capacity`
(`nix/node-b.nix`, vmctl environment). An idle computer holds its RAM for
30 minutes, or for as long as the host is under capacity. With full
snapshots a shorter timeout would mean more 8 GiB writes, so L4 waits on
L3, and on an honest snapshot budget (RES-14, FP8).

## 3. Deploys void every snapshot (L20)

Guest artifacts live at stable paths (`/var/lib/go-choir/guest/vmlinux`,
`initrd`, `rootfs.ext4`, `storedisk.erofs`). `validateSnapshot`
(`internal/vmmanager/snapshot.go`) refuses a snapshot whose launch config
hash or bound files changed (`launch_config_mismatch`,
`disk_identity_mismatch`). That refusal is right: the resumed guest's page
cache describes the old store image. But it means a deploy that changes
any guest artifact turns every hibernated computer's next wake into a
cold boot until `VM_SNAPSHOT_MAX_AGE` (36 h) would have retired them
anyway. Density report: cold boots run about 0.3 to 0.5 a second on
Node B against 3 to 5 snapshot wakes a second (estimates), and the
2026-10-14 11:43 `launch_config_mismatch` wedged a user.

## What is not known

- How many wakes a day fall back to a cold boot, and for which reason
  (no count since 2026-10-14).
- The resident fraction of a cold-booted guest, which sets the Diff
  file's size (the report estimates about 1.2 GiB of 8).
- Whether every Firecracker process's cgroup has `memory.swap.max` 0.

The read-only commands for these are in the design, section 4.
