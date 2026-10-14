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

## Fix prepared (branch `fix/snapshot-budget-fiemap-measure`, not deployed)

### What each unprivileged measure gives on btrfs zstd

Read-only on Node B, 15:05 to 15:18 UTC, with
`scripts/snapshot_disk_measure_compare.py` (run as root over ssh, so the
exact `BTRFS_IOC_TREE_SEARCH` column is available for comparison). No
snapshot file existed on the host at the time, so these are other files
written once:

| file | st_blocks | FIEMAP bound | truth (tree search) |
|---|---|---|---|
| guest `vmlinux` (2,895 compressed extents) | 397 MB | 323 MB | 212 MB |
| guest `storedisk.erofs` (lz4 inside, 1,321 compressed extents) | 2,881 MB | 2,856 MB | 2,799 MB |
| guest `initrd` (not compressed) | 22.3 MB | 22.3 MB | 22.3 MB |

- `st_blocks` counts the uncompressed length of allocated extents: blind
  to compression.
- `SEEK_DATA`/`SEEK_HOLE` find holes only. Firecracker writes every page,
  zeros included, so for a full snapshot they give the logical size.
- FIEMAP reports each extent's logical length and physical start, and
  flags compressed ones `ENCODED`, without their compressed length. Extents
  never overlap on disk, so a compressed extent occupies at most the
  distance from its start to the next higher start among the file's own
  extents. That is an upper bound, exact where the file's extents are
  packed together. In `vmlinux`, 1,343 of 2,895 compressed extents had no
  own successor within 128 KiB (other files or free space in between), so
  the bound recovered about 40% of the gap between `st_blocks` and the
  truth.
- No btrfs ioctl reports compressed lengths without `CAP_SYS_ADMIN`
  (`TREE_SEARCH`, `TREE_SEARCH_V2` and `ENCODED_READ` all require it).

### Chosen

Measure order: `btrfs_extents` (exact, if the capability is ever present),
then **`fiemap_bound`** (no privilege, never above `st_blocks`), then
`st_blocks`. Each fallback is logged once per process and reason with the
errno, for example
`vmmanager: snapshot disk measure btrfs_extents unavailable: operation not
permitted (BTRFS_IOC_TREE_SEARCH needs CAP_SYS_ADMIN, which is outside
vmctl's capability bounding set); using fiemap_bound`. Every hibernate line
already carries `disk_bytes_method`.

No capability is granted. `CAP_SYS_ADMIN` is the only capability the
kernel accepts for the extent search, and it is too broad for vmctl, a
long-running root daemon that serves HTTP and drives guests. It would
allow, among others, btrfs subvolume deletion and other admin ioctls,
namespace and quota operations, and `bpf`/`perf` (the unit's
`SystemCallFilter` keeps `@privileged` but drops `@mount`). If exact bytes
are needed later, the narrow form is a separate one-purpose unit holding
only `CAP_SYS_ADMIN`, read-only on the state dir, with no network, answering
"disk bytes of this snapshot file" over a local socket.

### What is not known yet

How tightly a real snapshot's compressed extents are packed. A snapshot is
one fast sequential write of mostly zero pages (about 4 KiB on disk per
128 KiB extent), so it may pack better than `vmlinux`, whose copy ran beside
other deploy copies. If half its extents are not packed, the bound is about
half the logical size, which is roughly 10 snapshots in the 48 GiB budget,
not 70.

Staging check after deploy: hibernate one test computer, then compare the
hibernate line's `disk_bytes` (`fiemap_bound`) with the script's
`tree_search` on the same `snapshot/mem`. If the bound is more than twice
the truth, the next step is sparse snapshots (density report, lever L3),
which make zero pages holes so that every measure is close. The privileged
helper is the fallback after that.
