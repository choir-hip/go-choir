# CI0 C6: runner slots (2026-10-25)

Station: CI0, slice C6. Mutation class of this note: green; every change it
proposes is in `.github/workflows/ci.yml`, which the worker token cannot push
(memory: ci0-worker-token-no-workflow-push) and which touches `check`'s
`needs` (red). Proposal for FP0, not implemented.

## What the logs show

- The account runs 20 jobs at once. When `plan` ends on a main push, 24 jobs
  are ready (16 test shards, scale, vet+build, doccheck, heresy, vocabulary
  gates, deploy-impact, the SBOM candidate) and `build-frontend` and
  `prebuild-staging` become ready 10 s later behind them.
- Baseline run 40138087758: jobs started within 2 s of plan's end except three
  shards (+12 to +26 s), `Build Frontend` (+28 s) and the prebuild at +62 s
  (22:21:51, plan ended 22:20:49). The prebuild is the deploy floor on main
  (finding 2 of docs/problems/ci-wall-clock-2026-10-24.md), so
  its 50 s of queueing is 50 s of push-to-deploy once the tests are under it.
- A pull-request run has no deploy jobs (prebuild, frontend, deploy-impact are
  skipped), 22 jobs ready, and all started within 3 s (run 40150248142:
  started at +56/+57 s, one at +74 s). Slots are not what limits the PR gate.
- `check` itself waited 56 s for a runner after its last dependency finished in
  run 40150248142 (created 01:30:24, started 01:31:20). Another run was using
  the account's slots then; nothing in the workflow explains it. It is one
  sample.

## Proposals (all red or touching red)

1. **Fold the three static gates** (doccheck 27-36 s, heresy-detector 19-27 s,
   vocabulary gates 10-16 s) **into one job** with three steps. Three slots
   become one for the first minute: two more of the queued shards or the
   prebuild start at once. `check` reads one result instead of three, and it
   must keep failing when any step fails. Expected: the prebuild starts at about
   +25 s instead of +62 s on main.
2. **Fold `go-test-scale` into `go-vet-build`'s tail** is not proposed: scale is
   an isolated load-sensitive lane by design (it exists so those tests run
   alone on a runner).
3. **Start `prebuild-staging` from `plan`** for pushes that are not docs-only
   (P3 of docs/design/ci-node-b-build-2026-10-24.md). Larger win (about
   50 s more), larger contract change.
4. **The SBOM candidate** holds a slot for about 10 min from t=0 and is off the
   deploy path; `sbom` jobs are excluded from this track. If FP0 touches them:
   `needs: [plan, deploy-impact]` would start the candidate after the quick
   jobs and let it queue behind the shards instead of in front of the
   prebuild.
5. **Contract tests in `plan`** (`.github/*` changes only): the plan job runs
   every CI contract test before any lane starts; with the shard planner's
   test it took 34-51 s in the PR runs (9 s on a main push that touches nothing
   under `.github/`). A separate job in parallel with the lanes, required by
   `check`, takes that off the start of the critical path for CI-tooling
   changes. The tests are cheap (the shard test alone is 13 s).

None of these changes what is tested.
