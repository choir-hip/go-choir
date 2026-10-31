# Lean system: open lanes

The register of lane branches this mission has opened that are not on main.
Owner, 2026-10-31: "we should be careful to not accumulate lane branches
that don't land. We should keep track of them and understand when we can
land them." A lane leaves this table when it lands on main (name the commit)
or is closed (name why). Update it every time a lane opens, lands or closes.

| lane | what it does | state (2026-10-31) | lands when | blocker |
|---|---|---|---|---|
| lane/backup-memory | weekly restic prune unit, prune-keygen | built, 1 commit | next deploy | none; missed the 2b2bc2c7d merge |
| lane/snapshot-sweep | sweeps VM memory snapshots that can never resume (8 GiB each) | built, tests pass | next deploy | none |
| lane/tape-seg-compact | rewrites sealed tape segments once; ~21 GB on Node B | built, tests pass | next deploy, then a d@e dry run and apply with --compsize | reclaim unmeasured on btrfs |
| lane/chaos-harness | d@e-only crash drills, scenarios 1-3 | built, selftest 111 pass | with or after tape-group-commit | needs group commit on d@e to be useful |
| lane/tape-group-commit | groups acknowledged tape writes; telemetry rides the next write | 3 commits, tier-2 review running | after review accepts | guest security epoch 23 conflicts with tool-layers |
| lane/tool-layers | erofs layer per tool (office, browser, toolchain, delegate) | 2 commits, unbooted | its own deploy, after group commit | epoch 23 conflict; needs a boot proof |
| lane/mac-arm-guest | aarch64 guest image in Nix | 2 commits (DuckDB test input dropped on aarch64 only); image build running | after a vfkit boot proof | moves the x86 floor digest: one cold boot of hibernated snapshots; ship with snapshot-sweep |
| lane/bundle-authenticity | receipt signature checks; refuse a snapshot root the tape does not name | WIP | when its builder reports | in progress |
| lane/store-churn | paper SQLite churn (clustered keys) | 1 commit, benchmark running | after benchmark and a d@e proof | in progress |
| lane/mac-follow | Mac app follows a computer through bundles (sync slice 1) | builder running | after host API deploy and the d@e follow E2E | in progress |

## Older lanes, after the 2026-10-31 cleanup

On October 31st, 356 remote lane branches whose work was already on main, or
was superseded, were deleted. Their names and tip commits are in
docs/evidence/deleted-lane-branches-2026-10-31.tsv. To restore one:
`git push origin <sha>:refs/heads/<branch>`.

Seven older lanes remain, each waiting on a decision:

| lane | what it holds | lands when |
|---|---|---|
| lane/fp3-cuts-on-main | ownerActMode observe to enforce in nix/node-b.nix (SEC-12: corpusd refuses an owner-mode write without the proxy's signed act). Red, auth | owner decision; then a deploy and a check of corpusd refusals |
| lane/host-b4-b5, lane/host-b4-b5-on-main | the same enforce commit, plus host containment and proxy commits that triage matched to main by subject | delete with fp3-cuts-on-main once that one lands or closes |
| lane/cite-promote | not a fix: a verbatim hand-promotion (2026-10-27) of b@c's self-development patch selfdev-5de7f406…, the citation toggle, which the owner reports did not work | parked (owner, 2026-10-31): the owner sees it running before any promotion; it waits until the data system, sync and the Mac app work, then returns with the PSA, self-development and Autopaper pivot |
| lane/fp0-jobpark-wired | a test only (capsule_job_park_wired_test.go); the fix landed | land the test if the wired path is otherwise uncovered, else delete |
| lane/fp0-apprewrite, lane/fp4-r4 | problem records not on main | land the records as docs |
| lane/fp0-obligations | a design doc; merges cleanly | land as docs, or delete if superseded |
