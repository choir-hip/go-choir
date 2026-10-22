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
  [platform-dolt-oldgen-grows-every-auto-gc-2026-10-22.md](platform-dolt-oldgen-grows-every-auto-gc-2026-10-22.md).
- **Holders:** VM state about 185 GiB (paper 80, candidate fleet 41, test
  computers about 60), nix store about 85 GiB,
  `dolt-backup-20261015-pre-2.4.2` 48.5 GB, swap file 16 GiB (its removal
  waits on tonight's backup proof), corpus-dolt 6.3 GB, builder 5.8 GB.

## Projection with no action (from Evidence 2)

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

The paper's Dolt journal grows about 1.6 GiB per 10 minutes (docs/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md). The online GC frees that inside the guest every few minutes, but the host image never gets the space back:

- Firecracker 1.16.1 has no virtio-blk discard. nix/node-b.nix says so where it deliberately leaves VM_DATA_DISCARD unset. So the guest's frees never reach the host.
- **Hypothesis, confirmed by Evidence 2 (15:00):** the guest filesystem keeps writing to blocks it never used before, so the host's sparse image grows until the whole cap is allocated. The guest reports 39.5 GB used while the host holds about 80 GiB.
- The real fix is RES-16 (data images that shrink: discard), which needs Firecracker 1.17. Node B's CPU has 39-bit physical addresses, and Firecracker 1.17 breaks virtio-mem hotplug on such CPUs (issue #6255; docs/problems/firecracker-1-16-1-virtio-mem-restore-bugs-2026-10-22.md).

## Consequences

- Every deploy is refused while free space stays under 90 GiB, so the whole landing queue is blocked. Deploy R failed this way.
- With no action, the root disk fills in about 5 hours, or a little later if the paper's image reaches its cap first. A full root disk stops every service and computer on the host, the paper included.

## Options (FP0's recommendation first)

1. **Lower the deploy floor** (DEPLOY_MIN_FREE_KIB) from 90 GiB to 30 GiB, so landings resume. A deploy's observed build footprint is about 3 GiB, so 30 GiB still leaves 10× headroom. This does nothing about the fill.
2. **Reclaim host space that nothing needs.** Candidates: stale VM state of long-stopped test computers (vmctl's retention prune: VMCTL_STALE_STATE_MIN_AGE=6h, VMCTL_STALE_STATE_MAX_DELETES=25 per pass), and nix store paths that only old CI deploy artifacts keep alive. Never the paper's or the owner's state.
3. **Stop the paper's image from growing.** Fast levers inside the paper: less journal growth (F-a batching is off for its drain bug; F-c is being built; GC cadence). Or a one-time compaction of the paper's data image: stop it, copy the live filesystem to a fresh sparse image, start it. That is red, touches the paper's store, and is the owner's call.
4. **Discard (RES-16)** for the paper, which needs a Firecracker that supports it on this CPU. That is the durable fix and is not quick.

## Pressure trigger design

Status: design, before code (lane fp0-disk-pressure). Mutation class: red
(VM lifecycle: it restarts the paper). It adds a second caller of the
existing compaction; it does not add a compaction path.

### What it does

A vmctl loop (`StartStoreCompactPressure`), every
`VMCTL_STORE_COMPACT_PRESSURE_INTERVAL` (default 5m), reads host free space
on the VM state dir's filesystem (statfs, `Bavail`) and, for each active,
unheld interactive **platform** computer (critical_protected,
public_platform: the sweep's own selection, `selectStoreCompaction` with no
size threshold), its data image `FileBytes` and `CapBytes`. A computer is
**due** when either:

- host free < `VMCTL_STORE_COMPACT_PRESSURE_HOST_FREE_GIB` (default 64 GiB), or
- `FileBytes` >= `VMCTL_STORE_COMPACT_PRESSURE_FILL_FRACTION` x `CapBytes`
  (default 0.6; the paper at 15:00 is about 0.64).

A due computer is **rate-limited** when its last compaction attempt (any
caller: this loop, the 04:00 timer, or a by-hand `compact-store`) is less
than `VMCTL_STORE_COMPACT_PRESSURE_MIN_GAP` (default 6h) ago. The last
attempt is persisted (`store-compact-attempts.json` beside the ownership
state) and written in the maintenance hook, after the guest accepted the
planned restart and before the compaction, so a vmctl crash mid-compaction
still counts and a restarted vmctl cannot compact again at once. An
unreadable attempts file refuses every pressure compaction (fail closed)
and says so. Guest refusals (busy, did not answer) are not attempts: they
cost a planned-restart request and are asked again at the next tick.

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
Work"). The 04:00 timer stays as it is.

### The host free-space floor (all callers)

`compactStore` refuses before it asks the guest for a planned restart when
host free space is below `VMCTL_STORE_COMPACT_MIN_HOST_FREE_GIB` (default
32 GiB; `0` turns the floor off). The reflink copy shares the original's
extents, but every block the compaction writes in the copy (the squashed
store and its GC output, up to the live store size: 12 to 28 GiB for the
paper today) is new host allocation until the rename drops the original.
Running it without that room fills the root disk and stops every service.
The refusal answers `skipped` with the reason
`host free <n> GiB below the compaction floor <m> GiB; the compaction needs room for the compacted copy`,
applies to the timer, the by-hand call and the pressure loop, and never
restarts the computer. It is a new refusal on the existing timer path:
fail closed.

### Failure modes, written before the code

1. Flapping: compaction, regrowth, compaction every tick. Bound: the
   per-computer gap (6h), persisted, counted from the attempt, any caller.
2. vmctl crash or restart during or right after a pressure compaction:
   the attempt was persisted before the compaction; the gap holds.
3. The attempts file cannot be read or written: no pressure compaction;
   the write failure fails the maintenance hook, so the computer boots its
   untouched image (the existing failure path).
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
  [node-b-deploy-disk-preflight-floor-2026-08-26.md](../evidence/node-b-deploy-disk-preflight-floor-2026-08-26.md))
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
[mission-mail-attachments…](../archive/mission-mail-attachments-and-app-cutover-2026-09-17.md) item 2;
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
  floor (32). A deploy then never starts a build that crosses `min-free`,
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
   shows `mode=observe interval=5m0s host_free_below=68719476736 fill_fraction=0.60 min_gap=6h0m0s attempts="/var/lib/go-choir/vm-state/store-compact-attempts.json"`.
2. Within 5 minutes, with the paper's image at or above 60% of its cap:
   `vmctl: store compaction pressure (observe): would compact vm-1669020a… (computer …): reason="data image NN% of cap (X GiB of 125.x GiB) >= 60%[; host free Y GiB below 64.0 GiB]" host_free=Y GiB`.
   If host free is under 32 GiB the line is `refused … the compaction needs room for the compacted copy` instead.
3. No action: no `store-compaction` planned-restart marker request in the
   paper's journal, no `vmctl: store compaction vm-…` outcome line, the
   paper's boot epoch unchanged, and no `store-compact-attempts.json` written.
4. Only then, by an owner/FP0 decision: `VMCTL_STORE_COMPACT_PRESSURE=on`
   (a nix change and deploy). The first `on` tick compacts the paper as a
   planned restart (`outcome=refreshed`, `planned_restart: true`), writes the
   attempts record, and later ticks log `rate-limited … next after …` for 6 h.

The floor applies now to the 04:00 timer as well: a timer run under 32 GiB
free reports the paper `skipped` with the floor reason in
the sweep's response (the by-hand `compact-store` answers HTTP 507) and does not restart the paper.

## Now (lane fp0-disk-pressure, 2026-10-22)

- Done: evidence and projection; platform-dolt oldgen record; trigger
  design; tests; trigger built (observe on staging via nix); deploy-floor
  proposal (docs only).
- FP0: land, deploy, run the observe proof above; decide `on`.
- Owner/FP0 decisions open: turning the trigger `on`; the coupled
  `min-free`/deploy-floor change; the first supervised Store A squash.
- Residuals: the gap is per VM ID (a re-created paper VM starts fresh);
  observe logs every tick while due (12 lines an hour); free space is
  `statfs` `Bavail` on btrfs, which over-reports when metadata is short
  (watch `btrfs filesystem usage` unallocated alongside).
