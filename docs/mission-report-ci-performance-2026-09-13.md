# Mission Report: CI Performance

Date: 2026-09-13 UTC
Status: in progress
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

## What shipped

(pending)

## Evidence

(pending — CI run ids, deploy phase timings)

## Residual risks

- Prebuild competes with an in-flight deploy for Node B CPU (accepted: builds
  are pure; activation is unaffected).
- Prebuild result symlinks are GC roots under /var/lib/go-choir/prebuild-results;
  bounded to one fixed name per attribute class.

## Rollback refs

- Revert commits via PR; prebuild job is additive-only.
