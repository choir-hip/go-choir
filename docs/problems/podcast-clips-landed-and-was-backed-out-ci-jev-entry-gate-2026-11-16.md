# Podcast clips landed, CI refused it, and it was backed out

2026-11-16, 05:10Z.

## Problem

The podcast clips series (18 commits, tip 93004c45f) passed four review
rounds and `scripts/land` (every suite green) and was pushed to main at
04:30Z. CI run 43804540972 failed. Nothing deployed; staging stayed on
a99e07d3b. Main was red for about 35 minutes until the revert af31e4fd0.

## Evidence

CI job `Vocabulary Gates`, step "Check the Jev entry point":
`check-jev-entry: internal/platform/podcast_match.go calls Jev directly and
is not on the list.` The gate (`scripts/check-jev-entry`, owner's rule of
2026-11-15) allows a Jev call only from the files on
`scripts/jev-entry-baseline.txt`, and with `--base` that list may only
shrink. The match job in corpusd builds and sends its own judgment request
through a gateway host caller.

## Two defects

1. **The match job is a new direct Jev site.** It makes one call a run, but
   it packs its own request with its own bounds rather than through the one
   job entry point (`jevJob`, in agentcore). That is the shape the owner's
   rule forbids. Four rounds of review did not see it: no review prompt named
   the Jev entry rule, and the lead did not read section 10 of the Jev design
   before ordering a Jev caller (the contract's "read the reference page
   before touching a new dependency").
2. **`scripts/land` does not run a gate CI runs.** `check-jev-entry` is in
   `.github/workflows/ci.yml` only, not in `scripts/gates`, so a landing can
   pass locally and fail on main.

## Belief

corpusd cannot import agentcore, so a conforming match job needs the job
entry point in a shared package and the gate taught to admit a new ENTRY
file. That changes the owner's gate and is the owner's decision.

## What was done

- Backed out as af31e4fd0 (one seat verified the tree equals a99e07d3b
  outside `docs/`).
- The series is being re-cut without the automatic match job: kinds, reads,
  operator writes, the worker, the programme's clip segment, and an operator
  tool that lists ranked candidates (no model) and writes judged clips
  through the existing verified route. Matching tonight is an operator act.
- `check-jev-entry` is being added to `scripts/gates`.

## Remaining error

Other CI-only gates may exist that the landing script does not run; the
workflow has not been compared with `scripts/gates` step by step. The
automatic match job needs the owner's decision on the entry point.
