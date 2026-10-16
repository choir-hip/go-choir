# A failed write of a run's terminal state leaves it "running" for an hour, and management absorbs every wake meanwhile (2026-10-16)

Status: documented. Fix to follow.

Found at the 12:29 UTC check: three paper stories opened 12:07–12:23
had no Texture run yet, and a fourth got its first run 16 minutes late.
The cause was an hour earlier.

## Evidence (the paper, run list and console)

Runs per half hour (Texture / management / research):

| | Texture | Management | Research |
|---|---|---|---|
| 10:00 | 63 | 30 | 19 |
| 10:30 | 68 | 29 | 16 |
| **11:00** | **6** | **1** | **0** |
| **11:30** | **1** | **0** | **0** |
| 12:00 | 107 (catching up) | 50 | 23 |

Console lines per 10 minutes fell from about 170 to 2–8, from 11:04 to
11:50.

The one management run of the hour, `2748a8cc` (persistent management
`management:e5962310…`):
- 11:02:39 started, read its records, and got its second model reply;
- 11:02:58 `runtime: update run 2748a8cc to failed: store: append
  projection batch: computer event appender: pin payload: … corpusd
  returned 400: … no space left on device`. Node B's btrfs metadata was
  exhausted (`node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md`);
- 12:02:42 `loop.cancelled: activation budget exceeded: progress deadline
  reached`. That is the progress-deadline sweep, one hour later.

The same 11:03 window failed the state writes of Texture runs
`d3986d36`, `b481f7ce`, `f56b57be`, `8d02b951` and `57c052a3`. The
dispatcher poisoned 6 management wakes at 11:03:15–11:03:36 after their
attempts failed. Storage recovered by 11:04:02 (the last ENOSPC on the
paper). The paper stayed idle until about 11:51–12:02, when the sweep
cancelled the stale runs.

## Mechanism (code)

1. **The terminal write is attempted once.** When a run fails,
   `internal/agentcore/runtime.go:4431` calls
   `persistActivationStateAndEmit`. On error it logs `runtime: update run
   … to failed` and returns. The loop is gone, but the stored run still
   says `running`. Nothing retries the write.
2. **A stored `running` run absorbs the desk's wakes.**
   `WakeRecordsInbox` (`internal/agentcore/inbox_records.go:897`) returns without
   starting a run when `activeRunByAgent` finds one. That check reads only
   the store (`latestActiveRunByAgent`), not whether this process has a
   live loop for the run. Each new record is left for "the running run's
   next cell", which never comes. The wake is consumed, so it does not
   come back.
3. **Only the progress deadline clears it**, after one hour
   (`continuation_schedule.go:315`).

Management is one persistent actor per owner. While it is blocked, no
story is opened or fed, so Texture and research go quiet too. One
minute of failed storage writes became about 50 minutes without a paper.

## Classification

This is substrate: run-state durability. The stored state of a run can
claim a live loop that is gone, and the wake path trusts it. ENOSPC was
this time's trigger. Any transient store or corpusd failure during a
terminal write does the same: a Dolt restart, a corpusd deploy, or a
network blip to the host.

## Fix direction

- **The terminal write must land.** When persisting a run's terminal
  state fails, keep retrying with backoff in the background (seconds up
  to about a minute apart, bounded at about 15 minutes). Stop once the
  write succeeds, or once the stored run is already terminal (the sweep
  or another path won). The loop has ended, so the retry holds no lock
  and blocks nothing.
- **Optionally, a stale run must not absorb a wake.** If the stored
  active run has no live loop in this process and is not parked, the
  wake should terminalize it as interrupted and start a fresh run.
  Parked runs are `running` without a goroutine by design, so this needs
  care. It is the second step, after the retry.

Ways the fix could fail (tests first):
- the retry loops forever on a permanent error (a run already terminal,
  or a transition the store refuses);
- the retry races the progress sweep and writes `failed` over
  `cancelled`, or the reverse;
- a retry that eventually succeeds does not run the terminal follow-ups
  (`bindTerminalRunOutcome`, the Texture mutation settlement), because
  the original call returned early on `!persisted`.

## Not fixed here

- The 6 poisoned management wakes are lost. The records they announced
  were read by the next management run after 12:02.
- The 11:03 Texture runs whose state writes failed ended without their
  turns' follow-ups. Their documents were woken again by later records.

## Step 2 (2026-10-16 23:35)

Step 1 (end writes retried, c8e1dc259) leaves a gap. A run whose loop
died without any end write, or whose retries ran out during a longer
outage, still absorbs a records desk's wakes until the progress
deadline.

Step 2: `WakeRecordsInbox` ends a stored active run when all three hold:
- it is stored `running`;
- this process has no loop for it (`rt.running`, which `executeActivation`
  clears whenever a loop ends, even when the end write fails);
- its state has not changed for longer than 2 minutes, so a run whose loop
  is about to register is never ended.

The wake then starts a fresh run. Pending runs are left alone, because
their dispatch may be in flight and the fresh-mint watchdog owns a lost
one. On "parked runs are running without a goroutine": parking stores a
run as `passivated`, so a parked run is never `running`.
`TestRecordsWakeReplacesARunWithNoLiveLoop` was written first; it
reproduced the absorbed wake and covers the live, just-started and
pending cases.
