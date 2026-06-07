# Mission: Global Wire SourceMaxx Newsroom Runtime

> **Superseded active target:** Do not use this as the current execution
> mission. User review on 2026-06-07 rejected "SourceMaxx" as a product name
> and clarified that the real priority is broad source ingestion in many
> languages plus VText-native article ownership/transclusion. Use
> `docs/mission-global-wire-broad-source-vtext-newsroom-v0.md` and
> `docs/choir-global-wire-broad-source-vtext-newsroom-spec-2026-06-07.md`
> for the active mission/spec. This file remains historical evidence.

**Status:** ambitious MissionGradient delivery mission after architecture and
design-language correction.
**Requirements contract:** `docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md`  
**Prior mission context:** `docs/mission-global-wire-style-vtext-collaborative-storygraph-v0.md`  
**Created:** 2026-06-07
**Rewritten:** 2026-06-07

## Goal String

```text
/goal Run docs/mission-global-wire-sourcemaxx-publication-system-v0.md to shipped staging proof. Deliver SourceMaxx -> processors/reconcilers -> researcher/VText reuse -> publication-quality Global Wire. Make it nice.
```

## Why This Mission Exists

The earlier Global Wire trajectory was under-specified at the most important
layer: what happens between a high-volume source firehose and a
publication-quality article.

The wrong object is:

```text
user clicks refresh
-> one or a few source results
-> deterministic signal
-> many UI panels
-> shallow style projection
```

The right object is:

```text
SourceMaxx ingestion
-> long-running processors with live understanding
-> reconcilers that see cross-story links and conflicts
-> existing researchers for targeted evidence work
-> existing VText agents writing with deep Style.vtext
-> VTexts with per-version source/style provenance
-> lightweight VText traversal/source indexes
-> readable publication surface
```

This is not a cosmetic mission. It is an architecture reset. Deletion is part
of the work when existing paths encode the wrong newsroom object.

## Cognitive Transform Set

Current obstacle: the product has been treated like a data surface and graph
slice, while the missing system is a live newsroom cognition loop. The newer
obstacle is over-correcting into developer-style timidity: proving local
behavior instead of delivering the product object.

Selected transforms:

1. **Depth extraction:** the banal object is "news app with sources." The deep
   object is a source-maxxed newsroom runtime whose live agents maintain
   understanding and publish through VText.
2. **Resident cognition:** processors should not wake up stateless and rebuild
   context from text every cycle. They should preserve hot context/KV cache,
   compact when needed, and carry handles to full source content.
3. **Role naming as architecture:** processors may span categories, regions,
   event families, source classes, or load-balanced firehose slices. The name
   must describe source processing, not a fixed beat boundary.
4. **Corpus reconciliation:** reconcilers are not downstream processors. They
   range over the story corpus: existing published VTexts, active platform
   VTexts, authorized user-owned VTexts, source state, processor notes,
   contradictions, questions, and update history.
5. **Reuse before invention:** Choir already has researcher and VText agents.
   SourceMaxx should route work into those agents instead of creating parallel
   evidence or writing systems.
6. **Style as editorial instrument:** `Style.vtext` is not a tab or prompt
   string. It is a citeable editorial source artifact selected because it fits
   the story and publication need.
7. **Subtractive product design:** a clean newspaper column over real source
   breadth is more correct than a busy dashboard exposing repeated internal
   artifacts.
8. **Topology not ladder:** the mission should not climb from fake demo to real
   object. Build the same production object at a workable resolution, then
   raise resolution aggressively along source volume, agent continuity,
   VText writing, style quality, and staging proof.
9. **Make it nice:** correctness is not enough. Publication output, source
   chronology, VText affordances, typography, and three-theme behavior must be
   good enough that the shipped surface feels intentional rather than merely
   functional.

Changed route:

- Deliver the full SourceMaxx newsroom object, not a throwaway slice: source
  volume, processors, reconcilers, researcher reuse, VText reuse, Style.vtext
  routing, user-owned VText boundaries, VText traversal indexes, and readable
  Global Wire UI move together.
- Use short control loops for evidence, but keep the target ambitious:
  product-shaped behavior on staging with a quality pass before stopping.
- Reuse existing researcher and VText agents as the evidence and writing path.
- Treat durable VText indexes as navigation/query accelerators over VTexts,
  versions, sources, and transclusions, not the authority for provenance or the
  whole intelligence.
- Prove readable publication output, not just API record creation, and do a
  final simplification/design pass before claiming delivered status.

## Real Artifact

The artifact is an AI newsroom runtime inside Choir:

```text
source registry and fetch loops
-> SourceItem ledger
-> simple routing
-> long-running processors
-> processor compaction chain and source handles
-> reconcilers over the story corpus
-> existing researcher requests/results
-> existing VText write/revision requests
-> article/story VTexts and Style.vtext projections
-> per-version source/style provenance inside VText
-> durable VText traversal/source indexes
-> clean Global Wire newspaper/workbench
-> user-owned forks, edits, contributions, and publications
```

The mission is incomplete if any core edge is missing: source intake,
processors, reconcilers, researcher reuse, VText reuse, Style.vtext routing,
ownership boundaries, or readable publication output.

The desired result is `delivered`: committed, pushed, CI/deploy observed,
staging identity verified, product-path behavior proven, and the owner-facing
Global Wire surface nice enough to use without apologizing for the architecture.

## Value Criterion

Move Choir closer to a live, source-grounded, publication-quality newsroom
while preserving VText ownership, provenance, non-oracle news, and staging
proof.

The product moves uphill when:

- source intake approaches hundreds of SourceItems per 15 minutes and has a
  credible path to faster/live ingestion;
- SourceItems are durable, deduped, source-tagged, time-tagged, and linked to
  fetch provenance;
- processors receive routed source batches and maintain high-context
  understanding across turns;
- processor compaction preserves source handles, unresolved questions, watch
  items, active developments, and prior judgments;
- reconcilers inspect the story corpus, published VTexts, active platform and
  user-owned VTexts where authorized, processor notes, source state,
  contradictions, consensus, drift, and questions;
- processors, reconcilers, and VText agents can request existing researcher
  agents for additional evidence;
- existing VText agents write or revise normal VTexts from processor/reconciler
  notes, research packets, and matched Style.vtext artifacts;
- `Style.vtext` routing is selective and explainable rather than all styles
  over all stories;
- publication output is genuinely readable and useful;
- the app presents news in clear columns, not nested panels or repeated cards;
- all articles can open as VTexts through a quiet repeated affordance rather
  than noisy label text;
- typography, spacing, source chronology, and three-theme behavior match
  [Choir Design Language](./choir-design-language-2026-06-07.md);
- staging proof shows the actual product path, not local-only or test-only
  behavior.

## Hard Invariants

- Every story remains a normal editable VText.
- User edits, forks, contributions, and publications are user-owned and never
  mutate platform stories directly.
- Platform corrections/updates are ordinary new VText versions through
  explicit candidate/review/version records.
- `Style.vtext` is a citeable, selectable, composable, replaceable source
  artifact.
- Style projections preserve evidence and cite source/style lineage within the
  produced VText version.
- News is non-oracle: uncertainty, contrary evidence, source standing, and
  change history remain inspectable.
- VText versions are the provenance-bearing objects. Sources are per-version,
  not per-VText. Durable indexes may accelerate VText graph walking, but they
  do not replace VText-native provenance or live processor/reconciler
  cognition.
- Existing researcher agents and existing VText agents must be reused unless a
  documented invariant proves they cannot serve the role.
- All app views work in Futuristic Noir, Carbon Fiber Kintsugi, and London Salmon.
- Product-path/staging proof is required before claiming behavior.

## Architecture

### Deterministic Substrate

Deterministic code owns scale, identity, provenance, routing, and mutation
boundaries.

Responsibilities:

- source registry for GDELT, many RSS feeds, many Telegram feeds, search
  providers, and curated/provider sources;
- scheduled fetch cycles and faster/live paths where providers allow;
- rate limits, backoff, provider health, and fetch-run metrics;
- SourceItem persistence with canonical URL/content refs, source IDs, source
  type, publisher/channel, timestamps, raw and cleaned content refs, hashes,
  standing metadata, and policy metadata;
- exact and safe dedupe while preserving echo/duplicate evidence when useful;
- simple routing by source, topic hints, geography, language, source class,
  event family, and load budget;
- durable request/result records for processors, reconcilers, researchers, and
  VText agents;
- ownership, native VText versioning, rollback refs, and publication indexes.

Clustering and embeddings are not required for delivery. They are later
realism axes after the processor/reconciler loop is proven.

### Processors

Processors are long-running agents that receive source batches and maintain
live understanding. They are not necessarily vertical-specific.

A processor may cover:

- a broad topic area;
- several related verticals;
- a geography;
- a source class;
- a developing event family;
- a load-balanced slice of the firehose.

Processors should:

- keep hot context/KV cache across source batches;
- ingest new SourceItems with source handles, not pasted untraceable blobs;
- update their live understanding of active developments;
- maintain watch items, unresolved questions, changed beliefs, and candidate
  story/update briefs;
- request existing researcher agents when evidence is missing, contradictory,
  high-risk, or publication-sensitive;
- request existing VText agents when a story should be written or revised;
- compact when context pressure rises, preserving source handles and the
  current cognitive state needed for continuation.

Processor compaction is not a generic summary. It is an agent-owned continuity
artifact with handles to full source content, prior compactions, active briefs,
research requests, VText requests, and unresolved questions.

### Reconcilers

Reconcilers are corpus-level story agents, not a stage after processors.

They range over active and historical story state:

- existing published VTexts;
- active platform VTexts;
- authorized user-owned VTexts, including published user versions;
- processor notes/briefs and source handles;
- researcher evidence packets;
- current relationship/index records;
- unresolved contradictions and open questions.

They ask:

- What connects across existing stories?
- What contradicts or has drifted since publication?
- What old story needs an update because new source flow changed public
  meaning?
- What published story should be linked, split, corrected, or followed up?
- What story needs more research before publication?
- What question should a processor, researcher, or VText agent pursue next?
- What new insight or perspective deserves its own VText?

Reconcilers may request existing researcher agents and existing VText agents.
They also write durable relationship/question/contradiction notes so the News
app and other agents can expose their work.

Processors and reconcilers use the same underlying shared Choir agent harness
as the existing researcher, VText, super, vsuper, and co-super agents. They are
role/prompt/capability specializations with profile-specific toolsets, durable
state, tool calls, compaction, continuation, and channel/request records. They
must not grow a parallel loop, provider adapter, run store, event stream,
delegation mechanism, compaction mechanism, or worker-update path unless a
future invariant proves the shared harness cannot protect correctness,
security, authority boundaries, or resource isolation.

### Existing Researcher Agents

Researchers remain Choir's bounded evidence workers.

Processors, reconcilers, and VText agents may ask researchers to:

- verify a claim;
- find missing sources;
- inspect contradictory accounts;
- evaluate source standing;
- gather context;
- prepare an evidence packet for a VText agent.

Researchers return source-backed evidence packets. They do not own final
article voice.

### Existing VText Agents

VText agents remain Choir's writers/editors.

They receive:

- processor notes/briefs;
- reconciler notes/briefs;
- researcher evidence packets;
- current VText handles when revising;
- matched `Style.vtext` artifacts;
- user/publication context.

They produce or revise ordinary article/story VTexts. A processor note,
reconciler note, or researcher packet may itself be a VText and may become the
v0/v1 seed for the article. They may request additional research when the brief
is too thin, contradictory, or risky.

### VText Traversal And Source Indexes

Use indexes to make VText-native provenance and transclusion paths searchable
and readable without creating a second authority:

- VText id and version id;
- per-version source refs and multimedia transclusions;
- VText-to-VText transclusions and links;
- `Style.vtext` citations and compositions;
- processor/reconciler/research VTexts where they exist;
- contradiction/update/timeline/related-story refs derived from VTexts and
  reconciler notes;
- publication state and user-owned published versions.

Do not make "StoryGraph authority" a vague abstraction. The concrete rule is:
agents write VText notes, candidate VText updates, and index records; canonical
platform stories change only through explicit VText version/update paths.
Corrections and updates are normal new VText versions, not special correction
objects. Source and style provenance lives inside each VText version; indexes
only point at it.

Future Autoradio likely needs this index because audio traversal means walking
a path through VText graph space and turning that path into one fluid narrative.
Do not build Autoradio in this mission; TTS/STT model exploration and narrative
path rendering are separate later work.

### Deep Style.vtext And Style Routing

`Style.vtext` artifacts must be publication-grade editorial sources, not short
prompt snippets.

A high-quality Style.vtext includes:

- editorial purpose and audience;
- voice principles;
- structure and section patterns;
- evidence and citation rules;
- uncertainty/correction/source-standing rules;
- examples of strong output;
- anti-patterns;
- revision policy;
- applicability metadata;
- "do not use" cases;
- composition/replacement rules;
- projection evaluation criteria.

Style routing asks: which style source or style composition should guide this
VText, if any?

Default behavior must not run every style over every story. The system should
select, rank, compose, or withhold styles based on story domain, audience,
source state, publication need, user/publication context, and evidence risk.
Users can request a different style, customize a style, or create a new
`Style.vtext` that their VText agents can use going forward.

### News App Redesign

Global Wire should read like a clean newspaper/workbench and should transclude
VTexts rather than replacing the VText app:

- front page columns, not card walls;
- no nested scrolling panels;
- no repeated display of the same limited information;
- no story border lines, boxes, or grid rules; text, whitespace, and section
  rhythm provide structure;
- primary scan shows headline, live change, source breadth, and tension;
- source chronology is available in reverse chronological order with filters
  by source class, vertical/topic hints, geography, and later search;
- full reading, editing, forking, style changes, and user publication happen
  by opening the VText in the VText app;
- evidence, style, VText traversal, processor/reconciler notes, and research
  detail are progressive disclosure, not dashboard panels;
- controls are compact and predictable;
- all three themes preserve readability and capability.

Do not build a contribution surface inside the News app for this delivery run.
Contributions are user-owned VText/source/style artifacts. The platform should
eventually surface a feed of user-published VTexts on `choir.news`, but that is
not the UI priority unless it becomes necessary for the delivered SourceMaxx
newsroom object.
Do not build Autoradio in this mission; keep only the architectural horizon for
future VText-path narration.

Browser/Computer Use screenshots are required for visual proof.

## Deletion And Replacement Targets

Audit and delete, replace, or quarantine:

- one-result or click-time source-refresh bottlenecks;
- frontend-only preview authority that looks like product truth;
- repeated artifact panels;
- nested scroll containers inside panels;
- card-heavy layouts that prevent news scanning;
- shallow style tabs;
- projection matrices that run all styles over all stories;
- deterministic single-signal source review paths that erase processor context
  or reconciler questions;
- new standalone researcher or writer systems that bypass existing agents;
- tests that prove singleton/button behavior when the product requires
  high-volume processor/reconciler behavior.

Follow problem-documentation-first before fixing newly discovered deployed
behavior problems. Planned deletion from this mission should be recorded in the
mission checkpoint or commit message.

## Homotopy Axes

Raise resolution aggressively while preserving the same product topology. These
are not rungs or permission to stop low; they are the realism dimensions that
must converge toward delivered status.

- **Source volume:** staging should demonstrate hundreds of SourceItems per
  15-minute window or record a provider/runtime limit with exact evidence and
  the best live/faster cadence achieved.
- **Source diversity:** GDELT, many RSS/Atom feeds, many Telegram feeds, search
  providers, and curated source-class sets should all be represented unless a
  provider is blocked with root-cause evidence.
- **Freshness:** scheduled ingestion should run without click-time dependency;
  live or shorter cadence paths should be used where provider/runtime limits
  allow.
- **Processor continuity:** processors should be long-running roles with
  preserved context, compaction chains, source handles, watch items, and
  request/result state.
- **Reconciler realism:** reconcilers should review the existing VText corpus
  plus new source/processor state, then produce contradiction, consensus,
  question, update, research, and VText request records.
- **Research reuse:** existing researcher agents should receive bounded
  evidence requests and return source-backed packets used by VText agents.
- **VText reuse:** existing VText agents should write and revise normal
  article/story VTexts, including processor/reconciler/research note VTexts
  where useful.
- **Style depth:** `Style.vtext` artifacts should be publication-grade
  editorial sources with examples, anti-patterns, applicability, revision, and
  composition rules.
- **Style routing:** the system should select, compose, withhold, and explain
  styles based on story fit, evidence risk, audience, source state, and user
  context.
- **UI readability:** Global Wire should be a clean newspaper-like collection
  surface with source chronology, no story boxes/rules, quiet VText affordances,
  and responsive Choir web desktop behavior across all themes.
- **Product proof:** proof should proceed through tests, product-path API,
  staging source volume, browser screenshots, deployed commit identity, and a
  durable acceptance record where the platform supports it.

## Dense Feedback And Verifiers

Proof must include:

- source adapter and ingest tests;
- batch count, provider count, dedupe count, freshness, backoff, and error
  metrics;
- routing records showing SourceItems delivered to processors;
- processor evidence showing context continuity, compaction, and source handles;
- reconciler evidence showing connections, contradictions, questions, and
  researcher/VText requests;
- product evidence that existing researcher agents receive and answer requests;
- product evidence that existing VText agents write/revise normal
  article/story VTexts;
- Style.vtext routing proof showing fitting styles selected and non-fitting
  styles withheld or deprioritized with reasons;
- ownership proof that user edits/forks remain user-owned;
- browser screenshots across Futuristic Noir, Carbon Fiber Kintsugi, and London Salmon;
- staging commit identity and deployed product-path acceptance.

Use browser-public product APIs only for product proof. Do not use internal,
test-only, or raw mutation endpoints.

## Forbidden Shortcuts

- Do not claim SourceMaxx by increasing `max_results` on a click-time refresh.
- Do not use frontend seed data as product truth.
- Do not require clustering or embeddings before the processor/reconciler loop
  is proven.
- Do not create new researcher or writer agent systems when existing
  researcher and VText agents should be reused.
- Do not run all styles over all stories by default.
- Do not flatten sources into untraceable processor summaries.
- Do not hide provenance to make the UI clean.
- Do not add more panels to compensate for unclear architecture.
- Do not ship generic assistant prose as publication-quality output.
- Do not let user edits mutate platform stories.
- Do not claim staging behavior from local-only evidence.

## Stopping Condition

Mark `complete` only when the mission reaches delivered status and staging
proves:

- high-volume source ingestion from multiple source classes;
- durable SourceItem provenance, dedupe, and routing;
- long-running processor behavior with hot-context continuity and compaction
  handles;
- reconciler behavior over the story corpus, including existing published
  VTexts and current source/processor state;
- existing researcher agent reuse for targeted evidence work;
- existing VText agent reuse for normal article/story VText writing/revision;
- VText traversal/source indexes over VTexts, versions, and transclusions;
- user-owned fork/edit behavior;
- deep Style.vtext artifacts and intelligent style routing;
- publication-quality VText output;
- readable newspaper-style Global Wire UI in all three themes, with no story
  boxes/rules, no nested panels, and quiet per-article VText open affordances;
- responsive Choir web desktop behavior for mobile-width layouts;
- CI/deploy/staging identity and product-path acceptance evidence.

Do not stop at a partial demo, a local-only proof, or an API record that has not
become product behavior. Use `checkpoint_incomplete` only as a handoff after
substantial delivered progress when an external limit, context boundary, or
operator boundary prevents continuing in the current run. Use
`blocked_incomplete` only after root-cause investigation, product-path probes,
serious alternative architecture routes, cognitive transforms, and the next
safe executable probe or external authority requirement are recorded.

## Run Checkpoint & Resumption State

status: checkpoint_incomplete

last checkpoint: 2026-06-07 SourceMaxx aggregate-status checkpoint after
shipping a product-safe authenticated staging proof of the expanded source
firehose and queued processor/reconciler handoffs. This is useful
shipped-direction progress, not delivered mission completion.

current artifact state: prior Global Wire slices still exist in backend
storage/runtime APIs and staging has some Source Service-backed paths,
StoryGraph/VText/projection/contribution records, publication artifacts, and
newsletter ledgers. The visible News app surface has now been simplified into a
SourceMaxx desk: front-page article columns, a source chronology column, quiet
per-article VText affordances, compact Style.vtext controls, no app-local
theme selector, no contribution dashboard, no Autoradio surface, no repeated
`Open in VText` label text, no story border lines, and no nested app-panel
scrolling. Sourcecycled now has a SourceMaxx handoff substrate: expanded
GDELT/RSS/Telegram registry, configurable per-source poll caps, durable
processor request records, durable corpus-reconciler request records, and an
internal SourceMaxx latest-cycle endpoint. Runtime now also exposes an
authenticated product API projection at `/api/global-wire/sourcemaxx-status`
that reports only non-sensitive aggregate SourceMaxx cycle/handoff metrics
while preserving the `/internal/*` public-edge boundary.
Processors/reconcilers still do not yet execute as resident product agents,
source handoffs are not yet connected to existing researcher/VText agent
request channels, and Style.vtexts are not yet deep publication artifacts. A
partial source-refresh batch experiment from the superseded route is preserved in
`stash@{1}` named
`superseded-global-wire-source-refresh-batch-experiment-2026-06-07`.
The wrong pipeline-shaped processor/reconciler request code is preserved in
`stash@{0}` named
`wrong-pipeline-processor-reconciler-request-slice-2026-06-07` and must not be
reapplied blindly.

what shipped: behavior checkpoint in
`frontend/src/lib/GlobalWireApp.svelte` and
`frontend/tests/global-wire-app.spec.js`, followed by a source-runtime
checkpoint in `cmd/sourcecycled`, `internal/cycle`, `internal/sources`,
`internal/sourceapi`, `configs/sources.json`, and the authenticated aggregate
SourceMaxx status route in `internal/runtime/global_wire.go`,
`internal/runtime/tools_research.go`, and `internal/runtime/api.go`. The
visible Global Wire app now projects the mission design language: clean
newspaper columns plus source chronology instead of a dense
StoryGraph/contribution/newsletter/Autoradio dashboard. Every article has a
quiet VText button and fork button. Style.vtext routing remains compact and
citeable. Sourcecycled no longer routes every cycle through a monolithic LLM
issue synthesizer; it persists source items and queues processor/reconciler
work records by source handles. The product-safe status route lets staging
verification prove SourceMaxx source volume and handoff counts without exposing
internal source-service routes publicly.

what was proven:

- `npm --prefix frontend run build` passed.
- `PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 npm --prefix frontend run e2e -- tests/global-wire-app.spec.js --project=chromium --workers=1 --reporter=line`
  passed: 4 tests.
- Commit `83af469309ed1874780283b6115a16b87232893d` was pushed to
  `origin/main`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27094084911` completed
  successfully for commit `83af469309ed1874780283b6115a16b87232893d`.
- `https://choir.news/health` reported deployed commit
  `83af469309ed1874780283b6115a16b87232893d` with deployed_at
  `2026-06-07T13:36:55Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/global-wire-app.spec.js --project=chromium --workers=1 --reporter=line`
  passed on staging: 4 tests.
- Browser plugin inspection of `http://127.0.0.1:5173/` confirmed
  `SourceMaxx desk`, 3 article VText open affordances, 16 sources in source
  chronology, first story `borderTopWidth: 0px`, first story
  `backgroundColor: rgba(0, 0, 0, 0)`, first source row transparent with no
  shadow, and source chronology text visible.
- `nix develop -c go test ./internal/runtime -run 'TestGlobalWire'` passed
  with no matching tests.
- `nix develop -c go test ./internal/store -run 'TestGlobalWire'` passed with
  no matching tests.
- `nix develop -c go test ./internal/sources ./cmd/sourcecycled` passed.
- `nix develop -c go test ./internal/cycle ./internal/sources ./internal/sourceapi ./cmd/sourcecycled`
  passed.
- `nix develop -c go test ./internal/runtime -run 'TestGlobalWire|TestSourceSearch|TestResearcher'`
  passed.
- Local live sourcecycled proof with
  `SOURCE_SERVICE_DB_PATH=/tmp/sourcecycled-sourcemaxx-proof.db
  SOURCE_SERVICE_ADDR=127.0.0.1:9876 nix develop -c go run ./cmd/sourcecycled`
  loaded 14 configured sources and completed the first cycle at
  `2026-06-07T13:50:27Z`.
- Local source service health for that proof reported `item_count: 710` and
  `fetch_count: 14`.
- Local `/internal/source-service/sourcemaxx/latest` reported cycle
  `cycle_b365d40c7c3e7db24a5fa864`, status `completed`, 710 SourceItems, 18
  queued processor handoffs, and 1 queued `story-corpus` reconciler handoff.
- Local source-type distribution for that proof was 500 GDELT items, 172 RSS
  items, and 38 Telegram items. Fetch status was `ok` for all 14 configured
  sources, with some feeds valid but empty for the cycle (`arxiv:cs_ai`,
  `rss:nikkei_asia`, `telegram:conflict_monitor`).
- Commit `32046f713f08c28bfcb735f12427adec8ab85749` was pushed to
  `origin/main`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27094523970` completed
  successfully for commit `32046f713f08c28bfcb735f12427adec8ab85749`,
  including staging deploy.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27094523958` completed
  successfully for commit `32046f713f08c28bfcb735f12427adec8ab85749`.
- `https://choir.news/health` reported deployed commit
  `32046f713f08c28bfcb735f12427adec8ab85749` with deployed_at
  `2026-06-07T13:55:50Z`.
- Public-edge probes of
  `https://choir.news/internal/source-service/health` and
  `https://choir.news/internal/source-service/sourcemaxx/latest` returned
  HTTP 403 with `internal routes are not available from the public edge`. This
  confirms the internal boundary; it does not prove staging sourcecycled volume.
- `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/global-wire-app.spec.js --project=chromium --workers=1 --reporter=line`
  passed on deployed staging after the source-runtime change: 4 tests.
- Commit `d43e22b66de181985e4e222dfb39d1288506053d` was pushed to
  `origin/main`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27094692531` completed
  successfully for commit `d43e22b66de181985e4e222dfb39d1288506053d`,
  including staging deploy.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27094692551` completed
  successfully for commit `d43e22b66de181985e4e222dfb39d1288506053d`.
- `https://choir.news/health` reported deployed commit
  `d43e22b66de181985e4e222dfb39d1288506053d` with deployed_at
  `2026-06-07T14:03:21Z`.
- `curl -i https://choir.news/api/global-wire/sourcemaxx-status` returned
  HTTP 401, matching authenticated Global Wire API behavior rather than a
  source-status-specific failure.
- Authenticated Playwright product-path probe using the real staging WebAuthn
  session helper returned HTTP 200 from
  `https://choir.news/api/global-wire/sourcemaxx-status`: cycle
  `cycle_8a3fd397a071c7d2b1f27b05`, status `completed`, started_at
  `2026-06-07T13:55:56Z`, ended_at `2026-06-07T13:55:57Z`, `item_count: 686`,
  `fetch_count: 14`, `processor_request_count: 17`,
  `reconciler_request_count: 1`, reconciler scope `story-corpus`, and
  `source_service_internal_only: true`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/global-wire-app.spec.js --project=chromium --workers=1 --reporter=line`
  passed on deployed staging after the aggregate-status route: 4 tests.

unproven or partial claims:

- sustained hundreds of source items per 15-minute window over multiple
  staging cycles; the authenticated product path proved one completed staging
  cycle with 686 items and 14 source fetches;
- exact per-source staging distribution, dedupe counts, provider health by
  source, and freshness over time;
- processor contracts and long-running context continuity;
- processor compaction with handles to full source content;
- reconciler contracts and corpus-level contradiction/question behavior across
  existing stories, current stories, and new source state;
- reuse of existing researcher agents from processor/reconciler/VText requests;
- reuse of existing VText agents for article writing/revision;
- intelligent Style.vtext routing and withholding/deprioritization;
- publication-quality VText output;
- source-processing behavior connected to product agents beyond durable queued
  handoff records.

belief-state changes:

- SourceMaxx requires resident processors plus corpus-level reconcilers, not
  just indexes or refresh endpoints.
- VText transclusion/version structure is the implicit graph; explicit indexes
  are accelerators, not authority.
- Existing researcher and VText agents are required infrastructure to reuse.
- Style.vtext routing is editorial judgment, not exhaustive permutation.
- UI correctness depends on source breadth and readability, not more panels.
- The previous frontend was optimizing the wrong visible object: it exposed
  artifact machinery as a dashboard before source breadth and article
  readability were solved. The SourceMaxx surface should stay text-led while
  deeper provenance remains available through VText/source disclosure.
- The previous sourcecycled loop also optimized the wrong object by attempting
  direct LLM issue synthesis after polling. The better deterministic boundary
  is SourceItem ledger plus processor/reconciler handoffs; resident agents and
  VText agents should perform cognition and publication.
- GDELT should be routed honestly as a global firehose until processors
  interpret it. The deterministic router should not pretend a feed-wide
  vertical label is semantic classification.
- The deployed product path can now prove aggregate source volume and handoff
  topology without exposing source-service `/internal/*` routes at the public
  edge. The remaining mission risk has moved from source-volume visibility to
  whether queued handoffs become resident processor/reconciler cognition and
  VText publication through existing agent loops.
- Processor and reconciler handoffs should be consumed by new first-class
  `processor` and `reconciler` agent profiles on the shared Choir harness, not
  by relabeling them as `researcher`, `super`, `co-super`, or a sourcecycled
  side loop. Their differences should live in role prompts, toolsets,
  request metadata, compaction policy, and product-visible state while sharing
  the same provider calls, run memory, continuation, channel, cancellation,
  retry, and event machinery as existing agents.
- Commit `27a70717f78d86982fe25f8e1e52c1dd20c0217e` connects SourceMaxx
  processor/reconciler handoffs to `/internal/runtime/runs` with
  `processor`/`reconciler` profile metadata and enables that dispatcher on
  Node B. CI, FlakeHub, staging health, and the Global Wire UI test are green
  for that commit. However, the authenticated product status route currently
  reports only handoff counts, keys, and scopes; it drops request status. That
  means product-path proof can see the source cycle and handoff topology, but
  cannot yet distinguish submitted shared-harness runs from queued or
  dispatch-failed handoffs. The next fix should expose non-sensitive
  processor/reconciler status counts through `/api/global-wire/sourcemaxx-status`
  before using it as dispatch acceptance evidence.
- Commit `65a74e60fee15ca4ce78b576fd9512deeb3eff34` adds those product-safe
  status counts and is deployed on staging. Authenticated staging proof for
  cycle `cycle_1b8dbcb84048e3cb949be6d0` now reports 686 SourceItems, 14
  fetches, 17 processor requests, 1 reconciler request,
  `processor_status_counts: {"dispatch_failed":7,"queued":10}`, and
  `reconciler_status_counts: {"dispatch_failed":1}`. This proves the shared
  harness dispatcher attempted the capped processor/reconciler submissions,
  but did not successfully create resident runs for that cycle. The likely
  root cause is Node B startup ordering: `sourcecycled` starts and runs its
  immediate cycle before the host-process sandbox at `127.0.0.1:8085` is ready,
  while the sandbox is ordered after sourcecycled. The next fix should avoid a
  hard service dependency cycle and make dispatch tolerate runtime startup by
  retrying transient runtime unavailability before marking handoffs
  `dispatch_failed`.
- Commit `d1f692f9e45c7b653d7909598019be5c744ea438` adds bounded retry for
  transient runtime dispatch failures. CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27095762398` and
  FlakeHub run `https://github.com/choir-hip/go-choir/actions/runs/27095762408`
  completed successfully, and `https://choir.news/health` reported proxy and
  sandbox deployed commit `d1f692f9e45c7b653d7909598019be5c744ea438` at
  `2026-06-07T14:48:30Z`. Authenticated staging proof for new post-deploy
  cycle `cycle_1e2aba70774480fdbf66ccbc` reported 686 SourceItems, 14
  fetches, 17 processor requests, 1 reconciler request,
  `processor_status_counts: {"submitted":7,"queued":10}`, and
  `reconciler_status_counts: {"submitted":1}`. This proves sourcecycled can
  now submit capped SourceMaxx processor/reconciler handoffs into first-class
  shared-harness agent profiles on staging. The 10 queued processor requests
  are the configured dispatch cap, not a failure. The remaining gap is
  resident agent result quality and lifecycle: processor/reconciler outputs,
  researcher delegation, VText delegation/publication, compaction/continuity,
  and publication-quality Style.vtext use.
- Current product evidence still cannot correlate individual SourceMaxx
  processor/reconciler handoffs to their resident runtime run records because
  sourcecycled persists request status but not `runtime_run_id`. The
  authenticated status route can prove aggregate submission counts, but it
  cannot yet inspect run state, errors, successful `submit_coagent_update`
  checkpoints, child researcher/VText runs, or other lifecycle evidence per
  handoff. The next fix should add durable run-id lineage to processor and
  reconciler request rows, expose only aggregate run-state/update evidence
  through the product-safe SourceMaxx status route, and keep raw prompts,
  internal run endpoints, and source-service internals private.
- Commit `677b3497ed4e7ac860cabae492d9ec6b226515a4` adds durable
  `runtime_run_id` columns for processor/reconciler requests, persists
  submitted runtime run IDs from `sourcecycled`, carries those IDs through the
  source-service response DTOs, and extends
  `/api/global-wire/sourcemaxx-status` with product-safe aggregate run-state,
  worker-update, and child-profile counts. CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27096173481` and
  FlakeHub run `https://github.com/choir-hip/go-choir/actions/runs/27096173478`
  completed successfully, and `https://choir.news/health` reported proxy and
  sandbox deployed commit `677b3497ed4e7ac860cabae492d9ec6b226515a4` at
  `2026-06-07T15:05:37Z`. Local focused tests passed for cycle storage,
  sourcecycled dispatch, and runtime status aggregation; local builds passed
  for `./cmd/sourcecycled` and `./cmd/sandbox`.
- Deployed authenticated SourceMaxx status proof after that commit returned
  cycle `cycle_f0d8b5056c405d541887b17d`, started
  `2026-06-07T15:05:44Z`, with 686 SourceItems, 14 fetches, 17 processor
  requests, 1 reconciler request,
  `processor_status_counts: {"queued":10,"submitted":7}`, and
  `reconciler_status_counts: {"submitted":1}`. It did not include the new
  runtime-run aggregate fields. This narrows the current problem: the shared
  harness submission path works at the status-count level, and the code now
  persists run IDs, but deployed product evidence still cannot resolve the
  submitted handoffs into runtime lifecycle evidence. The next fix must
  root-cause whether `sourcecycled` is running updated code, whether its
  SourceMaxx storage rows carry `runtime_run_id`, whether the source-service
  API is dropping those fields, or whether the sandbox runtime store cannot
  see the submitted runs because `sourcecycled` and sandbox are writing to
  different stores/owners. Do not add another agent architecture to patch
  around this; processors and reconcilers remain shared-harness profiles with
  profile-specific toolsets.
- Node B root-cause probe clarified the failure. Raw source-service
  `/internal/source-service/sourcemaxx/latest` for that same cycle includes 7
  processor `runtime_run_id` values and 1 reconciler `runtime_run_id`, so
  sourcecycled storage and DTO serialization are working. Sandbox logs show
  those exact run IDs were created, then immediately failed with
  `unsupported prompt role "processor"` and `unsupported prompt role
  "reconciler"`. The actual bug is that the shared runtime harness accepts
  processor/reconciler profiles at run submission and has tool registries for
  them, but `PromptStore.promptRoles()` does not register prompt defaults for
  those roles. The next fix should add processor and reconciler prompt
  defaults/registration in the shared prompt store, not create a separate
  processor/reconciler execution loop.
- Commit `8209adf3f281674b4d52a401f10c894270a9d271` adds processor and
  reconciler prompt defaults to the shared runtime prompt store and proves that
  these SourceMaxx roles load normal shared-harness prompts and tool catalogs.
  CI run `https://github.com/choir-hip/go-choir/actions/runs/27096467481` and
  FlakeHub run `https://github.com/choir-hip/go-choir/actions/runs/27096467476`
  completed successfully. Staging health reported proxy and sandbox commit
  `8209adf3f281674b4d52a401f10c894270a9d271`, deployed at
  `2026-06-07T15:17:41Z`.
- Post-deploy scheduled cycle `cycle_043b6ca2781a54d8b3b4f761` started at
  `2026-06-07T15:20:44Z` and completed at `2026-06-07T15:20:45Z` with 503
  SourceItems, 14 fetches, 11 processor requests, and 1 reconciler request.
  Raw source-service state shows 7 processor `runtime_run_id` values and 1
  reconciler `runtime_run_id`. Sandbox logs after the cycle show active
  processor/reconciler shared-harness tool loops and no
  `unsupported prompt role` failures. The authenticated product status route
  returns the same cycle and handoff/status counts, but still omits
  runtime-run aggregate fields because it only increments those fields after it
  resolves detailed run records from the request-serving runtime store. This
  is now a narrower product evidence problem, not a source ingestion or prompt
  role problem: the route should expose product-safe submitted runtime-run
  lineage counts directly from SourceMaxx request DTOs, while keeping detailed
  run-state/update/child-profile counts limited to records it can resolve.
- Commit `a5a5e8d86b67ef99ba1e630add84c29af4500481` fixes that
  product-safe projection gap. `processor_runtime_run_count` and
  `reconciler_runtime_run_count` now count submitted runtime IDs present in the
  SourceMaxx request lineage, while separate resolved/unresolved fields show
  whether detailed lifecycle records could be joined. Focused runtime and
  sourcecycled tests passed locally, `./cmd/sandbox` and `./cmd/sourcecycled`
  built locally, CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27096707813` succeeded,
  and FlakeHub run
  `https://github.com/choir-hip/go-choir/actions/runs/27096707817` succeeded.
  Staging health reported proxy and sandbox commit
  `a5a5e8d86b67ef99ba1e630add84c29af4500481`, deployed at
  `2026-06-07T15:27:31Z`.
- Deployed authenticated product proof against
  `/api/global-wire/sourcemaxx-status` now returns cycle
  `cycle_043b6ca2781a54d8b3b4f761` with 503 SourceItems, 14 fetches, 11
  processor requests, 1 reconciler request, `processor_runtime_run_count: 7`,
  `reconciler_runtime_run_count: 1`,
  `processor_unresolved_runtime_run_count: 7`, and
  `reconciler_unresolved_runtime_run_count: 1`. The unresolved counts are
  intentional evidence rather than hidden failure: product status can now prove
  submitted runtime lineage from source-service handoffs, while detailed
  lifecycle state still needs a product-safe join to the runtime store that
  accepted the sourcecycled runs. Sandbox logs remain the evidence that those
  submitted runs entered active shared-harness tool loops after the prompt-role
  fix.
- Lifecycle-output probe after that proof found a separate runtime availability
  issue. The 15:20 SourceMaxx processor/reconciler runs entered active tool
  loops and made repeated provider/tool calls, but the `a5a5e8d8` deploy
  restarted `go-choir-gateway.service` at `2026-06-07T15:27:48Z` while those
  runs were still in progress. Several runs then failed with
  `gateway call failed: Post "http://127.0.0.1:8084/provider/v1/inference":
  dial tcp 127.0.0.1:8084: connect: connection refused`. Gateway logs also
  show Fireworks 429 pressure before the restart. The active gateway is healthy
  again after the deploy (`/health` reports `status: ok`). This is a deployment
  interruption/backpressure finding, not a SourceMaxx role or toolset failure.
  The next proof should use a post-deploy scheduled cycle with gateway already
  stable, then check for processor/reconciler `submit_coagent_update`,
  researcher/VText delegation, and produced lifecycle output.
- Clean post-deploy cycle `cycle_448e8ace146d7a6579ce9a6b` started at
  `2026-06-07T15:35:44Z` and completed at `2026-06-07T15:35:45Z` with 502
  SourceItems, 14 fetches, 11 processor requests, and 1 reconciler request.
  Product status returned the same cycle with 7 processor runtime IDs and 1
  reconciler runtime ID. Node B internal runtime evidence for those submitted
  run IDs showed six processor runs completed cleanly by `15:38:24Z`, one
  processor was still running, and the reconciler was still running. Completed
  processors called `submit_coagent_update`; the running processor called
  `spawn_agent` twice and `wait_agent` once; the reconciler called
  `spawn_agent` three times and `wait_agent` once. Tool evidence also included
  `source_search`, `read_content_item`, `fetch_url`, `save_evidence`,
  `web_search`, and evidence/listing tools. Gateway logs for the clean cycle
  showed sustained Fireworks inference successes and search successes without
  the deploy-window `connect: connection refused` failure.
- The browser-visible status route still reports
  `processor_unresolved_runtime_run_count: 7` and
  `reconciler_unresolved_runtime_run_count: 1` for the clean cycle, even
  though the Node B internal runtime endpoint can resolve the same run IDs and
  list their events with `owner_id=global-wire-platform`. The root cause is
  now narrower: `/api/global-wire/sourcemaxx-status` joins lifecycle evidence
  through the request-serving runtime store, while sourcecycled submits these
  global SourceMaxx runs to the Node B runtime endpoint configured by
  `SOURCE_SERVICE_RUNTIME_BASE_URL`/`SOURCECYCLED_RUNTIME_BASE_URL`. The next
  code fix should make the product-safe status projection resolve lifecycle
  evidence through the same configured runtime endpoint that accepted the
  SourceMaxx submissions, using internal-caller auth over Node B only, instead
  of assuming the current handler's local store owns the submitted run IDs.
- Commit `4ed111a4e78a9d76945ebc3b40bafa1340303fa0` added that remote
  runtime lifecycle resolver and Node B host sandbox env for
  `SOURCE_SERVICE_RUNTIME_BASE_URL=http://127.0.0.1:8085`. CI, FlakeHub, and
  staging deploy passed, and `/health` reported the same deployed commit at
  `2026-06-07T15:48:19Z`. The next product proof still showed unresolved
  lifecycle joins for cycle `cycle_5b41c5876cc623de9ff30d69`: 686 SourceItems,
  14 fetches, 17 processor requests, 1 reconciler request, 7 processor runtime
  IDs, 1 reconciler runtime ID, and all 8 runtime IDs unresolved in
  `/api/global-wire/sourcemaxx-status`.
- Node B direct evidence narrowed the failure: `go-choir-sandbox.service` now
  has both `SOURCE_SERVICE_BASE_URL` and `SOURCE_SERVICE_RUNTIME_BASE_URL`, and
  `curl -H "X-Internal-Caller: true"
  http://127.0.0.1:8085/internal/runtime/runs/bd4f63f9-96d5-4b13-bc3e-010f1bebee1e?owner_id=global-wire-platform`
  returned HTTP 200 for a submitted processor run. The remaining likely root
  cause is cross-boundary environment wiring: VM boot args pass
  `choir.source_service_url` into sandbox VMs as `SOURCE_SERVICE_BASE_URL`, but
  they do not yet pass a matching runtime lifecycle URL/owner into the
  request-serving VM sandbox. The product route can therefore fetch SourceMaxx
  cycle summaries while still lacking the remote runtime resolver it needs to
  walk submitted agent run IDs.
- Commit `79a57893c59d8dc5c59e0a8054f5573cc4a5a3c7` passed local focused
  VM/runtime tests, CI run `27097492636`, FlakeHub run `27097492643`, and Node
  B deploy. Staging `/health` reported proxy and sandbox commit
  `79a57893c59d8dc5c59e0a8054f5573cc4a5a3c7`, `status: ok`, and
  `vmctl_status: ok` at deployed_at `2026-06-07T15:59:18Z`.
- Product-path authenticated public API proof still reported unresolved runtime
  lifecycle counts for the fresh post-deploy cycle
  `cycle_620122d38e3a67282f74b420`: 500 SourceItems, 14 fetches, 10 processor
  requests, 1 reconciler request, 7 processor runtime IDs, 1 reconciler runtime
  ID, and all 8 runtime IDs unresolved. A fresh synthetic owner
  `sourcemaxx-probe-1780848559` received the same unresolved counts, proving
  this was not only stale pre-deploy VM state.
- Node B vmctl evidence for that fresh owner showed a current VM
  `vm-bbc9d650d34598678cfa7f72ed8ac8aa` at sandbox URL
  `http://10.202.142.2:8085`, running commit `79a57893`. Its Firecracker boot
  args included `choir.source_service_runtime_url=http://10.202.142.1:8085`
  and `choir.source_service_runtime_owner_id=global-wire-platform`. Direct Node
  B runtime lookup of one submitted processor run
  `56affd8b-d443-4897-a1ee-166945bcf360` returned HTTP 200 with
  `agent_profile: processor`, `state: running`, and
  `request_source: sourcecycled`.
- Root cause is now identified: VM guests receive the right runtime lifecycle
  URL, but `tapReachableHostServicePorts()` admits/DNATs only host ports 8083,
  8084, 8087, and 8787. It does not include 8085, so guest sandboxes can reach
  the source-service summary on 8787 but cannot reach the host sandbox runtime
  lifecycle endpoint on 8085. The next code fix should add 8085 as a
  tap-reachable host service only for this internal runtime lifecycle evidence
  path, with tests documenting that this is not browser-public exposure.
- Commit `68166bfe9be182504ed7c4b3e0d621d2ce0261fd` added host sandbox
  runtime port 8085 to the per-VM tap-reachable host service allowlist. Focused
  local proof passed:
  `nix develop -c go test ./internal/vmmanager -run
  'TestTapReachableHostServicePortsIncludeHostPrivateServices|TestTapHostServiceInputRuleSpec|TestBuildFirecrackerConfig_MicrovmUsesStoreDiskAndKernelParams'`,
  `nix develop -c go build ./cmd/vmctl`, and `git diff --check`.
- Deployed proof for `68166bfe9be182504ed7c4b3e0d621d2ce0261fd` passed: CI
  run `27097898836`, FlakeHub run `27097898843`, and Node B deploy all
  succeeded. `https://choir.news/health` reported `status: ok`,
  `vmctl_status: ok`, and proxy/sandbox deployed commit
  `68166bfe9be182504ed7c4b3e0d621d2ce0261fd` at
  `2026-06-07T16:15:37Z`.
- Product-path authenticated public API proof with fresh owner
  `sourcemaxx-proof-1780849103` returned HTTP 200 for
  `/api/global-wire/sourcemaxx-status` and resolved SourceMaxx lifecycle
  evidence for cycle `cycle_620122d38e3a67282f74b420`: 500 SourceItems, 14
  fetches, 10 processor requests, 1 reconciler request, 7/7 processor runtime
  runs resolved as completed, 1/1 reconciler runtime run resolved as completed,
  processor update count 10, reconciler update count 2, and reconciler child
  profile counts `researcher: 2` and `vtext: 5`. The source firehose ->
  shared-harness processors/reconciler -> researcher/VText delegation evidence
  surface is now staging-proven at aggregate product status level.

remaining error field:

- sustained staging source daemon/storage behavior across repeated cycles,
  including provider-level distribution, freshness, dedupe, and backoff;
- first-class processor/reconciler shared-harness profiles are present,
  sourcecycled submits capped staging handoffs to them, staging logs show
  shared-harness tool loops, and browser-visible product status now resolves
  aggregate lifecycle evidence including processor/reconciler completion,
  updates, researcher/VText child delegation, and canonical VText article
  ownership; publication-quality Style.vtext selection and clean news UI
  production remain incomplete;
- deploys can interrupt active SourceMaxx runs because gateway/sandbox restart
  while processor/reconciler loops are mid-inference; provider 429 pressure was
  also observed under the current dispatch volume;
- processor load budget and routing scheme after live staging data;
- current researcher/VText agent invocation contracts for this workflow;
- deletion/reuse map for current Global Wire backend source paths;
- backend still uses many `StoryGraph` names and deferred contribution,
  newsletter, and Autoradio endpoints that should be audited before further
  product exposure.

highest-impact remaining uncertainty: how to make the VText articles
publication-quality through explicit Style.vtext source selection/composition
and how to surface those VTexts in the clean newspaper UI without reviving the
old busy panel/card design.

2026-06-07 VText normalization finding:

- Staging lifecycle evidence for SourceMaxx cycle
  `cycle_620122d38e3a67282f74b420` proves source volume and shared-harness
  processor/reconciler execution, but it does not prove canonical article
  ownership by VText.
- Direct Node B runtime evidence for reconciler run
  `e04814e0-c8ce-458b-9a81-0254808ec53a` shows `spawn_agent role=vtext`
  returned generic child runs such as
  `e347b4ea-3fe7-48fe-8939-c9d328cf2cb9`,
  `ff329968-122a-4457-97be-439ffc595d2a`, and
  `a6e65d64-7feb-4700-9116-87fb4c7c3504` on channel
  `reconciler:story-corpus`, with agent ids that are UUIDs rather than
  `vtext:<doc_id>` handles.
- One child run explicitly reported the root symptom: it produced a complete
  article but could not create the canonical document revision because
  `edit_vtext` requires a `vtext_agent_revision` run. This confirms that
  current processor/reconciler usage treats VText like a generic writer tool
  instead of the durable owner of the article.
- The normalized workflow must match prompt-bar VText topology: processors and
  reconcilers may decide that a story or update is needed, but the runtime must
  create/select a normal VText document, persist a source/brief seed revision,
  start an existing VText agent revision run with `agent_id=vtext:<doc_id>`,
  `channel_id=<doc_id>`, and `type=vtext_agent_revision`, and require the VText
  agent to call `edit_vtext`. Article text in generic run results is not a
  shipped story artifact.
- The next code fix should add a first-class processor/reconciler affordance
  for normal VText article/revision creation while preserving shared harness
  mechanics and existing researcher/VText agents. It should not create a
  parallel story table, a second writer role, or a special SourceMaxx-only
  document owner.

2026-06-07 VText normalization delivery:

- Commit `020d68467bc020a403939d1e1cc2913ef9a1589a` normalized
  processor/reconciler `spawn_agent role=vtext` delegation. For processor and
  reconciler callers, VText delegation now creates or selects a normal VText
  document, writes a SourceMaxx brief seed revision, persists `vtext:<doc_id>`
  as the appagent, and starts an existing VText `vtext_agent_revision` run on
  the document channel. Existing VText documents can be revised by passing the
  document id as `channel_id`.
- Local proof passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run
  TestProcessorAndReconcilerProfilesShareHarnessAndDelegateToResearcherOrVText`,
  and `nix develop -c go test ./internal/runtime -run
  'TestHandleInternalRunSubmissionAllowsSourceMaxxProcessorAndReconcilerProfiles|TestPromptStore|TestVText'`.
- CI/deploy proof passed for `020d68467bc020a403939d1e1cc2913ef9a1589a`:
  CI run `27098388192`, FlakeHub run `27098388205`, and Node B staging deploy
  all succeeded. `https://choir.news/health` reported proxy and sandbox
  deployed commit `020d68467bc020a403939d1e1cc2913ef9a1589a` at
  `2026-06-07T16:36:30Z`.
- A fresh post-deploy SourceMaxx cycle was forced by restarting only
  `go-choir-sourcecycled.service`, which intentionally runs a first cycle on
  daemon start. The cycle `cycle_690af0efdfdd6727b4b29643` fetched 686 items
  from 14 fetches and queued 17 processor requests plus 1 reconciler request.
  Authenticated public product status
  `/api/global-wire/sourcemaxx-status` resolved 7 processor runs and 1
  reconciler run for that cycle; after execution it reported the reconciler
  completed with child profile counts `vtext: 4`.
- Direct runtime event evidence for reconciler run
  `ae2b81bf-3db4-43e7-8078-0370d7961c2b` showed four VText child results with
  `agent_id=vtext:<doc_id>`, `channel_id=<doc_id>`, `seed_revision_id`, and
  `revision_loop_id`: docs
  `c93608d5-8698-49d1-98c2-093432d06f86`,
  `39146705-ebe6-4b20-a2f2-feaa5e46c9e2`,
  `784a39fa-dd9d-4f54-a3c0-24c4b39fab53`, and
  `2fbeb7e2-7e23-4a17-b960-6cbd53cfb8a4`.
- Dolt VText store proof showed all four documents existed with `.vtext`
  titles and current canonical heads. The head revisions were version `2`,
  `author_kind=appagent`, had metadata containing `source=edit_vtext` and
  `source_maxx_cycle_id`, and contained article text. This proves the fixed
  path produces canonical VText revisions rather than generic writer run
  output.
- New residual gap from the same proof: the four article heads did not mention
  or cite Style.vtext selection (`mentions_style=false` in the proof query).
  The next delivery loop must make Style.vtext a real cited editorial source
  in the VText revision prompt/context, not merely an optional sentence in the
  agent instructions.

next executable delivery loop:

1. Add explicit Style.vtext source selection/composition to the normalized
   processor/reconciler -> VText route so each article revision receives
   citeable style source context and records why the selected style fits.
2. Keep processors and reconcilers on the shared runtime harness with
   profile-specific prompts/toolsets only. Do not create a separate processor
   service loop to mask lineage gaps.
3. Route processor/reconciler research needs into existing researcher agents
   and route article/update needs into existing VText agents. Do not create a
   parallel researcher/writer system.
4. Replace remaining wrong-object paths while preserving product topology:
   high-volume source ingestion, durable SourceItems, routing, processor state,
   reconciler corpus review, researcher request/result reuse, VText
   write/revision reuse, Style.vtext routing, VText traversal/source indexes,
   and user-owned VText boundaries.
5. Discard or selectively mine the stashed source-refresh experiment only if it
   helps the delivered architecture; do not revive click-time source refresh as
   the product object.
6. Build through to staging behavior: tests, commit, push, CI/deploy monitor,
   staging identity, product-path source volume, processor/reconciler evidence,
   researcher/VText reuse evidence, ownership evidence, and browser screenshots.
7. Perform a quality pass before claiming delivery: simplify names and data
   flows, remove obsolete panels/routes/tests, make Style.vtexts publication
   quality, and make the Global Wire UI nice in Futuristic Noir, Carbon Fiber
   Kintsugi, London Salmon, and responsive Choir web desktop layouts.

2026-06-07 Style.vtext selection delivery and publication-quality gap:

- Commit `07bd7c687ac3ecaf9b052ede1cdd23513ddf77f3` added selective
  Style.vtext source context to the normalized processor/reconciler -> VText
  route. The runtime now records `selected_style_sources` and
  `selected_style_rationale` on the VText seed revision and VText agent
  revision run, and the VText prompt includes the selected Style.vtext source
  context instead of asking every story to run every style.
- Local proof passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run
  TestProcessorAndReconcilerProfilesShareHarnessAndDelegateToResearcherOrVText`,
  `nix develop -c go test ./internal/runtime -run
  'TestHandleInternalRunSubmissionAllowsSourceMaxxProcessorAndReconcilerProfiles|TestPromptStore|TestVText'`,
  and `git diff --check`.
- CI/deploy proof passed for `07bd7c687ac3ecaf9b052ede1cdd23513ddf77f3`:
  CI run `27098733233`, FlakeHub run `27098733232`, and Node B staging deploy
  all succeeded. `https://choir.news/health` reported proxy and sandbox
  deployed commit `07bd7c687ac3ecaf9b052ede1cdd23513ddf77f3` at
  `2026-06-07T16:50:58Z`.
- A fresh post-deploy SourceMaxx cycle, `cycle_419cbad08d2defc39978e709`, was
  forced by restarting `go-choir-sourcecycled.service`. The cycle fetched 686
  deduped SourceItems from 14 fetches, queued 17 processor requests and 1
  reconciler request, and public authenticated product status resolved 7
  processor runs and 1 reconciler run. During the proof window it reported
  processor states `completed: 5`, `failed: 1`, `running: 1`, and reconciler
  child profile counts `vtext: 4`; direct runtime events showed the reconciler
  actually spawned 7 VText article documents and 3 researcher agents.
- Direct runtime proof for the seven VText child runs showed all completed as
  normal `vtext:<doc_id>` appagent revision runs, carried the fresh
  `source_maxx_cycle_id`, and had selected Style.vtext metadata. Six general
  news stories selected `Style.vtext: Global Wire`; the SpaceX IPO story
  selected `Style.vtext: Market Brief`, proving story-aware style matching
  instead of all-styles-per-story.
- Durable VText store proof showed the seven current document heads had
  `author_kind=appagent`, `source=edit_vtext`, `selected_style_sources`
  metadata, and content that mentions `style.vtext`.
- New residual gap from the same proof: the VText current heads were not
  consistently publication-quality articles. Several heads remained
  `SourceMaxx Brief`, `Working Revision`, or `Evidence Gathering` versions
  despite completed VText runs. This means the route now proves normalized
  ownership and style selection, but it does not yet prove publication-quality
  delivered articles. The next code change should make SourceMaxx VText
  revision runs converge to article heads before completion, or surface a
  precise incomplete/publication-needed state rather than treating a seed or
  evidence-gathering revision as a completed article.

updated remaining error field:

- Style.vtext source selection is now staging-proven as metadata and prompt
  context, but publication-quality article completion is not yet proven. A
  completed VText child run may leave the current VText head as a brief or
  evidence-gathering revision.
- One processor failed during `cycle_419cbad08d2defc39978e709`; the failure
  has not yet been root-caused. It did not block reconciler VText spawning, but
  it remains a runtime reliability gap.
- Public status aggregation reported `reconciler_child_profile_counts:
  vtext: 4` while direct runtime events showed 7 VText spawn results. The
  status endpoint is useful for aggregate proof but may undercount child
  profiles while runs are still progressing.
- The clean newspaper UI remains the highest-value visible product gap after
  the VText article-completion fix. It must use the design language without
  panels, cards, nested scroll, visible theme selector, or noisy "Open in
  VText" labels.

2026-06-07 article-head prompt delivery and SourceItem accessibility blocker:

- Commit `d2a8ebe7a8245258544eb2395a9777236aaab20f` changed the SourceMaxx
  VText revision contract so VText remains the document owner but SourceMaxx
  handoffs are treated as grounded newsroom source context. SourceMaxx VText
  revision runs now require the first `edit_vtext` call to write a publishable
  article or correction/update draft, not a `SourceMaxx Brief`,
  `Working Revision`, `Evidence Gathering` note, outline, or placeholder.
  Ordinary prompt-bar VText runs still preserve the cautious
  working-response-first behavior for ungrounded factual/current prompts.
- Local proof passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestProcessorAndReconcilerProfilesShareHarnessAndDelegateToResearcherOrVText|TestSystemPromptForSourceMaxxVTextRunsRequiresArticleHead'`,
  `nix develop -c go test ./internal/runtime -run
  'TestHandleInternalRunSubmissionAllowsSourceMaxxProcessorAndReconcilerProfiles|TestPromptStore|TestVText'`,
  and `git diff --check`. A broader comprehensive prompt-profile command
  including `TestSystemPromptForSourceMaxxProfilesLoadsSharedHarnessPrompts`
  still fails on a pre-existing `Available tools:` expectation mismatch and
  was not used as proof for this change.
- CI/deploy proof passed for `d2a8ebe7a8245258544eb2395a9777236aaab20f`: CI
  run `27099098435`, FlakeHub run `27099098438`, and Node B staging deploy all
  succeeded. `https://choir.news/health` reported proxy and sandbox deployed
  commit `d2a8ebe7a8245258544eb2395a9777236aaab20f` at
  `2026-06-07T17:05:49Z`.
- A fresh post-deploy SourceMaxx cycle, `cycle_a59a364c43c0eed6d05a2a62`, was
  forced by restarting `go-choir-sourcecycled.service`. The cycle fetched 686
  deduped SourceItems from 14 fetches and queued 17 processor requests plus 1
  reconciler request. Authenticated public product status resolved 7 processor
  runs and 1 reconciler run; at proof time it reported processor states
  `completed: 6`, `running: 1`, and reconciler state `completed: 1`.
- The VText article-head proof did not execute because the reconciler spawned
  no VText children. Direct reconciler runtime events for run
  `ad28ea70-8707-4936-932a-1608e2ca5bc5` show the reconciler found a severe
  upstream SourceMaxx/source-service problem instead of article candidates:
  all 17 processor handles resolved to the same arXiv paper
  `2606.04850` / source item `srcitem_5edf1e34e3ac253df3d38899`, while
  queries for the 686 `srcitem_*` IDs listed in the reconciler request returned
  zero source-service results. The reconciler saved evidence ids
  `a393eff3-c0a5-4691-ba14-0270bc34f15c`,
  `35b3c7e4-7e15-4b28-af76-000de3f73bc3`, and
  `a80be16e-458a-4c8e-9694-2b62ff5a0c42`.
- This is now the highest-value blocker: SourceMaxx can fetch hundreds of
  items, but processor/reconciler handoff references are not reliably walkable
  through the source-service tool path. The next behavior change must root
  cause and repair source item lookup/routing so processors and reconcilers
  receive diverse, accessible SourceItem handles before more UI work or
  article-quality proof can be meaningful.

updated remaining error field:

- SourceMaxx source firehose volume is staging-proven, but source item
  accessibility is not: reconciler source_search returned zero results for
  the 686 request-listed `srcitem_*` handles in `cycle_a59a...`.
- Processor/reconciler request diversity is not proven: the fresh staging
  proof found 17 processor handoffs collapsing onto one arXiv source item.
- The article-head VText prompt fix is committed, tested, and deployed, but
  not yet staging-proven end-to-end because the latest reconciler correctly
  declined to spawn articles from broken source context.
- The next proof must show diverse, source-searchable SourceItems flowing into
  processors/reconcilers, followed by VText child documents whose current
  heads are publication article drafts with selected Style.vtext notes.

2026-06-07 source handle lookup repair and article-head staging proof:

- Commit `85751dc5c2968bec0ad6ad67165d31b17f6f8b6b` repaired the deployed
  Source Service search path for durable SourceItem handles. `SearchItems`
  now detects `srcitem_*` handles, including `source_service_item:<id>`
  citations and handles embedded in natural language, and resolves them by
  exact item id before falling back to lexical search.
- Local proof passed:
  `nix develop -c go test ./internal/cycle -run
  'TestSearchItemsResolvesDurableSourceItemHandles|TestSearchItemsTokenizesNaturalQueriesAndRanksMatches|TestBuildSourceMaxxHandoffRoutesSourceItemsToProcessorsAndReconciler'`,
  `nix develop -c go test ./cmd/sourcecycled -run
  'TestSourceServiceAPISearchAndResolveItems|TestSourceServiceAPISourceMaxxLatestReportsAgentHandoffs|TestSourceMaxxRuntimeDispatcherSubmitsProcessorAndReconcilerProfiles'`,
  and `git diff --check`.
- CI/deploy proof passed for `85751dc5c2968bec0ad6ad67165d31b17f6f8b6b`: CI
  run `27099394514`, FlakeHub run `27099394517`, and Node B staging deploy all
  succeeded. `https://choir.news/health` reported proxy and sandbox deployed
  commit `85751dc5c2968bec0ad6ad67165d31b17f6f8b6b` at
  `2026-06-07T17:18:48Z`.
- Direct staging Source Service proof after deploy showed exact handle queries
  now resolve:
  `srcitem_003f7a703a9160b3ebce75cb` and
  `source_service_item:srcitem_003f7a703a9160b3ebce75cb` both returned the
  Liveuamap Telegram SourceItem, and
  `srcitem_ff8cd01eb2b7ac445cf3f4fa` returned the GDELT item from
  `morungexpress.com`.
- A fresh post-fix SourceMaxx cycle, `cycle_7aae518f7f50f008fb3998a8`, fetched
  686 deduped SourceItems from 14 fetches and queued 17 processor requests plus
  1 reconciler request. Authenticated public product status later reported 7
  processor runtime runs, 1 reconciler runtime run, processor child profile
  counts `researcher: 10`, `vtext: 3`, and reconciler child profile counts
  `researcher: 1`, `vtext: 3`.
- Direct reconciler event proof for run
  `5afb6d10-b402-40d5-9aa4-9e15c34492b0` showed three normal VText child
  article documents plus one researcher:
  `177628ae-9f72-4e02-9626-c52179e34c4a` / loop
  `f08871be-d487-4d1a-83d3-73a4c7e3e1f8` for Pope Leo XIV Madrid,
  `e300c4cd-70a8-4c94-8aaf-9627426bcaa0` / loop
  `0bac5639-87ec-4975-b2f7-cd5bd2217784` for the Delhi hotel fire, and
  `6dda0284-eb82-47e0-9924-92c6cc14b288` / loop
  `e689b96f-0ade-4c8e-9f84-74b014b030f5` for the Congo Ebola / World Cup
  travel-restriction story.
- All three VText child runs completed as normal `vtext:<doc_id>` appagent
  revision runs, carried `source_maxx_cycle_id:
  cycle_7aae518f7f50f008fb3998a8`, selected `Style.vtext: Global Wire`, and
  recorded the selected-style rationale.
- Durable VText store proof showed the current document heads for all three
  docs had `author_kind=appagent`, metadata containing `source=edit_vtext` and
  `selected_style_sources`, content mentioning `style.vtext`, and no
  `SourceMaxx Brief`, `Working Revision`, or `Evidence Gathering` markers. The
  content prefixes were publication article drafts:
  `MADRID -- Pope Leo XIV...`, `NEW DELHI -- The death toll...`, and a Congo
  Ebola / World Cup travel-restriction lead.

updated remaining error field:

- The SourceMaxx -> source_search -> processor/reconciler -> researcher/VText
  path is now staging-proven for a high-volume cycle, source-handle
  resolution, VText ownership, Style.vtext metadata, and article-head
  generation at three-document scale.
- The proof is still not mission-complete: some processor runs remain running
  during the proof window, the reconciler was still running after spawning
  three VTexts, and the clean newspaper UI has not been rebuilt/proven.
- SourceMaxx breadth is still only 14 configured fetches in this deployed
  proof. The source-maxx target needs many more RSS/Telegram/GDELT/provider
  feeds and provider health/backoff visibility before calling the firehose
  mature.
- The next highest-value axis is the Global Wire UI: show these VText article
  heads in clean newspaper columns with quiet repeated VText-open affordances
  and a source chronology column, across Futuristic Noir, Carbon Fiber
  Kintsugi, and London Salmon, without cards, borders, nested scrolling, or
  panel repetition.

suggested resume goal string: use the Goal String section above.

evidence artifact refs: local browser screenshot emitted through Browser
plugin during this run; focused frontend/source test commands, staging
acceptance command, and local sourcecycled proof DB
`/tmp/sourcecycled-sourcemaxx-proof.db` listed above.

rollback refs: prior branch/worktree state before this mission;
`stash@{1}` named
`superseded-global-wire-source-refresh-batch-experiment-2026-06-07` preserves
the abandoned source-refresh batch edits; `stash@{0}` named
`wrong-pipeline-processor-reconciler-request-slice-2026-06-07` preserves the
abandoned pipeline-shaped processor/reconciler edits; behavior commits must
record their own rollback SHAs. The shipped SourceMaxx UI checkpoint rollback
SHA is the parent of `83af469309ed1874780283b6115a16b87232893d`.

2026-06-07 clean newspaper UI checkpoint:

- User review of the prior Global Wire surface found the object visually wrong:
  too busy, panel/card-heavy, nested-scrolling, repetitive, and unreadable. The
  target UI is a clean newspaper-like VText collection surface: article text in
  columns, no story boxes, no app-local theme selector, no visible repeated
  `Open in VText` labels, no contribution panel, and no Autoradio surface in
  this mission slice. The source chronology remains important because the
  product must signal SourceMaxx breadth and provenance without becoming a
  dashboard.
- Architecture review also clarified the UI object: Global Wire is not the
  owner/editor of articles. VText agents own article documents. Processors and
  reconcilers request normal VText agent revisions through the shared harness,
  and the Global Wire app indexes/transcludes article VTexts, opens them in the
  VText app, and leaves edits/forks/style changes to normal VText flows.
- Local implementation work after this checkpoint updates the app toward that
  object: masthead `Global Wire` with quiet `SourceMaxx newsroom` signal,
  registry description `SourceMaxx VText newspaper`, stable unboxed article
  columns, every story openable/forkable through compact VText affordances, and
  responsive one-column mobile behavior inside the Choir desktop shell.
- Local proof before staging: `PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 pnpm
  --dir frontend exec playwright test tests/global-wire-app.spec.js` passed
  4/4, proving all three preview articles are openable as VText, Style.vtext
  provenance stays compact, the old StoryGraph/Autoradio/Contribute/theme
  surfaces are absent, and the surface remains responsive across Futuristic
  Noir, Carbon Fiber Kintsugi, and London Salmon. `pnpm --dir frontend build`
  passed with the existing large-chunk warning. Browser screenshots captured
  `/tmp/global-wire-desktop-v2.png` and `/tmp/global-wire-mobile-v3.png` for
  visual inspection.

updated remaining error field:

- Clean UI local proof is in progress but not yet staging-proven. The next
  behavior commit must be pushed, CI/deploy monitored, staging commit identity
  verified, and the deployed Global Wire product path re-tested.
- The preview UI now reflects the VText-owned article architecture, but the
  authenticated deployed data path still needs proof that fresh SourceMaxx
  article heads appear in this cleaner newspaper surface instead of only seeded
  preview records.
- Firehose breadth is still not mature: the strongest staging proof remains
  686 deduped items from 14 fetches. SourceMaxx still needs many more RSS,
  Telegram, GDELT/provider feeds and health/backoff visibility before calling
  ingestion complete.

2026-06-07 clean newspaper UI shipped proof:

- Documentation checkpoint commit:
  `2194e6a4b0b6e6d3cf0fb0a94304eebc8f591e31`
  (`docs: checkpoint sourcemaxx newspaper ui`).
- Behavior commit:
  `0c784a0073ff0bec4dea144360a01cfdf7f14df9`
  (`frontend: clean up global wire newspaper view`).
- Local proof before push:
  - `PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 pnpm --dir frontend exec
    playwright test tests/global-wire-app.spec.js` passed 4/4.
  - `pnpm --dir frontend build` passed with the existing large-chunk warning.
  - `git diff --check` passed.
- CI/deploy proof for `0c784a0073ff0bec4dea144360a01cfdf7f14df9`:
  - GitHub Actions CI run `27099927834` passed, including frontend build,
    non-runtime tests, runtime shards, vet/build, and Node B staging deploy.
  - FlakeHub publish run `27099927829` passed.
  - `https://choir.news/health` reported proxy and sandbox deployed commit
    `0c784a0073ff0bec4dea144360a01cfdf7f14df9`, with deployed_at
    `2026-06-07T17:41:30Z`.
- Deployed acceptance proof:
  `PLAYWRIGHT_BASE_URL=https://choir.news pnpm --dir frontend exec playwright
  test tests/global-wire-app.spec.js` passed 4/4. This proves the deployed
  Global Wire preview surface has the Global Wire masthead, SourceMaxx
  newsroom signal, three unboxed article columns, all stories openable as
  normal VTexts, compact Style.vtext provenance, no app-local theme selector,
  no StoryGraph desk/news-desk label, no Autoradio or contribution surface,
  and responsive behavior across Futuristic Noir, Carbon Fiber Kintsugi, and
  London Salmon.
- Browser visual proof captured deployed screenshot
  `/tmp/global-wire-staging-0c784a00.png`.

updated remaining error field:

- Clean newspaper UI is now shipped and staging-proven for the public preview
  product path.
- This is still not mission-complete. The next high-value axis is authenticated
  Global Wire data realism: prove that fresh SourceMaxx VText article heads
  from processor/reconciler runs appear in the clean surface, not only seeded
  preview stories.
- SourceMaxx breadth remains below the requested source-maxxing target. The
  deployed firehose proof remains hundreds of items per cycle but only 14
  configured fetches; add many more RSS, Telegram, GDELT/provider feeds with
  provider health/backoff visibility.
- User-published VText feed exploration remains future work. The app now
  preserves the right architecture for it by treating article reading/editing
  as normal VText work, but the public user-published VText feed is not yet
  implemented.

2026-06-07 SourceMaxx VText indexing problem:

- Current deployed proof has two truths that are not yet joined:
  SourceMaxx processors/reconcilers have produced normal VText article
  documents with `source_maxx_cycle_id` and selected `Style.vtext` metadata,
  and the Global Wire UI is now a clean VText newspaper surface. However,
  `/api/global-wire/stories` still returns owner-scoped seeded Global Wire
  records via `ListGlobalWireStories`, while unauthenticated users see seeded
  frontend preview records. The fresh SourceMaxx VText article heads are not
  yet indexed into the Global Wire story collection surface.
- This is an architecture mismatch against the clarified product object:
  VText agents own articles, and Global Wire should index/transclude article
  VTexts. The old durable `GlobalWireStory`/StoryGraph-shaped seed path is now
  useful only as compatibility scaffolding and should not remain the sole
  source for the newspaper.
- The next behavior change should add a VText-native SourceMaxx article index
  path: find recent platform-owned VText revisions whose metadata marks them
  as SourceMaxx article revisions, project their current heads into
  `GlobalWireStory` response rows with VText doc ids, source/style provenance,
  and article text, and let the existing clean UI render them. This is an
  index over VTexts, not a new canonical article data structure.
- The index must preserve invariants: platform stories are not mutated by user
  edits; opening/forking happens through normal VText; source/style provenance
  remains per VText version; and SourceMaxx status remains non-oracle evidence
  rather than a global truth feed.

2026-06-07 deployed SourceMaxx VText index proof failed:

- Behavior commit `81cb3cbf49c6c189672dd5d496a7b030c793e68f`
  (`runtime: index sourcemaxx vtexts in global wire`) is deployed on staging.
  `https://choir.news/health` reports both proxy and sandbox at that commit,
  deployed at `2026-06-07T17:57:38Z`.
- Public deployed UI acceptance still passes:
  `PLAYWRIGHT_BASE_URL=https://choir.news pnpm --dir frontend exec playwright
  test tests/global-wire-app.spec.js` passed 4/4, proving the clean newspaper
  preview surface remains intact.
- Stronger authenticated product-path proof contradicted the intended data
  realism: a real passkey browser session calling `/api/global-wire/stories`
  received status `200`, response source `durable-storygraph`, `story_count:
  3`, and `source_maxx_vtext_count: 0`. The same session calling
  `/api/global-wire/sourcemaxx-status` received status `200` for latest cycle
  `cycle_749fc5d6e5e8f7f859fb69c2`, with `fetch_count: 14`, `item_count:
  500`, `processor_request_count: 10`, and `reconciler_request_count: 1`.
- Current belief: the VText-owned architecture remains correct, but the new
  Global Wire VText index is not discovering deployed SourceMaxx article heads.
  Likely investigation axes are platform-owner mismatch, missing/unfinished
  current SourceMaxx VText revisions for the latest cycle, revision metadata
  shape mismatch, or overly strict seed-content filtering.

updated remaining error field:

- The latest deployed behavior commit is not enough to claim authenticated
  Global Wire data realism. SourceMaxx status proves live source cycles, but
  `/api/global-wire/stories` still exposes only seeded durable StoryGraph rows.
- The next behavior change must root-cause why platform SourceMaxx VTexts are
  absent from the story response, then prove with an authenticated product-path
  browser session that `/api/global-wire/stories` returns
  `durable-storygraph+source-maxx-vtexts` and at least one
  `source-maxx-vtext-*` row with VText content, source/style provenance, and a
  normal VText open path.

2026-06-07 authenticated runtime split root cause:

- Follow-up staging investigation after commit
  `b139e518f360c044bd89bca3ff31cf19e5a4145d`
  (`nix: persist node b sandbox runtime store`) showed that the host sandbox
  now persists SourceMaxx VTexts correctly. Direct Node B Dolt queries under
  `/var/lib/go-choir/runtime/runtime.vtext` found four current
  `global-wire-platform` VText documents with `source: edit_vtext`,
  `source_maxx_cycle_id`, selected style metadata, and non-seed article
  content.
- The authenticated product response still did not surface those VTexts:
  `/api/global-wire/stories` returned `source: durable-storygraph` with only
  the three seeded story rows, while `/api/global-wire/sourcemaxx-status`
  resolved the live host SourceMaxx cycle and processor/reconciler runtime
  evidence.
- Root cause is now identified as a cross-runtime index lookup, not missing
  VText creation. The deployed proxy is configured with
  `PROXY_VMCTL_URL=http://127.0.0.1:8083`, and generic authenticated
  `/api/*` routes are resolved through vmctl to the user's active VM sandbox.
  SourceMaxx writes platform VTexts into the host/platform runtime at
  `SOURCE_SERVICE_RUNTIME_BASE_URL=http://127.0.0.1:8085`. The
  `/api/global-wire/stories` handler only calls `ListDocumentsByOwner` on the
  local request-serving runtime, so a user VM asks its own store for
  `global-wire-platform` article VTexts and receives none.
- This confirms the user's architectural concern: VText must be the article
  authority, but the news surface needs a normalized platform-VText read path.
  Processors and reconcilers can continue to use the shared harness and VText
  appagent ownership model; the Global Wire news app must index/transclude
  the platform-owned VText articles from the platform runtime instead of
  treating the request-serving user VM store as the whole article corpus.

updated remaining error field:

- Implement a product-safe platform SourceMaxx VText article projection path
  for `/api/global-wire/stories`, analogous to the existing SourceMaxx status
  remote-runtime evidence path, without creating a new canonical article data
  structure.
- Preserve user-owned edits by keeping open/fork actions in normal VText
  flows. The cross-runtime index may expose platform article heads and VText
  handles, but it must not mutate platform stories from user requests.
- Prove on staging with a passkey-authenticated browser session that the
  vmctl-routed Global Wire app receives at least one `source-maxx-vtext-*`
  story row sourced from platform VTexts, while the clean newspaper UI and
  public preview acceptance still pass.
