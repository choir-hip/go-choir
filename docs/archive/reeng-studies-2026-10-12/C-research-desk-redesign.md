# Research desk redesign: get to the bottom of things (2026-10-12)

Read-only design research against `main` a401c878. No repo file changed.
Inputs: the code below, `docs/archive/problems/research-desk-structural-flaw-root-cause-2026-10-11.md`,
`docs/precommitment-records.md`,
`docs/precommitment-records.md`,
`docs/archive/texture-directive-inbox-design-2026-10-11.md`. I did not read Node B
journals: the prior root-cause record already holds the gateway evidence, and
nothing here depends on new staging traces.

Labels: **[V]** verified in code at the cited line; **[I]** inference or
design judgment; **[H]** hypothesis that needs a staging trace.

Owner positions this design takes as given:

- the research RLM owns research; there is no runtime-owned job;
- no budgets and no round cap;
- research keeps going until returns diminish and reports progress through
  records;
- no claim quotas;
- epistemic status framing replaces "confirmed only";
- provider failures are normal;
- prompts describe the affordances that actually exist;
- intelligence lives in the models, not the runtime;
- no circuit breakers yet;
- search becomes async.

Owner correction (mid-task): a Cast states hypotheses and uncertainties, not
"find X, p=0.8". Section 5 covers it.

## Summary

**What is wrong.** The desk's prompts and runtime describe three different
desks:

- the prompt describes a desk that works in rounds and keeps going;
- the activation prompt describes a desk that must file a status report
  before it ends;
- the tool results describe the deleted typed-tool desk, which reports after
  every search with 2 to 4 claims.

The runtime implements a fourth desk:

- each activation is model-ended;
- the desk cannot settle its own work, because its reports are forced to
  `open`;
- it is restarted only by Texture, under a cap of 4.

The child researchers get the parent's whole prompt, including verbs they
cannot call.

**The redesign deletes more than it adds:**

- delete the `next_instruction` checkpoints and their dead gate;
- delete the forced status report, the 4-round cap, the per-activation egress
  call cap, the "degraded" wording, the "import to make it citable" rule, the
  retired verbs on the surface, and the child prompt inheritance;
- keep one act set: Search (async), Read, Report, Research (sub-RLMs), Ask,
  Reply and Note;
- add one ending: a Report with `work_disposition: completed` (the packet
  field already exists). The runtime must honour that field instead of
  overwriting it, which is a deletion at `rlm_reduce.go:2099`;
- if the model ends its turn while its thread is open, the run parks. It
  does not complete;
- a Texture Cast carries a belief state: hypotheses with probability,
  stakes and crux. Research reads depth from it, and its Reports move those
  hypotheses.

## 1. The research system prompt as actually assembled

Assembly order, from `systemPromptForRun` (`internal/agentcore/tool_profiles.go:196-315`) **[V]**:

1. Core prompt (`internal/promptstore/defaults/core.yaml:8-14`).
2. Temporal context (`internal/runtimeprompts/overlays/temporal_context.yaml`).
3. "Role-specific instructions": `internal/promptstore/defaults/research.yaml`
   (v6, lines 9-46).
4. No skill context. Research is excluded
   (`internal/agentcore/skill_context.go:48-56`).
5. `RLMResearchOverlay` (`internal/runtimeprompts/overlays/rlm_research_runtime.yaml:9-30`)
   (`tool_profiles.go:290-294`).
6. `DeskSurface("research","",readOnly=true)` (`tool_profiles.go:295-298`,
   `:569-574`). It is generated from the research module set
   (`internal/yaegikernel/choir.go:255-258`) and the docs
   (`internal/yaegikernel/surface.go:27-75`).
7. Run context (`internal/runtimeprompts/overlays/run_context.yaml`).
8. Tool catalog: `desk_go_eval` only (`tool_profiles.go:379-401`). Its
   description is at `internal/agentcore/tools_desk.go:261-265`.
9. The user turn is the activation prompt:
   `lifecycleControlActivationPromptForProfile`
   (`internal/agentcore/management_controller.go:1791-1797`) plus
   `buildAssignedWorkItemPromptForCarrier`
   (`internal/agentcore/runtime.go:2953-2999`).

Children (`choir.Research`) get `researchChildPreamble` plus the result of
`systemPromptForRun` on a research run record
(`internal/agentcore/research_children.go:27,201,213`) **[V]**. They therefore
get items 1 to 7 of the parent's prompt, including the parent's surface.

### 1.1 Instructions that are stale, false, rejected or hill-climbed

The table quotes each instruction and says what is wrong. Classes:

- **F**: false about affordances (violates the basic invariant);
- **S**: stale (deleted surface);
- **R**: structure the owner rejects (quota, required status, rounds,
  budget, "degraded");
- **H**: a hill-climbed patch.

| # | Where | Text (quoted) | Class | What is wrong |
|---|---|---|---|---|
| 1 | `internal/researchtools/researchtools.go:924` (every `choir.WebSearch` result) | "Submit a concise first update_coagent source packet from this search result before any additional search-only turn. Include 2-4 grounded claims …" | S, R, F | `update_coagent` is deleted. The gate `shouldRequireResearchUpdateAfterTool` (`:717-737`) looks for tool names `update_coagent`/`web_search` that never occur on the cell path, so it is true on every call **[V]**. It is also the source of the 2-to-4-claim quota. |
| 2 | `researchtools.go:787`, `:995`, `:1023` (fetch, import, read, source_search) | "…before any additional search/fetch turn. Use schema_version=…", "…whether a web_search is still needed." | S, R | Same gate, same dead verb. |
| 3 | `researchtools.go:922` (outage) | "Report a precise blocker from provider_health and attempts." | R | It turns a normal provider event into a stop-and-report. |
| 4 | `researchtools.go:883-893`, `:926-927` | `degraded=true` if *any* attempt is not `success` (cooling-down skips included); "one or more providers failed or were unavailable" | R | The degraded flag ignores whether the merged target was met. The gateway does the same: `internal/gateway/searchplane/router.go:85-104`, `:312-327` **[V]**. |
| 5 | `researchtools.go:913`, `:1017` | "full_evidence": "stored in Trace tool.result full_output/full_output_sha256" | F (likely) | On the cell path the full output goes *to the cell* inside the envelope (`ProjectionResultJSON`, `internal/toolregistry/toolregistry.go:260-269`; `HostEgress` returns it raw, `internal/researchtools/host_egress.go:22-32`). `serviceEgress` writes no trace (`internal/yaegikernel/desk_session_worker.go:414-427`). **[I]**: no trace write exists on this path. |
| 6 | `researchtools.go:664` | web_search description: "Research cadence: … then update_coagent on the next model turn…" | S | Dead text. The typed registry is not on the research desk (`tool_profiles.go:393-400`). |
| 7 | `runtime.go:2957` (activation prompt) | "Before ending this activation, use choir.Report to assert work_disposition=completed only when … otherwise use choir.Report with work_disposition=open and a precise blocker. Use choir.Cast to open assignments." | R, F | It forces a status report each activation. Texture rejected 5 of 11 such reports in demo19. It is also false: a research report's disposition is hard-set to `open` (`internal/agentcore/rlm_reduce.go:2099`; the `commitAddressedPacketIntent` path defaults it to open at `:1483-1485`) **[V]**, so research *cannot* complete its work. "Use choir.Cast to open assignments" contradicts overlay line 19 ("research does not cast work to other desks"). |
| 8 | `management_controller.go:1792` | "Continue assigned actor work. Process the coagent update packets in context." | S | Legacy vocabulary: "coagent update packets". |
| 9 | `runtime.go:2989-2996` | `step_budget=…`, `token_budget=…` printed into the work-item text | R | Budget framing shown to the model. Nothing in the research loop enforces it **[V]**: research has no `WithToolLoopBudget` (`runtime.go:3494-3500`). |
| 10 | `research.yaml:22-24` | "Work in rounds. After each round, send Texture everything you found with choir.ReportPacket, then keep going…" | R, F, H | Round thinking. "Keep going" is not implemented: the run ends at `end_turn` and only a Texture control restarts it (`admitLifecycleResearchProviderEntry`, `runtime.go:3007-3110`). |
| 11 | `research.yaml:18-21` | "Cover every open thread in each round: several searches per thread in one cell…" | H | v5 patch (2d093972). Round-shaped. |
| 12 | `research.yaml:39-40` | "In a packet, claims cite sources by source_id; import a page (choir.ImportURL) to make it citable." | F | The packet validator needs only `kind` + `target.uri` for a source and allows claims with no sources (`internal/coagentpacket/packet.go:186-226`) **[V]**. Any search-result URL is a valid `web_source`. The import rule is prompt-made, and it is the main reason only about 1% of results reached Texture. |
| 13 | `research.yaml:41-43` | "choir.Research runs child researchers in parallel and shares the computer's capacity, so use it only when a question splits into separate lines." | H | It discourages fan-out, which was used in 1 of 11 runs. |
| 14 | `research.yaml:44-46` | "Searches and fetches … have a budget and rate limits: when one is refused, narrow the query instead of repeating it." | R | Budget framing (64-call cap, below). |
| 15 | `research.yaml:36-37` | "Stop when another round would not change the picture, and say what remains open." | R (wording) | The intent is right (diminishing returns); "round" is wrong. Keep the idea. |
| 16 | Overlay `:18` | "Every network call charges the activation's egress budget … an exhausted budget refuses with an egress error, which is honest backpressure…" | R | Budget. |
| 17 | Overlay `:19` | "choir.Message and choir.Emit are retired, and research does not cast work to other desks — a lifecycle-bound researcher reaches its requester through Report…" | F | `Message`, `Emit` and `Cast` are still in research's module set (`choir.go:255`). `DeskSurface` lists them, and two of them only return an error (`surface.go:40-41`). The surface and the overlay contradict each other. Run context also says "Coordinate … (choir.ReportPacket, choir.Note, choir.Cast, choir.Ask)" (`run_context.yaml:17`). |
| 18 | Overlay `:25` | "Use choir.WebSearch and choir.FetchURL with parallelism appropriate to the task, novelty, and provider health." | F | Egress inside a cell is strictly serial. `egressRoundTrip` says "no concurrent frame traffic on this conn" (`internal/yaegikernel/sidecar.go:320-325`), and `sync` is not importable (`internal/yaegikernel/allowlist.go:10-27`) **[V]**. Today the only parallelism is `choir.Research`. **[I]**: goroutines calling verbs concurrently could interleave frames, because the read is not keyed by request id. |
| 19 | Overlay `:25` | "…distinguish final, partial, pending, scheduled, and blocked results." | H | A holdover taxonomy from the old evidence doctrine. |
| 20 | Overlay `:14` | "Send at most one desk_go_eval call per turn…" | OK | A true affordance (batching is refused). Keep. |
| 21 | Overlay `:20` | the full `coagent_source_packet.v1` literal | OK, but heavy | True. It belongs in `choir.Help("ReportPacket")`, not the system prompt. |
| 22 | `tools_desk.go:264` | "Host calls such as choir.WebSearch and choir.FetchURL take several seconds each (up to about 10 s)." | F | The guest search client times out at 30 s (`internal/search/search.go:85`). Demo19 had six searches at 29 to 30 s. |
| 23 | Child preamble `research_children.go:27` plus the inherited parent prompt | "You cannot fan out further, report, message or write" vs inherited `research.yaml:22-24` "send Texture everything … with choir.ReportPacket", plus the parent `DeskSurface` (Report, ReportPacket, Research, Cast, PollSource…) | F | A child's real module set is 11 read verbs (`choir.go:265-267`). Its prompt lists about 29. Children also inherit `next_instruction`, because they run under the root's RunID (`research_children.go:187`). |
| 24 | `research_children.go:238` | "You have about %s for this objective; findings you have not returned by then are lost." | R | A time budget, from `researchCallWindow` 10 min × 0.75 (`:44-46`). |
| 25 | `coagentpacket/packet.go:118-160` | Validation errors say "update_coagent …" | S | The cell sees a retired verb name in its refusals. |
| 26 | `surface.go:59` | `SearchWireCorpus`: "Searches the owner's published corpus." | S (naming) | "Wire" is a retired product name (CLAUDE.md). |
| 27 | Texture side: `tool_profiles.go:252` | "Each owner request … allows a bounded number of research rounds; the runtime drops further rounds…, so finish with the evidence in hand." | R | This is Texture's model of research. It must change with the cap. |
| 28 | Texture side: `internal/textureprompts/overlays/run_system.yaml:31` | "It works in rounds and reports after each; it keeps working after a report. … Research rounds per owner request are bounded…" | F, R | "Keeps working after a report" is false today (row 10). |

Cleanup signal: research prompt edits took 11 commits in 2 days (2026-10-10
and 10-11; `git log` on the two prompt files), from v1 to v6 plus overlay
edits. That is hill-climbing over a runtime that contradicts the prompt.

## 2. Runtime-side rules that shape research behaviour

| Rule | Where | Effect | Belongs in | Recommendation |
|---|---|---|---|---|
| 4 research rounds per owner request; Cast, Ask **and Note** each count | `internal/textureowner/texture_turn_runtime.go:161`, `:296-306`, note `:405`; every research dispatch folds to `open_researcher:true` (`internal/agentcore/texture_dispatch_fold.go:98-120`) | Hard stop on research. Steering Notes spend rounds. | Model (Texture decides when to ask; research decides when it is done) | **Delete**. The never-idle loop it guarded (texture-research-loop-never-idles-2026-10-09) goes away when research continues on its own, so Texture stops re-asking for continuation. |
| Opener merge (one research opener per turn) | `texture_turn_runtime.go:209-234` | One desk per document | Substrate (PD1 addressing) | Keep. |
| Research must be restarted by an exact Texture-control fingerprint | `runtime.go:3007-3110` | Research cannot continue its own work | Substrate authority (who may make provider calls) | Keep the authority, but let a **parked** research run resume on any record addressed to it (Ask, Note, child report). See §4. |
| No terminal or passivating result for research; `end_turn` completes the run | `runtime.go:3468-3521` (only engineering has `WithTerminalToolResult`; Texture has `WithPassivatingToolResult`) | The activation ends after the first report | Substrate | **Add**: a completed Report is terminal; `end_turn` with the thread still open passivates (parks). |
| Research reports forced to `work_disposition=open` | `rlm_reduce.go:2099` (record-native path); `:1483-1485` (envelope path never copies the packet field) | Research cannot settle its work. This is the "research work never settles" issue in `docs/ACTIVE.md:115-116`. | Model (research decides it is done) | **Delete** the override. Pass `payload.WorkDisposition` through. Texture's turn already settles `completed` (`texture_turn_runtime.go:116-131`). |
| Forced status report in the activation prompt | `runtime.go:2957`, `management_controller.go:1792` | Status-only packets that Texture rejects | Model | **Delete**. |
| `next_instruction` checkpoints and the dead gate | `researchtools.go:717-737`, `:784-788`, `:922-924`, `:995`, `:1023` | Report after every search, 2 to 4 claims | Nothing (dead) | **Delete** all of it. |
| Egress cap: 64 calls and 32 MiB per activation; children charge the root | `internal/researchtools/egress_budget.go:46-47`, wired at `tool_profiles.go:478`; `research_children.go:187`, `:241-250` | Caps breadth (the baseline used about 250 to 300 calls) | Owner: no budgets | **Delete** the call and activation-byte caps. Keep the per-response byte caps in the tools (256 KiB fetch, 4 MiB decode) as memory safety. The ledger map is never pruned (`egress_budget.go:63-73`), another reason to remove it. |
| "degraded" when any provider fails | `router.go:85-104`, `:312-327`; `researchtools.go:883-893` | The desk blames providers and stops | Substrate signal, wrong semantics | **Replace** with `providers: "M of N answered"`. Set `degraded` only when the merged count is below target (or 0 = outage, a fact field with no instruction). |
| Envelope returned to the cell (`durable_output` + `model_output`) | `host_egress.go:22-32` | Doubles the bytes. `durable_output` sorts first. | Substrate | **Return** `model_output` (or a typed Go struct) only. |
| Packet schema validation | `coagentpacket/packet.go:118-160` | Shape only: no source requirement, no claim count | Substrate | Keep. Rename error strings away from `update_coagent`. |
| Texture rejecting sourceless or status-only packets | `textureTurnPendingInbound`; dispositions are model-named (`texture_turn_runtime.go:107-131`) | 5 of 11 updates rejected | Model (Texture prompt) | Fix the Texture prompt: leads and inferences with epistemic status are content. Status-only packets disappear once rule 6 is gone. |
| Child concurrency 4 (queues, never refuses) | `research_children.go:23` | Memory protection | Substrate | Keep. |
| Child model-call spacer, 1.5 s computer-wide | `research_children.go:36` | Keeps under the gateway's 60 calls/min | Substrate | Keep. |
| Child window 10 min × 0.75; children cannot fan out | `research_children.go:44-46`, `:226-239`; `choir.go:265-267` | A time budget, because the parent cell blocks | Substrate today (a blocking cell needs a bound); the no-recursion rule is the owner's (PD1) | Keep for stage 1. Replace with async sub-RLMs that report through records (stage 3). |
| Children leave no trace: `noEmit` | `research_children.go:210`, `:215`; only a log line at `:221` | Child work cannot be reviewed from the ledger | Substrate defect | **Fix** (stage 3): child cells must emit run events. This breaks CLAUDE.md's "review the traces". |
| Activation budget 60 min (all desks) | `internal/provideriface/config.go:24`; `runtime.go:374-399` | The wall clock of one activation | Substrate | Keep. A long research thread spans several activations (park and resume). The deadline should not be what ends research. **[I]** At the deadline, research should park, not fail. Owner call; see §4.5. |
| Guest search timeout 30 s (equals the gateway provider timeout) | `search.go:85` | Spurious guest timeouts | Substrate | Superseded by async search (separate study). |
| Desk session retention 30 min, 12 idle | `tools_desk.go:44-53` | REPL variables die after 30 min idle | Substrate | Keep. The prompt must say the session can be new. The first cell already says so. |
| Cell output caps: 48,000 characters per value, 100 KB per tool result | `internal/yaegikernel/session_loop.go:191`; `batch_executor.go:321` | Truncation | Substrate | Keep. Truncation must be visible in the result. |
| One cell per turn | Overlay `:14` | — | Substrate | Keep. |
| Model `gpt-5.6-luna`, effort `low` | `internal/modelpolicy/model_policy.go:340-343` | Possible shallowness **[H]** | Config | Separate experiment after stage 2 (problem record, next observation 3). |
| No tool-loop turn or token budget for research | `runtime.go:3494-3500` | — | — | Matches the owner. Keep it that way. |

## 3. Sub-RLMs today

**[V]** throughout, from `research_children.go` and `tools_desk.go:322-323`.

- **Spawn.** A research cell calls `choir.Research(children)`, where children
  are an objective, a list, or `{objective, model}` maps
  (`internal/yaegikernel/choir.go:488-520`). That becomes `ActionResearch`
  and then `runResearchChildren` (`research_children.go:132-169`).
- **Execution.**
  - Each child runs its own `RunToolLoop` with a fresh worker pool
    (`:194`) and a `research_child` session.
  - It has no run record and no channel. Events go to `noEmit`.
  - Its RunID is the root's, used for the egress budget (`:181-187`).
  - Its model is inherited, or picked by role name or model id from this
    computer's policy (`:257-284`).
  - At most 4 run at once (`:151-166`). Every child model call waits on the
    1.5 s computer-wide spacer.
- **Scope.** Read-only world verbs only: search, fetch, source search,
  import, read content, the wire corpus, read and list evidence (`choir.go:265-267`).
  Children cannot report, message, cast or call `choir.Research`.
- **Return.** The parent cell blocks until every child finishes or the
  window closes (at most 10 minutes; each child gets 75% of the remaining
  time). It gets back
  `[]ResearchChildResult{Objective, Model, Findings (final text), Error, DurationMs}`.
  Findings are free prose. Nothing reaches Texture until the parent reports.
- **Fan-out in practice.** Possible: real parallel readers on separate
  objectives. Used in 1 of 11 demo runs. The prompt discourages it, the
  parent can report nothing while blocked, and the 64-call cap is shared
  across the tree.
- **Parallel readers on sources.** Only as children. A cell cannot fetch in
  parallel (§1.1 row 18).

## 4. Proposed redesign

### 4.1 Principles

**[I]** The desk is a researcher with a question it cares about. The runtime
provides five things:

- a REPL;
- async search;
- page reading into captures;
- sub-RLMs;
- one way to say something to Texture (a Report record).

It decides nothing about the desk's process. Records carry progress.

### 4.2 Minimal research surface (the module set)

| Verb | Shape | Notes |
|---|---|---|
| `choir.Search(query string, opts any) SearchHandle` | Async. It returns at once. Results land in REPL state (for example `choir.Results(h)` or a session-held map) as they arrive, with `providers: "M of N answered"`. | Shape owned by the async-search study. It replaces `WebSearch` and folds in `SourceSearch` as a provider. |
| `choir.Read(url string, opts any) Capture` | Fetch and capture in one call. Returns the capture id, title, dates, URL, and text from `offset` with up to `maxChars`. | It replaces `FetchURL`, `ImportURL`, `ImportDocument`, `ReadContentItem`, `ListContentSelectors` and `ReadContentSelector`. It reuses the W1 `web_capture` store, so `ReadCapture(id, offset)` pages further. **[I]** Whether `Read` is also async follows from the search study. |
| `choir.Report("texture", packet)` | The one way to tell Texture something. Each claim carries its epistemic status (§5.3). It may carry `work_disposition: "completed"` and a `stop_reason`, and it may update hypotheses (§5). | It replaces `Report` + `ReportPacket`. Packet field docs move to `choir.Help("Report")`. |
| `choir.Research(children)` | Sub-RLMs. Stage 1: as today. Stage 3: async (see 4.6). | — |
| `choir.Ask` / `choir.Reply` / `choir.Note` to Texture | Conversation with the caster | Kept. Ask needs a prediction under P3. |
| `choir.Help`, `choir.Context`, `choir.Updates`, `choir.Pack` | Observation | Kept. |

Removed from research's module set (`choir.go:255-258`):

- `Message` and `Emit` (retired);
- `Cast` (research casts to no desk; sub-RLMs are `Research`);
- `Escalate`, `CancelAct`, `Precommit`, `Resolve` and `Disagreement`: keep
  only those the protocol needs from research. **[I]** Hypothesis updates
  ride Report (§5), so standalone `Precommit` and `Resolve` are not needed
  on this desk;
- `SaveEvidence`, `ReadEvidence`, `ListEvidence` and `RunMemoryEntry`, the
  legacy evidence store. Captures replace them;
- `SearchWireCorpus` (rename it, or move it to Autopaper's surface);
- `PollSource` (management and Autopaper's job).

Each deletion needs a citer check (standing questions).

### 4.3 The prompt core (replaces `research.yaml` body and most of the overlay)

About 1,500 characters, against about 8,300 today for `research.yaml` plus
the overlay. Draft:

> You are the research desk for this document. You find out what is true
> and what is likely, and you keep going until more work would not change
> what anyone should believe.
>
> Texture's Cast says what it believes, how sure it is, what would change
> its mind and which uncertainties matter most. Work hardest where the
> uncertainty is widest and matters most. If the Cast says there may be
> important things it has not mentioned, look beyond its framing too, and
> mark what you find there as outside the framing.
>
> Search widely. Searches run in the background: start several, then read
> results as they arrive. Read the sources that matter. A snippet is a
> lead, not evidence.
>
> Tell Texture what you learn as you learn it, with choir.Report. Include
> what is verified and what is not but matters: leads, rumours, inferences
> and your own estimates. Give each claim its status (confirmed, reported,
> inferred, speculative or refuted), who says it, and how likely you think
> it is. When something changes a hypothesis in the Cast, say how: it
> confirms, qualifies or contradicts it, or it raises a new one.
>
> Some search providers are always down. "3 of 5 answered" is normal; work
> with what came back.
>
> When further work stops moving any hypothesis that matters, send a final
> Report with work_disposition "completed". Say why you stopped and what
> remains open.
>
> Your Go session usually continues between runs, but it may be new. The
> first cell says which.

The rest is generated surface (`DeskSurface`) and run context. The child
prompt is its own preamble plus the *child* surface (`DeskSurface("research_child", …)`),
not the parent's prompt.

### 4.4 How the desk decides it is done

**[I]** In the model, stated in its final Report. The prompt gives one
criterion: further work stops moving any material hypothesis. The model
judges it from its own REPL state, for example:

- new distinct sources per search, falling;
- hypothesis probabilities that stopped moving;
- the cruxes named in the Cast are answered or shown unanswerable from
  public sources.

The final Report carries:

- `work_disposition: "completed"`;
- `stop_reason` (one or two sentences);
- remaining open hypotheses with their current probabilities.

That makes "diminishing returns" a scored claim: Texture or the owner can
re-Cast with a follow-up hypothesis, and the record shows whether
re-opening found more. P4 scoring can later measure premature stops. No
runtime stop rule exists.

### 4.5 Staying alive without a cap: the thread is the unit

**[I]** design; the existing machinery is cited.

- **Open.** Texture's first Cast to `research:<doc>` opens the research
  work item, as today (P1/P2a).
- **Continue.** Later Casts, Asks or Notes from Texture join it, as today,
  but no longer count against a cap.
- **Run lifecycle.** A research activation runs cells until one of these
  happens:
  - a Report with `work_disposition=completed` commits. It is terminal,
    like engineering's `fate_terminal` (`runtime.go:3468-3474`); the work
    item settles through Texture's turn (`texture_turn_runtime.go:116-131`);
  - the model ends its turn without that. The run **passivates**: the thread
    stays open and idle, as Texture's `WithPassivatingToolResult`
    (`runtime.go:3509-3514`) and management's park waiter
    (`management_controller.go:2714-2770`) already do. Texture's pack shows
    "research idle, thread open". A new record addressed to the desk (a
    Texture Ask or Note, or later a child or async-search arrival) resumes
    the same run;
  - the activation budget deadline arrives. The same as passivation.
    **[I]**: this needs `HandleActivationBudgetDeadline`
    (`internal/agentcore/continuation_schedule.go:408-411`) to park
    lifecycle research instead of failing it. Owner call (open decision 3).
- **Optional guard.** There is a risk the model ends its turn early "by
  habit" and parks with work left. **[I]** Mitigation, as model-side as
  possible:
  - the prompt says an idle open thread is idle research;
  - one completion-guard turn on `end_turn` while open, which states the
    fact ("Your research thread is still open; continue, or close it with
    a final Report") and parks on the second `end_turn`.

  This is a statement of state, not a budget. Engineering has the same
  guard (`runtime.go:3476-3492`). Whether to include it is open decision 2.
  It is not a circuit breaker.
- **Texture's side.**
  - Delete the round cap and its prompt text (`tool_profiles.go:252`,
    `run_system.yaml:31`).
  - Texture asks research new questions; it never asks it to "continue".
  - Report wakes stay batched by the consume-at-commit default:
    `textureTurnPendingInbound` folds every pending report up to the
    scheduled sequence into one turn (`texture_turn_runtime.go:84-152`).
    So frequent research Reports do not mean one Texture turn per report.
    **[H]**: Texture turn rate under frequent reports must be measured.

**Crash interplay.**

- **A crash restart closes the thread.** Pre-boot work occurrences are
  refused as `interrupted_by_restart` (`internal/actorruntime/handler.go:84-112`,
  `restartWorkKinds`). The REPL session is process memory and is gone
  (`tools_desk.go:44-49`).
- **Requirement:** at a crash boot, an open research work item, and any
  parked research run, is closed with the visible fate "interrupted by a
  restart", and Texture sees it. **[I]** Today the work item probably stays
  `open` with nothing to wake it. That needs a staging check before the
  stage 2 design is final.
- **A planned restart resumes once.** The marker is consumed: one resume
  per planned restart (CLAUDE.md). The resumed run has its transcript
  memory but no Go variables, and its first cell says the session is new.
- **No boot-time resumer** without the marker.

### 4.6 Fan-out

- **Stage 1.** Keep `choir.Research` and remove the discouraging prompt
  line. Children get a truthful prompt and surface.
- **Stage 3**, after async search lands. Sub-RLMs become async, like
  search: `choir.Research` returns handles, and findings arrive into REPL
  state. Each child also emits run events, so it is traceable. A child
  returns structured hypothesis updates (the §5.3 shape), not prose. The
  parent reduces and Reports. **[I]** Children should not report to Texture
  directly: the parent owns the belief state. Keep one level of fan-out
  (owner, PD1).

### 4.7 What to delete (consolidated)

1. `next_instruction`, `shouldRequireResearchUpdateAfterTool`,
   `addResearchUpdateCheckpointRequirement` and `latestSuccessfulResearchToolSeq`
   (`researchtools.go:717-788`, `:922-924`, `:995`, `:1023`), and the
   `requireFindingsCheckpoint` parameters.
2. The `update_coagent` text in `researchtools.go:664` and in
   `coagentpacket` error strings.
3. The forced-report clause in `runtime.go:2957`, plus `:2961-2965`, and the
   "coagent update packets" line (`management_controller.go:1792`).
4. `WorkDisposition: types.WorkItemOpen` hard-set (`rlm_reduce.go:2099`);
   pass the packet's value through.
5. `textureResearchRoundsPerOwnerRequest`, `researchRoundsSinceOwnerInput`,
   the drop note (`texture_turn_runtime.go:155-161`, `:181-199`, `:296-306`,
   `:405`), and the Texture prompt sentences about bounded rounds.
6. The egress call and byte ledger for research (`egress_budget.go`, the
   wiring at `tool_profiles.go:478`), keeping per-response caps.
7. The "degraded" wording and any-failure semantics in the projection
   (`researchtools.go:883-893`, `:926-927`) and the router flag semantics.
8. `Message`, `Emit`, `Cast` and the evidence and poll verbs from research's
   module set. Retired verbs are not listed on any surface.
9. Child prompt inheritance (`research_children.go:201`, `:213`).
10. Most of `research.yaml` and `rlm_research_runtime.yaml`, replaced by §4.3.
11. Stale comments saying children can call `choir.Research` (`choir.go:80-82`,
    `research_children.go:127-129`).

## 5. Owner correction: a Cast states hypotheses, not a target

### 5.1 What exists today

**[V]**

- `types.CommitmentPrediction` already has this shape
  (`internal/types/commitment.go:115-122`): `Hypothesis`, `Assumptions`,
  `Alternatives`, and `Questions []TypedQuestion`. Each question has
  `Answers`, `Probabilities`, and a materiality `Weight` (`:75-83`, "likelihood
  x impact x relevance").
- `CommitmentRevision{Weakened, Strengthened, Hypothesis}` (`:162-167`) is the
  post-outcome update.
- `DiscrepancyClass` has `confirmed`, `qualified`, `contradicted`,
  `answered` and `unresolved` (`:15-30`).
- But a **Cast** record today fills no prediction at all. It is a
  `Directive{Subtype: cast, Body, Objective}` (`rlm_reduce.go:1135-1137`). An
  Ask fills `Hypothesis = question` with an empty distribution
  (`:1111-1124`).
- The P3 proposal, `CommitmentExpectation` / `ExpectQuestion`
  (`docs/precommitment-records.md` §3.1;
  protocol design §4 `Expectation{Questions, By, Subject, Resolver}`), points
  the other way. It holds 1 to 4 yes/no count predicates with `p` and a
  deadline `By`: "Mistral's own site confirms Large 4, p=0.55", "≥2 new
  sources, p=0.6", defaulting research to 45 minutes.

That is exactly the "find X; I expect a primary source, p=0.8" shape the
owner rejects. It scores the *search process* (hosts, source counts), not the
*belief*. Its `By` is a time budget by another name.

### 5.2 The Cast shape (a belief state)

**[I]** Proposed. It reuses `CommitmentPrediction` rather than adding a
parallel `Expectation` type.

```go
// On every Cast or Ask that asks for work: the caster's current belief.
type CommitmentPrediction struct {
    Hypothesis   string              // one-line summary of what the caster believes overall (kept)
    Hypotheses   []BeliefHypothesis  // NEW: the itemized belief state
    Uncertainties []OpenUncertainty  // NEW: what the caster does not know and cares about
    Unknowns     *CatchAll           // NEW: "important things I haven't mentioned" (§5.6)
    Assumptions  []string            // kept
    Alternatives []string            // kept (rival explanations)
    Questions    []TypedQuestion     // kept for machine-countable side questions; optional
    CommittedAt  string
}

type BeliefHypothesis struct {
    ID       string  // h1..hn, stamped at stage time
    Claim    string  // "Anthropic released Claude Haiku 5.5 this week"
    P        float64 // caster's probability, in [0.02, 0.98]
    Stakes   string  // why it matters to the document: "lead story if true"
    Weight   float64 // materiality 0..1 (same scale as TypedQuestion.Weight)
    Crux     string  // what evidence would change the caster's mind
    Basis    string  // what the belief rests on now ("one outlet", "model prior")
}

type OpenUncertainty struct {
    ID       string  // u1..un
    Question string  // "Which chip deals were announced this week?" (no hypothesis yet)
    Weight   float64 // how much the answer matters
}
```

What changes from the P3 draft:

- **Drop `By`** as a required field (owner: no budgets). Keep an optional
  `UsefulUntil` only when the stakes are time-bound ("for tomorrow's
  edition"). That is a fact about value, not a stop rule.
- **Drop the 1-to-4 question cap.** The belief state is as large as the
  question.
- **Drop mandatory count predicates.** Count questions stay optional, for
  side facts the runtime can count. They are not the hypothesis.
- **Keep P3's "mandatory" rule**, applied to the belief state: a Cast or
  Ask must carry at least one hypothesis *or* one open uncertainty, each
  with a weight. Hypotheses also need `P`.

An example Texture Cast for the AI news demo:

- h1: "Anthropic shipped Haiku 5.5 on Oct 7". P 0.85. Weight 0.9. Crux:
  "Anthropic's own post or release notes". Basis: "two outlets".
- h2: "Mistral Large 4 is released, not just previewed". P 0.5. Weight 0.6.
  Crux: "Mistral's site or API listing".
- u1: "What else happened in chips, funding and policy this week?". Weight
  0.8.
- unknowns: P 0.3, scope "non-US labs, open-weights releases", Threshold
  0.2 (§5.6).

### 5.3 How research reads it: depth from uncertainty × stakes

**[I]**

- **Effort.** Research's effort on an item is roughly `Weight × closeness of
  P to 0.5`, and for open uncertainties `Weight` alone. Breadth comes first,
  because open uncertainties have no hypothesis yet.
- **Priority.** A crux tells research what to look for first.
  - h1 at 0.85 with a stated crux: one primary-source check, then move on.
  - h2 at 0.5 with weight 0.6: work it until it moves.
  - u1 at weight 0.8 and no hypothesis: broad async searches. Research
    forms its own hypotheses and reports them as `new`.
- **The stopping rule is the same quantity.** Returns diminish when recent
  work moves no high-weight item's probability, and no open uncertainty is
  yielding new hypotheses. This is stated in the prompt (§4.3), not
  computed by the runtime.

### 5.4 How Reports update hypotheses

**[I]** Report packet additions. The claim-level fields are additive to
`types.CoagentPacketClaim` (`internal/types/evidence.go:75-81`) or a new
`updates[]` list:

```go
type HypothesisUpdate struct {
    Ref      string  // "h2" on the Cast record, or "new"
    Claim    string  // restated or new hypothesis text
    Verdict  string  // confirms | qualifies | contradicts | new | unresolved
    P        float64 // research's current probability
    Status   string  // confirmed | reported | inferred | speculative | refuted
    Who      string  // who says it (outlet, primary, research's own inference)
    SourceIDs []string
    Why      string  // one line: what moved it
}
```

- **Interim Reports** carry updates as they happen. That is "report every
  step" in record terms: every step that changes a belief is a record. A
  step that only launches searches needs no report.
- **The final Report** resolves the Cast. The runtime maps each referenced
  hypothesis to a `DiscrepancyClass` (confirms→confirmed,
  qualifies→qualified, contradicts→contradicted, unresolved→unresolved) and
  writes a `CommitmentRevision`: Weakened and Strengthened from the verdicts,
  and Hypothesis = research's revised summary.
- **Scoring (P4)** then has a real learning target: the caster's prior `P`
  against the resolved verdict. That is Texture's calibration per subject.
  Research's interim `P` against its own final verdict is research's
  calibration. Neither needs the scorer before P4.
- **Status** carries the owner's epistemic framing (rumour = `reported` with
  low P; inference = `inferred` with `Why`). Texture's prompt already
  renders "an unconfirmed lead appears once, with what it claims, who
  reports it and how likely it is" (`run_system.yaml:10`).

### 5.6 The catch-all hypothesis: "there are important things I haven't mentioned"

Owner addition. **[I]** design throughout.

Every Cast may carry one catch-all. The default is to carry it. Leaving it
out is equivalent to P≈0, which is a strong and scoreable claim:

```go
type CatchAll struct {
    P         float64 // caster's probability that an important finding exists outside Hypotheses/Uncertainties, in [0.02, 0.98]
    Scope     string  // optional: where the caster suspects blind spots ("policy", "non-English sources")
    Threshold float64 // the probability-shift delta that counts as "important" for this Cast (default 0.2)
}
```

#### A computable meaning of "important"

A finding is important when it is outside the Cast's listed hypotheses and
uncertainties, and at least one of these is true:

1. **Structure change.** The caster's next revision after the finding
   changes the document's lede or top-level structure *because of* it:
   - a new or removed top-level heading, or the first block rewritten;
   - and the revision cites a source from that finding.

   The structural part is countable from revision diffs
   (`texturedoc` outline before and after). The "because of" part is the
   cited source id.
2. **Belief shift.** The finding moves a stated hypothesis's probability by
   at least `Threshold`. The caster must re-state the probability in its
   next Cast or Ask, or in a Resolve, which can be required on the
   resolving turn. Or the finding creates a new hypothesis the caster then
   adopts with P≥0.5.
3. **Third-party judgment.** When neither count fires but the finding is
   plausibly material, a judge other than research decides. The judge sees
   the Cast, the finding and the revision, and answers a single yes/no
   question with a stated criterion.
   - The judge is the caster (Texture), management, or Jev, as P4 settles
     the scorer.
   - Never research.

Research only *nominates* candidates. In its Reports it tags a finding
`outside_framing: true` and links it to `unknowns`. It cannot score the
catch-all. Research is paid in calibration only through its own Reports,
so it gains nothing by inflating nominations. Over-nominating costs it
precision on the "outside_framing" tag. That precision is itself countable:
the share of nominations judged important.

#### Resolution

The catch-all resolves `yes` if at least one nominated finding meets the
definition above by the time the thread closes. It resolves `no` if none
does. The scored pair is the caster's `P` and the outcome. It also resolves
`yes` when a *later* Cast on the same document, within the same owner
request, names a hypothesis that research had reported as `new` and that
then met the definition. This catches importance that shows up one turn
late.

#### How it drives exploration

- **Permission.** A catch-all with non-trivial P tells research that looking
  beyond the framing is part of the job, not scope creep. Today's prompt and
  Texture's packet both narrow research to the listed questions ("Texture
  asked you to confirm"; `research.yaml:20-21` patched this by hand).
- **Pressure.** Research allocates breadth effort in proportion to
  `CatchAll.P`, as with any open uncertainty (§5.3). At P=0.3 it runs a
  share of its searches with deliberately off-frame queries: adjacent
  topics, other languages, the named organisations' own sites, "what else
  happened". The final Report states what it did for the catch-all and what
  it found.
- **Stopping.** Off-frame searches that keep returning nothing new are
  evidence that the catch-all is closing. That feeds the same
  diminishing-returns judgment.

#### What its calibration measures

Over many Casts, compare the caster's catch-all P with how often it
resolved yes. This measures **how well the caster knows what it does not
know**:

- A caster whose catch-alls at 0.1 resolve yes 40% of the time is framing
  too narrowly and is overconfident about its own framing. Its future
  catch-all P should rise, and its packs (P4) should say so.
- A caster that always says 0.5 and is right 10% of the time is wasting
  research effort.

This is separate from the per-hypothesis calibration in §5.4: one scores
beliefs inside the frame, the other scores the frame. It is per desk, model
and subject, for example Texture on AI-news documents.

#### Guards

- The specificity guard from the P4 design (§4.5 of the scoring doc)
  applies: a constant catch-all P scores no better than its base rate.
- **Goodhart risk.** Texture could suppress "important" by never changing
  structure. The third-party judge (path 3) and the count of `new`
  hypotheses research reported cover this.

### 5.7 Under async search

**[I]** The loop the prompt implies:

1. Fire searches for every high-weight item and open uncertainty.
2. Read results as they land in REPL state.
3. Read the sources that bear on cruxes.
4. Report updates.
5. Fire follow-ups suggested by what was read (names, primary sites,
   dates). This is the recursion the owner asked for, with no agent tree.
6. Repeat until nothing material moves.

Breadth no longer costs serial latency, so nothing pushes research to stop
early.

## 6. Staged plan

Each stage starts with a problem or mission doc update (Problem
Documentation First). The acceptance environment is staging,
`https://choir.news`, on a fresh test computer (idle computers keep old
builds). The baseline is demo19 and demo20: 13 and 15 claims, 10 and 11
sources, 5 of 11 and 5 of 10 updates rejected, 67 to 126 s per activation,
Texture-started rounds only.

### Stage 1: truthful surface, no stale pressure

Mutation class: **orange** (tool output and prompts; no lifecycle change).

- Delete items 1, 2, 3, 7 (projection wording), 8, 9, 10 and 11 from §4.7.
- Return `model_output` only to the cell.
- Correct the timeout text.
- Make the child prompt and surface truthful.
- Add the epistemic `status`/`p` fields as text conventions first (no
  schema change).

**Rollback:** git revert. Prompts are seeded defaults, so check for
per-user overrides in the prompt store.

**Acceptance:**

- one demo run with the cell trace captured (problem record, next
  observation 1);
- zero `next_instruction` in cell results;
- a report carries more than 4 claims, or carries leads with status;
- research runs more than one cell after its first report;
- no "degraded" except below target;
- `go test ./internal/researchtools ./internal/yaegikernel` plus a focused
  `./internal/agentcore -run Research`.

**Note.** The problem record recommended landing the `next_instruction`
deletion with the loop change. I split it out because it is pure deletion of
dead pressure, and its effect is measurable on its own: the "report after
the first search" hypothesis (cause 3) gets confirmed or refuted by one
trace. The owner may prefer to bundle it with stage 2.

### Stage 2: the thread is the unit

Mutation class: **red**.

Protected surfaces: run lifecycle and acceptance, the Texture control path,
the evidence ledger.

- Honour `work_disposition=completed` on research Reports (delete the
  override at `rlm_reduce.go:2099`).
- A completed Report is terminal; `end_turn` while open passivates.
- Parked research resumes on records addressed to it.
- Delete the round cap and the Texture round prompts.
- Delete the egress call and byte cap.
- Crash boot closes open research work as "interrupted by a restart".
- Optional: the one-turn state guard (open decision 2).

**Rollback:** git revert. Drain parked research runs on rollback; they hold
no packets that would re-arm.

**Heresy delta:**

- *repaired:* research cannot settle its own work; prompt-runtime mismatch
  about continuing;
- *discovered:* (this report) reports forced to `open`; children leave no
  trace; the cell-parallelism claim is false;
- *introduced:* none intended; the risk is a research thread that never
  closes (residual, no breaker by owner decision).

**Acceptance (staging demo, AI news):**

- one Texture Cast;
- research makes more than one Report before its final;
- the final Report has `stop_reason`;
- Texture sends no continuation-only controls (doc events);
- the work item settles `completed`;
- distinct sources and claims at or above the demo baseline (target: the
  Haiku baseline's 26 merged items as the realism axis);
- research duty cycle more than 50% of active time;
- a staging crash restart of the test computer mid-research shows the
  thread closed as interrupted, with no resume.

### Stage 3: hypotheses and async sub-RLMs

Mutation class: **red**.

Protected surfaces: record schema, delivery, the scoring substrate.

Preconditions: P3 and the async-search station land, and the owner settles
the design in §5.

- Change the P3 Expectation design to the belief state in §5.2.
- Add the Report updates in §5.4 and the runtime mapping from verdicts to
  Discrepancy and Revision.
- Add the catch-all (§5.6): the structure-change count from outline
  diffs, the re-stated-probability requirement on resolving turns, and the
  third-party judge question.
- Make sub-RLMs async and traceable.

**Acceptance:**

- the demo's Cast carries hypotheses and uncertainties;
- every final-Report hypothesis resolves to a discrepancy class;
- a calibration view shows Texture's priors against the outcomes;
- the catch-all resolves on every closed thread, and at least one demo
  shows an off-frame finding nominated and judged;
- child cells appear in run events.

## 7. Residual risks and open decisions

1. **Texture turn churn** under frequent research Reports **[H]**. Batching
   exists, but Texture's turn rate must be measured in stage 2. The
   directive-inbox work (P2d2) touches the same consumption path.
2. **Guard or no guard** on `end_turn` while open (§4.5). The model-side
   purist answer is no guard. The pragmatic answer is one factual reminder.
3. **Activation deadline**: park or fail lifecycle research at 60 minutes.
4. **Crash close of open research work items.** Today's behaviour must be
   confirmed on staging before stage 2 is final **[I]**.
5. **Low effort.** `gpt-5.6-luna` at `low` may cap depth regardless of
   structure. Test after stage 2 **[H]**.
6. **A thread that never closes.** It costs model calls until the activation
   deadline and the gateway rate limit stop it. The owner deferred circuit
   breakers. Owner Cancel remains the stop.
