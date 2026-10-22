# The release apply-check test saw the repository change under CI and failed deploy I's second run

Date: 2026-10-22. Status: **test fixed (`0d35db2b4`, yellow); cause most likely, not proven: newRepo's own background auto maintenance; PatchedTree cleared**. Found by FP0 while
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

## Reproduction attempts (2026-10-22, lane fp0-release-flake)

Diagnostic first: `ed2ca1ed5` makes both PatchedTree fingerprint checks name
the part that moved (index sizes and sha256, added and removed paths under
`.git/objects`, HEAD, status). What the tests require is unchanged.

Local git is 2.54.0 (Nix); Go 1.26.4; 8 CPUs. The global config here sets only
`user.*` and gh credential helpers; no `/etc/gitconfig`. CI runs ubuntu-latest's
own git (version not recorded in the run) with `TMPDIR=/dev/shm`. All runs use
the race-built test binary, `-run PatchedTree`, both tests per iteration:

| Condition | Count | Result |
|---|---|---|
| `GOMAXPROCS=2` | 300 | pass |
| `GOMAXPROCS=2`, 8 × `yes > /dev/null` | 300 | pass |
| same, `TMPDIR=/dev/shm` | 300 | pass |
| same, empty global config (`HOME` at an empty temp dir) | 300 | pass |
| same, global `feature.manyFiles=true` | 300 | pass |
| same, global `core.untrackedCache=true` | 300 | pass |
| same, global `gc.auto=1` | 300 | pass |
| same, global `maintenance.strategy=incremental` | 300 | pass |
| fingerprint polled 10 times right after `newRepo`, CPU hog, `/dev/shm` | 1000 | pass |

Not reproduced in 3,400 iterations. (`HOME` reaches PatchedTree's git as well
as the test's; `GIT_CONFIG_GLOBAL` would not, since `gitOutput` strips `GIT_*`.)

### What strace shows (`strace -f` over one run of both tests)

- PatchedTree's own git processes (`rev-parse --verify`, `rev-parse
  --git-common-dir`, `read-tree`, `apply --cached`, `write-tree`) create,
  rename, link and unlink only under the scratch `choir-release-tree-*`
  directory: `index.lock`, `index`, `objects/xx/tmp_obj_*` and the objects.
  Nothing under the source repository. Hypothesis 2 has no support on 2.54.
- The one other process that writes under the source repository's
  `.git/objects` is the auto maintenance `git commit` starts in `newRepo`:
  `git maintenance run --auto --quiet --detach`. It creates
  `.git/objects/maintenance.lock`, forks a `setsid` child, and (on 2.54) the
  parent unlinks the lock before `git commit` returns. The detached child
  keeps running after that and, at exit, tries to unlink the lock again.
- The git list carries a fix, "builtin/maintenance: fix locking with
  `--detach`" (Patrick Steinhardt, v3 2026-05-13), that hands the lock to the
  detached child (`lock_file_reassign_owner`) so it covers the background
  tasks. Git 2.55.0 has since been released. Whether the fix is in 2.55 is
  **not confirmed here**. With that behavior, `maintenance.lock` stays in
  `.git/objects` after `git commit` returns, until the detached child exits.
  A child slowed by a loaded runner would remove it between the two
  fingerprints: an "objects: removed [maintenance.lock]" failure, rare and
  load-dependent, as observed.

### Hypotheses, ranked after these runs

1. (most likely) The detached auto maintenance from `newRepo`'s commit
   (hypothesis 1, through hypothesis 3's mechanism): its `maintenance.lock`
   in `.git/objects` outlives `git commit` on the runner's git. It is outside
   PatchedTree.
2. Another background step of the same child: on 2.54 it only reads, and in a
   3-object repository no gc, commit-graph or pack task is due.
3. PatchedTree or gitOutput writes the source repository: no write seen in
   strace, and it would fail every run.

## Cause, fix, tests, rollback

**Cause (most likely, not reproduced).** The writer is the test's own setup,
not PatchedTree. The `git commit` in `newRepo` starts a detached `git
maintenance run --auto`, which creates `.git/objects/maintenance.lock`. On a
git where the detached child holds that lock past the commit's return (the
fix on the git list), a loaded runner removes it between the two
fingerprints. Not proven: the runner's git version is not in the CI log, and
2.54 here releases the lock before `git commit` returns. The diagnostic in
`ed2ca1ed5` names the part if it moves again.

**PatchedTree's guarantee (I3/A7) holds as far as the evidence reaches.**
strace shows its git processes write only the scratch directory. No red
change.

**Fix (`0d35db2b4`, yellow, test only).** `newRepo` sets, in the test
repository's own config before its commit: `maintenance.auto=false`,
`gc.auto=0` (no background maintenance or gc), `core.untrackedCache=false`,
`core.fsmonitor=false` (so a runner's global `feature.manyFiles` or fsmonitor
cannot add index writers). That excludes exactly git's background work on the
repository under test. The fingerprint is unchanged and ignores no path: a
`maintenance.lock`, `tmp_obj_*` or `*.lock` that appears still fails the
check, and nothing in the test creates them now. The scratch clone in the
sibling test still runs auto maintenance. It is never fingerprinted.

**Tests.** strace after the fix: no `git maintenance` process starts for
either fingerprinted repository. `go vet ./internal/release`; `go test -race
./internal/release -count=1` pass; 300 iterations of `-run PatchedTree` with
`GOMAXPROCS=2`, `TMPDIR=/dev/shm` and 4 × `yes` pass.

**Evidence that would settle it.** If the check fails again in CI, the
failure text names the part. `objects: removed [maintenance.lock]` (or
added) before this fix confirms the cause. Any failure after this fix means
another writer, and PatchedTree goes back under suspicion first.

**Rollback.** `git revert 0d35db2b4` (the config lines) and, if wanted,
`ed2ca1ed5` (the diagnostic). Both touch only `internal/release/git_test.go`.
