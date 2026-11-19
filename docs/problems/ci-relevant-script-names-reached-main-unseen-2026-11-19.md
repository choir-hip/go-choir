# Scripts that CI must run for reached main classified as docs (2026-11-19)

## Problem

CI's plan job runs `.github/scripts/ci-impact-classify-test`, which scans
the tree for scripts named by deployed, CI, Go or kept files and fails when
one of them still classifies as docs-like (a push of only docs-like paths
skips CI and the deploy). Nothing ran that test before a push unless the
push touched `.github/`. Five such names were on main or on the way:

- `scripts/proofs/corpus-search-recall`, run by a Go test of the ranked
  search (`internal/platform/capture_rank_test.go`);
- `scripts/status-vectors`, named by `internal/platform/vector_worker.go`;
- `scripts/autoradio-native-ceiling.txt` and
  `scripts/autoradio-native-voice-list.txt`, read by the native-boundary
  gate that `scripts/gates` runs;
- `scripts/e1`, run by `scripts/selftest-e1`, which `scripts/gates` runs.

## Evidence

CI run 44506541473 on `7988d2473`, the first to cover the vector train and
the experiment driver, failed in the plan job two minutes in, naming the
first of them. Fixing each locally showed the next. Main was red and the
train could not deploy until the repair.

## Belief

The test is a contract between the tree and the classifier, and the only
place it ran was after the push. A builder adds a script and a reference in
one change; neither the builder's checks, nor review, nor `scripts/land`
compared the two.

## What was done, and the remaining error

- The five names are listed in `.github/scripts/ci-docs-like-paths`, so a
  change to any of them runs CI.
- `.github/scripts/deploy-impact-classify` ignores them (with the driver's
  self-test and fixtures) as tooling no host or guest closure reads, so a
  change to one alone is not a deploy.
- `scripts/gates` runs the contract test on every landing (about 25
  seconds).
- Remaining: `scripts/gates` and other gate tooling still classify as an
  unknown deployed path in the deploy classifier (a conservative host
  deploy with no guest refresh); not changed here.
