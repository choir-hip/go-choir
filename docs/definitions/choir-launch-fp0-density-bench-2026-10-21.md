---
definition_version: 4

# FP10/FP0 support of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): RES-19 and RES-07 are
# measurements FP0 runs on Node B; RES-21 is a doc reconcile. This lane
# writes the scripts and the exact steps. It runs nothing on Node B.
readiness: drafted

review:
  reviewer: none; green docs and read-only scripts, plus one flag-gated benchmark
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-22T00:00:00Z
  source:
    canonical_ref: main at d1c60ad87 (lane base)
    deploy_identity: not_applicable (nothing here is deployed)
  worktrees:
    - path: /home/agent/lanes/fp0-density-bench
      status: clean at start
      class: goal_candidate
      owner: FP0-support worker
      touch: goal_owned
      recovery: branch lane/fp0-density-bench

finish:
  deliver: >-
    RES-21: the docs that misstate guest sizes say what the code and Node B
    set. RES-19 and RES-07: scripts under scripts/density/ and a runbook
    (scripts/density/README.md) so FP0 can take the host and guest memory
    readings, and the density benchmark's idle ladder, on Node B in one pass.
  artifact: >-
    The station file; the RES-21 doc commit; scripts/density/ (read-only
    by default, one flag-gated VM-creating benchmark); the README with the
    exact steps, the safety stops and the named gaps.
  acceptance:
    - action: python3 -m py_compile scripts/density/*.py and the scripts' --help, plus the guard's refusal cases run against canned vmctl list JSON (scripts/density/test_guard.py)
      proves: the scripts parse, and a creating step refuses the paper, the owner computer, the delegate and any non-test computer
      evidence_class: unit
    - action: FP0 runs host_snapshot.py and guest_memory.py on Node B per the README, then density_bench.py --create-vms for the idle ladder
      proves: RES-19's idle-ladder numbers and RES-07's page-cache split (or its named gap)
      evidence_class: deployed proof
  rollback: git revert of the lane commits; nothing deployed
  landing:
    required: false
    environment: not_applicable
    required_receipts: [pushed_commit]

value:
  better_means: >-
    Numbers in vm-density-limits §3 and §4 that replace estimates, taken
    without risking a protected computer, and docs that agree with the code.
  goodharting_would_be: >-
    A benchmark that "passes" because it skipped its safety stops; a guard
    that matches the paper by name and so passes a renamed one; claiming
    the fincore read is done when the guest has no route for it.

homotopy:
  realism_axis: >-
    Canned-JSON guard tests, then a read-only run on Node B, then one
    minted test computer, then the ladder.

boundaries:
  mutation_class: green (docs, read-only scripts); the flag-gated benchmark creates test computers only when FP0 passes --create-vms
  authority_sources:
    - docs/security-density-architecture-checklist-2026-10-21.md items RES-07, RES-19, RES-21
    - docs/reports/vm-density-limits-2026-10-14.md §4 and Residuals
    - docs/archive/definitions/choir-ledger-lr-resource-2026-10-13.md (source sweep, oversubscription probe)
    - docs/archive/reports/lr-guest-memory-return-2026-10-14.md (staging proof plan, steps 3-6)
  must_preserve:
    - A creating step runs on test computers only and refuses the paper, the owner computer and the delegate computer-ad1b46d4c20129e84b9e239cf4720fb9.
    - Reads are read-only; no secret is printed; no SQL, no tape edits, no guest block-device access.
    - The benchmark stops on the report's safety stops (PSI some avg10 > 1.0, MemAvailable below floor + 2 GiB, a non-test computer reclaimed, CPU above 80% for 60 s).
    - No ssh from a worker; FP0 runs these.
  excluded:
    - Running anything on Node B. Adding a fincore route to the guest (orange, guest image; named as a gap).
    - Benchmark steps 2 to 4 beyond a stub (hibernate and wake waves, busy ladder, deploy storm): they need a fixed workload and an announced window.
  protected_surfaces: []

now:
  status: ready_for_fp0
  slice: >-
    RES-21 done (1fae88623: four reports corrected; dated problem records
    left as point-in-time). scripts/density/ written and unit-checked
    (py_compile, --help, test_guard.py, the collector run locally); nothing
    has run on Node B. Waiting on FP0's runs (scripts/density/README.md steps
    1 to 3) and on one owner question: add a guest fincore field to procmem?
  source_ref: d1c60ad87
  deploy_identity: unchanged
  candidate:
    id: FP0-density-bench
    state: built, not run on Node B
    ref: lane/fp0-density-bench
    scope: [docs, scripts/density]
  conjecture:
    id: C-res19-measurable
    claim: >-
      The idle ladder and the memory split can be read from Node B with the
      existing vmctl list, the guest's host-sourced procmem route and /proc
      alone, with no new code on the host or in the guest.
    test: FP0 runs the README steps and gets a JSON receipt per sample.
    edge: missing_oracle (no guest fincore route; page-cache attribution is correlational)
    delta_o: a procmem fincore field, an orange guest change, if the owner wants the split
