# The paper's dispatcher slots are saturated: activations wait minutes for a slot (2026-10-23)

Found 2026-10-23 09:40 UTC by FP0 reading the paper's console. Recorded by a
read-only investigation worker (lane/fp0-paper-slots, mutation class green:
this is a record, no code and no config changed). Per the contract, this is
the problem record; it names options and does not choose one.

Evidence class: the paper's console logs (`console.log.4` through current,
03:24 to 09:39 UTC), the paper's `/api/runtime/observability` (24 samples,
09:48 to 09:51), `GET /api/runs/{id}/events` for 52 sampled runs, the gateway
journal on Node B (03:00 to 09:50, the paper's lines only), the gateway
`/health` via `scripts/proofs/fp7-gateway.sh --step 10`, and the Synthetic
watch log. Code read at origin/main 7319914be. Tags: [V] verified from a
trace or the code, [H] hypothesis.

Subject: computer-1355b83bced1613eb048ee28d66688a8 (vm-1669020adcece51c2f5ff9b6df399ef5),
pool `synthetic-luna` (threads split 1:1 between Synthetic DeepSeek V4.1 Flash
1017 and gpt-6-luna, no failover), dispatcher MaxConcurrent 8, Synthetic gate
`SYNTHETIC_MAX_CONCURRENT=3` since 05:16 UTC.

## One-paragraph finding

[V] The 8 shared dispatcher slots are full of runs, and a run holds its slot
for its whole life. From 05:16 to 09:36 UTC the gateway had on average 6.0 of
the 8 slot holders inside a model call: 2.65 inside a Synthetic call, 2.9
queued at the Synthetic gate (3 in flight), 0.45 inside a luna call. The other
25 percent of slot time is not in a gateway model call (tools, search, store;
not separated). [V] Synthetic is slow per call (service about 9 s at gate 3,
against about 3 s for luna), and a Synthetic-thread cell spends about 113
slot-seconds in model calls against about 10 for a luna-thread cell, so the
Synthetic half of the threads takes most of the slot time and the gate queue
parks about a third of the slots doing nothing. [V] A queued agent therefore
waits for a slot a median of 5 minutes, and a Texture cell needs two or three
passes through that queue to do 25 seconds of work.

## Q1. The activation wait line

[V] Emitted in `internal/actor/dispatcher.go`, `logActivationWait` (line
670), called from `activate` (line 464) after the activation already holds
its slot and has read its backlog. Threshold: `activationWaitLogAfter = 30 *
time.Second` (line 128). The text is
`dispatcher: activation wait agent=<id> waited=<d> updates=<n> slots=<a>/<b> due_agents=<c>`.

What the fields are, from the code:

- `waited` [V]: now minus the oldest due time among the agent's unprocessed
  updates, where due is `max(CreatedAt, NotBefore)`.
- `updates` [V]: how many unprocessed updates the agent's backlog held when
  the activation started (1 on 45 percent of lines here; the rest 2 or more, up to 64).
- `slots=a/b` [V]: `runningInLocked(lane)`, the activations in flight in this
  agent's lane, and the lane's cap. `running[agentID]` is set before
  `activate` runs, so `a` counts the activation that is logging. `b` is
  `MaxConcurrent` (8) for the shared slots, or the lane cap (4 for the source
  lane). 2,637 of 2,716 lines (03:24 to 09:39) read 8/8 and 63 read 4/4.
- `due_agents` [V]: `len(PendingAgents)` at the last dispatch pass, every
  agent with a due unprocessed update. It includes the agents already running,
  so the number waiting is about `due_agents` minus the 8 running. It is the
  median 44 to 50 on the paper from 05:00, so about 36 to 42 agents wait.

Bias, all [V] from the code:

1. It logs only waits of 30 s or more, so it cannot show the short end. The
   FP0 count (678 over 5 min, 782 from 1 to 5 min, 33 from 10 to 60 s) is the
   tail by construction. I reproduced it: over console.log.2, .1 and current,
   682, 799 and 33.
2. It logs at activation start, so a wait is recorded only once it ends. An
   agent still queued, or one whose queue wait ends in a restart, logs
   nothing. Each of the 8 boots on this night (03:31, 03:51, 04:04, 06:25,
   06:53, 07:28, 08:05, 09:36) dropped its queue.
3. `slots=8/8` is close to a tautology when a queue exists: an activation
   starts only when a slot frees, and it takes that slot. The 8/8 says a
   queue exists; it does not say what the holders do.
4. `waited` is from the oldest due update and includes time that is not slot
   contention: the agent's own previous activation still running (the
   serial-per-agent fence), a handler-retry hold, and wake-outbox projection
   delay (measured below, small).
5. One line per activation, not per update.

The distribution shows the logged waits are a queue, not a few stragglers: of
968 lines in the stable window 04:04 to 06:25 (excluding 15 minutes after a
boot), `slots_full` is 961 and the median wait is 202 s.

## Q2. What the 8 slot holders wait on

[V] A slot is held for the whole run. `ExecuteActivationSync`
(`internal/agentcore/runtime.go`, comment at lines 494 and 495): "the caller's goroutine (the
actor goroutine) IS the run goroutine". Run events agree: for Texture run
a869ee10 (`GET /api/runs/a869ee10-.../events`) the second activation logged
`waited=12m11s` at 07:09:32, its first model call completed at 07:09:35, and
the run completed at 07:09:58.

[V] What the holders wait on, from the gateway journal for the paper's calls,
05:16 to 09:36 UTC (between gateway restarts, in-flight counted per second
from `inference request` to `inference succeeded` or `provider call failed`;
the request line is logged before the Synthetic gate, so it includes the gate
wait):

| Where the slot holder is | Mean slots | Share of 8 |
| --- | --- | --- |
| Inside a Synthetic call (gate in service) | 2.65 | 33 percent |
| Queued at the Synthetic gate (limit 3) | 2.88 | 36 percent |
| Inside a luna call (ChatGPT) | 0.45 | 6 percent |
| Not in a gateway model call (tool, search, store, other) | about 2.0 | 25 percent |

- Synthetic in flight at the gateway is 5.52 on average and is never above 8
  in 15,600 seconds. 8 is the slot count, so the callers are slot holders. It
  is 4 or more for 81 percent of the seconds and 6 or more for 64 percent.
- The gate is the queue: `/health` `synthetic_gate` at 09:53:57 (counters
  since the 09:36:10 gateway start) reads `limit 3, in_flight 3, peak 3,
  granted 268, waited 140, wait_timeouts 0, cancelled 0`. 52 percent of calls
  waited. No call has timed out on the 120 s bound.
- The gateway's `model_scheduler` is `mode off` (all counters 0), so there is
  no second queue at the gateway for luna or for the other providers.
- [H] The 25 percent that is not in a gateway model call is mostly tools and
  search inside `desk_go_eval` plus store waits. Evidence for tools: in three
  sampled Synthetic research runs (025f6b2d, 1263e0d9, 2fff2d52) the span is
  532 to 972 s, model time 336 to 727 s, and `tool.result` durations sum to
  roughly 190 s each (about 20 to 25 percent). 23 `tool loop: slow tool call`
  lines (30 to 57 s each, all `desk_go_eval`) appear on the night. I did not
  separate tools from store.
- The store is a second-order contributor, not the first. [V] write-pool wait
  from the 73 `store_stalled` console lines is 0.20 to 0.30 goroutine-seconds
  of waiting per second over every process of the night (0.22 in the stable
  window 08:05 to 09:36; average 33 ms per acquire). [V] 09:48 to 09:51, 12
  to 15 minutes after the 09:36 boot, the engine mutex was held about 69
  percent of the time and waiters accumulated 0.75 wait-seconds per second,
  the write pool 1.1; this window is inflated by boot work (the derived-keys
  scan of `actor_wake_outbox.projected`, 70,464 rows at 208 rows a second,
  was still running). [H] steady-state engine-mutex wait is unmeasured and
  could be 0.2 to 0.8 wait-seconds per second.
- Wake delay before the dispatcher is small [V]: 341 `actor wake latency`
  lines (logged at 3 s or more) have median 5.5 s, p90 13 s, dispatch 0.17 s;
  max 122 s.
- Management tasks have their own queue [V]: `management_tasks` on the
  observability endpoint reads `slots 4, running 2 to 4, queued 82 to 102,
  oldest_queued_age_s 5,275 to 5,505` (09:39 to 09:51). Running fell below 4
  while 89 to 102 were queued, so that queue is not bounded by its slot count
  alone; [H] the slot starter's cadence or the dispatcher slots below it
  limit it. Not investigated further.

Non-causes read out [V]: no Synthetic 429s and no circuit-open lines since
the gate (the only failures after 05:16 are 5 to 6 Synthetic 502s an hour), `rate limit exceeded` (60 requests a
minute per computer) 7 and 8 lines in the 06 and 07 hours against 183 and 214
in the 04 and 05 hours.

## Q3. The per-pool split

Gateway side [V], 05:16 to 09:36 (4h20m):

| | Synthetic (DeepSeek V4.1 Flash 1017) | luna (ChatGPT gpt-6-luna) |
| --- | --- | --- |
| Calls | 4,513 succeeded | 2,502 succeeded |
| Rate | 17.3 a minute | 9.5 a minute |
| Mean in flight | 5.52 (2.65 in service, 2.88 gate queue) | 0.45 |
| Latency, request to response | 19 s (service 9.2 s, gate wait 10.0 s) | 2.8 s |
| Input tokens a call | 39.7k mean (median 31k, p90 84k), 93.5 percent cached | 12.3k mean (median 10k), 81 percent cached |
| Output tokens a call | 716 mean (median 227, p90 1,853) | 210 mean (median 132, p90 492) |
| Share of the 6.0 slots in a model call | 5.52 of 5.97 (92 percent) | 0.45 (8 percent) |

- Latency uses Little's law over per-second in-flight counts; one-second
  resolution under-reads luna by up to a second.
- [H] Service time follows output tokens: nine half-hour buckets give service
  7.5 to 10.6 s against 542 to 880 output tokens a call, and luna serves 210
  tokens in 2.8 s. About 75 tokens a second in both pools.
- Roles [V] (luna `cache_label`, 03:00 to 09:50): management_task 2,121
  calls, research 1,538, texture 745, management 159, engineering 81. Roles
  are not tied to a pool: the split is per thread (1:1 weights,
  `internal/modelcatalog/catalog.go`, `SyntheticLunaPool`), so every role
  appears on both pools.
- Guest side [V] (`loop.progress.provider_call_ms`, run events): 26 sampled
  Texture cells after the gate (05:25 to 09:30): Synthetic 13 cells, 4.3 calls
  a cell, call median 20.6 s (mean 26.3, p90 54), 113 model-seconds a cell;
  luna 13 cells, 2.6 calls a cell, call median 3.5 s (mean 3.9, p90 7), 10
  model-seconds a cell. Before the gate (04:20 to 05:12, 8 cells, successes
  only): Synthetic call median 5.1 s (mean 15.0), luna median 3.8 s. So the
  gate moved the Synthetic call from 5 s to 21 s at the median and left luna
  alone.
- [V] Per-pool slot time, one Texture cell: about 113 slot-seconds on
  Synthetic against about 10 on luna, 11 times. Research runs on Synthetic
  are the longest: 21 to 49 calls, 330 to 970 s (sampled 3 of 3 above 500 s,
  one cut by the 06:53 restart); on luna, 3 to 22 calls and 32 to 300 s.
  Management-task runs (8 sampled, all luna): 5 to 16 calls, 14 to 121 s.
- [H] Synthetic-pool research is the largest single user of slot-seconds.
  The sample is 16 runs of 8 research and 8 management-task runs; no run
  listing exists to weight them (`GET /api/runs` timed out at 90 s; not
  retried).

## Q4. Steady or bursty, and the 05:1x change

[V] Steady since 05:16. From the console, all lines, per clock hour (UTC;
lines are waits of 30 s or more):

| Hour | Lines | Median wait | p90 | Over 5 min | due_agents median |
| --- | --- | --- | --- | --- | --- |
| 03 (from 03:24) | 137 | 52 s | 107 s | 1 | 21 |
| 04 | 388 | 81 s | 194 s | 23 | 33 |
| 05 | 482 | 246 s | 536 s | 183 | 50 |
| 06 | 449 | 410 s | 649 s | 293 | 52 |
| 07 | 450 | 260 s | 762 s | 189 | 46 |
| 08 | 457 | 233 s | 594 s | 140 | 42 |
| 09 (to 09:39) | 353 | 353 s | 623 s | 241 | 49 |

Some hours include a boot (06:25, 06:53, 07:28, 08:05, 09:36), each of which
re-forms the queue and adds the long tail (max 1,868 s at 06:xx, 2,149 s at
08:xx). The stable windows give the clean read: 04:04 to 06:25 (excluding 15
minutes after a boot) median 202 s, p90 576 s, 330 of 968 over 5 minutes; and
08:05 to 09:36 median 275 s, p90 606 s, 282 of 612 over 5 minutes, 612 of 612
at 8/8. The logged rate holds near 7 to 9 activations a minute in every
window, and `due_agents` plateaus at 42 to 52 (it does not grow), so the
queue is a full steady-state queue, not a growing backlog. By Little's law
44 waiting agents at a 275 s mean wait is about 9.6 activations a minute,
which matches the logged rate; so a slot is held about 50 s on average
(8 slots / 9.6 a minute).

[V] The step at 05:16 (gate 8 to 3):

| Window | Lines | Median | p90 | Under 60 s | Over 5 min | due_agents median |
| --- | --- | --- | --- | --- | --- | --- |
| 04:19 to 05:16 (gate 8, 429s) | 414 | 91 s | 327 s | 106 | 42 (10 percent) | 35 |
| 05:16 to 06:25 (gate 3) | 556 | 338 s | 623 s | 0 | 288 (52 percent) | 60 |

Synthetic gateway latency per call went from about 5 s (04:00 to 05:00) to 17
to 22 s (05:30 onward). [H] Cause: before 05:16 a Synthetic call that met the
account's limit failed in a second (1,159 calls failed with 429 or circuit
open between 03:00 and 05:16; 497 429s in the 05 hour alone before the gate),
the run errored, the activation ended early and the slot freed; after, the
call waits about 10 s at the gate and then runs. The work that used to fail
now waits, and the wait holds slots. The 05:18 reading in the ledger (30 a
minute against 21) was a 2.5 minute reading; sustained over four hours the
successful Synthetic rate is 17.3 a minute (537 to 623 per half hour), the
same as the 8-gate period's successful rate (355 to 677 per half hour,
12 to 22 a minute). [H] Gate 3 costs no successful throughput against gate 8
at this load, so Synthetic's usable concurrency on this key is about 3 to 4
now.

## Q5. A desk cell's model calls and duration

[V] A Texture cell is two or three dispatcher activations, each re-entering
the queue, and the model work is the last one. Run a869ee10: `loop.submitted`
06:56:14; activation 1 logged `waited=4m8s` at 06:55:35 and posted the run;
`texture.agent_revision.started` 06:57:21 (67 s, wake delay 65 s);
activation 2 logged `waited=12m11s` at 07:09:32; four luna calls
(`provider_call_ms` 2,503, 5,775, 3,612 and 3,850), the revision
created at 07:09:58; activation 3 on the same agent logged `waited=4m20s` with
5 updates at 08:13. The cell did 16 s of model work and 25 s of loop in 825 s
of wall time.

Sampled Texture cells (post-gate; `start->first call` is from
`texture.agent_revision.started` to the first model call, which is the queue
wait of activation 2):

| Run | Pool | Calls | Model s | Start to first call | Total wall |
| --- | --- | --- | --- | --- | --- |
| a869ee10 | luna | 4 | 16 | 734 s | 825 s |
| 06528d84 | luna | 7 | 29 | 616 s | 647 s |
| 5c48f360 | luna | 2 | 6 | 338 s | 343 s |
| ef27ace3 | luna | 2 | 9 | 241 s | 254 s |
| e772ca5c | luna | 4 | 19 | 175 s | 207 s |
| daa914cd | luna | 1 | 4 | 115 s | 116 s |
| 22294e77 | luna | 2 | 4 | 141 s | 156 s |
| 39a8f25b | Synthetic | 5 | 274 | 234 s | 483 s |
| 5d80c280 | Synthetic | 4 | 121 | 304 s | 407 s |
| 64bad260 | Synthetic | 4 | 71 | 256 s | 349 s |
| 7a26c0ed | Synthetic | 4 | 53 | 213 s | 260 s |
| 9cecf25b | Synthetic | 7 | 172 | 48 s | 180 s |

Mean wait to first call: 250 s (Synthetic) and 284 s (luna) post-gate, against
31 s and 60 s before the gate (8 pre-gate cells). Other desks: management-task
runs 5 to 16 calls, 8 to 48 model-seconds, spans 14 to 121 s (model share 39
to 79 percent). Research runs 2 to 49 calls (above).

[V] Cell outcomes are often held: of the story-publish lines on the night, 167
published and 358 were `not published` (it cites 0 or 1 outlets; a story
publishes at 2 or more), against 580 Texture cell turns (352 `done:true`).
[H] A held cell is queue and slot time spent on a revision that does not
publish; whether they are wasted depends on the next revision. Not analysed.

Restarts cost work [V]: 8 boots in the 6 hours; 100 `passivated run` lines;
the 06:53 restart cut a 16 minute Synthetic research run
(025f6b2d, `activation.passivated`).

## The bottleneck as far as the traces show it

1. [V] The binding resource is dispatcher slots: 8 shared slots, each held for
   a run's whole life, FIFO by oldest due update, with 36 to 42 agents
   always waiting.
2. [V] Slot time is mostly spent in model calls, and most of that is
   Synthetic: 5.5 of 8 slots, of which about half sits in the gate queue
   (3 in flight) doing nothing. 6 percent of slot time goes to the luna calls
   that are 36 percent of calls.
3. [V] Synthetic's throughput is capped by the gate (3 at about 9 s a call is
   0.33 a second; observed 0.29), so for Synthetic-thread work the gate is the
   rate limit and slots are the concurrency; for luna-thread work only the
   slots limit it, and Synthetic holders starve it. [H] If gate waiters
   released their slots, luna and tool work would have up to 2.9 slots more.
4. [V] Hand-offs multiply the cost: each Texture cell re-queues 2 to 3 times,
   and a queue wait (about 5 minutes) is paid again on each pass.
5. [H] The store contributes but is not the first-order limit in steady state;
   its engine mutex is busy about 70 percent after a boot and could become the
   next limit if slots or gate rise.

## Options (not chosen), with the cost the evidence implies

| Option | What it moves | Cost the evidence implies |
| --- | --- | --- |
| More dispatcher slots (8 to 12 or 16) | Concurrency of luna calls, tools and store work. | Synthetic gets no more throughput, so added Synthetic-thread holders only deepen the gate queue: with all slots Synthetic-bound, mean gate+service time is slots / 0.33 a second, 24 s at 8, 36 s at 12, 48 s at 16 (the 120 s bound is not reached on the mean; bursts could reach it, and a timeout is a run error with no fallback). Memory per slot is small: a desk worker is about 20 MiB idle (RES-17) and the guest has 7.9 GiB, 4 vCPUs, 4.3 GiB available of 7.8 GiB at 09:50, daemon RSS about 2.6 GiB in the RES-18 reading [H on busy-worker size]. The real risk is the Dolt engine mutex, about 70 percent held at 09:50 [H], and CPU on 4 vCPUs. |
| Higher Synthetic gate (3 to 4, 5, 6) | Synthetic rate by about +33, +67, +100 percent if service time holds at 9 s. | 429s: the account refused at 8 (peak 7; 481 of 803 calls refused in 15 min at 05:11 before the 3), and clean at 3 (0 in 4.3 hours). Where between 3 and 7 the limit sits is unmeasured; 429s cost no tokens but are run errors with no fallback. Allowance: 64.8 percent remained at 09:29; the burn was 6.1, 5.5, 6.7 and 8.0 points an hour over the last four hourly readings (about 0.65 points per 100 successful calls at 17 a minute), so gate 4 to 6 projects to about 9 to 13 points an hour (8.0 was the last hour's reading at today's rate), 5 to 7 hours of allowance against 8 to 10 now [H, linear in calls; regen semantics of `weekly_next_regen` 12:39 not modelled]. Rollback is one env upsert and a gateway restart (as at 05:16). |
| More luna share (1:1 to 1:2, 1:3, or all luna) | Slot-seconds a cell: about 113 to about 10, and gate queue shrinks. A Synthetic-to-luna move frees roughly 100 slot-seconds a cell. | Loses the owner's experiment (how much Synthetic allowance the paper uses). Luna capacity is unmeasured: no luna 429 or failure in 2,500 calls on the night, but the ChatGPT plan's limit is unknown [H] and reasoning is `high`. Luna calls are smaller (12k input, 210 output tokens) but there is no per-thread cost reading. Model-policy change is a config switch (`System/model-policy.toml`), reversible in one act. |
| Fewer cells or fewer queue passes per story | Queue passes a Texture cell pays (2 to 3 today) and cells that end unpublished (358 of 525 publish decisions). | Code change in Texture/dispatcher hand-off (orange), no Synthetic or allowance cost, cuts burn. Needs a design: keep the slot across the submit and loop passes, or run the loop in the first activation. Unknown: whether held cells are needed as input for the next revision. |
| Scheduling (lanes, caps, order) | Who gets the slots: for example cap Synthetic-bound activations near the gate size plus a small queue (about 5), reserve slots for luna threads and short desks, or put research in its own lane (the source lane already is one, 4 slots). | No new resource; costs complexity. The dispatcher does not know an agent's pool today (the thread key resolves it in the model policy) [H on how to look it up]; a cap risks starving research. FIFO by oldest due update is fair but ignores run length, and slots are held 50 s on average with research runs holding 300 to 970 s. |

## What is not known, and the next read-only probes

- Slot-seconds by role and pool: log activation duration (end minus start)
  with the agent kind and pool; today it must be rebuilt from sampled run
  events. A guest change; not made here.
- Whether the 25 percent non-model slot time is tools, search or store.
- The steady-state engine-mutex wait away from a boot (a sample 30 minutes
  after a boot; the paper is restarted about every 45 minutes, so a quiet
  window is rare).
- Synthetic's concurrency ceiling between 3 and 7, and the weekly allowance's
  regeneration rule.
- A run listing weighted by kind (`GET /api/runs` timed out at 90 s).

## Method notes

- All reads were GET or journal and file reads: console logs read from the
  host's `vm-state` directory on node-b, gateway journal `journalctl -u
  go-choir-gateway`, `scripts/proofs/fp7-gateway.sh --step 10` (writes two
  state files under TMPDIR), observability polled 24 times at 5 s, run events
  for 52 run ids taken from console lines (two had no model calls).
- Gateway pairing: `inference request from autoputer vm-1669...` to
  `inference succeeded` / `provider call failed`, per provider, in-flight reset
  at each gateway restart (05:15:55, 06:24:32, 06:52:29, 07:28:04, 08:04:59,
  09:36:10). Second-resolution; short luna calls are under-read.
- The console logs are size-rotated (about 1 MB each); `.4` through current
  cover 03:24 to 09:39 without a gap.
- The bias in the activation wait line (Q1) applies to every wait statistic
  here that comes from it.

## Heresy delta

discovered: the activation wait line cannot show what slot holders do and
reads 8/8 by construction under a queue; the Synthetic gate parks about a
third of the slots idle; the fixed 05:18 throughput reading (30 a minute)
did not hold over four hours. introduced: none. repaired: none.
