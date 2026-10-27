---
definition_version: 4

# Station V of docs/definitions/choir-consolidate-and-scale-metamission-2026-10-26.md.
# Owner 2026-10-27, in session: "This rescan process, is that necessary? I
# feel like we're crashing because of some BS"; "we should be getting rid of
# this garbage"; "Is the rescan even necessary? Can we just skip it?"; "when
# you bring the paper back, skip the rescan. Fix it and get rid of that dead
# code."
readiness: executable

review:
  reviewer: tier 2, review 361 (Devin and DeepSeek), then review 362 (Devin) on the fix
  frozen_ref: lane/vocab-boot c71c6fc3d (landed as 8f8b2a26c)
  verdict: accept (362, no new defect; 361's blocking finding, a resumed rebuild trusting an earlier process's rows, fixed)
  evidence_ref: /Users/wiz/Dev/lanes/_runs/bunch361/, bunch362/devin.out

start:
  observed_at: 2026-10-27 19:25 UTC
  canonical_ref: origin/main 9407fe4cc; deployed b9045445d (12 GiB guests)
  computers:
    - the paper computer-1355b83b…, back since 19:19:29 by an owner-consented disk edit of its vocabulary report (replay marker cleared, alias count 4 -> 12); 12 GiB shape
    - b@c computer-6c50d6e5…, refused a start at 19:15 for host memory (the 12 GiB paper); unconfirmed since
  dirty_worktrees: none in the main checkout
  open_lanes: lane/vocab-skip 0c7be0fc3 (rejected by review 360; not to land)
  known_failures:
    - a boot that replays one row on a fenced store sets a durable marker; a boot killed before the full pass finishes leaves it; every later boot pays the full pass (on the paper 23+ min, OOM at 8 GiB, unfinished at 12 GiB after 32 min)
    - deploys restart vmctl, which kills a protected computer's in-flight boot

finish:
  deliver: >-
    A computer's boot never runs the whole-store vocabulary pass, so a
    deploy or a crash cannot hold a computer down for a migration that
    finished in October.
  artifact: >-
    On main and deployed: the boot paths (internal/autoputer/run.go) arm the
    vocabulary write guard and check at most the rows this boot replayed,
    never blocking on a finding (a log line and a count instead); the
    cross-process replay marker (ReplayOpenedAt, noteVocabReplayDeposit's
    marker write) is deleted with its tests; the full migration remains
    only where a store is built from the whole tape (projectionbase
    rebuilder, rematerialize); interactive guests back to 8 GiB.
  acceptance:
    - action: >-
        A deploy refreshes the paper; its console shows the vocabulary
        phase under one second and the paper answers a guest route within
        two minutes of the refresh.
      proves: boot no longer pays for the migration.
      evidence_class: deployed proof
    - action: >-
        The paper and b@c both run on 8 GiB guests, and b@c starts with no
        hand stop.
      proves: the one-time memory raise is undone.
      evidence_class: deployed proof
  rollback: >-
    git revert of the train (the boot path returns to the fenced pass;
    reports keep their fields, so the old code reads them).
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize boot work that does not depend on what this boot changed,
    while keeping V2 vocabulary on every live write (the write guard) and
    the full migration where a store is rebuilt from V1-era tape.
  goodharting_would_be: >-
    Skipping the pass by a flag that stays in the code; raising memory
    until the pass fits; trusting a count instead of deleting the marker.

homotopy:
  realism_axis: >-
    From the store unit tests, through d@e's refresh, to the paper's
    deploy refresh.

boundaries:
  mutation_class: red
  authority_sources:
    - the owner's statements above (2026-10-27)
    - docs/choir-doctrine.md, CLAUDE.md
  must_preserve:
    - live writes refuse non-V2 role vocabulary (the write guard)
    - a store rebuilt from the tape is migrated before it serves
    - crash restarts close work; planned restarts resume
  excluded:
    - deleting the V1 -> V2 migration itself (rebuilds need it)
    - the 8 aliases the paper's replays recorded since 2026-10-26 21:54: a one-off repair, its own slice
  protected_surfaces:
    - boot path (internal/autoputer/run.go)
    - vocabulary fence and write guard (internal/store/vocab_*.go)

now:
  status: checkpoint_incomplete
  slice: landed and deployed; b@c start on 8 GiB not yet observed
  bridge_conjecture: >-
    If boot checks only what it replayed and every live write is guarded,
    then a resume boot cannot serve non-V2 vocabulary that the full pass
    would have caught, except references to IDs renamed by V1-era replay,
    which only a rebuild performs.
  believed_state: >-
    Deployed 8f8b2a26c (CI run 40575882836 success 20:24:49 UTC). The
    paper's planned refresh at 20:21:49 ran the vocabulary phase in 0 s
    (status=complete duration=0s), on an 8192 MiB guest, and was running
    desk tool loops by 20:24. Rebuilds resumed from a durable scratch
    distrust the fence and take the full pass.
  main_uncertainty: whether b@c starts with no hand stop on host memory now that the paper is back to 8 GiB
  next_observation: b@c's next start (owner's page or a scoped start) and vmctl's capacity decision
  next_action: observe b@c's next start; then settle complete
  residuals:
    - Devin 362 LOW, a seeded scratch resumed with nothing replayed still pays the full pass (cost only)
    - Devin 362 NIT, stale comments (rematerialize.go:208-210, vocab_replay_fence_test.go:19-28, FencedAt doc)
    - unfenced stores still take the one-time migration at boot (required by must_preserve)
    - the paper's 8 alias references recorded since 2026-10-26 21:54 (excluded; own slice)

receipts:
  - 49ddad6a2, 8a5f6f605, 8f8b2a26c on main; CI 40575882836 success; staging build 8f8b2a26c at 20:24:50 UTC
  - paper console 2026-10-27 20:21:49 "startup phase=vocabulary-migration status=complete duration=0s"; mem_size_mib 8192
