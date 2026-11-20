# A train went to main red: an unregistered edge kind and a stale vendor hash

Date: 2026-11-20. Main was red from 11:54Z (`dc584af3b`, CI 44731379068) to
12:26Z (`e33d9df5f`). Nothing was deployed from the red commit: CI failed
before the deploy step, and staging stayed on `f883ea88d`.

## Problem

The lead's pre-landing suites for the third train (entity station E2 with its
follow-up, and two settings removed) passed, and CI then failed on two things
those suites never ran.

## Evidence

- `internal/objectgraph` `TestEveryWrittenKindIsRegistered` failed: the edge
  kind `in_event` is named at `internal/platform/capture_ready.go:247` and was
  not in `DefaultRegistry`. The test scans the whole tree, so a change in
  `internal/platform` can fail a test in a package that neither imports it nor
  is imported by it.
- Every Nix service build failed on a hash mismatch: `flake.nix` pins the hash
  of the vendored Go modules, and the train changed which packages the tree
  imports while `go.mod` and `go.sum` stayed the same.
- The train's suites were the touched packages and their importers, as the
  contract says. Neither failure is in that set. The three review seats that
  read E2 did not run tests.

## Belief

"Every touched package and every package that imports it" misses two kinds of
check in this repository: tests that scan the tree (the kind registry, the
switch registry, the desk protocol lock, which also failed the fourth train
before it left the build machine), and the Nix build's pinned hash. Both are
cheap to run before a landing.

## Repair

- `e33d9df5f` registers `in_event` (the design's kind, owner corpusd) and sets
  the vendor hash; `scripts/cutover/vendor-hash.sh` reproduces the value CI
  computed. One review seat accepted it.
- From the fourth train on, the lead's train script also runs
  `./internal/objectgraph/...`, `./cmd/deskprotocol-lock` and
  `scripts/cutover/vendor-hash.sh` on the rebased tree.

## Remaining error

- `scripts/land` is the place for these checks, so that no lead has to
  remember them. Not done in this session.
- I have not listed the other tree-scanning tests; there may be more than the
  three named here.
