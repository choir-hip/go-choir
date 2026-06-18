# Mission: Texture As One Durable Deep-Research Thread v0

> Hard-cutover rewrite (2026-06-18). This supersedes the earlier framing of this
> same mission (a wake-driven, one-write-per-run actor patched with park/rewarm
> and per-prompt classifiers). The full overnight construct/falsification trail
> for that framing is preserved in
> `docs/mission-texture-long-running-agent-v0.ledger.md` as historical evidence;
> do not treat it as the current target. The owner-side review that re-pointed
> the mission is the latest ledger entries.

## Target model (one sentence)

A Texture document is **one durable agent thread** — the most basic multi-turn
LLM primitive — whose message history accumulates across the document's whole
life, that **always deep-researches** by actively driving its own next research
and revision, suspends cheaply (no billed calls, no polling) while awaiting
results, and resumes the **same thread** (never a reconstructed one). It never
stops: on diminishing returns or budget exhaustion it **quiesces** (suspends
awaiting input) rather than terminating, and ends only when its document is
deleted.

## Why (deeper goal G)

Texture is Choir's human-supervision narrative and artifact control plane. The
owner watches a document deepen revision by revision — for a news/research
question that means many grounded versions building toward a deep answer, and
for long-running coding/agent work it means a fresh supervisable revision as the
work progresses. A one-shot draft pane cannot do this. The substrate must be a
continuously-updating durable thread, not a dormant actor woken to emit one
chatbot answer.

This is **not a mode**. Every Texture document is a deepening thread by default.
There is no "deep research" flag and no per-prompt classifier deciding whether to
go deep.

## Problem (what is wrong today, post-revert)

The overnight work landed real mechanics (multi-write-per-run, a from-weights
first paint, a cheap park primitive, a per-actor budget) but built them on the
**wrong spine** and left ad-hoc detritus. Concretely, in current `main`:

1. **It is still run-centric, not a thread.** When a parked actor's 2-minute
   idle deadline expires the run *completes (dies)*. The next findings packet
   starts a **brand-new run** (`reconcileTextureAgentWake` ->
   `submitTextureAgentRevisionRun`, `texture_controller.go`) that rebuilds the
   user turn from the **current doc head + recent channel messages**
   (`buildAgentRevisionRequest`) and injects only a **summary** of the prior
   run's memory (`run_memory.go` `actor_rewarm`). This "cold rewarm" is lossy
   reconstruction; the conversation does not continue.
2. **Depth is reactive and capped.** The thread only revises when a findings
   packet is delivered to it; it never drives its own next research question. A
   single grounded pass is treated as fulfillment, so deep-research queries stop
   at ~V2. The overlay even encodes the wrong mental model
   (`revision_policy.yaml:57`: "Multi-revision work stalls when researchers stop
   early, not when Texture incorporates").
3. **Ad-hoc classifiers and guards remain.** `texturePromptNeedsSuperExecution`
   (a ~40-keyword list: `code`,`fix`,`test`,`verify`,`deploy`,`github`,`bash`,
   `artifact`...) and `texturePromptNeedsWorldKnowledge` (`latest`,`current`,
   `news`,`government`,...) plus `explicit*FromPrompt` /
   `texturePromptExplicitlyRequests*` parsers are semantic role-choreography by
   string match. They gate `textureModelPriorCompletionGuard` (the one remaining
   wired completion guard), so depth/grounding enforcement is keyword-gated and
   brittle. Content-reject guards (no-op, prompt-copy) and exact-first-tool retry
   machinery were bolted on to force staging probes to pass.
4. **Discovery-by-DB-query.** The park `ready()` and the wake path call
   `ListPendingWorkerUpdates` to *find out* whether work arrived. There is no
   busy-poll loop (good — `waitForAgentSignal` is channel-based and the
   all-docs reconcile is boot-only), but results are discovered by querying the
   store rather than delivered directly into the thread.

## Owner direction (authoritative constraints)

- **Accumulating thread, not cold rewarm.** One agent, one growing context
  window, turns that build on each other. Compact only on real context-window
  overflow; never reconstruct from the doc head.
- **Always deep research.** It is the standard behavior, not a mode. Texture
  actively requests more research / asks follow-up questions; it does not just
  passively receive updates.
- **No parent/child semantics.** Actors are uniform peers that communicate via
  the message/update substrate. Texture does not spawn or own researchers and
  does not manage their lifecycle.
- **Super is a singleton.** There is one persistent Super per owner/computer,
  addressed by request (`request_super_execution`), never spawned by a Texture
  agent.
- **No polling.** Suspension and wake are event-driven (signal channels);
  results are delivered into the thread, not discovered by querying a table.
- **Hard cutover, maximize deletion.** No migration, no dual-path, no
  compatibility shims. There are no real users yet; prefer deleting code over
  preserving the old run-centric scaffolding. Less code is the win.

## Scope / Domain Ramp (deletion-oriented)

Roughly in dependency order. Each item should *remove* more than it adds.

- **R0 Texture document as a versioned JSON record (body + system-attributed
  provenance).** Rebase the canonical unit from a bare markdown string to a typed
  record serialized as **JSON** (chosen over YAML: JSON has a real
  canonicalization standard, RFC 8785 JCS, which the future digital-signature
  work needs; YAML has no robust canonical form and implicit-typing footguns).
  The record carries `schema_version`, `body` (a markdown string), and a
  `provenance` block: authoring model, timestamps, the queries actually executed,
  and the collated `sources` (evolve the existing `textureSourceEntity` schema in
  `texture_media_sources.go`, which already has entity ids, selectors with
  `TextQuote`/`ContentHash`, evidence state, and `provenance.created_by`).
  **Provenance is system-attributed, not model-authored**: the model authors only
  the `body` and inline citation markers referencing real source ids; the runtime
  fills provenance from ground truth. DELETE the regex source-scraping
  (`sourceServiceItemIDsFromText`, `contentItemIDsFromWorkerMessage`, body-URL
  scraping) in favor of typed findings packets. YAML, if wanted, is only a
  human/export projection rendered from the JSON. Digital signatures are
  explicitly out of scope now, but the canonical serialization is chosen so they
  can be added without a re-format.
  Storage shape: do **not** stuff the JSON record into the existing markdown
  `Revision.Content` `LONGTEXT`. Keep `body` as the markdown column and add
  provenance as a **typed sibling** (column/blob), so the editor, renderer,
  diff/blame, export, user save, source renderer, and Wire publication keep
  reading a markdown body. "Hard cutover / no shim" means no dual code path and no
  data migration (there are no users) — it does **not** mean readers are
  untouched; every reader of `revision.content` (frontend `TextureEditor.svelte`,
  `texture-source-renderer.ts`, `internal/wirepublish`, store, diff) is updated in
  the same cutover. This is the largest-blast-radius ramp item; sequence it first
  and behind tests precisely because it touches store schema + UI + Wire.
- **R0b Source chain of custody (deterministic citation validation).** A source
  id is **minted by the runtime at the retrieval boundary**, never chosen by a
  model. Two retrieval boundaries exist:
  - **Deterministic media ingestion (runtime, no model call).** YouTube and image
    URLs are retrieved + embedded by the runtime itself the moment a URL appears
    (in the prompt or body), not by a researcher or Texture tool call. This is the
    existing `registerTextureMediaSourceRefs` -> `ImportURLContent` path, kept and
    promoted to deterministic infrastructure (it must not wait on a model tool
    call). For YouTube this includes a **deterministic transcript fetch** stored as
    a content item (the ref already carries `transcript_content_id` /
    `transcript_availability`). It produces importer-attributed source entities.
  - **Researcher semantic retrieval.** Facts come from researcher peers
    (`web_search`/`source_search`/`fetch`); Texture does no *semantic* retrieval
    and authors/cites only from delivered findings + deterministically-ingested
    media. So "Texture does no retrieval" means no semantic/research retrieval; it
    does not forbid the deterministic media-embedding pipeline.
  The same deterministic validator runs at each boundary (harness-uniform):
  (1) researcher -> finding: every source id a finding cites must be in the set
  that researcher actually retrieved; (2) Texture body -> collated list: every
  inline citation must resolve to a real collated source. Validation is
  **source-type-aware** (per Codex review — a blanket quote gate both
  false-rejects and false-accepts):
  - sources with a **verifiable text body** (researcher full-text fetch / reader
    snapshot): require the cited quote to match a *specific stored body
    version/selector* (bind the marker to the selector text + content version, not
    just any `ContentHash`, since raw bytes vs cleaned reader markdown differ);
  - **media / whole-resource / source-service projections / feed summaries /
    metadata packets** (often no full body, only a hash or excerpt): validate by
    id existence (+ selector/timestamp where present), **not** by quote match.
  Validation failures return a tool error (listing offending refs + the valid id
  set) to the model for retry. Hard gate: "every citation resolves, and where the
  source has a verifiable text body its quote matches that stored body version";
  the reverse ("every retrieved source must be cited") is soft and not enforced.
- **R1 Collapse to one durable thread (a real run-lifecycle change, not just a
  deletion).** Make `texture:<docID>` a single persistent agent whose message
  history accumulates; V0 is the first user turn, each canonical revision is an
  assistant turn that calls a texture write tool. The honest core change (per
  Codex review): today `runtime.go` boot **passivates** active runs, drops
  residency, and rewarms by creating a *replacement* run seeded from run-memory
  keyed by `loop_id` (`RunPassivated` is non-terminal-but-not-active). Deleting
  `actor_rewarm` alone does **not** yield "same thread" — it strands passivated
  runs. So R1 must change the run model so a passivated Texture thread resumes
  under the **same `loop_id`** with its run-memory replayed verbatim (see R4),
  then DELETE the wake-as-new-run path: `reconcileTextureAgentWake` starting fresh
  runs, the `scheduleTextureWorkerWake` debounce, per-wake
  `submitTextureAgentRevisionRun`, and the `actor_rewarm` summary reconstruction.
  This is the load-bearing item; "delete the cold rewarm" is the *consequence*,
  not the mechanism.
- **R2 Event-driven delivery into the thread (live or quiesced).** Owner
  follow-ups and inbound findings append as turns on the resident thread and
  signal it directly when it is live. When the thread is quiesced/passivated, the
  update is written to a **durable per-thread inbox cursor** and the appended
  turn(s) are folded into the thread's run-memory on resume — not rediscovered by
  re-querying. This replaces the `ListPendingWorkerUpdates` discovery +
  summarize-into-metadata + mark-delivered cycle (`texture_controller.go`,
  `runtime.go`). No idle-death; no DB-discovery rediscovery for a thread that
  already exists.
- **R3 Always-deep-research, prompt-only, progressively exhaustive.** Rewrite the
  Texture overlays so the thread treats the **user prompt as the sole anchor**
  (no model-invented "agenda" of what the answer should contain) and gets
  progressively more exhaustive across revisions. The continue / quiesce /
  redirect decision is driven by the prompt and the **actual research stream**,
  not a fabricated plan:
  - **deepen** an open-ended prompt across successive revisions;
  - **halt-on-answer**: if a fact of the matter answers the prompt, quiesce once
    it is found;
  - **halt-on-saturation**: if research keeps returning the same content (no new
    information), stop looping and quiesce;
  - **redirect-on-linger**: if the prompt's question(s) remain unresolved, take a
    *different* approach (new angle / query), do not repeat the same one.
  DELETE `texturePromptNeedsSuperExecution`, `texturePromptNeedsWorldKnowledge`,
  `texturePromptExplicitlyRequests*`, `explicit*FromPrompt`,
  `textureModelPriorCompletionGuard`, and the no-op/prompt-copy content-reject
  guards. Grounding honesty (below) is enforced by prompt, not by keyword guard.
- **R4 Passivation as true sleep/resume (same `loop_id`).** vmctl refresh /
  process restart sleeps the thread; resume reuses the **same `loop_id`** and
  replays the **literal** run-memory (compaction only on overflow), not a summary
  reconstruction. This is the run-model change R1 depends on: a passivated Texture
  thread is resumable in place, not replaced. Cumulative cost/spend persists with
  the thread across sleeps (see R5).
- **R5 The thread never stops; it quiesces.** Remove the 2-minute idle death and
  the 16-resume cap as lifecycle gates. There is **no terminal "done" state and
  no kill-switch**. The thread quiesces — suspends with zero billed calls and
  resumes the same thread when input arrives — never ends, in two cases:
  (a) **nothing to do right now** (awaiting outstanding research/execution
  requests, or the model judges further autonomous work low-value); and
  (b) **budget** — the cumulative per-actor budget bounds *autonomous* spend
  between owner interactions, so exhausting it quiesces (stops spending on its
  own) rather than failing. This requires a real **budget-quiesced state** distinct
  from the current failure path: today budget exhaustion returns an ordinary error
  that `handleExecutionError` turns into a *failed* run (`toolloop.go`,
  `runtime.go`). Needs a durable autonomous-spend ledger; a new owner intent
  re-authorizes a bounded next window (not an unbounded reset). The thread ends
  **only** on doc delete / cancel.
  Note (owner direction): **infinite deepening is acceptable** — Texture looping
  to keep improving a document is a feature, not the overnight-`/goal` failure
  mode, and is fine to tune later. So quiescence is for cheap idle/await and
  budget accounting, **not** an infinite-loop guard. Quiescence should still emit a
  durable structured record (reason, outstanding requests, tried-query set, budget
  state) so the thread's state is *observable*, but proving termination is not a
  settlement requirement.
- **R6 Cutover cleanup.** The `AgentMutation` row is **load-bearing**, not just
  idempotency scaffolding (per Codex review): `commitTextureToolEdit` gates writes
  on a pending mutation, and the row also carries liveness/pending state, the
  cancel handle, stale-run recovery, and "latest revision written by this run". So
  do not naively delete it — **collapse its roles into the durable-thread record**
  (write authority, residency/liveness, cancel handle, stale recovery). Replace
  the one-write semantics with a per-thread **revision transaction protocol**:
  keep the optimistic `base_revision_id` head check, add a **per-turn idempotency
  key** and the durable **inbox cursor** (R2) so concurrent researcher findings +
  owner follow-ups have defined ordering/dedupe/retry without the process-local
  `textureEditMu` being the only guard. DELETE the exact-first-tool /
  required-initial-tool retry machinery and now-dead controller code. Reconcile the
  workflow verifier, Trace projection, heresy detectors, tests, and doctrine
  (`docs/texture-agentic-invariants-2026-06-13.md`) to the thread model — including
  a **new acceptance/verifier state contract**: a never-completing thread is
  accepted on revision cadence + a quiescence-evidence record, not on terminal run
  completion (today tests and Trace assume completed/blocked/failed).
- **R7 Deploy + staging proof.** A deep-research prompt yields many grounded
  revisions accumulating over time from one thread (well beyond the V2 cap), a
  fast interim first paint, survival of a vmctl refresh as **resume** (not cold
  rebuild), bounded cost, and doc-delete cancellation. The document is a JSON
  record whose citations all resolve to collated sources with matching quotes
  (deterministic validation passes; a deliberately fabricated citation is rejected
  and retried). Record a RunAcceptanceRecord.

## Invariants / qualities

- **I accumulating-thread continuity**: one durable thread per doc; context
  accumulates; resume is verbatim; compaction only on real overflow; never
  reconstruct from doc head.
- **I active deepening is the default**: the thread drives its own next
  research/revision; "always deep research", no mode, no per-prompt classifier.
  Progressively exhaustive, anchored to the user prompt. Continue/quiesce/redirect
  is driven by real signals — deepen open-ended prompts, halt on a definitive
  answer, halt on research saturation (same content returning), redirect (new
  approach) when questions linger — never by a model-authored coverage plan.
- **I peer actors, no hierarchy**: no parent/child spawn-ownership; Super is a
  singleton addressed by request; researchers fulfill requests and deliver
  findings back to the thread; uniform harness across roles (AGENTS.md
  harness-minimalism).
- **I event-driven, no polling**: suspend/resume via signal channels; results
  delivered into the thread, not discovered by querying.
- **I grounding honesty / no hallucination**: the user prompt is authoritative
  and taken as-is; the thread does not invent sub-questions, requirements, or
  facts. A from-weights first turn is allowed and useful but flagged
  interim/model-prior and must not assert current facts as grounded; grounded
  turns cite retrieved evidence. Depth is driven by retrieved evidence, never by
  fabricated structure. Prompt-enforced.
- **I source provenance & chain of custody**: source ids are runtime-minted at
  the retrieval boundary; provenance is system-attributed (the model authors only
  body + citation refs). Cited refs are deterministically validated at the
  researcher->finding and Texture-body->collated-list boundaries; every inline
  citation must resolve to a real collated source, and **for sources with a
  verifiable text body** the quote must match a specific stored body version. The
  gate is source-type-aware (quote-match for text-bodied sources; id+selector
  existence for media/whole-resource/summary projections). Failures return a tool
  error for retry. This is a deterministic gate, not a classifier.
- **I retrieval split**: Texture does no *semantic/research* retrieval — facts
  come from researcher peers and Texture authors/cites from delivered findings.
  But **deterministic media ingestion** (YouTube/image URL retrieval + embedding)
  is runtime infrastructure that runs without any model tool call. The collated
  source list = researcher-delivered sources + deterministically-ingested media.
- **I JSON canonical document**: the canonical Texture document is a versioned
  JSON record (body + system-attributed provenance), chosen for canonicalizable
  bytes (future signatures); YAML is at most a human projection.
- **I canonical integrity**: monotonic versions, no lost foreground updates, no
  duplicate/garbled revisions.
- **I bounded autonomous cost**: a cumulative per-actor budget bounds autonomous
  spend between owner interactions; exhausting it **quiesces** the thread (no kill,
  no terminal state); a new owner intent re-authorizes spend. Quiesced/parked
  time costs no provider calls.
- **I hard cutover**: no shims, no dual-path, no semantic role-choreography or
  ad-hoc string classifiers. Net lines deleted > added.
- **Q acceptance proves the product path** (prompt-bar + public APIs), not
  manual worker invocation; per-revision Trace/Texture boundaries stay legible
  within one long thread; trajectory/work-item attribution preserved.

## Open design edges (name before building)

- **Request idempotency without AgentMutation.** HTTP renewal/retry must not
  duplicate a turn or a revision. Likely: resident-thread dedupe +
  content-derived delivery id (already used for owner revises in `dfc78fcd`).
- **Concurrency vs await.** The thread may issue several research/execution
  requests and keep revising on what it already has; it only suspends when there
  is nothing left to do but await outstanding results.
- **Compaction safety (grounded artifacts only).** Compaction compresses old
  conversational scratch but must re-seed the window verbatim with **grounded**
  state only: the latest canonical doc head (durable; re-read via tool), the index
  of **sources actually retrieved** with their citations, and a record of
  **queries/approaches already tried** — all of which now live as fields in the
  R0 JSON provenance record, so compaction re-seeds from durable structured state
  rather than from a free-text summary. It must NOT synthesize or carry a
  model-invented
  agenda of what the answer "should" cover — that is the hallucination risk to
  avoid. Saturation/linger judgments are made against the retrieved-sources record
  and the prompt, not a fabricated plan.
- **Termination signal (decided: the thread never stops, it quiesces).** There
  is no terminal "done" state and no kill-switch. Both diminishing returns and
  budget exhaustion cause **quiescence** (suspend awaiting input), not
  termination; budget bounds autonomous spend between owner interactions and a
  new owner intent re-authorizes it. The thread ends only on doc delete/cancel.
  This avoids both the silent V2 cap and the overnight infinite-loop without a
  brittle runtime stop-heuristic. Risk to watch: a thread that quiesces too
  eagerly looks identical to the old V2 cap, so the deep-research proof must show
  many revisions before quiescence; and budget accounting must persist across
  sleeps so re-authorization, not a fresh full budget, is what resumes spend.
- **Researcher multiplicity.** Researchers are peers fulfilling requests; their
  pool/lifecycle is out of scope here beyond "deliver findings back to the
  requesting thread."
- **Media transcript delivery to researchers (deferred).** YouTube transcripts
  are fetched deterministically (R0b). Open: whether to *inject* the transcript
  text into the researcher's context or hand it a **content handle** the
  researcher reads agentically via `list_content_item_selectors` /
  `read_content_item_selector` (bounded by selectors), possibly **adaptive to the
  researcher's open context window** (inject when small, handle when large). The
  existing transcript-as-content-item + selector tools favor the handle path, but
  this is not decided now and does not block the schema.

## Variant (V) and current value

V descends as each ramp item lands with deployed evidence and as net code
shrinks. Current value after the 2026-06-18 owner-side review and revert:

- Reverted the 6-commit Super-worker completion-guard choreography (`f002e07a`).
- Landed the two foreground P1 fixes: resident-thread owner-revise delivery and
  reconcile-only-on-terminal-run (`dfc78fcd`), with deployed cadence
  non-regression (V1 +26s, V2 +63s, trajectory completed) on
  `dfc78fcd`/`667c70d2`.
- Still present and to be deleted by this rewrite: run-centric cold rewarm, the
  keyword classifiers, the model-prior completion guard, the no-op/prompt-copy
  guards, the exact-first-tool retry machinery, the 2-min idle death, and the
  AgentMutation one-write scaffolding.
- Not yet built: the durable accumulating thread, prompt-only active deepening,
  true sleep/resume, budget-bounded lifetime, deep-research depth beyond V2.

## Budget / authority / mutation class

One broad red-surface paramission executed iteratively (implement -> critical
review -> iterate). Broad change and deletion are authorized; the owner values
the correct durable-thread architecture over incremental compatibility.

Execution mutation class **red**. Protected surfaces: Texture canonical writes,
the run lifecycle / tool loop, the park/suspend and budget primitives,
passivation/rewarm, worker-update consumption bookkeeping, the Texture workflow
verifier, Trace/evidence, and deployment routing. Before touching orange/red
surfaces, name the conjecture delta, protected surfaces, admissible evidence
class, rollback path, and heresy delta.

## Evidence packet (settlement)

Focused local tests for: single durable thread with accumulating turns and
N revisions; event-driven delivery + wake without polling; prompt-only active
deepening; sleep/resume of the literal thread across passivation; budget
exhaustion quiesces (does not terminate) and a new owner intent re-authorizes
spend; doc-delete cancellation; verifier N:1 within one thread.
Then `go-test-runtime-shards`; CI; Node B deploy identity; deployed cadence/depth
probe (a deep-research prompt produces many grounded revisions over time from one
thread, fast interim first paint, resume across a vmctl refresh, bounded cost);
prompt-bar submission id, conductor run id, doc id, Texture thread id;
RunAcceptanceRecord at staging-smoke-level; net-lines-deleted figure; residual
risk note. Rollback = revert the mission commits.

## Suggested goal string

```text
/goal Use Parallax on docs/mission-texture-long-running-agent-v0.md (read it and its ledger first; the ledger trail is historical, the doc body is the current target). Make each Texture document ONE durable agent thread whose message history accumulates across the doc's life: V0 = first user turn, each canonical revision = an assistant turn calling a texture write tool, inbound findings and owner follow-ups = appended turns on the same thread. It is ALWAYS deep research (not a mode): after each revision the thread actively requests more research/execution and deepens; it never stops, it QUIESCES on diminishing returns or budget exhaustion (suspend cheaply, no billed calls, no polling) and resumes the SAME thread (never reconstructed) when new owner input/findings arrive; budget bounds autonomous spend between owner interactions and a new owner intent re-authorizes it; the thread ends only on doc delete/cancel. Hard cutover, maximize deletion: remove cold rewarm (reconcileTextureAgentWake-as-new-run, scheduleTextureWorkerWake debounce, actor_rewarm summary), the keyword classifiers (texturePromptNeedsSuperExecution/NeedsWorldKnowledge/explicit*FromPrompt), textureModelPriorCompletionGuard, the no-op/prompt-copy content guards, the exact-first-tool retry machinery, the AgentMutation one-write scaffolding, and the 2-min idle-death/16-resume gates. No parent/child semantics; Super is a per-owner singleton addressed by request; actors are uniform peers. Enforce grounding honesty and active deepening via prompt, not runtime classifiers. Work R1-R7 in order; land what you can safely prove and leave precise file-cited blockers. Verify with focused internal/runtime tests + scripts/go-test-runtime-shards, then commit -> push origin main -> CI -> Node B deploy identity -> deployed proof on https://choir.news (deep-research prompt yields many grounded revisions over time from one thread; resume across a vmctl refresh; bounded cost); record a RunAcceptanceRecord at staging-smoke-level. Mutation class red. Rollback = revert mission commits.
```

## Lineage

Supersedes the prior framing of this mission (wake-driven actor + park/rewarm +
per-prompt classifiers) and folds in
`docs/mission-texture-product-loop-recovery-v0.md`. Sits on the portfolio spine
after the Texture product-loop work and before continuation deletion (M4); the
sleep/resume work here should be reconciled with M4 and the durable-actors
rearchitecture (`docs/choir-rearchitecture-durable-actors-2026-06-11.md`), which
this thread model is an instance of.

## Settlement requirement

Not met. Settles only with deployed staging proof that a deep-research prompt
produces many grounded revisions accumulating over time from a single durable
thread (well beyond the current V2 cap), a fast honest interim first paint,
survival of a vmctl refresh as resume of the same thread (not cold rebuild), a
cumulative per-actor budget that quiesces (never kills) the thread on exhaustion,
doc-delete cancellation, deletion of the
run-centric/classifier/guard scaffolding (net lines deleted), updated
verifier/tests/docs/heresy detectors, and a RunAcceptanceRecord at
staging-smoke-level or higher.
