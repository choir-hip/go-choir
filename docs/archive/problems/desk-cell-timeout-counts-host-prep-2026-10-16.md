# A desk cell's own timeout_ms starts before the cell does (2026-10-16)

Status: fixed in 57c4d0b9b, deployed 21:38; outcome below. **A management cell on the paper was killed by its
`timeout_ms=1000` after running 3 ms.**

## Evidence

The paper's console (vm-1669020adcece51c2f5ff9b6df399ef5) logged seven
cells killed by their own `timeout_ms` today (console.log.1 and
console.log, 16:42–20:43). Most report run times below their bound:

| Time (UTC) | Run | Desk | timeout_ms | Reported run time |
|---|---|---|---|---|
| 16:42:54 | f75a90b4 | research | 1000 | 872 ms |
| 18:25:48 | db92dadd | management_task | 1000 | 419 ms |
| 18:27:42 | 15998948 | management_task | 1000 | 958 ms |
| 20:43:53 | 9e5ae184 | management | 1000 | **3 ms** |

The 10 s and 120 s bounds show the same gap, smaller in proportion
(9822 ms and 9919 ms of 10000, 119997 ms of 120000).

The events of run 9e5ae184 show where the time went: `tool.invoked` at
20:43:49.963, the `desk.cell` event's `started_at` (the eval start) at
20:43:53.518, killed 3 ms later with `waited_ms=0`. The model read "cell
stopped by its own timeout_ms=1000 after 3 ms", which matches nothing it
wrote; the cell was 242 bytes.

## Mechanism (code-confirmed)

In `desk_go_eval` (`internal/agentcore/tools_desk.go`), the bound goes on
`evalCtx` with `context.WithTimeout` before the host's frame preparation:
`pendingCellUpdates` and `pendingCellEmits` (store reads) and
`armCellTerminalDeadline` (schedules a continuation). `evalStart` is taken
after them. On a busy paper that preparation took about a second, all of
it charged to the cell's budget and none of it to the reported time.

The host's management deadline (S1) has the same shape, but at 60 s or more
the loss is small.

## Effect

- A small cell with a tight `timeout_ms` is killed before or just after
  it starts, and the session is lost with it.
- The model reads a run time that contradicts its bound, so it cannot
  calibrate: lowering the bound fails sooner, and nothing in the cell
  was slow.

## Fix direction (orange)

Arm the bound immediately before `EvalCell`, after the host's preparation,
so `timeout_ms` measures the cell. A regression test charges the
preparation step a delay longer than the cell's bound and expects the
cell to finish. Rollback: revert the commit.

## Outcome (21:38)

57c4d0b9b arms the bound just before `EvalCell`.
`TestDeskCellTimeoutCountsOnlyTheCell` makes the host's preparation take
700 ms behind a 500 ms bound. Without the move it reproduces the paper
("timeout_ms=500 after 10 ms"); with it the cell runs, and a runaway cell
is still stopped by its bound. CI green (govulncheck, report-only, fails
as on every code commit). Deployed 21:38:13; the paper's planned refresh
booted at 21:37:26 (tail=0, 4 s). Not yet seen live: a tight-bound cell
on the paper after the deploy.
