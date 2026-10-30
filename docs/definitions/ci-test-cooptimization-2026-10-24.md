---
definition_version: 4
readiness: executable
review:
  reviewer: none (the goal file is tier 0; each candidate carries its own tier, below)
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-24T23:55:56Z
  source:
    canonical_ref: main 5e11072c5
    deploy_identity: staging 9cab8e56f (CI 40138087758)
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty (M go.sum, ?? scratchpad/)
      class: user_wip
      owner: owner
      touch: forbidden
      recovery: leave in place
  baseline:
    run: 40138087758, push to main of 9cab8e56f
    timeline_utc: >-
      created 22:19:54; plan 22:20:40 to 22:20:49; prebuild (Node B nix
      build) 22:21:51 to 22:28:55; non-runtime race shard 1 22:20:51 to
      22:38:09; deploy-staging 22:38:15 to 22:39:47. Push to deployed: 19.9 min.
    jobs_minutes:
      race_non_runtime: {s1: 17.3, s4: 9.1, s0: 6.9, s2: 6.6, s3: 6.0, s5: 5.5, s6: 2.8, s7: 2.1}
      race_agentcore_textureowner: 3.6 to 4.8 (8 shards)
      prebuild_node_b: 7.0
      sbom_candidate: 10.4 (off the deploy path; deploy needs check, build-frontend, deploy-impact)
      deploy_staging: 1.5
    facts:
      - every test lane runs -count=1, so Go's test-result cache is never used
      - the Actions cache holds 10.2 GB against the 10 GB repository limit, so entries evict and some runs start cold (2026-10-22 finding 4, again true today)
      - pull-request runs execute every test job and never deploy; main runs serialize in one concurrency group
      - the pre-land mirror on the agents VM runs light -race only and skips agentcore and store ("GitHub PR CI shards it"); FP0 has been landing without a PR run, so heavy-race failures surface on main (W2, 2026-10-24) and cost a fix-forward cycle
      - the agents VM has 10 GiB memory and its /home is 96% full (go-build cache 76 GB)
    prior_work:
      - docs/problems/ci-wall-clock-2026-10-24.md (fix landed as 17a7b353f and 1ed31abef; its status line is stale; shard 1 is the new outlier)
      - docs/archive/choir-ci-optimization-2026-07-16.md

finish:
  deliver: >-
    Every change reaches a verified staging deploy in half the time it takes
    today or less, failures are caught before main instead of after, and the
    suite carries fewer, faster, sharper tests, with no real failure mode
    losing its test.
  artifact: >-
    Landed changes to the shard planners, the CI test jobs and caches, the
    landing procedure (a pull-request run as the pre-land gate), the test
    suite, and the pruning manifest docs/evidence/ci-test-pruning-2026-10-24.md
    listing every deleted or merged test with its category and reason.
  acceptance:
    - action: median of the first five code pushes to main after each landed slice, created to deploy-staging complete, from gh run view --json jobs
      proves: push-to-deploy wall clock against 19.9 min (target 10, stretch 5)
      evidence_class: deployed CI timing
    - action: median pull-request run, created to check complete
      proves: pre-land gate latency (target under the review latency, about 10 min)
      evidence_class: CI timing
    - action: count of main CI runs that fail on a test after a green pre-land gate, over the track
      proves: failures move before main (target zero)
      evidence_class: CI history
    - action: the pruning manifest, one row per removed test
      proves: each removal names what it covered and why that coverage stays (duplicate of a named test, tests deleted code, superseded by a named test, asserts nothing)
      evidence_class: review of the manifest
  rollback: git revert of each landed commit; the planners, workflow and procedure are plain text
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the time from a finished change to a verified staging deploy,
    including the cycles lost to failures found only on main, while every
    real failure mode keeps a test that runs under -race before the change
    can deploy.
  goodharting_would_be: >-
    Skipping tests, moving them to report-only or nightly jobs, dropping
    -race, adding t.Skip, loosening the hang guards, narrowing plan's path
    filters so code changes stop running tests, deleting tests by age rather
    than by what they cover, or a "reuse" that deploys a tree no green run
    tested.

homotopy:
  realism_axis: >-
    the CI logs of past runs, then a draft pull-request run, then a main push
    with its deploy, then the median of five main pushes

boundaries:
  mutation_class: >-
    orange for the shard planners, the test jobs and caches and test-only
    code; yellow for test deletions with the manifest; red for anything
    that decides whether a deploy may proceed (check's needs, the green-tree
    reuse in C3) or the Node B build (C4)
  authority_sources:
    - owner, 2026-10-24 ("an overnight work track for Sonnet to optimize CI, actually co-optimize CI and tests ... cut wall clock time in half or even by 3/4")
    - owner, 2026-10-24 ("accelerate it to come before them ... running this now on the VM on Node A"; "intelligently design this, noting that if we speed up CI, we'll accelerate everything")
    - owner, 2026-10-24 ("if we're adding something to CI ... not further lengthening the wall clock time for CI")
    - CLAUDE.md, Mutation Classes and Landing Loop
  must_preserve:
    - every test still runs under -race before the change it can catch deploys
    - the repo gates (check-v1-writer-purity, check-decode-roots, check-test-hang-guards, selftest-guest-token) stay required in check
    - check fails closed; an empty shard selection fails (1ed31abef)
    - no deploy of a tree that no green run tested
    - no deletion of a test that is the only coverage of a failure mode
  excluded:
    - edits to deploy-staging, prebuild-staging, deploy-impact, sbom, flakehub and the Node B preflight (red; findings go to FP0 as proposals)
    - product code, except test helpers and test-only seams
    - pushing to main; only FP0 lands
  red_surface_tests: >-
    Tests of tape/evidence, auth/session, restore and checkpoint, Texture
    canonical writes, store schema, vmctl and gateway calls may be sped up
    (fake clocks, no sleeps, shared fixtures) but are only proposed for
    deletion, in the manifest's "proposed" section; FP0 adjudicates with a
    Devin seat.

now:
  status: waiting_for_FP0
  slice: C1-C6 done as far as the worker can take them (C2 candidate 5 measured, C3 code off, C4 and C6 proposals, C5 manifest with no removals)
  candidate: {id: ci0-c2-e, state: proven on draft PR #104 run 40153944994 (all green, check done 504 s), not landed}
  conjecture:
    id: CI0-C2
    claim: with the shard maximum near 7 min the deploy is bound by the Node B prebuild (about 9.3 min plus 1.5 min deploy)
    test: pull-request run test steps per shard, then the median of five main pushes after landing
    edge: missing_oracle
    delta_o: per-test Elapsed and whole-machine CPU use from the PR run logs
    scope_if_supported: C4 (incremental Go build) and C3 (prove once) are what take the deploy below 10 min
    status: supported on the pull-request side (504 s created to check vs 18.3 min baseline); the main-push median awaits landing
  belief:
    believed_state: >-
      tests are no longer the deploy's critical path once the lane lands and
      the caches hold the export-data objects; push-to-deploy should be about
      11 min, prebuild-bound; the cache prune (workflow patch) is unmeasured
    main_uncertainty: what the 7 min Node B closure build spends (no per-derivation timing exists), and whether the cache prune keeps the Actions cache under 10 GB
    next_observation: FP0 applies docs/evidence/ci0-workflow-ci-yml.patch to the PR branch, reads the prune lines and gh cache list, then lands; the median of the first five main pushes
  next_action: FP0 lands the lane (all non-workflow commits are pushable), applies the workflow patch, runs the prebuild timing change in docs/design/ci-node-b-build-2026-10-24.md (P0)

receipts: []
---

# CI and test co-optimization (CI0)

Faster CI accelerates every other station, so this track runs first, in
parallel with the readiness waves, and its slices land between them. A Sonnet
worker runs it on the agents VM, measuring from the CI logs and proving each
candidate on a draft pull-request run; FP0 reviews and lands.

## Where the time goes, and the order of slices

Push to deploy is 19.9 min today. The tests end at 17.3 min only because one
shard does; the next slowest ends at 9.1. Behind the tests sits the Node B
build (7 min, starting about 2 min in). So the slices are ordered by how much
wall clock each removes:

- **C1, measure.** The per-test table for every shard from the last eight
  race runs; the prebuild's phase times; the cache keys and sizes; the
  landing loop as FP0 runs it (mirror, review, CI, deploy, verification).
- **C2, the outlier and the cache.** Split or speed whatever holds shard 1;
  fit the Actions cache under 10 GB so no lane starts cold. Expected: push
  to deploy about 11 min, prebuild-bound. Tier 1.
- **C3, prove once.** The pull-request run becomes the pre-land gate, run in
  parallel with review, replacing the light mirror, so heavy -race failures
  surface before main. The main push of a tree whose identical git tree
  already passed a pull-request run reuses that verdict instead of running
  the tests again; check verifies the green run by tree hash through the
  API and fails closed if it cannot. Expected: main push to deploy is the
  prebuild floor, about 8 min, and fewer fix-forward cycles. Red (it decides
  whether a deploy proceeds), behind a workflow input, tier 1 with a design
  note FP0 reviews first.
- **C4, the Node B build.** Find what the 7 min nix build spends (it
  rebuilds the Go binaries from scratch every deploy) and propose an
  incremental build. Red, proposal only; FP0 builds or adjudicates it. This
  is the step from one half to three quarters.
- **C5, prune and speed.** Remove tests that are duplicates, test deleted
  code, are superseded, or assert nothing; speed slow ones (sleeps, per-test
  Dolt setup, serial suites that could be parallel). Each removal in the
  manifest. This lowers pull-request latency and runner minutes, and keeps
  C3's gate under the review time. Tier 0 with the manifest outside red
  surfaces.
- **C6, runner slots.** The account runs 20 jobs at once and a run asks for
  about 25; fold tiny jobs and keep report-only jobs off the first minutes.

Out of scope as a lever: test-result caching across runs (every lane runs
-count=1 on purpose); C1 may measure what it would save, as a proposal.
