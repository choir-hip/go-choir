---
definition_version: 4

# FP10 of the launch first pass (choir-launch-first-pass-metamission-2026-10-21.md),
# item RES-08 of docs/security-density-architecture-checklist-2026-10-21.md
# (P2, red): share the base OS in guest RAM with a read-only virtio-pmem
# device and an uncompressed EROFS mounted -o dax. The item is behind a
# security panel (DEC-05: cross-tenant shared pages, Flush+Reload), and its
# value is unmeasured. vmmanager, vmctl and the VM image belong to FP8
# (lane/fp8-image-jailer) during this pass, so this lane builds nothing there.
readiness: drafted

review:
  reviewer: none yet; FP0 convenes the DEC-05 security panel on the design report
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-22T00:00:00Z
  source:
    canonical_ref: main d1c60ad87 (lane base)
    deploy_identity: not read by this lane (FP0 owns staging)
  worktrees:
    - path: /home/agent/lanes/fp10-res08-pmem-dax
      status: clean at start
      class: goal_candidate
      owner: FP10 RES-08 worker
      touch: goal_owned
      recovery: branch lane/fp10-res08-pmem-dax

finish:
  deliver: >-
    The owner and the DEC-05 panel can decide RES-08 on measured numbers:
    how much guest RAM is a private copy of the base OS, what a shared pmem
    DAX image would save net of its own costs, and what cross-tenant channel
    it opens against today's baseline.
  artifact: >-
    A problem record; a design and threat-analysis report with the panel's
    questions; a read-only guest page-cache residency read that FP0 can take
    with no shell in the guest; FP0's exact measurement steps and the
    decision thresholds that turn the numbers into go, per-tenant go, or
    close in favour of L5.
  acceptance:
    - action: go test ./internal/server -run 'PageCache|DebugPprof'
      proves: the residency read sums only resident pages, refuses non-host callers, never reads file contents, and stops at its bounds
      evidence_class: unit
    - action: FP0 runs the measurement steps in the design report §5 on staging after the read deploys
      proves: the store share of idle and busy guest page cache, and the host's one-copy residency of the image
      evidence_class: deployed proof (read-only)
    - action: the DEC-05 panel's verdict on the design report §4
      proves: the cross-tenant decision is not this worker's own
      evidence_class: review receipt
  rollback: >-
    The residency read is one git revert (an additive debug route). The
    design and problem docs need no rollback. No device, image or vmmanager
    change is made in this pass.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Turn the L10 estimate (0.3 to 0.45 GiB per idle guest, a hypothesis)
    into a measured number and a panel decision, while preserving every
    tenant boundary that exists today and opening no new cross-tenant
    channel without the panel.
  goodharting_would_be: >-
    Shipping the pmem device "off by default" with the global shared image
    already wired, so turning it on needs no panel; counting page cache that
    is Dolt data as store; quoting a saving that ignores the guest memmap
    for the pmem region.

homotopy:
  realism_axis: >-
    Unit tests of the residency read on a temp tree, then the read on one
    idle staging test computer, then the paper and a busy test computer, then
    (after the panel) a pmem DAX boot on one test computer under FP8's
    vmmanager, then the density benchmark.

boundaries:
  mutation_class: >-
    red item; this pass ships green docs plus one orange read-only debug
    route in the guest daemon (internal/server)
  authority_sources:
    - owner 2026-10-21 (the metamission quotes)
    - docs/security-density-architecture-checklist-2026-10-21.md RES-08, DEC-05
    - docs/reports/vm-density-limits-2026-10-14.md lever L10, finding 3, residual on fincore
    - docs/reports/computer-isolation-security-review-2026-10-14.md §1.5, §3.7, §4.7
  must_preserve:
    - No cross-tenant shared guest mapping without the DEC-05 panel's verdict.
    - The metamission's boundaries.must_preserve (no boot-path scan of og_objects, no boot-time resumer without a planned-restart marker, red fails closed).
    - The residency read is host-sourced only, reads no file contents and names no path outside the store.
    - No edits to vmctl, vmmanager or the VM image (FP8 owns them this pass).
  excluded:
    - Any virtio-pmem wiring, image format change or guest mount change.
    - KSM (L13, rejected) and shared snapshot templates (ARCH-11; same DEC-05).
  protected_surfaces: [none touched; the design names vmmanager drives, the guest image and the jailer as the surfaces a later slice touches]

now:
  status: working
  slice: problem record and station file; next the design report, then the residency read (tests first)
  source_ref: d1c60ad87
  deploy_identity: unchanged
  candidate:
    id: FP10-RES-08-measure
    state: not built
    ref: lane/fp10-res08-pmem-dax
    base: d1c60ad87
    digest: none
    scope: [docs, internal/server]
  conjecture:
    id: C-RES-08-store-share
    claim: >-
      Most of an idle guest's page cache (about 640 MiB) is base-OS store
      pages, so a shared DAX image saves 0.3 GiB or more per idle guest net
      of its memmap cost.
    test: the guest residency read split by filesystem on idle and busy staging computers
    edge: missing_oracle (no residency read exists in the guest today)
    delta_o: the pagecache debug read plus FP0's host fincore of the image
    scope_if_supported: Node B guests on one release
    status: active
    evidence_refs:
      - docs/reports/vm-density-limits-2026-10-14.md finding 3 and residuals
  decision:
    what: Whether guests of different tenants may share one physical copy of base-OS pages (DEC-05)
    kind: security
    status: open (panel)
    evidence_ref: docs/reports/res-08-shared-base-os-pmem-dax-design-2026-10-22.md
    owner_ratification_ref: pending
  belief:
    believed_state: the base OS is private per guest in RAM; its size is unmeasured
    main_uncertainty: whether the store or the Dolt data dominates idle guest page cache
    next_observation: FP0's residency read after the route deploys
  blocker_or_risk: the panel (DEC-05) and FP8's ownership of vmmanager and the image
  next_action: write the design report

receipts: []
---

# FP10 · RES-08: share the base OS in guest RAM

The problem: [base-os-copied-into-every-guest-ram-2026-10-22.md](../problems/base-os-copied-into-every-guest-ram-2026-10-22.md).
The design, threat analysis and measurement steps:
[res-08-shared-base-os-pmem-dax-design-2026-10-22.md](../reports/res-08-shared-base-os-pmem-dax-design-2026-10-22.md).
