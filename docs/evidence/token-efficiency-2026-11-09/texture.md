# Token efficiency audit: the Texture slice (2026-11-09)

Everything here is read-only. Window: the paper's 77 most recent texture runs, created 18:40:32 to 19:18:17 UTC (38 min), run list via `paper.sh "/api/runs?limit=120"`, events for each via `paper.sh "/api/runs/<id>/events?limit=500"`, revisions via `/api/texture/documents/<doc>/revisions`, gateway via `ssh node-b "sudo journalctl -u go-choir-gateway --since '-3h'"`. Raw data: `eff/tx/` (runs.json, ev/*.json, rv/*.json, gw.log, pair.py). Source: `internal/agentcore/texture_frame.go`, `internal/texturewrite/frame.go`, `internal/textureprompts/`, `internal/textureowner/texture_controller.go`, `texture_turn_runtime.go`.

## 0. Correction to the budget doc

The budget doc (section 1.4) lists texture's system_chars as 37943 and 49908 and research as 17637. The trace says otherwise. Texture's `system_sha256` is stable (da75db21 on 231 of 236 calls) and I matched 167 texture loop.progress events to gateway request lines by exact (tokens_in, tokens_out) and messages count: 161 of them are `system_chars=17637`, only 3 are 49908, 2 are 37943 (measured, `eff/tx` matching script, inline). So the Texture system prompt is 17,637 chars, about 4.4k tokens (about 4.2k to 4.7k, inferred from the 4.6k to 4.7k first-call input of a fresh run whose frame is small), not 10k. 37943 is almost certainly research (prompt-research.txt is 37.8k chars) and 49908 management. The "78% of texture is fixed system prompt" figure in the budget doc is therefore about 30% (see 1.3); the larger share is something the budget doc did not isolate: inherited transcript (46%).

## 1. What the slice spends

### 1.1 Totals (measured)

| quantity | value | command/source |
|---|---|---|
| texture runs in 38 min | 77 on 46 documents (66 completed, 11 cancelled by a restart at 19:14 to 19:18) | runs.json |
| model calls | 236 (3.06 per run, median 2, max 12) | loop.progress phase=tool_loop |
| input tokens | 3.46M in 38 min = about 5.5M/h; output 63k (1.8% of input) | same |
| cross-check | `/api/costs` window 17:46 to 19:17: texture 8.52M input, 592 calls (14.4k mean) = 5.7M/h; cached 0 | eff/m/paper_costs.json |
| revisions created | 61 in 38 min = about 96/h (`texture.document_revision.created`); 16 of 77 runs wrote nothing | events |
| tokens per run / per revision | 45k per run, 57k per revision (3.46M / 77, / 61) | derived |
| share of paper | 13% in the budget doc's window (6.3M/h of about 50M); after the management wedge is fixed the same texture bill is about 25% of what is left | budget doc |
| d@e | texture 1.29M input over about 10 h (54 calls) = 3% of d@e; engineering (35M) and management dominate. Not a lever there. | eff/m/de_costs.json |

Cache: every texture call reports `cache_reported=true, cached_input_tokens=0` (crouton-alpha via openrouter, all 236).

### 1.2 Per wake: what a wake carries

Two kinds of run, split by the `messages` count on call 1 (measured):

| kind | runs | call-1 input (p50) | calls/run | tokens/run | total tokens |
|---|---|---|---|---|---|
| fresh (messages <= 4) | 17 | 4.7k | 3.1 | 15.9k | 0.27M (8%) |
| carried (messages > 4: 7 to 36, one 95) | 52 | 12.1k | 3.5 | 61.4k | 3.19M (92%) |

Why: the run metadata has `actor_rewarm_carry_conversation: true` and `actor_rewarm_source_loop_id`. A document's next wake reactivates its passivated run (`reactivatePassivatedTextureRun`, texture_controller.go ~line 696; passivation after `actor_park_idle_seconds` = 120) with the whole earlier conversation in memory. Verified chain: call 1 of the new run equals the source run's last input plus one new frame of about 2.4k to 5k tokens, e.g. doc c30e05 10.8k -> 22.8k, doc 348e6f 9.2k -> 24.8k, doc 94656e 13.3k -> 15.8k. The frame is injected by `textureFrameInjector` whenever revision or records changed; it holds the whole document as blocks (budget 12,000 chars before it digests; every document in the sample is under 4.4k chars so it never digests) plus every record since the cursor in full (report text, claims, hypotheses, sources with a 300-char excerpt, failures).

Decomposition of the 3.46M (derived; system taken as 4.4k per call, inherited = call-1 input minus 4.7k, multiplied by that run's calls):

| part | tokens | share |
|---|---|---|
| fixed system prompt x 236 calls | 1.04M | 30% |
| inherited prior-wake transcript, resent every call of a carried run | 1.59M | 46% |
| new this wake: frame (about 3.5k), capture reads (237 cell results, 890k chars total, p50 1.8k chars, p90 8.9k), write cells, growth within the run (about 1.0k to 1.3k per call) | about 0.83M | 24% |

So 76% of texture's tokens are a stable prefix resent. Output is 1.8%.

### 1.3 What a run does (measured, 237 cells in 77 runs)

Cell mix: ReadCapture 95, Write 76, Done/Wait 68, other 37, ReadDoc 18, Cast/Ask 18, Help 15, Records/Updates 5. A fresh run is almost always the same three calls: (1) `choir.ReadCapture("r1.c1",0,0)` or ReadDoc, 13 to 57 output tokens, input about 4.7k; (2) `fmt.Println(choir.Help("Cast"))`, 1.5k chars of help text, input about 5.2k; (3) Write plus Cast in one cell, input about 5.5k. Five fresh runs dumped to confirm (eff/tx/ev). 13 of 77 runs (17%) call Help at all. So about 70% of calls (about 168 of 236) are read-only cells that exist only because the frame withholds the source words ("a source shown without its words": `ReadCapture`) or the Cast packet shape is not in the prompt.

### 1.4 Revisions: edited or rewritten (measured)

25 consecutive appagent revision pairs over 8 documents (`/revisions`, line-level diff, eff/tx/rv): documents are tiny, 189 to 3,911 chars (a story is about 300 to 1,000 tokens), a revision keeps only 17% of the previous version's blocks unchanged, and 81% of its characters are new. Writes: 33 of 76 use `@all`, 43 use block directives; median Write cell source 2.3k chars. So in this regime a story is rewritten each wake and the document is not what costs tokens: a diff or patch revision format would save at most the Write cell source (about 0.6k tokens per revision) and the repeated copy in the next frame. That is under 2% of texture. Not a lever. (It would matter only if documents were long; the digested-frame path in `renderDoc` exists for that case and is never hit on the paper.)

### 1.5 Wake rate and update_dispositions

- 77 wakes on 46 documents in 38 min = 121/h; 1.7 wakes per document. Inter-wake gaps for a document: p10 281 s, median 474 s, p90 1706 s; none under 120 s, only 5 of 31 under 300 s. Each wake is a distinct research report arriving; there is no rapid-fire to coalesce in this window (contradicts the budget doc's L4 "coalesce wakes" at least for the window measured; the 2.3 wakes per document in the budget doc fits the 2-hour span).
- Request source: `inbox_record` 66, `update_coagent` 11; intent is `integrate_execution_findings` for all 77.
- update_dispositions (`textureTurnPendingInbound`, texture_turn_runtime.go): runtime default is "delivered" for every covered packet, the model names dispositions only for rejects. Cells that touch `Updates()` are 5 of 237. It costs essentially no tokens; it is bookkeeping, not a lever.
- Wakes that produced no revision: 16 of 77. Of those, 4 completed with an explicit `choir.Done("...duplicate of evidence already integrated; no document change")` and cost 170k tokens (4.9% of texture; 34.6k, 30.4k, 38.1k, 67.2k); 11 were cancelled by the 19:14 to 19:18 restart (211k tokens, 6.1%, transient, `fate: interrupted_by_restart`); one was a pure Cast.
- Loop.retry events: 50 (model_failover 15, provider_quota 14, quota_incident 12, quota_retry 7, rate_limit 2); retries cost no tokens at the gateway (failed requests are not billed) but delay.

## 2. Waste, with evidence

W1. Carry-conversation resend (46% of texture, 1.59M per 38 min). A fresh run on a comparable document costs 15.9k; the carried median costs 61k. Confound to state: carried runs are on documents that already have history, so some of the gap is document size. Evidence against a document-size explanation: the documents are all under 4.4k chars, and the growth between wakes is the transcript (earlier capture reads of 2k to 9k chars each, earlier Write cells that contain the whole story, earlier frames that contain the whole story), visible as messages 7 -> 12 -> 23 -> 36 for one document. The frame already re-states the document and every new record, so the older frames are stale duplicates inside the same transcript. Deepest chain seen: 95 messages, 68k input on call 1 (run 91c51d96).

W2. The fixed prompt is bigger than the job (30% of texture, 1.04M). 17,637 chars; 7,356 chars (42%) is the generated "REPL interaction surface": 15 verbs of which Texture uses ReadCapture, ReadDoc, Write, Cast, Done, Wait, Reject. ListDir, ReadFile, Pack, Context, CellOut, Ask, Records, Updates, Help are listed in full; the p_success/weight/settlement paragraph (about 600 chars) is printed twice (Ask and Cast); the Write verb restates the body's Write paragraph; Done, Wait, Reject restate the "Close each turn" list. Another 1.1k chars is the generic multi-desk preamble ("Choir is a multiagent ...", conductor/research routing) that Texture does not act on. Estimated reducible to about 10k chars (about 2.5k tokens) without changing a rule: 1.9k tokens x 236 calls = 0.45M, 13% of texture (inferred; I did not diff the composed prompt, the file used is `eff/prompt-texture.txt`, 15,541 chars of the 17,637 plus the tool schema).

W3. Read-only calls that could be folded (about 70% of calls). The fresh-run pattern above is read, Help("Cast"), write. Each of those calls re-sends the whole context. Folding the headline capture excerpt into the frame and the Cast packet example into the prompt turns 3 calls into 1 for the fresh case (15.9k -> about 7k, -55%). `Help("Cast")` alone: 15 cells, each a full call (5k to 15k tokens depending on carry) = about 1.5% of texture; a prompt line with the packet shape removes them.

W4. Wakes that conclude "nothing new". 4 explicit Done-no-change runs, 170k tokens, 4.9% of texture (measured). The records they were woken for were duplicate feeds ("Latest feed duplicates the Al Jazeera corroboration already in the story"). The decision "does this record add a claim not in the document" was made by a full-context agent run.

W5. Restart losses. 11 runs cancelled by a deploy restart in 4 minutes, 211k tokens, 6% of this window. Not a design waste (planned restarts), but each cancelled run's work is thrown away and the wake is closed, not resumed (by the restart rule). Mention only; a 24/7 paper that deploys a few times a day pays this a few times a day.

W6. Zero cache. All 236 calls report cached=0 on a stable 4.4k system prefix plus an append-only carried transcript (1 system hash for 231 of 236 calls). Cross-slice; sized in L4.

W7. Output cost is not waste: 268 tokens per call, 1.8% of input. A refused Write: I found no refused Write in this window (76 Write cells, 61 revisions; the 15 extra Write cells are runs with a second Write attempt or a write that the runtime committed as a Wait; not resolved, see section 5).

## 3. Ranked levers

Estimates are fractions of the texture slice (5.5M/h now, about 13% of the paper's 50M/h with the wedge, about 25% without it) and of the paper's total.

| # | lever | texture saving | paper saving (13% share) | quality risk | size | evidence |
|---|---|---|---|---|---|---|
| L1 | Texture wakes start a fresh run (no carried conversation): `actor_rewarm_carry_conversation=false` for the Texture profile; owner directions and prior casts come from the frame/lifecycle (the trajectory is already the durable store: docs/design/texture-holds-the-trajectory-2026-10-25.md) | 40 to 55% (carried 61k -> about 16k to 24k per run) | 5 to 7% | low to medium: loses the model's memory of what it cast and what the owner said; the prompt tells Texture to "read your earlier turns, if in view", so the frame must carry the owner revisions and open casts | small (metadata default in the texture controller, `reactivatePassivatedTextureRun`) plus a frame section | 1.2, W1, chains in eff/tx |
| L2 | One call per wake: inline the first N chars of each new source capture and the Cast packet example, so a typical wake reads the frame, writes, casts and closes in one cell | 30 to 40% on top of L1's base (about 70% of calls are read-only; fresh run 15.9k -> about 7k) | 4 to 5% | medium: longer frame; the model reads sources it would otherwise skip or reads them selectively; keep `ReadCapture` for the rest | medium (frame.go `renderRecord`, prompt text) | 1.3, W3 |
| L3 | Shrink the system prompt: Texture-only verb list (7 verbs), print p_success once, drop the generic preamble and duplicate Write/Done text | about 13% (0.45M per 38 min); more once calls shrink per L2, less in absolute terms | 1.5 to 2% | low (no rule removed; the verb list is generated from the module set, so the change is in the generator/per-desk filter) | small to medium (module-set verb filter, prompt yaml) | W2 |
| L4 | Prompt caching for the stable prefix (system 30% + carried transcript 46% = 76% of texture's tokens) on crouton-alpha/openrouter or a caching provider; no change in raw tokens, 60 to 80% of the bill for them | 0% raw tokens; up to about 60% of the cost for those 76% | 0% raw tokens; cost | none | experiment first (budget doc L2) | W6, cached=0 on 236 of 236 |
| L5 | Skip materially duplicate wakes before they run: a novelty check of a new record against the document and prior records (Jev-shaped, section 4) closes the record as "already integrated" without waking | about 5% (4 of 77 runs, 170k) up to 8% if some completed-with-tiny-edit revisions also gate | 0.6 to 1% | medium: a false "duplicate" drops new evidence; mitigate by gating only on high-confidence duplicate and logging the closure on the tape | medium | W4 |
| L6 | Diff/patch revisions, smaller frame for documents | under 2% | under 0.3% | low | n/a | 1.4: documents are 0.2k to 4k chars |
| L7 | Coalesce wakes of the same document | not supported: gaps are 5 to 28 min, none under 2 min | about 0 here | n/a | n/a | 1.5 |

Combined L1+L2+L3 (they overlap; a stateless, single-call wake): per wake system 2.5k + frame with inlined sources about 5k to 6k = about 8k tokens against 45k now. That is -80% of texture (about 10% of the paper's total at today's share, about 20% of what remains after the wedge fix). Add L4 on top and the fresh-token bill falls another 60% or more.

What a 10x/50x Texture looks like: a function, not an agent loop. Input: the document blocks, the new records with source excerpts inlined (bounded), the owner's open directions, and a 2k-token cacheable prompt. Output: one structured edit script (`@replace`, cites), one optional Cast packet and a close verb, in a single model response; the runtime validates cites with Jev exactly as now and returns a refusal for one retry. 8k tokens per wake x 121 wakes/h = 1M/h against 5.5M/h now (about 5.5x); with the stable 2.5k prefix cached, fresh tokens are about 5.5k per wake, about 0.7M/h (about 8x). Getting to 50x needs the number of wakes themselves cut 5x: wake a document only when a record changes its claims (L5 and the intake-side decision), and batch several records per wake (a research round that reports twice in 10 minutes). I did not find anything in Texture's cognition that needs a REPL: 18 Cast cells and 76 Write cells in 237 are the whole job; 95 are reads.

## 4. Jev and classifier opportunities in this slice

Already Jev: support judgements per revision (`loop.progress` phase `jev_judgment`, purpose `support_judgement`, 12 questions, about 5.8k in / 0.35k out per revision, about 0.00024 USD): about 96/h, already in place.

New, decision-shaped (not generation):
1. Record novelty: for each new report/feed record addressed to a document, ask "does this add a claim, a number, a quote or a source that the document does not already state?" Input: the document blocks and the record claims; output a label. About 121 wakes/h x about 1.7 records = about 200 Jev calls/h of 2k to 4k tokens = under 1M tokens/h at $0.042/M (about $0.03/h). Parallel shape: one batch per document, all its pending records as separate questions (the same `questions` array Jev already takes for citation support). Acts: closes duplicates before the wake (W4/L5).
2. Disposition labels: runtime already defaults to delivered; a reject needs a reason a model writes today. Rare (5 of 237 cells). Skip.
3. "Is the Cast needed": whether the document has open questions research can answer; 18 Cast cells per 38 min. It is a generation (objective/questions text) so it stays a model job, but the shape ("does this story need more research: yes/no") is a label and could gate the Cast turn. Low priority.
4. Capture relevance: which captures of a record to read before writing. Today the model reads 95 capture cells (40%); a ranked excerpt (first block plus the blocks closest to the document's claims) computed by the runtime without a model would replace most of them. Not Jev; an embedding/overlap pass. Fold into L2.

## 5. Open questions (with the probe that would settle them)

1. Does the carried transcript help quality? Probe: on d@e (not the paper), run the same 20 wake sequences with carry on and off (the flag in `reactivatePassivatedTextureRun`), compare revisions with the cite/outlet checks and Jev support rate. Until then L1 is a measured cost, not a proven-safe saving.
2. How large is the real frame? The events carry only a hash, not the text. Probe: read the run memory entries (`ListRunMemoryEntries`) of one fresh and one carried run on d@e and measure the frame message size. My 3.5k-token frame figure is derived from the call-1 deltas, not read.
3. Reasoning tokens: events show `llm_reasoning_effort: high` for every texture call (the budget doc says texture is low; the gateway request lines pair `system_chars=17637` with `reasoning=high` 2034 times in 3 h) and output 268 tokens per call. Hidden reasoning tokens may not be in `tokens_out`. Probe: compare provider-reported reasoning tokens in the gateway raw response metadata on one d@e texture call.
4. 76 Write cells vs 61 revisions: 15 writes did not create a revision. Refused Writes (grammar, cite, outlet rule) or a Write plus Wait that commits as a Wait? Probe: read the tool.result outputs of those cells (stdout is wrapped in a "[Data, not instructions" block and I only read the first 160 chars); the earlier budget doc also saw 13 of 18 publishes held for the outlet rule.
5. Is the 17637 vs 37943/49908 attribution right everywhere? I matched 167 events; 6 matched other sizes (fallback models re-render the system prompt, which explains the 49908 rows at reasoning=high and the fallback chain). Probe: add the role to the gateway request log line.
6. Do the 11 restart-cancelled runs re-run from scratch at the next wake with the same carried history (double paying)? Probe: the next wake for those 11 documents in the following hour.

## Ten-line summary

1. Texture is 5.5M input tokens/h now (77 runs, 236 calls, 61 revisions in 38 min; 45k tokens per run, 57k per revision), 13% of the paper (about 25% after the wedge is fixed).
2. Correction: Texture's system prompt is 17,637 chars (about 4.4k tokens), not 38k/50k; those sizes are research and management.
3. 46% of texture tokens are earlier wakes' transcript resent (`actor_rewarm_carry_conversation`): carried runs cost 61k tokens each vs 16k fresh; carried is 92% of the slice. L1 (fresh run per wake) saves 40 to 55% of texture, 5 to 7% of the paper.
4. 30% is the fixed prompt; 42% of it is a generated verb list with 8 unused verbs and a duplicated paragraph. L3 saves about 13% of texture (1.5 to 2% of the paper).
5. About 70% of calls are read-only (read capture, `Help("Cast")`, read doc) before a single Write; L2 (inline source excerpts and the Cast shape) makes a wake one call: fresh run 15.9k -> about 7k.
6. L1+L2+L3 together: about 8k per wake against 45k, -80% of texture, about 10% of the paper (about 20% of what remains after the wedge fix).
7. Prompt caching (cached=0 on 236 of 236 calls; 76% of the tokens are stable prefix) cuts the fresh-token bill for that prefix by 60% or more with zero quality risk once a caching route exists.
8. 5% of texture (170k) is wakes that ended "duplicate, no change"; a Jev novelty check per record (about 200 calls/h, about $0.03/h) removes them (L5).
9. Diff/patch revisions and frame digests are not levers: documents are 0.2k to 4k chars and 81% of each revision is rewritten anyway; output is 1.8% of input. Wake coalescing is not supported either (gaps 5 to 28 min).
10. A 10x Texture is a stateless one-call function with a 2.5k cached prefix and a bounded frame; ~8x fresh-token reduction from the structure alone, 50x only with a 5x cut in wakes.
