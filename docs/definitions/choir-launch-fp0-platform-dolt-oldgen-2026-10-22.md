---
definition_version: 4

# A lane of FP0 in the launch first pass (choir-launch-first-pass-metamission-2026-10-21.md):
# platform-dolt's oldgen. FP0's assignment (2026-10-22): root cause from
# code, the supervised squash runbook, a lasting cadence; build the source
# fix if clear and small.
readiness: drafted

review:
  reviewer: none yet; the switch (90ea1e378) is red and goes to FP0's landing panel
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-22T17:00:00Z
  source:
    canonical_ref: main f0b1ee313 (lane base)
    deploy_identity: not read by this lane
  worktrees:
    - path: /home/agent/lanes/pdolt
      status: clean at start
      class: goal_candidate
      owner: FP0 platform-dolt worker
      touch: goal_owned
      recovery: branch lane/fp0-platform-dolt-oldgen

finish:
  deliver: >-
    Store A's oldgen stops growing between squashes, and the history it
    holds now is dropped once by a supervised squash.
  artifact: >-
    docs/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md
    (cause, writer map, readings, runbook, cadence); doltbatch Mode and
    PLATFORM_DOLT_SNAPSHOTS (90ea1e378).
  acceptance:
    - action: FP0's read-only readings (record §1)
      proves: H1 (conjoin artifact, rate as on 10-15) or H2 (a real rise)
      evidence_class: deployed reading
    - action: land 90ea1e378 inert, then the owner writes the env file (record §4 step 2)
      proves: oldgen du and dolt_log flat for a day, auto GC under a minute, backup ok
      evidence_class: deployed observation
    - action: the owner's supervised squash (record §3)
      proves: root hash identical, commits 1 or 2, services back, the space returned after the backup is deleted
      evidence_class: deployed run
  rollback: delete /var/lib/go-choir/platform-dolt-snapshots.env and restart corpusd; git revert 90ea1e378; the squash's reflink backup.
  landing:
    required: true (the switch changes corpusd and checkpointd, inert as deployed)
    environment: staging
    required_receipts: [pushed_commit, ci_run, deploy, no_effect_proof]

value:
  better_means: Node B's disk stops losing about 1.6 GB or more a day to Dolt history nobody reads.
  goodharting_would_be: raising the audit's 50 GiB alarm; deleting oldgen files by hand; a squash without the backup.

homotopy:
  realism_axis: code reading, unit tests on the embedded driver, then Node B readings, the flip and the supervised squash.

boundaries:
  mutation_class: green (record, runbook), yellow (tests), red (the switch)
  authority_sources:
    - FP0's assignment (2026-10-22)
    - owner notes 2026-10-14 §23 and §27 item 7 (Store A squashed with a backup first)
  must_preserve:
    - The metamission's must_preserve list, in full.
    - No SQL against production Dolt; the squash and the flip are the owner's call.
    - Snapshots stay on unless the env file says off; an unknown value keeps them on.
  excluded:
    - Store B's committer (same mechanism, corpus-dolt 6.3 GB).
    - The paper's guest store and the other disk holders.
  protected_surfaces: [Store A canonical writes, checkpoint boundary commits, corpusd and checkpointd units]

now:
  status: blocked
  slice: >-
    Record, tests and the inert switch pushed; runbook and cadence written.
    Waits on FP0's readings, landing, and the owner's flip and squash.
  source_ref: 90ea1e378
  deploy_identity: unchanged
  candidate: {id: pdolt-snapshots-off, state: pushed, ref: lane/fp0-platform-dolt-oldgen, base: f0b1ee313, digest: none, scope: [internal/doltbatch/committer.go, internal/doltbatch/committer_test.go, internal/platform/store.go, internal/platform/snapshot_mode_test.go, nix/node-b.nix, docs/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md, docs/definitions/choir-launch-fp0-platform-dolt-oldgen-2026-10-22.md]}
  conjecture:
    id: C-pdolt-snapshots-off
    claim: >-
      With PLATFORM_DOLT_SNAPSHOTS=off, Store A's oldgen stops growing
      between squashes, with no lost write and no failed backup.
    test: oldgen du and dolt_log count flat for 24 h after the flip; auto GC durations under 60 s; corpusd and the paper healthy; the 02:30 backup succeeds.
    edge: missing_oracle (no Node B reading since the flip; server 2.4.2 against the 2.2.0 code read)
    delta_o: record §4 step 2
    scope_if_supported: Store A
    status: active
    evidence_refs: [docs/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md]
  decision:
    what: stop Store A's snapshots at the source (switch, off by file) rather than a frequent squash timer; one supervised squash for the history already there
    kind: operational
    status: proposed
    evidence_ref: record §4
    owner_ratification_ref: pending (the flip and the squash are the owner's)
  belief:
    believed_state: >-
      The rate is most likely 10-15's (36 GiB is what 1.6 GB a day from
      24.4 G predicts; the 25.7 GB file reads as a conjoin). The cause is
      the snapshot commits; no new writer since 10-15.
    main_uncertainty: the 6.2 GB file at 14:54; Store A's live set size; vmctl and guests with Store A down.
    next_observation: FP0's readings (record §1).
  blocker_or_risk: Node B steps are FP0's and the owner's.
  next_action: FP0 reads §1, lands 90ea1e378 in an inert batch; the owner decides the flip and the squash window.

receipts: []
---

# FP0 lane: platform-dolt's oldgen

Everything is in
[platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md](../problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md):
§1 the cause and the readings, §3 the runbook, §4 the switch and the
cadence.
