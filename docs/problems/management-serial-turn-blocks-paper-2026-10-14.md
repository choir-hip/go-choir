# Management's Serial Turn Stops The Paper For 30+ Minutes

Date: 2026-10-14, 03:50 UTC. Found by the owner ("Noticed last article is
28 minutes old. Is autopaper down?"). Computer: the live paper,
computer-1355b83bced1613eb048ee28d66688a8, build 66fe0917. Status:
documented; owner direction recorded; not fixed.

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

## Related

- LR lever 6 (bulk cell output out of the transcript) cuts the input
  growth; it deploys after C1e B1.
- The cost-track report names management's one-turn queue as the serial
  ceiling for one computer
  (`docs/reports/cost-track-sa-s3-sp-investigation-2026-10-14.md`).
- Model choice for management (a free preview model with minutes of
  latency) is a model-policy question for SM; pricing is being
  researched now (owner: "token pricing per model and provider is easily
  researched").
