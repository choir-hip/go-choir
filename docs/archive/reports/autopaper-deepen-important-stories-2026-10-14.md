# Autopaper: deepen important thin stories with web research (design note)

Date: 2026-10-14. Owner direction about 13:40 UTC: "if it's important, we
should be devoting more resources towards making sure we do have a quality
article. That is doing web research in addition to pulling from our source
ingestion." Problem record:
[front-page-leads-with-a-one-source-stub-2026-10-14.md](../problems/front-page-leads-with-a-one-source-stub-2026-10-14.md).
Part 1 (depth in the rank) and LF1 (sections) landed in 1f84fa908; this
note is part 2.

## What the trace says about the Mauritius stub

Read from the paper's ledger (computer-1355b83b…, document
e097eeb5-6fba-5829-bd26-5294a14a7732, research run
dc4c5adc-ae98-4112-a994-b231ab927b1e, 12:56:55–12:57:40 UTC), before
attributing anything:

- Texture cast research one question round: verify the riots and find
  independent or primary corroboration.
- Research searched its corpus first (`SearchWireCorpus`, 0 captures:
  "searched this computer's newest 1000 captures"), then **did search the
  web**: three `SearchStart` queries. Each started on 1 or 2 providers with
  7 or 8 of 9 **cooling** (searxng, tavily, parallel, exa, serper, serpapi,
  chatgpt, and for one query brave).
- It read two pages, Africanews (AFP) and the BBC article whose headline
  the story carries, and both were captured.
- Its one Report (`work_disposition: completed`, about 45 s after it
  started) cited **one** source, the Africanews capture. The BBC page it
  read was not in the packet, and its open question said the capture
  Texture opened from "was not identifiable in the available corpus".
- Texture published v3 from that packet: one paragraph, one citation.

So the stub is not "research never searched the web". It is one shallow
research round that stopped after one source, with the web search running
on one or two of nine providers. A sample of the paper's twenty newest
research runs (13:22–13:41) shows the same provider state in every run
that searched: 1 or 2 providers live, 7 or 8 cooling. Deepening will be
bounded by that provider state until it is fixed; it is a separate,
substrate problem (search provider circuits), named here as a residual,
not fixed by this note.

## What exists to build on (read from code, 2026-10-14)

- **Durable ticks:** the Autopaper source schedule
  (`internal/agentcore/source_schedule.go`) arms a `source_poll_due`
  actor wake through the generic `rt.scheduleActor`. A tick makes no
  model call: it polls, then appends one system Report record to
  `management:<owner>` (`capture_arrival.go`), which wakes management
  through the existing `inbox_record` path. Schedules survive restarts as
  standing intents (`recoverSourceSchedulesAtBoot`; `source_poll_due` is
  in `RestartWorkKinds`). A schedule carries only a source id: there is no
  generic "standing instruction" schedule.
- **Reaching research for an existing story:** management cannot cast
  research (`recordsRoute` allows it `texture:<doc>` and engineering). It
  casts or notes the story's Texture (`choir.Cast("texture:<doc>", …)`),
  and Texture opens research with a question packet
  (`textureprompts/texture.yaml`). Research already searches the corpus
  first, then the web (`promptstore/defaults/research.yaml`); there is no
  flag that requires web search, only the objective and questions passed
  down.
- **Management tasks (S2a):** only the `intake` kind is open, leased on
  captures. A `deepen` kind would need a new kind, a non-capture lease and
  validation changes.
- **What management can see:** `choir.Stories` gives doc_id, headline and
  version, with no importance or depth. `choir.ProductAPI` cannot reach
  `/api/platform/...`. The public front route
  (`/api/platform/publications/front`) carries publication_id,
  importance, promo, section, source_count and words; publication_id is a
  deterministic hash of owner and doc id
  (`platform/publication_versions.go`), so a guest can map its own doc ids
  to front rows. Whether a guest can GET that route through its proxy URL
  is not verified.
- **Standing questions (L4):** intent only; nothing runs.
- **Host side:** `corpusd front-importance` (every ten minutes) already
  reads importance and source_count for every front story, but it has no
  path into a guest's inbox.

## Chosen mechanism (recommended, not yet built)

A **deepen tick on the paper**, copied from the source-schedule pattern,
that hands management a list and lets the existing desks do the work:

1. A `story_deepen_due` actor wake, armed by `rt.scheduleActor` every 20
   minutes, gated by a new owner computer setting `paper.deepen` (boolean,
   default off, so the owner turns it on). Added to `RestartWorkKinds` and
   re-armed at boot beside `recoverSourceSchedulesAtBoot`; a crash closes
   only the tick in flight.
2. The tick makes no model call. It GETs the public front list (limit
   100) through the proxy, maps the paper's own story doc ids to
   publication ids, and picks stories with importance >= 7, source_count
   < 3, not promo.
3. Bounds applied in the tick, recorded in the record: at most **3 per
   hour**; a story is not named again within **6 hours**, or until its
   latest revision changes; a story named twice without its source count
   rising is dropped for the day (no endless deepening of a story the web
   cannot deepen).
4. It commits one system Report record to `management:<owner>`
   (provenance `system:deepen`, a deterministic id over the story and
   revision set, like `CaptureArrivalRecordID`) naming each story's doc
   id, inbox, importance, source count and words.
5. One short management prompt paragraph, separate from the capture
   intake section: a deepen record names important thin stories; for each,
   `choir.Cast("texture:<doc>", "deepen", spec)` asking for independent
   reports and the primary record **from a web search**, not only the
   corpus, and naming the sources already cited. One line in Texture's
   research guidance: a deepen request's research round cites at least
   three independent sources, or says why it cannot.
6. Cost: each deepen is one Texture turn plus the research round(s) it
   opens; those runs carry their own model-cost records. The tick logs
   each record it sends; the bound caps spend at 3 deepens an hour.

Why this over the alternatives:

- **Management reads the front list in its turn context.** It would put
  importance into every management turn, the capture intake turn the L2a
  merge-recall work is tuning, and spend a model turn per check. Rejected:
  it overlaps the intake prompt and costs a turn whether or not anything
  is thin.
- **A host timer** (extend `front-importance`). It already knows the thin
  stories, but has no path into the guest inbox and holds no owner
  authority on the paper. Building that path is a larger authority change.
  Kept for measurement only (below).
- **A `deepen` management task kind.** A new kind, lease and validation
  for a job one Cast does. Rejected for now.
- **Raise the bar for every story** (research cites three sources before
  it completes; Texture does not publish a one-source story when research
  read more). The Mauritius trace points here too: research read two pages
  and reported one. It costs research on every story, the owner asked to
  spend on important ones, and it is prompt tuning before the cause is
  understood (CLAUDE.md, understand before tuning). Worth a separate,
  traced look at why research reported one of the two pages it read.

## Why the deepen tick is not built on this branch

It is not a prompt and a small data surface. It adds a durable wake kind
with restart semantics (`RestartWorkKinds`, boot re-arm), a computer
setting, a guest-to-proxy read of the public front route (reachability
unverified), and a new system record provenance into management's inbox,
which the V2 vocabulary fence already refused once today (0a25fe6cf,
management_task). The prompt paragraph lands in the same management
prompt and overlay files the L2a agent is working in. That is a station:
orange with red edges (runtime wakes and restart behaviour), owing a
problem-first slice, a staging proof on the paper and a rollback through
the setting.

Decisions needed before building it:

1. **Budget:** 3 deepens an hour is the proposed cap. Research is already
   about three quarters of the paper's token spend; the owner should
   confirm the extra spend, or name a cap.
2. **Search providers first?** Every research run sampled today searched
   on 1 or 2 of 9 providers, the rest cooling (problem class:
   `search-rate-limit-benches-provider-for-a-day-2026-10-11.md`,
   `search-plane-empty-result-benches-healthy-providers-2026-10-11.md`,
   `search-plane-cooldown-on-caller-cancel-2026-10-09.md`). Deepening on
   one provider may find little more. Recommended order: restore provider
   health, then turn the deepen tick on.
3. **Owner switch:** default off, turned on by the owner act
   (`paper.deepen`), so the change ships dark.

Lead's choices (2026-10-14, about 14:05 UTC, under No Blocking Asks; the
owner asked for more resources on important stories, so the spend is in
scope):
1. Budget: 3 deepens an hour, as proposed; the depth log and /api/costs
   report the extra spend, and the cap is the setting's to change.
2. Order: search provider health first
   (docs/archive/problems/search-providers-cooling-most-of-the-time-2026-10-14.md,
   in progress, with a clustering assessment of the three earlier
   benching problems), then the deepen tick.
3. Rollout: ships behind `paper.deepen`, default off, turned on for the
   paper once provider health is restored and verified.

## What landed now: the measure

`corpusd front-importance` logs, every run, the depth of the paper's
important stories (importance >= 7, not promo, depth known): how many,
how many cite three or more sources, their median source count, and the
thin ones by publication id (`internal/frontimportance/depth.go`). It
reads only what the scorer already fetches, writes nothing new, and runs
every ten minutes, so the journal is the before/after series:

    journalctl -u go-choir-front-importance --since -6h | grep 'depth:'

Read at about 13:45 UTC (front list, computed the same way): of the 11
important stories, 6 cite three or more sources; the thin five are
Mauritius (1), Pope Leo (1), the Nvidia test body (1), the Nord Stream
trial (2) and the Turkish TV raid (2).

Success for the deepen tick, once on: over a six-hour window, the share of
important stories citing three or more sources rises from about 55% and
their median sources rise from 3, at no more than 3 deepens an hour; a
thin story named twice without gaining sources counts as a miss.

## Mutation class and rollback

- Measure (landed): yellow, a log line in a host job; rollback is
  revert.
- Deepen tick (proposed): orange (runtime behaviour, model spend) with
  red edges (durable wakes and restart behaviour). Rollback: turn
  `paper.deepen` off (closes the chain at the next tick), then revert.
