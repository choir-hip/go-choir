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
  (no count since 2026-10-14). Answered by R1 below: in 58 hours every
  wake fell back, on `launch_config_mismatch`.
- The resident fraction of a cold-booted guest, which sets the Diff
  file's size (the report estimates about 1.2 GiB of 8). Answered by R5
  below: 1.15 to 1.37 GiB of 8.
- Whether every Firecracker process's cgroup has `memory.swap.max` 0.
  Answered by R2 below: their leaf cgroup has no such file; the vmctl
  service above it has 0.

The read-only commands for these are in the design, section 4.

## Readings (FP0, Node B, read only, 2026-10-22)

Taken as root over `ssh node-b`, commands as written in the design
section 4. Host-wide readings name every VM because they read host
metadata only; the per-guest reading (R5) is FP0's own test computer.

**The journal is shorter than 7 days.** `go-choir-vmctl`'s journal starts
at 2026-10-19T19:05:51Z (262 MB of journals on disk), so every
"7 days ago" reading below covers about 58 hours.

**R1, hibernate and wake outcomes (04:45:42Z).**

```
--- hibernates by snapshot
8
--- not possible reasons
--- resumed from snapshot
0
--- cold boot after fallback reasons
      2 (launch_config_mismatch
--- snapshot ms / disk_bytes / method
      1 snapshot=9258ms	disk_bytes=1356226560	disk_bytes_method=btrfs_extents
      1 snapshot=8627ms	disk_bytes=821219328	disk_bytes_method=btrfs_extents
      1 snapshot=8395ms	disk_bytes=1001492480	disk_bytes_method=btrfs_extents
      1 snapshot=8194ms	disk_bytes=828735488	disk_bytes_method=btrfs_extents
      1 snapshot=6688ms	disk_bytes=2443395072	disk_bytes_method=btrfs_extents
      1 snapshot=6425ms	disk_bytes=996552704	disk_bytes_method=btrfs_extents
      1 snapshot=5887ms	disk_bytes=933744640	disk_bytes_method=btrfs_extents
      1 snapshot=5576ms	disk_bytes=735551488	disk_bytes_method=btrfs_extents
```

The eight hibernates, one line each (all `reason=idle` in vmctl's line):

```
2026-10-20T11:57:49Z vm-fa4a03e4 snapshot=5887ms bytes=12885000510 disk_bytes=933744640
2026-10-20T15:04:58Z vm-c6318055 snapshot=8395ms bytes=12885000510 disk_bytes=1001492480
2026-10-20T15:36:51Z vm-a91cbc86 snapshot=5576ms bytes=12885000510 disk_bytes=735551488
2026-10-20T16:33:31Z vm-4a9afa23 snapshot=9258ms bytes=12885000510 disk_bytes=1356226560
2026-10-20T17:33:28Z vm-b1091b1e snapshot=6688ms bytes=8590024448  disk_bytes=2443395072
2026-10-21T05:00:40Z vm-878ab006 snapshot=6425ms bytes=12885000510 disk_bytes=996552704
2026-10-22T02:02:18Z vm-80b173b9 snapshot=8627ms bytes=12885000503 disk_bytes=821219328
2026-10-22T03:23:24Z vm-fdcb6a45 snapshot=8194ms bytes=12885000503 disk_bytes=828735488
```

Reading: in 58 hours eight hibernates, every one Full and idle, writing
5.6 to 9.3 s each (the 10-14 report said 7.6 to 8.1 s). No snapshot was
ever resumed. The two wakes that R1 counts both cold booted on
`launch_config_mismatch`. The other six, from the journal lines that
name each VM:
- vm-c6318055 and vm-b1091b1e were stopped through the product
  (`POST /internal/vmctl/stop caller="proxy.lifecycle.stop"`) 3.5 and
  12 minutes after hibernating, which discards the snapshot by design.
  vm-c6318055 then booted on resolve 18 s later (`started retained VM`),
  a cold boot that neither R1 nor R6 counts.
- vm-fa4a03e4 and vm-a91cbc86 were removed 24 h after hibernating
  (`destroyed stale VM state`), the ephemeral-account retention.
- vm-878ab006 and vm-fdcb6a45 were still hibernated at R3 (04:46);
  vm-878ab006 was then destroyed as stale VM state at 05:01:23, 24 h
  after its hibernate.

**R1 for L20, mismatch timing against deploys.**

```
2026-10-21T03:45:30+00:00 ... vmmanager: cold boot of VM vm-4a9afa2305e274afa45c468e714a1550 after snapshot fallback (launch_config_mismatch)
2026-10-22T04:40:22+00:00 ... vmmanager: cold boot of VM vm-80b173b9ea3d75571723286e867dc191 after snapshot fallback (launch_config_mismatch)
```

Deploy times come from vmctl's own `deploy refresh caller=deploy ...
target=` lines (first line per target), checked against FP0's ledger
and `gh run list --workflow ci.yml --limit 60` for tonight. The journal
holds 76 distinct deploy targets in 58 hours, about 1.3 an hour.

- vm-4a9afa23 (the owner's computer) hibernated 2026-10-20 16:33:31. Twelve
  deploys landed before its wake: 17:57 7eb24d630, 18:14 ac4737862, 19:48
  2db9b8bf9, 20:04 22b287a5d, 21:39 710ce53cb, 22:30 4346e9577, 23:06
  43b5bc034, 10-21 00:06 be786b801, 02:28 7b55b19a7, 02:44 e0d1b0b6f,
  03:20 7bb7b3e65, 03:32 8d027370b. It woke 13 minutes after the last,
  11.2 hours after hibernating.
- vm-80b173b9 (FP2's test computer) hibernated 2026-10-22 02:02:18. Six
  deploys landed before its wake: 02:18 4dbdee107 (A), 02:32 51cb3a8cd,
  02:51 3c46d5196 (B), 03:39 3c16f7c33 (C), 04:18 fd58229aa (D), 04:29
  840e1a701 (D's vendorHash fix). It woke 11 minutes after the last,
  2.6 hours after hibernating.

Why every deploy mismatches: `/var/lib/go-choir/guest` is a symlink to
the current guest image's Nix store path, and `kernel-params` carries
`init=/nix/store/...-nixos-system-go-choir-autoputer-...` and
`build_commit=`. Each commit builds a new guest image, so each deploy
changes the launch config. Even 840e1a701, which changed only the Go
vendorHash, has its own guest image path.

Reading: the mismatch is found at the wake, not at the deploy, and with
a deploy every 45 minutes on average and a 30-minute idle timeout, no
snapshot in this window lived to be resumed. At tonight's cadence a
Diff snapshot (L3) would save write time and PSI at hibernate but would
almost never be loaded, so L20 is what lets snapshots pay off on staging.

**R2, can guest memory swap (04:46:28Z).**

```
NAME           TYPE SIZE   USED PRIO
/swap/swapfile file  16G 189.3M   -2
cat: /sys/fs/cgroup/system.slice/go-choir-vmctl.service/supervisor/memory.swap.max: No such file or directory
3403323 /system.slice/go-choir-vmctl.service/supervisor swap.max= VmSwap:	       0 kB
(the same two lines for 3921600, 3921732, 3922239, 3925035, 3926651)
```

The leaf has no memory controller, so a supplementary read of its parent
(04:46:53Z):

```
go-choir-vmctl.service/cgroup.subtree_control:
go-choir-vmctl.service/memory.swap.max: 0
go-choir-vmctl.service/memory.max: max
go-choir-vmctl.service/memory.swap.current: 0
go-choir-vmctl.service/memory.current: 23571836928
supervisor/cgroup.controllers:
Delegate=yes
MemoryMax=infinity
MemorySwapMax=0
```

Reading: guest memory cannot swap today. All six Firecracker processes
read `VmSwap: 0 kB`. They sit in `go-choir-vmctl.service/supervisor`,
which has no controllers enabled, so their memory is charged to
`go-choir-vmctl.service`, whose `memory.swap.max` is 0 and swap use 0.
Swap itself is on (16 GiB file, 189 MB used by other processes). The
design's check as written fails here: it reads `memory.swap.max` in the
process's own cgroup and finds no file. The check has to walk up from the
process's cgroup to the root and take the guarantee from any ancestor
whose `memory.swap.max` is 0, since the cgroup v2 limit is hierarchical.
The jailer's cgroup parent is `system.slice/go-choir-vmctl.service/vms`
(`VMCTL_JAILER_CGROUP_PARENT`), inside the same service, so the 0 should
still bound jailed processes even if their own leaf reads `max`. Repeat R2
once FP8's jailer is on for any computer.

**R3, snapshot files on disk (04:46:29Z).**

```
/var/lib/go-choir/vm-state/vm-878ab006efc29480ec2193fdfaab793c/snapshot/mem size=12884901888 blocks=16777216 x 512
/var/lib/go-choir/vm-state/vm-fdcb6a457f1c003913e344bb1ec92f64/snapshot/mem size=12884901888 blocks=16777216 x 512
     Total   Exclusive  Set shared  Filename
   8.00GiB     8.00GiB       0.00B  /var/lib/go-choir/vm-state/vm-878ab006efc29480ec2193fdfaab793c/snapshot
   8.00GiB     8.00GiB       0.00B  /var/lib/go-choir/vm-state/vm-fdcb6a457f1c003913e344bb1ec92f64/snapshot
```

Reading: two snapshots are on disk. Each memory file is 12 GiB long
with 8 GiB allocated (`st_blocks`), so a Full snapshot is already sparse
by 4 GiB. A guess, not confirmed: that is the unplugged virtio-mem range.
Their hibernate lines report 0.83 and 1.0 GB on disk by `btrfs_extents`,
so on btrfs with zstd `st_blocks` counts allocated, uncompressed bytes.
Design failure mode 6 ("a Diff file's `st_blocks` is its real size")
holds for allocation, not for compressed bytes on disk.

**R4, Firecracker and snapshot-editor (04:46:53Z).**

```
fc=/nix/store/7nph1fsip8ak8k4ps56jhh1ny1lqvblj-firecracker-1.16.1/bin/firecracker
Firecracker v1.16.1
clippy-tracing
cpu-template-helper
firecracker
jailer
rebase-snap
seccompiler-bin
snapshot-editor
always [madvise] never
0
0
```

Reading: Firecracker 1.16.1 is above the 1.15.0 floor (Diff corruption
#5705 fixed), and `snapshot-editor` is installed next to it, so L3b's
rebase tool is there. KSM is off and has merged nothing. THP is
`madvise`, not `never` as the design assumed, so its "THP off" condition
fails as written. Guest memory gets THP only if Firecracker madvises it;
R5's `AnonHugePages` for the guest mapping says whether it does.

**R5, resident fraction of a guest.** In its own section below.

**R6, how soon hibernated computers wake (04:47:29Z).**

```
n=2 p10=9484 p50=40319 p90=40319
vm-4a9afa2305e274afa45c468e714a1550 40319
vm-80b173b9ea3d75571723286e867dc191 9484
```

Reading: two wakes in 58 hours, at 2.6 h and 11.2 h after the hibernate.
Both were cold boots. Too few to set L4's timeout from; what they show
is that tonight's hibernated computers wake hours later, past several
deploys.

**R6, PSI across a hibernate.** FP0 ran the design's sampler at 1 Hz
from 04:50:31Z (`printf "%s " "$(date -u +%T)"; grep some
/proc/pressure/memory`, two hours, log
`/Users/wiz/Dev/lanes/_runs/res22/psi-1hz.log` on FP0's Mac). Two
hibernates fell inside it, both pressure reclaims, not idle ones:

```
2026-10-22T04:59:56 vmctl: pressure reclaim decision=reclaim signal=sustained streak=4/4 mem_available_mib=2281 floor_mib=4779 some_avg10=21.97 some_avg60=13.60 ... vm=vm-c5c801f8...
2026-10-22T05:00:07 vmmanager: hibernated VM vm-c5c801f8... by snapshot (fence=6ms pause=2ms snapshot=10369ms bytes=12885000503 disk_bytes=4033429504 disk_bytes_method=btrfs_extents ...
2026-10-22T05:03:52 vmctl: pressure reclaim decision=reclaim signal=sustained streak=4/4 mem_available_mib=4043 floor_mib=4779 some_avg10=24.93 some_avg60=20.74 ... vm=vm-80b173b9...
2026-10-22T05:04:02 vmmanager: hibernated VM vm-80b173b9... by snapshot (fence=4ms pause=0ms snapshot=9522ms bytes=12885000503 disk_bytes=1576189952 disk_bytes_method=btrfs_extents ...
```

`some avg10` (the kernel updates it every 2 s) and the stall per
sample, from the `total=` counter, in ms:

```
04:59:36-04:59:44  avg10 21.9-30.3   stall 336-586 ms/s   (before the first reclaim)
04:59:56-05:00:07  avg10 19.4->13.7  stall  88-126 ms/s   (vm-c5c801f8 writing, 10.4 s)
05:00:13-05:00:16  avg10 16.5-21.1   stall 253-431 ms/s   (after)
05:03:31-05:03:45  avg10 28.9-29.7   stall 227-398 ms/s   (before the second reclaim)
05:03:52-05:04:02  avg10 24.9->17.8  stall 108-235 ms/s   (vm-80b173b9 writing, 9.5 s)
05:04:04-05:04:30  avg10 15.5-18.7   stall 112-294 ms/s   (after; FP0's test computer admitted 05:04:19)
quiet baseline 04:50:30-04:55:00: 10.4 ms/s mean
```

Reading: tonight's host was short of memory, stalling 20 to 30% of the
time for minutes before each reclaim. The 10.4 s and 9.5 s Full writes
added no visible step: the stall fell while each guest was written out,
because the shortage, not the write, was the stall. So this sample does
not show that a hibernate is free. It shows that on a host at the floor
the write is not what hurts, and it gives L4 no quiet baseline to
compare with. L3's proof step still needs its own before-and-after PSI on
a quiet host. Both writes were longer than the eight in R1 (5.6 to
9.3 s), and vm-c5c801f8 wrote 4.0 GB on disk, the largest so far.

**R7, a guest release's size (04:47:05Z).**

```
total 3981M
   1M -r--r--r-- 1 root root    1M Jan  1  1970 build.json
   1M -r--r--r-- 1 root root    1M Jan  1  1970 guest-image-manifest
  26M -r--r--r-- 1 root root   26M Jan  1  1970 initrd
   1M -r--r--r-- 1 root root    1M Jan  1  1970 kernel-params
  41M -r--r--r-- 1 root root   41M Jan  1  1970 rootfs.ext4
3531M -r--r--r-- 1 root root 3531M Jan  1  1970 storedisk.erofs
 385M -r-xr-xr-x 1 root root  385M Jan  1  1970 vmlinux
     Total   Exclusive  Set shared  Filename
   3.89GiB     3.89GiB       0.00B  /var/lib/go-choir/guest
```

Supplementary (04:49:42Z): `/var/lib/go-choir/guest` is a symlink to
`/nix/store/lmg4sg19nxg0cb8cy73myczrvy8sv1mb-go-choir-guest-image`
(`build.json` commit 840e1a701). The store holds 11
`*-go-choir-guest-image` paths; six of them measured 3.14 to 3.90 GiB
each, with 0 to 222 MiB shared between them.

Reading: a release is about 3.9 GiB, almost all `storedisk.erofs`, and
consecutive releases share little on disk. The store already keeps each
release in its own directory until `go-choir-nix-gc` (every 4 h)
collects it. So L20's `releases/<digest>/` could be a GC root on the
store path rather than a reflink copy. That is for FP10 to weigh, not
decided here. Retention is the cost: two releases kept beyond `current`
is about 8 GiB.

## R5 and the test computer's hibernate-and-wake (FP0, 2026-10-22)

FP0 minted its own test computer for R5: label `res13-r5-1022`,
computer-2bd23366dc4bc1f9fd3aed0b299491a5, VM
vm-f2588c7b67c78f61388c540947ec35a3, class `test` (so virtio-mem hotplug
on, jailer off). Every per-guest reading below is of this VM only.

**Admission.** The mint started 04:45:06Z. Host memory admission refused
the computer every 3 s from 04:49 (`available_mib` about 4200 against
2048 + the 4779 MiB floor + 1024 headroom) until two pressure reclaims
(R6) freed memory: `admission decision=admit ... available_mib=7878` at
05:04:19, booted 05:04:32 (epoch 13227). The key was minted 05:04:47.

**Its boots, hibernates and wakes (vmctl journal):**

```
05:04:32 booted Firecracker VM vm-f2588c7b... (epoch=13227)                       cold boot at mint
05:12:05 destructive kill vm=vm-f2588c7b... cause=refresh epoch=13227             deploy d991907e6
05:12:14 refreshed VM vm-f2588c7b... (new_epoch=13228, deploy-image-refresh)      cold boot
05:44:13 hibernated VM vm-f2588c7b... by snapshot (fence=5ms pause=0ms snapshot=8429ms bytes=12885000503 disk_bytes=826048512 disk_bytes_method=btrfs_extents ...)  reason=idle
05:44:44 vmctl restarted for deploy 8fabec36a (new guest image), 31 s after the snapshot
06:02:06 snapshot resume of VM vm-f2588c7b... not used (launch_config_mismatch)
06:02:06 cold boot of VM vm-f2588c7b... after snapshot fallback (launch_config_mismatch)
06:02:14 booted VM vm-f2588c7b... (epoch=13229); started retained VM ... on resolve
```

The 05:44:13 snapshot on disk (05:51:33Z, R3's commands):

```
/var/lib/go-choir/vm-state/vm-f2588c7b67c78f61388c540947ec35a3/snapshot/mem size=12884901888 blocks=16777216 x 512
     Total   Exclusive  Set shared  Filename
   8.00GiB     8.00GiB       0.00B  /var/lib/go-choir/vm-state/vm-f2588c7b67c78f61388c540947ec35a3/snapshot
```

Reading: the third mismatch tonight, and the plainest. The snapshot was
31 s old when a deploy voided it. This Full hibernate is also the
baseline for L3a's proof: 8.4 s, 8 GiB written into a 12 GiB file
(the 4 GiB unplugged hotplug range stays a hole), 0.83 GB on disk after
zstd, from a guest with about 1.2 GiB resident (below).

**Two findings about the wake and the idle clock (with traces).**

1. *An API key cannot wake a hibernated computer.* With the computer
   hibernated, every `GET /api/runtime/observability` with its key from
   05:51:59 to 05:59:19 answered 502 `failed to resolve user autoputer`.
   The proxy logged `failed to resolve autoputer for owner 00a66cfa...:
   api key computer realization unavailable`, and vmctl logged nothing
   for the computer. That is by design
   (`internal/proxy/api_key_computer_authority.go`,
   `resolveComputerURLForComputerTarget`): "a bearer request never
   re-resolves or wakes a logical desktop after authorization ...
   Recovery remains an explicit computer route." FP0 woke it with the
   explicit route the CLI's `computer start` uses, `POST
   /api/computers/{id}/lifecycle/start` (06:02:04Z, 201,
   `prior_lifecycle_state` hibernated, `resulting_lifecycle_state`
   active, epoch 13228 to 13229). The design's R5 step and FP0's
   assignment assumed a product read wakes it; it does not with a key.
2. *API-key traffic does not move the idle clock.* vmctl hibernated the
   computer at 05:44:13, 32 minutes after the 05:12:14 refresh boot, but
   only 25.5 minutes after FP0's last API-key request (05:18:42) and
   its model run (passivated 05:18:31). With a 30-minute timeout and a
   2-minute sweep, the clock must have started near the 05:12:14 boot,
   not at the requests. That fits the
   idle sweep's own comment (`idleOwnershipCandidates`: `LastActiveAt`
   "tracks ownership-resolution traffic only"), which a bearer request
   never reaches. Hypothesis, not confirmed by a `LastActiveAt` read:
   a computer driven only through its key hibernates 30 minutes after
   its last resolve, however busy its key is between runs, and then
   answers 502 until a lifecycle start. That matters for L4: at 10
   minutes, key-driven computers would hibernate between calls.

**The second cycle, which resumed.** After the 06:02:14 cold boot FP0
made one model call (below), left the computer idle, took R5 at 25 idle
minutes, and woke it through `lifecycle/start` as soon as vmctl's idle
sweep hibernated it, to leave a deploy no time to void the snapshot:

```
06:32:51 hibernated VM vm-f2588c7b... by snapshot (fence=3ms pause=0ms snapshot=5987ms bytes=12885000510 disk_bytes=904536064 disk_bytes_method=btrfs_extents ...) reason=idle (epoch=13229)
06:32:52 snapshot/mem size=12884901888 blocks=16777216 x 512; btrfs du 8.00GiB exclusive; guest build.json 8fabec36a (unchanged since the boot)
06:32:53 POST /api/computers/computer-2bd23366.../lifecycle/start -> 201 (hibernated -> active, epoch 13229 -> 13229)
06:32:53 admission decision=admit ... need_mib=1413 need_source=measured available_mib=9877
06:32:54 vmmanager: resumed VM vm-f2588c7b... from snapshot (host=http://10.201.105.2:8085 epoch=13229 load=8ms wake_to_healthy=351ms clock_step=8556ms credential=delivered)
```

That is the only snapshot resume in the journal's 60 hours. PSI across
this hibernate, on a host that had gone quiet (stall per 1 s sample, ms):

```
06:32:31-06:32:44  avg10 0.10->0.02  stall 0-3
06:32:45-06:32:50  avg10 0.02->1.03  stall 22-36   (the 6.0 s Full write)
06:32:51-06:33:15  avg10 1.03->0.09  stall 0-2
```

Reading: on a quiet host a Full hibernate of an 8 GiB guest with 1.4 GiB
resident raised `some avg10` to 1.03 and stalled about 25 to 36 ms in
each second of the 6 s write. That is a small step, smaller than the
10-14 report's 2.5, and it is the baseline L3a's Diff proof compares
against.

**R5 readings** (the design's command; one extra `awk` for the same
mappings' `Private_Clean`, `Private_Dirty`, `Anonymous`, `AnonHugePages`,
`Swap`, `THPeligible`). Cold, 25 idle minutes after the 06:02:14 boot
(06:27:35Z):

```
pid=4055898 comm=firecracker start=Thu Oct 22 06:02:05 2026
size_kB=4194304 rss_kB=0 72f738000000-72f838000000 ---p 00000000 00:00 0
size_kB=8388608 rss_kB=1435420 72f838000000-72fa38000000 rw-p 00000000 00:00 0
72f838000000-72fa38000000 Private_Dirty=1435420 Anonymous=1435420 AnonHugePages=0 Swap=0 THPeligible=0
VmRSS:	 1441532 kB
VmSwap:	       0 kB
```

Restored, 32 s after the resume (06:33:26Z, before the post-wake call):

```
pid=4072903 comm=firecracker start=Thu Oct 22 06:32:53 2026
size_kB=4194304 rss_kB=0 7533c0000000-7534c0000000 ---p 200000000 00:20 904160121  .../snapshot/mem (deleted)
size_kB=5242880 rss_kB=219204 7534c0000000-753600000000 rw-p c0000000 00:20 904160121  .../snapshot/mem (deleted)
size_kB=3145728 rss_kB=18036 753600000000-7536c0000000 rw-p 00000000 00:20 904160121  .../snapshot/mem (deleted)
5 GiB region: Private_Clean=184332 Private_Dirty=34872 Anonymous=34872 THPeligible=0
3 GiB region: Private_Clean=17540 Private_Dirty=496 Anonymous=496 THPeligible=0
VmRSS:	  243628 kB
VmSwap:	       0 kB
```

All readings of this guest's 8 GiB of RAM (the 4 GiB `---p` mapping is
the unplugged hotplug range; nothing is ever resident there):

| time (Z) | process | when | resident | share of 8 GiB |
|---|---|---|---|---|
| 05:06:34 | epoch 13227, cold | 2 min after boot, 1 min after a model run | 1,236,308 kB | 14.7% |
| 05:20:09 | epoch 13228, cold | 8 min after a refresh boot, 1.5 min after a run | 1,208,464 kB | 14.4% |
| 06:27:35 | epoch 13229, cold | 25 idle minutes after boot | 1,435,420 kB | 17.1% |
| 06:33:26 | epoch 13229, restored | 32 s after the resume | 237,240 kB | 2.8% |
| 06:42:37 | epoch 13229, restored | 9.7 min after the resume, 9 min after a model run | 494,240 kB (338,228 clean, 156,012 dirty) | 6.0% |

Reading:
- A cold guest holds 14 to 17% of its RAM (1.15 to 1.37 GiB), all
  anonymous dirty pages, in line with the report's 0.15. A Diff
  hibernate of it would write about 1.4 GiB where a Full one writes
  8 GiB. That is L3a's case and the number its gate asked for.
- THP never reaches guest memory: `THPeligible 0` and `AnonHugePages 0`
  on every guest mapping, cold or restored. So R4's `madvise` does not
  block L3a. The design's condition should test the guest mapping, not
  the host's THP mode.
- A restored guest's RAM is a private mapping of the snapshot file, and
  most of what it touched is clean file pages (201,872 of 237,240 kB
  here), which
  the kernel may evict. `mincore` would miss evicted pages. This is the
  design's failure mode 1, seen on staging: a restored guest must not
  take a Diff without L3b's base. The `ColdOrigin` guard is needed.
- The mapping already reads `(deleted)`: a healthy resume removes the
  snapshot's names at once (`snapshot_lifecycle.go`, after the health
  wait: "Removing the names is safe (the mapping holds the inode)").
  The design's L3b paragraph says the directory stays until the next
  hibernate; on main it does not. L3b has to link the memory file to
  `snapshot.base/mem` in place of that discard. Until a restored guest
  exits, its snapshot blocks (0.9 GB here) stay allocated but nothing
  under `vm-state` names them. That is a gap in RES-14's honest-bytes
  count.

**The model calls through the product path** (for FP7's
`bearer_binding.would_refuse` read; FP0 did not read or change the
gateway). Each is `POST /api/prompt-bar` with the computer's key and the
text "Reply with the single word: ready". The prompt bar opens a
Texture run, and that run makes the model call:

| call (Z) | boot | submission | run (Texture) | model | tokens in / cached / out | run passivated (Z) |
|---|---|---|---|---|---|---|
| 05:05:36 | epoch 13227, cold, booted 05:04:32 | 423c728f-757e-58c0-8f06-ac127169dcce | 885b4dcd-b0b0-4a19-9b4a-fef2f06f8c5a | chatgpt gpt-6-luna | 4031 / 3072 / 102 | 05:05:42 |
| 05:18:23 | epoch 13228, cold, booted 05:12:14 | 046626dd-a6a5-5db3-b712-c009d77cf7aa | 935cfb2c-3280-4c17-a2ed-d138aeccde6d | chatgpt gpt-6-luna | 4005 / 0 / 96 | 05:18:31 |
| 06:02:50 | epoch 13229, cold, booted 06:02:14 | 66ceb4d8-fd3f-5a83-94ee-494d8be5e5e6 | cc565855-33b0-4460-8e07-7b05516e7ad4 | chatgpt gpt-6-luna | 4009 / 3072 / 123 | 06:02:54 |
| 06:33:26 | epoch 13229, restored, resumed 06:32:54 | 08de4aba-2d5c-5b7e-b159-854fcc8753f3 | f970f55e-e31b-497d-ae1e-307fc2445e57 | chatgpt gpt-6-luna | 3545 / 3072 / 98 | 06:33:31 |

The pair for FP7's hibernate-and-wake leg is the last two: run
cc565855 before the hibernate and run f970f55e 32 s after the resume, on
the same boot (epoch 13229, guest build 8fabec36a deployed 06:02:12).

## Fix, L3a (built 2026-10-22, lane/fp10-res13-l3a; not deployed)

Tests first (b0072c20e, ff96a7bf2), then the fix (c657c0220):
`internal/vmmanager/snapshot_diff.go` plus small hunks in `config.go`,
`fcapi.go`, `manager.go`, `snapshot.go` and `snapshot_lifecycle.go`.
A hibernate takes `snapshot_type: Diff` only when `VM_SNAPSHOT_TYPE=diff`,
`VM_SNAPSHOT_DIFF_ONLY` selects the computer, the process was cold booted
by this vmctl, Firecracker is 1.15.0 or later, some cgroup from the
process's own up to the root reads `memory.swap.max` 0 (any ancestor
reading 0 bounds the leaf; none reading 0 refuses), `VmSwap` is 0, and
every guest mapping reads `THPeligible 0`. Otherwise Full, with the reason in the hibernate
timeline's `diff_refused` and the log line. A Diff create error takes Full
in the same pause. A Diff loads only with `cold_origin` and a memory file
exactly `(mem_size_mib + memory_hotplug_mib)` MiB long. `nix/node-b.nix`
sets `VM_SNAPSHOT_TYPE=full` and an empty list: no computer changes.

Round 2 (review bunch 49, DeepSeek v4.1 flash and Gemini 3.8 flash):
tests 4d97cc4be, fix in the red commit after it.
- F2: the Full-or-Diff decision runs before the pause, and
  `firecracker --version` runs for it only after the off, selection,
  cold-origin, pid and shape checks pass. A full-mode hibernate (the
  paper's) runs no exec for it, and no hibernate runs one inside the pause.
- F1: Firecracker 1.13+ `set_len()`s a Diff file to the full length, so a
  short write cannot happen. The reachable mismatch is the shape: vmctl
  selected a hotplug region the launch omitted (the device is dropped
  when its sizes fail validation), or the reverse. The shape is checked
  before the pause (`hotplug_shape_mismatch`), and the file's length after
  the create (`diff_length_mismatch_at_create`); either takes Full in the
  same pause. The load-time length rule stays as defence.
- F4: a Diff meta loads only while the lever still selects its computer
  (`diff_lever_off` otherwise): emptying `VM_SNAPSHOT_DIFF_ONLY` or setting
  `VM_SNAPSHOT_TYPE=full` cold boots a pending Diff. A git revert of the
  code is different: old code has no Diff rules and loads it, which is
  correct for a cold-origin Diff.
- F5: tests pin that a reattached process is never cold origin and that a
  jailed Diff error clears the jail's capture dir. F6: the wake's timeline
  records `snapshot_type` and `cold_origin`.
- Follow-up (round 2 accepts; Sonnet's mutation tests): the hotplug
  subtest now asserts the hibernate took Diff, and a jailed Diff success
  and a jailed wrong-length fallback are tested. Each new test kills a
  mutant the earlier suite let through (the create-time length without the
  hotplug term; the create-time stat on the host partial dir instead of
  the jail's). The design no longer says the pause adds no file read: the
  create-time stat runs inside it.

Known limit: a vmctl restart reattaches running guests with
`cold_origin` false (vmctl did not see their launch), so after any deploy
that restarts vmctl a running guest's next hibernate is Full until its
next cold boot. At the 2026-10-22 deploy cadence that covers most
hibernates; persisting the origin beside the pid file is the follow-up.
