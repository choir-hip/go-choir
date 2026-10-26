# New-computer creation takes 14 seconds (2026-10-26)

Recorded by FP0 from a read-only Sonnet investigation, 2026-10-26 02:2x. Status: open, documented before any fix. The owner, creating the d@e.com account: "it took like 10 seconds to make that computer ... it should be instant to fork the base image." A fix is red (vmctl, boot path), tier 2.

Recommended order, not yet decided: (1) reflink a prebuilt seeded data.img template (about 4 s, no guest change); (2) trace and trim rt.Start and the 800 ms boot-receipt poll (up to about 1.9 s); (3) design a Firecracker snapshot of a booted base restored at signup, with identity delivered after restore (about 1 to 2 s total; a guest change, and the same identity rules as forks); no standing warm pool. S9 forks (docs/definitions/choir-appdev-s9-forks-and-fleets-2026-10-01.md) were reviewed but never built.

## Investigation


Read-only investigation. Nothing in the repo or on Node B was changed. Source of the numbers:
`journalctl -u go-choir-vmctl/-auth/-proxy` on `node-b`, and the per-boot receipt
`/var/lib/go-choir/vm-state/vm-70070cf7337d45f2d46eafc83b9783b9/boot-timeline.json`
(host marks + the guest's own receipt, written by `internal/vmmanager/boot_timeline.go`),
plus that VM's `console.log` and `fc-config.json`. No du/find on the artifacts root, no token read.

The account: user `c061de6f-e4ea-4b28-b126-aa2f066dd890` (email hash 850fffdd), computer
`computer-59c902c5d46caecc0a180a8557164e72`, VM `vm-70070cf7337d45f2d46eafc83b9783b9`, epoch 13301,
class `primary`, 8192 MiB / 4 vCPU, jailer on, host kernel 6.18.57, Firecracker 1.17.0, guest commit a03d5266.

## Headline

Measured wall time from `register_finish` (01:26:06.668) to vmctl logging "assigned VM" (01:26:21.046):
**14.4 s**. Admission to assigned is 14.34 s. Not 10 s. (The owner's ~10 s is probably the visible
part after the page painted, or a different clock; the five creations in the journal since 10-19 all
sit at 13.6 to 16.1 s: 13.63, 13.83, 14.40, 16.12, 14.34.)

Provisioning starts 38 ms after the passkey ceremony finishes (the browser fires the first
authenticated request, `/api/shell/bootstrap`, which resolves the computer). There is no queue and
no idle wait. All 14.3 s is work on the critical path.

## Timing table

M = measured (a timestamp pair in a log or in the boot receipt). I = inferred (arithmetic or code
reading, no direct timestamp). Times are host UTC on 2026-10-26; offsets are from `boot_begin`
(01:26:06.755) unless stated.

| # | Step | Start -> end | ms | M/I | Evidence |
|---|------|--------------|----|-----|----------|
| 0 | Passkey ceremony done; first proxied request reaches vmctl resolve | 06.668 -> 06.707 | 38 | M | auth `register_finish challenge_consumed` 01:26:06.667; vmctl `admission decision=admit` 01:26:06.706 |
| 1 | Identity: VM id, ComputerID (`stableComputerID`, a deterministic hash), host-memory admission, epoch reservation, ownership persisted, recovery pre-check, gateway token, computer credential envelope from corpusd (idempotent, keyed by realization+epoch) | 06.707 -> 06.755 | 49 | M (as a block) | vmctl admission line -> timeline `boot_begin`. Sub-steps not individually timed. |
| 2 | Host port/subnet, state dir, epoch consume | 06.755 -> 06.759 | 4 | M | marks `host_port_allocated`, `boot_epoch` at +3 ms |
| 3 | **Data image: `git clone /opt/go-choir` + `git checkout <commit>` into a staging dir, then `truncate` a 32 GiB file, `mkfs.ext4 -F -d <staging>` (copies ~3,870 files, 100 MiB of packs), fsync** | +3 -> +4066 | **4,063** | M total, split I | mark `data_image_ready` at +4066 ms. Existing computers pass the same mark in 3 to 99 ms (their image exists). Result: 430,977,024 bytes allocated of 34,359,738,368. Split between clone, mkfs and fsync not measured. Code: `internal/vmmanager/manager.go:986-1017`, `createDataImage` :3122, `sourceSeedStagingRoot` :3139-3178 (git clone :3162, checkout :3168), `createSparseDataImageSeeded` :3181-3221. `/opt/go-choir`: 3,870 tracked files, 99.7 MiB packs (`git count-objects -vH`, read-only). **bootVM holds the global `m.mu` for this whole step** (manager.go:887-890 lock, released only at :1128 after launch), so concurrent signups serialize on it. I (by code read). |
| 4 | Credential disk: build a small ext4 (`mkfs.ext4 -d`) with envelope, gateway token, grants | +4066 -> +4074 | 8 | M | `credential_disk_ready` +4074. manager.go:1083, :3033 |
| 5 | Jailer + Firecracker launch: uid alloc, tap device, jail root, shared kernel/initrd/store (reflink or shared copy), hardlink data/credential disks, cgroup, `fc-config` push | +4074 -> +4236 | 162 | M | `firecracker_spawned` +4236; vmctl: "allocated VMM uid" 06.831 (it is 4.12 s after admission), "jailed firecracker" 06.991. Code: manager.go:1123, :2236; confinement.go:663 |
| 6 | Guest kernel start and 26 MB initrd unpack until initrd systemd is up | 11.05 -> ~12.83 (host) | ~1,800 | M (console.log timestamps: Firecracker "started microvm" 01:26:11.053, first block-device notify 01:26:12.83) | `console.log` |
| 7 | Initrd: find `/dev/disk/by-label/nix-store` (3.5 GB erofs), mount `/sysroot`, mount `/nix/store`, find closure, NixOS activation, switch root | ~12.83 -> ~16.15 | ~3,300 | M (console.log: stage-2 systemd starts at 01:26:16.149) | guest `systemd_analyze initrd_s=3.136`, `total_s=6.841` |
| 8 | Stage-2 systemd: udev, journal flush, firewall, networkd, signer units, kernel capability probe, updater, `go-choir-autoputer.service` started | ~16.15 -> 18.19 | ~2,040 | M | guest units: first unit at monotonic 6,192 ms, `multi-user.target` 6,840 ms, autoputer process main at uptime 7.12 s = 01:26:18.193 |
| 9 | Autoputer process: open runtime store (Dolt), credential exchange with corpusd, build runtime, HTTP listen | 18.193 -> 18.965 | 772 | M | guest marks `runtime_store_open` +542, `credential_exchange_done` +748, `http_listen_ready` +772 |
| 10 | Replay phase (empty tape, genesis): reconstruct, vocab fence, record indexes | 18.963 -> 18.989 | 26 | M | marks `replay_begin` +770 -> `replay_done` +796 (`applied_rows 0`) |
| 11 | `rt.Start` and file-tree hydrate: crash-close sweeps, platform-update resume, source schedules, projector start, etc. | 18.989 -> 20.463 | **1,474** | M total, content I | marks `lifecycle_reconciled` +796 -> `runtime_started` +2270 (autoputer run.go:830-841, `afterReplay` at :678-701). Which phase inside costs the time is not measured here; the guest logs "runtime: boot phase end ... dur=" (runtime.go:915-917) but those lines are in the guest journal, not in the receipt. |
| 12 | Host sees `/health` 200 | -> 19.149 | (+8,157 after spawn) | M | `first_healthy` +12,393 (host poll, 250 ms interval, manager.go:2932) |
| 13 | Host waits for the guest receipt to show `runtime_started`, polling every 800 ms | 19.149 -> 21.044 | **1,895** | M | `guest_receipt_fetch_begin` +12,393, `awaiting_runtime_started` +12,507, `fetch_done` +14,289. Code: boot_timeline.go:230-300 (`time.Sleep(800ms)`), called at manager.go:1175 before `BootVM` returns. The guest reached `runtime_started` at 20.463, the host noticed at 21.044: about 580 ms of pure poll slack on top of step 11. |
| 14 | vmctl flips ownership active, notifies waiters, logs "assigned VM" | 21.044 -> 21.046 | 2 | M | vmctl 01:26:21.046 |

Sum check (host clock): 38 + 49 + 4 + 4,063 + 8 + 162 + 8,157 (steps 6-12 inclusive of guest) + 1,895
= about 14.4 s. The guest's seven-plus seconds of OS boot is steps 6-8 (about 7.1 s measured, equal to
guest uptime at process start); steps 9-11 add another 2.3 s inside the guest.

Rolled up:

| Phase | Seconds | Share |
|-------|---------|-------|
| Host prep before the VMM exists (steps 0-4) | 4.16 | 29% (of which data image 4.06) |
| Jailer/Firecracker launch (5) | 0.16 | 1% |
| Guest OS boot to autoputer process (6-8) | 7.1 | 49% |
| Autoputer init to listening (9-10) | 0.8 | 6% |
| Runtime start and host's wait for it (11, 13) | ~1.9 beyond health | 13% |
| Everything else | <0.1 | <1% |

The same boot-receipt pattern on already-existing computers (refresh boots 00:33-00:36 today and
10-22): spawn -> healthy is 8.2 to 14.5 s, `systemd_analyze total_s` 6.2 to 6.5 s, data image 3 to 99 ms.
So guest boot (steps 6-8) is a fixed ~6.5 to 7 s cost that every cold start pays, new or not; the new
account adds only the 4.06 s data-image build.

## Top three costs

1. **Guest OS cold boot, about 7.1 s measured (steps 6-8), plus 0.8 s autoputer init.** Kernel + 26 MB
   initrd 1.8 s, initrd stage 3.3 s, stage-2 systemd 2.0 s. Nothing here is specific to the new
   account.
2. **Data image build, 4.06 s measured (step 3), done serially under the global manager lock.** It
   rebuilds the same 32 GiB ext4 with the same ~3,870-file source seed for every account. Nothing in it
   is per-owner. (The split between git clone, mkfs and fsync is inferred, not timed.)
3. **Runtime start and the host's polling wait, about 1.9 s measured (steps 11, 13).** 1.47 s is
   `rt.Start` and its sweeps (which phase is unmeasured), and about 0.58 s is the 800 ms poll
   granularity of `fetchGuestBootTimeline`, which gates `BootVM` returning. The wait exists to
   record the guest receipt, not to make the computer usable. It is observation on the critical path.

## Options, per cost

Existing machinery that already helps (all in the repo, none wired to creation):
- Node B's vm-state is on **btrfs** (`stat -f` = btrfs, mount shows `compress=zstd:3`); `cloneFile`
  (FICLONE) is at `internal/vmmanager/launch_linux.go:152` and `placeReadOnlyCopy` (reflink, copy
  fallback) at `confinement.go:663`. The manager already comments "allocated bytes show reflink
  extent" (manager.go:1021).
- **Firecracker snapshot hibernate/resume is deployed** (`VM_HIBERNATE_MODE=snapshot`,
  `VM_SNAPSHOT_TYPE=full` in the vmctl unit; `snapshot_lifecycle.go:37` hibernate, `:440`
  `ResumeVMFromSnapshot`). Measured elsewhere: `wake_to_healthy=351ms load=8ms credential=delivered`
  (docs/problems/hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md, staging
  06:32:54 line) and 444 / 458 ms in other records. Post-resume it already delivers a fresh credential
  envelope (`guestHibernateResumed`, snapshot.go:1022) and steps the guest clock.
- Not present: any warm pool, base template, or fork constructor. S9 (forks and fleets,
  docs/definitions/choir-appdev-s9-forks-and-fleets-2026-10-01.md) is reviewed/accepted on paper
  and never built. grep for prewarm/warm pool/template finds only the doctrine sentence that "post-auth
  prewarm may start immediately after register/login" (docs/runtime-invariants.md:50), the capsule
  doc, and the CPU report's idea C15 (a preforked desk-worker pool, unrelated).

### Option for cost 2 (data image): reflinked seeded template. Smallest, safest, do first

Build one seeded `data.img` template per source commit (the existing `createSparseDataImageSeeded`
output, built once in the background after vmctl starts or after a deploy changes
`buildinfo.Commit`), then `FICLONE` it into `vm-state/<vm>/data.img`. Same btrfs filesystem, so the
clone is a metadata operation (expected single-digit to low tens of ms; inferred, not measured on Node
B; the existing snapshot code does the same clones). Saves about 4.0 s and removes the global-lock
hold.
Gates to respect:
- The template must contain no identity. Today's seed is a public git checkout and an empty ext4, so
  it is clean. Anything the guest writes at first boot (signer state under `/mnt/persistent/choir-signers`,
  privacy key, store genesis) must still be created per computer after the clone, never in the template.
  Add a check that scans the template for key material before it is promoted.
- ext4 filesystem UUID/label are shared across clones; fine for separate VMs but never mount two
  clones in one guest. The template needs a rebuild trigger on commit change (the seed is "the commit
  this vmctl binary was built from", manager.go:3161-3168), and a guard that refuses a stale template.
- Mutation class: red (VM lifecycle, boot path). One-act rollback: an env switch back to the current
  per-VM mkfs path (keep it as the fallback when the template is absent). Tier 2 review per CLAUDE.md
  (boot path), not tier 1.
- Restart rules: not touched (this is first creation, not a restart).

### Option for cost 3 (runtime start + poll wait): stop gating `BootVM` on the receipt

Return from `bootVM` at `first_healthy` plus a bounded wait (or a 100 ms poll) and let
`fetchGuestBootTimeline` finish in the background to write the receipt. Alternatively shorten the
poll to 100 to 150 ms. Saves up to ~0.6 s (poll slack) and up to ~1.9 s if the receipt wait is
decoupled, but only if `/health` 200 already means "safe to serve". The `replayHealthGate`
(run.go:670-701) holds `/health` at 503 until replay finishes, but `rt.Start` runs after that gate in
`afterReplay`; the runtime-started wait may be what guarantees genesis/lifecycle reconcile is done
before the first user write (docs/problems/s0b-registration-computer-missing-genesis-2026-10-04.md
fix shape 2 and 3). Verify that before decoupling; otherwise keep the gate and just fix the poll
interval. Class: orange (observation path) unless it moves a readiness gate, then red. Also measure
which `rt.Start` phase costs 1.47 s using the guest's "boot phase end ... dur=" lines before touching it
(Convergence Before Patching: trace before attributing).

### Option for cost 1 (guest OS boot): do not boot at claim time

Two candidate designs, in increasing ambition. Neither is built; both are new red work.

A. **Firecracker snapshot of a booted, unclaimed base, restored per new computer.** Boot one base
guest once per guest-image/commit to the point just before any per-computer identity exists
(after stage-2 systemd and the autoputer binary is loaded, but with signer units, credential exchange,
store open and genesis deferred to claim). Snapshot it. A new account then: `FICLONE` the template data
image (option 2), `FICLONE` the snapshot memory file, load it (load=8 ms, wake ~350 to 450 ms measured
for hibernate resumes), deliver identity at claim. Expected creation time: roughly 1 to 2 s total
(inferred: 0.4 s resume + store open/credential exchange 0.75 s + genesis + runtime start 1.5 s unless
that is trimmed). Memory cost per resumed computer is the touched working set (1.15 to 1.37 GiB of 8,
measured in R5 of the same problem doc), not a standing pool.
What it must respect:
- **Fork re-keying before any execution** (S9 acceptance: distinct ComputerID, signer keys, privacy
  key, gateway token before any child runtime or network access). The base snapshot must contain *no*
  identity material in memory or disk: no signer keys (the guest receipt signer and verifier signer
  currently start at boot, journal units at monotonic 6,262 to 6,456 ms, and keep state under
  `/mnt/persistent/choir-signers/...`), no privacy key (comes from the CHOIR_CRED credential disk,
  `/mnt/persistent/choir-credentials/privacy-key`), no gateway token, no owner/computer id (today
  these are on the kernel cmdline: `choir.computer_id`, `choir.owner_id`, `choir.realization_id`,
  `vm_id`, `epoch`, `ip`). Each of these has to move from boot-time cmdline/credential-disk to a
  post-restore claim call, the way `guestHibernateResumed` already delivers an envelope. That is a real
  guest change (autoputer reads identity at process start, run.go:163-206 and :485).
- **Snapshot uniqueness hazards**: guest RNG/entropy and VMGenID after restore, machine-id, TLS/ssh
  host keys, clock step (the resume path already steps the clock: `clock_step=8556ms`), network identity
  (the IP and tap name are baked into the cmdline `ip=` and the guest state; every clone needs its own
  /30 and tap, so the claim must reconfigure the guest network or each clone needs its own netns). The
  snapshot is also bound to the launch config hash and the exact Firecracker binary (a deploy that
  changes the guest image or Firecracker version voids it: problem doc L20), so the base must be rebuilt
  per deploy and a stale base refused, exactly as `validateSnapshot` already does for hibernates.
- **Never resume a base against a drifted or consumed disk** (S3's one-shot consumption and
  paired-disk identity check, snapshot.go:429-473). A base must be clone-once-per-use, never loaded in
  place.
- Restart rules: a crash restart never resumes work; this does not resume work, it creates a fresh
  computer, so the marker rules are not engaged. The pool/base itself holds no owner work. A restored
  base that crashes is simply discarded. No boot-time resumer without a planned-restart marker is added.
- Security gate set: red, tier 2 (two seats), plus the S9 offline scan for parent identity material
  in the child disk and memory file.

B. **Warm pool of pre-booted unassigned computers, claimed and re-keyed.** Keep 1 to 3 fully booted
"blank" computers (full guest boot paid in the background) and hand one to the next signup, delivering
identity through the same claim call. Creation time becomes the claim cost (about 1 to 3 s, inferred),
but it needs the same guest changes as A (identity after boot), holds 2 GiB of admission (need_mib
default) to 8 GiB of RAM per spare permanently, and Node B's headroom at the creation time was
available_mib 10,307 against a floor of 4,779. It trades standing RAM for latency, against the owner's
"resource use ahead of short-term uptime" ruling, and it has the identity-in-memory problem (blank
computer has run signer services). A is strictly cheaper in RAM and gives the same latency, so prefer A
if guest-side claim lands; B is only a fallback if snapshot restore of a *base* proves unsafe.

## Recommended path

1. Fix the cheap, known 4 s first: reflinked seeded data-image template (cost 2). It needs no guest
   change, reuses `cloneFile`, and is the part of the "fork the base image" idea that is true today
   (the disk really is a copy-on-write fork of a base).
2. In the same series, trace and then trim the 1.9 s of runtime start and poll wait (cost 3):
   read the guest's `boot phase end dur=` lines for this VM before changing anything, then shorten the
   poll or decouple the receipt fetch, if readiness semantics allow.
3. Then design (not build) the claim-time identity change that makes a snapshot-of-a-booted-base (A)
   possible. This is the only route to a genuinely near-instant creation (about 1 to 2 s), and it is the
   missing half of S9's "fresh child identity material" requirement, so it should be written as a
   problem record and a design before code (Problem Documentation First), and put through the
   agentic-consensus panel because it moves identity material out of the boot path.
4. Do not build a standing warm pool.

Expected result if 1 and 2 land: about 14.3 s to about 8.4 to 9.5 s (inferred: 14.3 - 4.06 - up to 1.9);
if 3 lands: about 1 to 2 s (inferred).

## What is measured and what is inferred

Measured (timestamps on the host): everything in steps 0, 1 (as a block), 2, 3 (4,063 ms total), 4, 5,
6, 7, 8 (to the second, from `console.log` and the guest receipt), 9, 10, 11 (total only), 12, 13, 14;
the five creation durations in the journal; existing computers' `data_image_ready` at 3 to 99 ms.

Inferred: the clone/mkfs/fsync split inside step 3; the content of the 1.47 s in step 11; the
sub-steps of step 1; all "expected" times after an option is applied; the claim that snapshot
restore of a base will take 0.4 s (that is measured for hibernate resumes of existing computers, not
for a base); the signer/identity-in-memory risks (from the unit list and the cmdline keys, not from
a memory scan); the owner's "about 10 s" (not reproduced: this account measured 14.3 s).

## Code path, in order (file:line, origin/main working tree a03d5266 deployed)

1. Browser, first authenticated request after register: `/api/shell/bootstrap` ->
   `internal/proxy/handlers.go:1357` `ResolveDesktopContext` (via `resolveComputerURLOnce`, :1344; route
   authority `ensureComputerVersionRoute` :1309).
2. vmctl: `internal/vmctl/handlers.go:393` `ResolveOrAssignDesktopContext` ->
   `internal/vmctl/ownership.go:2067`/`:2075` `resolveDesktopContext`.
3. Mint: `generateVMID` :2270; `stableComputerID(userID, desktopID, "")` (deterministic) :2275/:2297;
   host-memory admission `admitHostMemoryLocked` :2280 (log line at `host_admission.go:500`); `nextEpoch`;
   ownership persisted `writePersistenceLocked` ~:2318; `admitRecoveryStart(own, true)` ~:2331;
   `ReserveBootEpoch` :2341; `issueGatewayToken`; `issueComputerCredentialEnvelope` :2369 (corpusd,
   idempotent, retried 3 times with 1 s/2 s backoff on failure: ownership.go:1773-1790).
4. `mgr.BootVM` :2379 -> `internal/vmmanager/manager.go:886` `bootVM`: global lock :887; port
   allocation :939; epoch consume ~:970; data image :986-1017 (`createDataImage` :3122, seed
   :3139, `createSparseDataImageSeeded` :3181); credential disk :1083 (`createCredentialDisk` :3033);
   Firecracker config; `launchFirecracker` :1123 (:2236; jailer in `confinement.go`, `launch_linux.go`);
   unlock :1128; `waitForGuestReady` :1145 (:2867, 250 ms poll :2932); `fetchGuestBootTimeline` :1175
   (`boot_timeline.go:230`, 800 ms poll); "booted VM" log.
5. Guest: Nix guest image (`nix/autoputer-vm.nix`): kernel -> initrd -> stage-2 systemd ->
   `go-choir-autoputer.service` -> `internal/autoputer/run.go` (main :163, store open :206, credential
   exchange :485, runtime built :609, listen :657, replay phase :677-701 / `runReplayPhase` :753,
   `rt.Start` via `afterReplay`, `runtime_started` :841; genesis mint pre-replay per the s0b problem doc)
   -> `internal/agentcore/runtime.go:888` `Start`.
6. vmctl: `transitionVM(Active)` -> "assigned VM" `ownership.go:2432`; proxy's pending bootstrap calls
   complete (they logged `context canceled` at 01:26:21.046, so the browser had already given up on
   those requests and retried).

## Comparison with the existing records

- docs/problems/capsule-coldstart-18s-2026-10-22.md is about engineering *capsules* (3 passes of one git
  process per file, 17 to 18 s) and is a different path. It is related only in lesson: per-file git
  forks are the usual hidden cost here, and the data-image seed above is a one-shot `git clone` plus
  `mkfs.ext4 -d`, not per-file processes, so 4 s is the measured total, not a fork storm.
- docs/definitions/choir-appdev-s3-fast-resume-2026-10-01.md quoted "cold boot about 15-30 s, warm
  boot about 5 s" and promised ~1 s snapshot wake. The snapshot wake did land (350 to 460 ms
  `wake_to_healthy` measured on existing computers), but it is only used for resume after hibernate,
  never for creation. A cold boot is still ~8 s of guest plus host work (today's refresh boots
  measure 8.2 to 14.5 s spawn->healthy).
- docs/definitions/choir-appdev-s9-forks-and-fleets-2026-10-01.md is where "fork the base image" lives:
  a two-path fork constructor (same-owner reflinked disk + cold boot, or semantic snapshot) with fresh
  child identity. It was reviewed 10-01 and never built (no fork constructor in `internal/`). Even the
  S9 same-owner path does a *cold boot*, so it would not be instant. The base-snapshot plus claim design
  above is not what S9 specifies; it is a new third path and needs its own definition.
- docs/problems/hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md: full snapshots
  (8 GiB memory file, ~560 MB after btrfs zstd), and every guest-artifact deploy voids all snapshots.
  A base snapshot has the same lifecycle: it must be rebuilt after each deploy that changes the guest
  image or Firecracker, in the background, off the critical path.

## Residuals of the data-image template (review 267, left open on purpose)

- The seed identity covers the resolved mkfs path but not mkfs's version, argv or `mke2fs.conf`
  (DeepSeek F2). On NixOS the store path carries the version; on another host an in-place e2fsprogs
  upgrade or an argv change needs a manual `dataImageTemplateFormat` bump.
- A `VMCTL_DATA_IMAGE_TEMPLATE_DIR` on a different filesystem than the state directory makes every
  FICLONE fail with EXDEV, so each creation falls back to today's build and says so in its receipt
  (Devin F3). No startup check refuses it yet.
- Unsetting the switch leaves the template and any crashed `.building-*` file in place (DeepSeek F4);
  clones share extents, so the cost is the seed's extents. Reclaiming them is an explicit prune, not an
  automatic act of the off switch.
