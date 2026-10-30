# The research desk's structural flaw: a root cause (2026-10-11)

Problem record and structural assessment, written under "Convergence Before
Patching" (CLAUDE.md): more than three research problems this week trace to
one substrate. No fix here and no prompt changes. The owner asked: "Research
seems to have some structural flaw, needs root causing. Maybe we aren't
getting enough search results? Or searching wrong? Or not reporting back soon
enough? Or something else." And: "search needs to be fast, iterative, deep,
broad, recursive, comprehensive."

## Summary

**What I found.** Search returns plenty. The gateway gave research 1,084
result cards in demo19 and 767 in demo20, with one outage in 55 searches and
no rate-limit rejections. Breadth dies after search, for three structural
reasons.

1. **Research is not a loop. Each round is one short activation that
   Texture has to start.** All 11 research runs in the two demos were started
   by a Texture control. Each ran 67 to 126 seconds, made 5 to 9 model turns
   and 2 to 10 searches, then ended itself right after its first report. The
   only way to continue is for Texture to take a turn (1 to 4 minutes) and
   send another control, and Texture may send at most four per owner request.
   So each request gets about 4 × (≈5 searches, ≈3 claims). Observed: 13 and 15
   claims (10 and 11 sources) per demo, against the Haiku baseline's roughly 190 searches
   and 26 merged items. Research was busy about half the active time and
   waited on Texture for the rest.
2. **The runtime tells research to stop and report after every search, and
   to send 2 to 4 claims.** Every in-cell `choir.WebSearch`, fetch, import and
   read returns a leftover `next_instruction` from the deleted typed-tool
   path: "Submit a concise first update_coagent source packet from this
   search result before any additional search-only turn. Include 2-4 grounded
   claims". It fires on every call because its guard looks for legacy tool
   names that never appear on the cell path. Every sourced research packet in
   both demos carried 2 to 4 claims. This comes from code reading plus that
   correlation. No cell trace confirms it yet.
3. **A report only carries confirmed claims backed by an imported page.
   Unconfirmed leads have no path to Texture.** Items per round are capped
   by how many pages the model imports in one activation (2 to 4). The leads
   it saw ("possible OpenAI revenue, Nvidia investment, cloud deals") vanish
   when the activation ends. 5 of 11 research updates in demo19 were
   rejected, and 5 of 10 in demo20. Most were status-only reports that the
   activation prompt requires.

Secondary causes:

- Searches run one after another, and fan-out is barely used. Children were
  used in 1 of 11 runs, and the egress budget of 64 calls per activation
  would stop a baseline-sized fan-out anyway.
- The search projection marks working searches as "degraded" and tells
  research to "Report a precise blocker", so research blames search providers.
- Round budget goes to Texture turns woken by status-only reports.

**How sure.**

- Cause 1: high. Run records, document events and gateway logs all agree,
  and five prompt versions (v2 to v6) failed to change it.
- Cause 2: medium-high. The code path is unambiguous, but I could not read
  the cell output: the internal run-events route was outside my permissions.
- Cause 3: high on the funnel numbers, medium on how much of it is the
  injected instruction versus the evidence rule.
- Model effort (`gpt-5.6-luna` at `low`) is a contributing hypothesis that
  I cannot separate from the structure with this evidence.

**What I recommend.** Stop treating research as a request and reply that
Texture drives one round at a time. Research should be a job the runtime
owns. It plans threads, runs searches in parallel below the model, and
writes every result into a durable lead ledger. It streams unconfirmed
leads to Texture on a timer, confirms them asynchronously, and stops when
new items stop arriving (saturation), not after a fixed round count.
Most of the parts already exist; the sketch is at the end. Delete the
leftover checkpoint instructions from the cell path as part of that work,
not as a standalone patch.

## Evidence

Sources:

- Demo19: test computer demo-p2a-2026-10-11, document 07968375, staging
  a2bcf724, 19:19 to 19:48Z.
- Demo20: demo-p2c-2026-10-11, document 34f4781e, staging b172895b, 20:22
  to 20:46Z.
- Product API reads: `/api/runs`, `/api/runs/<id>`, `/api/trajectories/<id>`
  (packets with their dispositions) and the document events.
- Node B gateway journal lines for VMs vm-1b97a02218e9… and vm-d2b35d35….
- Saved revisions in the session scratchpad (demo19/, demo20/).

Run cell events (the code each cell ran and its output) were not
captured. Reading them needs the guest's internal run-events route, which
my permissions did not allow.

### Every research run is one Texture-started round

All 11 research runs have `request_source=lifecycle_texture_control`, and
each one's `requested_by_run_id` is a Texture run. No research run started
any other way.

| demo | run | span (UTC) | model turns | searches | claims / sources sent | input tokens |
|---|---|---|---|---|---|---|
| 19 | acec9ce2 | 19:20:15–19:22:14 | 7 | 7 (1 outage, retried) | 3 / 3 | 96k |
| 19 | 532df1d9 | 19:25:59–19:27:44 | 7 | 5 | 3 / 3 | 229k |
| 19 | 54d94e8a | 19:29:09–19:31:06 | 9 | 6 | 4 / 3 | 266k |
| 19 | 699687ea | 19:41:48–19:43:54 | 7 | 6 | 3 / 1 | 234k |
| 19 | fb27c3a9 | 19:45:28–19:46:47 | 9 | 4 | 0 / 0 (the AMD item had no source; rejected) | 255k |
| 19 | bcc0f77c | 19:46:57–19:48:12 | 5 | 4 | 0 / 0 (status only; rejected) | 37k |
| 20 | 03707bc1 | 20:23:05–20:24:15 | 5 | 3 | 4 / 3 (one claim unsourced) | 129k |
| 20 | adf76450 | 20:25:15–20:26:22 | 10 | 2 | 4 / 3 (two packets) | 404k |
| 20 | c26a952a | 20:37:02–20:38:51 | — | 5 | 3 / 2 | 370k |
| 20 | f17f4500 | 20:40:13–20:41:24 | — | 3 | 0 / 0 (status only; rejected) | 155k |
| 20 | 002ff285 | 20:42:58–20:44:59 | — | 10 (including one child fan-out of 3 parallel searches) | 4 / 3 | 64k |

How the columns were counted:

- Model turns come from `gpt-5.6-luna` inference requests in the gateway
  log. In demo20 children and desk share that model, so later rows are left
  blank.
- Searches come from gateway `search request` lines inside each run's span.
  Only one research run is active at a time and Texture never searches, so
  the attribution is unambiguous.
- Claims and sources come from `packet.claims` and `packet.sources` in the
  trajectory snapshot's updates, leaving out the separate status-only
  packets.

Each run ends with a final text message (gateway `text_len>0`, for example
19:22:14 `tokens=21602+170 text_len=741`) within about 10 seconds of its
report. The run's result text says what is still open, for example:
"Business, policy, safety, scientific, and infrastructure coverage remains
open … I reported `work_disposition=open`". The runtime does not end the
run: the tool loop has no turn cap (internal/toolregistry/toolloop.go).
The model ends it.

Continuing takes another control. Texture's turn between a research report
and the next control took:

- demo19: 3m53s (including a failed Texture run 7fa3f5cf), 1m35s and 1m43s;
- demo20: 65s, 96s and 98s.

The cap is `textureResearchRoundsPerOwnerRequest = 4`
(internal/textureowner/texture_turn_runtime.go:161, enforced at :304).
Demo19 used all four controls on both requests:

- first request: 19:20:15, 19:25:59, 19:29:09 and 19:30:09, reaching the cap
  ten minutes in;
- follow-up: 19:41:48, 19:45:27, 19:46:01 and 19:46:56, reaching the cap five
  minutes after the follow-up.

After the cap the document stopped, with chips, funding and policy "still
open" and an empty "Chips and compute" heading in revision 7.

Demo20's Texture stopped after two controls on the first request and three
on the follow-up. So the cap is not the only stop: low yield per round also
makes Texture conclude early.

Duty cycle: research was busy 10m21s of about 20m51s of active time in
demo19, leaving out the 7m36s the harness waited before the follow-up. In
demo20 it was busy 7m18s of about 15m31s. The rest of the time research sat
idle while Texture read, wrote and decided.

Prompt versions v3 to v6 each told research to keep going after a report
("Work in rounds. After each round, send Texture everything … then keep
going on what is still open"; internal/promptstore/defaults/research.yaml).
Texture's prompt says the same about research ("it keeps working after a
report"; internal/textureprompts/overlays/run_system.yaml line 31). The
pattern held before and after:

- problems/research-runs-stop-after-first-report-2026-10-10.md: eight runs
  of 35 to 63 seconds;
- PD1 receipts S1c to S1i;
- today: 11 runs of 67 to 126 seconds.

The prompt describes a desk the runtime does not implement. Nothing
re-activates research on its own open work. Only a Texture control or a
child call does.

### The runtime tells research to report after each search and to send 2 to 4 claims

Code path:

1. A research cell calls `choir.WebSearch`. The host runs the same
   registry `web_search` tool through `Dependencies.HostEgress`
   (internal/agentcore/tools_desk.go:326; internal/researchtools/host_egress.go:22).
2. The tool computes `shouldRequireResearchUpdateAfterTool`
   (internal/researchtools/researchtools.go:717). For the research profile it
   lists the run's events and compares the latest `update_coagent` result
   with the latest `web_search`, `fetch_url` or similar result. On the cell
   path every tool event is named `desk_go_eval` (batch_executor.go: the
   event's `"tool"` is the call name). Both sequence numbers are therefore
   0, and the function returns `latestResearch == 0`, which is **true on
   every call**.
3. `compactWebSearchProjection` then sets `next_instruction`: "Submit a
   concise first update_coagent source packet from this search result before
   any additional search-only turn. Include 2-4 grounded claims …"
   (researchtools.go:924). Fetch, import, read and selector results get the
   same "before any additional search/fetch turn" instruction (:324, :412,
   :500, :548, :621, :1022). `update_coagent` is the retired tool.
4. On the tool-loop path, `parseToolOutputProjection` would strip the
   envelope down to `model_output` (batch_executor.go:333). HostEgress
   returns the raw envelope to the cell, so the cell receives
   `{"__choir_tool_projection", "durable_output", "model_output",
   "projection"}`. That is the full results and the projected results
   together, and Go's sorted map keys put `durable_output` first.
   Whatever the cell prints is capped at 48,000 characters per value
   (yaegikernel/session_loop.go:191) and 100 KB per tool result
   (batch_executor.go:321).

Children run with the root's `RunID` (research_children.go:187), so they
get the same instruction.

Observed: every packet with sources in both demos carried 2, 3 or 4 claims
(demo19: 3, 3, 4, 3; demo20: 4, 2, 2, 3, 4). Every activation ended within
about 10 seconds of its first report. That is consistent with the model
obeying a runtime instruction to report "before any additional search-only
turn" and then also obeying the activation prompt "Before ending this
activation, use choir.Report to assert work_disposition"
(internal/agentcore/runtime.go:2957). Status: hypothesis with strong code
support. Confirm it from one research cell's output: whether
`next_instruction` text appears in the cell result, and whether the model's
next cell is a report.

This is the "existing replacement" check from CLAUDE.md, run in reverse.
The typed research tools were deleted on 2026-10-05 (SR cutover), but their
checkpoint cadence survives inside the tool funcs that the cell path reuses.
It is code that should be deleted, not patched.

### Breadth dies between search and report

The funnel, measured:

| stage | demo19 | demo20 | baseline (5 Haiku slices) |
|---|---|---|---|
| searches requested | 32 (30 distinct queries) | 23 | roughly 190 (about 37 per slice where counted) |
| searches succeeded at the gateway | 31 (1 outage) | 23 | — |
| result cards returned | 1,084 | 767 | — |
| searches at or above 29 s (probable guest timeouts) | 6 | 0 | — |
| claims reaching Texture | 13 | 15 | — |
| distinct sources reaching Texture | 10 | 11 | 139 links across the slices |
| research updates rejected by Texture | 5 of 11 | 5 of 10 | — |
| final document | about 9 items, 6 citations | about 12 items, 10 citations | 26 merged items, 43 links |

About 1 percent of the result cards became a cited source. The research
prompt says: "In a packet, claims cite sources by source_id; import a page
(choir.ImportURL) to make it citable." Texture rejects packets without
sources ("Status-only packet … Nothing citable to incorporate"). So the unit
of report is an imported, confirmed claim, and the import is a serial,
fallible fetch inside a short activation.

Leads have no transport:

- demo20 round 1 sent "Searches surfaced possible OpenAI revenue, Nvidia
  investment, cloud deals, and model-release stories, but these remain
  unconfirmed leads" with no URL, outlet or date.
- demo19 run fb27c3a9's result text names a dated AMD item from 6 October,
  but its packet had 0 claims and 0 sources and was rejected. The item came
  back three runs later in demo20, and never in demo19.

The packet schema already has a slot for a lead: `sources[].evidence.state`
`pending` and `evidence.confidence` (internal/coagentpacket/packet.go:170).
Neither demo used it, and nothing outside the model fills it.

This is the problem in
problems/research-leaves-leads-unresolved-and-unexplained-2026-10-11.md,
seen from the transport side. The owner wants unconfirmed findings sent
right away and confirmed later. That needs leads to flow without the model
transcribing them.

### Everything is serial; fan-out is rare and capped

- **Within a cell.** Each gateway search request starts within a second of
  the previous one finishing (for example 19:20:39, then 19:21:08, 19:21:38,
  19:21:42, 19:21:44). A six-search cell took 66 seconds. The one exception
  is demo20 run 002ff285, whose `choir.Research` children ran three searches
  in parallel at 20:43:46–48.
- **Across the desk.** There is one research run at a time per document
  (PD1), and none runs while Texture takes its turn.
- **Fan-out.** It is discouraged: the prompt says "use it only when a
  question splits into separate lines". Children cannot spawn children
  (yaegikernel/choir.go:263; PD1 owner direction). It runs at most 4 at once,
  within a 10-minute window.
- **Egress.** All of it is charged to the root activation's
  `DefaultResearchEgressMaxCalls = 64`
  (internal/researchtools/egress_budget.go:46; research_children.go:187).
  The baseline's five researchers made about 250 to 300 tool calls. A
  baseline-shaped fan-out would be refused at call 64.
- **Rate.** Research ran about 1.1 searches per active minute in demo19. The
  baseline ran about 47 per minute.

### Research misreads search health and explains thin coverage with it

- One outage and six searches of 29 to 30.4 seconds in demo19 (median 4 s).
  The guest's search client gives up at 30 s (internal/search/search.go:85),
  the same budget as each gateway provider call
  (problems/guest-search-timeout-equals-gateway-provider-timeout-2026-10-11.md).
- The projection marks a search `degraded` whenever any provider attempt
  failed, even when the merged target of 40 was met
  (researchtools.go:892, 926). It then adds "one or more providers failed or
  were unavailable". With Serper and SerpAPI out of quota, Exa failing to
  parse and SearXNG empty (PD1 S1g), most successful searches carry that
  flag.
- An outage result adds `next_instruction`: "Report a precise blocker from
  provider_health and attempts" (researchtools.go:922). Demo19's first search
  was that outage.
- Research's reports blame provider cooldowns, rate limits or timeouts in 8
  of 11 run results. Demo19 round 1 says coverage was limited "because search
  providers were largely in cooldown or timing out". The gateway logged 1
  outage in those 7 searches, and 2 searches near 30 s.

The search plane is mostly delivering. The desk is told it is degraded and
told to report blockers, and the low-effort model converts that into a
reason to stop.

### Round budget is spent on status reports

The activation prompt requires a `choir.Report` with `work_disposition`
before ending, so each run sends a second, status-only update. That update
wakes Texture too, and Texture rejects it. In demo19's follow-up, two of the
four controls came from Texture turns woken by rejected status packets:

- doc events 49 to 51 (19:46:01): `update_rejected`, then `control_queued`;
- events 59 to 61 (19:46:56): the same.

Those two rounds produced nothing citable, and the cap stopped research
afterwards.

### What is not the cause

- **Too few results.** 40 merged results per search (MinMergedResults 40;
  `compactWebSearchProjection` shows up to 40 cards with 400-character
  snippets). The model had about 30 to 40 cards per search in view and used
  about 1 percent of them.
- **Rate limits.** No `rate limit` lines for either VM. The pacing from
  3faa001a holds.
- **Provider outages.** 1 outage in 55 searches.
- **Report size.** Packets have no claim cap. The excerpt cap of 2,000
  characters was not reached.
- **Texture dropping sourced material.** Every packet with sources was
  `incorporated`. All rejections were of status-only or sourceless packets.

## Root causes ranked by evidence strength

1. **Rounds are model-ended activations that Texture must restart, under a
   count cap.** Evidence: high (11 of 11 runs; cap code; event timeline;
   failed prompt fixes). Effect: per request, at most about 4 × (≈5 searches,
   ≈3 claims), half the time idle.
2. **Report unit = confirmed and imported; no lead transport.** Evidence:
   high (funnel numbers, rejected packets, the lost AMD item, the empty
   evidence-state slot). Effect: about 1 percent of results become sources;
   leads die when the activation ends.
3. **Leftover checkpoint instruction on every cell world verb.** Evidence:
   medium-high (code path certain; claim-count correlation; cell output not
   yet read). Effect: a first report after one search batch, with 2 to 4
   claims.
4. **Serial execution and the egress cap.** Evidence: high for the
   seriality (gateway timestamps) and for the cap (code). Effect: about 1
   search per minute; a baseline-shaped fan-out is impossible under 64 calls.
5. **Search-health signal inverted at the desk.** Evidence: medium (code
   plus run-result wording; which guest calls timed out needs the cell
   trace). Effect: wrong explanations and abandoned branches.
6. **Status reports wake Texture and spend rounds.** Evidence: high in
   demo19's follow-up. Effect: 2 of 4 follow-up rounds wasted.
7. **Model and effort.** `gpt-5.6-luna` at `low`
   (internal/modelpolicy/model_policy.go:340); about 2k output tokens per
   run. Evidence: hypothesis only. It cannot be separated from causes 1 to 3
   until those are removed.

Root-cause clustering: these are one substrate. Several earlier problem
records are symptoms of it:

- research-runs-stop-after-first-report-2026-10-10;
- texture-research-loop-never-idles-2026-10-09, whose cure is the count cap;
- texture-research-opener-dropped-silently-2026-10-11;
- research-leaves-leads-unresolved-and-unexplained-2026-10-11;
- research-fanout-overruns-computer-model-budget-2026-10-11;
- research-parent-cannot-read-its-children-2026-10-11;
- guest-search-timeout-equals-gateway-provider-timeout-2026-10-11;
- PD1 receipts S1c to S1i (prompt v3 to v6).

The substrate: research's unit of work is the model's voluntary activation,
and Texture is the scheduler. Each patch moved the bottleneck between search
capacity, Texture's question choice and the citation gate. None changed it.

## A structurally sound research loop (sketch, not built)

Principles from the owner: fast, iterative, deep, broad, recursive,
comprehensive; one research desk that fans out when needed; report back
early, unconfirmed first and confirmed later; all inter-agent traffic as
precommitment records.

1. **A research job owned by the runtime, not a model activation.**
   - A Cast to `research:<subject>` opens a job with a budget (wall time,
     egress calls, model calls) and a stop rule. The job stops at
     saturation: the last N searches added fewer than K new distinct dated
     in-window items. Texture's round count no longer stops it.
   - The runtime re-activates the desk on its open job until the stop rule
     or the budget fires. Texture steers with Note or Ask and stops it with
     Retract or Cancel, but is not the scheduler.
   - This is continuation within a running computer. At boot, open jobs
     close as interrupted, as today ("Restarts End Work").
   - Reuse: the work-item lifecycle and target binding (internal/store);
     the PD2 persistent desk session; the continuation and deadline machinery
     in internal/agentcore/continuation_schedule.go; and the precommitment
     `Expectation` with `By` and count questions as the job's stated target
     (for example "≥20 new dated items by T+10m, p=0.6"), resolved by count.
   - The never-idle risk that the round cap guards against moves into the
     job's budget and saturation rule.
2. **A lead ledger below the model.**
   - Every search result becomes a typed candidate item: canonical URL,
     title, `published_at`, snippet, query, thread and provider. Items are
     deduplicated and stored durably outside the model's context.
   - Reuse: the search plane's merge and dedupe (internal/gateway/searchplane);
     the W1 shared capture objects (`choir.web_capture` in Store B through
     corpusd, read with `ReadCapture`), which already hold title, URL, dates
     and body outside Dolt; and `SaveEvidence` and `ListEvidence`.
   - The model reads and ranks the ledger. It does not have to transcribe
     search JSON into packets.
3. **Leads stream to Texture first; confirmation follows asynchronously.**
   - On a timer (for example every 60 to 90 seconds) or at a count threshold,
     the runtime flushes new in-window items as one packet whose sources
     carry `evidence.state=pending` and a confidence. The schema already has
     both fields.
   - Confirmation (import the page, find the primary record or a second
     report) runs in parallel workers. Each lead resolves to confirmed,
     refuted or likely-with-reason through `Resolve` on its record.
   - Texture's wakes are debounced, so it batches packets instead of taking
     one turn per packet.
4. **Breadth through planned, parallel fan-out.**
   - The opener already lists the threads (demo20's opener had five
     questions). Run one child per thread at once. Children return
     structured items (a JSON array of the ledger shape), not prose.
   - Size the egress budget to the job tree, not to 64 per activation.
     Keep the computer-wide model-call spacer (research_children.go:36).
   - Give the cell a batch search verb, or let one cell run searches in
     parallel, so a thread's 5 to 10 queries take one search latency, not
     their sum.
   - "Recursive" can mean query-frontier expansion: each result proposes
     follow-up queries (names, primary sites, dates) that the job runs
     within budget. That gives depth without the agent trees the owner
     turned down on 2026-10-11.
5. **An honest search contract.**
   - The guest's deadline should be a request-scoped deadline passed to the
     gateway, not two fixed 30-second timeouts.
   - Mark a search `degraded` only when it missed its merged target.
   - Remove `next_instruction` from every cell-path result: it is a leftover
     of the deleted typed-tool cadence.
   - Return `model_output`, or a typed struct, to the cell instead of the
     doubled envelope.
6. **Status as state, not as an update.**
   - `work_disposition` is a field on the job, not a separate update that
     wakes Texture.
   - Only packets with items or resolutions wake Texture.
7. **Measured, not read by hand.** The job's precommitment records score
   themselves:
   - items per minute;
   - time to first lead;
   - leads confirmed, refuted or left open;
   - distinct dated items and citations against the Haiku baseline.

   This is the evaluation the owner asked for before more hill climbing.

Mutation class of building this: red. It touches the Texture control path,
the run lifecycle and continuation, the gateway search contract, and the
evidence packets. Item 5's deletion of `next_instruction` is orange on its
own, but it should land with the loop change rather than as a patch measured
by itself.

## Next observations (to move hypotheses to findings)

1. **Capture one research cell trace** with the guest's internal run-events
   route (scratchpad trace_snapshot.sh), on any research run. Check whether
   the cell output carries `next_instruction` and the `durable_output` /
   `model_output` envelope, and whether the model's next cell is a report.
   This confirms or refutes cause 3.
2. **Match guest search errors to gateway successes.** In the same trace,
   pair each guest error with the gateway's 29 to 30 second successes to
   count real guest timeouts (cause 5).
3. **Isolate the model-effort hypothesis.** Once causes 1 to 3 are removed,
   run the same request with research at `medium` effort.

## Open questions for the owner

1. **Who stops research?** Should a research job stop on saturation and a
   budget that the runtime enforces, instead of Texture's four rounds per
   request? The round cap exists only to stop the never-idle loop.
2. **What does "recursive" mean?** Is query-frontier expansion (results
   seed the next queries, within one job) the recursion you mean, given
   your 2026-10-11 direction against recursive agent fan-out?
3. **Should leads stream without the model?** Should unconfirmed leads reach
   Texture straight from the search results, marked pending, with no model
   step? And should Texture show them before confirmation, as you asked?
4. **How big should a research job's budget be?** The baseline used about
   250 to 300 calls for one weekly-news request. The current cap is 64 per
   activation, and the gateway allows 60 model calls a minute per computer.
5. **Should the lead ledger be the W1 shared capture store?** That would
   share it with Autopaper. The alternative is a per-computer evidence table.
