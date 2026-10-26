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
        it is bigger than one slice.
      class: red
    - id: D3
      what: >-
        Idle hibernation reaches primary computers
        (VMCTL_PRIMARY_KEEPALIVE_MODE, warmness_policy.go) and the idle
        timer drops to minutes, never a computer with a live run, capsule,
        apply or self-development operation, never the paper
        (critical_protected), public_platform or the owner's computers.
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
    state: rehearsing
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
    D1 failure modes, then tests, then code; push lane/density for FP0's
    review.

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
