---
definition_version: 4

# FP10 of the launch first pass (choir-launch-first-pass-metamission-2026-10-21.md),
# items RES-13 (snapshot levers L3, L4, L20) and RES-22 (host slab and
# btrfs metadata, L19) of docs/security-density-architecture-checklist-2026-10-21.md.
# Both are P2 and red. FP0's assignment: both need host or guest readings
# this lane cannot take (no Node B), and vmctl and vmmanager are FP8's
# (lane/fp8-image-jailer), so this station writes the design and FP0's
# read-only measurement commands, and stops.
readiness: drafted

review:
  reviewer: none; docs only (green). Each build slice is red and goes to FP0's panel on its frozen diff.
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-22T00:30:00Z
  source:
    canonical_ref: main d1c60ad87 (lane base)
    deploy_identity: not read by this lane (FP0's ledger names 53a691be6 serving, FP3 group 1 deploying)
  worktrees:
    - path: /home/agent/lanes/fp10-res13-res22-density
      status: clean at start
      class: goal_candidate
      owner: FP10 worker
      touch: goal_owned
      recovery: branch lane/fp10-res13-res22-density

finish:
  deliver: >-
    A hibernate writes only the guest's resident pages; an idle computer
    hibernates after minutes, not half an hour; a guest-artifact deploy no
    longer turns every wake into a cold boot; host slab and btrfs metadata
    stop growing with the tape's file count.
  artifact: >-
    This pass: two problem records, the design
    (docs/design/fp10-snapshot-levers-and-host-slab-2026-10-22.md) and FP0's
    read-only measurement commands. Later passes: one red commit set per
    slice in the design's order, each off by default, with a failing test
    first and FP0 proof steps.
  acceptance:
    - action: FP0 runs the design's section 4 readings on Node B and records them in the two problem records
      proves: the numbers each slice's gate needs (resident fraction, swap guarantee, fallback reasons, slab and file growth)
      evidence_class: deployed reading
    - action: per build slice, its focused unit tests, then FP0's staging proof on a test computer
      proves: the slice's claim on the deployed product
      evidence_class: deployed proof
  rollback: >-
    This pass is docs only (git revert). Every build slice is a flag whose
    off value is today's behaviour.
  landing:
    required: false (docs only this pass)
    environment: staging
    required_receipts: [pushed_commit]

value:
  better_means: >-
    Fewer bytes written per hibernate, less RAM held by idle computers and
    by host slab, fewer cold boots after a deploy, while preserving guest
    memory integrity across hibernate and wake, the snapshot validity
    rules, and the restart rules.
  goodharting_would_be: >-
    A Diff snapshot that loses pages a restored guest needed; a shorter
    idle timeout that hibernates working computers; resuming a snapshot
    onto a guest release a security deploy meant to retire; raising
    vfs_cache_pressure so the paper's Dolt pages are evicted instead.

homotopy:
  realism_axis: >-
    Readings, then unit tests with a fake Firecracker API, then a test
    computer on staging with guest memory checksums, then every class.

boundaries:
  mutation_class: green this pass; each build slice red (vmctl/vmmanager, deployment routing for L20)
  authority_sources:
    - owner 2026-10-21 (the metamission quotes)
    - docs/reports/vm-density-limits-2026-10-14.md (levers L3, L4, L19, L20; order section 4)
    - docs/reports/s3-snapshot-hibernate-design-2026-10-14.md (validity rules, failure modes 7, 8, 17)
  must_preserve:
    - The metamission's must_preserve list, in full.
    - A snapshot loads only if every validity rule in validateSnapshot holds; no slice weakens a rule, only adds the facts it compares.
    - Guest memory never swaps while a Diff snapshot may be taken from it.
    - No edit to internal/vmctl, internal/vmmanager or the VM image while FP8 holds them.
  excluded:
    - RES-14 (the budget's byte count) and every FP8 item.
    - The tape repack (choir-tape-segments-a2-repack-2026-10-21.md owns it).
  protected_surfaces: [vmctl, deployment routing]

now:
  status: blocked
  slice: >-
    Design and FP0's read-only readings written (design section 4);
    nothing built. The build waits on the readings and on FP8 releasing
    vmmanager; two owner questions with defaults (design section 5).
  source_ref: d1c60ad87
  deploy_identity: unchanged
  candidate: {id: none, state: none, ref: none, base: d1c60ad87, digest: none, scope: []}
  conjecture:
    id: C-FP10-sparse
    claim: >-
      A Diff snapshot without dirty tracking of a cold-booted guest whose
      memory cannot swap holds every page the guest can read, so it
      resumes identically while writing only resident pages.
    test: >-
      On a test computer: write a known pattern into guest memory, hibernate
      by Diff, wake, check the pattern and the guest's health; compare
      st_blocks, write time and PSI with a Full hibernate of the same guest.
    edge: missing_oracle (no reading of the resident fraction or of the jailer cgroup's swap setting)
    delta_o: design section 4, readings R2 and R5
    scope_if_supported: cold-booted computers on Node B, class:test first
    status: active
    evidence_refs: [docs/problems/hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md]
  decision:
    what: design only; slice order L3, then L4, then L20; RES-22 is readings, then an alert, then (only if readings show it) vfs_cache_pressure
    kind: operational
    status: settled
    evidence_ref: FP0's assignment to this lane (2026-10-21)
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      L3 is a small vmmanager change behind a flag once two readings come
      back. L4 is config, gated on L3. L20 is a medium change across
      vmmanager, the deploy and node-b.nix, and has an owner question. The
      btrfs file growth already has its substrate fix (tape segments, on);
      what RES-22 lacks is the reading that shows it worked, and an alert.
    main_uncertainty: whether the jailer (FP8) keeps guest memory out of swap, and how much of a cold guest is resident.
    next_observation: FP0's section 4 readings.
  blocker_or_risk: FP8 holds vmmanager; Node B readings are FP0's.
  next_action: FP0 takes readings R1 to R7 and S1 to S5; then the L3 slice once FP8 lands.

receipts: []
---

# FP10: snapshot levers and host slab

The design, the owner questions and FP0's read-only commands are in
[docs/design/fp10-snapshot-levers-and-host-slab-2026-10-22.md](../design/fp10-snapshot-levers-and-host-slab-2026-10-22.md).
The problems are
[hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md](../problems/hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md)
and
[node-b-host-slab-btrfs-inode-cache-2026-10-22.md](../problems/node-b-host-slab-btrfs-inode-cache-2026-10-22.md);
the btrfs metadata record is
[node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md](../problems/node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md).
