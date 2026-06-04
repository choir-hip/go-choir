# MissionGradient Campaign v1: Platform Source Service, VText Sources, And Publication

**Status:** draft for owner review  
**Date:** 2026-06-04  
**Supersedes:** deleted v0 draft for this campaign  
**Requirements contract:** [source-external-data-publication.md](source-external-data-publication.md)

## Goal String

```text
/goal Run docs/mission-platform-source-service-vtext-publication-campaign-v1.md as a Codex-operated MissionGradient campaign. Treat docs/source-external-data-publication.md as the authoritative requirements contract and keep the mission conformant to it: Source Service owns source registry, adapters, source policy, ingestion, fetch/item ledgers, source health, manifests, search, item resolution, and future official/public/private source lanes; runtime researcher tools consume Source Service only through internal service APIs; researchers produce durable source findings with refs/selectors/hashes/caveats; VText is the canonical writer of document revisions and revision-scoped source_entities; VText sets display policy metadata from writing context; every visible citation marker is a transclusion point; quoted excerpts that are part of the reading surface default to embedded transclusion; background/support citations default collapsed unless context says otherwise; embedded or expanded transclusions can open their owning app/window/source surface; publication preserves source metadata into immutable citation, transclusion, access-policy, export-policy, manifest, and rollback artifacts; copy/download/export read canonical artifacts, not rendered DOM. Execute as one clean trajectory from the ideal topology backward: replace direct SQLite/sandbox source retrieval with the Source Service API boundary, deploy sourcecycled as a managed service, prove live ingestion and researcher source_search on staging, extend VText source entities over source-service and official-source items, preserve metadata through publication, then add canonical copy/download and route/export policy. Do not claim success without staging evidence across ingestion, service API retrieval, researcher findings, VText metadata, publication metadata, citation-to-transclusion expansion, default embedded quoted excerpts, owning-surface open actions, access/export behavior, and user-visible copy/download.
```

## Ideal State

Choir has one source substrate that supports news, official sources, future private corpora, VText citations, and publication/export.

Source Service owns:

- source registry and source policy;
- adapters for public news, official macro/economic sources, and future private corpus connectors;
- fetch, item, cycle, health, and manifest ledgers;
- search and item-resolution APIs;
- stable source item IDs, hashes, caveats, timestamps, selectors, and provenance.

Researcher agents consume that substrate through tools. They retrieve source evidence, compare it with live web search, and report durable findings. They do not write canonical document text.

VText remains the canonical artifact surface. It writes document revisions and hidden `source_entities`. Every visible citation marker is a transclusion point: tapping/clicking the citation expands the source material inline, and the expanded transclusion can open the owning app/window for source-service items, official data releases, ContentItems, local files, other VTexts, and publication spans.

Publication preserves the VText source metadata into immutable route artifacts: citation records, transclusion records, manifests, access policy, export policy, and downloadable representations.

## Current Belief State

- The Source Service ledger nucleus exists: source records, fetches, items, cycles, cycle events, and search storage.
- A direct runtime/sandbox SQLite `source_search` path was attempted and exposed the wrong boundary. Runtime should call Source Service APIs instead of importing source adapters or reading service storage.
- VText source entities exist for narrow media cases, but not yet for source-service items, official data releases, local files, publication spans, or private corpus records.
- Publication records exist, but publish currently drops important revision metadata and does not yet expose source-aware copy/download or route/export policy.
- Markdown/text artifacts can open through VText aliases, but the canonicalization invariant still needs product-path proof: once a user works on a text artifact in VText, the canonical working object is VText.

## Execution Checkpoint: Source Search Boundary Problem

**Problem observed:** runtime `source_search` consumed Source Service storage
directly. It imports Source Service data types, opens a SQLite DB from
`SOURCE_SERVICE_DB_PATH` / `SOURCECYCLED_DB_PATH`, and reports
`source_service_sqlite`. That contradicts the authoritative contract: Source
Service storage is private to the service, and runtime researcher tools must
retrieve through Source Service APIs.

**Evidence recorded before fix:** `internal/runtime/tools_research.go`
contained the SQLite client and item-table query. `cmd/sourcecycled/main.go`
ran only the ingestion loop and did not expose health, search, or
item-resolution endpoints.

**Checkpoint change:** add a Source Service API contract in
`internal/sourceapi`; expose `sourcecycled` health/search/item-resolution
endpoints; make runtime `source_search` an HTTP API client; route host and VM
sandboxes through `SOURCE_SERVICE_BASE_URL`; keep service-local storage behind
the API boundary.

**Local proof on 2026-06-04:**

```text
nix develop -c go test ./cmd/sourcecycled ./internal/cycle ./internal/sources ./internal/sourceapi
nix develop -c go test ./internal/vmmanager -run 'TestBuildFirecrackerConfig_MicrovmUsesStoreDiskAndKernelParams|TestGuestInitScript_NoProviderCredentials'
nix develop -c go test ./internal/runtime -run 'TestResearcherSourceSearch|TestShouldRequireResearchFindingsAfterResearchToolBatches'
nix develop -c go build ./cmd/sourcecycled ./cmd/sandbox ./cmd/gateway
nix eval .#packages.x86_64-linux.sourcecycled.pname --raw
nix eval .#packages.x86_64-linux.sandbox.pname --raw
```

**Result:** local tests and command builds pass. Staging ingestion, deployed
Source Service API retrieval, researcher product-path retrieval, VText source
entities, publication metadata, access/export behavior, and user-visible
copy/download remain unproven.

## Execution Checkpoint: Publication Source Metadata Gap

**Problem observed:** the platform publication path can publish a VText
projection, but it does not yet carry the revision's hidden source metadata
through the platform boundary. The proxy reads the private VText revision and
posts `content` plus `citations` to `platformd`; `metadata_json` is not part of
the request. Platform records therefore cannot preserve `source_entities`,
default transclusion display policy, route/access policy, or export policy as
canonical publication data.

**Evidence recorded before fix:** `internal/proxy/platform_publish.go`
`sandboxVTextRevision` carries `Citations` but not `Metadata`, and the
constructed `platform.PublishVTextRequest` does not include revision metadata.
`internal/platform/types.go` has no source-entity, transclusion, access-policy,
or export-policy fields on `PublishVTextRequest` / `PublicationBundle`.
`internal/platform/service.go` writes an artifact manifest and citation edges,
but no source-entity or transclusion records derived from VText metadata.

**Fix direction:** extend the internal publication contract so the proxy passes
revision metadata; platformd extracts/preserves `source_entities` and display
policy; publication bundles expose source entities, transclusion records,
access policy, and export policy; copy/download endpoints can later read the
same canonical artifact and policy records instead of scraping rendered DOM.

**Checkpoint change:** proxy now forwards private VText revision metadata to
platformd during publication. Platformd stores publication source entities,
transclusion records, and publication access/export policy in durable platform
tables, includes the source metadata hash and policy in the artifact manifest,
and returns source entities, transclusions, and policy in publication bundles.

**Local proof on 2026-06-04:**

```text
nix develop -c go test ./internal/platform ./internal/proxy -run 'TestPublishVTextCreatesImmutablePublicRecords|TestHandleVTextPublicationReadsPrivateRevisionAndPostsProjection'
nix develop -c go test ./internal/platform ./internal/proxy
nix develop -c go build ./cmd/platformd ./cmd/proxy
```

**Result:** local platform/proxy tests and command builds pass. This proves the
internal publication boundary can preserve source metadata, including a
`source_service_item` entity and an `embedded_excerpt` transclusion policy, in
local service tests. Staging publication, VText UI citation expansion,
source-item open actions, and user-visible copy/download remain unproven.

## Execution Checkpoint: Canonical Publication Export Gap

**Problem observed:** publication bundles carry canonical artifact content, but
there is no dedicated copy/download API that reads the canonical artifact and
enforces export policy. UI copy currently covers the public link, not the full
published text. Download formats are not exposed as publication artifacts.

**Evidence recorded before fix:** proxy exposes publication publish, resolve,
retrieval search, and proposal routes, but no `/api/platform/publications/...`
export or download route. Platformd exposes internal publish/resolve/search and
proposal routes, but no internal export route. The VText UI has
`data-vtext-copy-public` for copying the route URL, not canonical text export.

**Fix direction:** add a platformd export operation that resolves a publication
route, reads the stored artifact blob, checks export policy, and emits
canonical `.txt`, `.md`, or `.html` bytes with a content hash. Proxy should
expose that as a public product API so the frontend can implement copy full
text and download without scraping rendered DOM.

**Checkpoint change:** platformd now exposes an internal publication export
operation over the canonical artifact blob and publication export policy.
Proxy exposes `/api/platform/publications/export`, and VText's publication UI
uses that API for copy-full-text and Markdown download actions.

**Local proof on 2026-06-04:**

```text
nix develop -c go test ./internal/platform ./internal/proxy
nix develop -c go build ./cmd/platformd ./cmd/proxy
npm --prefix frontend run build
```

**Result:** local backend tests, command builds, and frontend production build
pass. This proves the export endpoint and VText controls compile locally.
Staging product-path copy/download, access policy variants, and browser
interaction proof remain unproven.

## Execution Checkpoint: VText Citation/Transclusion Rendering Gap

**Problem observed:** publication bundles now preserve `source_entities` and
transclusions, but the VText renderer still primarily reads source entities
from private revision metadata. Published VText bundles can therefore carry
source metadata without rendering citation markers, default embedded excerpts,
or open-owning-surface controls from that preserved publication metadata.

**Evidence recorded before fix:** `frontend/src/lib/VTextEditor.svelte`
`revisionSourceEntities()` reads `currentRevision.metadata.source_entities` and
falls back to media source refs. `loadPublishedContext()` sets
`publishedBundle` and `editorValue`, but does not project
`publishedBundle.source_entities` / `publishedBundle.transclusions` into the
renderer. Existing inline source controls are chips for `[label](source:id)`
syntax or a broad source rail/deck, not the publication-bundle transclusion
surface required by the source contract.

**Fix direction:** make the VText renderer consume source entities from both
private revision metadata and resolved publication bundles; render compact
citation markers for collapsed citations; render `embedded_excerpt` entities
open by default with quoted source text; and route open actions according to
the source target/open-surface metadata.

**Checkpoint change:** VText source refs now render as compact citation
controls marked as citation/transclusion points. Source entities render as
typed transclusion details with their display policy in the DOM; `embedded_*`
and `expanded` policies open by default. Published bundles are projected into
the same local source-entity rendering shape as private revision metadata, and
source open actions route through the entity target/open-surface metadata.
Publication source metadata normalization now proves that a text quote without
an explicit display policy defaults to `embedded_excerpt`.

**Local proof on 2026-06-04:**

```text
nix develop -c go test ./internal/platform ./internal/proxy
npm --prefix frontend run build
npm --prefix frontend run e2e -- vtext-source-entities.spec.js
git diff --check
```

**Result:** local backend tests prove publication metadata, export policy, and
default quoted-excerpt transclusion normalization. The focused browser test
proves private VText source entities render citation/transclusion controls,
default embedded source detail, media preview, and owning-surface open actions.
Staging product-path proof across live Source Service ingestion, researcher
`source_search`, publication-bundle VText rendering, access/export behavior,
and user-visible copy/download remains unproven.

## Verification Snapshot: Current Local Tree

**Local proof on 2026-06-04:**

```text
nix develop -c go test ./cmd/sourcecycled ./internal/cycle ./internal/sources ./internal/sourceapi
nix develop -c go test ./internal/runtime -run 'TestResearcherSourceSearch|TestShouldRequireResearchFindingsAfterResearchToolBatches'
nix develop -c go test ./internal/vmmanager -run 'TestBuildFirecrackerConfig_MicrovmUsesStoreDiskAndKernelParams|TestGuestInitScript_NoProviderCredentials'
nix develop -c go test ./internal/platform ./internal/proxy
nix develop -c go build ./cmd/sourcecycled ./cmd/sandbox ./cmd/gateway ./cmd/platformd ./cmd/proxy
npm --prefix frontend run build
npm --prefix frontend run e2e -- vtext-source-entities.spec.js
git diff --check
```

**Interpretation:** the current local tree has shape proof for the service API
boundary, runtime researcher API consumption, VM source-service URL propagation,
publication metadata/export, VText source entity rendering, citation-to-
transclusion controls, and default embedded source details. This is not yet
the acceptance proof required by the goal: staging still needs deployed live
Source Service ingestion, deployed researcher `source_search`, deployed VText
publication rendering, deployed copy/download, and route/export policy proof.

## Execution Checkpoint: Sourcecycled Nix Vendor Hash Deploy Failure

**Problem observed:** after pushing commit
`5020f539e0489d515654d0cfcd5134ffa3fafa3c`, GitHub Actions CI run
`26976110949` passed Go/frontend gates but failed the Node B staging deploy.
The host NixOS closure build failed while building
`sourcecycled-0.1.0-go-modules.drv` because the fixed-output vendor hash was
stale.

**Evidence recorded before fix:**

```text
specified: sha256-dcaVDKz/yHrr173nTDgVffcuD2rtjEx418J5VcZ7br0=
got:       sha256-2uExDYKXWdF4NyIMX6NVVXcuXRoTm+/S/CxuwPExXiI=
```

**Fix direction:** update only the `sourcecycled` package `vendorHash` in
`flake.nix` to the hash emitted by the failed staging build, then rerun the
landing loop. Proxy and sandbox public health already reported commit
`5020f539e0489d515654d0cfcd5134ffa3fafa3c`, but the full host closure and
`sourcecycled` service deployment remain unproven until the rerun succeeds.

## Execution Checkpoint: Researcher Source Refs Do Not Become VText Entities

**Problem observed:** the deployed Source Service API boundary can return
`source_service_item` results and the researcher tool can expose those IDs, but
the VText metadata path only normalized source entities that were already in
revision metadata or derived from media source refs. Addressed researcher
updates containing durable `source_service_item:<id>` refs could wake VText and
inform prose, but they did not automatically become revision-scoped
`source_entities`. That leaves the chain from researcher source findings to
VText citation/transclusion metadata incomplete.

**Evidence recorded before fix:** `internal/runtime/vtext.go` normalized
`media_source_refs` into `source_entities`, then passed existing
`source_entities` into the VText prompt. `internal/runtime/vtext_media_sources.go`
had no source-service item target fields and no parser/normalizer for
researcher worker messages containing `source_service_item:` refs.

**Fix direction:** derive bounded `source_service_item` source entities from
eligible addressed researcher worker messages before starting the VText run.
Carry those entities through run/revision metadata, expose their item IDs in
the VText prompt, render them as collapsed citation/transclusion points by
default, and preserve the existing media-source entity path.

## Authoritative Requirements

Use [source-external-data-publication.md](source-external-data-publication.md)
as the official requirements contract for source ingestion, external data
cleaning, VText metadata, transclusion, publication policy, and export. Older
mission, problem, incident, and review docs are evidence artifacts only.

## Cognitive Transform Summary

- **End-state backcasting:** define the final platform boundary first, then implement dependencies in that topology.
- **Via negativa:** remove paths that create future cleanup, especially runtime DB reads, adapter imports in sandbox, DOM export, and metadata-free publication.
- **Boundary correction:** Source Service is a platform service; VText is the canonical writer; publication is the immutable ledger.
- **Evidence over optimism:** local shape tests guide implementation, but staging proves product behavior.

## Work Trajectory

### 1. Source Service Boundary

Turn `sourcecycled` into the deployed platform service boundary:

- managed service on staging;
- internal health endpoint;
- internal search endpoint;
- internal item-resolution endpoint;
- service-local durable storage retained behind the API;
- source health and manifest output sufficient for diagnostics.

Runtime `source_search` becomes an API client. Sandbox/runtime should not import ingestion adapters, mount the Source Service DB, or depend on storage schema as its product contract.

### 2. Live Ingestion And Official Source Lane

Prove real ingestion through the service:

- at least one public/news source;
- at least one official macro/economic source;
- stable source IDs and item IDs across repeated ingestion;
- persisted fetch/item/cycle evidence;
- source health/backoff state;
- release/vintage/caveat metadata for official sources.

Marco/econ POC learnings should inform official-source metadata and artifact shape, not become a separate forecast runtime inside this mission.

### 3. Researcher Retrieval

Expose Source Service results to researcher agents:

- `source_search` returns source-service hits with item/source/fetch IDs, URL, title, excerpt, hashes, caveats, timestamps, and `target_kind`;
- researcher prompts require durable source findings after source-service retrieval;
- `web_search` remains available;
- later federation into `research_search` can combine Source Service, web search, ContentItems, local files, and public Choir/Dolt records once each plane has a proven provenance contract.

### 4. VText Source Entities

Extend VText metadata so source identity survives document work:

- `source_service_item`;
- `official_data_release`;
- `content_item`;
- `local_file`;
- `private_corpus_item`;
- `published_vtext_span`;
- `vtext_revision_span`.

VText rendering should show lightweight citation markers, normally superscripts
or equivalent compact controls. Tapping/clicking a citation expands the
associated transclusion inline. Some source entities should render with their
transclusion embedded by default, especially quoted excerpts that are part of
the argument. The VText agent should be able to set this display mode directly
from context. The expanded or default-embedded transclusion should also offer
an open action into the owning app/window while keeping metadata out of the
prose unless the user asks to expose it.

The Markdown-to-VText boundary is part of this work: editing or revising a text artifact in VText must produce canonical VText revision identity, with source refs preserved.

### 5. Publication And Export

Make publication source-aware:

- publish requests carry VText source metadata;
- platform artifacts store source entities, transclusions, source refs, content hashes, and metadata hashes;
- public/private bundle resolution returns enough source metadata for citations and export subject to policy;
- copy full text comes from canonical artifacts;
- downloads for `.txt`, `.md`, and `.html` come from canonical artifacts;
- route/export policy is represented as data, starting with public/unlisted/private and copy/download controls.

PDF, DOCX, EPUB, richer RBAC, comments, and private legal subscriptions can follow once the artifact and policy model is correct.

## Dense Evidence

Use local tests to shape behavior:

```text
nix develop -c go test ./cmd/sourcecycled ./internal/cycle ./internal/sources
nix develop -c go test ./internal/runtime -run SourceSearch
nix develop -c scripts/go-test-runtime-shards
```

Use staging as acceptance:

- staging health reports the pushed SHA;
- `sourcecycled` service is active;
- live source ledger has nonempty fetch/item rows;
- Source Service API health/search/item resolution works;
- a researcher run retrieves source-service and web evidence;
- VText revision metadata contains source entities;
- VText UI shows citation markers that expand into transclusions and can open
  the owning source app/window;
- publication bundle preserves source metadata;
- copy/download returns canonical artifact bytes with hashes.

## Common Failure Modes To Avoid

- Building a second newspaper/source browser instead of the Source Service substrate.
- Letting runtime/sandbox read the Source Service DB or import ingestion adapters.
- Treating source-rich publication as plain prose plus visible citation text.
- Exporting from rendered DOM instead of canonical artifacts.
- Claiming completion from local tests or code identity without staging product-path evidence.

## Stopping Condition

The campaign is complete only when staging proves the full path:

```text
real source ingestion
  -> Source Service API search/item resolution
  -> researcher source findings
  -> VText source entities and citation-to-transclusion expansion
  -> publication metadata preservation
  -> route/export policy
  -> copy/download from canonical artifacts
```

If only some links are proven, report `checkpoint_incomplete` with the exact next dependency and evidence needed.

## Rollback

- Source registry entries can be disabled without deleting history.
- Source Service API changes must be additive until staging proof is stable.
- Publication route/export behavior must have route rollback refs.
- Runtime tool changes must fall back cleanly to existing `web_search` with a precise blocker when Source Service is unavailable.
