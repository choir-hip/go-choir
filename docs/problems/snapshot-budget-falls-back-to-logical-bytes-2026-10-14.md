# Snapshot Budget Still Counts 8 GiB per Snapshot on Node B

Date: 2026-10-14. Found while writing the VM density report
([vm-density-limits](../reports/vm-density-limits-2026-10-14.md)).
Status: documented; no fix yet. Mutation class of any fix: red (vmctl unit
capabilities or the snapshot path).

## Evidence (read-only, Node B, 2026-10-14 ~15:00 UTC)

- vmctl journal, 13:33:54 UTC, after `a110fe782` (the real-bytes budget)
  was deployed:
  `hibernated VM vm-48bc… by snapshot (… bytes=8590024247
  disk_bytes=8590024704 disk_bytes_method=st_blocks …)`.
  The btrfs extent measure was not used. The snapshot was counted at its
  full 8 GiB, not at the ~560 MB it really uses on disk (compsize,
  [artifact store problem](artifact-store-grows-12gb-a-day-2026-10-14.md)).
- `systemctl show go-choir-vmctl -p CapabilityBoundingSet` reads
  `cap_net_admin cap_sys_ptrace`. `BTRFS_IOC_TREE_SEARCH` needs
  `CAP_SYS_ADMIN`. The comment at
  `internal/vmmanager/snapshot_disk_linux.go:24-25` says vmctl "has
  [CAP_SYS_ADMIN] as root"; under this unit it does not.
- `measureFileDiskBytes` swallows the ioctl error and falls back to
  `st_blocks*512` (`snapshot_disk_linux.go:46-50`) without logging the error.
  The fallback was meant to overcount in the safe direction. It does, but
  on Node B it is used every time.

Hypothesis (very likely, not yet confirmed by an errno): the ioctl fails
with `EPERM` because of the capability bounding set.

## Effect

`VM_SNAPSHOT_BUDGET_GIB=48` admits about 5 committed snapshots (48 / 8),
not the ~70 the S3 follow-up claimed. Hibernates beyond that evict the
oldest snapshot, so those computers cold-boot (a crash boot) at their next
wake. The S3 follow-up's conjecture is falsified by its own stated test
("`disk_budget` kill-path hibernates with fewer than ~60 committed
snapshots") once a sixth computer hibernates.

## Fix directions (to design)

1. Log the errno on fallback, so the method field is explained.
2. Either grant the measure its capability (add `CAP_SYS_ADMIN` to vmctl's
   bounding set: widens a root daemon, needs the red ceremony), or measure
   in a small helper that has it, or
3. make snapshots sparse (only resident pages written, see the density
   report's lever L3). Then `st_blocks` is close to the real size without
   any privilege. This is the preferred direction because it also removes
   the 8 GiB write that drives the pressure-reclaim cascade
   ([pressure reclaim problem](pressure-reclaim-wedged-a-user-computer-2026-10-14.md)).
