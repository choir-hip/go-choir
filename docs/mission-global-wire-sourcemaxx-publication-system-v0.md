# Mission: Global Wire SourceMaxx Publication System

**Status:** new MissionGradient mission; supersedes the prior Global Wire
slice-delivery trajectory where it conflicts with source volume, readability,
or publication-quality `Style.vtext` requirements.  
**Requirements contract:** `docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md`  
**Prior mission context:** `docs/mission-global-wire-style-vtext-collaborative-storygraph-v0.md`  
**Created:** 2026-06-07

## Goal String

```text
/goal Run docs/mission-global-wire-sourcemaxx-publication-system-v0.md as an overnight MissionGradient mission. Re-architect Global Wire as a continuous high-volume source ingestion and publication-quality collaborative StoryGraph system, not a click-time source-refresh demo. Deliver the spec in docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md by building SourceMaxx ingestion that can ingest hundreds of GDELT/RSS/Telegram/search SourceItems per 15 minutes or faster where feasible, normalize/dedupe them, cluster them into story source neighborhoods, compute provenance/source-standing/contradiction/relevance/change signals, and feed StoryGraph without oracle mutation. Redesign the News app into clean readable newspaper-style columns with no nested scrolling panels, no repeated card wall, and no low-density artifact repetition. Deepen Style.vtext into citeable publication-grade editorial source artifacts that produce genuinely high-quality VText projections with voice, structure, evidence rules, examples, anti-patterns, revision policy, and projection evaluations. Delete or replace architecture/UI/data paths that encode the wrong object, including one-result refresh bottlenecks, frontend-only preview authority, redundant panels, and shallow style tabs. Preserve all invariants: every story is a normal editable VText; user edits/forks/contributions are user-owned and never mutate platform stories; Style.vtext is selectable/composable/replacable/citeable; news is non-oracle and provenance-rich; graph nodes are Story VText headlines with source-neighborhood semantics; all app views work in Future Noir, Carbon Kintsugi, and London Salmon. Use cognitive transforms before major route changes and before stopping. Use staging/product-path proof for real ingestion volume, StoryGraph propagation, VText ownership, publication-quality projections, readable app behavior, and deployed commit identity. Update this mission doc with checkpoint/resumption state before stopping.
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

Changed plan:

- implementation: start with the ingestion and source-neighborhood substrate,
  then expose a clean publication surface over it.
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
-> claim/entity/event extraction
-> story clustering and source neighborhoods
-> StoryGraph candidates and durable Story VTexts
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
- source neighborhoods show diversity, contradiction, standing, and update
  state instead of one deterministic signal;
- StoryGraph consumes neighborhoods without blindly mutating canonical stories;
- Story VTexts are normal editable VTexts;
- user edits create user-owned forks/versions;
- `Style.vtext` artifacts are deep enough to shape publication-quality prose;
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

### Story Neighborhood Pipeline

Source flow should enter a neighborhood pipeline:

```text
SourceItem batch
-> normalize
-> dedupe
-> extract claim/entity/event/time/place hints
-> cluster into story neighborhoods
-> compute overlap/contradiction/standing/change/prominence signals
-> create StoryGraph candidates
-> queue research/reconciliation work
```

The first implementation may use simple deterministic clustering if it records
the limits honestly. It must not collapse the neighborhood into one signal.

### StoryGraph And VText

StoryGraph remains the durable evidence and relationship object. Story VTexts
are readable/editable projections over it.

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
- projection evaluation criteria.

Projection quality is part of product correctness. A projection fails if it is
generic, shallow, repetitive, source-thin, stylistically indistinct, or
publication-unworthy even when it preserves facts.

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
- deterministic single-signal source review paths that erase neighborhoods;
- tests that only prove a button or singleton response when the product needs
  source volume and neighborhood behavior.

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
- **Neighborhood richness:** single classification -> clusters -> overlap,
  contradiction, standing, change, prominence, and source-density signals.
- **StoryGraph realism:** candidates -> reviewed updates -> timeline and
  related-story graph.
- **Style depth:** simple style source -> publication-quality Style.vtext ->
  composed/replaced/revised style artifacts.
- **Projection quality:** generic summary -> sourced article -> distinct
  editorial voice -> publication-ready update package.
- **UI readability:** panel wall -> columns -> responsive newspaper/workbench
  across all themes.
- **Product proof:** local tests -> source daemon metrics -> staging ingestion
  volume -> browser screenshots -> product-path ownership proof.

## Dense Feedback And Verifiers

Use layered proof:

- focused Go tests for source adapters, batch ingest, dedupe, clustering,
  neighborhood signal creation, and StoryGraph candidate creation;
- ingestion metrics proving item counts, provider counts, dedupe counts,
  freshness windows, and error/backoff behavior;
- product-path API proof through browser-public routes only;
- staging proof that the deployed source system ingests real or configured
  high-volume sources;
- Playwright/browser/Computer Use screenshots across desktop and mobile for
  Future Noir, Carbon Kintsugi, and London Salmon;
- projection evaluation fixtures comparing at least two deep Style.vtexts over
  the same StoryGraph evidence;
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
- Do not ship generic assistant prose as publication-quality content.
- Do not use internal/test-only routes for product proof.
- Do not claim staging behavior from local-only evidence.

## Stopping Condition

Mark `complete` only when staging proves:

- continuous or scheduled ingestion of high-volume source batches, with a
  documented path to faster/live ingestion;
- durable SourceItems from multiple source classes;
- dedupe and story-neighborhood signals;
- StoryGraph candidate/reconciliation state from source neighborhoods;
- normal Story VText and user-owned fork/edit behavior;
- citeable, deep `Style.vtext` artifacts and publication-quality projections;
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
- source-neighborhood clustering over high-volume batches;
- publication-quality Style.vtext projections;
- redesigned readable newspaper UI;
- all-theme visual proof.

belief-state changes:

- the highest-value axis is source architecture plus readable publication
  surface, not more Global Wire panels;
- deletion of wrong abstractions is likely necessary;
- Style.vtext quality must be treated as product correctness.

remaining error field:

- exact current sourcecycled staging configuration and source volume;
- whether existing source adapters can already support the desired cadence;
- ingestion storage/performance limits;
- best clustering boundary between deterministic code, search providers, and
  researcher/model workflows;
- which current UI components should be deleted versus reused.

highest-impact remaining uncertainty: whether the deployed source system can be
configured and proven to ingest high-volume GDELT/RSS/Telegram batches on the
desired cadence without a deeper source daemon/storage redesign.

next executable probe:

1. Recover or intentionally discard the partial source-refresh experiment from
   `stash@{0}` (`superseded-global-wire-source-refresh-batch-experiment-2026-06-07`)
   only after deciding whether its "do not discard result two onward" fix
   belongs in the new ingestion architecture.
2. Inspect `cmd/sourcecycled`, `internal/sources`, source storage, staging
   source configuration, and current Global Wire source-refresh code.
3. Produce a deletion/reuse map for current Global Wire UI and source paths.
4. Implement the smallest continuous ingestion proof that records high-volume
   SourceItem batches and exposes source-neighborhood counts through a
   product-safe path.
5. Redesign the Global Wire front page into readable columns over that data.

suggested resume goal string: use the Goal String section above.

evidence artifact refs: none yet for this mission.

rollback refs: prior branch/worktree state before this mission;
`stash@{0}` named
`superseded-global-wire-source-refresh-batch-experiment-2026-06-07` preserves
the abandoned source-refresh batch edits; any behavior commits must record
their own rollback SHAs.
