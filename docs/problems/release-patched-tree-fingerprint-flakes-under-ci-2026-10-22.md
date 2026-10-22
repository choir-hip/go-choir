# The release apply-check test saw the repository change under CI and failed deploy I's second run

Date: 2026-10-22. Status: **documented; cause not yet found**. Found by FP0 while
landing deploy I (freeze round 5). Evidence class: CI (GitHub Actions), not
staging. Mutation class of this record: green.

## Evidence

- CI run 39745680806 on main `54040b730`, a docs head carrying deploy I's code
  (`c042989a1`, `f68100cf6`, `4c501bae6`). Job "Go Test (race, non-runtime
  shard 7)" (119713490372) failed at 2026-10-22T08:11 UTC. "Go Vet + Test +
  Build" failed with it.
- The only failing test in the run:

      --- FAIL: TestPatchedTreeRefusesAPatchThatDoesNotApply (0.05s)
          git_test.go:120: a refused apply check wrote to the repository

- Deploy I's code touches `internal/agentcore` only
  (`engineering_assignment_fate.go` and its test). `internal/release` last
  changed on 2026-10-15 (`b244507f4`, the fingerprint's
  `--no-optional-locks`). The same test passed on every main run between then
  and now, including deploy H's run 39739760307.
- Deploy I's first run (on `4c501bae6`) failed on a different flaky test,
  TestAppendFuncBuildsOnlyBetweenOtherCalls
  (tapestore-appendfunc-test-races-its-own-done-flag-2026-10-22.md, fixed in
  `58e705802`). Two unrelated test flakes in two consecutive runs.

## What the test checks

`internal/release/git_test.go:102`. `repoFingerprint` (line 48) joins the
bytes of `.git/index`, the list of every file under `.git/objects`, `HEAD`,
and `git --no-optional-locks status --porcelain`. The test takes it before and
after `PatchedTree` refuses a conflicting patch and requires them equal.
`PatchedTree` (`internal/release/git.go:49`) runs `rev-parse --verify`,
`rev-parse --git-common-dir`, then `read-tree` and `apply --cached` with
`GIT_INDEX_FILE` and `GIT_OBJECT_DIRECTORY` in a scratch directory and the
repository's objects as an alternate.

## Hypotheses (not confirmed; nothing logs which part changed)

1. Something outside `PatchedTree` writes `.git/index` or `.git/objects`
   between the two fingerprints: a background step left by `git commit` in
   `newRepo` (auto maintenance, a commit-graph or an index refresh), or a
   runner-wide git config (`feature.manyFiles`, untracked cache, fsmonitor).
2. `gitOutput` strips the caller's `GIT_*` variables and adds the scratch
   ones; a git version on the runner that ignores `GIT_INDEX_FILE` for
   `read-tree` or `apply --cached` in some path would write the real index.
   It would then fail every run, not one, so this is less likely.
3. The fingerprint itself is unstable: the object walk lists paths, and a
   temporary file git creates and removes under `.git/objects` while the walk
   runs would show in one fingerprint and not the other.

The test does not say which of the four parts differed, so the record cannot
tell these apart.

## Effect

A CI failure blocks the deploy that carries it. The release record's guarantee
(an apply check never writes the source repository, failure modes I3/A7) is
either broken under some condition or the test reports a false break; both
matter.

## Next

1. The test reports which part of the fingerprint differs (index bytes, object
   list with the added and removed paths, HEAD, status) when it fails.
2. Reproduce under load (`go test -race -count=200 ./internal/release -run
   PatchedTree` with `GOMAXPROCS=2`, and with a busy CPU) to find the writer.
3. Fix the writer if it is `PatchedTree`; fix the fingerprint if it is the
   test. Do not weaken the check to pass.
