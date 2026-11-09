# Slice: intake-and-jev (token efficiency audit, 2026-11-09)

Scope: the Autopaper source/capture intake path (schedules, polls, captures, intake labels,
events layer, intake tasks, front importance, support/candidate judges) and every call site that
uses a model to decide, label, score, rank, dedupe or route. Read-only. Every number is tagged
[M] measured (command given) or [I] inferred / [H] hypothesis.

Paper = computer-1355b83b..., VM vm-1669020a.... Windows: Jev receipts 2026-11-09 13:00-19:18 UTC
(6.3 h); paper runs/costs 17:00-19:18; sources recent-polls since 13:00.

## 0. Headline

- The intake chain costs about 5.1-5.4M input tokens an hour, 10-11% of the paper's 49.5M/h:
  2.07M/h of it is Jev (cheap in dollars, about $0.09/h) and 3.0-3.3M/h is the full model
  (management_task intake runs). [M, sections 1.1 and 1.2]
- Jev as a whole is 2.53M tokens/h and 501 calls/h, $0.106/h (5.1% of paper tokens, essentially free in
  dollars). The two largest Jev spenders are per-capture intake labels (66%) and the events layer (16%).
  [M]
- The events layer is a shadow: nothing in production reads its output (intake.event_wakes is off), and
  its corroboration signal does not predict what the paper publishes (1 of ~70 opened stories came
  from a corroborated event). It is 0.4M tokens/h of pure waste. [M]
- The full-model intake task is a read-heavy agent loop for what is mostly a decision Jev has already
  made: 67% of its cells are reads (338 of 501), 6.6 read cells before the first act, 10.4 calls and
  78k input tokens per run. Where Jev says "same event as story X", the model fed that story in
  24 of 24 cases. [M, 50-run sample]
- /api/costs misses most Jev spend: it shows 266 calls / 0.53M tokens where the gateway receipts show
  640 calls / 3.55M tokens for the same 90 minutes (labels and events-layer calls have no run to
  attribute to). [M]

## 1. What this slice spends

### 1.1 Jev (all call sites), measured from the gateway receipts

Source: `/var/lib/go-choir/gateway/jev-transport-receipts.jsonl` on Node B (158k lines since
2026-09-30, 207 MB; one line per returned distribution with the request digest, computer id and the
usage block). Read by a one-off aggregation script run on node-b over `sudo python3` that printed only
counts and sums (no distributions, no request bodies). Call-site classification is by the answer key
names in the response (e.g. `event`+`topic` = intake label; `k<N>` = events layer; `p<N>` = support;
`c<N>.answered` = sub-call judge; `p<N>.contradictable` = inert judge; `novelty`/`role-N` = System 1
story analysis; host computer id = front importance).

Window 13:00:04-19:18:19 UTC (6.30 h), 3157 calls, 15.92M input tokens, $0.67:

| call site | file:line (origin/main) | calls | calls/h | input tok | tok/h | tok/call | share |
|---|---|---|---|---|---|---|---|
| intake labels (`event`,`topic`), one call per new capture | agentcore/intake_labels.go:343,390 (via callJudgeWithin) | 1147 | 182 | 10.48M | 1.664M | 9140 | 66% |
| events layer (`k1..k12`), chunks of up to 12 captures | agentcore/event_layer.go:387 | 423 | 67 | 2.55M | 0.405M | 6034 | 16% |
| support judge (`p<N>`), per Texture revision's citation pairs | agentcore/support_judge.go:288 | 792 | 126 | 1.83M | 0.291M | 2317 | 11.5% |
| front importance (`importance`,`promo`,`section`) | frontimportance/scorer.go:407 via gateway host route | 305 | 48 | 0.39M | 0.062M | 1277 | 2.4% |
| System 1 story analysis (`novelty`,`role-N`,`type-N`) | agentcore/story_analysis.go via system1_verb.go:274 | 201 | 32 | 0.34M | 0.054M | 1684 | 2.1% |
| sub-call settler (`cN.answered`,`cN.qN`) | agentcore/subcall_judge.go:566,635; desk_cast_judge.go:481 | 192 | 30 | 0.25M | 0.040M | 1314 | 1.6% |
| inert judge (`pN.contradictable`) | agentcore/inert_judge.go:173 | 56 | 9 | 0.036M | 0.006M | 641 | 0.2% |
| disclosure, prompt route, ad hoc System 1 questions | disclosure_gate.go:560, prompt_router.go | ~40 | 6 | ~0.03M | 0.005M | <1500 | 0.2% |

Per hour: 501 calls, 2.53M input tokens, 0.12M output tokens (output is free). Hourly series is flat
(400-580 calls/h, 1.5-2.8M tokens/h) since 2026-11-08 15:00. Jev dollar cost is $0.10-0.12/h; the
dollar cost of the full model is $0 (crouton-alpha is priced 0 in /api/costs) so the constraint is
concurrency and quota, not money.

Fraction of Jev calls that are exact duplicates (same request digest): 2919 of 158158 over all time
(1.8%); front importance 59 of 305 (19%) in the window. Intake and events: 0 duplicates.

Commands:

    scp agg.py node-b:/tmp/ && ssh node-b "sudo python3 /tmp/agg.py"   # reads the jsonl, prints aggregates only
    ssh node-b "sudo wc -l /var/lib/go-choir/gateway/jev-transport-receipts.jsonl"

Call-by-call composition of the 9.1k-token intake label call [I from code, intake_labels.go:244-340;
not separately measured]: up to 30 recent stories + 10 overlap stories (headline, updated_at, lede),
two questions each with one criteria string per candidate (~40-50 choices x ~25 tokens x 2 questions
= ~2.5k), siblings (up to 11 other captures of the poll, 200-rune summaries), 12 correction examples
(~0.7k), the capture (600-rune summary), and a ~150-token task text. The same ~6k story block and
criteria are re-sent for every capture of every poll.

### 1.2 The full-model intake task (management_task, kind intake)

Source: `paper.sh /api/runs?limit=500` for the role/state (90 management_task runs created
17:01-19:18, i.e. 39 runs/h); `/api/runs/{id}/events` for 50 completed runs (loop.progress and
tool.result events); `/api/costs?detail=1` entries for token sums. Files in eff/mt/*.json.

- 90 runs in 2.28 h; 219 captures leased (96/h); 78 completed, 8 cancelled (211k tokens, mostly 18:39-18:40 when providers ran out of quota), 2 failed, 2 pending. Leased per run: 1 capture in 44 runs, 2 in 19, 3 in 11, 12 in 3.
- 50 sampled completed runs: 520 calls (10.4/run), 3.89M input tokens (77.8k/run), first call 3.0k
  tokens (system prompt 11,986 chars + tool + user turn), last call 10.6k. Output about 0.8k tokens/run.
  Whole-population figure from /api/costs: 78 completed runs 3.17M tokens / 430 calls (40.7k per
  run, 5.5 calls: costs window covers only 17:46 onward so partial runs are cut); the earlier
  evidence doc gives 6.6M tokens for 79 runs in 2 h = 3.3M/h. I use 3.0-3.3M/h.
- Per-capture: 77.8k per run / ~1.7 captures per labelled run in the sample = ~45k tokens per capture.
- Cell mix in the 50 runs: Stories 109 (2.2 per run), Records 96, ReadCapture 80, Help 47,
  CellOut 33 (all reads = 338 cells) versus Cast 81 (open or feed-with-hypotheses), Feed 39,
  Decline 9, Done 50 (acts). Mean 6.6 read cells before the first act (distribution: 6 reads in 9
  runs, 5 in 8, 7 in 7, 9 in 5, and up to 17). `choir.Stories(50, "")` returns about 2.8k tokens that
  ride in every later call of the run.
- A traced 1-capture run (df6fda10): 7 calls, 38.8k input tokens, 365 output tokens: Records, ReadCapture,
  Stories(50), CellOut x2 (searching the capture text), Feed, Done. The capture's label said "new
  (p 0.96)" and the model fed it to an existing story anyway after reading the capture; that is a
  Jev miss the model caught (see 2.4).

### 1.3 Volume entering the funnel

- 66 active schedules (12 official, 26 rss, 20 bsky, 7 mastodon, 1 arxiv); 180 scheduled polls/h
  (170 observed). `paper.sh /api/autopaper/sources/schedules`.
- New captures since 13:00: 1183 in 6.3 h = 188/h (mastodon 47%, rss 26%, bsky 24%, official 1%);
  matches 182 label calls/h. 63 polls/h produce at least one new capture (so about 63 arrivals/h);
  34 of 397 such polls hit the 12-item cap. Lifetime totals: 93,893 polls, 2.53M fetched items,
  77,239 new, 638 MB of item bytes.
- Settings on the paper: intake.labels true, intake.events true, intake.event_wakes false
  (`paper.sh /api/computer-settings`). So both per-capture labels and the events layer run, and
  arrivals wake intake tasks directly (not held for events).

## 2. Waste found

### 2.1 The events layer is a shadow that duplicates the labels (0.405M Jev tokens/h, 100% unused)

- Code: event_layer.go header says V1 is "a shadow: nothing reads the events except the event view;
  production still wakes per arrival until V3". Consumers on origin/main: `HandleWorldEvents`
  (event_api.go:18, read-only) and event_wakes.go (inactive: intake.event_wakes=false).
  `grep eventIndexFor|eventSummary` finds no other reader. [M code]
- Spend: 423 calls, 2.55M tokens in 6.3 h, 1170 captures, 2.77 captures per call (chunk size is 12
  but a 60 s flush interval with ~3 captures/min fills few slots), 2181 tokens per capture. [M]
- Both layers classify the same captures against overlapping candidate sets with the same Jev
  question ("same real-world event as ..."). Labels: against stories (c1..) and siblings. Events:
  against events (e1..) and chunk siblings. They share the one `intake_labels` rate bucket. [M code]
- The signal does not predict publication. `paper.sh /api/autopaper/events?limit=300` (newest
  300 events, 17:27-19:17): 295 uncorroborated, 278 single-member; 204 of 300 have 0 counted origins
  (social posts do not count), 146 are mastodon-only, 69 bsky-only, 72 rss-only. Joining 75 opened
  stories from the sampled intake runs to their capture's event: 1 came from a corroborated event,
  ~70 from single-origin events, and of those 32 are on the front and 35 are not. So gating
  production on corroboration (event_wakes) would cut story opens by ~97% while the paper publishes
  about half of its single-origin opens by finding the second outlet in research. [M, n=75,
  title-match for "on the front", noisy]
- The index load is bounded at 10,000 events and `/api/autopaper/events?limit=300` took >120 s on the
  first try (a lock on the index while it loads), then 0.35 s; a read can stall behind the load. [M, anecdote]

### 2.2 Per-capture labels resend a 6k-token story block per capture (1.66M tokens/h, ~6x avoidable)

- 1147 calls for 1147 captures, 9140 tokens each, 0 duplicate digests. The events layer proves the
  batch shape works with this Jev (same task, 12 captures per call within 24 KB; 2181 tokens/capture
  at its current 2.8 captures/call). Extrapolating the events-layer shape at 12 captures per call
  with the larger story block: (~6k shared + 12 x ~1k per question) = ~18k per call = 1.5k per
  capture, 6x less than 9.1k. [I, arithmetic from the code and the 6.0k/2.8 measurement]
- Unlabelled captures: 97 of 1980 (4.9%) in the schedules' recent polls since 2026-11-08 19:20; the
  per-call timeout is 5 s and the poll budget 15 s (intake_labels.go:60-62), hypothesis that
  9k-token calls time out under concurrency 4. [M count, H cause]

### 2.3 The intake task re-reads what Jev and the host already know

- 67% of cells are reads, six of them before the first act (1.2). The first call is 3.0k tokens; the
  last is 10.6k because every read result rides in the history. Records output alone (the arrival record
  with labels) is 2.6k characters for a one-capture run; `Stories(50)` is ~2.8k tokens; the model then
  asks `Help` (47 calls, ~1 per run) for verb signatures that the prompt could carry.
- The model's disposition equals Jev's label where Jev names a story: 24 of 24 same-story labels
  were fed (p>=0.9: 15, 0.7-0.9: 5, <0.7: 4). [M, 87 labelled captures in 50 runs; disposition parsed
  from the cell source text, so approximate]
- Whole runs that are pure same-story feeds: 8 of 50 runs (16%), 355k of 3.89M tokens (9.1%) spent
  to feed already-decided captures. [M]
- Jev "new" labels vs the model: p>=0.9: 31 opened, 5 fed, 4 declined (78% agree); p 0.7-0.9: 6 open,
  4 decline; p<0.7: 8 open, 4 feed, 1 decline. Taking the model as truth, Jev's recall on "joins an
  existing story" is 24/(24+9) = 73%, and every join it misses becomes either a duplicate story or a
  model catch. [M, n=87]

### 2.4 Intake work for captures that never publish

Funnel on the paper (all [M] unless marked):

| stage | per hour | source |
|---|---|---|
| new captures | 188 | schedules `recent` since 13:00 |
| Jev label calls | 182 | receipts |
| captures leased to intake tasks | 96 | 219 leased / 2.28 h |
| intake dispositions (87 labelled in the 50-run sample) | feed 38%, open 49% (43+2), decline 10% | tool.result sources |
| story documents created | ~50-54 | `paper.sh /api/texture/documents?limit=1000`, 377 created 11:00-18:00 |
| stories on the public front updated | ~31 | public front endpoint, 400 stories 06:28-19:17 |

- At most 31 of the ~54 docs created per hour appear on the front, and some fronts are updates of
  older stories, so at least 43% of opened stories do not publish within hours. Direct match: of 377
  docs created 11:00-18:00 only 197 (52%) have a title on the front; 110 of the 180 that do not are
  still at version 1 (opened, never got a first Texture revision), 14 at version 0. Title matching is a
  lower bound on publication (titles can change), so the never-publish rate is roughly 25-48%.
- Research runs completed since 17:50 (37): 27 on a published doc (468k input tokens each), 10 on a
  doc not (yet) on the front (402k each). So 27% of research runs (about 4.0M tokens in 1.5 h =
  2.7M/h = 5.4% of the paper) are spent on stories that are not published. Lower bound for "never"
  (some are in flight). The cost per unpublished story is nearly the same as per published one. [M, n=37]
- All 9 declines in the sample were mastodon captures (27% of mastodon captures declined, 0 of
  bsky); publication by source class of opened stories (n=74, noisy): rss 6/16, bsky 11/25,
  mastodon 10/18, unknown 7/15 on the front. Source class is not a strong predictor in this sample.
- The intake tokens themselves spent on captures that are later declined or whose stories never
  publish: declines ~10% of captures x 45k = ~0.4M/h, opens that never publish ~25-48% of 47 opens/h at
  45k intake tokens = 0.5-1.0M/h. But the large number is downstream: each opened story costs ~1.2M
  tokens overall (previous evidence doc) and the never-publish fraction of that is the 2.7M/h of
  research above plus the Texture runs on those stories.
- Captures that never reach a task: 188 new/h vs 96 leased/h. 63 arrivals/h but only ~39 intake runs/h
  start. I cannot account for the 92 captures/h gap: candidates are `captureDelegable` false
  (already disposed), tasks still queued (observability shows 13 queued, oldest 9.9 h, 4 slots, 1
  running), and the queue's 24 h expiry (management_tasks.go:167, LIFO newest-first). Probe in section 5.

### 2.5 Smaller items

- The cost ledger undercounts Jev by ~85% of tokens (above): the labels and events-layer calls have no run id.
  Any spend decision made from /api/costs underweights this slice. [M]
- Front importance rescored 59 identical requests in the window (19% of its calls); trivial (0.012M/h).
- The Jev route is protected by one gateway-wide failure hold (`upstreamHold`, gateway/upstream_hold.go): one
  429/5xx/timeout stops every caller, every computer, for 1 minute, doubling to 15 minutes. Raising
  fan-out can therefore take the support judge down with it. No hold events appeared in the 3 h
  gateway journal. [M code, M journal]
- Receipts are written with an fsync per call and never rotated (207 MB). Not token cost.
- `pii/slm_redactor.go` (a 7B Ollama classifier) has no production caller. Not spend.

## 3. Ranked levers

Savings are of today's volumes. "Slice" = the intake chain (5.2M/h); "paper" = 49.5M/h.

| # | lever | saving | slice | paper | quality risk | size | evidence |
|---|---|---|---|---|---|---|---|
| 1 | Pre-hydrated single-pass intake task: the host puts the leased captures' excerpts, Jev's label and the top ~8 candidate story headlines in the task's first user turn, shows the act verbs inline (no `Help`), so a task is one act cell plus Done (2-3 calls) | 55-75% of intake task tokens = 1.7-2.3M/h | 33-44% | 3.5-4.7% | low (same information, the model still decides) | M (frame builder + overlay prompt) | 1.2, 2.3: 6.6 read cells, 10.4 calls, 78k tokens per run |
| 2 | Merge labels and events into one batched Jev call per flush (up to 12 captures, <=24 KB), and delete the shadow events layer's separate call | 1.7M Jev tok/h (2.07M -> ~0.4M) | 32% | 3.4% (tokens; $0.07/h) | low-medium (Jev context limit; sibling matching across chunks) | M (event_layer.go already has eventQuestion; labels' story alias logic) | 2.1, 2.2 |
| 2a | Smallest cut: stop the events layer now (no consumers) | 0.405M Jev tok/h | 8% | 0.8% | none today | S (delete) | 2.1 |
| 3 | Host feeds a capture to the story Jev names (any p, since 24/24 fed) without a model run; model only sees "new" and low-recall cases | 9% of intake tokens by whole-run skip, up to ~15% with partial; overlaps lever 1: after lever 1 it is ~0.2-0.4M/h | 4-8% | 0.4-0.8% | low for precision (24/24), but mis-feeds hide a duplicate-check that the model does for "new" misses only; measure with the override stream | M (a host-minted Feed under a system act; authority design) | 2.3 |
| 4 | Importance-ordered intake queue: score each arrival with a Jev publishability/importance question and start tasks highest-score-first instead of newest-first; skip below a floor | spend moves to better captures; if 1/3 of the 27% unpublished research could be avoided: ~0.9M/h research | n/a | ~1.8% | medium (needs a validated predictor; source class alone is not one) | M | 2.4; needs probe 4 |
| 5 | Open without hypotheses authored at intake (host opens from the label/headline; research writes its own hypotheses) so the model is not needed for 49% "open" dispositions at all | takes the remaining intake task loop to ~0 for opens: ~0.3-0.5M/h beyond lever 1 | 6-10% | 0.6-1% | medium: hypotheses with probabilities are scored (calibration); changes who authors them | M-L, and an open question (5) | 1.2, task overlay |
| 6 | Put Jev usage in the cost ledger / `/api/costs` | visibility only | | | none | S | 2.5 |
| 7 | De-duplicate front-importance rescoring; raise events flush window | trivial | | <0.1% | none | S | 2.5 |

Combined realistic (1 + 2 + a thin 3): intake chain 5.2M/h -> ~1.0-1.3M/h (a 4-5x cut), i.e. 3.9-4.2M/h
saved = 8% of the paper's 49.5M/h in raw tokens; in dollars the Jev share is nearly free so the real
value is the 2.3M/h of full-model tokens (quota and the 429-concurrency ceiling) = 4.6%.
A 10x leaner intake chain: Jev merged batch (0.4M/h) + a full-model "open" step only for
Jev-new captures, in a single non-agentic call (47 opens/h x ~5k = 0.24M/h) + a residual agent
loop for ~20 ambiguous captures/h x 15k (0.3M/h) = ~0.95M/h, 5.5x. 10x or better needs lever 5 (no
model on opens) which pushes the chain to ~0.5M/h. 50x on this slice is not reachable without the
owner dropping the open-with-hypotheses contract; the hard floor is the Jev batch (~0.3-0.4M/h at
188 captures/h x ~1.5-2k) unless candidates are pre-filtered (see parameters below).

## 4. Jev / classifier opportunities in this slice

### 4.1 What is already a Jev call (inventory by file, origin/main)

| decision | call site | batching today | scope/bucket |
|---|---|---|---|
| same event / same topic for a new capture | intake_labels.go labelOneCapture | none: 1 call per capture, 2 questions; concurrency 4, 5 s call, 15 s poll budget | `intake_labels` |
| same event across the poll queue | event_layer.go resolveEventChunk | 12 captures per call; chunks sequential per computer (events minted in a chunk become candidates in the next); flush every 60 s | `intake_labels` (shares the bucket) |
| does the cited passage support the sentence | support_judge.go judgePendingSupport -> askJevForSupport | all unjudged pairs of a revision in one call (avg 5.2 pairs/call), runtime-wide concurrency 2, queue wait 2 min, 90 s call | default `judgments` |
| sub-call answered / p_success | subcall_judge.go, desk_cast_judge.go | one call per fan-out / per round | default |
| inert (contradictable) | inert_judge.go | batch of records | default |
| research closed-choice questions | system1_verb.go:274; story_analysis.go (2 calls of 12) | question batches | default |
| front-page importance, promo, section | frontimportance/scorer.go:407 | 1 story per call, 3 questions, 80 calls max per 10-minute run | host route (no scope) |
| disclosure claims; prompt routing | disclosure_gate.go:560; prompt_router.go | single | default |

### 4.2 What is still a full-model decision (the candidates)

1. Intake disposition (feed / open / decline): 3.0-3.3M tokens/h. Feed is a choice (Jev already
   chooses); decline is a choice; open needs a headline and hypotheses (generation).
2. Compaction through the thread call (toolregistry/thread_call.go): generation, not Jev.
3. Management, Texture, research, engineering loops: generation / tool use. Out of scope for Jev.
   The one classifier-shaped decision inside research ("is it corroborated, stop?") is not separable
   in traces (previous evidence doc).
4. Nothing else in this slice: search ranking is non-LLM; capture dedupe is by URL/content hash in
   the poll (`repeated`, `skipped` counters); entity extraction is entitytext (regex/lexicon).

### 4.3 Proposed batch shapes

Shape A, intake-match (replaces labels and the events call): one call per flush, flush when the poll
queue has 12 captures or 3-4 minutes after the first queued (at 3.1 captures/min that fills a call; the
4 slots already leave tasks queued for minutes, so the wait is hidden).

- state: `task`, `stories` (<=30 recent within 72 h, headline only; ledes only for the top ~8 by
  title-token overlap with the batch; plus <=10 overlap stories), `captures` k1..kN with title,
  300-rune summary, outlet, language.
- questions, one `choice` per capture: [c1..cM stories | k_j siblings | new], plus one `choice`
  per capture `kind` in {news event, listing/feed, social chatter, opinion/ad}. Optional third
  `score` per capture: publishability 0-1 for lever 4 (see probe 4).
- size bound: `eventStateMaxBytes` 24 KB (Jev slows above ~32 KB). Per-question criteria are the bulk
  of the bytes: 40 candidates x 2 questions x ~25 tokens. Reduce candidates per capture to the
  top-8 by lexical overlap plus the 10 newest (not 30), which also cuts question text ~3x.
  Probe 2: whether `criteria` is optional.
- parallelism: chunks of one flush are sequential if sibling dedupe across chunks matters (as now);
  independent flushes (different computers) run freely. For replay/backfill (re-labelling a stale queue)
  fan out 4 concurrent calls. Do not exceed that: the gateway holds all of Jev for 1-15 min after
  one failure (upstreamHold) and a hold stops the support judge too.
- rate scope: keep `intake_labels` for shape A; add scopes `intake_publishability` (lever 4) and
  `intake_replay` (backfills) in gateway/jev.go judgmentRateScope so a drain cannot starve the support
  judge. The default per-bucket limit is 60 requests/min (ratelimit.go:12), so a 12-capture batch
  allows 720 captures/min per computer, ~230x today's 188/h.
- expected volume: 63 flushes/h at one per arrival is the ceiling; with a 3-minute window 20 flushes/h x
  ~16k = ~0.33M tokens/h.
- accuracy risk: Jev's decision quality drops with long states (its documented slow mode above ~32 KB);
  the events layer's false merges at p 0.85-0.92 (docs/problems/event-layer-false-merges-at-marginal-p)
  show that join precision needs p>=0.92. Keep the existing thresholds and the "correction examples"
  in the state (12 examples ~0.7k tokens per call, now once per batch instead of per capture).
  Regression check: replay the 87-capture sample plus the stored label/override pairs
  (store `choir.capture_label_override`) through shape A and compare dispositions.

Shape B, host-side disposition (replaces ~1/3 of intake tasks):

- Rule on the merged answer: event=story c_j -> host Feed (0 model tokens; 24/24 in sample);
  kind in {listing, chatter} with p>=0.9 -> host Decline with the Jev reason; new -> intake task.
- The task's first turn carries, per capture, the excerpt (800 chars), the label, the top candidate
  story headlines and the verb signatures, so the model's work is a single cell (lever 1).

Shape C, queue ranking (lever 4): a `score`/`choice` question per arrival at flush time
("is this a reportable event that two independent outlets will cover?"), start tasks in score order
and expire below-floor items at 6 h instead of 24 h. Cost: one extra question per capture in shape A
(~50 tokens), no extra call.

## 5. Open questions and probes

1. Why 188 new captures/h but only 96 leased/h? Probe: count tasks created, started, `expired_in_queue`
   and `captureDelegable=false` skips per hour from the store (needs a store query or a counter; GET
   `paper.sh /api/runtime/observability` only shows queue depth: 13 queued, oldest 9.9 h). If the
   gap is expiries, the labels and event calls for those captures are fully wasted (~0.8M Jev tok/h).
2. Are `criteria` required by the Jev Decisions endpoint? `validateJudgmentRequest` (gateway/jev.go)
   only checks the question is a JSON object of type choice/score/noul, so criteria look optional
   locally; the upstream may not. A ~$0.0001 call with choices and no criteria answers it, and the
   answer decides whether batches can be 3x smaller. Not run (no paid calls).
3. Jev latency vs state size. No latency is logged for /provider/v1/judgments. 4.9% of captures
   end unlabelled under a 5 s call timeout; a timing log in HandleJudgment (or the receipt's created_at
   minus request time) would show the p50/p95 by token count.
4. Can Jev predict publication? Probe: take the ~377 docs created 11:00-18:00 with their opening
   captures and published/not-published outcome (docs.json x front), ask Jev one `score` question
   on the capture title and summary, compute AUC. 377 calls at ~1k tokens each is ~$0.016. This sizes
   lever 4 (and the 2.7M tokens/h of research on stories that do not publish).
5. Does `choir.Cast("texture:new", ...)` require hypotheses? If not, lever 5 is a deletion of the
   hypothesis authoring from intake; if yes, a Jev-assisted open needs a one-call generator.
6. Per-story total tokens for published vs unpublished stories. /api/costs lists only ~1.5 h
   (10,000 events); a join of costs entries by trajectory over a day needs the gateway journal
   keyed by run (not logged) or the tape. The 402k vs 468k research tokens per run above is the best
   current number.
7. Jev's join recall of 73% (24 of 33 joins): which candidate was missing from the shortlist? The
   9 false-new captures were fed to stories that may be older than 72 h or beyond the 30+10
   candidates. Probe: for those 9, compare the fed story's `updated_at` with the shortlist horizon.
8. Do feeds (38% of captures) each wake Texture? Texture is woken 2.3 times per doc in 2 h
   (evidence doc); if every feed is a wake, coalescing feeds per story per window is a Texture-slice
   lever worth a trace (belongs to the texture slice).

## Commands used (all read-only)

    git -C /Users/wiz/Dev/go-choir fetch -q origin ; git show origin/main:<path>
    ssh node-b "sudo journalctl -u go-choir-gateway --since '-3h' --no-pager" > gw3h.log
    ssh node-b "sudo python3 /tmp/agg.py"   # jev-transport-receipts.jsonl aggregate; script removed after
    paper.sh /api/autopaper/sources/schedules ; /api/computer-settings ; /api/runtime/observability
    paper.sh "/api/autopaper/events?limit=300" ; "/api/texture/documents?limit=1000"
    paper.sh /api/runs/<id>/events    # 50 completed management_task runs, saved in eff/mt/
    curl -s "https://choir.news/api/platform/publications/front?limit=100"   # paged by cursor
    existing: eff/m/paper_costs.json (costs detail), eff/rt/runs.json (500 runs)
