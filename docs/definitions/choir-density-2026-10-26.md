---
definition_version: 5

# Station D (density) of
# docs/definitions/choir-consolidate-and-scale-metamission-2026-10-26.md
# (being committed by FP0; referenced by path). Computers start and wake
# without a person making room for them.
# Owner 2026-10-26, in session: "this scalability work is all a priority,
# above other residuals even"; "we should consider hibernating before 30
# minutes. If we can restore fast we can hibernate idle computers at will";
# "We shouldn't need to be doing that manual memory management. ... we can
# have many machines running simultaneously, and we can virtualize their
# memory, and we can like subtly background or hibernate them and
# intersperse them". Swap stays ruled out (owner 2026-10-09: "swapping is
# service degradation. we cant tolerate it").
# Evidence: docs/problems/start-refused-for-memory-no-reclaim-2026-10-26.md
# (FP0's investigation; landing with the metamission). Per-item failure
# modes, written before each item's code:
# docs/problems/density-station-d-failure-modes-2026-10-26.md.
readiness: drafted

review:
  reviewer: none yet (one Devin seat for this file; each red item is reviewed at its own tier)
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-26T12:20:00Z
  source:
    canonical_ref: origin/main 618727c97
    deploy_identity: staging 99c4d9a9a (train 12; train 13 in CI)
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty (M go.sum, ?? scratchpad/)
      class: user_wip
      owner: owner
      touch: forbidden
      recovery: leave in place
    - path: /Users/wiz/Dev/lanes/fp0-doc
      status: dirty (staged: the metamission, the start-refused problem doc, related-stories evidence)
      class: other_agent_wip
      owner: FP0
      touch: read_only
      recovery: FP0 commits it
    - path: /Users/wiz/Dev/lanes/density (branch lane/density)
      status: clean at start
      class: goal_candidate
      owner: density lane worker
      touch: goal_owned
      recovery: branch lane/density on origin
  observed:
    - >-
      2026-10-26 11:49: b@c's start was refused by admitHostMemoryWith
      (internal/vmctl/host_admission.go:418): required 4779 floor + 1024
      headroom + 0 reserved + 7847 measured peak = 13650 MiB against 13202
      MiB available. FP0 stopped d@e by hand to make room. Fourth recorded
      recurrence (10-13, 10-22 twice, 10-26 03:1x).
    - >-
      Admission only refuses; nothing on the refusal path hibernates
      anything. Pressure reclaim fires only under the 4779 MiB floor with
      PSI avg60 over 5.0 for four samples, so between 4.8 and 13.6 GiB
      available starts are refused and nothing is reclaimed.
    - >-
      Live vmctl env: VMCTL_IDLE_TIMEOUT=30m, VMCTL_IDLE_SWEEP_INTERVAL=2m,
      VMCTL_PRIMARY_KEEPALIVE_MODE=under-capacity (primary computers never
      idle-hibernate unless sustained pressure),
      VMCTL_PRESSURE_RECLAIM_MIN_IDLE=30m,
      VMCTL_PRESSURE_MIN_MEMORY_AVAILABLE_MIB=4096 and _PERCENT=15,
      VM_HIBERNATE_MODE=snapshot, VM_BALLOON_FREE_PAGE_REPORTING=1,
      VM_MEMORY_HOTPLUG=off, VMCTL_ADMISSION_MODE=measured.
    - >-
      Snapshot wake is 0.1 to 0.4 s
      (docs/evidence/s3-snapshot-hibernate-staging-2026-10-14.md) but every
      guest-artifact deploy voids every snapshot: 0 of 8 resumed in 58 h,
      each wake a cold boot on launch_config_mismatch
      (docs/problems/hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md, R1).
    - >-
      Admission charges a wake the computer's last peak RSS (7847 MiB for
      b@c, clamped to the 8 GiB guest); a snapshot wake lands at about 0.3
      to 1.4 GiB resident and guest page cache never returns to the host.

finish:
  deliver: >-
    A person starts or wakes a computer and it starts: when the host is
    full, vmctl hibernates idle computers to make room instead of refusing;
    idle computers hibernate within minutes and wake in under a second,
    across deploys; nobody stops a computer by hand to make room.
  artifact: >-
    On main and deployed to staging, each behind its own env switch:
    D1 admission makes room (VMCTL_ADMISSION_MAKE_ROOM); D2 snapshots that
    survive a deploy where safe (design first; build only if one slice);
    D3 idle hibernation reaches primary computers and the idle timer drops
    to minutes (nix/node-b.nix); D4 admission charges a wake's expected
    cost; D5 diff snapshots on; D6 balloon inflation returns guest page
    cache; D7 the density bench run with its numbers recorded.
  items:
    - id: D1
      what: >-
        A start that does not fit makes room: hibernate (snapshot) the
        lowest-priority idle eligible computer(s), then admit. Refuse only
        when nothing eligible exists, naming why. One decider at a time; no
        double hibernate; a start in flight stays reserved and is never a
        victim.
      class: red
      switch: VMCTL_ADMISSION_MAKE_ROOM=active|off
    - id: D2
      what: >-
        Snapshots survive a deploy where it is safe. Why deploys void them,
        what makes a snapshot of build X unsafe under host build Y, the
        planned-restart marker rules. Design first; stop at the design if
        it is bigger than one slice (it is: see "D2 build spec"). b@c is
        crash-looping at boot (X1) and may not be used for any D2 proof
        until X1 lands; use d@e and test computers.
      class: red
    - id: D3
      what: >-
        Idle hibernation reaches primary computers
        (VMCTL_PRIMARY_KEEPALIVE_MODE, warmness_policy.go) and the idle
        timer drops to minutes, never a computer with a live run, capsule,
        apply or self-development operation, never the paper
        (critical_protected), public_platform or the owner's computers.
        b@c is crash-looping at boot (X1) and may not be used for any D3
        proof until X1 lands; use d@e and test computers.
      class: red
    - id: D4
      what: >-
        Admission charges a wake's expected cost (the snapshot's resident
        size or a measured post-wake RSS), not the last peak.
      class: red
    - id: D5
      what: Diff snapshots on (RES-13 L3a, built and off), then L3b.
      class: red
    - id: D6
      what: Balloon inflation returns guest page cache (RES-10 step 3, unbuilt).
      class: red
    - id: D7
      what: The density bench (RES-19), run, numbers recorded.
      class: yellow
  acceptance:
    - action: >-
        On staging, start five test computers in a row on a host that
        cannot hold them all resident, with no hand stop; read vmctl health
        admission.recent for each start (decision admit or
        admit_after_make_room, the hibernated VM ids) and each later wake's
        timeline.
      proves: >-
        D1 removes the manual stop for test-class starts; each wake under
        one second proves D2+D4 for the woken computers.
      evidence_class: deployed proof
    - action: >-
        Host memory (MemAvailable before and after each start) and snapshot
        bytes recorded for each step in the evidence doc.
      proves: the density numbers are measured, not estimated.
      evidence_class: deployed proof
  rollback: >-
    One env switch per item in nix/node-b.nix (D1
    VMCTL_ADMISSION_MAKE_ROOM=off; D3 VMCTL_PRIMARY_KEEPALIVE_MODE and
    VMCTL_IDLE_TIMEOUT back to under-capacity/30m; D4 its estimator mode
    back to peak; D2 its own flag, off), and git revert of each item's code
    commit.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize hand steps that make room for a computer (manual stops,
    refused starts) and the RAM idle computers hold, while a working
    computer is never hibernated and every wake stays under one second.
  goodharting_would_be: >-
    Lowering the floor or headroom so starts pass; hibernating computers
    that are working; counting cold boots as wakes; charging admission so
    little that the host goes under the floor and the OOM killer decides.

homotopy:
  realism_axis: >-
    From two test computers on one host, through every test computer at
    once, to primary computers under the same policy; the paper and the
    owner's computers stay excluded throughout.

boundaries:
  mutation_class: red
  authority_sources:
    - the owner's statements of 2026-10-26 in this file's header
    - docs/choir-doctrine.md, CLAUDE.md (restart rules)
    - the metamission's station D note
  must_preserve:
    - a computer stays on while it is working; never hibernate one with a live run, capsule, apply or self-development operation
    - the paper (critical_protected), public_platform and premium_always_on computers are never hibernated for room or idleness
    - crash restarts close work; planned restarts resume once (marker rule)
    - guests do not swap (owner 2026-10-09)
    - a held computer is never hibernated
    - admission still refuses when nothing eligible can make room
  excluded:
    - raising host RAM or lowering the floor as the fix
    - node-a changes
    - email
  protected_surfaces:
    - vmctl admission (host_admission.go)
    - vmctl reclaim (pressure_reclaim.go, pressure_sustained.go) and the idle sweep (warmness_policy.go, ownership.go)
    - vmctl/vmmanager hibernate and snapshot resume (VM lifecycle)
    - nix/node-b.nix vmctl environment (deployment config)

now:
  status: working
  slice: D1
  source_ref: origin/main 618727c97
  deploy_identity: staging 99c4d9a9a
  candidate:
    id: lane/density
    state: frozen
    ref: lane/density
    base: origin/main 618727c97
    digest: none
    scope: [internal/vmctl, cmd/vmctl, nix/node-b.nix, docs]
  conjecture:
    id: C-D1
    claim: >-
      If a refused start hibernates the lowest-priority idle unprotected
      computer and re-admits on the fresh sample, then the 11:49 case
      (448 MiB short with an idle d@e holding about 2 GiB) admits without a
      hand stop, and no working computer is hibernated.
    test: >-
      Unit tests on the registry (admission, ranking, concurrency); then
      the staging five-test-computer start with admission.recent read from
      vmctl health.
    edge: missing_oracle
    delta_o: >-
      admission.recent records each make-room decision with the victims and
      the reason a candidate was skipped, so the staging read shows why.
    scope_if_supported: >-
      test and primary starters on Node B with snapshot hibernate on.
    status: active
    evidence_refs: [docs/problems/start-refused-for-memory-no-reclaim-2026-10-26.md]
  decision:
    what: >-
      D1 victim rule: unprotected class, priority at or below the
      starter's, not held, not busy by the guest's own probe, no lifecycle
      claim, not the starter, and the pressure-reclaim idle rule (test
      computers need no idle time unless the starter is also test, then the
      admission settle window).
    kind: operational
    status: proposal
    evidence_ref: docs/problems/density-station-d-failure-modes-2026-10-26.md
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      Hibernate and fast wake work but are not used to make room; admission
      charges the last peak; primary computers never idle-hibernate.
    main_uncertainty: >-
      How fast MemAvailable reflects a hibernated VM's freed RSS (the
      snapshot write passes through page cache), which sets how long a
      start waits after making room.
    next_observation: >-
      The first staging make-room: available before, victims' RSS, and
      available at re-admission.
  blocker_or_risk: >-
    A start that makes room waits for a full snapshot (6 to 10 s) before
    its own boot; D5 (diff) shortens it.
  next_action: >-
    D1 on lane/density (docs ae8301b55, tests be447ccfd, code 6020c59c9)
    awaits FP0's review and the landing loop. D2 (design only), D3 and D4
    go to Sonnet workers from the build specs below.

receipts: []
---

# Density (station D)

Item order is by impact (the investigation's gap list): D1 removes the hand
stop; D4 is its safety net (a smaller, honest charge means fewer victims);
D2 makes a hibernate a fast wake instead of a cold boot; D3 makes idle
computers give memory back before anyone asks. D5 to D7 follow.

Review tiers: D1, D3 and D4 are red behind a config switch with a one-act
rollback (tier 1, Devin); D1 adds concurrency on the VM lifecycle, so FP0
may take it to tier 2. D2's design is reviewed like the change it designs.

## Rules for every item (read before building D2, D3 or D4)

- Problem documentation first: add the item's section to
  `docs/problems/density-station-d-failure-modes-2026-10-26.md` (copy the D1
  section's shape: Problem, Design, Failure modes M1..Mn, Protected
  surfaces/evidence/rollback) and commit it alone (green). Then the tests
  alone (yellow, "red until the code"). Then the code (red). Push
  `lane/<name>` after each commit; never push main.
- Focused tests only on the laptop: `go test ./internal/vmctl -run <Names>`,
  `go test ./cmd/vmctl -run Env`, `go test ./internal/vmmanager -run <Names>`.
  Add `-race` for anything with goroutines. Broad runs go to
  `ssh choir-agents ~/lane-test.sh` (read its usage first).
- Gates before every push:
  `direnv exec . sh -c 'scripts/check-v1-writer-purity.sh && scripts/check-decode-roots.sh && scripts/check-test-hang-guards.sh && scripts/selftest-guest-token && go build ./...'`,
  and when `nix/` changed, both evals:
  `nix eval --raw .#nixosConfigurations.go-choir-b.config.system.build.toplevel.drvPath`
  and the same for `go-choir-autoputer-vm`.
- **b@c is not a proof computer until X1 lands.** b@c is crash-looping at
  boot from the X1 bug (the crash settlement reruns the apply checkpoint
  replay; docs/problems/apply-checkpoint-replay-fills-guest-tmp-2026-10-26.md).
  No density proof may use b@c, as starter or victim. Use d@e and test
  accounts (email domains example.com / example.test, which are class
  `test`).
- Test fixtures (internal/vmctl): `newAdmissionFixture` and
  `newMakeRoomFixture` (host_admission_test.go,
  host_admission_make_room_test.go) give a registry with a fixed clock
  (`f.now`), a settable MemAvailable (`f.setAvail`), per-VM RSS
  (`f.mgr.setRSS` then `f.reg.sampleVMMemory()`), and a hibernate that frees
  the VM's RSS. `addOwnership` inserts an ownership.
- Traps found while building D1:
  - `checkIdleOwnerships` (the idle sweep) measures idle against
    `time.Now()`, not the registry clock; pressure reclaim and make-room use
    `r.now()`. A sweep test must set `LastActiveAt` relative to real time.
  - `NewOwnershipRegistry` normalizes an empty keepalive mode to `off`; the
    `under-capacity` default lives only in `cmd/vmctl/main.go`
    (`warmnessPolicyConfigFromEnv`, `envOr`). Test both.
  - `rankPressureCandidates` probes every active guest over HTTP (4 s
    timeout each) before filtering; filter first, probe last (as
    `planMakeRoom` does).
  - `protectedReclaimReasons` (pressure reclaim) does not exclude
    public_platform; `warmnessClassProtected` (idle sweep) and make-room
    do. Do not reuse the former for an exclusion rule.
  - Make-room is enabled only when a route guard is set
    (`SetMakeRoomRouteGuard`, done in main.go). Existing tests that never set
    it see today's refusal; new tests must set it.
  - `admitHostMemoryLocked` runs under `r.mu`; anything that hibernates
    must run with `r.mu` released (see the new-computer branch of
    `resolveDesktopContext`).
  - GNU sed is on PATH: `sed -i ''` does not mean "no backup", it treats
    `''` as the script. Use `sed -i` or an editor.

## D2 build spec: snapshots survive a deploy where safe (design, then stop)

Deliverable of this item is a **design**, not code: the change spans the
deploy (nix), vmmanager and vmctl and needs owner-ruled inputs, so it is
bigger than one slice. Write it as a new section "L20 revised 2026-10-26" in
`docs/design/fp10-snapshot-levers-and-host-slab-2026-10-22.md` (its L20
section is the starting point) and stop there for review.

- Why deploys void snapshots today (state it with citations):
  `refreshConfigForCurrentDeploy` (`internal/vmmanager/manager.go`) maps a
  snapshot's config onto the current guest artifacts, and the deploy
  overwrites them in place at stable paths (`nix/node-b.nix`
  `VM_KERNEL_IMAGE=/var/lib/go-choir/guest/vmlinux`, `VM_ROOTFS_IMAGE`,
  `VM_STORE_DISK_IMAGE`, the install loop near
  `for artifact in vmlinux rootfs.ext4 initrd storedisk.erofs kernel-params build.json`).
  `validateSnapshot` (`internal/vmmanager/snapshot.go`) then refuses with
  `launch_config_mismatch` or `disk_identity_mismatch`. That refusal is
  correct: the resumed guest's memory maps the old disk contents.
- What makes a snapshot of build X unsafe under host build Y (each is a
  refusal rule the design must keep or add): the Firecracker version or
  host kernel changed (kept: `firecracker_version_mismatch`,
  `host_kernel_mismatch`); X's disk files are gone or changed (kept: disk
  identity); X is below the **floor** (owner ruling 2026-10-25 Q2: "all
  host-level and security-relevant code, everything engineering has no
  access to"; a deploy that changes the floor voids older snapshots); X is
  older than the **support window** (owner 2026-10-25 Q1: one week; today
  also bounded by `VM_SNAPSHOT_MAX_AGE=36h`); the host-guest protocol X
  speaks is not served by Y (the design must say how compatibility is
  checked, e.g. a protocol version in build.json); confinement changed
  (kept: `confinement_mismatch`); a forced security fix (owner 2026-10-25:
  security first; a forced fix is never deferred, and a computer is never
  contained or stopped for it).
- Mechanism to specify: release directories named by guest manifest digest
  (`/var/lib/go-choir/guest/releases/<digest>/`, `current` symlink) and
  their retention (current, previous, and any named by an unconsumed
  snapshot inside the window); vmctl/vmmanager record the release digest
  in `VMConfig` and `SnapshotMeta`; `expectedLaunchConfig` uses the
  snapshot's release when retained and at or above the floor.
- Restart rules: a computer woken onto an older release is marked behind;
  the deferred-refresh sweep (`deploy_refresh.go`) moves it to `current` at
  its next idle moment. An idle computer has no open obligation, so that
  refresh needs no planned-restart marker; a busy one takes the ordinary
  marker path (CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned
  Update)"); a woken-behind computer must never be resumed twice by one
  marker.
- Failure modes the design must answer: resume onto a deleted or
  overwritten release; resume below the floor; a behind computer never
  refreshed; disk growth from retained releases (size each release; the
  snapshot budget is `VM_SNAPSHOT_BUDGET_GIB=48`); the snapshot vault and
  jailer paths (`snapshot_vault_*`, `confinement_mismatch`); credential
  delivery on resume unchanged; a vmctl restart resumes nothing.
- Switch for the later build: `VM_SNAPSHOT_RELEASE_RETAIN=off|on` (off is
  today). Rollback: off, then the next deploy voids old snapshots as today.
- Evidence the design names: on staging, hibernate d@e or a test
  computer, deploy a guest-artifact change, wake it, and read the wake
  timeline (`snapshot` reason absent, wake_to_healthy under 1 s) and
  vmctl's deferred refresh moving it to `current` once idle. Not on b@c
  until X1 lands (see the rules above).

## D3 build spec: idle hibernation reaches primary computers, in minutes

- Files: `nix/node-b.nix` (vmctl environment: `VMCTL_IDLE_TIMEOUT`,
  `VMCTL_PRIMARY_KEEPALIVE_MODE`, `VMCTL_PRESSURE_RECLAIM_MIN_IDLE`);
  `internal/vmctl/warmness_policy.go` (`idleOwnershipCandidates`,
  only if a code change is needed); `cmd/vmctl/main.go`
  (`warmnessPolicyConfigFromEnv` default) only if the code default
  changes; tests in `internal/vmctl` (new `idle_hibernation_density_test.go`)
  and `cmd/vmctl/env_config_test.go`.
- Change: `VMCTL_PRIMARY_KEEPALIVE_MODE=off` (primary computers idle-
  hibernate like test ones) and `VMCTL_IDLE_TIMEOUT=10m`,
  `VMCTL_PRESSURE_RECLAIM_MIN_IDLE=10m` (make-room's idle rule follows the
  latter). The number: 10 minutes is the snapshot-levers design's Q1
  default and the owner asked for "before 30 minutes"; it is bounded below
  by the full-snapshot write (6 to 10 s of 8 GiB I/O and host PSI per
  hibernate, R1) until D5 (diff) is on, so do not go under 10m before D5.
  Record the hibernates-per-hour before and after from vmctl's journal
  line `hibernated VM ... reason=idle`.
- Exclusions that must hold (already in code; pin them): protected classes
  (`warmnessClassProtected`: premium_always_on = the owner's paid
  computers, critical_protected = the paper, public_platform); held
  computers; a busy guest (`httpGuestBusy`: `running_runs`,
  `desk_pending_mutations`, `selfdev_active_operations` > 0, and any
  non-200 or unreadable answer counts as busy).
- Failure modes (each a test before the config change): a primary
  computer with a live run, pending desk mutation or active
  self-development operation is hibernated; a computer with a live
  engineering capsule is hibernated (**verify** that a live capsule shows in
  one of the three health fields; if it does not, that is a new problem doc
  and a code change to the busy probe before the timer drops); the paper,
  public_platform or owner computer is hibernated; a held computer is
  hibernated; a computer is hibernated just after its apply restart
  (docs/problems/idle-sweep-hibernates-a-computer-just-after-its-apply-restart-2026-10-24.md:
  a planned restart is not activity, PR-09); a computer driven only by API
  keys hibernates while busy (LastActiveAt counts resolves only; the busy
  probe is the fence); the keepalive env default changes silently.
- Tests: in `internal/vmctl`, call `idleOwnershipCandidates` (pure
  function; pass `now`) with keepalive `off` and a 10m timeout over
  ownerships of each class, busy and not, held and not, idle 9m and 11m;
  assert exactly the idle unheld non-busy primary and test computers are
  candidates. In `cmd/vmctl/env_config_test.go`, assert
  `VMCTL_PRIMARY_KEEPALIVE_MODE=off` and `under-capacity` both parse.
- Switch: the three env values. Rollback: `VMCTL_PRIMARY_KEEPALIVE_MODE=under-capacity`,
  `VMCTL_IDLE_TIMEOUT=30m`, `VMCTL_PRESSURE_RECLAIM_MIN_IDLE=30m`.
- Cost to state in the commit: until D2 lands, a primary computer woken
  after a deploy cold boots (about 10 s, crash-style boot that closes open
  obligations; an idle computer has none) instead of staying resident.
- Evidence: staging, d@e idle 11 minutes is hibernated (`reason=idle`),
  woken by a session request, wake timeline read. Not b@c (X1).

## D4 build spec: admission charges a wake's expected cost

- Files: `internal/vmctl/host_admission.go` (`estimateNeedWith`,
  `outstandingLocked`, `sampleVMMemory`, `HostAdmissionConfig`,
  `normalizeHostAdmissionConfig`); `internal/vmctl/ownership.go`
  (`VMOwnership`: a new persisted field `MemorySettledRSSBytes uint64
  json:"memory_settled_rss_bytes,omitempty"` beside `MemoryPeakRSSBytes`);
  `cmd/vmctl/main.go` (`hostAdmissionConfigFromEnv`); `nix/node-b.nix`;
  tests in `internal/vmctl/host_admission_test.go` and
  `cmd/vmctl/env_config_test.go`.
- Change: record each start's **settled RSS**, the VM's resident memory
  when its reservation's settle window (5 minutes) ends (in
  `outstandingLocked`, where a settled reservation expires, capture the
  latest `led.rss` for that VM; `sampleVMMemory` persists it on the
  ownership). With `VMCTL_ADMISSION_NEED=wake`, `estimateNeedWith` charges a
  start the settled RSS (floored at `MinNeedBytes`, 1 GiB; clamped to the
  guest size), falling back to `DefaultNeedBytes` (2 GiB) when unmeasured,
  never the peak. The peak stays recorded and shown. Evidence for the
  numbers: a snapshot wake settles at about 0.3 to 0.4 GiB and a cold boot at
  1.15 to 1.37 GiB (docs/evidence/s3-snapshot-hibernate-staging-2026-10-14.md;
  hibernate-writes-whole-guest R5), against b@c's 7847 MiB peak. The 1 GiB
  floor covers both.
- Failure modes: overcommit, the admitted computer later grows to its peak
  and the host falls under its floor (RES-15; the backstops are sustained
  pressure reclaim and D1 make-room on the next start; say so, and keep the
  OOM order: ordinary VMs at oom_score_adj 500); a settled RSS taken from a
  start that failed or was killed inside the window; the field missing on
  old ownership files (must read as unmeasured, not zero need); protected
  starts unchanged; guest-size mode unchanged; `VMCTL_ADMISSION_NEED=peak`
  restores today's estimate exactly; a vmctl restart does not invent a
  settled value.
- Tests: the 11:49 numbers: an ownership with peak 7847 MiB and settled
  1.2 GiB is admitted at 13202 MiB available under `wake` and refused under
  `peak`; unmeasured charges 2 GiB; settled 300 MiB charges 1 GiB; the
  settle capture happens only at window end and only for a started
  reservation (`ticket.finish(true)`); a JSON round trip of an ownership
  without the field.
- Switch: `VMCTL_ADMISSION_NEED=wake|peak` (code default `peak` until the
  staging proof, then flip in nix; or default `wake` if FP0 rules built
  features default on). Rollback: `VMCTL_ADMISSION_NEED=peak`.
- Evidence: staging, vmctl health `admission.recent` shows `need_source`
  `settled` for d@e's wake and the need near its settled RSS. Not b@c (X1).
