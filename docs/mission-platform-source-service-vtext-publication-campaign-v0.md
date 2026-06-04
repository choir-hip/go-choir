# MissionGradient Campaign: Platform Source Service, VText Source Entities, And Publication v0

**Status:** draft for owner review  
**Date:** 2026-06-04  
**Method:** Cognitive Transform Portfolio + MissionGradient  
**Supersedes:** [mission-standalone-sourcecycled-data-platform-v0.md](mission-standalone-sourcecycled-data-platform-v0.md) as the active framing  
**Primary synthesis input:** [news-econ-publishing-synthesis-2026-06-04.md](news-econ-publishing-synthesis-2026-06-04.md)

## One-Line Goal String

```text
/goal Run docs/mission-platform-source-service-vtext-publication-campaign-v0.md as a Codex-operated MissionGradient campaign. Build Choir's platform Source Service by reshaping sourcecycled into a secure, provenance-first ingestion, retrieval, source-entity, and publication-support substrate for global news, official macro/economic sources, later private corpus sources, and VText/publication transclusion. Preserve VText as the canonical artifact-level surface, ContentItem and platform source records as durable source artifacts, researchers as source-representation producers, VText as canonical document writer, and publication records as immutable public/private citation and export ledgers. Start with the smallest deployed product path that ingests a few real news/official sources into durable fetch/item/source records, retrieves over them alongside web search for researcher agents, represents selected evidence as hidden VText source_entities with visible expandable citations/transclusions, preserves that metadata through publication, and exposes copy/download from canonical artifacts. Do not build a standalone newspaper island, parallel transcript/source browser, DOM-scraped exporter, markdown-only citation syntax, sourcecycled writer, or publication route that bypasses access policy. Do not claim success without staging proof over real source ingestion, exact source artifact resolution, deployed researcher retrieval, VText metadata preservation, publication projection, and user-visible copy/download.
```

## Executive Thesis

The next product object is not a newspaper, a feed reader, a citation UI, or an export menu. The product object is Choir's source substrate:

```text
source registry
  -> polite ingestion and fetch ledger
  -> stable source artifacts
  -> retrieval over source artifacts plus web search
  -> researcher source representations
  -> hidden VText source_entities and inline expandable citations
  -> publication artifact manifests
  -> copy/download/export under route policy
```

`sourcecycled` should remain a recognizable service surface, but the active framing should change from "standalone data platform" to "Choir platform Source Service." It may expose CLI, HTTP, and WebSocket contracts, but those contracts exist so Choir agents, user computers, future client projects, and external consumers can inspect and consume the same source ledger. They are not a separate product island.

The service should ingest public global news and official sources because Choir will always need a current, durable, source-grounded view of the world. The same architecture later expands to private legal subscriptions, client document sets, firm knowledge bases, Dropbox/Drive-style "Choir Base" artifacts, and sourcecycled manifest import. Those later private sources must deform from the same source artifact model, not bypass it.

## Review Ground Truth

This campaign follows a review of the current repo state and the synthesis doc created on 2026-06-04.

### Sourcecycled Current State

The staged `sourcecycled` WIP is useful but still toy-shaped:

- `cmd/sourcecycled/main.go` is a blocking daemon that loads `configs/sources.json`, polls every 15 minutes, clusters by vertical, runs LLM synthesis, and writes SQLite.
- `internal/sources` has RSS, GDELT, and Telegram adapters plus a Polymarket source type, but `configs/sources.json` includes Polymarket without an adapter.
- RSS imports `github.com/mmcdole/gofeed`, but the dependency is absent from `go.mod`.
- Deduplication is in-memory only, so restart loses duplicate knowledge.
- There is no durable fetch ledger, cycle table, cycle-event stream, source policy table, source health, raw snapshot store, citation map, or exact source manifest.
- Synthesis prompts the model to cite source labels, but persisted issues only store all item IDs globally; there is no proof that rendered citations map to exact item IDs.
- The current synthesizer imports Choir `internal/provider`, which conflicts with the old standalone mission and clarifies that the real boundary is now a Choir platform service boundary.

### Existing Search And Research State

Choir already has a multi-provider web search plane through the gateway and researcher tools:

- `web_search` routes through the gateway search plane and records provider attempts, health, latency, and result counts.
- `fetch_url`, `import_url_content`, and `read_content_item` give researchers durable owner-scoped source artifacts.
- Researcher prompts correctly require early `submit_coagent_update` checkpoints.
- VText does not have web search tools and should not get them; researchers own source investigation.

The missing capability is a unified source retrieval path. When a researcher searches the web, the same turn should be able to search:

- platform Source Service items;
- owner-scoped ContentItems and local file artifacts where authorized;
- public Choir/Dolt publication records;
- official macro/economic source artifacts;
- live web search providers.

### VText Source Entity State

VText source entities are real but narrow:

- YouTube and image URLs can become `media_source_refs`.
- Those refs can normalize into `source_entities` metadata.
- The frontend renders source rails, cards, and inline `source:` refs.
- VText prompts preserve source refs and tell researchers to read ContentItems first.

The current source entity schema does not yet cover sourcecycled items, official data releases, private corpus records, public platform records, local files, web snapshots, export policy, or route access policy.

### Publication State

Publication is started but metadata-thin:

- Publication records create immutable platform artifacts, retrieval spans, citation edges, provenance entities, attestations, consent records, and rollback refs.
- Public resolution returns route, publication summary, version summary, artifact content, render model, retrieval spans, citations, proposals, and provenance.
- Search over published VTexts exists but is simple text scanning over public active publications.

Current publication drops important private revision metadata:

- `internal/proxy/platform_publish.go` fetches revision content and citations but not revision metadata.
- `platform.PublishVTextRequest` does not accept `source_entities`, `media_source_refs`, transclusions, content item refs, route policy, export policy, or hidden artifact metadata.
- Public copy/download/export does not exist yet.
- Therefore export must not be implemented from DOM scraping; it must read immutable revision or publication artifacts plus metadata.

### Access Control State

Current access control is not client-portal-ready:

- `/api/*` proxy routes are auth-gated.
- Public publication route resolution uses active public routes.
- Publication schema has `visibility` in proposals and public retrieval visibility fields, but route policy, reader roles, passwords, capabilities, and export controls are not first-class.

The role-gated publishing system is a full platform feature. It should be designed now and implemented after source metadata projection and basic export, not bolted onto the public reader afterward.

### Markdown-To-VText Side Bug

Opening text or Markdown files already creates/reuses a canonical VText document alias through `/api/vtext/files/open`. However, the product invariant needs to be sharper:

```text
When a user starts working on a non-.vtext text artifact in VText, it becomes a VText-owned canonical document automatically.
```

The original file may remain a source/export target, but editing should not leave the user in an ambiguous "Markdown file pretending to be a VText" state. The mission should prove that open/edit/revise/publish uses VText revision identity and produces a `.vtext` manifest or equivalent alias without requiring the user to manually convert.

## Cognitive Transform Review

### Current Uncertainty Or Obstacle

The work can collapse into four tempting but wrong smaller objects:

- a better feed daemon;
- a news issue generator;
- source cards in VText;
- copy/download buttons on public posts.

Those are useful surfaces, but none is the real artifact. The real uncertainty is whether Choir can preserve source identity across ingestion, retrieval, VText revision, publication, and export while keeping the user-facing document clean.

### Selected Transforms

1. **Depth Extraction** - The banal version is "add rich sources." The deep version is source identity survival through transformations. The load-bearing variable is whether every factual/exported/transcluded claim can resolve to durable source artifacts, selectors, hashes, and access policy.
2. **Boundary Correction** - `sourcecycled` should not be optimized as a standalone product island. It is a Choir platform Source Service with clean contracts. CLI/API/WebSocket remain valuable, but the trust and deployment boundary is the platform service.
3. **Via Negativa** - Remove citation theater, DOM export, parallel source browsers, live-only news summaries, VText-as-retriever shortcuts, and publication routes that ignore metadata or access policy.
4. **Homotopy** - The first slice can be small, but it must have the same topology as the full system: registry -> fetch -> item -> retrieval -> source entity -> publication metadata -> export. A hardcoded newspaper issue that later needs a rewrite is a fake island.
5. **OODA / Inner Loop** - Researchers should search source service, local/user artifacts, public Choir records, and web search together. Every useful search result should shorten the loop from source discovery to grounded VText revision.

### Route-Changing Insights

- The first implementation should prioritize source/fetch/item/retrieval contracts before adding more feeds.
- "Official sources" are not just another adapter category; they need release/vintage/caveat metadata, especially for macro/economic data.
- Publication export depends on metadata-preserving publication. Copy/download before metadata projection would create a weak artifact path.
- Security/RBAC should be modeled now as route/export policy metadata, even if only public/private/unlisted is implemented in the first cut.
- The Markdown-to-VText side bug belongs in P0 because it affects the canonical artifact boundary for every imported source or client draft.

### Changed Plan

**Implementation:** reshape sourcecycled into `Source Service` interfaces and tables; add a `source_search` or unified `research_search` tool that searches source service plus web; extend VText source entities for source-service targets; preserve source metadata through publish; add canonical copy/download endpoints.

**Verifier/evidence:** black-box deployed proof over real ingestion, researcher retrieval, VText revision metadata, publication bundle metadata, and export bytes. Local tests shape behavior, but staging is the acceptance environment.

**Scope:** no separate newspaper app, no standalone-only service mandate, no private legal subscription integration in v0. Build public/global/official sources first, with access-policy fields that can later admit private corpus sources.

**Stopping condition:** source ingestion, retrieval, VText source entity representation, metadata-preserving publication, and copy/download are proven together on staging over at least one real news source and one official macro/economic source.

## Real Artifact

The real artifact is the deployed Choir Source Service and its VText/publication projection:

```text
Platform Source Service
  registry:
    public news sources
    official macro/economic sources
    future private corpus sources
  ingestion:
    polite adapters
    fetch ledger
    source health
    raw snapshot hashes
    stable item identity
  storage:
    source records
    fetch records
    item records
    source versions
    selectors/spans
    provenance/caveat metadata
  retrieval:
    source search
    official-source search
    local/user content search where authorized
    public Choir/Dolt publication search
    web search fanout
  agent interface:
    researcher tools produce source representations
    VText remains the canonical writer
  VText interface:
    hidden source_entities metadata
    inline expandable citations/transclusions
    source opening to owning app/window
  publication interface:
    artifact manifests preserve source metadata
    route/access/export policy
    copy full text
    download txt/md/html first, PDF/DOCX later
```

## Hard Invariants

- **VText is the artifact surface.** Source Service never writes canonical VText versions.
- **Researchers produce source representations.** They may search, import, read, and report evidence; they do not write canonical document text.
- **Source identity survives.** Every cited/transcluded/exported source ref must resolve to stable item IDs, source IDs, selectors, hashes, and provenance metadata.
- **Ingested content is untrusted.** Source text is evidence, never instructions to agents or runtime.
- **Official source caveats are first-class.** Macro/economic artifacts need fields such as release date, data vintage, source agency, revision policy, lookahead status, and evidence level.
- **Publication is a ledger.** Published pieces preserve metadata in immutable artifact manifests and citation/transclusion records.
- **Export comes from canonical artifacts.** Copy/download must not scrape transient DOM or drop hidden source metadata.
- **Access policy is data.** Public, unlisted, private, password, role, capability, comment, copy, and download controls must be represented as route/export policy, not frontend convention.
- **No duplicate source product.** Do not build a parallel source browser, transcript app, newspaper UI, or markdown citation system that bypasses VText/source provenance.
- **Markdown work becomes VText work.** Text/Markdown files opened for real editing must automatically become VText-owned canonical documents with aliases/manifests.

## Value Criterion

Minimize the distance from "a source exists in the world or in a user's corpus" to "a researcher can retrieve it, VText can cite or transclude it, a reader can expand it, and a publication/export can preserve it," while preserving source good standing, trust boundaries, access policy, and exact provenance.

The campaign moves uphill when:

- source additions are registry/policy changes, not code changes;
- fetches and failures are durable, inspectable, and rate-policy aware;
- source items have stable IDs across restarts;
- researcher search returns source-service results alongside web results;
- VText source entities can target source-service items and official data releases;
- publication bundles carry hidden source metadata;
- exports are derived from immutable artifacts;
- staging proof shows the full path without manual seeding.

## Quality Gradient

Expected quality level: **solid platform v0**.

Solid means:

- buildable source service packages;
- durable schema with migrations/idempotent bootstrap;
- test fixtures for deterministic ingestion;
- at least one live safe news source and one official macro/economic source;
- source registry policy fields;
- fetch/item/cycle/source-search tests;
- researcher tool tests;
- VText metadata round-trip tests;
- publication metadata projection tests;
- deployed staging proof;
- clear rollback refs for platform route/schema changes.

Substandard work in this mission:

- adding many feeds before fetch/item identity is durable;
- generating a news issue without exact citation maps;
- exposing copy/download from rendered DOM;
- publishing source-rich VTexts as plain text blobs;
- implementing RBAC only as hidden frontend buttons;
- making sourcecycled a VText writer;
- claiming local proof for staging-only behavior.

## Homotopy Parameters

Increase realism along these axes without changing topology:

- **Source type:** fixture -> RSS/GDELT -> official macro feed -> official data release -> private corpus.
- **Retrieval breadth:** source search only -> source plus web -> source plus web plus local/user artifacts -> public Choir/Dolt records.
- **Entity target:** YouTube/image ContentItem -> source-service item -> official data release -> VText span -> publication span -> private corpus span.
- **Publication policy:** public -> unlisted -> password -> authenticated user -> role/capability-gated -> export/comment controls.
- **Export format:** txt/md/html -> PDF from canonical render -> DOCX -> EPUB.
- **Proof:** unit -> integration -> local browser -> staging API -> staging browser/product path.
- **Scale:** one source -> small registry -> frequent polling -> backoff/failure handling -> concurrent cycles.

## Prioritized Campaign

### P0: Source Service Nucleus

Replace toy daemon semantics with durable service contracts:

- source registry with type, tier, jurisdiction/region, language, vertical, poll interval, auth policy, robots/TOS class, body retention, rate policy, and source-good-standing fields;
- fetch ledger with request URL, canonical URL, status, headers subset, content hash, raw snapshot ref, started/ended timestamps, error class, and retry/backoff state;
- stable source item IDs from source ID plus canonical original ID/content hash fallback;
- persistent dedup across restart;
- cycle records and cycle events;
- source health summary;
- issue/source manifest type that maps every cited label to exact item IDs and selectors;
- buildable tests for RSS/GDELT/official macro fixture ingestion.

### P0: Markdown-To-VText Canonicalization Bug

Prove and fix the product boundary:

- opening `.md`, `.txt`, or equivalent text files for work creates/reuses a VText document alias;
- first edit/revise/save uses VText revision identity;
- a `.vtext` manifest or equivalent durable alias exists;
- original file write-through is explicit export/sync behavior, not the canonical edit store;
- publication uses VText revision metadata, not the raw source file.

### P1: Researcher Source Retrieval

Add a researcher-facing retrieval path that searches source service and web together:

- `source_search` or `research_search` returns source-service hits, public publication hits, authorized local/user content hits, and web-search hits with provider/source provenance;
- existing `web_search` remains available, but broad research prompts should prefer the unified retrieval path when configured;
- source results can be opened through `read_source_item` or projected into ContentItems;
- researcher checkpoints include source IDs, item IDs, selectors, hashes, caveats, and unresolved gaps.

### P1: Official Sources And Macro/Economic Lane

Import Marco's strongest patterns, not its forecast runtime:

- official source registry entries for central banks, statistics agencies, regulators, and macro-policy sources;
- release/vintage/caveat metadata;
- artifact handoff shape for data releases and model/forecast research artifacts;
- clear labels for `vintage_policy`, `lookahead_status`, `evidence_level`, source agency, and release date.

### P1: VText Source Entity Expansion

Generalize `source_entities` without exposing metadata noise to the user:

- target kinds for `source_service_item`, `official_data_release`, `content_item`, `local_file`, `private_corpus_item`, `published_vtext_span`, and `vtext_revision_span`;
- selector kinds for whole resource, text quote, byte range, text position, timestamp range, table cell/range, and data vintage;
- hidden metadata preserved in revision JSON;
- visible inline chip/disclosure and source deck remain simple;
- source entities can open the right owning surface or a focused source window.

### P2: Publication Metadata Projection

Extend publication payloads and platform records:

- private revision metadata is fetched by proxy and sent to platformd;
- publication artifact manifests include source entities, transclusions, media refs, source service refs, content item refs, and export policy;
- citation/transclusion edges are created from source entities where possible;
- public bundle resolution returns enough metadata for expandable citations and export, subject to route policy;
- retrieval spans become finer than whole-document where needed.

### P2: Copy And Download From Canonical Artifacts

Expose client-visible publishing primitives:

- copy full text for private revision and published version;
- download `.txt`, `.md`, and `.html` from immutable revision/publication content;
- export manifests include content hash, source metadata hash, and source refs;
- UI actions call export endpoints instead of scraping rendered DOM;
- PDF and DOCX follow after the canonical render/export model is stable.

### P3: Publication Access, Role, And Export Policy

Design and begin implementing route policy:

- route visibility: public, unlisted, private, password, authenticated;
- subject grants: owner, named user, client, team, role, capability;
- export controls: copy allowed, download allowed, allowed formats, watermark/audit flags;
- comment/proposal controls;
- share links and revocation;
- audit records for route access and downloads where appropriate.

This is intentionally after metadata-preserving publication because policy must guard real artifacts, not a thin reader route.

## Dense Feedback Channels

- `nix develop -c go test ./cmd/sourcecycled ./internal/cycle ./internal/sources` until source service WIP builds.
- Source service fixture tests for stable IDs, dedup across restart, fetch ledger, source health, and citation manifest validation.
- Researcher tool tests proving unified retrieval returns source-service and web results without giving VText direct search tools.
- VText tests proving source entities survive edit/revise/history/serialization.
- Publication proxy/platform tests proving metadata is included in `PublishVTextRequest`, stored in artifact manifests, and returned by bundle resolution.
- Export endpoint tests proving bytes come from immutable artifacts and include stable hashes.
- Staging API proof over `https://choir.news` for ingestion, retrieval, publication, and export.
- Browser proof for user-visible expandable citations and copy/download controls.

## Evidence Ledger Format

Every nontrivial claim should be recorded with:

```text
claim:
evidence source:
command or observation:
artifact path:
result:
uncertainty/caveat:
promotion relevance:
```

Examples:

```text
claim: RSS fixture item IDs are stable across restart.
evidence source: go test
command: nix develop -c go test ./internal/sources ./internal/cycle -run TestSourceItemStableIDAcrossRestart
artifact path: internal/cycle/source_service_test.go
result: pass
uncertainty/caveat: fixture only, live source proof still pending
promotion relevance: supports source service P0
```

```text
claim: Published VText export preserves source metadata.
evidence source: staging API
command: curl authenticated export endpoint for publication version
artifact path: platform artifact manifest id and downloaded file hash
result: exported markdown content hash and source metadata hash match publication bundle
uncertainty/caveat: PDF/DOCX deferred
promotion relevance: supports publication/export gate
```

## Forbidden Shortcuts

- Do not merge a source daemon that does not build.
- Do not add feed count before durable fetch/item identity exists.
- Do not generate source-grounded news issues without citation-label-to-item maps.
- Do not make Source Service or sourcecycled write `.vtext` files or canonical VText revisions.
- Do not give VText direct source retrieval/search tools to compensate for researcher/tool gaps.
- Do not publish source-rich VTexts as plain text plus citations only.
- Do not implement copy/download by reading rendered DOM.
- Do not treat local filesystem proof as proof of staging ingestion, gateway search, or platform publication.
- Do not implement role/access policy only as frontend-hidden controls.
- Do not integrate private legal subscriptions in v0 except as schema/policy design pressure.

## Rollback Policy

- Git rollback: every behavior-changing change lands in small commits with a revertable SHA.
- Schema rollback: additive schema changes first; destructive migrations require explicit owner review.
- Source ingestion rollback: source registry entries can be disabled without deleting source/fetch history.
- Publication rollback: route rollback refs disable bad public/private routes without deleting artifacts.
- Export rollback: export endpoints can be disabled per route policy while preserving publication records.
- Agent-tool rollback: unified retrieval can coexist with existing `web_search`; if source retrieval fails, researchers can fall back to web search with a precise blocker.

## Learning Side-Channel

Tactical learnings go into this mission doc's checkpoint section. Durable architecture changes go into canonical architecture docs only when they alter current operating rules. Source policy discoveries should update the source registry/schema docs. Staging evidence should be linked from this mission doc or a dated evidence artifact, not buried in chat.

Classify surprises:

- **Tactical:** adapter bug, missing dependency, schema field naming, fixture gap. Fix and continue.
- **Target-level:** sourcecycled must split into packages or platformd should own more/less of source storage. Update mission and continue.
- **Invariant-level:** Source Service needs to write VText, bypass publication access policy, scrape prohibited sources, or expose private source text publicly. Stop and escalate.

## Stopping Condition

The v0 campaign is complete only when staging proves:

- at least one real public/news source and one official macro/economic source ingest into durable source/fetch/item records;
- repeated ingestion preserves stable identity and persistent dedup;
- researcher retrieval returns source-service evidence and web-search evidence in one product path;
- VText writes or revises a document with hidden source entities that target source-service/official-source artifacts;
- user-visible VText rendering shows expandable inline citations or source disclosures;
- publication preserves source metadata in artifact manifests and bundle resolution;
- copy full text and download `.txt`, `.md`, and `.html` work from canonical private revision or publication artifacts;
- route/export policy fields exist and are enforced for at least public/unlisted/private or a documented smaller first policy;
- all claims are backed by tests plus deployed staging proof.

If only part of this lands, report `checkpoint_incomplete`, not complete.

## Run Checkpoint And Resumption State

**status:** checkpoint_incomplete

**last checkpoint:** P0 Source Service nucleus implemented locally after the mission was reframed from standalone sourcecycled to platform Source Service.

**P0 storage-boundary decision:** Source Service v0 should keep its own
service-local durable SQLite ledger under `sourcecycled` while exposing stable
CLI/API contracts and exportable manifests. Platform Dolt remains the
publication/citation/transclusion ledger, and runtime `ContentItem` remains the
owner-scoped artifact substrate. This avoids prematurely stuffing high-churn
poll/fetch telemetry into platform publication tables while preserving a clean
adapter path: Source Service items can later be projected into ContentItems,
VText `source_entities`, and platform citation/transclusion records by stable
IDs, hashes, and manifests. If staging evidence shows the platform needs Dolt
lineage for ingested public source records earlier, promote selected source
manifests into platform tables as a target-level reparameterization, not by
making Source Service a VText or publication writer.

**current artifact state:**

- Sourcecycled WIP is buildable and has a service-local SQLite ledger for source registry policy metadata, fetch records, source items, cycles, cycle events, and issue citation-map placeholders.
- RSS uses a bounded standard-library RSS/Atom parser, removing the missing `gofeed` dependency from the P0 path.
- Default source config includes public news/global sources and one official macro-policy source lane, with unsupported Polymarket removed until an adapter exists.
- Search/research tools exist but are source-service blind.
- VText source entities exist for YouTube/image but not platform source items or official data.
- Publication ledger exists but drops revision metadata and lacks export/access policy.
- Markdown/text file opening creates VText aliases, but the product invariant needs proof and likely tightening.

**what shipped:** local P0 source-service code checkpoint only; not pushed or deployed yet.

**what was proven:**

```text
nix develop -c go test ./cmd/sourcecycled ./internal/cycle ./internal/sources
```

Result:

```text
?    github.com/yusefmosiah/go-choir/cmd/sourcecycled [no test files]
ok   github.com/yusefmosiah/go-choir/internal/cycle
ok   github.com/yusefmosiah/go-choir/internal/sources
```

This proves the sourcecycled command builds, RSS/source identity tests pass, and storage tests prove fetch/item persistence, dedup across restart, source search, cycle records, and official-source caveat metadata persistence.

**unproven or partial claims:**

- exact staging behavior of source entities after current dirty VText draft WIP;
- staging deployment status of any sourcecycled WIP;
- source-service retrieval through researcher tools;
- live ingestion against the default registry;
- publication projection/export/access policy.

**belief-state changes:**

- standalone sourcecycled is no longer the right framing;
- Source Service should be a platform service with clean contracts;
- publication export depends on metadata-preserving publication first;
- source retrieval must become a researcher tool/path, not VText authority;
- service-local SQLite is acceptable for P0 high-churn source/fetch/cycle records, with later projection into runtime ContentItems and platform publication/citation ledgers.

**remaining error field:**

- expose source-service retrieval to researchers;
- reconcile VText draft-versioning WIP with source-entity tests;
- design publication metadata/export policy schema;
- define first official macro source lane from Marco patterns.

**highest-impact remaining uncertainty:** how soon Source Service retrieval
should federate with platform Dolt/publication search versus first proving a
service-local source ledger and researcher tool path.

**next executable probe:**

```text
Add the first researcher-facing source retrieval tool/path over the source
service ledger, then prove a VText/researcher path can cite a source-service
item without giving VText direct retrieval authority.
```

**suggested resume goal string:**

```text
/goal Run docs/mission-platform-source-service-vtext-publication-campaign-v0.md as a Codex-operated MissionGradient campaign. Start with P0 Source Service Nucleus and Markdown-To-VText Canonicalization. Preserve all hard invariants, update the mission checkpoint after each evidence-bearing change, and do not claim completion without staging proof across ingestion, retrieval, VText source entities, publication metadata, and export.
```

**evidence artifact refs:**

- [news-econ-publishing-synthesis-2026-06-04.md](news-econ-publishing-synthesis-2026-06-04.md)
- [mission-vtext-source-entities-multimedia-transclusion-v0.md](mission-vtext-source-entities-multimedia-transclusion-v0.md)
- [mission-standalone-sourcecycled-data-platform-v0.md](mission-standalone-sourcecycled-data-platform-v0.md)

**rollback refs:** not applicable; this draft mutates no product behavior.
