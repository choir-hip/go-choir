---
definition_version: 4

# Station FP10 / RES-20 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): Cloud Hypervisor
# behind a backend seam. P2, red when built. Authority: the security and
# density checklist (RES-20), the owner's "Firecracker and Cloud
# Hypervisor would be the two real production candidates" (2026-10-14),
# and FP0's brief: write the design, build nothing in vmmanager while FP8
# changes it.
readiness: drafted

review:
  reviewer: none yet; design only, no red diff
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T23:30:00Z
  source:
    canonical_ref: main d1c60ad87
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: lanes/fp10-res20-ch-seam
      status: clean
      class: goal_candidate
      owner: FP10 RES-20
      touch: goal_owned
      recovery: branch lane/fp10-res20-ch-seam
    - path: lane/fp8-image-jailer (branch only, read with git show)
      status: unknown
      class: other_agent_wip
      owner: FP8
      touch: forbidden
      recovery: leave in place

finish:
  deliver: >-
    A design for the seam between vmmanager and the hypervisor that fits
    main as it is now plus FP8: where the seam goes, which Firecracker code
    moves behind it, how Cloud Hypervisor is confined without a jailer,
    what CH gives (density, virtio-mem, snapshots, VFIO), and an ordered,
    revertible migration whose first step can start once FP8 lands.
  artifact: >-
    docs/problems/vmmanager-has-no-hypervisor-seam-2026-10-21.md and this
    file (green); docs/reports/vm-hypervisor-seam-design-2026-10-21.md
    (green), linked from the 2026-10-14 interface report.
  acceptance:
    - action: FP0 reads the design against main and FP8's landed diff
      proves: the file inventory and the migration order match the code that will be moved
      evidence_class: review receipt
  rollback: git revert of the docs commits
  landing:
    required: false
    environment: none (docs only; the docs truth checker runs on push)
    required_receipts: [pushed_commit]

value:
  better_means: >-
    Fewer unknowns between main and one test computer booting on Cloud
    Hypervisor under a cage, with no production computer changed.
  goodharting_would_be: >-
    Landing the recovered package as it is to tick the item, or claiming
    CH density or virtio-mem gains that are unmeasured or already on
    Firecracker.

homotopy:
  realism_axis: >-
    design on paper -> package rebased with no caller -> Firecracker
    through the seam on staging with byte-identical config -> one test
    computer on CH in a cage -> CH density measured.

boundaries:
  mutation_class: green for this station (docs); every build step in the design is red (VM lifecycle, vmctl)
  authority_sources:
    - docs/security-density-architecture-checklist-2026-10-21.md (RES-20)
    - docs/reports/vm-backend-interface-2026-10-14.md
    - docs/research/isolation-runtimes-2026-10-14.md
  must_preserve:
    - no edit to vmmanager, vmctl, the VM image or nix while FP8 owns them
    - no production computer selectable onto a second VMM
    - a hibernated computer's wake is never turned into a crash boot by a backend change
    - no boot-time resumer without a planned-restart marker
  excluded:
    - building any step of the migration in this station
    - CH snapshot, copy-on-write restore, live migration, VFIO (each its own mission)
  protected_surfaces: [VM lifecycle, vmctl, snapshot validity, VMM confinement]

now:
  status: checkpoint_incomplete
  slice: >-
    Station opened; problem record written. The design is next.
  source_ref: d1c60ad87
  deploy_identity: unknown to this lane
  next_action: write docs/reports/vm-hypervisor-seam-design-2026-10-21.md

receipts: []
---

# FP10 RES-20: Cloud Hypervisor behind a backend seam

Problem: [vmmanager-has-no-hypervisor-seam-2026-10-21.md](../problems/vmmanager-has-no-hypervisor-seam-2026-10-21.md).
Design: [vm-hypervisor-seam-design-2026-10-21.md](../reports/vm-hypervisor-seam-design-2026-10-21.md).
