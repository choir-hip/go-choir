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
    - action: go test ./internal/server -run 'PageCache|ProcMem|DebugPprof' (passes at fa547e9c7)
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
    - docs/archive/reports/computer-isolation-security-review-2026-10-14.md §1.5, §3.7, §4.7
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
  status: checkpoint (round 5 done: comments and docs only; FP0 lands it on round 4, then the reads)
  slice: >-
    The read landed in deploy D (fd58229aa, 840e1a701 on main). Review bunch
    13 (DeepSeek v4.1 Flash) accepted with edits. Its findings 1, 2, 3, 4
    and 6 are fixed in 53c91bc1f: bounds hold per file, per directory and
    per mapped chunk; the overlay upper is its own root (store_upper) and is
    excluded from persistent; skipped mounts and examined files are counted;
    new tests. 079e11245 makes the probe surface skips and refuse an
    unpartitioned, unmeasured or partial store number, and records finding 5
    (procmem's guard is literal only in hostauth enforce mode) as a residual
    in report §8. Review bunch 16 (Gemini flash accept; Devin accept with
    edits): a8f27991d makes an overlay whose upper does not resolve report
    partitioned false, and pins the exclusion by path in the test;
    fb2b6587c makes the probe refuse a store with skips, print null store
    numbers while unquotable, and records the DT_UNKNOWN residual. The bunch
    13 fixes were rebased to 5b33a456e and d661cd756. Bunch 18 accepted round
    2 (it rides deploy F). Round 3 answers DeepSeek's late bunch-16 review of
    001edc8e8: a94d459ab (stacked overlays and an overlay without a parsed
    upper report partitioned false with a reason; caps documented per root;
    an escape test) and 126f5136d (the probe refuses a store that measured
    nothing, nulls persistent while unpartitioned, and quotes the reason; the
    count claim is corrected). Round 4 answers DeepSeek's bunch 20 (four low
    findings): the tests (yellow), 273b1ceb2 (lowerdir+= and datadir+= are
    stacked; stacking no longer depends on mount order, so a hidden overlay
    is stacked too), and the report's comma wording, corrected to what the
    code does. Round 5 (bunch 23, comments and docs only) corrects what round 4
    overclaimed: a literal comma that truncates an upper to a path that
    exists is NOT caught (DeepSeek's counterexample, reproduced); it is
    unreachable because the kernel emits \054. It also fixes the stale test
    header (item 19), the PageCacheLayering and Stacked godoc, and the report's
    reason list. Every layering reason is now asserted. Already fixed by round 2: F1 (store fields),
    F3 and F7. F6 was not fixed by round 2 and is fixed in round 3.
  source_ref: 8fabec36a
  deploy_identity: deploy D serves the first read (no store_layering); the probe refuses its store numbers
  candidate:
    id: FP10-RES-08-measure-r6
    state: built, not frozen; round 5 (comments and docs) on top of round 4
    ref: lane/fp10-res08-pmem-dax
    base: 8fabec36a
    digest: none
    scope: [internal/server/debug_pagecache*.go, scripts/res08_pagecache_probe.py, scripts/test_res08_pagecache_probe.py, docs]
  conjecture:
    id: C-RES-08-store-share
    claim: >-
      Most of an idle guest's page cache (about 640 MiB) is base-OS store
      pages, so a shared DAX image saves 0.3 GiB or more per idle guest net
      of its memmap cost.
    test: the report §5 reads (idle-30m, cold-boot, busy-run) and its threshold table, quoted only when store_quotable
    edge: missing_oracle until round 2 (deploy F) and round 3 deploy (deploy D's read can double count the overlay upper)
    delta_o: FP0's res08_pagecache_probe.py receipts
    scope_if_supported: Node B guests on one release
    status: active
    evidence_refs:
      - docs/reports/vm-density-limits-2026-10-14.md finding 3 and residuals
      - internal/server/debug_pagecache_test.go (failure modes 1-20; against deploy D's logic the three bound tests failed, the skipped-subtree test passed, and the rest are tests of new API that could not compile there)
      - scripts/test_res08_pagecache_probe.py (refusal rules)
  decision:
    what: Whether guests in different trust domains may share one physical copy of base-OS pages (DEC-05)
    kind: security
    status: open (panel); the lane proposes O2 (per-domain inode) or O3 (L5 instead), never O1
    evidence_ref: docs/reports/res-08-shared-base-os-pmem-dax-design-2026-10-22.md §3, §4, §7
    owner_ratification_ref: pending
  belief:
    believed_state: >-
      The base OS is private per guest in RAM; its size is unmeasured. The
      daemon runs as guest root, so the store is measurable. A layered
      release adds an overlay upper, which the fixed read counts apart.
    main_uncertainty: whether the store or the Dolt data dominates idle guest page cache, and how much of the store is mapped code (what L5 cannot reclaim)
    next_observation: FP0's idle-30m read after round 3 deploys, with store_quotable true
  blocker_or_risk: >-
    The panel (DEC-05), and FP8's ownership of vmmanager and the image for any
    build slice. The Firecracker 1.16.1 pmem facts marked "verify" in the
    report are unproven. Finding 5 (guard in hostauth off mode) stays with
    the hostauth owner.
  next_action: FP0 lands round 5 on top of round 4, deploys, and runs report §5 step 1.

receipts: []
---

# FP10 · RES-08: share the base OS in guest RAM

The problem: [base-os-copied-into-every-guest-ram-2026-10-22.md](../problems/base-os-copied-into-every-guest-ram-2026-10-22.md).
The design, threat analysis and measurement steps:
[res-08-shared-base-os-pmem-dax-design-2026-10-22.md](../reports/res-08-shared-base-os-pmem-dax-design-2026-10-22.md).
