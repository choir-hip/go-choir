# Snapshot Budget Still Counts 8 GiB per Snapshot on Node B

Date: 2026-10-14. Found while writing the VM density report
([vm-density-limits](../reports/vm-density-limits-2026-10-14.md)).
Status (2026-10-21): partly fixed, unverified. The unprivileged FIEMAP
bound landed (9001e9842), and vmctl's unit gained `CAP_SYS_ADMIN` for the
jailer (10e9f18e1), which may let the exact method succeed; no staging read
records which method hibernates now log. The 2026-10-22 audit at the end
names what the budget counts, two small gaps and FP0's probe. Tracked as RES-14 in the
[checklist](../security-density-architecture-checklist-2026-10-21.md). Mutation class of any fix: red (vmctl unit
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

## Audit, 2026-10-22 (RES-14, FP10): what the budget counts against what the disk holds

Code read at lane base `51cb3a8cd`; no Node B reading was taken by this
lane (FP0 runs the probe below). Status after this audit: the count is in
the safe direction everywhere except two small, bounded gaps (G1, G2), and
which measure runs on Node B is still unread.

### What the budget adds up

- The admission check is `ensureSnapshotRoom`
  (`internal/vmmanager/snapshot.go:723-757`). It sums `entry.bytes` of every
  committed snapshot except the capturing VM's own (`:734`), adds `need`,
  evicts oldest-first until the sum fits `VM_SNAPSHOT_BUDGET_GIB`
  (48 GiB, `nix/node-b.nix:1350`), then requires
  `statfs.Bavail - need >= 16 GiB` (`:756`, floor at `:71`).
- `entry.bytes` is `meta.DiskBytes` from the snapshot's `meta.json`
  (`snapshot.go:697`), measured once, at commit, after both files are
  fsynced (`snapshot_lifecycle.go:261, 275`). Only a meta without
  `disk_bytes` (written before `a110fe782`) is re-measured at each listing
  (`snapshot.go:698-699`). The figure is never refreshed afterwards.
- `need` for the new snapshot is the logical worst case,
  `MachineMemSizeMib<<20 + 64 MiB` (`snapshot_lifecycle.go:140`, overhead
  at `:31`), because its compressed size is unknown until written.
- `dirDiskBytes` (`snapshot_disk.go:79-104`) sums every regular file of
  `snapshot/` (`mem`, `vmstate`, `meta.json`) through
  `measureFileDiskBytes` (`snapshot_disk_linux.go:29-67`), which tries in
  order:
  1. `btrfs_extents` (`snapshot_disk_linux.go:48`,
     `snapshot_disk.go:112-154`): `BTRFS_IOC_TREE_SEARCH` over the inode's
     `EXTENT_DATA` items in the fd's own subvolume; each distinct
     `disk_bytenr` counts its `disk_num_bytes` once (`:146-150`), holes
     count zero, inline extents their inline length, preallocated extents
     their allocation. That is the file's **referenced compressed bytes**,
     exactly what compsize reports. Needs `CAP_SYS_ADMIN`.
  2. `fiemap_bound` (`snapshot_disk_linux.go:54-59`,
     `snapshot_disk.go:247-286`): FIEMAP extents, each ENCODED extent
     bounded by the gap to the file's next physical start, clamped to
     `st_blocks` (`:281-286`). An upper bound of (1); no privilege.
  3. `st_blocks*512` (`snapshot_disk_linux.go:65`; `snapshot_disk.go:93`
     when a file cannot be opened): allocated uncompressed bytes. For a Full
     Firecracker snapshot (`fcapi.go:126`), which writes every page,
     zeros included, this equals the apparent size.

### Which measure Node B now runs (hypothesis, unread)

vmctl runs as root (no `User=`), and since `10e9f18e1` its unit's bounding
set includes `CAP_SYS_ADMIN` (`nix/node-b.nix:1207-1210`, for the jailer)
and the syscall filter allows `ioctl`. So measure (1) should now succeed and
every hibernate line should read `disk_bytes_method=btrfs_extents`. No
journal line since that deploy has been read. Two code comments and one log
hint still describe the old unit and are now stale:
`snapshot_disk.go:12-14`, `snapshot_disk_linux.go:25-26` and the EPERM hint
at `snapshot_disk.go:306` ("outside vmctl's capability bounding set").
Green, left for whoever next holds vmmanager (FP8 holds it in this pass).

### Apparent, allocated, compressed, shared, exclusive

| quantity | what the code uses it for |
|---|---|
| apparent (`st_size`) | `meta.Bytes` (`dirLogicalBytes`), logged as `bytes=`; not budgeted |
| allocated (`st_blocks*512`) | the last fallback; equals apparent for a Full snapshot; blind to compression |
| referenced compressed (tree search) | the budget, when measure (1) runs |
| FIEMAP bound | the budget, when (1) fails; between (1) and allocated |
| exclusive (bytes freed if the snapshot is deleted) | not measured |

Referenced equals exclusive only if no other file or subvolume snapshot
shares the extents. Snapshot files are written fresh by Firecracker; a
jailed capture is moved out of the jail by `rename`
(`snapshot_lifecycle.go:228-231`), not copied or reflinked; Node B's
config creates no btrfs subvolume snapshots of `@` (`nix/disks.nix`; no
snapper or btrbk) and restic copies data rather than sharing extents. So
referenced should equal exclusive. The probe checks this (FIEMAP `SHARED`
flag per extent): a nonzero shared count means a deleted snapshot frees
less than the budget subtracts, and the budget overstates what eviction
reclaims.

### Gaps between the count and the disk

- **G1. Resumed VMs' memory files are not counted.** After a resume the
  snapshot names are removed, but Firecracker keeps `mem` mapped
  `MAP_PRIVATE` for its whole life (`snapshot_lifecycle.go:594-597`), so its
  extents stay allocated until that VMM exits. The budget never sees them.
  Only the 16 GiB free-space floor does (via `statfs`), which is what the
  comment at `snapshot.go:68-70` relies on. Size: one snapshot's
  compressed bytes (about 0.5 GiB, per the 2026-10-14 compsize reading) per
  running resumed VM. Bounded by running VMs; safe while the floor holds.
- **G2. `need` may omit the hotplug region (hypothesis).**
  `need` uses `MachineMemSizeMib` only. A test computer with
  `VM_MEMORY_HOTPLUG` carries a 4 GiB virtio-mem region
  (`cfg.MemoryHotplugMiB`, `snapshot.go:462`). If Firecracker's Full memory
  file includes that region, the logical worst case is 4 GiB larger than
  `need`, and both the budget reservation and the floor check under-reserve
  by up to that much. The region is shrunk to base before the snapshot
  (`snapshot_lifecycle.go:158-163`), unplugged blocks are zeros and zstd
  stores them at about 1/32, so the real exposure is small. The probe
  decides it: on a hotplug computer, compare the `mem` apparent size with
  `meta.mem_size_mib`. If it is larger, the fix is one line
  (`need` adds `cfg.MemoryHotplugMiB<<20`), red, for the vmmanager holder.
- **G3. Metadata is not counted.** Each 128 KiB compressed extent costs an
  `EXTENT_DATA` item, an extent item and checksums in btrfs metadata (an
  estimate of 150 to 200 bytes, doubled under a DUP metadata profile): on
  the order of 10 to 25 MiB per 8 GiB snapshot, a few percent of its
  compressed bytes. It comes from the metadata chunks the headroom timer
  guards (`nix/node-b.nix:1520-1555`), not from data.
- **G4. Stale figures age out.** A snapshot committed under an older
  measure keeps its recorded figure until it is resumed, evicted or ages
  out (`VM_SNAPSHOT_MAX_AGE=36h`). Those figures are overcounts (safe).
- **Partial captures** (`snapshot.partial/`) are not counted, but each is
  removed before the next capture of that VM (`snapshot_lifecycle.go:177`)
  and by the startup sweep (`snapshot.go:836-872`); at most one 8 GiB
  partial per interrupted capture, until vmctl's next start.

Over-reservation is deliberate: `need` is the logical size (8 GiB) while a
committed snapshot is expected around 0.5 GiB, so with measure (1) running
the budget holds about (48 - 8) / 0.5 = 80 committed snapshots. At that
point the free-space floor and the 36 h age, not the 48 GiB figure, are the
real bounds. Revisiting 48 GiB and 36 h (the checklist's next step) needs
the probe's per-snapshot compressed bytes and Node B's free space.

### No code fix in this pass

The count is wrong in the unsafe direction only in G1 (covered by the
floor) and possibly G2 (bounded, unproven). Neither is shown wrong by a
reading yet, and vmmanager is FP8's in this pass. The probe below takes
the readings that decide them.

### Probe (FP0, read-only, Node B)

`scripts/snapshot_budget_bytes_probe.py` prints, for every
`<state>/<vm>/snapshot` and `snapshot.partial`: the recorded `bytes`,
`disk_bytes` and `disk_bytes_method` from `meta.json`; the apparent,
allocated, FIEMAP bound, tree search (referenced compressed) and exclusive
bytes it measures itself; the shared extent count; then the totals the
budget sees against the totals the disk holds, the deleted memory files
still mapped by running VMMs (G1), and `statfs` free space. It opens files
read-only, issues only FIEMAP (without `FIEMAP_FLAG_SYNC`) and the tree
search ioctl, reads `/proc`, and writes nothing. Steps are in the
[station file](../definitions/choir-launch-fp10-res-14-2026-10-22.md).

### Probe reading (FP0, Node B, 2026-10-22 09:00 UTC)

`sudo python3 scripts/snapshot_budget_bytes_probe.py` (read-only, euid 0,
tree search exact), on deploy I (`58e705802`):

| VM | recorded (method) | apparent | allocated | FIEMAP bound | tree search | exclusive | created |
|---|---|---|---|---|---|---|---|
| vm-80b173b9 | 1503.2 MiB (btrfs_extents) | 12288.1 | 8192.1 | 3992.1 | 1503.2 | 1503.2 | 05:04 |
| vm-f2588c7b | 857.3 MiB (btrfs_extents) | 12288.1 | 8192.1 | 3441.8 | 857.3 | 857.3 | 07:05 |
| vm-fdcb6a45 | 790.3 MiB (btrfs_extents) | 12288.1 | 8192.1 | 2195.0 | 790.3 | 790.3 | 03:23 |

- Totals: 3 committed snapshots; the budget counts 3,150.8 MiB of its
  49,152 MiB, which equals the tree search's referenced compressed bytes
  and the exclusive bytes. No shared extents, no delalloc, no partial
  captures, and no deleted snapshot memory still mapped by a VMM (G1 reads
  0 at this moment). statfs: 119.3 GiB available, 103.3 GiB over the 16 GiB
  floor.
- **G2 answered:** each memory file's apparent size is 12 GiB for an 8 GiB
  guest (ratio 1.4884), so Firecracker's Full memory file does include the
  4 GiB virtio-mem region. Its allocated size is exactly 8,192 MiB: the
  region is all holes, because it is shrunk to base before the capture. The
  logical worst case on disk is therefore the base size, which is what
  `need` reserves. G2 does not under-reserve; no fix.
- **The recorded figure is the true one** for every committed snapshot
  (`btrfs_extents` equals the tree search), so measure (1) is running on
  Node B. The FIEMAP bound over-counts by 2.8 to 4.4 times (compressed
  extents' bounds), as the record expected.
- **48 GiB and 36 h:** at 0.8 to 1.5 GiB a snapshot, the budget holds about
  35 to 60 committed snapshots; free space (103 GiB over the floor) and the
  36 h age are the binding limits, not 48 GiB. No change proposed from one
  reading; a second reading under more hibernated computers would be
  needed to move either figure.
