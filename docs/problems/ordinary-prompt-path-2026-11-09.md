# The ordinary prompt path: few sources, a runaway on a note, and a slow v2 (2026-11-09)

Status: investigation only. Nothing was changed. Mutation class: green (docs).
Rollback: revert.

Owner report, 2026-11-09, from their own computer (not readable by the
investigator):

- (A) "What's latest in ai?": Keenable gave 40 sources and the story cited 4
  (v2) then 7 (v3); Texture said some items "proved unreadable and were
  excluded" and that "the Google Singapore research grants item was not
  captured".
- (B) "Gotta fix the prompt bar": only a v1 acknowledgement. The Models panel
  shows management at 498 calls, 165.6M input tokens and 52k output on
  crouton-alpha, plus 177 calls and 69.3M input on gpt-6-luna; engineering 30
  calls and 994k input.
- (C) A first version took about a minute and v2 "quite some time".

Labels used below: **CONFIRMED** (trace or code read in this investigation),
**HYPOTHESIS** (consistent with the evidence, not shown by it). Per
CLAUDE.md "Review the traces before attributing behavior", any claim about
the owner's own computer is a hypothesis; only d@e was read.

Reproduction: d@e (computer-59c902c5d46caecc0a180a8557164e72), two prompts
submitted 2026-11-09 03:07:32Z (of the three allowed). Clock times are Z.

| prompt | submission | doc | trajectory |
|---|---|---|---|
| "What's latest in ai?" | df94687a | 24e292a8 | 4efc7c68 |
| "Gotta fix the prompt bar" | e137062a | 7b4a53c5 | 2eec7251 |

Runs (GET /api/runs/<id>/events): AI router df94687a, Texture v1 befa871f,
research 3b98e770, Texture v2 d9a30b60, follow-on Texture rewrites 698fb7d8
and 8ddc474b. Note router e137062a, Texture v1 2f8aecfe, management
ddd61829, engineering run:assignment-cbd3340826fb2c5991f3c3e9. Earlier
receipt for the loop pattern: management 90c960b5 (ledger 2026-11-09).

## 1. (A) Where the sources go

### 1.1 The funnel on d@e (run 3b98e770, the research desk)

| hop | count | what dropped the rest |
|---|---|---|
| Keenable results per search | up to 40 (s1 = 40 at seq 15; s5 = 40 at seq 67) | none: the cap is the request, 40 |
| searches the desk started | 13 (4 at seq 8, 4 at seq 43, 3 at seq 60, 2 at seq 77) | model choice, no budget |
| distinct pages the desk tried to Read | 12 | model choice (it chose from titles and URLs) |
| not captured (Read failed) | 3: openai.com HTTP 403 (seq 45), venturebeat.com HTTP 429 (seq 48), errorless.com "no readable text" (seq 57) | capture failure |
| captured | 9 distinct pages (10 capture ids: the anthropic.com page was read via three r.jina.ai URL spellings) | |
| captured but not passed | 2: cryptobriefing.com and tlt.com, read OK at seq 17, never packed | model choice |
| passed to Texture | 6 sources, 9 claims, one ReportPacket (seq 81, 03:09:56) | |
| passed but not cited | 0 | Texture cited 6 of 6 (18 cites, seq 37 of d9a30b60) |

CONFIRMED: of 12 pages tried, 6 reached Texture and all 6 were cited. Texture
did not cut. The loss is upstream, in research.

### 1.2 Rule at each hop

1. **Search to the desk: no cut.** `deskSearchMaxResults = 40`
   (internal/agentcore/desk_search.go:37) is passed to every provider
   (desk_search.go:137); Keenable clamps to 50
   (internal/gateway/search.go:302-304) and returns up to that. On d@e only
   one provider answers per search ("1 providers (8 cooling: searxng, tavily,
   brave, parallel, exa, serper, serpapi, chatgpt)", seq 43), so a search is
   one Keenable list of 40. CONFIRMED.
2. **Desk to Read: model choice, with no budget in code or prompt.** The
   research prompt says "Search, then read the sources behind the results"
   and "with parallelism appropriate to the task" with no count
   (internal/promptstore/defaults/research.yaml:22-24;
   internal/runtimeprompts/overlays/rlm_research_runtime.yaml, Evidence
   Doctrine). There is no read counter in `readPage`
   (internal/researchtools/read.go:105-205). The desk read what looked like
   roundups and primary pages; it never opened ~95% of the ~500 results.
   CONFIRMED that no code rule limits Reads; the choice of 12 is the model's.
3. **What the model could see of the results is cut.** A cell print over 8000
   characters reaches the model as a 6000-rune head, a marker and a
   1500-rune tail (internal/yaegikernel/cell_output.go:19-26). Four searches
   printed as title|URL lines were about 22k characters (seq 15 output was
   7920 shown; the rest via `choir.CellOut`). The model then panicked two
   cells by slicing a string out of range (seq 35 `out[2000:10500]`, seq 66
   `r.Title[:80]`), each of which replaces the Go session ("unsafe_to_reuse"),
   so the search table (s1..s4) was lost and ids restarted (seq 78 prints
   "s1 started" again). CONFIRMED. Whether this lowers the number of sources
   read is HYPOTHESIS: the desk did keep finding primary pages afterwards.
4. **Read to capture: capture failures.** `readPage` refuses HTTP >= 400
   (read.go:125-126), non-HTML/PDF/text types (read.go:150-151) and pages
   with no extractable blocks (read.go:153-154). The desk discarded these
   errors in most cells (`p1, _ := choir.Read(...)`, seq 46), so the failure
   reason is visible only in the egress rows of the desk.cell event (seq 45,
   48, 57), not in the model's context. CONFIRMED. The desk recovered
   openai.com and anthropic.com via The Verge and an r.jina.ai mirror
   (seq 58), a good workaround, and the 429 on venturebeat was not retried.
5. **Capture to packet: model choice.** `coagentpacket.Validate` has no cap
   on `sources` and requires `target.uri` only
   (internal/coagentpacket/packet.go:225; `capture_id` is not required).
   The prompt says to report "everything you found"
   (research.yaml:28-31). The desk packed 6 of 9. CONFIRMED that no rule
   drops; the 3 omitted are the model's choice.
6. **Interim packets were skipped.** The research inbox prompt and the
   overlay both say "Send an interim packet ... as soon as you have read one
   new source you can cite" (internal/agentcore/inbox_records.go:925;
   rlm_research_runtime.yaml, ReportPacket paragraph). The desk sent one
   packet at the end (one `ReportPacket(` in the whole run). CONFIRMED.
   Effect on (C) in section 3.
7. **Packet to frame: no cut, but uncaptured sources are labelled.**
   `RecordsFrom` lists every packet source (internal/texturewrite/frame.go:292-
   294, no limit). A source with no capture shows the hint "(not captured: it
   cannot be cited until research Reads it)" (frame.go:385-390), decided by
   whether `CaptureBlocks` finds a capture (internal/agentcore/texture_frame.go:
   161-168). CONFIRMED.
8. **Frame to citation: Texture's write gate.** `choir.Write` refuses the whole
   revision when a cite's quote is not found word for word in the capture
   (internal/texturewrite/apply.go:650-652, 701). On d@e Texture's first v2
   Write was refused for one paraphrased quote (d9a30b60 seq 24: "NVIDIA
   added four open-weight checkpoints ..." not in r1.c6) and rewritten
   (seq 37), costing 17 s. CONFIRMED. A model that is told "cannot cite
   uncaptured" and facing this refusal can drop a source instead of fixing
   the quote. HYPOTHESIS that this explains the owner's 4 of N.

### 1.3 The owner's wording

- "not captured": CONFIRMED mechanism (7 above). A research packet may carry
  a source with a URL and no capture (packet.go:225), and Texture's frame says
  so. HYPOTHESIS that the Singapore grants item was such a source; the
  owner's trace is needed (frame text of the v2 run).
- "proved unreadable and excluded": no code or prompt string produces this
  (grepped internal/textureprompts and texturewrite). It is the Texture
  model's own words, probably paraphrasing a research claim or excerpt such
  as d@e's "OpenAI returned 403" (seq 81 excerpt of s5). HYPOTHESIS. It also
  breaks the Texture rule "Never describe the sources or the work"
  (internal/textureprompts/texture.yaml:46). The d@e v2 prose has no such
  sentence (export of 24e292a8 at c189596f).
- 4 then 7 sources: not reproduced; d@e had 6 at v2. The count tracks how many
  pages the desk happened to Read and pack, not any limit.

### 1.4 Answer

The cut is a **model choice** at two hops (which pages to Read; which
captures to pack) plus a **capture failure** class (403, 429, no text) that
the desk mostly absorbs silently. It is **not a budget** and **not a prompt
instruction**: nothing says how many to read, and nothing stops more.
The one mechanical loss is a refused Texture Write (quote gate) that can
eliminate a source, and an unrecorded Read error that the model cannot see.

### 1.5 Design space (A), cheapest first

1. Prompt: a floor for the desk ("read at least N independent sources for a
   survey question, or say why fewer") and "pack every capture you read"
   (research.yaml). Cheapest; model-dependent.
2. Surface the read ledger: put a `reads` list (url, ok|error, capture id)
   in `choir.Searches`/Context so a dropped error is never invisible, and
   print dedupe-by-host summaries instead of 40 raw lines (fixes the 8000
   rune cut and the slicing panics at their source).
3. Mechanical pack: a `choir.Captures()` verb that lists this run's captures,
   and a ReportPacket check that warns (not refuses) when a captured read of
   this run is absent from the packet, mirroring Texture's frame.
4. Texture: on a quote refusal, report which cite failed and keep the source
   in the revision with a corrected quote (apply.go:701 message names the
   handle already; the repair is the model's).
5. Dearer: a fan-out reader (a child per URL batch) so the desk judges
   summaries, not 40 titles. Uses `choir.Research` children, which already
   exist.

## 2. (B) The note, "Gotta fix the prompt bar"

### 2.1 What d@e did (no runaway)

- 03:07:33.77 router (conductor) e137062a, completed 03:07:34.72 (0.95 s).
- Texture v1 2f8aecfe 03:07:34 to 03:08:01 (27 s, 3 calls, 25.8 s of model):
  it wrote the v1 acknowledgement ("... engineering dispatched") and
  `choir.Cast("management", ..., kind: "execution_request")` (seq 11 refused
  `owner_directions`, seq 16 accepted). The Texture prompt sends any request
  to "build, run or test" to management (texture.yaml:30-34), and there is no
  path for a note that asks nothing yet. CONFIRMED.
- Management ddd61829 03:08:00.8 to 03:08:11.3 (10.5 s, 3 calls, 36.7k input):
  `choir.Records()`, then one cell that Cast engineering (with the
  predictions map), then end_turn "Engineering assignment opened for the
  prompt-bar repair" (seq 12). CONFIRMED. No loop.
- Engineering run:assignment-cbd33 started at 03:09:35, 84 s after the
  management turn ended (a capsule start gap, not measured further); 9
  calls, 27 s of model, then parked on a screenshot job until 03:21:09
  (`capsule_job_park`, seq 23). The round stays open while the job runs, so
  the document stays at v1. CONFIRMED.
- GET /api/texture/documents/7b4a53c5/models: management 3 calls, 36.7k input;
  texture 3 calls, 17.9k input. So the d@e Models panel for this note is
  about three orders of magnitude below the owner's.

The owner's runaway is **not reproduced** (one trial; no later management
wake in 6 minutes).

### 2.2 The loop mechanism on d@e (the flint receipt)

Ledger 2026-11-09: management 90c960b5 on recursive/flint-v1.0:free ran 00:24
to 00:49. Its trace (this investigation, GET /api/runs/90c960b5.../events):

- seq 67 onward: one-line cells `choir.ListDir("/work")` returning `=> []`,
  repeated at about 1-3 s each; 60 calls in the 00:36:39 to 00:40:46 window
  alone (input 48.7k to 49.5k, growing about 54 tokens per call). The cells
  are identical and return identical empty results; the model does not
  change approach.
- The run ended only when the serving model changed (later events show
  `hf:deepseek-ai/DeepSeek-V4.1-Flash-1017` doing real work from 00:47:37).
- The loop was an engineering-shaped task given to management (find the
  package source under /work), with no repository mounted in management's
  cell, so the observations were empty.

CONFIRMED mechanisms, any model:

- The tool loop has **no iteration bound**:
  `for i := 0; ; i++` (internal/toolregistry/toolloop.go:841). The only
  exits are a final answer, a tool error that propagates, quota exhaustion
  (toolloop.go:1030, 1070) or a cancelled context.
- There is **no repeated-call detector**: nothing compares a cell to the one
  before it or counts identical results.
- Run memory compacts at 70% of the model's window
  (internal/provideriface/config.go:45; run_memory.go:794-807), which bounds
  one call's context but not the number of calls.

### 2.3 Does the owner's case match the flint loop?

HYPOTHESIS only. What matches: management, an engineering-shaped note, a
run with no stop rule. What does not fit:

- The owner's management averaged about 332k input tokens per call on
  crouton-alpha (165.6M / 498) and 391k on luna (69.3M / 177); engineering
  averaged 33k (994k / 30), the same order as d@e's engineering (about 30k
  by the end). A management turn on d@e starts at 11.9k tokens. The flint
  loop grew by 54 tokens per call, so 500 such calls would sit near 40k,
  not 332k. A 332k average means the owner's management context was large
  from the start or each iteration printed large results (for example
  `choir.Records()` with a long Cast body, or a ProductAPI list), so this is
  not the d@e ListDir shape.
- The Models panel is the **lineage** of a document, not one run:
  `DocumentModelProvenance` sums every run on the document's channel or any
  of its trajectories plus the management run that cast an engineering run
  (internal/agentcore/model_provenance.go:145-208; capped at 400 runs). The
  panel's "runs" field per model says how many runs make the 498. If
  management turns for other work share the note's trajectory (they wake per
  trajectory), the panel can mix them. HYPOTHESIS; the owner's
  `GET /api/texture/documents/<id>/models` (field `runs`) and
  `GET /api/runs/<management run>/events` (`iteration` and `last_user_sha256`
  on loop.progress) decide it in one read each.
- "The round never closed" fits an open engineering job (2.1) as well as a
  loop. On d@e the round was open for the job's duration with nothing wrong.
- Cache: every d@e call on crouton-alpha reports `cache_reported: true` and
  `cached_input_tokens: 0` (e.g. research loop.completed, 630,714 input, 0
  cached). With no cache hit, a long run's cost is the sum of every call's
  full context. CONFIRMED on d@e; the cause (provider does not cache, key
  mismatch after failover) is not known.

### 2.4 Design space (B)

1. Stop rule in the loop: a per-run iteration soft cap (say 60 for a desk
   turn) that ends the turn with the visible fate "stopped: N calls without
   progress", plus an identical-cell detector (same source hash, same
   output hash, 5 times) that injects "you repeated this" once and then ends
   the turn. Substrate fix (toolloop.go:841). Closes the flint class and the
   owner's class whichever model runs.
2. Texture: a note is not a build. Let Texture Write the v1 and Cast nothing
   when the prompt asks for no work (an `ack_only` intent), or Cast management
   only for "build, run, test" with the owner's words (texture.yaml:30-34
   already says so; the model judged "fix the prompt bar" as a build, which is
   reasonable). Cheaper: the prompt-bar route (promptroute) offers a "note"
   destination so the owner chooses.
3. Management cell surface: give management an empty-state hint on
   `ListDir` ("no repository is mounted in management; engineering has it"),
   so an empty list is a fact, not a puzzle.
4. Cost: report cache hits per provider and fail loud when a long run has
   0% cache on a model that bills uncached input.
5. Panel: print `runs` beside `calls` so one number is never read as one run.

## 3. (C) Time budget per hop on d@e

AI prompt, 2026-11-09 (all CONFIRMED from event timestamps):

| hop | from | to | wall | note |
|---|---|---|---|---|
| router (conductor, df94687a) | 03:07:32.45 | 03:07:33.41 | 1.0 s | default route, no model |
| Texture v1 (befa871f) | 03:07:33.05 | 03:07:54.22 | 21.2 s | 3 calls, 20.0 s model: 4.1 + 11.9 + 4.0 s; one compile error (`_` used as a value, seq 11) cost the third call |
| dispatch to research | 03:07:53.92 (v1 revision) | 03:07:53.905 (research submitted) | ~0 s | Cast commits with the Write |
| research (3b98e770) | 03:07:53.9 | 03:10:00.6 | 126.7 s | 23 calls, 114.5 s model (90%), 9.1 s cells; searches return in under 0.4 s, Reads 0.1 to 3.2 s; the final packet call alone took 40.7 s (3062 output tokens, seq 79) |
| first citable source read | | 03:08:11.8 | +18 s | seq 17; no packet sent then |
| Texture v2 (d9a30b60) | 03:09:56.60 | 03:10:54.71 | 58.1 s | woke 0.0 s after the report; 9 calls, 55.9 s model; 6 sequential ReadCapture cells (03:10:01 to 03:10:16, 15 s); first Write call 17.4 s and refused (quote), second Write call 14.3 s |
| Texture rewrites 698fb7d8, 8ddc474b | 03:10:55.5 | 03:11:21.2 | 25.7 s | two more revisions of block b4 (support failures, `integrate_execution_findings`) |
| prompt to v2 | 03:07:32.45 | 03:10:54.7 | 202 s | |
| prompt to settled | | 03:11:21.2 | 229 s | |

Note prompt: router 0.95 s; Texture v1 27 s (3 calls, 25.8 s model, 15.0 s
in the call that wrote the v1 and the Cast); management 10.5 s; engineering
capsule start gap 84 s.

Reading: 90% or more of every desk's wall time is model latency; cells,
searches, Reads and dispatch are under 10%. The scheduler waits that the
owner has been chasing (docs/problems/activation-wait-times-2026-11-09.md)
are small here: dispatch to research is 0.2 s, wake to Texture v2 is 0.0 s.
The model chain starts with two or three instant failover hops on every run
(nemotron `known_exhausted`, then two opencode-go models with an open
circuit), 0.3 s in total; the default pool's first choices are already out.

What is not shown: the owner's "about a minute" first version and "quite
some time" v2. d@e's v1 was 21 s. The prior record shows one free-tier call of
165 s inside an otherwise normal Texture turn (activation-wait-times 2026-11-09),
which is the likely owner-side cause. HYPOTHESIS; the owner's v1 run's
`provider_call_ms` per `loop.progress` would settle it.

### 3.1 Where v2 time can be recovered

- **Interim packets** (the largest): research had its first citable source at
  +18 s and sent nothing until +126 s. A packet at +20 s would put Texture v2
  at about +60 s from the Cast instead of +180 s. The instruction exists
  (inbox_records.go:925, overlay) and was not followed; the owner's "v2 after
  quite some time" is this wait. CONFIRMED on d@e.
- The research final-packet call (40.7 s) is a single generation of nine
  claims and six sources with long quotes. Packing quotes as block ids, with
  the runtime copying the text, would remove most of it (and the quote-gate
  refusals). Design only.
- Texture v2 reads captures one cell each (15 s); one cell reading all six
  would cut that to about 3 s (prompt change: `choir.ReadCapture` in a loop;
  the first cell did read three, the rest one by one).
- Compile and quote errors cost one full model call each (4 to 17 s).

### 3.2 Design space (C)

1. Enforce interim reporting mechanically: wake Texture on each captured Read
   that research has not yet reported (a runtime nudge to the desk after its
   first successful Read: "report what you have now"), not on prose alone.
2. Parallel Reads and one Texture read cell (prompt).
3. Packet by reference: sources as `{capture_id, block_ids}` and let the
   runtime fetch the exact quotes, so neither desk types a quote.
4. Model: a free tier call can take minutes; a latency budget per call with
   failover (the failover exists for quota; a deadline is not part of it).

## 4. What would turn the hypotheses into findings

Owner-side reads (GET only, one each):

1. `GET /api/texture/documents/<note doc>/models` (field `runs` per model).
2. `GET /api/runs?limit=N` filtered to management around the note, then one
   management run's events: the cells it ran, the `iteration` count, and the
   input tokens per call.
3. The v2 Texture run's frame (first user turn): which sources carried "not
   captured" and what research's packet said about the Singapore item.
4. The v1 run's `provider_call_ms`.

## 5. Residuals and next investigation

- One trial on each prompt; the model pool on d@e at test time was already on
  crouton-alpha through failover, which the owner's panel also shows.
- Not tested: a second AI prompt (to see whether research's source count
  varies); a note without an engineering-shaped verb.
- The two Texture rewrites after v2 (698fb7d8, 8ddc474b) are unexplained
  here; they follow support failures and read like repair of citations.
