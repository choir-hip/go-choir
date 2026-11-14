# The CI classifier self-test fails on main, and CI only runs it when .github changes

2026-11-14. Recorded 09:57Z.

## Problem

`.github/scripts/ci-impact-classify-test` fails on main's tree. CI runs it
only on a push that touches `.github`, so main has been red on it without
any run showing it. The next change under `.github` fails CI for a reason
that is not its own.

## Evidence

- On main at 79dd71b57, in a clean `git archive` snapshot on the runner VM
  and in two workstation checkouts: "script scripts/chaos/s2-corpusd-crash.sh
  is named by a deployed, CI, Go or kept file but classifies as docs-like;
  add it to ci_deploy_script_path in ci-docs-like-paths". A builder found
  the same for `scripts/off-host-restore-drill.sh` and
  `scripts/node-b-growth-meter`.
- CI run 43576567543 on 79dd71b57 is green: the job that runs the self-test
  was not selected, because the push changed nothing under `.github`.
- I first read the local failure as a dirty-checkout artifact because CI
  was green. That was wrong; the clean snapshot settles it.

## Belief

Developer-side `scripts/` became docs-like on 2026-11-12. Three scripts
that Go tests or nix name were not listed as exceptions, and the self-test
that would have said so did not run on the commit that made the change or
on any since.

## Remaining error

Not repaired on main. Two unlanded lanes carry the same repair (the three
paths classified, with assertions): `lane/prune-at-boot` (held for the
owner) and `lane/receipts-step2ab` (in review). Whichever lands first
repairs it. Separately, the self-test should run on every push that
changes `scripts/` or any file it scans, not only `.github`; that is not
in either lane.
