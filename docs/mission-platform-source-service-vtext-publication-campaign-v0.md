# MissionGradient Campaign: Platform Source Service, VText Source Entities, And Publication v0

**Status:** reviewed draft for owner review  
**Date:** 2026-06-04  
**Method:** Cognitive Transform Portfolio + MissionGradient  
**Supersedes:** [mission-standalone-sourcecycled-data-platform-v0.md](mission-standalone-sourcecycled-data-platform-v0.md) as the active framing  
**Primary synthesis input:** [news-econ-publishing-synthesis-2026-06-04.md](news-econ-publishing-synthesis-2026-06-04.md)

## Review-First Correction

This mission is a review-first revision. The synthesis doc was created before
the Source Service nucleus landed, so it is still valuable as a strategic
snapshot but stale as a code-state report in several places. In particular, it
records a missing `gofeed` dependency, unsupported Polymarket default config,
manual Node B deploy gating, and a sourcecycled build blocker. Current `main`
has already changed those facts:

- `sourcecycled` now builds against a bounded standard-library RSS/Atom parser.
- `configs/sources.json` no longer includes unsupported Polymarket as a default
  source.
- staging deploy on push to `main` has been restored.
- the Source Service ledger nucleus has shipped to staging at
  `4682b092be3ada59e1034c4cdd879f162814f989`.

Therefore this mission should not optimize for "make the old WIP build." The
next real uncertainty is whether the new source ledger can become a product
path: researcher retrieval, VText source identity, metadata-preserving
publication, and canonical copy/download.

## Review Inputs Actually Checked

The mission framing was corrected after reviewing:

- [news-econ-publishing-synthesis-2026-06-04.md](news-econ-publishing-synthesis-2026-06-04.md)
- `cmd/sourcecycled/main.go`
- `configs/sources.json`
- `internal/sources/types.go`
- `internal/cycle/storage.go`
- `internal/cycle/storage_test.go`
- `internal/runtime/tools_research.go`
- `internal/runtime/prompt_defaults/researcher.md`
- `internal/platform/types.go`
- `internal/proxy/platform_publish.go`
- dirty VText draft/versioning WIP in `frontend/src/lib/VTextEditor.svelte`
  and `frontend/tests/vtext-document-stream.spec.js`
- recent commits through `1d070e7 docs: checkpoint source service ledger deploy`

Reviewed current facts:

- Source Service v0 has durable service-local SQLite tables for sources,
  fetches, items, cycles, cycle events, and issue citation-map placeholders.
- `SearchItems` exists in `internal/cycle/storage.go`, but no runtime
  researcher tool uses it yet.
- Researcher tools are currently `web_search`, `fetch_url`,
  `import_url_content`, and `read_content_item`.
- VText does not receive web search tools and should not receive source search
  tools directly.
- Publication publish requests carry content and citations but not
  `source_entities`, `media_source_refs`, transclusions, source-service refs,
  route policy, or export policy.
- VText source entities exist for the YouTube/image path, but not yet for
  source-service items, official data releases, local files, publication spans,
  or private corpus records.
- The dirty VText draft WIP changes autosave semantics so typed drafts persist
  locally without advancing canonical versions; that is strategically aligned
  with "explicit VText writes versions," but it still needs product-path proof
  with source refs.

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

The current `sourcecycled` implementation is no longer just an unbuildable WIP,
but it is still a nucleus rather than the product path:

- `cmd/sourcecycled/main.go` is a blocking daemon that loads
  `configs/sources.json`, polls every 15 minutes, clusters by vertical, runs
  optional LLM synthesis, and writes a service-local SQLite ledger.
- `internal/sources` has RSS, GDELT, and Telegram adapters. Polymarket remains
  a source type in code, but it is not in the default registry until an adapter
  exists.
- RSS no longer imports `github.com/mmcdole/gofeed`; it uses a bounded
  standard-library RSS/Atom parser.
- Storage now has durable tables for source registry policy metadata, fetches,
  items, cycles, cycle events, and issues with a citation-map placeholder.
- `SearchItems` exists and tests cover source item search, persistence, dedup
  across restart, fetch counts, and official-source caveat metadata.
- The daemon still uses hardcoded local paths, has no product-facing CLI/API or
  runtime integration, and has no deployed live ingestion proof.
- Synthesis can save an issue and `SaveIssueManifest` can persist a citation
  map, but there is not yet a verified parser/contract proving generated
  citation labels map to exact item IDs and selectors.
- The synthesizer imports Choir `internal/provider`, confirming that the right
  boundary is a Choir platform Source Service with inspectable contracts, not a
  standalone newspaper island.

### Existing Search And Research State

Choir already has a multi-provider web search plane through the gateway and researcher tools:

- `web_search` routes through the gateway search plane and records provider attempts, health, latency, and result counts.
- `fetch_url`, `import_url_content`, and `read_content_item` give researchers durable owner-scoped source artifacts.
- Researcher prompts correctly require early `submit_coagent_update` checkpoints.
- VText does not have web search tools and should not get them; researchers own source investigation.

The missing capability is a unified source retrieval path. When a researcher
searches the web, the same turn should be able to search:

- platform Source Service items;
- owner-scoped ContentItems and local file artifacts where authorized;
- public Choir/Dolt publication records;
- official macro/economic source artifacts;
- live web search providers.

The first implementation should probably be a researcher-only `source_search`
tool over the Source Service ledger because that is the smallest real bridge
from the landed nucleus to VText work. A later `research_search` can federate
Source Service, ContentItems, platform publications, local filesystem search,
and web search after each individual source plane has proved its provenance
contract.

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

This means copy/download is not the next isolated UI polish task. It is a
publication artifact task: preserve metadata first, then expose bytes from the
canonical private revision or platform publication version.

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

1. **Depth Extraction** - The banal version is "add rich sources." The deep
   version is source identity survival through transformations. The
   load-bearing variable is whether every factual, exported, or transcluded
   claim can resolve to durable source artifacts, selectors, hashes, caveats,
   and access policy. This changes the route by putting fetch/item/retrieval
   contracts ahead of issue writing and export UI.
2. **Review-State Inversion** - The synthesis doc correctly identified old WIP
   blockers, but current `main` already landed the buildable ledger nucleus.
   The live uncertainty is no longer "can sourcecycled build?" but "can
   source-service evidence move through researchers, VText, publication, and
   export without losing identity?" This changes the next probe from build
   repair to researcher retrieval.
3. **Boundary Correction** - `sourcecycled` should not be optimized as a
   standalone product island. It is a Choir platform Source Service with clean
   contracts. CLI/API/WebSocket remain valuable, but the trust and deployment
   boundary is the platform service. This changes scope by rejecting a separate
   newspaper app and a separate publishing system.
4. **Via Negativa** - Remove citation theater, DOM export, parallel source
   browsers, live-only news summaries, VText-as-retriever shortcuts, and
   publication routes that ignore metadata or access policy. This changes the
   implementation order: metadata-preserving publication must precede serious
   export.
5. **Homotopy** - The first slice can be small, but it must have the same
   topology as the full system: registry -> fetch -> item -> retrieval ->
   source entity -> publication metadata -> export. A hardcoded newspaper issue
   that later needs a rewrite is a fake island.
6. **OODA / Inner Loop** - Researchers should search source service,
   local/user artifacts, public Choir records, and web search together. Every
   useful search result should shorten the loop from source discovery to
   grounded VText revision. This changes the tool plan by favoring a
   researcher-only `source_search` bridge now and a federated `research_search`
   after the individual source planes prove out.

### Route-Changing Insights

- The first implementation should prioritize source/fetch/item/retrieval contracts before adding more feeds.
- "Official sources" are not just another adapter category; they need release/vintage/caveat metadata, especially for macro/economic data.
- Publication export depends on metadata-preserving publication. Copy/download before metadata projection would create a weak artifact path.
- Security/RBAC should be modeled now as route/export policy metadata, even if only public/private/unlisted is implemented in the first cut.
- The Markdown-to-VText side bug belongs in P0 because it affects the canonical artifact boundary for every imported source or client draft.
- The dirty VText draft WIP is not just a bug fix; it is the product-level
  inversion that typed drafts are local/noncanonical until explicit VText
  revision. It must be proved with source refs before broad publishing work.

### Changed Plan

**Implementation:** continue from the shipped Source Service ledger nucleus;
add a researcher-only `source_search` over the source ledger before federating
into unified `research_search`; reconcile VText draft/versioning semantics with
source-ref preservation; extend VText source entities for source-service
targets; preserve source metadata through publish; add canonical copy/download
endpoints only after metadata survives publication.

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

### P0: Review Checkpoint And Source Service Baseline

Do not start from stale synthesis blockers. Start from the current baseline:

- source registry policy metadata exists;
- durable source/fetch/item/cycle/cycle-event tables exist;
- stable source item IDs and content hashes exist;
- source item search exists in storage;
- RSS/GDELT/Telegram adapters exist;
- default registry includes public/global and official Fed press source lanes;
- the buildable ledger nucleus has shipped to staging.

The remaining P0 Source Service gaps are:

- no runtime/researcher retrieval tool;
- no live deployed source ingestion proof;
- no source health/backoff summary;
- no exact generated citation-label-to-item selector verifier;
- no Source Service API/WebSocket surface;
- no projection into ContentItems, VText source entities, or platform
  publication records.

### P0: Markdown-To-VText Canonicalization Bug

Prove and fix the product boundary:

- opening `.md`, `.txt`, or equivalent text files for work creates/reuses a VText document alias;
- first edit/revise/save uses VText revision identity;
- a `.vtext` manifest or equivalent durable alias exists;
- original file write-through is explicit export/sync behavior, not the canonical edit store;
- publication uses VText revision metadata, not the raw source file.

This is also where the dirty draft WIP must be judged. The desired invariant is:

```text
typing creates/restores a local draft without advancing canonical versions;
explicit VText save/revise creates exactly one canonical user revision;
source refs and source_entities survive that explicit revision.
```

Do not land source-heavy publication work on top of ambiguous draft/versioning
semantics.

### P0: Researcher Source Retrieval Bridge

Add the first researcher-facing retrieval path over the Source Service ledger:

- `source_search` returns source-service hits with source IDs, item IDs,
  fetch IDs, URLs, titles, excerpts, hashes, caveats, published/fetched
  timestamps, and target kind `source_service_item`;
- VText still does not receive direct source/web retrieval tools;
- researcher prompts require source findings checkpoints after source-service
  evidence just as they do after web/import evidence;
- the first proof can search only the service-local SQLite ledger, but the tool
  response shape must deform into later Source Service API/publication/local
  search federation;
- existing `web_search` remains available. A later `research_search` can join
  source-service hits, public publication hits, authorized local/user content
  hits, and web-search hits after the individual planes are proven.

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

### P1: Publication Metadata Projection

Extend publication payloads and platform records:

- private revision metadata is fetched by proxy and sent to platformd;
- publication artifact manifests include source entities, transclusions, media refs, source service refs, content item refs, and export policy;
- citation/transclusion edges are created from source entities where possible;
- public bundle resolution returns enough metadata for expandable citations and export, subject to route policy;
- retrieval spans become finer than whole-document where needed.

### P1: Copy And Download From Canonical Artifacts

Expose client-visible publishing primitives:

- copy full text for private revision and published version;
- download `.txt`, `.md`, and `.html` from immutable revision/publication content;
- export manifests include content hash, source metadata hash, and source refs;
- UI actions call export endpoints instead of scraping rendered DOM;
- PDF and DOCX follow after the canonical render/export model is stable.

### P2: Publication Access, Role, And Export Policy

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

**last checkpoint:** P0 Researcher Source Retrieval Bridge implemented locally
over the Source Service SQLite ledger after the P0 Source Service nucleus was
landed and reviewed.

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
- Researcher tool profiles now include `source_search` over a configured
  Source Service SQLite ledger path. VText still has no direct source/web
  retrieval tools.
- Sourcecycled and runtime source search share `SOURCE_SERVICE_DB_PATH` /
  `SOURCECYCLED_DB_PATH`, with the daemon retaining `var/sourcecycled.db` as a
  fallback.
- VText source entities exist for YouTube/image but not platform source items or official data.
- Publication ledger exists but drops revision metadata and lacks export/access policy.
- Markdown/text file opening creates VText aliases, but the product invariant needs proof and likely tightening.

**what shipped:** P0 source-service code checkpoint committed and pushed:

```text
4682b092be3ada59e1034c4cdd879f162814f989 feat: add source service ledger nucleus
```

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

Additional local P0 retrieval proof for the current checkpoint:

```text
nix develop -c go test ./internal/runtime -run 'TestResearcherSourceSearch|TestShouldRequireResearchFindingsAfterResearchToolBatches|TestResearcherFailureSynthesizesCheckpointAfterSearch'
```

Result:

```text
ok   github.com/yusefmosiah/go-choir/internal/runtime  6.031s
```

This proves a researcher-only `source_search` tool can read a seeded Source
Service item table and return `target_kind=source_service_item`, item/source/fetch
IDs, content hashes, official-source caveats, result projection metadata, and
checkpoint guidance while remaining unavailable to VText. It also proves source
research participates in the same first-findings checkpoint cadence and runtime
fallback update path as `web_search`.

```text
nix develop -c go test ./cmd/sourcecycled ./internal/cycle ./internal/sources
```

Result:

```text
?    github.com/yusefmosiah/go-choir/cmd/sourcecycled [no test files]
ok   github.com/yusefmosiah/go-choir/internal/cycle    1.115s
ok   github.com/yusefmosiah/go-choir/internal/sources  (cached)
```

This reproves the sourcecycled command still builds after aligning its DB path
configuration with the runtime retrieval bridge.

Broader runtime verification:

```text
nix develop -c go test ./internal/runtime
nix develop -c go test -tags comprehensive ./internal/runtime -run TestInstallDefaultAgentToolsProfiles
```

Result:

```text
ok   github.com/yusefmosiah/go-choir/internal/runtime  18.811s
ok   github.com/yusefmosiah/go-choir/internal/runtime  2.179s
```

Pushed behavior checkpoint:

```text
08be24b994a78cfa51e5a39cf282262a8dc6ccb0 feat: add researcher source service search
```

CI run for that SHA:

```text
GitHub Actions CI run: 26969263344
Go Test (non-runtime): success
Go Test (integration-tagged smoke): success
Go Test (internal/runtime shards 0-3): success
Go Vet + Build: success
Build Frontend: skipped
Go Vet + Test + Build: success
Deploy to Staging (Node B): failure
```

Deploy evidence is mixed. The deploy job step concluded failure, and the
available GitHub API token could not download the private job log
(`403 Must have admin rights to Repository`). However staging health reports
the new commit for both proxy and sandbox:

```text
curl -sS https://choir.news/health | jq '.build,.upstream_build'
proxy deployed_commit:   08be24b994a78cfa51e5a39cf282262a8dc6ccb0
sandbox deployed_commit: 08be24b994a78cfa51e5a39cf282262a8dc6ccb0
deployed_at: 2026-06-04T17:48:00Z
```

This supports "code identity deployed" but does not support a clean landing
loop or deployed product-path source retrieval proof.

**CI and deploy evidence:**

```text
GitHub Actions CI run: 26968010028
Go Test (non-runtime): success
Go Test (integration-tagged smoke): success
Go Test (internal/runtime shards 0-3): success
Go Vet + Build: success
Build Frontend: success
Deploy to Staging (Node B): success
```

Staging health:

```text
curl -sS https://choir.news/health | jq .build,.upstream_build
proxy deployed_commit:   4682b092be3ada59e1034c4cdd879f162814f989
sandbox deployed_commit: 4682b092be3ada59e1034c4cdd879f162814f989
deployed_at: 2026-06-04T17:24:18Z
```

**unproven or partial claims:**

- exact staging behavior of source entities after current dirty VText draft WIP;
- staging deployment status of any sourcecycled WIP;
- deployed source-service retrieval through researcher tools;
- live ingestion against the default registry;
- root cause of the failed GitHub Actions deploy step after staging began
  serving the new commit;
- an externally addressable deployed source-service API or daemon proof;
- publication projection/export/access policy.

**belief-state changes:**

- standalone sourcecycled is no longer the right framing;
- Source Service should be a platform service with clean contracts;
- publication export depends on metadata-preserving publication first;
- source retrieval has a local researcher tool bridge, not VText authority;
- runtime should query the Source Service ledger contract directly rather than
  importing the ingestion/cycle package, because `internal/cycle` currently
  reaches provider/runtime through synthesis and creates an import cycle;
- service-local SQLite is acceptable for P0 high-churn source/fetch/cycle records, with later projection into runtime ContentItems and platform publication/citation ledgers.

**remaining error field:**

- deploy and configure source-service retrieval for researchers on staging;
- reconcile VText draft-versioning WIP with source-entity tests;
- design publication metadata/export policy schema;
- define first official macro source lane from Marco patterns.

**highest-impact remaining uncertainty:** how to configure and prove the
deployed Source Service ingestion/retrieval path on staging without collapsing
Source Service into a VText writer, parallel news app, or DOM/export shortcut.

**next executable probe:**

```text
Push and deploy the local researcher `source_search` bridge, configure a real
staging Source Service ledger path, then prove one real ingested source item can
be found by a researcher and handed to VText as durable coagent evidence without
giving VText direct retrieval authority. In parallel, document and prove the
VText draft/versioning boundary with source refs before publication/export work.
```

**suggested resume goal string:**

```text
/goal Run docs/mission-platform-source-service-vtext-publication-campaign-v0.md as a Codex-operated MissionGradient campaign. Start from the reviewed Source Service baseline, then execute P0 Markdown-To-VText Canonicalization and P0 Researcher Source Retrieval Bridge. Preserve all hard invariants, update the mission checkpoint after each evidence-bearing change, and do not claim completion without staging proof across ingestion, retrieval, VText source entities, publication metadata, and export.
```

**evidence artifact refs:**

- [news-econ-publishing-synthesis-2026-06-04.md](news-econ-publishing-synthesis-2026-06-04.md)
- [mission-vtext-source-entities-multimedia-transclusion-v0.md](mission-vtext-source-entities-multimedia-transclusion-v0.md)
- [mission-standalone-sourcecycled-data-platform-v0.md](mission-standalone-sourcecycled-data-platform-v0.md)

**rollback refs:** not applicable; this draft mutates no product behavior.
