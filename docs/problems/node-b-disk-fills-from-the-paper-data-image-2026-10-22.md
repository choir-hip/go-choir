# Node B's root disk fills from the paper's data image; deploys are blocked by the disk floor

Date: 2026-10-22, 14:40 UTC. Found by: FP0, when deploy R (main 0bf8a097d, CI 39794075862) failed at "Deploy to Staging (Node B)". Status: open. Mutation class of the fixes: red (deploy routing; the paper's store and data image; host disk).

## Evidence (read-only, from Node B)

- Deploy R's log: `Deploy disk preflight failed: root filesystem still below required headroom`, with `root_available_kib_after_reclaim=93855960` against `min_free_kib=94371840` (90 GiB; `scripts/node-b-deploy-disk-preflight`). The preflight's own reclaim had just freed 5.7 GiB (149 store paths). Failure evidence: `/var/lib/go-choir/deploy-failures/39794075862-1.json`.
- Root filesystem `/dev/md126` (btrfs) is 476G. Available: 90 GiB at 14:22, 85,744,784 KiB at 14:32:42, 85,494,248 KiB at 14:33:42, 84,493,092 KiB at 14:37:03. It falls about 250 to 290 MiB a minute, so at that rate it reaches zero in about five hours.
- Who grows, per `du` over 90 s at 14:35:
  - The paper's state dir (vm-1669020a…) grew +260 MiB (78,570 → 78,830 MiB).
  - The candidate fleet, 71afbf14, the owner's computer and the FP2 test computer did not grow.
- The paper's data image: `data.img` holds 78,881 MiB on the host. vmctl's lookup reads file_bytes 82,721,660,928 and cap_bytes 137,438,953,472 (128 GiB). About 54 GB of headroom is left before the cap, and at about 130 to 170 MiB a minute the image reaches it in about 5 to 7 hours.
- Totals: VM state 180 to 185 GiB, the nix store about 84 GiB, the journal 222 MiB. There are only 4 system generations, so old generations offer little to collect.

## Evidence 2 (read-only, FP0, 14:50 to 15:00 UTC)

- **Root:** btrfs, 475 GiB, 72 GiB free. btrfs unallocated 38 GiB;
  metadata 84% used of 18.6 GiB. Free space falls about 200 MiB a minute:
  the paper's state dir about 140 MiB/min, platform-dolt about 45 MiB/min.
- **The guest's own view** (`/health` `persistent_disk`): 39.5 GB used of
  135 GB (29%); `dolt_gc` online, live 28 GiB, journal 787 MiB. On the host
  the paper's `data.img` holds about 80 GiB allocated (sparse, cap 125.7 GiB).
  So about 40 GiB of host allocation is blocks the guest has already freed.
  **This confirms the hypothesis below**: the image grows because the guest's
  frees never reach the host, not because the guest's data grows.
- **This morning's 04:00 sweep compacted the paper** (planned restart,
  busy allowed): 4786 commits squashed to 2, store 32.2 GB to 12.1 GB, 6m12s,
  `file_bytes` before 88,975,036,416. Since then the image regrew to about
  80 GiB in 11 hours. At the current rate it reaches its 125.7 GiB cap
  around 20:30 UTC and stays there until the next 04:00 sweep.
- **platform-dolt** (the host's Store A) is 36 GiB, almost all
  `noms/oldgen` `.darc` files. Auto GC runs about every 30 minutes and
  writes a new oldgen archive each time (a 25.7 GB file from 11:12, a 6.2 GB
  file at 14:54), so oldgen grows about 1.5 to 2.5 GiB an hour and nothing
  collects it. Own record:
  [platform-dolt-oldgen-grows-every-auto-gc-2026-10-22.md](../archive/problems/platform-dolt-oldgen-grows-every-auto-gc-2026-10-22.md).
- **Holders:** VM state about 185 GiB (paper 80, candidate fleet 41, test
  computers about 60), nix store about 85 GiB,
  `dolt-backup-20261015-pre-2.4.2` 48.5 GB, swap file 16 GiB (its removal
  waits on tonight's backup proof), corpus-dolt 6.3 GB, builder 5.8 GB.

## Evidence 3: the early compaction (15:35 UTC, owner approval)

The owner approved an early by-hand compaction of the paper. FP0's
evidence: at 15:35 the store went from 33,020,774,378 to 14,451,421,725
bytes in 6m11s, root hash unchanged; the paper's host state dir went to
45.5 GiB, and host free space to about 125 GiB. This is the measurement the
compaction floor is set from (§ The host free-space floor). The projection
below was taken before it and no longer describes tonight; the refill rate
(about 140 MiB a minute for the paper) still holds, so the image climbs
back toward its cap over the following day.

## Projection with no action (from Evidence 2, before Evidence 3)

- The paper's image stops growing at its cap (about 20:30 UTC), which takes
  about 140 MiB/min off the fall; platform-dolt keeps growing about 45 MiB/min
  (1.5 to 2.5 GiB/h).
- 72 GiB at 15:00, less about 46 GiB to the paper's cap, less about 20 to
  30 GiB of platform-dolt oldgen by 04:00: **the host is within a few GiB of
  full by 04:00 UTC**, exactly when the 04:00 sweep needs room to write the
  compacted store (`cp --reflink` shares extents, but every block the
  compaction tool writes inside the copy is new host allocation, up to the
  live store size, 12 to 28 GiB).
- Deploys are refused now (the 90 GiB floor); they stay refused all night.
- btrfs metadata is 84% of its chunks with 38 GiB unallocated; a full data
  allocation also starves metadata (the 2026-10-16 incident,
  [node-b-btrfs-metadata-exhausted…](node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md)).

## What the daily sweep cannot do

The 04:00 sweep is the only thing that gives the paper's host allocation
back (its `fstrim` on the loop-mounted copy punches holes in the host file).
Once a day is too slow when the image refills to about 80 GiB in 11 hours.
A pressure trigger that runs the same sweep when host free space or the
image's fill crosses a threshold is the bounded remedy until discard
(RES-16) lands; it is designed in
[§ Pressure trigger](#pressure-trigger-design) below.

## Cause (read in docs and code; the guest-side half is a hypothesis)

The paper's Dolt journal grows about 1.6 GiB per 10 minutes (docs/archive/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md). The online GC frees that inside the guest every few minutes, but the host image never gets the space back:

- Firecracker 1.16.1 has no virtio-blk discard. nix/node-b.nix says so where it deliberately leaves VM_DATA_DISCARD unset. So the guest's frees never reach the host.
- **Hypothesis, confirmed by Evidence 2 (15:00):** the guest filesystem keeps writing to blocks it never used before, so the host's sparse image grows until the whole cap is allocated. The guest reports 39.5 GB used while the host holds about 80 GiB.
- The real fix is RES-16 (data images that shrink: discard), which needs Firecracker 1.17. Node B's CPU has 39-bit physical addresses. An earlier line here said Firecracker 1.17 breaks virtio-mem hotplug on such CPUs (issue #6255). Corrected 2026-10-22: every version since 1.15.1 places the region the same way, so 1.17 changes nothing for hotplug on Node B, and hotplug does not work there today ([memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md](memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md)).

## Consequences

- Every deploy is refused while free space stays under 90 GiB, so the whole landing queue is blocked. Deploy R failed this way.
- With no action, the root disk fills in about 5 hours, or a little later if the paper's image reaches its cap first. A full root disk stops every service and computer on the host, the paper included.

## Options (FP0's recommendation first)

1. **Lower the deploy floor** (DEPLOY_MIN_FREE_KIB) from 90 GiB to 30 GiB, so landings resume. A deploy's observed build footprint is about 3 GiB, so 30 GiB still leaves 10× headroom. This does nothing about the fill.
2. **Reclaim host space that nothing needs.** Candidates: stale VM state of long-stopped test computers (vmctl's retention prune: VMCTL_STALE_STATE_MIN_AGE=6h, VMCTL_STALE_STATE_MAX_DELETES=25 per pass), and nix store paths that only old CI deploy artifacts keep alive. Never the paper's or the owner's state.
3. **Stop the paper's image from growing.** Fast levers inside the paper: less journal growth (F-a batching is off for its drain bug; F-c is being built; GC cadence). Or a one-time compaction of the paper's data image: stop it, copy the live filesystem to a fresh sparse image, start it. That is red, touches the paper's store, and is the owner's call.
4. **Discard (RES-16)** for the paper, which needs a Firecracker that supports it on this CPU. That is the durable fix and is not quick.

## Pressure trigger design

Status: built (ca135c22b), revised after FP0's round 2 review
(5c6b59894; § Round 2 below). Mutation class: red
(VM lifecycle: it restarts the paper). It adds a second caller of the
existing compaction; it does not add a compaction path.

### What it does

A vmctl loop (`StartStoreCompactPressure`), every
`VMCTL_STORE_COMPACT_PRESSURE_INTERVAL` (default 5m), reads host free space
on the VM state dir's filesystem (statfs, `Bavail`) and, for each active,
unheld interactive **platform** computer (critical_protected,
public_platform: the sweep's own selection, `selectStoreCompaction` with no
size threshold), its data image `FileBytes` and `CapBytes`. A computer is
**due** when its image holds at least
`VMCTL_STORE_COMPACT_PRESSURE_MIN_IMAGE_GIB` (default 32 GiB, the timer's
own minimum) and either:

- host free < `VMCTL_STORE_COMPACT_PRESSURE_HOST_FREE_GIB` (default 64 GiB), or
- `FileBytes` >= `VMCTL_STORE_COMPACT_PRESSURE_FILL_FRACTION` x `CapBytes`
  (default 0.6). The paper at 14:40 was 0.602 by vmctl's own numbers
  (82,721,660,928 of 137,438,953,472); the 0.64 written here before mixed
  FP0's `du` figure with the guest's 125.7 GiB. So the paper was right at
  the threshold, not above it.

**The gap, for every caller** (round 2): `compactStore` refuses (429,
`skipped`, "last compaction attempt …, next after …") when the computer's
last attempt (by this loop, the 04:00 timer, or by hand) is less than
`VMCTL_STORE_COMPACT_PRESSURE_MIN_GAP` (default 6h, read even when the loop
is off) ago; the refusal names `{"force":true}`. A by-hand
`{"force": true}` passes the gap and is logged (`forced past the gap`,
`forced=true`); nothing else passes it. The sweep reports a computer
inside its gap as skipped and does not fail. Two exceptions (round 3): the
timer is not held by a gap whose attempt failed (it retries), and an
attempt dated in the future is clamped to now (logged and persisted), so
it blocks for at most one gap.

**Back-off, pressure loop only** (round 3 N2): after a compaction that gave
back less than 8 GiB of host allocation, the loop waits until the image has
regrown 8 GiB past what that compaction left, however long that takes. It
is not a span of wall-clock time: a 24 h span aliased with the 24 h timer
(a 04:00:07 attempt made the next day's 03:59:54 run a silent 429). The
timer and by-hand calls are never backed off. A failed compaction records
no after-size, so it is never read as "reclaimed nothing".

The attempt is persisted (`store-compact-attempts.json` beside the
ownership state, times in UTC): `{vm_id: {at, file_bytes_before,
file_bytes_after, store_bytes_after, failed}}`. Before the guest is asked,
`compactStore` writes and removes a probe file beside the record and
refuses (507) if it cannot; the record itself is not touched. Inside the
maintenance hook (the VM is down) the attempt is kept in memory first and
then written, and a write failure there is logged and does not stop the
compaction. An unreadable record (including JSON `null`, an array, a
number or truncated JSON) refuses every pressure compaction. Only an
attempt recorded inside the hook (a compaction certain) replaces it: the
bad file is first moved to `<path>.corrupt`; if it cannot be moved it is
left in place and the error stays; if the move succeeds and the write
fails, it is moved back. Guest refusals (busy, did not answer) are not
attempts and never touch the record.

Modes, `VMCTL_STORE_COMPACT_PRESSURE`:

- `off` (default, and any unknown value, which is logged): the loop does
  not run.
- `observe`: logs one line per due computer and does nothing:
  `vmctl: store compaction pressure (observe): would compact vm-… (computer …): reason="data image 64% of cap (80.0 GiB of 125.7 GiB) >= 60%" host_free=… `
  and a rate-limited or floor-refused computer says that instead.
- `on`: calls the same `compactStore` the sweep calls, one computer at a
  time, as a planned restart with `allow_busy` for the platform classes.

Everything the compaction already refuses is unchanged and inherited:
held, not active, divergent, one refresh per computer at a time, one
compaction at a time, and the guest's refusal during a self-development
apply or while an engineering capsule is live (CLAUDE.md "Restarts End
Work"). Two things change for the 04:00 timer: the gap and the floor
apply to it, and a floor refusal fails its unit. A compaction never
crash-boots: a guest that cannot write the planned-restart marker defers
it (a plain deploy refresh of an idle legacy guest is unchanged).

### The host free-space floor (all callers)

`compactStore` refuses before it asks the guest for a planned restart when
host free space is below the floor, or cannot be read. The floor (round 3
N1) is `VMCTL_STORE_COMPACT_MIN_HOST_FREE_GIB` (default **24 GiB**; `0`
turns the floor off), raised to the computer's last compacted store size
(the tool's `bytes_after`, kept in the attempts record) plus 10 GiB when
that is larger. The compacted store grows about 5 GB a day (12.1 GB at
04:00, 14.45 GB at 15:35), so a fixed 24 GiB would lose its margin in
about two days; the derived floor keeps it. The reason names it:
`… below the compaction floor 30.0 GiB (last compacted store 20.0 GiB + 10.0 GiB) …`.
Before the floor check, under the compaction lock, `compactStore` removes
what an interrupted run left (its transient unit, mount and
`data.img.compacting`), so a stale copy cannot hold the host under the
floor forever (DeepSeek round 3, 5).

What the compaction writes (read in `internal/projectionbase/squash.go`):
a squash commit (small), then `DOLT_GC('--full', '--archive-level', '0',
'--incremental-file-size', 256 MiB)`, which writes one new copy of every
reachable chunk, that is the compacted store, before it drops the old
files. The copy is a reflink of the original, so the old files' blocks are
shared and freeing them in the copy (and its `fstrim`) frees nothing on
the host until the rename drops the original. Peak new host allocation is
therefore about the compacted store's size, not the live size the guest
reports: 14.45 GB (13.5 GiB) at 15:35 today. The floor is that plus about
10 GiB for the live set's growth (12.1 GB this morning, 14.45 GB this
afternoon), ext4 and btrfs metadata, and the rest of the host writing
during the run's six minutes (platform-dolt about 45 MiB a minute).

Why not 32 or 40 (the reviews): 32 was set from the live size (12 to 28
GiB) as if the GC wrote it; it writes the compacted size. 40 adds headroom
for a 28 GiB write that does not happen. Why not 0 to 16: 16 is barely
above one 13.5 GiB write. A per-caller split (lower for the timer) was
rejected: the write is the same whichever caller runs it.

The refusal answers `skipped` with the reason
`host free <n> GiB below the compaction floor <m> GiB; the compaction needs room for the compacted copy`
and never restarts the computer. **It is loud for the timer:** a sweep
with a floor refusal (or an unwritable attempts record) answers HTTP 507,
a sweep whose compaction failed answers 500,
`curl --fail-with-body` fails `go-choir-store-compaction.service`, the
journal carries the body, and vmctl logs
`store compaction sweep refused <vm> …`.

Not measured: the peak itself. Evidence 3 gives the output size and free
space after, no `df` during the run; "peak is about the compacted size" is
read from the code. FP0: sample `df -k /` and `btrfs filesystem usage /`
every 10 s through the next supervised compaction and put the peak here.

### Deadline and panics (round 3 N4, N5)

A compaction that has not finished in `VMCTL_STORE_COMPACT_TIMEOUT`
(default 45 min, at least 1 min; observed runs take 6 to 11 min) is ended:
its context is cancelled, the transient unit carries the same deadline as
`RuntimeMaxSec` so systemd stops a stuck tool, the error path stops the
unit, unmounts and removes the copy, and the computer boots its untouched
original image. The compaction lock is released, so later callers are not
answered 409 until a vmctl restart. A panic inside the maintenance hook is
recovered there and returned as an error, so the manager boots the image
the compaction left (the original until the rename) instead of unwinding
past the boot. A run past the guest's 15 min planned-restart window is a
crash boot (open work closed), as before.

Not measured: btrfs unallocated space. `statfs` `Bavail` can look healthy
while metadata chunks cannot grow (the 2026-10-16 incident); vmctl does not
read `btrfs filesystem usage`. Named residual.

### Failure modes, written before the code

1. Flapping: compaction, regrowth, compaction every tick. Bound: the
   per-computer gap (6h), persisted, counted from the attempt, any caller.
2. vmctl crash or restart during or right after a pressure compaction:
   the attempt was persisted before the compaction; the gap holds.
3. The attempts file cannot be read or written: no pressure compaction;
   an unwritable record refuses before the guest is asked (round 2; the
   first build failed the hook after the kill, a restart for nothing).
4. A compaction while the host has no room for the copy: the floor refuses
   first, for every caller.
5. A self-development apply or a live capsule: the guest refuses the
   planned-restart mark; the loop reports deferred and asks next tick.
6. An account's computer: never selected (platform classes only, the
   sweep's selection).
7. The timer and the loop at once: `storeCompactState.mu.TryLock` (one
   compaction at a time) and the per-computer deploy-refresh guard; the
   loser is deferred.
8. Observe mode doing anything: it never calls `compactStore`; a test pins
   no compactor and no marker call.
9. Free space unreadable (statfs error): treated as unknown; the free
   trigger does not fire, and the floor refuses (fail closed).
10. A stopped or held paper: not selected.

### Ceremony

- Conjecture delta: a pressure-driven compaction, at most every 6 h,
  keeps the paper's host allocation below its cap and Node B above the
  deploy floor until discard (RES-16) lands.
- Protected surfaces: vmctl lifecycle (planned restart of the paper), the
  paper's data image. Not touched: the tape, Texture canonical writes,
  auth, checkpoints, deploy routing.
- Admissible evidence: unit tests (fake compactor, fake marker, fake
  statfs) for selection, gap, persistence, floor, observe mode; on staging,
  observe mode first: the log line naming the paper and its reason, with no
  marker request, no restart, and no compaction.
- Rollback: `VMCTL_STORE_COMPACT_PRESSURE=off` (or unset) stops the loop;
  `VMCTL_STORE_COMPACT_MIN_HOST_FREE_GIB=0` removes the floor; revert the
  commits for the code.
- Heresy delta: discovered: none; introduced: none intended (no boot-time
  resumer; the restart is the existing marker-backed planned restart);
  repaired: none yet (the leak stays until discard).

## Deploy floor: what the preflight needs (proposal, docs only)

`scripts/node-b-deploy-disk-preflight` refuses a deploy unless root has
`DEPLOY_MIN_FREE_KIB` (90 GiB) free after its own nix reclaim.
Proposal, docs only; no number below changes in this lane.

### What a deploy writes (measured where the repo has it)

- **No CI log in the repo measures a deploy's disk footprint.** The repo's
  claims are "single-digit GiB" (the preflight's own comment and
  node-b-deploy-disk-preflight-floor-2026-08-26.md)
  and FP0's "about 3 GiB" above; neither cites a `nix path-info -S` or a
  `df` before and after. So the exact figure is **unknown**.
- The one measured component: each deploy adds a system generation whose
  closure roots a new `go-choir-guest-image` of **3.26 GB**
  ([artifact-store-grows-12gb-a-day-2026-10-14.md](artifact-store-grows-12gb-a-day-2026-10-14.md)).
  The host Go binaries and the rest of the system closure delta are
  smaller and not measured. A bounded estimate: 4 to 8 GiB per deploy, plus
  the prebuild's results for the same commit (the same paths, shared).
- The measurement to take on the next deploy (FP0, read-only):
  `df -k /` before the prebuild and after the switch, and
  `nix path-info -S` of the new system and guest image.

### The binding constraint is nix `min-free`, not the footprint

`nix/node-b.nix` sets `min-free = 80 GiB`, `max-free = 100 GiB`. When free
space drops below `min-free` during a build, nix runs a store GC inside the
build, and that GC deletes the build's own unrooted inputs ("not a valid
path"; the deploy looped on 2026-09-17,
mission-mail-attachments… item 2;
ci.yml's prebuild runs `nix store gc` first for the same reason). The
90 GiB floor is above `min-free` plus a deploy's footprint, which is why it
holds. **Lowering only `DEPLOY_MIN_FREE_KIB` to 30 GiB (option 1 above)
brings back mid-build GC deletions**: every build on a host with less than
80 GiB free would GC itself.

### Proposed sizes (both move together)

- `min-free` = 24 GiB, `max-free` = 40 GiB (nix: GC only near real
  exhaustion, and only to 40 GiB).
- `DEPLOY_MIN_FREE_KIB` = 48 GiB: after a deploy's footprint (bound 8),
  40 GiB remain, above both nix's `min-free` (24) and the store compaction
  floor (24). A deploy then never starts a build that crosses `min-free`,
  and never takes the room the compaction needs.
- Order: lower `min-free` first (it ships in the deploy itself, so it
  applies after the switch), then the preflight floor in the next commit.
  Both are red (deploy routing). Rollback: revert either number.
- Until the paper's image is bounded (pressure trigger on, or discard),
  free space falls about 200 MiB a minute, so 48 GiB is about 2 hours of
  the fall below today's 72 GiB: the lower floor buys landings, not runway.

## Pressure trigger: staging proof (FP0's steps, observe mode first)

Built on lane `lane/fp0-disk-pressure` (`internal/vmctl/store_compaction_pressure.go`;
nix sets `VMCTL_STORE_COMPACT_PRESSURE=observe`). After the deploy:

1. vmctl's start line: `journalctl -u go-choir-vmctl | grep 'store compaction pressure mode='`
   shows `mode=observe interval=5m0s host_free_below=68719476736 fill_fraction=0.60 min_image=34359738368 min_gap=6h0m0s attempts="/var/lib/go-choir/vm-state/store-compact-attempts.json"`.
   (Watch the gap and floor lines too: `rate-limited … next after …; a by-hand {"force":true} passes the gap`.)
2. Within 5 minutes, with the paper's image at least 32 GiB and at or
   above 60% of its cap (after the 15:35 compaction that is some hours
   away; host free about 125 GiB will not trigger):
   `vmctl: store compaction pressure (observe): would compact vm-1669020a… (computer …): reason="data image NN% of cap (X GiB of 125.x GiB) >= 60%[; host free Y GiB below 64.0 GiB]" host_free=Y GiB`.
   If host free is under 24 GiB the line is `refused … the compaction needs room for the compacted copy` instead;
   within 6 h of the 15:35 compaction (if it was recorded; it predates
   this code, so it was not) the line is `rate-limited …`.
3. No action: no `store-compaction` planned-restart marker request in the
   paper's journal, no `vmctl: store compaction vm-…` outcome line, the
   paper's boot epoch unchanged. Observe never writes
   `store-compact-attempts.json`; the 04:00 timer's run (or a by-hand
   call) creates it, but only when it gets past the floor and the gap: a
   floor-refused timer never reaches the record.
4. Only then, by an owner/FP0 decision: `VMCTL_STORE_COMPACT_PRESSURE=on`
   (a nix change and deploy). The first `on` tick compacts the paper as a
   planned restart (`outcome=refreshed`, `planned_restart: true`), writes the
   attempts record, and later ticks log `rate-limited … next after …` for 6 h.

5. The timer's floor refusal, if host free is ever under the floor at
   04:00: `systemctl status go-choir-store-compaction` failed, its journal
   holding the 507 body with the floor reason (500 with the error for a
   failed compaction).

### Sequencing on Node B tonight (both reviewers; projections, not measurements)

From the record's own rates (about 125 GiB free at 15:35; paper refill
about 140 MiB/min until its cap, platform-dolt about 45 MiB/min):
free crosses the 90 GiB deploy preflight at about 18:50 UTC, the 64 GiB
trigger at about 21:15, the 24 GiB floor at about 00:55; the paper's
image reaches its cap at about 01:20 with about 19 GiB free; the 04:00
timer then meets about 12 GiB and is refused (507, unit failed); the host
fills at about 08:30 from platform-dolt alone. The observe line itself
appears only around 19:15, after the deploy floor has closed. So observe
mode cannot be followed by an `on` deploy tonight unless one of these
happens first (FP0's and the owner's choice):

- ship `on` in the same deploy as this code, before about 18:50 UTC;
- land the coupled `min-free`/deploy-floor change first;
- or run a by-hand compaction before about 00:55 UTC, while the floor
  still passes (the 15:35 run is not in the record, so no gap and no
  `force` are needed).

**Superseded by measurement (FP0 sampler, 18:02 UTC):** Node B had 172 GB
free at 18:02 and free space was flat for the preceding hour. The times
above assumed the 15:00 fall rates (about 185 MiB/min) continued after
15:35; they did not, so none of those crossings (18:50, 21:15, 00:55,
04:00, 08:30) holds, and nothing forces the `on` decision tonight. Why the
fall stopped is not established here (not read from a trace). The next
projection should start from the sampler's series, not from the 15:00
rates.

## Round 2 (FP0's review of 4515538eb; fixes 8873d9c52 tests, 5c6b59894 code)

| # | Finding | Disposition |
|---|---|---|
| 1 | JSON `null` record: nil map, `record` panics in the hook after the kill, paper left down | Fixed: null/array/number/truncated are load errors; no write into a nil map; tick under `recover` |
| 2 | Unwritable record: a planned restart with no compaction, every mode | Fixed: rewrite check before the guest is asked (507); in the hook the attempt is kept in memory and the compaction proceeds |
| 3 | Floor 32 GiB unmeasured, timer refusal silent | Fixed: 24 GiB from the measured compacted-copy write (above); timer refusal is HTTP 507, unit fails |
| 4 | Host-free trigger restarts tiny images every 6 h; no back-off | Fixed: 32 GiB minimum image; under 8 GiB reclaimed makes the next gap 24 h |
| 5 | Gap only in the pressure loop | Fixed: in `compactStore` for every caller; by-hand `force` passes it |
| 6 | Legacy guest + idle `/health` crash-boots with maintenance | Fixed: deferred when maintenance is set |
| 7 | No coordination with a deploy build | Residual: vmctl has no host signal for a build in flight (CI serializes with GitHub concurrency groups; no host lock). A pressure compaction (up to about 14 GiB written) during a deploy build can still push free space under nix `min-free`. Next step if it matters: the deploy writes a host marker file vmctl reads |
| 8 | Corrupt record's error sticky | Fixed: next successful write keeps `<path>.corrupt` and clears it |
| 9 | Surviving mutants M8, M9d, M11/M12, M13, M16b | Fixed: one test each |
| 10 | Nits | 0.602 corrected; timer-change contradiction removed; doc comment restored; `compaction_error` in the log; interval minimum 30 s; full rollback stated; the yellow commits 0eef6382c and 8873d9c52 are compile-red until their code commit; round 1 added 15 test functions, not 16 |

Full runtime rollback: `VMCTL_STORE_COMPACT_PRESSURE=off` and
`VMCTL_STORE_COMPACT_MIN_HOST_FREE_GIB=0`. The gap then still applies to
the daily timer (harmless at one run a day); reverting 5c6b59894 and
ca135c22b removes it.

## Round 3 (Sonnet and DeepSeek reviews of 440f8ee48; tests 06c476dce, code in the red commit after it)

| # | Finding | Disposition |
|---|---|---|
| S-N1 / D-4 | Fixed 24 GiB floor erodes as the compacted store grows | Fixed: floor = max(configured, last `bytes_after` + 10 GiB); 0 still off |
| S-N2 / D-7 | 24 h back-off aliases with the 24 h timer; longer than the refill | Fixed: back-off is regrowth-based (8 GiB past the last after-size), pressure loop only; timer and by-hand never backed off |
| S-N3 | Mutants S2 (back-off whatever the reclaim) and S4 (after-size recorded on failure) survive | Fixed: `TestStoreCompactPressureLargeReclaimKeepsSixHourGap`, `TestStoreCompactFailedCompactionRecordsNoAfter`; C2 (floor boundary) killed by `TestStoreCompactFloorBoundaryPasses` |
| S-N4 | A hung compaction leaves the paper down and the lock held | Fixed: 45 min deadline, `RuntimeMaxSec` on the unit, original image boots, lock released |
| S-N5 / D-2 | A hook panic unwinds past the boot | Fixed: `recover` inside the hook returns an error; the manager boots the original |
| S-N6 / D-3 | A future-dated attempt silently blocks the timer | Fixed: clamped to now (logged, persisted); attempt times stored UTC |
| S-N7 | `force` not named in the 429, not logged; main.go "refuses every compaction" | Fixed: 429 names `{"force":true}`; `forced past the gap` and `forced=true` logged; main.go says "every pressure compaction" |
| S-N8 | Corrupt-record heal before a compaction is certain; rename unchecked | Fixed: the preflight writes a probe file only; the record is replaced only inside the hook; a failed move leaves it in place; a failed write after a move moves it back |
| S-N9 / D-1 | Observe-then-`on` cannot happen tonight before the deploy floor closes | Docs: § Sequencing on Node B tonight; superseded by FP0's 18:02 sample (172 GB free, flat for an hour) |
| D-5 | A stale `.compacting` copy can hold the host under the floor | Fixed: removed under the lock before the floor check |
| D-6 | A failed attempt holds the timer for 6 h | Fixed: the timer retries after a failed attempt; the loop and by hand keep the gap |
| D-8 | The sweep fails the unit only on 507 | Fixed: a failed compaction answers 500 |
| D-9 / S nits | The focused `-run` pattern missed two tests; proof step 3 | Fixed: three tests renamed to start with `TestStoreCompact`; step 3 says a floor-refused timer never writes the record |
| S-F7 | Gap keyed by VM ID | Residual, unchanged |

## Now (lane fp0-disk-pressure, 2026-10-22)

- Done: evidence and projection; platform-dolt oldgen record; trigger
  design; tests; trigger built (observe on staging via nix); deploy-floor
  proposal (docs only); round 2 review findings fixed (7 is a residual);
  round 3 findings fixed before `on`.
- FP0: land, deploy, run the observe proof above; decide `on` and its
  sequencing tonight (§ Sequencing); sample `df` through the next
  supervised compaction.
- Owner/FP0 decisions open: turning the trigger `on`; the coupled
  `min-free`/deploy-floor change; the first supervised Store A squash.
- Residuals: no deploy-in-flight coordination (finding 7); btrfs
  unallocated not read; the 15:35 compaction is not in the attempts record;
  the gap is per VM ID (a re-created paper VM starts fresh);
  observe logs every tick while due (12 lines an hour); free space is
  `statfs` `Bavail` on btrfs, which over-reports when metadata is short
  (watch `btrfs filesystem usage` unallocated alongside).
