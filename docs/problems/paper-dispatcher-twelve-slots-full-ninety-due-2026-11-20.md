# The paper's dispatcher: twelve slots full, about ninety agents due (2026-11-20)

Status: open. Read-only investigation, 2026-11-20 06:20Z to 06:28Z. Nothing in
production was changed.

## Problem

On the paper computer (guest of host `node-b`) every `dispatcher: activation
wait` line since the console begins (04:56Z) shows `slots=12/12` and
`due_agents` between 57 and 99. Waits of ten minutes or more are common
(62 of 809 logged waits are 600 s or longer; the longest is 2,302 s). Yesterday
afternoon the same lines showed `slots=2/8` and `due_agents=2` (owner's
observation, not re-read: the console does not reach back that far).

## Evidence

Sources: the guest console, five files (`console.log` to `console.log.4`,
15,043 dated lines, 04:56:37Z to 06:22:20Z); the repo at `7f5ad41e1`; the public
front-page API; the paper's API with the paper key (reads only). Counts only.
Console rotation keeps about 85 minutes, so the **onset is before 04:56Z and
unknown**.

### 1. Ten-minute buckets (UTC, from `activation wait` lines)

| Bucket | Console lines | Waits logged | Max due_agents | cause=noroom | cause=busy | slots seen |
| --- | --- | --- | --- | --- | --- | --- |
| 04:50 (from 04:56) | 605 | 38 | 63 | 12 | 26 | 12/12 |
| 05:00 | 1,394 | 106 | 79 | 82 | 24 | 12/12, 10/12 |
| 05:10 | 1,369 | 137 | 62 | 67 | 70 | 12/12 |
| 05:20 | 2,189 | 119 | 70 | 51 | 68 | 12/12 |
| 05:30 | 3,732 | 129 | 61 | 57 | 72 | 12/12 |
| 05:40 | 1,520 | 113 | 87 | 44 | 69 | 12/12 |
| 05:50 | 1,315 | 118 | 87 | 50 | 68 | 12/12 |
| 06:00 | 1,343 | 95 | 82 | 35 | 60 | 12/12 |
| 06:10 | 1,015 | 61 | 99 | 18 | 37 | 12/12 |
| 06:20 (to 06:22) | 561 | 32 | 98 | 32 | 0 | 12/12 |

Trend: not draining, drifting up (57 to 63 at 04:56, 82 to 99 after 05:40).
Restarts do not cause or clear it: the due set is durable. After the planned
restart at 04:59:46Z the first wait lines read `due_agents=71`; after the boot
at 06:17:59Z, `due_agents=97` with research agents waiting 17 to 38 minutes for
their first `initial_dispatch`. A wait is measured from the update's due time,
so those waits began before the restart. The backlog is neither a restart
artefact nor a traffic spike seen in these 85 minutes; the console cannot say
when it began.

### 2. Who holds a slot and for how long

The wait line's `holder_for` is the age of the same agent's own in-flight
activation (cause=busy, the serial fence). 432 busy lines, 185 distinct
holders:

| Holder kind | Distinct holders |
| --- | --- |
| texture | 123 |
| research | 56 |
| source | 5 |
| management | 1 |

Hold time per holder (largest seen): median 443 s, p90 3,000 s, maximum 4,195 s;
30 held 30 minutes or more, 15 held an hour or more. The `noroom` waiters are
texture 293, research 75, source 2, management 2.

Code reading (confirmed): `Dispatcher.activate` takes one slot for the whole
activation and runs **every** pending update of the agent in order inside it
(`internal/actor/dispatcher.go:513`). `initial_dispatch` and `coagent_result`
run the whole model turn inside the handler
(`ExecuteActivationSyncChecked`, `internal/actorruntime/handler.go:711`). So a
slot is held across every model call, tool call and search of the turn, and then
across the rest of the agent's queue. A texture agent that holds a slot for a
turn drains its queued deadline updates afterwards in the same slot (queues of
up to 126 updates; texture p50 6, p90 21).

Model activity during the window: 2,547 `model call` lines in 85 minutes (about
30 per minute across twelve slots); 5,228 research and 896 texture `corpus
read` lines. Which holder is in a model call at a given moment is not in the
console (it logs a call's start, not its end, and no activation duration).

### 3. Who is due and what woke them

Of 809 logged waits, 643 are texture, 152 research, 9 source, 5 management.
Queued updates counted in those waits: `cell_terminal_deadline` 5,331,
`activation_budget_deadline` 1,042, `inbox_record` 476, `initial_dispatch` 282,
`source_poll_due` 9, `fresh_mint_management_resume_deadline` 2. Waits whose
queued updates are all deadlines: 247 of 809 (31%); among `noroom` waits, 190 of
372 (51%). `HandleCellTerminalDeadline` (`internal/agentcore/continuation_schedule.go:143`)
is cheap for work already done: it reads the run and the cell's fate and
returns when the run is terminal or the fate exists. But each such wake still
needs a slot, a snapshot load and a log commit, and it arrives in bulk
(every research cell arms one at its activation's deadline and nothing disarms
it when the cell finishes).

### 4. Cost to the product

- Publishing continues at the usual rate. Front-page stories by hour of last
  revision (one count per story, so a story re-revised is counted at its latest
  hour): 2026-11-20 04:00Z 28, 05:00Z 25; the day before the same hours were 17
  and 22, with an all-day range of 3 to 29 per hour. 2026-11-19 total 318,
  2026-11-20 to 06:22Z 127. The 06:00 hour shows 8 by 06:22 (the host restarted
  the guest at 06:18).
- Console: 51 `story publish ... published` lines against 43 `failed and stays a
  gap` and 76 `not published: it cites N outl...` lines in 85 minutes. I have no
  prior day's rate for these.
- Latency, not loss: waits p50 250 s, p90 516 s; 62 of 809 at 600 s or more. A
  research agent waited 38 minutes for its first dispatch at 06:19Z.
- Not measured: web searches per hour compared with yesterday (46 `runtime:
  search` lines in 85 minutes; no baseline), and timeouts per hour (the console
  logs deadlines queued, not fired).
- The paper's `GET /api/runs?limit=5` timed out at 90 s twice (06:25Z, 06:26Z).
  The store is slow from the API side too.

### 5. The store is under pressure at the same time

`store_stalled holders` appeared 31 times in 85 minutes. The write pool has one
connection (`max_open=1`); before the 06:18 boot it reported 46,330 waits for
1,613 s in total, since the 04:59 boot, and the read pool (four connections) 128,508 waits for
2,085 s, with all four in use in about half the reports. Holders are
`ListObjects`, `ReadObjectSnapshotFiltered`, `ListObjectsPage`,
`ListObjectsByKindOwnerWhere`, `GetObjectByMetadata` and `RedriveFamilyStatus`.
After the 06:18 boot: 1,072 write waits for 70 s in 150 s. Many `intake labels:
flush ... context deadline exceeded` and `texture api: ... context canceled` lines.

A background derived-key pass ran 04:56 to 04:57 (the previous boot's) and
05:31 to 06:17 (`store: derived keys: the periodic full pass starts`) at 11 to
14 rows per second over 455,771 `actor_wake_outbox.projected` rows
(an estimated 9 to 10 hours at that rate). It is **not the cause**: from 05:00
to 05:30 no pass ran and `due_agents` was 62 to 79, and from 06:18 none ran and
it was 98. It may add to the load (due 61 in the 05:30 bucket, 87 by 05:40).

## Belief

1. Confirmed by the console and the code: the twelve slots are full and about
   ninety agents wait; the backlog survived two restarts; the waiters are mostly
   texture agents; a slot is held for a whole model turn plus the agent's whole
   queue.
2. Hypothesis (not confirmed): the backlog is the product of arrival rate
   exceeding what twelve slots clear when each activation includes seconds to
   minutes of store waits on a one-writer store, plus a bulk of deadline wakes
   that need a slot each though their work is finished. What would confirm it:
   a per-activation duration and a per-activation store wait in the activation
   log (the hold time distribution above is only the busy fence's view);
   a count of `cell_terminal_deadline` wakes consumed as no-ops against ones
   that recorded a timeout; the arrival rate (updates created per hour) beside
   the completion rate.
3. Not established: when it began, and what changed from "2/8, due 2"
   yesterday afternoon. The console reaches back 85 minutes. The paper's run
   list timed out, so the run table could not be read for start and finish
   times by hour.
4. Against "twelve is too few": twelve slots are not the evidence. A longer
   queue of no-op deadline wakes (51% of noroom waits) is not relieved in the
   right place by more slots, and more slots increase load on the one-writer
   store.

## Remaining error

- The onset time and the trend before 04:56Z.
- Whether holders are in model calls or in store waits (neither duration is
  logged).
- The wait lines are written when an activation finally starts, so agents that
  have not started are not in the table; the true waiting set is `due_agents`.
- Product cost: only stories per hour and wait times; no search or timeout
  baseline.

## Smallest fix that is not a knob (proposed, not made)

A cell's `cell_terminal_deadline` and an `activation_budget_deadline` for a run
that has ended should not be queued at all. Two structural options, in order:

1. **Disarm on completion.** When a cell or run reaches a terminal fate, mark
   its deadline wake processed in the same commit (or delete its not-before
   row), so a finished cell leaves no due wake. This removes the bulk
   (5,331 + 1,042 of the 7,200 queued updates seen) without a setting.
2. **Separate cheap wakes from turns.** A deadline wake whose handler needs only
   store reads should run outside the shared slot table (it is bookkeeping,
   not a model turn), so it never queues behind an expensive turn.

Do not raise the slot count, and do not add a limit. Before either change,
record per-activation duration and store wait in the activation log (a
green/yellow change), then read two hours of it.

## Next step

1. Log activation duration and the store wait it saw, per agent kind, with the
   handler kind. 2. Read the paper's run table (start and finish by hour) when
   `/api/runs` answers, or from the host's database read-only, to date the onset.
3. Count `cell_terminal_deadline` no-ops against fates. 4. If the no-op share
   holds near half, land fix 1.
