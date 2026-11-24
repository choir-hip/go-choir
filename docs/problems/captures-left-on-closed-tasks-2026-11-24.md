# Captures left on a task that closed without deciding them (2026-11-24)

## Problem

A management task can close with captures still on it. The host then hands
them to nobody: it writes them into the task's final Report as "residuals",
and whether they are ever decided depends on the management model choosing,
in some later turn, to delegate that Report again. Nothing obliges it to,
nothing counts what was left, and nothing shows a capture that fell out this
way. It is the second way the paper loses captures; the first (queue expiry
under a busy dispatcher) is in
[captures-dropped-and-stalled-2026-11-24.md](captures-dropped-and-stalled-2026-11-24.md)
and is fixed on today's code by `lane/intake-starve`, which does not touch
this one.

## Evidence

**Measured** (the copy of the paper's database of 2026-11-24, the same record,
seven days to 2026-11-24T00:40Z): 476 captures sit on a task that closed
without deciding them, about 2% of the 25,608 that arrived.

| How the task closed | Captures left |
| --- | --- |
| its run failed | 198 |
| its run was passivated (a planned restart; a task is not resumed) | 164 |
| it finished (`done`) and left them | 89 |
| its run completed without closing the task | 19 |
| its deadline (10 minutes from its start) | 6 |

**From the code on `origin/main`** (read 2026-11-24):

- A task is started by the host from an arrival record
  (`internal/agentcore/arrival_task.go`), with no model in the loop.
- When it closes, the store works out each leased capture's state. One with
  no disposition becomes a `TaskResidual` on the closing Report, with the
  closure reason as its only explanation
  (`internal/store/management_tasks.go`, the closing plan, about lines 510
  to 540). No row is written for the capture itself: its disposition index
  entry still names the closed task as its lease.
- A Report that is not clean wakes the parent, management
  (`RecordWakesReader`, same file, about line 147). The exception is a
  closure by a crash restart, which wakes nobody.
- The only path back into the queue is `planDelegation`
  (`internal/agentcore/management_tasks.go`, about lines 623 to 700): when
  management's cell calls `choir.Delegate` naming that Report, its residuals
  that are still delegable are leased to a new task. `captureDelegable` (same
  file) allows it once the old task is closed.
- So the arrival path is host-driven and the recovery path is model-driven.
  There is no host step that re-queues a residual, no deadline on
  management's answer, and no count of residuals outstanding. The queue line
  `lane/intake-starve` adds counts queued tasks, not these.

**Not measured:** for each non-clean Report, whether management was woken,
whether its turn ran, and whether it delegated, declined or ignored the
residuals. That needs the Reports and management's following turns read from
the ledger (the copy is damaged in places; the rows are readable by rowid
range). Without it the split between "management never saw them" and
"management saw them and did nothing" is unknown.

## Belief

- The loss is a property of the design, not of one bug: work returned to a
  model as a suggestion is not an obligation. A capture can leave the queue
  with no decision row saying what happened to it.
- `lane/intake-starve` should shrink two rows a little, as an inference from
  the code and not a measurement: a task's 10-minute deadline runs from its
  claim, and on a busy day two to six of those minutes were spent waiting for
  the dispatcher. With its own lane a task gets its whole deadline, so fewer
  deadline closures (6) and perhaps fewer failed runs (198). It does nothing
  for passivated runs (164) or tasks that finish and leave captures (89).
- 164 of the 476 come from planned restarts: every deploy that refreshes the
  paper while tasks run drops their captures into this path. With the deploy
  rate of this month that is the most avoidable row.

## What the design asks for

[desks-and-engine-2026-11-24.md](../design/desks-and-engine-2026-11-24.md),
section 2: arrivals are held as data by the capture desk, each group goes to
a sub-call, the custodian service writes what comes back, and no desk
session is spent on a single capture. Read against this problem, that design
removes the task as the unit that can close with work on it, but only if it
keeps one rule the present path lacks:

1. **A capture leaves the queue only by a decision row** written by the
   custodian: taken into a group and compressed, declined with a reason, or
   failed with the failure named. "Its sub-call ended" is not a decision.
2. **A sub-call that fails, times out or is cut by a restart returns its
   captures to the queue by host code**, with a count of attempts, not by a
   message to a model. After a bounded number of attempts the capture gets a
   decision row that says so, which is visible and countable.
3. **A planned restart is not a loss path**: captures in flight at a restart
   are still in the queue afterwards (CLAUDE.md, Restarts: a crash restart
   never resumes work; a capture that was never decided is not work in
   progress to resume, it is queue state).
4. **One number says it holds**: captures arrived, minus captures with a
   decision row, minus captures queued, is zero. Today that difference is
   the 476.

## A smaller fix on today's code, not built

If the capture desk is not close, the same rule can be put on the present
path: when a task closes for any reason other than `done` with every capture
decided, the host re-queues its undecided captures itself (a new task from
the same delegated records, as `planDelegation` already builds for a
residual Report), with an attempt count on the capture's index entry and a
closing fate after a bound, and logs the count. This is host code doing what
the Report asks a model to do. It needs its own failure list and review; it
changes what a task closure does, so it is at least orange.

## Remaining error

- The 476 are from a damaged copy; the split by cause is approximate near
  the gaps the first record names.
- What management did with the Reports is not measured (above).
- Whether the 89 "finished and left them" are a model ending early or a
  refusal the task could not express is not known; the reason on the
  residual is the task's closing note and was not read out.
- Whether any of the 476 mattered to a story the paper then missed is not
  measured.
