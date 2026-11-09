# Slice: research desk tool loop (paper, 2026-11-09)

Read-only. Data: 83 research runs on the paper (`computer-1355b83bced1613eb048ee28d66688a8`)
created 17:00:14 to 19:16:02 UTC (the 500 most recent runs the list returns), with every
run's full event stream. Raw files: `eff/research-data/` (runs.json, ev/<run>.json, an*.py,
summ.json, front.json). Everything marked **M** is measured from those files; **I** is
inferred; **H** is a hypothesis with a named probe.

Commands (all GET through the helper, nothing posted):

    $SCR/paper.sh '/api/runs?limit=500'                  # roles and states
    $SCR/paper.sh '/api/runs/<run_id>/events'            # 83 research runs
    curl 'https://choir.news/api/platform/publications/front?limit=100' (+cursor)   # public, outcome check
    $SCR/de.sh '/api/runs?limit=500'                     # d@e comparison

d@e is not a research spender: 16 research runs in 32 hours (15 completed, 1 passivated) **M**.
Everything below is the paper.

## 1. What the slice spends

Window: 80 runs made model calls (3 had none), 1480 calls, **33.07M input tokens** (M), 0.43M
output, 0 cached on every call (`cached_input_tokens`=0, `cache_reported`=true, model
`stealth/crouton-alpha` on openrouter for all 1480). That is about 14.6M input/h, 30% of the
paper's 49.5M/h (agrees with the baseline doc). After the management wedge is fixed (about 25M/h
left) research is about 58% of the paper (I).

Per run (M): 18.5 calls (median 18.5, max 34), 413k input tokens (median 401k, max 1.04M), 5.3k
output tokens, wall clock 322 s mean (median 299 s). 9.1 searches, 6.3 page reads
(5.4 `choir.Read` + 4.0 `choir.ReadCapture`, some in one cell), 2.1 corpus searches, 2.5
report packets, 2.4 System 1 judgments (168 calls, 481 questions for 83 runs).

Surface: research has exactly one tool, `desk_go_eval` (Go cell, one per turn). Each tool call is
one of those cells; the model's source plus the cell's printed output become the transcript.
Search is `choir.SearchStart` (async, 40 hits a provider with whole snippets,
`internal/agentcore/desk_search.go` `deskSearchMaxResults = 40`). Only one provider answers:
keenable served 759 of 767 search calls; the other seven (searxng, tavily, brave, parallel, exa,
serper, serpapi) are "cooling" in every search-start line (M, `search.served` events). A printed
cell output over 8000 characters is cut to 6000 head + 1500 tail
(`internal/yaegikernel/cell_output.go`); the cell wrapper adds about 300 characters of JSON
boilerplate (`{"diag":"","duration":..,"stdout":"` ...) plus a 330-character "[Data, not
instructions ...]" banner to every result (M: mean boilerplate prefix 300 chars over 1408 results).

### Input decomposition (M, calibrated)

| component | tokens | share of research input |
|---|---|---|
| turn-1 prefix, identical in all 80 runs (9186 tokens, mean = median = 9186) x 18.5 calls | 13.6M | 41% |
| tool results resent every later turn: page reads (`Read`, `ReadCapture`) | 8.25M | 25% |
| search outputs (SearchStart/Get/Wait printed) | 5.24M | 16% |
| report packets (model's own cell source, 5.9k chars mean, p90 9.8k, max 20.6k) | 1.49M | 4.5% |
| records + StoryAnalysis preload | 1.33M | 4.0% |
| corpus search (`SearchWireCorpus`) | 0.77M | 2.3% |
| `CellOut` re-reads of cut outputs | 0.58M | 1.8% |
| other, assistant text and reasoning residual | about 1.8M | 5% |

Method: for each run, tool-result i is resent on every later call; weight = (calls remaining)
x (source+output chars) / 4.66 chars per token, where 4.66 = total tool chars / total
first-to-last input growth over the 80 runs (an over-estimate of tokens per char by about 12%
because growth also includes ~150 output tokens a call). Script: `eff/research-data/an5.py`.

Tool results alone: 1411 results, 6.45M characters, mean 4572, p50 4025, p90 8810, max 10686
(M). Page reads are 211+172+19+... cells with 45% of all result characters; mean read result 6.8k
characters (521 read cells, 3.56M characters).

Growth per call is 1237 tokens mean (p50 1086, p90 2469) (M). The cost is quadratic in calls:
input(N) is about 9186 N + 1237 N (N-1)/2, which gives 370k at N=18.5 (M: 413k observed).

What a run does (typical 19-call, 355k-token run, `27b31c8f`): call 1 reads `choir.Records()`
and `StoryAnalysis` (nothing else); calls 2 to 10 are single searches and single reads, with 50 to
190 output tokens each; a first interim packet at call 11; four more single reads; a second packet
at 16; one more read; the final packet plus `Relate` at 18. Most calls are micro-steps of 100
output tokens that each resend 10 to 28k.

Tasks are narrow verifications: every objective I could read (10 of 83; the first cell prints
differently in the rest) is "corroborate / verify with a second independent outlet / primary record"
(M for 10, I for the rest).

## 2. Waste found (with evidence)

W1. **No prompt caching, on a stable prefix (M).** 33.07M input, 0 cached. 41% of input is a
prefix that is byte-identical across all runs and the other 54% is an append-only transcript.
With a working prefix cache the fresh part of a run would be about 9.2k (first call, itself a
cross-run hit) + 17 x 1.24k = 30k of 413k (7%). The 2026-10-26 owner ruling that removed research
trimming ("Since we instituted caching everywhere...", `docs/problems/research-economy-switches-removed-2026-10-26.md`)
rested on caching; the routing change that put crouton-alpha under research removed the premise,
and 85 to 87% hit rates remain only on synthetic and opencode-go (baseline doc).

W2. **Micro-turn structure.** 18.5 calls per run with a median cell source of 448 characters
and median output of 100 tokens (M). Each call pays the whole transcript. The runtime already has
what a batch needs (SearchStart is async, `choir.Searches`, goroutines in a cell) but the loop
rules force "one `desk_go_eval` per turn" and the prompt never shows a search-wait-read-digest
recipe. Half the iterations are single `choir.Read` or `choir.ReadCapture` calls (see call list
above) that could be one cell reading five URLs.

W3. **Reads dump page text the run never uses (M and I).** Read/ReadCapture results are 25% of
research input. Text quoted in report cells totals 287k characters (counting quotes repeated in
later interim packets, so an upper bound) against 3.56M characters read: **at most 8% of read text
is ever quoted** (M). The model must see candidate blocks to quote them, but not 12000-character
windows (`choir.ReadCapture(id, 0, 12000)` in the sample run).

W4. **Search outputs print whole result lists.** 40 hits per call, snippets whole; search cells
average 6.1k characters (searchstart+searchread cells, M). They are 16% of input. One provider
(keenable) answers all of them, so "40 hits per provider on every provider at once" is in
practice 40 hits.

W5. **Cells that fail and are re-sent: 131 of 1411 (9.3%).** The next call after each pays the
full transcript: **2.54M tokens, 7.7% of research input** (M: sum of the input tokens of the call
that follows each error result). Causes (M): 39 `reflect.Value.Slice: string slice index out of
bounds` (the model slices a string, e.g. `out[:8000]`, past its length), 8 `reflect.Value.Field
on zero Value` (nil read results), the rest compile errors on field names the model guesses
(`r.Ok`, `r.CaptureID`, `r.Json`, `sN.Id`, assignment mismatch on `choir.Records()` and
`choir.Context()`). Also 9 stale `CellOut` handles from an ended session and 36 `SearchWait`
calls on search ids that this session never started ("no search s5; this session's searches: s1..s4")
(M). These are API-discoverability defects, not model errors.

W6. **Repeated queries are mostly re-sends after a failure, not repeats of work.** 687 queries,
114 exact repeats of an earlier query text in the same run, 85 more near-duplicates (token Jaccard
>= 0.6) (M, regex count; script `an2.py`). But only **10 cells** re-issue only queries already
issued and do not follow an errored cell, costing 0.6% of input (M, `an3`-style script above).
So the baseline doc's duplicate-query-suppression lever (run `52fb37c2`) is small: most of the
repeats come after W5. Cross-run repetition is nil in this window: 79 channels, 4 with more than
one research run; 0 of 284 distinct URLs read were read in two runs; 1 of 545 distinct queries
appears in two runs (M). A search/read cache across runs would hit almost nothing here.

W7. **Own-corpus published-story search is broken on the paper (M).** 83 of 104 completed
`SearchWireCorpus` calls (80%) return `published pieces not searched: published search: capture
index refused: corpusd returned 403: {"error":"internal caller required"}` (45 of 55 at
17:xx, 20 of 27 at 18:xx, 18 of 22 at 19:xx). The events and captures halves of the same
result work. Candidate cause (H): the `HandleEdges`/object routes now refuse a guest caller
(`internal/platform/objectgraph_handlers.go`, "no guest lists edges: host callers only", and the
2026-11-09 change "guests write no objects or edges through the generic graph routes"); probe:
call the capture-index route from a guest with the guest token on d@e and read the status.
Cost: a doctrine step ("search the computer's own corpus first") returns nothing for the
published half, so published-story dedupe and `RelateStories` candidates are lost. Small in tokens,
a functional defect.

W8. **Runs that end without a result: 12 of 83 (M).** 6 failed + 1 blocked with "every model is
out of quota after N retry rounds over 18m" (iterations 11, 14, 19, 26, 28, 52), 6 passivated
runs. These runs spent 2.79M input tokens (8.4% of research; failed 1.64M, passivated 1.15M).
Quota failures arrive after 5 to 18 minutes of retry rounds at iteration 11 to 52, so the work
done (reads, partial packets) is paid and lost unless the Cast is redelivered (not verified).

W9. **Depth after the minimum is reached (M, with H on value).** First interim packet comes at call 8
(median 9.0 mean); the final packet at 9.7 calls later (66 of 67 completed runs have >= 2
packets). In 38 of 67 runs the first packet already cites >= 2 distinct hosts, which is the
publish rule's floor; the final packets cite 4.4 hosts on average versus 2.0 in the first
(M, host counted from `target.uri`). Those 9.7 calls are about half the run's input. Whether the
extra outlets raise quality (published stories carry 2 to 8 sources, mode 3; M from the front
list) is not measurable from these traces. That is a policy question, not waste by itself.

W10. **Outcome per token.** Of the 58 runs where the story's publication id could be recovered,
43 (74%) are on the front page now and consumed 20.6M tokens (480k each, 20.4 calls); 15 are not
on the front list, consuming 5.1M (15% of research input), although 11 of those 15 sent a final
completed Report (M; a story may publish later, the list covers updates since 13:00 UTC only).
25 runs have no recoverable id (7.3M). Plain reading: about two thirds of tokens produced a
published story within the observation window; one fifth did not (lower bound, not a conversion rate).

W11. **First cell is a pure read.** Call 1 does nothing except print `Records()`,
`Context()` and `StoryAnalysis()` (2.4k characters of output). The same content could be in the
first user message; that removes one full call and the 9.2k tokens it costs (2.2% of input),
and the loop prompts already tell the model to do this first. Small.

## 3. Ranked levers

Percentages of research input (R = 14.6M/h, 30% of today's 49.5M/h; about 58% after the
management wedge is fixed). They overlap; combined estimate follows.

| # | lever | saves R | saves paper (today / after wedge fix) | quality risk | size |
|---|---|---|---|---|---|
| L1 | Prefix caching for the model serving research (or route research to a caching provider) | about 93% of fresh tokens, raw unchanged | 27% / 54% of fresh | none | config or routing; experiment: 2 identical long requests, read `cached_in` |
| L2 | Batched research cell: `choir.Gather(cast)`-style helper or a prompt recipe: start all searches, wait, pick top k per distinct outlet, read in parallel, print a bounded digest; and allow the model to issue it in one cell | 55 to 65% (calls 18.5 -> about 8) | 17% / 35% | low to medium | medium (new verb plus prompt) |
| L3 | Extractive read: `choir.Read` returns the title, block count, and the 4 to 6 blocks best matching the Cast questions (lexical/BM25 or Jev scores), rest via `ReadCapture(from, max)` | 16% (alone), mostly inside L2 | 4.7% / 9% | low (quotes still come from whole blocks) | small to medium |
| L4 | Search digest: `choir.Searches.Digest(id, n)`: top n deduplicated by host, title plus 160 characters, instead of 40 whole snippets | 9% | 2.7% / 5% | low | small |
| L5 | Remove the error turns: safe slice helper (`choir.Cut(s, a, b)`), documented result field names/types in the overlay (or a `go doc`-style `choir.Help("Read")`), make `SearchWait` tolerate cross-session ids, return `(value, error)` shapes consistently | 7.7% | 2.3% / 4.5% | none | small |
| L6 | Stop rule at sufficiency: end with the final Report once the Cast's questions are answered and >= 2 distinct independent hosts are quoted, unless the Cast asks for more | up to 34% (38 of 67 runs, about 60% of their input), expect half | 5 to 10% / 10 to 20% | medium (fewer sources; H) | small (prompt plus `p_complete` threshold) |
| L7 | Admission by provider health: do not start a research run while every model is cooling/out of quota; park the Cast | up to 8.4% (W8), expect 3 to 5% | 1 to 1.5% / 2 to 3% | none | small |
| L8 | Preload `Records`/`StoryAnalysis` into the first message | 2.2% | 0.7% / 1.3% | none | small |
| L9 | Fix W7 (403 on published search) | tokens about 0, restores dedupe and related-story candidates | | none | small |

Combined, raw tokens (no caching): L2 + L3 + L4 + L5 + L8 plausibly take a run from 413k to about 90
to 110k (4x fewer), L6 on top to about 70 to 80k (5x to 6x). With L1 on top, fresh tokens fall to
about 10 to 15k a run (about 30x fewer than today's 413k of fresh). Do not stack arithmetic: L2
removes most of what L3, L4 and L5 save (fewer turns means fewer cells to fail).

Interaction with the 2026-10-26 owner ruling (append-only transcript for caching; no elision):
every lever above is compatible with it. L2 to L5 shrink what is appended, none rewrites earlier
history. Observation masking (baseline L3) does conflict with it and is not recommended while a
cache exists; it only pays when no cache is possible.

### What a 10x and a 50x research desk looks like

The Cast is a verification question with a known shape: one captured claim, "find a second
independent origin or the primary record". The current design hands that to an open-ended
18-turn loop.

10x (about 40k tokens a run, no new model): the L2 digest cell, L3/L4 bounded outputs, L5,
L6, and a first-message preload. Calls fall to 5 to 7: (1) frame in the first message; (2) one cell
that searches 3 to 5 queries built from the Cast, waits, reads the top three distinct-host hits in
parallel, prints per-page best blocks; (3) one cell to read more if the digest was thin; (4) the
report cell; (5) `Relate`. Fresh tokens are then about 30k and, with caching, 8 to 10k.

50x (about 8k tokens a run): make most of the loop a pipeline, with one language-model call at
the end.
1. The events layer already knows, per event, `outlets`, `origins`, `corroborated`
   (`internal/researchtools/corpus_events.go`). If the story's event is already corroborated (two
   origins) with capture ids, skip research and hand Texture the corroborating captures directly.
   How often that is true for Casts is not measured here (probe in section 5).
2. Else search with templated queries from the headline and Cast questions (no model).
3. Jev ranks the hits ("same event as this claim?", "primary record?", "same origin as outlet A, or
   independent?") in one parallel batch per search.
4. Fetch the top 2 to 3 distinct-host hits, Jev scores each block against the Cast questions,
   keep the top 5 blocks per page.
5. One model call (cacheable prefix 9k + about 4k of selected blocks + the packet) writes the
   report; `internal/quotematch` already verifies quotes exactly, so a hallucinated quote cannot
   pass.
Two model calls (frame+report, optional second pass) of about 10k each, about 20k, 20x. With the
cross-run cache on the 9k prefix: about 8k fresh. The loop stays as the fallback for Casts the
pipeline marks "thin" (fewer than two independent hosts after step 4), which is the only case that
needs open-ended search. Quality cannot be asserted from these traces: the loop's strength is
reformulating after a miss, and 59 of 67 runs found new hosts after the first packet.

## 4. Jev / classifier opportunities (research)

All three are decisions already or nearly shaped as choices; Jev costs $0.042/M input, output
free, and batches in parallel. Rates assume 36.6 runs an hour (M: 83 runs / 2.27 h).

| decision | today | per hour (M unless marked) | batch shape | tokens/h through Jev |
|---|---|---|---|---|
| Is this search hit about the same event as the claim, or a primary record, or unrelated? | the model reads 40 whole snippets per search | 333 searches x 39 hits = 13.0k hits | one judgments request per search: 39 items, question "same_event / later_development / background / unrelated", context = claim text | about 13.0k x 120 = 1.6M (I, 120 tokens per snippet+claim) |
| Which blocks of this page answer the Cast question? | the model reads 4 to 12k characters of the page | 230 reads x about 25 blocks (H, blocks per page not measured) = 5.8k blocks | one request per page, one item per block | about 5.8k x 150 = 0.9M (I) |
| Is outlet B independent of outlet A (own reporting, or the same wire copy)? | judged inside the loop, in prose | about 2 per run = 73 | one pair per question | under 0.1M |
| Is the Cast satisfied (answers every question)? `p_complete` is already stated by the model and scored independently | model | 36 | one per final packet | under 0.05M |

Total about 2.7M Jev input tokens an hour, about $0.11 an hour, against research's 14.6M
model input an hour. The saving is the model reading: items 1 and 2 are what L3/L4 would use, and
they replace roughly 40% of the input the model reads today (searches 16% + reads 25%, less the
bounded part that stays). Note the existing tools: `choir.System1` is the same family already
used by research (168 calls, 481 questions in 83 runs, about 2 a run), so the verb, the budgets
and "may be off on this computer" handling exist; what is missing is a bulk form that the runtime
(not the model) calls, so the 13k hit judgments are not 13k model-authored questions. System 1 calls
are bounded per run and per day (overlay text); a runtime-side bulk path needs its own budget.

## 5. Open questions and probes

1. **What fills the 9186-token first call?** Research `system_chars` is 17637 (gateway line) plus
   a 874-character prompt and a tiny tool schema, about 4.2 to 4.7k tokens at 4 to 4.7 characters a
   token, but the first call measures 9183 to 9186 in all 80 runs. Management_task's ratio is 4:1
   (11986 chars, 3.0k tokens). About 4.5k tokens are unexplained: a different tokenizer for crouton
   (H), a second prompt block not counted in `system_chars` (H), or a tool/verb listing injected as
   a message (H; `messages`=2 on call 1). Probe: dump the call-1 request body through the gateway
   debug path (`system_preview`/`last_user_text` exist in `internal/toolregistry/toolloop.go` near
   line 898) on d@e, not the paper. It matters because it is 41% of input and is a trimming or
   caching target.
2. **Does crouton-alpha / openrouter cache at all?** Two identical long requests through the
   gateway on d@e; read `cached_in` on the second. Decides L1 versus routing.
3. **How many Casts are already corroborated by the events layer?** Replay: for each research run,
   take the first `SearchWireCorpus` output and check for an event with `corroborated:true` and
   `origins >= 2` whose headline matches the Cast. Decides how much the 50x pipeline can skip.
4. **Do final-packet extra sources change publication?** For the 38 runs whose first packet had >= 2
   hosts, compare publish decision, section, importance and Texture rounds between stories with
   more and fewer cited hosts. Decides L6.
5. **Are Casts redelivered after a quota failure?** Read the Cast's conversation after a `failed`
   research run (`/api/runs/<id>/events`, then the next research run on the same channel). W8 says
   how much is wasted only if they are not.
6. **Blocks per page and per-block relevance rate** (sizes the Jev block batch).
7. **Is the 403 (W7) from the 2026-11-09 guest-graph change?** Probe named in W7.
8. **The 25 runs with no recoverable publication id** (7.3M tokens): a different first cell; the
   outcome per token for them is unknown.

## Top levers (10 lines)

1. L1 prefix caching: 41% of research input is a byte-identical 9,186-token prefix and the rest append-only; 0% cached on crouton. About 93% of research fresh tokens, 27% of today's paper (54% after the wedge fix). No quality risk; needs the cache experiment.
2. L2 batched search-wait-read digest cell: calls per run 18.5 -> about 8; 55 to 65% of research raw tokens, 17% of paper today (35% after wedge fix). Low to medium risk.
3. L6 stop at sufficiency (>= 2 distinct hosts and Cast answered): up to 34% of research, expect about half; 5 to 10% of paper. Medium quality risk (the extra 2.4 hosts per story are the cost).
4. L3 extractive reads (at most 8% of read text is ever quoted; reads are 25% of input): 16% of research. L4 search digest: 9%.
5. L5 error turns (131 of 1411 cells, 2.54M tokens): 7.7% of research from a safe slice helper and documented result fields. Zero risk, small change.
6. L7 admission by provider health (12 of 83 runs ended in quota failure or passivation, 2.79M tokens): 3 to 5% of research.
7. Defect W7: `SearchWireCorpus` published search returns 403 "internal caller required" on 80% of calls on the paper; breaks dedupe and related-story candidates; token cost small.
8. Correction to the baseline: pure repeated-query cells are 0.6% of input (10 cells), not a major lever; most repeats follow a failed cell. Cross-run caching hits nothing (0 of 284 URLs shared across runs).
9. Jev: per-hit relevance and per-block relevance batches, about 2.7M Jev tokens an hour (about $0.11/h) would replace about 40% of what the model reads.
10. 10x is available as a pipeline in this slice (about 40k tokens a run; 4 to 6x raw with L2 to L6, 30x fresh with caching); 50x needs the events layer to answer corroborated Casts and Jev-selected blocks feeding one model call (about 8k fresh). Quality of the 50x is untested.
