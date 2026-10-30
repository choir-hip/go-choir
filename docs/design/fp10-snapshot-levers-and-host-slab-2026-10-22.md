# FP10 design: snapshot levers (RES-13) and host slab and btrfs metadata (RES-22)

Status 2026-10-30: the L20 section is superseded by the density D2 design ([density-d2-snapshots-survive-deploys-2026-10-26.md](density-d2-snapshots-survive-deploys-2026-10-26.md)); read D2 for that lever.

Date: 2026-10-22. Station:
[choir-launch-fp10-res-13-22-2026-10-21.md](../definitions/choir-launch-fp10-res-13-22-2026-10-21.md).
Problems:
[hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md](../problems/hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md),
[node-b-host-slab-btrfs-inode-cache-2026-10-22.md](../problems/node-b-host-slab-btrfs-inode-cache-2026-10-22.md),
[node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md](../problems/node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md).

Mutation class of this document: green. Every build slice below is red
(vmmanager/vmctl; L20 also deployment routing) and ships behind a flag
whose off value is today's behaviour. Nothing here is built. The code is
FP8's while lane/fp8-image-jailer is open, so every slice starts after FP8
lands, rebased on it.

## 1. Order

1. FP0 takes the readings in section 4. They decide each gate.
2. **L3a**, Diff snapshots of cold-booted guests (S).
3. **L3b**, Diff snapshots of restored guests, rebased onto the old memory
   file (S, after L3a has run a week on test computers).
4. **L4**, a shorter idle timeout (XS, config), after L3 is on for every
   class and RES-14 (FP8) shows honest bytes.
5. **L20**, snapshots that survive deploys (M), after FP8's jailer, with
   an owner question (section 5).
6. RES-22: readings; then a headroom alert (XS); then the knob, only if the
   readings call for it. The file-count substrate is tape segments, which
   are on; the legacy repack belongs to the tape segments goal.

## 2. RES-13

### L3a: Diff snapshots of cold-booted guests

**Mechanism.** `CreateFullSnapshot` (`internal/vmmanager/fcapi.go:124`)
gains a sibling `CreateDiffSnapshot`, `"snapshot_type": "Diff"`. With
dirty-page tracking off (vmmanager never sets `track_dirty_pages`),
Firecracker 1.13 and later find the pages to write with `mincore(2)` and
write only resident pages into a sparse file of the full memory size. For a
guest whose Firecracker process was cold booted, a page that is not
resident was never touched, or was discarded by free page reporting or
never plugged by virtio-mem. Each of those reads as zero, which is what a
hole in the file reads as. So the sparse file is a complete memory image
and loads with the existing `LoadSnapshot` path unchanged.

**Flag.** `VM_SNAPSHOT_TYPE=full` (default, today) | `diff`, with
`VM_SNAPSHOT_DIFF_ONLY=class:test` or a list of ComputerIDs, read the
same way as `VM_MEMORY_HOTPLUG_ONLY`.

**When a hibernate may take Diff** (every condition, else Full, as today):
- the flag selects this computer;
- the running Firecracker process was cold booted, not restored (a new
  in-memory fact set at launch; a restored process sets it false);
- the Firecracker version is at least 1.15.0 (diff corruption #5705 fixed);
- the process's memory cannot swap: `VmSwap` in `/proc/<pid>/status` is 0,
  and walking up from the process's cgroup (`/proc/<pid>/cgroup`, the
  `0::` line) towards the root, some cgroup's `memory.swap.max` reads `0`.
  The cgroup v2 limit is hierarchical, so the nearest ancestor that sets 0
  bounds the leaf. Today (reading R2, 2026-10-22) the leaf
  `go-choir-vmctl.service/supervisor` has no memory controller (no file)
  and `go-choir-vmctl.service` reads 0. Under FP8's jailer the process sits
  under `VMCTL_JAILER_CGROUP_PARENT` (`go-choir-vmctl.service/vms`) and the
  same walk applies. No ancestor reading 0, or an unreadable cgroup,
  refuses Diff. This is the check that keeps the S3 design's D3 objection
  answered under the jailer;
- THP never reaches guest memory: every guest memory mapping in
  `/proc/<pid>/smaps` (every mapping of 32 MiB or more) reads
  `THPeligible: 0`, and at least one such mapping exists. The host's
  global THP mode is not the test (reading R4: Node B is `madvise`, and
  R5 found `THPeligible 0` on every guest mapping). KSM is off on Node B
  and Firecracker never marks guest memory mergeable; a merged page is
  resident anyway, so KSM is no hazard to `mincore` and has no check.

**Meta.** `SnapshotMeta` gains `SnapshotType` (`Full` when empty, so old
metas read as Full) and `ColdOrigin`. No version bump: a version bump would
void every existing snapshot at the deploy. Validation adds one rule: a
`Diff` meta loads only if `ColdOrigin` is true and the memory file's
length equals the guest's memory size: base memory plus the virtio-mem
region (`MemSizeMiB + Config.MemoryHotplugMiB`, in bytes). Reading R3: a
hotplugged test computer's Full memory file is 12 GiB long, 8 GiB base
plus the 4 GiB region. An unknown `SnapshotType` is refused.

**The decision is taken before the pause** (round 2, F2): none of the
decision's reads (`/proc` status and smaps, the cgroup walk) and no
`firecracker --version` exec adds frozen time, and the version runs only
once the cheap checks pass, so a full-mode hibernate never execs it. Inside
the pause a selected computer adds the Diff create and one `os.Stat` of
its memory file (the create-time length check); a full-mode hibernate adds
nothing.

**A Diff create error** falls back to a Full create inside the same pause,
then to the kill path as today. It never leaves a partial Diff in `snapshot/`.

**Ways it could fail (written before code; each gets a test):**
1. A restored guest takes Diff: untouched pages are file-backed by the old
   memory file and may be evicted, so `mincore` misses them, and the
   guest wakes with zero pages. Guard: `ColdOrigin`; test that a resumed
   VM always takes Full.
2. Guest memory swapped: `mincore` misses swapped pages. Guard: the swap
   check; test with a fake `/proc` reading `VmSwap: 4 kB` and with
   `memory.swap.max` = `max`.
3. Firecracker returns an error for Diff: test the in-pause Full fallback.
4. The memory file has the wrong length. Firecracker 1.13+ `set_len()`s
   a Diff file to the full memory size before writing, so a short write
   does not happen (review bunch 49). The reachable mismatch is the
   shape: vmctl selected a hotplug region that the launch omitted (the
   device is dropped when its sizes fail validation), or the reverse.
   Guards: before the pause, the launch config's device must agree with
   `MemoryHotplugMiB`; after the create, the file must be
   `(MachineMemSizeMib + MemoryHotplugMiB)` MiB long; either failure takes
   Full in the same pause. At load the length rule refuses a mismatch and
   the computer cold boots.
5. A rollback. Turning the lever off (an empty `VM_SNAPSHOT_DIFF_ONLY` or
   `VM_SNAPSHOT_TYPE=full`) refuses a pending Diff at load
   (`diff_lever_off`), so it cold boots. A git revert of the code is
   different: an old vmctl ignores the unknown fields, sees a regular
   memory file and loads it, which is correct for a cold-origin Diff. Either
   way the rollback needs no repair.
6. The budget: a Diff file's `st_blocks` is its real size, so the budget
   becomes honest for Diff snapshots without `CAP_SYS_ADMIN` (RES-14 still
   matters for Full ones).

**Proof (FP0, one test computer).** Cold boot it. Wake it every time
through `POST /api/computers/{id}/lifecycle/start`: an API-key read cannot
wake a hibernated computer (reading R5, `api_key_computer_authority.go`). In the guest (through the
product's own terminal or engineering capsule, not a block device), write
1 GiB of random bytes to tmpfs and record its sha256; record
`/proc/meminfo`. Hibernate. On the host, record the hibernate log line
(`snapshot=…ms`, `disk_bytes`, `disk_bytes_method`), `stat -c '%s %b %B'`
of `snapshot/mem`, and `/proc/pressure/memory` before and after. Wake.
In the guest, check the sha256 and that the computer is healthy and its
desks answer. Then repeat with `VM_SNAPSHOT_TYPE=full` on the same computer
for the comparison. Then hibernate the resumed computer again and check the
log says Full (failure mode 1).

### L3b: Diff snapshots of restored guests

After the first wake, every later hibernate of that computer is of a
restored guest, so L3a alone covers only the first hibernate after each
cold boot. Today a resume marks the snapshot consumed
(`consumeSnapshot`), and once the guest is healthy it removes the
snapshot's names; the restored process keeps the inode mapped, so its
memory mapping reads `.../snapshot/mem (deleted)` (reading R5). L3b
instead links the consumed memory file to `snapshot.base/mem` in place of
that removal, takes a Diff of the
restored guest, then builds the new image as a reflink copy of the base
(`cp --reflink=always`, btrfs) with the Diff applied by Firecracker's
`snapshot-editor edit-memory rebase`. The result is complete because every
page the Diff missed is a page unchanged since the restore, whose bytes
are the base's. Costs: the base stays on disk while the computer runs
(about 0.5 GB real, zstd), counted by the budget; one rebase per
hibernate. Gate: L3a's proof passed and a week of class:test Diffs with no
`snapshot_files_missing` or post-wake health failure. Needs
`snapshot-editor` on Node B (reading R4).

### L4: hibernate after short idle

Config only: `VMCTL_IDLE_TIMEOUT=30m` to `10m` in `nix/node-b.nix`.
Always-on classes (`critical_protected`, `public_platform`, paid primaries)
are unaffected because the sweep never idle-hibernates them. Gate: L3 on
for every class, a hibernate under 2 s with no PSI step (reading R6 and
L3's proof), RES-14 honest. `VMCTL_PRIMARY_KEEPALIVE_MODE=under-capacity`
stays: with cheap hibernates the reclaim path already hibernates
lower-ranked idle computers under pressure, and changing the keepalive
mode is a separate owner choice (section 5, Q1). Rollback: set 30m back.

### L20: snapshots that survive a guest-artifact deploy

**Mechanism.**
- The deploy installs guest artifacts into a release directory named by
  the guest image manifest digest,
  `/var/lib/go-choir/guest/releases/<digest>/` (`vmlinux`, `initrd`,
  `rootfs.ext4`, `storedisk.erofs`, `kernel-params`), with reflink copies
  of unchanged files, and points `/var/lib/go-choir/guest/current` at it.
  The stable paths in the vmctl environment become `current/…`.
- vmctl resolves `current` at launch and records the release digest in
  `VMConfig` and `SnapshotMeta`. Every bound-file identity rule stays.
- `expectedLaunchConfig` (`internal/vmmanager/snapshot.go:448`) builds the
  config against the snapshot's release when that directory still exists
  and is not below the release floor; otherwise against `current`, which
  mismatches and cold boots, as today.
- A computer resumed on an old release is marked behind; vmctl's deferred
  refresh sweep cold boots it onto `current` at its next idle moment (an
  idle computer has no open obligation, so this needs no restart marker).
- Retention: a release directory is kept while it is `current`, the
  previous, or named by an unconsumed snapshot younger than
  `VM_SNAPSHOT_MAX_AGE`. A bounded deletion pass in the existing disk
  retention sweep removes the rest. Reading R7 sizes a release.
- The **release floor** is a file the deploy writes. A snapshot of a
  release below the floor never loads. Firecracker version and host kernel
  changes still void every snapshot, as today.
- Flag `VM_SNAPSHOT_RELEASE_RETAIN=off` (default) | `on`; off keeps today's
  stable-path behaviour, and the release directories are inert.

**Files:** `internal/vmmanager/snapshot.go`, `config.go`, the deploy job
in `.github/workflows/ci.yml` and the guest install step it calls,
`nix/node-b.nix`, the disk retention sweep. FP8's jailer copies the store
image into each jail
([jailer-copies-store-image-per-vm-2026-10-21.md](../problems/jailer-copies-store-image-per-vm-2026-10-21.md)),
so the jailed path must resolve the same release; design detail waits for
FP8's landed shape.

**Ways it could fail:** resuming onto a release a security deploy meant to
retire (the floor; Q2); a retention pass deleting a release a snapshot
still names (the pass reads the metas, and a missing release only cold
boots, never loads wrong bytes, because the bound-file identities still
hash every file); the deploy's switch of `current` racing a launch (the
launch resolves the link once and binds the resolved paths).

### L20 revised 2026-10-26

L20 is superseded by its own design,
[density-d2-snapshots-survive-deploys-2026-10-26.md](density-d2-snapshots-survive-deploys-2026-10-26.md)
(station D, item D2; tier 2 review 306 answered in its §12). That file is
the one authority for L20; this section only points there. The main
differences from the shape above: releases live beside the guest symlink
in `/var/lib/go-choir/guest-releases/`, not under it, because the guest
path is now a symlink into the Nix store; the floor is a digest over the
floor sources and the build recipe, and the security epoch is enforced by
a CI check; and S6, the switch, is gated on a measured share of deploys
that keep the floor.

## 3. RES-22

The btrfs metadata half has its substrate fix already wired: tape
segments are on for every computer (`PLATFORM_TAPE_SEGMENTS=write`), so
new projection-batch pins no longer make one file each. Per CLAUDE.md
("check for an existing replacement"), FP10 does not add another packing
scheme. What RES-22 lacks:

1. **A reading that the growth stopped** (S4) and, if it has not, which
   event kinds still write legacy files. That goes to the tape segments
   goal, not here.
2. **An alert.** `go-choir-btrfs-headroom` (`nix/node-b.nix`) logs and
   balances but never fails. Slice: after its balance, exit non-zero when
   unallocated is under 20 GiB or metadata used is over 85% of metadata
   total, so the unit shows in `systemctl --failed` and in whatever FP0's
   B1 health script reads. Orange (host config), no guest effect. Rollback:
   revert.
3. **The slab.** Hypotheses only (the problem record). If S1 and S2 show
   `btrfs_inode` above about 3 GiB at the times guests boot or wake, and
   the rise follows a walker's window (S5), the first move is to take that
   walker off the tape (the repack shrinks restic's walk; the GC already
   skips the tape). `vm.vfs_cache_pressure=200` is the fallback knob, and
   it trades inode cache for page cache, which the paper's Dolt reads use;
   it lands only with a before-and-after reading of both. No change if
   slab is not near the guests' working set.

## 4. Readings for FP0 (Node B, read only)

Every command reads; none writes outside FP0's own terminal, touches a
guest block device, or prints a secret. Run as root where marked. `$VM`
is a test computer's VM id; never the paper, the owner's computer or the
delegate. Record outputs in the problem records.

**R1. Hibernate and wake outcomes, last 7 days.**
```sh
J='journalctl -u go-choir-vmctl --since "7 days ago" -o cat'
eval "$J" | grep -c 'by snapshot (fence='
eval "$J" | grep -oE 'snapshot hibernate of VM \S+ not possible \([a-z_]+' | awk '{print $NF}' | sort | uniq -c
eval "$J" | grep -oE 'resumed VM \S+ from snapshot' | wc -l
eval "$J" | grep -oE 'cold boot of VM \S+ after snapshot fallback \([a-z_]+' | awk '{print $NF}' | sort | uniq -c
eval "$J" | grep 'by snapshot (fence=' | grep -oE 'snapshot=[0-9]+ms|disk_bytes=[0-9]+|disk_bytes_method=\S+' | paste - - - | sort | uniq -c | sort -rn | head
```
For L20: the timestamps of the `launch_config_mismatch` and
`disk_identity_mismatch` fallbacks against the deploy times (FP0's CI
ledger): `journalctl -u go-choir-vmctl --since "7 days ago" -o short-iso | grep -E 'after snapshot fallback \((launch_config|disk_identity)_mismatch'`.

**R2. Can guest memory swap?** (root)
```sh
swapon --show
for pid in $(pgrep -x firecracker); do
  cg=$(awk -F: '$1=="0"{print $3}' /proc/$pid/cgroup)
  printf '%s %s swap.max=%s %s\n' "$pid" "$cg" "$(cat /sys/fs/cgroup$cg/memory.swap.max)" "$(grep VmSwap /proc/$pid/status)"
done
```
Every line must read `swap.max=0` and `VmSwap: 0 kB`. Repeat after FP8's
jailer is on for any computer.

**R3. Snapshot files on disk.** (root)
```sh
for m in /var/lib/go-choir/vm-state/*/snapshot/mem; do stat -c '%n size=%s blocks=%b x %B' "$m"; done
btrfs filesystem du -s /var/lib/go-choir/vm-state/*/snapshot 2>/dev/null | tail -n +1
```

**R4. Firecracker and snapshot-editor.**
```sh
fc=$(systemctl show go-choir-vmctl -p Environment | tr ' ' '\n' | sed -n 's/^VM_FIRECRACKER_BIN=//p')
"$fc" --version; ls "$(dirname "$fc")"
cat /sys/kernel/mm/transparent_hugepage/enabled /sys/kernel/mm/ksm/run /sys/kernel/mm/ksm/pages_sharing
```

**R5. Resident fraction of a guest** (root; once on a cold-booted test
computer after 30 idle minutes, once on a restored one; the journal's
`resumed VM $VM` line says which):
```sh
pid=$(pgrep -f "firecracker.*$VM" | head -1)
awk '/^[0-9a-f]+-[0-9a-f]+ /{m=$0} /^Size:/{sz=$2} /^Rss:/{if (sz>=1048576) print "size_kB="sz, "rss_kB="$2, m}' /proc/$pid/smaps
```
The large mapping is guest memory; `rss_kB/size_kB` estimates the Diff
file's share (the report estimates 0.15).

**R6. How soon hibernated computers wake** (gaps in seconds between a
hibernate and the same VM's next start):
```sh
journalctl -u go-choir-vmctl --since "7 days ago" -o short-unix \
 | grep -E 'hibernated VM \S+ by snapshot|resumed VM \S+ from snapshot|cold boot of VM' \
 | awk '{t=int($1); for(i=1;i<=NF;i++) if($i=="VM"){vm=$(i+1);break}
        if ($0 ~ /hibernated VM/) h[vm]=t; else if (vm in h) {print t-h[vm]; delete h[vm]}}' \
 | sort -n | awk '{a[NR]=$1} END{if(NR) print "n="NR, "p10="a[int(NR*.1)+1], "p50="a[int(NR*.5)+1], "p90="a[int(NR*.9)+1]}'
```
And `cat /proc/pressure/memory` sampled each second across one hibernate:
`for i in $(seq 20); do date +%T; grep some /proc/pressure/memory; sleep 1; done`.

**R7. A guest release's size.**
```sh
ls -ls --block-size=M /var/lib/go-choir/guest/
btrfs filesystem du -s /var/lib/go-choir/guest
```

**S1. Slab now.** (root)
```sh
grep -E '^(MemTotal|MemAvailable|Cached|Slab|SReclaimable|SUnreclaim):' /proc/meminfo
slabtop -o -s c | head -15
sysctl vm.vfs_cache_pressure
```

**S2. Slab across a day** (from FP0's machine, 26 h, every 10 minutes;
`$NODEB` is FP0's ssh target):
```sh
while :; do
  ssh "$NODEB" 'date -u +%FT%TZ; grep -E "^(MemAvailable|Slab|SReclaimable):" /proc/meminfo | tr -s " " | tr "\n" " "; sudo awk "\$1==\"btrfs_inode\"{print \"btrfs_inode_bytes=\" \$3*\$4}" /proc/slabinfo'
  sleep 600
done >> slab-$(date -u +%F).log
```

**S3. btrfs chunks.** (root)
```sh
btrfs filesystem usage -b / | grep -E 'unallocated|^Data|^Metadata'
journalctl -u go-choir-btrfs-headroom --since "7 days ago" -o cat
```

**S4. Tape file growth without walking the tape.** A btrfs directory's
size is the sum of twice each entry's name length, so two reads of
`st_size` 24 h apart give the net entries added, with no inode walk:
```sh
for d in pin-receipts computer-event computer-event-payload og; do
  stat -c "%n %s" /var/lib/go-choir/platform-artifacts/sha256/$d
done
ls -f /var/lib/go-choir/platform-artifacts/sha256/computer-event | head -3   # name length
```
Entries added ≈ Δsize / (2 × name length). Zero growth in the three tape
directories means segments took over.

**S5. The walkers' windows.**
`systemctl list-timers --all | grep go-choir`, read against S2's log: a
`btrfs_inode` step at 02:30 UTC points at the off-host backup; hourly
steps at the artifact GC; four-hourly at the disk retention sweep.

## 5. Owner questions (each has a default; none blocks the readings)

- **Q1 (L4).** Hibernate idle computers after 10 minutes? Default: 10m,
  only after L3 is on everywhere; `under-capacity` keepalive kept.
- **Q2 (L20).** May an idle computer wake on the previous guest release
  and update at its next idle moment, and who sets the release floor?
  Default: L20 ships off; when the owner turns it on, a deploy that
  changes the guest kernel or anything under the guest's confinement sets
  the floor, and any other deploy leaves old releases resumable for at
  most `VM_SNAPSHOT_MAX_AGE`.
