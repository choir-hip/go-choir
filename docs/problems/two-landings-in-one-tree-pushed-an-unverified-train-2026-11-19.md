# Two landing runs in one tree pushed a train whose suites had not run (2026-11-19)

## Problem

At 10:18Z the vector train (33 commits: the Go half of the vector index and
the ranked corpus search) was pushed to `origin/main` by a landing run that
was not landing it. Only that run's build and gates had passed on it. The
test suites `scripts/land` runs on the build VM (every touched package and
every package that imports one, `internal/agentcore` under the race detector
among them) had not run on that stack, and the push was not held, though the
lead had started the train's own run with its push held on a file.

## Evidence

- 10:01Z: the lead started `scripts/land origin/lane/vec-host` in the one
  landing tree, then decided to hold its push, stopped it with `kill` on the
  process ids `pgrep -f` returned, saw a count of zero, and started a second
  run of the same lane with `LAND_HOOK_AFTER_VERIFY` waiting on a hold file.
  The first run was not dead: it pushed the host half at 10:06Z
  (`edf47882f`, `5e2dd4b42`, `40a5bbdff`). The second run kept waiting.
- 10:13Z: the lead removed the hold file, believing no run was left. The
  second run woke, fetched, found main moved, and went on to rebase and
  re-verify "the train", which for it means the tree's `HEAD`.
- 10:15Z: the lead started `scripts/land origin/lane/vec-train` in the same
  tree. It moved `HEAD` to main plus the 33 commits of the vector train.
- 10:18Z: the older run finished build and gates on that `HEAD` and pushed
  it. Main went `40a5bbdff` to `b6298ae88`. Its log says
  "the moved commits touch nothing under test; build + gates only".
- The train's own run was stopped by the lead a minute later for another
  reason (to add the import); nothing else was pushed by it.

## Belief

`scripts/land` assumes it is alone in its tree and identifies its train by
`HEAD`. Nothing refuses a second run in the same tree, and a run held by the
hook can wake long after its operator has forgotten it. The lead's check
that a process was gone was a process-list search, which missed it twice.

## What was done, and the remaining error

- The content is reviewed (the index: five rounds and an accept of the
  split; the ranked search: two rounds, accept), and each builder ran the
  touched packages' suites on its own lane. What was skipped is the
  landing's whole-suite run on the stacked, rebased tree.
- CI runs the whole race suite before any deploy; the push is not deployed
  unless CI passes. The lead also ran the directly touched packages' suites
  on the build VM after the fact (results in the landing's report).
- Not reverted: a revert would be a second unverified push of the same
  size, and CI is still in front of the deploy.
- Owed: `scripts/land` takes a lock in its tree and refuses a second run
  while one lives; the hook's hold says in the log that it is holding. Until
  then the lead starts a landing only after reading the process list by
  hand and never reuses the tree while a held run exists.
- Remaining error: the claim for this landing (C151) is settled on CI and
  the deploy as written; it does not measure what the skipped run would
  have caught that CI does not (CI's PostgreSQL has pgvector only since
  this change, never run before).
