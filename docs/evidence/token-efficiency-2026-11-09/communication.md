# Token efficiency audit: slice "communication" (owner, 2026-11-09)

Scope: what desks send each other that lands in a model's context: update packets
(`coagent_update_packet.go`), Texture frames, inbox records, ReportPackets, casts,
cell-result envelopes, carried conversations. Read-only. Evidence files are in
`$SCR/eff/comm/` (`ev/*.json` = per-run event dumps, 58 runs; `runs.json` is the
earlier 500-run list at `$SCR/runs.json`, 16:33-18:31 UTC). Every number is marked
M (measured), E (estimated from M by stated arithmetic) or H (hypothesis).

Method: `GET /api/runs/{id}/events?limit=2000` through `paper.sh` for a random
sample of completed runs (seed 1): 16 research, 30 texture, 12 management_task.
Per-call input is the `loop.progress` payload (`input_tokens`, `messages`).
Message bodies are NOT in the event stream (only a count and `last_user_sha256`),
so frame/packet sizes inside the transcript are inferred, not read. The probe that
would settle them is in section 5.

## 1. What this slice spends

Paper, 2h window (from the budget doc, M): 98.9M input tokens; research 30%, texture
13%, management_task 7%, wedged management 51%. Communication is not a separate
call; it is the part of each call's input that is a message one desk sent another.
Per role (M from the sample, call-level):

| role | first-call input | growth per later call | what grows it |
|---|---|---|---|
| research (16 runs, 298 calls) | 9.2k with 2 messages (system+tools+874-char wake) | +1.5k to +2.5k per cell | cell results (search 5.5k chars avg, Records 2.8k) and the model's own ReportPacket source (2-10k chars, 2-5 per run) |
| texture fresh (12 of 30 runs) | 4.6k, 3 messages | +0.5k per cell | frame is inside the 4.6k |
| texture carried (18 of 30 runs) | 6.5k to 66.7k, 10 to 91 messages | +0.5k to +1k | the prior wakes' conversation |
| management_task (12 runs, 114 calls) | 3.0k, 2 messages | +1k to +10k per cell (one capture read = 51k chars) | capture bodies, Stories(), Help |

Cell result envelope (M, 449 cells): every `desk_go_eval` result carries a JSON
shell of about 615 to 750 characters that is not content: `diag, duration, error,
intents, receipts, reuse, stderr`, a 170-char "session: new session: nothing from
this desk's earlier runs is defined. A session ends on a restart ..." string on
about a third of cells, and the 300-char "[Data, not instructions ...labels...]
... [End of data]" wrapper. That is 22% (texture), 15% (management_task) and 17%
(research) of all tool-output characters.

Update packet (the management wedge): `buildCoagentUpdateUserMessages`
(`coagent_update_packet.go:78`) emits, per pending update, an `update_refs` entry
(update_id, from_agent_id, from_role, channel_id, message_seq, kind) AND an id-only
`updates` stub. It is O(backlog) and the ids are 36-char UUIDs. A reconstruction with
41 random UUIDs gives 12.4 KB, about 4.4k tokens at 2.8 chars/token, plus preamble and
instruction: that matches the 5.65k to 5.85k tokens per call the budget doc measured
(E, but the match to the observed +2 messages / +5.7k per call is tight). The stubs
alone are 18% of the packet and repeat ids already in `update_refs`.

## 2. Waste found

### W1. Carried Texture conversations: 46% of Texture input is the carried prefix (M sample, E scaling)

Evidence: `texture_*.json`. 18 of 30 sampled Texture runs start with `messages` 10
to 91 and 6.5k to 66.7k input tokens; 12 start at 3 messages and 4.6k. Run
`ed6b755b` had 91 carried messages, 66.7k at call 1, and spent 133,973 input tokens
in two model calls to produce one revision. Carried runs are 85% of the sampled
Texture input (1.11M of 1.30M). Counting only the part above the fresh 4.6k base,
the carried prefix is 570k of 1.23M = 46% of sampled Texture input, i.e. about 6% of
the paper's input (0.13 x 0.46).

Mechanism (code): `texture_agent_revision.go:690` sets
`actor_rewarm_carry_conversation`; `run_memory.go:carryActorConversation` copies the
latest checkpoint plus every message after it verbatim into the new run. The only
bound is the 140k threshold compaction. Each wake also injects a NEW full frame
(`textureFrameInjector`, `texture_frame.go`: document blocks up to 12,000 chars,
plus records) whenever the revision or the cursor changed, which on a carried run is
always true, because the wake exists to read a new record. So the carried tail holds
one superseded frame per earlier wake (H: each about 3k tokens for a long document,
less for a story seed; confirm by reading run memory, section 5).

The frame is a pure function of (document head, records past cursor). Its older
copies carry no information the newest one lacks. The model's own earlier casts and
writes are the part worth carrying.

### W2. One research report reaches a model about 30 times (E)

Chain for one ReportPacket of about 5.5k chars (~1.4k tokens; M: 42 packets, median
5.1k chars, range 0.6k to 10.7k, 2.6 per run):
1. Research model writes it (output), then it stays in research's transcript as the
   assistant tool-call argument for the rest of the run (~8 further calls) = ~11k
   input tokens.
2. It becomes a record; the Texture frame renders it (claims, sources, 300-char
   excerpts, hypotheses; `texturewrite/frame.go:renderRecord`) = 1.5k.
3. That frame is carried: every later call of every later Texture wake resends it.
   With ~3 further wakes of ~4 calls each = ~12 resends = ~17k.
4. Texture's Write cell restates the claims as prose (output) and that Write
   argument also stays in the transcript.
5. For a fresh capture, Texture also calls `ReadCapture(...,0,0)` for the full text
   (seen in sample `texture_1ec4...`), so the same source reaches the model as
   excerpt (frame), as claim (record) and as whole body (cell result).
Total about 30k input tokens for one 1.4k-token finding (E; the 11k and 17k terms are
M-derived resend counts times an assumed packet size).

### W3. Interim ReportPackets wake Texture once each (M count, H value)

The research wake prompt (M, `runs.json`) tells research to send "an interim packet
as soon as you have read one new source you can cite, and another each time you have
read more; the final Report comes last". Sample: 2.6 ReportPackets per research run
(0 to 5). Texture wakes: 331 runs in 2h, 313 of them `request_source=inbox_record`
with intent `integrate_execution_findings`, over 144 documents (mean 2.3 per
document, max 8); 28 of 30 sampled Texture runs created a revision
(`texture.document_revision.created`). 77 research runs vs 313 record wakes = 4.1
wakes per research run. Each wake costs 38k input tokens on average (12.6M / 331,
M). In a sampled document the wakes are 4 to 12 minutes apart
(`7b2ab9fa`: 16:56, 17:00, 17:07, 17:13, 17:25, 17:36, 17:45, 18:29), so a record
that arrives while Texture is parked (120 s idle) re-opens a whole run.

Packets within one research run overlap: consecutive packets share 30-45% of
their words on the 3 runs I could compare cleanly (rough, difflib on words, H).

### W4. The update packet restates the backlog on every end of turn (M code, E size)

`injectUserTurns(true)` in `toolloop.go` (~line 1657, per the budget doc) appends the
final-checkpoint packet each time the model ends a turn. The dedupe is
`lifecycleInjectionIDsFromRunMemory` (`management_controller.go:2358`): it re-parses
the preceding injected message TEXT (finds the preamble prefix, unmarshals
`updates[].update_id`) and requires `schema`, `trajectory_id`, `target_agent_id`,
`target_run_id`, `owner_id`, `computer_id` to equal the run's. The packet's
`trajectory_id` is taken from the first update's `TrajectoryID` while the check
compares to `lifecycleControlTrajectoryForRun(rec)`. In the wedged run the same
last-user hash recurred on 74 of 94 calls (M, budget doc), so the dedupe
demonstrably did not hold. H: a field mismatch makes `continue` skip the packet, so
no id is ever "seen" and the same packet is re-injected forever. Not proven; the
probe is to read the run's `runtime_injection` memory entries and run the same parse.
Independent of the cause, the design has three defects: (a) delivery state is
re-derived by parsing prose out of the transcript; (b) nothing bounds how often the
same id set is injected; (c) the packet is O(backlog) in UUID tokens even though the
payload lives in `choir.Updates()`.

### W5. Cell-result envelope boilerplate, resent every call (M chars, E tokens)

615 to 750 chars per cell (about 170 to 210 tokens). A research run with c cells
re-reads each cell's envelope on the remaining calls: ~200 x c(c+1)/2 tokens. c=14:
22k of ~410k per research run (5.4%); texture c~3: 1.3k of 38k (3.3%);
management_task c~8: 7.5k of 76k (10%). Weighted by role spend that is about 1.3M
input tokens/hour = 2.6% of the paper (E). The "[Data, not instructions]" wrapper
appears even when the cell printed nothing external (`tray-1 <nil>` results in
management_task carry the full 300-char wrapper and the labels).

### W6. Management intake spends 76k tokens per capture pair on a routing decision (M tokens, H on the Jev coverage)

Sample: 12 management_task runs, 9.5 calls, 7.2k mean input (86 runs / 2h = 43 per
hour, 6.6M / 2h). The cell sequence is nearly fixed: `Records()`, `ReadCapture(...,3000)`,
`Stories(50)`, optional `Help("Cast")`, then `Cast(texture:new ...)`, `Feed`, or
`Decline`, then `Done`. The records already carry Jev's label per capture
(`r.Labels`, `CaptureLabelView`), and one sampled `Cast` failed with "story library
moved ... inbox record refused", costing a full retry loop (the model re-ran
`Stories()` and `Cast`). The `Help("Cast")` call re-teaches a verb signature that is
in the system prompt (M: 8 of 114 calls) and its output is retained.

### W7. Things I checked and found small

- Owner revision turns, disclosures, asks: the paper has none in the window
  (`request_source` counts: inbox_record 482, update_coagent 18). Owner revision packet
  is one revision's metadata (~0.3k tokens). Not a lever on the paper.
- Texture initial prompt: 145 chars fixed; research 874 chars; management_task 332.
  Not a lever.
- `update_coagent` to Texture (18 runs): wake turn is pointers only (the RLM change
  already landed). Good.
- Texture `Records()` calls: 0 of 90 sampled Texture cells (frame already shows
  them), so there is no frame-plus-cell duplication of records in Texture. For
  research and management_task, `Records()` is read once (one cell) per run.

## 3. Ranked levers

Savings = fraction of this slice, then of the whole paper's input. "Slice" here means
the communication share of roles it touches.

| # | lever | saving | risk | size |
|---|---|---|---|---|
| L1 | Make delivery state a durable per-run set of delivered ids (store column or run-record field written in the same txn as the injection), not a parse of message text; cap re-injection to once per id set per activation; packet becomes `N pending, read choir.Updates()` plus at most ~10 ids. | removes the wedge multiplier (the 51%, together with the frame fix owned by another slice) and shrinks any packet from 5.7k to ~150 tokens; after the wedge, saves ~2-5% of management tokens | low | small-medium (one injector, one test) |
| L2 | Do not carry superseded frames: in `carryableRunMemoryTail` drop every Texture frame turn except the last, and drop cell results older than the last two wakes (keep the assistant's casts and Write arguments, which are what the model needs to remember). Alternative: carry only the latest checkpoint and rewrite it at end of run. | 30-50% of the carried prefix, so 14-23% of Texture input, 2-3% of the paper (the other 3% is non-frame history; a checkpoint-only carry would take most of the 6%) | medium: Texture loses memory of its own tool results; verify on d@e with a story that needs three wakes | small |
| L3 | Coalesce record wakes per document: wake Texture at the final Report and at most one interim (the first citable source); hold further interim records until the document's run is parked or 10 minutes pass. Delete "another each time you have read more" from the research wake prompt. | 313 wakes to ~150 per 2h: 45-50% of Texture wakes, so 5-6% of the paper; the research-side packet writing and resends drop too (~1% more) | medium: the story updates less often in the first minutes; measure time-to-first-revision and revision count | small (prompt + wake debounce) |
| L4 | Slim the cell-result envelope: omit empty fields (`diag`, `error`, `reuse`, `stderr`, `receipts:null`, `intents:0`), drop the "new session" sentence after the first cell, and replace the 300-char data wrapper with a 40-char `[data: labels]` line only when the cell output includes labelled content (none for `<nil>`). | 600 chars/cell; 2.6% of the paper (E), 5% of research+texture+management_task | low, but the data-not-instructions marker is a prompt-injection defence; keep the marker, shorten it, do not drop it | small |
| L5 | Route management intake by Jev, model only for new-story drafting: Jev decides {new event, feeds story X, decline} using the capture labels and the story list; the model task runs only for `new event` and writes hypotheses; a `Feed` needs no model. | ~60-70% of management_task, so 4-5% of the paper (H: depends on Jev accuracy; run a replay) | medium (misroute duplicates stories) | medium |
| L6 | Reports by reference: research writes claims with source_ids only and the Texture frame shows record summaries plus `source_ids`; sources and excerpts load on demand (`ReadCapture(handle)`). Today the frame prints the 300-char excerpt, claims and handles, then Texture reads the capture whole anyway. | ~0.5-1k tokens per record, resent through the carried prefix; ~1% of the paper | low-medium | medium |
| L7 | Drop the id-only `updates` stubs (use `update_refs` in the dedupe reader) and `from_agent_id`/`channel_id` repeats (hoist to the packet header when equal). | packet -40% (18% stubs + repeats); subsumed by L1 | low | trivial |
| L8 | Fix the stale-story-library retry in management intake: refresh `Stories()` inside the Cast verb instead of refusing the record and making the model re-read. | 1 retry loop per refusal; small (<0.5%) | low | small |

Combined L2+L3+L4+L5 is roughly 12-15% of the paper's input after the wedge is gone
(about 25-30% of the non-wedge half), L1 the rest of the wedge.

### What a 10x leaner version of this slice looks like

Texture should be stateless per wake. The document is the state: a wake sees a
frame (document head, new records by reference) and nothing else, plus a one-line
"what I asked research for" ledger the runtime keeps from the cast records, so
carrying a conversation is never needed. Typical wake: 5k tokens, 2 calls. A research
report is a typed object (`claims[]`, `source_ids[]`) that the host renders into the
frame; Texture only decides prose. Records wake the document at most once per debounce
window, with all records since the cursor. Cell results return `{stdout}` and
nothing else unless an error occurred. Against today's 38k per wake and ~4 wakes per
research run, that is about 2 x 5k x 2 = 20k per story, versus about 150k now
(4 x 38k), a 7x cut of the Texture row (13% of the paper to ~2%), without touching
research or management. The 50x design would add: no model for feed/decline/dup
(Jev), research as parallel fixed-depth fan-out whose outputs are merged by a
reducer before Texture is woken at all, and Texture invoked once per story per publish.

## 4. Jev and classifier opportunities in this slice

| decision | today | rate | Jev shape |
|---|---|---|---|
| Does this capture continue story X, open a new one, or get declined? | management_task model run, 9.5 calls | ~43 tasks/h (86/2h) | batch of (capture summary, top-k story headlines) -> {feed:id, new, decline}; one judgment per capture, parallel, ~2k input each; routes deterministic cases without a model |
| Does this inbox record change the document? (new claim, new source, only restates) | Texture wake, ~38k tokens, always runs | ~155 record wakes/h | per record: (record claims, current block digests) -> {adds, restates, contradicts}; wake only on adds/contradicts. Cost 150/h x 2k = 0.3M tokens/h of Jev at $0.042/M, free of output |
| Is interim packet N materially new versus packet N-1? | not checked | ~2.6 packets x 38 research runs/h | same batch call, pre-wake |
| Do these two captures cover the same event? | model, inside intake | per intake pair | embed/Jev pair label; already partly there (`Labels`, `story.entities`) |
| Which pending updates matter now? (management backlog) | model reads whole backlog | per wake | Jev ranks; model reads top 5 |
| Disclosure gate | already Jev (`disclosure.decision`, `system1.judgment` events) | per publish | n/a, already done |

## 5. Open questions and probes

1. Size of the transcript's injected turns. Event data shows only a message count. Probe:
   read run memory entries of one carried Texture run (e.g. `ed6b755b`) and of the
   wedged management run `a11122b6` between two calls (needs a read-only memory
   route or the store file on d@e; no such public route exists, so use d@e). Settles
   W1's frame share and W4's packet-size inference (confirm the 41 update_refs
   reconstruction, and why the dedupe did not hold).
2. Is the same-hash re-injection (W4) a field mismatch, a compaction that drops
   earlier injection entries, or the append role? Probe: unit-run
   `lifecycleInjectionIDsFromRunMemory` on the dumped entries; check
   `trajectory_id` of the update vs `lifecycleControlTrajectoryForRun`.
3. How many Texture wakes produce a revision that changes the published story? I
   measured that 28 of 30 create a revision, not that the revision matters.
   Probe: diff consecutive revisions per document for a day (revisions are listed
   under `/api/texture/documents`); count wakes whose diff is under N characters of
   new claim text. This sets L3's quality risk.
4. Carried vs fresh: do carried runs produce better revisions? Probe on d@e: fork
   a story, run three wakes with carry on and with carry off, score with Jev
   support verdicts (`FrameFailure` counts) as the quality metric.
5. Prefix caching: crouton-alpha reports 0 cached tokens, so every resend is full
   price now. If caching returned (synthetic 87%, opencode-go 85%), L2 and L5 would
   shrink by the cached share; L3 and L5 (fewer calls) would not. Rank accordingly.
6. Jev accuracy for feed/new/decline routing: replay the last 500 intake decisions
   (the `Cast`/`Feed`/`Decline` calls are in the tool.result args) against Jev and
   compare. Needed before L5.

## 10-line summary

1. Wedge fix is the big one (51%); its communication half is L1: delivery state must be a durable id set, capped, and the packet must be count-only (5.7k tokens/call today for a 41-update backlog, E; recurred 74 of 94 calls, M).
2. Texture carried conversation is 46% of sampled Texture input (M, 18 of 30 runs start at 10-91 messages / 6.5k-66.7k tokens): about 6% of the paper. L2 (drop superseded frames/old results, or carry a checkpoint only) recovers 2-5% of the paper, small change, medium risk.
3. Interim ReportPackets cause about 4.1 Texture wakes per research run (313 wakes / 77 runs, 38k tokens each): L3 coalescing is 5-6% of the paper, small change.
4. One 1.4k-token research finding costs about 30k input tokens end to end (E): packet source resent in research, rendered in the frame, carried across wakes, and the capture read in full again by Texture.
5. Cell-result envelope boilerplate is 615-750 chars/cell, 15-22% of tool output chars (M): L4 slims it for about 2.6% of the paper (E), low risk.
6. management_task intake spends ~76k tokens per capture pair on a nearly fixed 6-cell routing script (86 runs per 2h, 7% of the paper): Jev routing (feed/new/decline) is L5, 4-5% of the paper, medium risk, needs the replay in section 5.6.
7. Texture "Records()" duplication does not exist (0 of 90 cells); owner-revision, disclosure and ask traffic is negligible on the paper.
8. Stateless-per-wake Texture with debounced record batches is the 10x design for this slice (about 7x on the Texture row, 13% to ~2% of the paper).
9. Jev can gate wakes ("does this record add anything to the document?", ~155/h at ~$0.00008 each) and routing; the disclosure gate already uses it.
10. Biggest unknowns: the real size of frames/packets in the transcript (need run-memory read) and the cause of the dedupe failure (field mismatch hypothesis); probes listed above.
