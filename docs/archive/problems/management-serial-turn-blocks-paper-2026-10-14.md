# Management's Serial Turn Stops The Paper For 30+ Minutes

Date: 2026-10-14, 03:50 UTC. Found by the owner ("Noticed last article is
28 minutes old. Is autopaper down?"). Computer: the live paper,
computer-1355b83bced1613eb048ee28d66688a8, build 66fe0917. Status:
documented; owner direction recorded. The Jev half (support judgement
leaves management) landed in `d5139fd6` and is live on the paper's build
9ace75e5: [design note](../reports/support-judgement-to-jev-design-2026-10-14.md).
Management delegating to sub-RLMs is designed, not built:
[design note](../reports/management-delegation-design-2026-10-14.md).

## What happened

Staging was healthy and all 14 source schedules were active and polling.
No story was opened or revised from 03:20 UTC on, because one management
run held management's only turn:

- Run `1565c043-84fc-465d-bd3b-bbe1551ba40a`, management, request source
  `inbox_record`, model `longcat-2.5-preview-free` (opencode-go, pool
  default, reasoning high). Submitted 03:13:54 UTC, still running at
  03:50.
- The trace (GET /api/runs/<id>/events), in order:
  1. iteration 1 lists its records with `choir.Records()`; among them
     `support_pairs` records (a Texture's new citations);
  2. iterations 2-3: compile errors guessing field names
     (`p.Cited.Block`, `c.DocID`);
  3. iterations 4-11: it checks the citations itself, reading captures
     one or two per cell with `choir.ReadCapture`. **It never calls
     `choir.Judge`**, which would have sent all pairs to Jev in one call
     (`internal/agentcore/support_judge.go`).
- Model call latency per iteration: 7 s, 6 s, 5 s, 5 s, then 2 min 16 s,
  9 min 0 s, 5 min 41 s, 6 min 24 s, 1 min 47 s, 1 min 38 s.
- Input tokens per call grew from 7,704 to 113,382 by iteration 10: every
  step re-sends all earlier cell output (LR lever 6's target).

Meanwhile new captures waited in management's inbox. Nothing was
dropped (A5 makes per-poll limits lossless); everything waited.

## Why (from the trace and the code)

1. **Management is serial.** It runs one turn at a time, and every kind
   of work routes through that turn: capture dispositions (open, feed,
   decline), Texture's Casts, Asks and Notes, and support judgement. One
   slow turn stops all of them.
2. **Support judgement is a management turn.** A Texture commit lists
   its new citation pairs to management as a `support_pairs` record; the
   prompt (`internal/runtimeprompts/overlays/rlm_management_runtime.yaml:21`)
   says to pass them to `choir.Judge` (Jev judges them) "or {key, verdict,
   why} maps for pairs you judge yourself". The second option invites
   exactly what this run did. Hypothesis, not yet confirmed across runs:
   management often judges by hand.
3. **A slow free model, with growing input.** Latency rose with input,
   from seconds to nine minutes per call.

## Owner direction (2026-10-14, 03:55 UTC)

> Yeah we can't do serial. Management should delegate to subrlms. And
> Indeed, source checking can probably be done by jev.

So, two changes, both substrate rather than prompt tuning:

- **Source checking leaves management.** A Texture commit's support
  pairs go straight to Jev on arrival, the way intake labels do on the
  poll path; a failure reaches the Texture, a pass changes nothing.
  Management no longer receives `support_pairs` records, and the "judge
  yourself" option goes away.
- **Management delegates.** Management stops doing work items serially
  in its own turn: it hands bounded pieces to sub-RLMs (child
  activations, as `choir.Research` children already are for research)
  and stays free for dispositions. A design note and a panel come
  before code; it touches the management runtime (red).

## After Jev took support judgement (08:30 UTC read)

Management is still serial with support judgement gone. Run
`305a582d-85c9-44ae-8a31-ef63084769c0` (07:12-07:40 UTC,
deepseek-v4.1-flash, 1,680 s) spent its first turn on one Texture Ask
(seq 469), then disposed of the captures that had waited. Its first act
committed 22.6 minutes into the run. 58 model calls, 4.1 M input tokens.

**New problem: a management cell can run 14 minutes with no host bound.**
In that run, cell 6 (a string search over `choir.Pack()`) ran 860,565 ms
and ended `frame: read header: EOF`: the worker died, and nothing timed
it out. A cell has a deadline only when the model sets `timeout_ms`
(`internal/agentcore/tools_desk.go:442-444`). Hypothesis, not
confirmed: the cell's hand-written index loop did not advance and grew
memory until the worker died; the exit cause was not recorded.

Window counts (management inbox seq 1-514, 10-13 15:22 to 10-14 08:23
UTC): 154 management runs, duration p95 812 s, max 3,601 s; management
in a run 46% of the time; 105 of 132 capture arrivals found it busy;
arrival-to-disposition estimated from run boundaries at p95 about 21-25
minutes. In the 02, 03 and 04 UTC hours the inbox has only 1, 0 and 1
capture arrivals, so during the 38-minute run what waited was mostly
Texture requests. Details and the design:
[management-delegation-design-2026-10-14](../reports/management-delegation-design-2026-10-14.md) §2.

## S0 baseline (09:12 UTC read, exact join)

`scripts/management_latency_probe.py` (delegation slice S0) over the live
paper, product routes only, window 10-14 06:51:00-09:12:07 UTC
(intake.labels on since 06:50:57). Output:
`~/.config/choir-qa/traces/management/s0-paper-20261014T091207Z.json`.

- 52 arrival records naming 239 captures. First dispositions: 87 opens,
  38 feeds, 69 declines, 45 none by 09:12.
- Arrival to first open or feed (125 captures): p50 225 s, p95 906 s, max
  926 s. Declines: p50 265 s, p95 922 s. AC1's bound (p95 at most 300 s,
  max at most 900 s) fails on the baseline, as expected.
- Parent turns (22 started, all finished): p50 106 s, p95 436 s, max
  1,680 s (305a582d); 1 over 600 s; 5 failed. Management was in a run
  53% of the window; 44 of 52 arrival records found it busy.
- Management cells: 295, p50 1 ms, p95 482 ms, p99 4.6 s, max 860.6 s
  (the one cell over 60 s, the dead worker above). Over the longer window
  10-13 15:00 to 10-14 09:12 (1,582 cells): p99 983 ms, and the same one
  cell over 60 s. So S1's default, max(300 s, 2 × p99), is 300 s.

**New observation, hypothesis only:** 45 captures have no disposition,
33 of them more than 15 minutes after arrival; 28 arrived in the 08:22
UTC burst, when two management runs failed (opencode-go circuit open:
e20d19a2 with no cells, a8081845 after 9 cells), and management's cursor
is now at its head. If a cell that shows records commits the cursor
advance and the run then fails before disposing of them, those captures
are never shown again. Not yet confirmed from the cursor trace; the
probe's join may also miss a disposition named only in free text.

## Related

- LR lever 6 (bulk cell output out of the transcript) cuts the input
  growth; it deploys after C1e B1.
- The cost-track report names management's one-turn queue as the serial
  ceiling for one computer
  (`docs/archive/reports/cost-track-sa-s3-sp-investigation-2026-10-14.md`).
- Model choice for management (a free preview model with minutes of
  latency) is a model-policy question for SM; pricing is being
  researched now (owner: "token pricing per model and provider is easily
  researched").
