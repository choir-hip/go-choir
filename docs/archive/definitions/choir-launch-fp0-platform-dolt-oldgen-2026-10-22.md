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
    docs/archive/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md
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
    Review round 1 (bunch 75: Sonnet, DeepSeek, flash; accept with edits)
    repaired: findings recorded (1dfa3c38f), tests first (0fd3f3ab0,
    f45e4248e), fixes (c4fe5db2d), record §4 revised. Review round 2
    (bunch 90, DeepSeek): R9 an unread receipt count passed the drill,
    fixed (504ed930b, 4b7128bf6, e16099811); R10 newgen wording fixed,
    the shared 8 GiB line a residual (§4 step 1 covers it). Review round 3
    (bunch 103, DeepSeek): F1 to F3 fixed (b234006f7, 07ef10ec3,
    ea77517f7): the drill validates the recorded receipt count and its
    scratch directory. The switch stays
    inert until the env file exists. Waits on FP0's landing (not a
    host-only deploy: projectionbase maps to vmctl and the guests) and the
    owner's flip, after the first-hour auto GC check in record §4.
  source_ref: ea77517f7
  deploy_identity: unchanged
  candidate: {id: pdolt-snapshots-off, state: pushed, ref: lane/fp0-platform-dolt-oldgen, base: f0b1ee313, digest: none, scope: [internal/doltbatch/committer.go, internal/doltbatch/committer_test.go, internal/platform/store.go, internal/platform/snapshot_mode_test.go, internal/projectionbase/squash.go, internal/projectionbase/squash_test.go, nix/node-b.nix, scripts/dolt-history-audit.sh, scripts/test-dolt-history-audit.sh, scripts/off-host-backup.sh, scripts/off-host-backup-dolt-snapshot-test.sh, scripts/off-host-restore-drill.sh, .github/scripts/deploy-impact-classify, .github/scripts/deploy-impact-classify-test, docs/archive/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md, docs/archive/definitions/choir-launch-fp0-platform-dolt-oldgen-2026-10-22.md]}
  conjecture:
    id: C-pdolt-snapshots-off
    claim: >-
      With PLATFORM_DOLT_SNAPSHOTS=off, Store A's oldgen stops growing
      between squashes, with no lost write and no failed backup, and the
      squash can fold the working set on every later run.
    test: auto GC line and journal under 256 MiB within 2 h of the flip; oldgen du and dolt_log flat for 24 h; total noms growth per day recorded; auto GC under 60 s; the backup manifest holds dolt_rows; corpusd and the paper healthy.
    edge: missing_oracle (no Node B reading since the flip; 2.4.2 read in source, not yet observed)
    delta_o: record §4 step 2
    scope_if_supported: Store A
    status: active
    evidence_refs: [docs/archive/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md]
  decision:
    what: stop Store A's snapshots at the source (switch, off by file); a supervised squash on newgen 4 GiB, slow auto GC or monthly
    kind: operational
    status: proposed
    evidence_ref: record §4
    owner_ratification_ref: pending (the flip and the squash are the owner's)
  belief:
    believed_state: >-
      The rate is most likely 10-15's; the cause is the snapshot commits;
      the switch removes history overhead but not live growth (D), whose
      daily rate is unmeasured.
    main_uncertainty: total noms growth after the flip (a retention question if near 1.6 GB a day); the 6.2 GB file; vmctl and guests with Store A down.
    next_observation: FP0's pre-landing du readings and §1 readings.
  blocker_or_risk: Node B steps are FP0's and the owner's.
  next_action: FP0 reads du (record §4 step 1), lands the lane, proves no effect; the owner decides the flip.

receipts: []
---

# FP0 lane: platform-dolt's oldgen

Everything is in
[platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md](../problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md):
§1 the cause and the readings, §3 the runbook, §4 the switch and the
cadence.
