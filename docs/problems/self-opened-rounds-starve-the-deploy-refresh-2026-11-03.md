# Self-opened rounds starve the deploy refresh

Mutation class: green (this doc). Date: 2026-11-03.

## Evidence

Self-development test computers (class test, ring R1) never took a deploy
refresh while management kept opening rounds on its own:

- deepseek (computer-e80d229b…) stayed on build e4f1c685 for hours. Management
  self-opened Calendar r2 at 21:38Z on that build, missing fixes that had
  already deployed.
- luna self-opened Library r2 at 22:00Z on a pre-fixture build.
- claude ran a stale build until 20:37Z the same way.

vmctl's retry sweep logged each refusal as
`busy: a self-development operation, a live capsule or a streaming snapshot
would not survive a restart; restart deferred until it settles`. The operator
cold-booted the arms to get them onto the current release, which closed their
work as interrupted by a restart.

## Cause

The guest refuses a planned restart while a capsule is live
(`HandleInternalPlannedRestart`, `plannedRestartWork.unsafe`), which is
correct. Between rounds the capsule is gone, but only for seconds: management
judges the candidate and casts the next round in the same activation, and
engineering admission binds the new capsule at once. Nothing in the guest knows
that the host has an update waiting, so admission never leaves the gap open.
The sweep asks once a minute for 30 attempts and then backs off up to an hour
(`deployRefreshRetryDue`), so by the end of a long round its next ask is far
away even when the gap does open.

Related: problem 22 (a frozen or awaiting candidate holds the refresh,
3f32a6ee4) and problem 27 (a new target resets the backoff, e4f1c685a). Both
keep work safe from the refresh; neither makes room for it.

## Belief state

The guest learns of a pending refresh only from the host's planned-restart
request, which it refuses with 409. That refusal is the signal to hold new
engineering admissions: a queued request stays open and unbound, holds no
capsule, and a planned boot keeps and resumes it
(`resumeQueuedEngineeringRequestsAtPlannedBoot`). No restart semantics change.

## Remaining error

The hold must be bounded so that a refresh that never lands (a decision hold,
a run that never ends on a computer without allow_busy) does not stop work, and
the sweep must ask often enough while the guest holds that the drained gap is
seen within a minute.
