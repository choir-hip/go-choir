# Mission: Global Wire SourceMaxx Publication System

**Status:** new MissionGradient mission; supersedes the prior Global Wire
slice-delivery trajectory where it conflicts with source volume, readability,
or publication-quality `Style.vtext` requirements.  
**Requirements contract:** `docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md`  
**Prior mission context:** `docs/mission-global-wire-style-vtext-collaborative-storygraph-v0.md`  
**Created:** 2026-06-07

## Goal String

```text
/goal Run docs/mission-global-wire-sourcemaxx-publication-system-v0.md as an overnight MissionGradient mission. Re-architect Global Wire as a continuous high-volume source ingestion and publication-quality collaborative StoryGraph system, not a click-time source-refresh demo. Deliver the spec in docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md by building SourceMaxx ingestion that can ingest hundreds of GDELT/RSS/Telegram/search SourceItems per 15 minutes or faster where feasible, normalize/dedupe them, route them into long-running processors, let processors maintain hot-context/KV-cache understanding with agent-owned compaction, and let reconcilers connect stories, contradictions, and open questions across processors. Reuse existing researcher agents for additional evidence work and existing VText agents for article writing/revision from processor/reconciler briefs plus matched Style.vtext artifacts. Do not require clustering or embeddings in the first architecture pass; defer those until the processor/reconciler loop is proven. Redesign the News app into clean readable newspaper-style columns with no nested scrolling panels, no repeated card wall, and no low-density artifact repetition. Deepen Style.vtext into citeable publication-grade editorial source artifacts that produce genuinely high-quality VText projections with voice, structure, evidence rules, examples, anti-patterns, revision policy, projection evaluations, and intelligent story-style matching. Do not run every style over every story by default; select, rank, compose, or withhold Style.vtexts based on story domain, audience, source neighborhood, editorial need, and user/publication context. Delete or replace architecture/UI/data paths that encode the wrong object, including one-result refresh bottlenecks, frontend-only preview authority, redundant panels, and shallow style tabs. Preserve all invariants: every story is a normal editable VText; user edits/forks/contributions are user-owned and never mutate platform stories; Style.vtext is selectable/composable/replacable/citeable; news is non-oracle and provenance-rich; graph nodes are Story VText headlines with source-neighborhood semantics; all app views work in Future Noir, Carbon Kintsugi, and London Salmon. Use cognitive transforms before major route changes and before stopping. Use staging/product-path proof for real ingestion volume, processor/reconciler behavior, existing researcher and VText agent reuse, VText ownership, intelligent style matching, publication-quality projections, readable app behavior, and deployed commit identity. Update this mission doc with checkpoint/resumption state before stopping.
```

## Why This Is A New Mission

The prior mission improved the deployed slice but exposed a wrong center of
gravity: Global Wire was drifting toward a manually refreshed, low-volume,
panel-heavy product. That is not the spec.

The real object is a live editorial source engine that continuously transforms
large source flow into source-grounded Story VTexts and publication-quality
style projections. The app is a reading/editing surface over that engine, not a
dashboard of internal artifacts.

This mission explicitly permits deletion and replacement. If a prior addition
or existing path makes the wrong object easier and the right object harder, the
agent should remove, quarantine, or migrate it after documenting the problem
first when required by the repo contract.

## Cognitive Transform Baseline

Current obstacle: the architecture is under-resolving the source system and
over-exposing implementation artifacts in the UI.

Selected transforms:

1. **Depth extraction:** the banal version is "news cards with sources." The
   deep version is a publication system whose authority comes from live source
   breadth, source standing, contradiction handling, and editorially excellent
   VText projections.
2. **Load-bearing variable:** the key variable is not number of panels or
   buttons. It is source neighborhood quality: volume, freshness, diversity,
   dedupe, standing, contradiction, and usable projection into stories.
3. **Anti-Goodhart:** do not satisfy "more sources" by raising `max_results`
   on a user click. The system must continuously ingest and preserve many
   SourceItems as durable source-neighborhood evidence.
4. **Subtractive architecture:** deleting wrong surfaces is progress. A clean
   newspaper view over a real source neighborhood is more correct than a busy
   dashboard showing repeated internal records.
5. **Publication-quality transform:** `Style.vtext` is not a style tab or
   prompt snippet. It is a citeable editorial source artifact whose quality is
   evaluated by actual resulting prose.
6. **Editorial routing:** not every story wants every style. The system should
   choose style because it serves a story, audience, source neighborhood, or
   user/publication intent, not because a projection matrix is easy to render.
7. **Resident cognition:** processors are not stateless summarizers. They are
   long-running agents that preserve hot context/KV cache, compact when needed,
   and keep durable handles to full source content and prior compactions.
8. **Reconciliation as role:** cross-story connections, contradictions, and
   questions need an explicit live agent role. The durable graph records the
   projection of that work; it does not replace the work.

Changed plan:

- implementation: start with the ingestion and source-neighborhood substrate,
  processor/reconciler agent loop, then expose a clean publication surface over
  it.
- verifier/evidence: prove source volume and freshness, not merely one API
  response; prove readable app screenshots, not merely DOM existence.
- scope: one or a few story neighborhoods are acceptable only if they are fed
  by real high-volume ingestion and can deform into the full system.
- stopping condition: do not stop at a working refresh button, dense dashboard,
  or shallow style projection.

## Real Artifact

The artifact is the end-to-end Global Wire publication system:

```text
continuous source registry
-> GDELT/RSS/Telegram/search/provider ingestion
-> SourceItem batches
-> normalization, dedupe, source standing, fetch provenance
-> simple source routing into processors
-> long-running processor hot context and compaction chain
-> reconciler connection/contradiction/question work
-> existing researcher evidence requests where needed
-> existing VText agent article/update requests
-> durable graph records and Story VTexts
-> publication-quality Style.vtext projections
-> clean News app columns
-> user-owned edits, forks, contributions
-> research/reconciliation-ready queues
```

The mission is not a source demo, not a dashboard, and not an experiment where
quality does not matter. The product must produce news content that is worth
reading.

## Value Criterion

Minimize divergence from a live, source-grounded, publication-quality
collaborative news system while preserving all ownership, VText, provenance,
and non-oracle invariants.

The product moves uphill when:

- ingestion volume and freshness approach "hundreds of articles/posts per 15
  minutes" with a clear path to faster/live ingestion;
- SourceItems are durable, deduped, provider-tagged, and traceable to fetch
  runs;
- processors preserve high-context understanding of source flow instead of
  reconstructing every cycle from summaries;
- reconcilers surface cross-story connections, contradictions, and open
  questions;
- existing researcher agents can be requested by processors, reconcilers, or
  VText agents;
- existing VText agents write/update articles from processor/reconciler briefs,
  research packets, and matched Style.vtext artifacts;
- durable graph records consume processor/reconciler outputs without blindly
  mutating canonical stories;
- Story VTexts are normal editable VTexts;
- user edits create user-owned forks/versions;
- `Style.vtext` artifacts are deep enough to shape publication-quality prose;
- style matching chooses, ranks, composes, or withholds styles based on story
  fit instead of projecting every style over every story;
- projected stories are readable, sourced, different in meaningful ways, and
  not generic assistant summaries;
- the app reads like a clean newspaper/workbench, not a set of nested panels;
- staging proof demonstrates real source flow, readable views, and correct
  ownership behavior.

## Hard Invariants

- Every story remains a normal editable VText.
- User edits and contributions create user-owned artifacts and do not mutate
  platform stories.
- Platform story mutation remains a separate reviewed process.
- `Style.vtext` is a citeable source artifact, not hardcoded app config.
- Style projections must preserve evidence and cite source/style lineage.
- News is non-oracle: uncertainty, contrary evidence, source standing, and
  change history remain inspectable.
- Graph nodes are Story VText headlines by default; source/claim/entity views
  are overlays.
- Source neighborhoods carry semantics: overlap, contradiction, update,
  freshness, prominence, and standing.
- All views work in Future Noir, Carbon Kintsugi, and London Salmon.
- Product-path/staging proof is required before claiming platform behavior.

## Architecture Direction

### SourceMaxx Ingestion

Target shape:

- a durable source registry with GDELT, many RSS feeds, many Telegram feeds,
  search providers, and future curated/provider feeds;
- scheduled and, where feasible, live fetch loops;
- per-source and per-provider rate policy;
- fetch-run records with timing, counts, errors, backoff, and provenance;
- SourceItem records with canonical URL/content ref, source id, source type,
  publisher/channel, timestamp, cleaned text pointer, raw snapshot pointer,
  content hash, standing metadata, and policy metadata;
- batch ingestion capable of hundreds of SourceItems per 15 minutes in staging;
- dedupe by canonical URL, provider id, normalized title/time, and content hash;
- preservation of duplicates as source-standing/echo evidence when useful,
  without creating repeated article rows.

Low resolution may begin with configured feeds and deterministic extractors,
but it must preserve the topology of continuous ingestion. A manual
`source-refresh` button is a control/debug affordance, not the source system.

### Processor And Reconciler Pipeline

Source flow should enter a processor/reconciler pipeline:

```text
SourceItem batch
-> normalize
-> dedupe
-> simple routing by source, topic hint, geography, language, and load budget
-> long-running processors absorb sources into hot context
-> processors compact when context pressure rises while preserving source refs
-> processors emit briefs, watch items, research requests, and VText requests
-> reconcilers inspect processor briefs for links, contradictions, and questions
-> researchers answer targeted evidence requests
-> VText agents write/revise articles using briefs, research, and Style.vtext
-> durable graph records lineage, relationships, contradictions, and versions
```

The first implementation should not require clustering or embeddings. Simple
routing is enough if processors and reconcilers can maintain useful live
understanding and durable provenance. Clustering and embeddings are later
realism axes after the agent loop works.

Processors are not necessarily vertical-specific. A processor may own a broad
topic, a geographic region, a source class, a developing event family, or a
temporary load-balanced slice of the firehose. The number of processors should
be tuned to source volume and LLM budget.

Reconcilers are the bridge role. They receive processor briefs and selected
source/research handles, then ask what connects, what conflicts, and what is
missing. They can request additional research and can ask VText agents for
cross-cutting stories or updates.

### Agent Reuse And Durable Graph Records

This mission must reuse Choir's existing researcher agents and VText agents.
Do not create parallel researcher or writer systems.

Processors and reconcilers may request:

- existing researcher agents for bounded evidence checks, source-standing
  reviews, additional search, contradiction inspection, and missing context;
- existing VText agents for new articles, updates, rewrites, and publication
  packages from processor/reconciler briefs plus Style.vtext.

Durable graph records remain the evidence and relationship projection of the
agent work. Story VTexts are readable/editable projections over those records.

Canonical story updates require review/candidate records. User edits/forks must
remain user-owned.

### Deep Style.vtext

`Style.vtext` artifacts must become publication-grade editorial sources. A good
Style.vtext should include:

- editorial purpose and audience;
- voice principles;
- structure and section patterns;
- evidence and citation rules;
- uncertainty and correction rules;
- source-standing rules;
- examples of good output;
- anti-patterns;
- revision policy;
- composition rules for hybrid styles;
- applicability metadata: domains, audience, source-neighborhood conditions,
  story states, publication contexts, and explicit "do not use" cases;
- routing policy for when the style should be selected, composed with another
  style, replaced, or withheld;
- projection evaluation criteria.

Projection quality is part of product correctness. A projection fails if it is
generic, shallow, repetitive, source-thin, stylistically indistinct, or
publication-unworthy even when it preserves facts.

Style routing quality is also part of correctness. The system should not create
a cartesian matrix of every style over every story by default. It should make
editorial matches: a market brief belongs on market-moving business evidence,
a skeptical claim audit belongs on disputed or weakly sourced claims, a policy
brief belongs on institutional/legal/regulatory stories, and a wire style may
serve baseline public updates. User or publication preferences can override
the default route, but the default product behavior should be selective and
explainable.

### News App Redesign

Replace the current dense panel/card wall with a readable newspaper/workbench:

- front page uses columns like news, not cards;
- no nested scrolling panels;
- no repeated display of the same limited information;
- primary screen emphasizes story, source breadth, recency, and tension;
- evidence details are progressive disclosure, not permanent clutter;
- controls are compact and predictable;
- source neighborhoods are readable as context, not internal debug dumps;
- all three themes preserve capability and readability.

Computer Use or browser screenshots are required for visual proof. Do not claim
the redesign is good from code inspection alone.

## Deletion And Replacement Targets

Audit and delete, replace, or quarantine paths that encode the wrong object:

- one-result or click-time source-refresh bottlenecks;
- frontend-only preview authority that looks like product truth;
- repeated artifact panels that show the same source/contribution/candidate
  data in several places;
- nested scroll containers inside app panels;
- card-heavy layouts that prevent scanning stories like news;
- shallow style tabs that do not open/select citeable `Style.vtext` artifacts;
- projection matrices that run every style over every story without editorial
  routing or story-fit evidence;
- deterministic single-signal source review paths that erase processor context
  or reconciler questions;
- new standalone researcher/writer implementations that bypass existing
  researcher or VText agents;
- tests that only prove a button or singleton response when the product needs
  source volume and processor/reconciler behavior.

Follow problem-documentation-first before fixing any newly discovered deployed
behavior problem. For architectural cleanup that is planned by this mission,
document the deletion rationale in the mission checkpoint or commit message.

## Homotopy Axes

Increase resolution along these axes without changing object identity:

- **Source volume:** a few feeds -> many feeds -> hundreds per 15 minutes ->
  faster/live where provider-compatible.
- **Source diversity:** one provider -> GDELT/RSS/Telegram/search -> curated
  domain-specific source sets.
- **Freshness:** manual refresh -> scheduled 15-minute batches -> faster
  per-source cadence -> live events where available.
- **Processor cognition:** stateless source batches -> long-running processors
  -> hot-context/KV-cache preservation -> compaction chains with source refs.
- **Reconciler realism:** no bridge role -> cross-processor contradiction and
  question detection -> research requests -> related-story records.
- **Graph realism:** candidate records -> reviewed updates -> timeline and
  related-story graph.
- **Style depth:** simple style source -> publication-quality Style.vtext ->
  composed/replaced/revised style artifacts.
- **Style routing:** manual style choice -> story-fit ranking -> automatic
  select/compose/withhold decisions -> user/publication override with
  provenance.
- **Projection quality:** generic summary -> sourced article -> intelligently
  matched editorial voice -> publication-ready update package.
- **UI readability:** panel wall -> columns -> responsive newspaper/workbench
  across all themes.
- **Product proof:** local tests -> source daemon metrics -> staging ingestion
  volume -> browser screenshots -> product-path ownership proof.

## Dense Feedback And Verifiers

Use layered proof:

- focused Go tests for source adapters, batch ingest, dedupe, source routing,
  processor/reconciler request records, and graph candidate creation;
- ingestion metrics proving item counts, provider counts, dedupe counts,
  freshness windows, and error/backoff behavior;
- runtime/product evidence that processors preserve context across source
  batches, compact with source handles, and resume from compaction;
- runtime/product evidence that reconcilers produce connections,
  contradictions, questions, and research/VText requests;
- runtime/product evidence that existing researcher and VText agents receive
  and act on those requests;
- product-path API proof through browser-public routes only;
- staging proof that the deployed source system ingests real or configured
  high-volume sources;
- Playwright/browser/Computer Use screenshots across desktop and mobile for
  Future Noir, Carbon Kintsugi, and London Salmon;
- projection evaluation fixtures proving at least two deep Style.vtexts can
  produce strong projections when they fit, and that at least one non-fitting
  style is withheld or deprioritized with an explainable reason;
- ownership proof that user forks/edits remain user-owned;
- mission doc checkpoint with evidence refs before stopping.

## Forbidden Shortcuts

- Do not claim SourceMaxx by setting `max_results` higher on a click-time
  refresh.
- Do not use frontend seed data as product truth.
- Do not add more panels to compensate for unclear information architecture.
- Do not hide provenance to make the app look clean.
- Do not let user edits mutate platform stories.
- Do not treat `Style.vtext` as a short prompt string.
- Do not run all styles over all stories as the default product behavior.
- Do not create new researcher or writer agent types when existing researcher
  and VText agents should be reused.
- Do not require clustering or embeddings before proving the processor and
  reconciler loop.
- Do not ship generic assistant prose as publication-quality content.
- Do not use internal/test-only routes for product proof.
- Do not claim staging behavior from local-only evidence.

## Stopping Condition

Mark `complete` only when staging proves:

- continuous or scheduled ingestion of high-volume source batches, with a
  documented path to faster/live ingestion;
- durable SourceItems from multiple source classes;
- dedupe and source routing into processors;
- long-running processor behavior with context preservation and compaction
  handles;
- reconciler behavior that surfaces cross-story connections, contradictions,
  and open questions;
- existing researcher and VText agent reuse from processor/reconciler requests;
- graph candidate/reconciliation state from processor/reconciler outputs;
- normal Story VText and user-owned fork/edit behavior;
- citeable, deep `Style.vtext` artifacts, intelligent style-story matching,
  and publication-quality projections;
- clean newspaper-style Global Wire views with no nested panel scrolling;
- all required views work in Future Noir, Carbon Kintsugi, and London Salmon;
- deployed commit identity, CI, and product-path acceptance are recorded.

Use `checkpoint_incomplete` if useful progress lands but any of the above is
not proven. Use `blocked_incomplete` only after root-cause investigation,
alternative routes, cognitive transforms, and a smallest-safe-next-probe are
recorded.

## Run Checkpoint & Resumption State

status: checkpoint_incomplete

last checkpoint: 2026-06-07 new mission authored after owner correction that
the source architecture and UI direction were under-resolved.

current artifact state: prior Global Wire slices exist and staging has some
Source Service-backed paths, StoryGraph/VText/projection/contribution records,
publication artifacts, newsletter ledgers, and a dense Global Wire app surface.
The current direction is insufficient: source processing is too close to
manual/source-refresh semantics, the UI is too busy, and Style.vtexts are not
yet deep publication artifacts. A partial source-refresh batch experiment from
the superseded route was preserved in a named stash instead of left in the
working tree.

what shipped: this mission document only, unless a later checkpoint says
otherwise.

what was proven: not yet run under this mission.

unproven or partial claims:

- high-volume ingestion of hundreds of source items per 15 minutes;
- many Telegram/RSS/GDELT sources configured and observed on staging;
- processor and reconciler agent contracts over high-volume batches;
- long-running processor context/KV-cache preservation and compaction;
- intelligent Style.vtext-to-story matching and withholding/deprioritization;
- publication-quality Style.vtext projections;
- redesigned readable newspaper UI;
- all-theme visual proof.

belief-state changes:

- the highest-value axis is source architecture plus readable publication
  surface, not more Global Wire panels;
- deletion of wrong abstractions is likely necessary;
- Style.vtext quality must be treated as product correctness.
- Style.vtext routing must be treated as editorial judgment, not exhaustive
  permutation.
- processors may span multiple verticals; "vertical" is a routing/category
  concept, not the agent type name.
- reconcilers are the bridge role for connections, contradictions, and
  questions across processor outputs.

remaining error field:

- exact current sourcecycled staging configuration and source volume;
- whether existing source adapters can already support the desired cadence;
- ingestion storage/performance limits;
- processor/reconciler agent contracts, compaction policy, and load budget;
- best boundary between deterministic routing, processors, reconcilers,
  existing researchers, and existing VText agents;
- which current UI components should be deleted versus reused.

highest-impact remaining uncertainty: whether the deployed source system can be
configured and proven to ingest high-volume GDELT/RSS/Telegram batches on the
desired cadence and feed long-running processors without a deeper source
daemon/storage/runtime redesign.

next executable probe:

1. Recover or intentionally discard the partial source-refresh experiment from
   `stash@{0}` (`superseded-global-wire-source-refresh-batch-experiment-2026-06-07`)
   only after deciding whether its "do not discard result two onward" fix
   belongs in the new ingestion architecture.
2. Inspect `cmd/sourcecycled`, `internal/sources`, source storage, staging
   source configuration, and current Global Wire source-refresh code.
3. Produce a deletion/reuse map for current Global Wire UI and source paths.
4. Implement or specify the smallest continuous ingestion proof that records
   high-volume SourceItem batches, routes them to processors, and records
   processor/reconciler requests through product-safe paths.
5. Redesign the Global Wire front page into readable columns over that data.

suggested resume goal string: use the Goal String section above.

evidence artifact refs: none yet for this mission.

rollback refs: prior branch/worktree state before this mission;
`stash@{0}` named
`superseded-global-wire-source-refresh-batch-experiment-2026-06-07` preserves
the abandoned source-refresh batch edits; any behavior commits must record
their own rollback SHAs.
