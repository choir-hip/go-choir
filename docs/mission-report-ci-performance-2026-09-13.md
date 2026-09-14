# Mission Report: CI Performance

Date: 2026-09-13 UTC
Status: complete — verified on hosted runs
Mutation class: red (deployment routing, CI check topology)
Problem doc: docs/problems/ci-deploy-critical-path-and-ungated-lanes-2026-09-13.md

## Mission goal and artifact

Improve CI performance: reduce wall-clock time and runner-minutes of the
`ci.yml` pipeline on `main` pushes without weakening any gate, receipt, or
identity check.

## Value criterion

Minimize push-to-deployed wall time and wasted runner-minutes while preserving:
test gate before activation, exact commit identity in all deployed artifacts,
fail-closed receipts, conservative classification for unknown paths.

## Belief state (conjecture ledger)

| # | Claim | Test | Status |
|---|-------|------|--------|
| C1 | Deploy critical path is dominated by the on-host NixOS closure build | Deploy phase logs | supported: 751s/870s in run 34779534767 |
| C2 | The Nix build is pure and can run before `check` completes | Nix semantics; identical commit+attrs -> identical store paths | active |
| C3 | `scripts/*` non-deployed paths trigger full host+guest deploys via `*)` catch-all | Classifier run on e27a217a | supported: deploy_needed=true for a report generator |
| C4 | `go-test-runtime` missing `go` gate is drift, not intent | git history; sibling jobs gated; `check` ignores its result when go=false | supported |
| C5 | Prebuild on Node B during tests hides ~12min from critical path | Measure deploy `nix build` phase on next host deploy | proposed |

## Timeline

- 2026-09-13: measured 12 recent main runs; identified deploy nix build (751s),
  scripts/* misclassification, ungated go-test-runtime as the three load-bearing
  inefficiencies. Wrote problem doc.

- 2026-09-14 (UTC): dispatch run 34803283760 verified prebuild+deploy overlap:
  deploy 7m51s vs ~14m38s baseline; nix build phase 309s vs 751s; staging
  serves 8c6858ba. a4e7358f moved nix store gc into prebuild (off the deploy
  critical path). Push run 34804467147 green with prebuild/deploy skipped.

## What shipped

- `58e149cb` green(docs): problem doc + this report (documentation-first).
- `bca48e0e` red(ci): deploy-impact-classify — scripts/guest-signer-* map to
  the guest boot contract; scripts/{check-*.sh,lint,generate_*.py,m3_*,mail-*}
  ignored as non-deployed tooling; catch-all stays conservative.
- `8c6858ba` red(ci): go-test-runtime gated on needs.plan.outputs.go;
  prebuild-staging job (continue-on-error, own concurrency group, disjoint
  /opt/go-choir-prebuild checkout, GC roots under
  /var/lib/go-choir/prebuild-results) builds frontend/host-services/toplevel
  per deploy-impact outputs while tests run.

## Evidence

- Push run 34802632517 (8c6858ba): success; race topology exercised; contract
  tests pass; prebuild/deploy correctly skipped (no deploy-impacting paths).
- Dispatch run 34803283760 (force_staging_deploy): success end-to-end.
  - Prebuild Deploy Artifacts: 03:39:36 -> 03:53:11 (~13.5min incl. first-run
    clone of /opt/go-choir-prebuild).
  - Deploy to Staging: 03:47:30 -> 03:55:21 (**7m51s**, baseline ~14m38s).
  - Deploy `nix build` phase: **309s vs 751s baseline** — the phase ended 1s
    after prebuild completion, i.e. the deploy's own nix build realized the
    warm store paths instantly (content-addressed hit, no marker needed).
  - Staging health: choir.news serves 8c6858ba3969c0a4be548e0f8976594b9b3dc519.
- Push run 34804467147 (a4e7358f): success; prebuild/deploy skipped.
- Deploy savings scale with test duration: prebuild hides min(tests, build)
  from the critical path; on race-selected runs (~8min tests) roughly half the
  ~12.5min build is hidden, on longer test runs more.

## Residual risks

- Prebuild competes with an in-flight deploy for Node B CPU (accepted: builds
  are pure; activation is unaffected).
- Prebuild result symlinks are GC roots under /var/lib/go-choir/prebuild-results;
  bounded to one fixed name per attribute class.
- Node B store sits near min-free (auto-GC freed ~86GB during run 34779534767's
  switch); a4e7358f moved `nix store gc` into prebuild so collection happens
  off the deploy critical path and pre-frees space for the build.
- The ~750s build itself is unchanged: every commit rebuilds all Go services
  because buildCommit is embedded via ldflags (required for identity
  verification). Shrinking it further needs infra (remote builders/binary
  cache) or product changes (fewer services, faster tests) — outside this
  mission's authority boundary.

## Rollback refs

- Revert bca48e0e, 8c6858ba, a4e7358f via PR; prebuild job is additive-only.
