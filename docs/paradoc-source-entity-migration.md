# Parallax: Source Entity Object Migration

## Status

Working design, revised after O3 independent review.

This document is the O3 source program for migrating source truth from Texture
metadata into durable objectgraph objects. It deliberately supersedes the older
outline in this path and salvages the useful preserved O0 design state from
`7a355806` while removing stale `source_embed` assumptions.

## Mission Conjecture

If Choir promotes source truth into durable `choir.source_entity` objects and
version-pinned `choir.source_ref` edges, while preserving Texture canonical
write boundaries and tri-state citation semantics, then News, Texture, and
Autoradio can cite the same source substrate without disappearing sources,
markdown/prose-link regressions, or parallel source inventories.

## Deeper Goal

Source entities are the first object-kind migration after the objectgraph
foundation. O3 should prove the reusable migration pattern:

1. Define the object and edge shape.
2. Write source objects at producer boundaries.
3. Carry source objects through Texture revision creation without weakening
   canonical revision invariants.
4. Render and open source refs through product surfaces.
5. Keep Qdrant and publication/export as rebuildable projections.
6. Deprecate legacy metadata only after graph reads and product proof exist.

## Current Ground

- `internal/objectgraph` exists and registers `choir.source_entity` and
  `choir.source_ref`, but it is not wired into runtime routes and its current
  SQLite store upserts by `canonical_id`. O3 must not pretend it already has
  immutable Texture-participating version chains.
- `internal/qdrant` exists and can build/switch/rollback a derived index from
  objectgraph objects. Qdrant remains derived and disposable, not source truth.
- Choir Doctrine I15 is authoritative: source citation is tri-state, and
  `source_embed` is removed. All body citations are `source_ref` nodes; display
  shape is `source_ref.display_mode`.
- Existing Texture/source APIs still carry legacy `src_...` IDs. O3 must keep
  those compatibility fields while adding explicit canonical graph IDs.

## Mutation Class And Protected Surfaces

Design-only edits are green/yellow. Implementation is orange/red-adjacent
because it touches runtime behavior, product APIs, source identity, Texture
canonical revision writes, source-open routing, publication/export projection,
and Qdrant projections.

Before any behavior-changing O3 commit, the worker must restate:

- conjecture delta;
- protected surfaces touched;
- admissible evidence class;
- rollback path;
- heresy delta (`discovered`, `introduced`, `repaired`).

Problem Documentation First applies if implementation or staging evidence
reveals a new reliable behavior problem.

## Target Object: `choir.source_entity`

`choir.source_entity` is a durable source object. Its canonical ID is stable for
the same owner-scoped source target. Its versions are immutable snapshots of the
source payload as known at a point in time.

Canonical identity:

```text
canonical_id = obj:choir.source_entity:<base64-owner-id>:<source-key>
source-key   = sha256(owner_scope + normalized_source_kind + normalized_target)
```

The source key must prefer stable target identity in this order:

1. canonical URL or reader URL;
2. content item ID;
3. media/file artifact ID;
4. public record ID;
5. legacy Texture `src_...` ID only as a final compatibility fallback.

Required logical fields:

```text
canonical_id
object_kind = choir.source_entity
owner_id
computer_id
version_id
content_hash
body                 # small text snapshot or summary when available
metadata.schema_version = choir.source_entity.v1
metadata.legacy_entity_id
metadata.source_kind         # canonical internal/sourcecontract kind
metadata.target.kind
metadata.target.identity
metadata.display.title
metadata.display.url
metadata.evidence.state      # available | pending | failed | withdrawn
metadata.created_run_id
metadata.created_update_id
metadata.superseded_by
```

Compatibility rule: product/API DTOs may continue to expose `entity_id` or
`source_entity_id` as legacy `src_...` IDs, but canonical graph identity must
use explicit fields such as `canonical_id` and `source_entity_canonical_id`.
Never overload legacy fields with graph IDs.

Version rule: a historical Texture revision must keep resolving to the same
source entity version it cited when the revision was created. Enriching a source
later creates a new source entity version; it must not mutate historical refs.

## Target Edge: `choir.source_ref`

`choir.source_ref` is an edge object from a Texture revision/source occurrence
to a pinned source entity version. It is not a prose link and not a separate
display-node kind.

Canonical identity:

```text
canonical_id = obj:choir.source_ref:<base64-owner-id>:<revision-id>:<occurrence-hash>
```

Required logical fields:

```text
canonical_id
object_kind = choir.source_ref
owner_id
computer_id
version_id
content_hash
metadata.schema_version = choir.source_ref.v1
metadata.doc_id
metadata.texture_revision_id
metadata.body_node_id
metadata.body_node_path_hash
metadata.legacy_source_entity_id
metadata.source_entity_canonical_id
metadata.source_entity_version_id
metadata.display_mode = numbered_ref | expanded_ref
metadata.citation_state = cited
metadata.created_run_id
metadata.created_update_id
```

`display_mode` is reader-toggleable presentation on the same `source_ref` node.
Do not reintroduce `source_embed`, `block_embed`, markdown source links,
footnote prose, or `Source:` lines as canonical citation state.

## Tri-State Citation Semantics

Every available source entity for a Texture revision must end in exactly one of
three states:

- `cited`: the body contains a native `source_ref` node, backed by a
  `choir.source_ref` edge pinned to a source entity version.
- `toolbar-only`: a Style.texture/source entity shaped the document but is not
  cited in the body. It remains visible in source/tooling surfaces, not hidden
  metadata.
- `unused`: the revision records `mark_source_unused` with a rationale.

No source may be silently dropped. O3 implementation must preserve old revision
reads during transition, but new canonical writes must not create a fourth
state through metadata inventories.

## Texture Transaction Boundary

Texture canonical writes are protected. O3 must not make a Texture revision
canonical unless the source graph changes required for that revision are durable
under the same logical commit boundary.

Acceptable implementation routes:

1. Extend the objectgraph store so `choir.source_entity` versions and
   `choir.source_ref` edges can participate in the same Dolt/Texture transaction
   as `CreateTextureRevision`.
2. Or implement Texture-store tables that satisfy the same logical objectgraph
   contract while the generic objectgraph store remains a local foundation.

The implementation worker must choose one route before code changes. If source
object or source-ref writes fail, document head advancement must fail too.

The current `internal/objectgraph` SQLite store is acceptable for focused
package proof, but not sufficient evidence for Texture canonical-state
transactionality.

## Producer Mapping

Producer paths that currently emit `CoagentPacketSource`, `EvidenceRecord`, or
Texture available-source metadata should map into `choir.source_entity` versions
before Texture sees them.

Mapping principles:

- normalize source kind through `internal/sourcecontract`;
- derive canonical target identity from URL/content/media/file/public-record
  fields before legacy IDs;
- preserve legacy `src_...` IDs for compatibility;
- put text snapshots in `body` only when small and relevant for retrieval;
- put structured target/display/evidence data in metadata;
- record producer run/update identifiers in metadata, not canonical identity.

Researcher and source ingestion may continue to carry legacy packet fields in
DTOs during transition, but graph identity is canonical for new source writes.

## Consumer Mapping

Texture revision creation should load candidate sources in this order:

1. graph-backed `choir.source_entity` versions already attached to the document
   or prompt context;
2. compatibility projection from legacy revision/source metadata when graph refs
   are missing;
3. pending coagent/source-ingestion entities addressed to the Texture agent.

Texture body edits must cite with native `source_ref` nodes. The tool layer may
continue accepting legacy `source_entity_id` during compatibility mode, but it
must resolve that ID to `source_entity_canonical_id` plus
`source_entity_version_id` before canonical revision commit.

Source-open and frontend rendering must resolve through source refs first:

```text
source_ref edge -> pinned source_entity version -> target resolver
```

The frontend may display legacy IDs for compatibility/debugging, but source-open
must not parse markdown or prose links as proof of a source path.

## Qdrant And Publication/Export

Qdrant is a derived index over graph source objects. O3 should use the accepted
`internal/qdrant` pipeline when source entities become readable through an
objectgraph-compatible source. Qdrant proof for O3 requires count/hash/sample
query evidence against graph source entities; provider/web-search calls are not
Qdrant evidence.

Publication/export source tables are also derived projections. They may keep
legacy compatibility fields, but graph `source_entity` and `source_ref` records
are canonical for authoring and revision history once graph-read mode is active.

## Phased Plan

### Phase 0: Problem Documentation First

If O3 implementation reveals a new behavior problem, first commit a mission or
checkpoint doc update naming the problem, evidence, belief state, and remaining
error. Fix commits come after.

### Phase 1: Schema And Store Boundary

- Decide objectgraph transaction route: extend generic objectgraph store into
  Texture/Dolt transaction participation, or add Texture-store source tables
  behind the objectgraph contract.
- Add canonical serialization/content hashing for `choir.source_entity` and
  `choir.source_ref`.
- Add deterministic canonical IDs and version IDs.
- Add compatibility projection back to existing Texture DTOs.

### Phase 2: Producer Shadow Writes

- Convert researcher/coagent/source-ingestion packets into source entity
  versions before Texture revision construction.
- Preserve legacy packet fields.
- Add tests proving model/generated IDs cannot become canonical graph IDs.

### Phase 3: Texture Revision Writes

- Resolve every body `source_ref` node to a source entity version.
- Write `choir.source_ref` edges pinned to the Texture revision and source
  entity version.
- Fail the Texture revision commit if source graph writes fail.
- Record toolbar-only and unused states explicitly.

### Phase 4: Reads, Frontend, And Source Open

- Return `source_entities` and `source_refs` object-wrapper records from Texture
  APIs while preserving legacy fields.
- Render `display_mode` as numbered or expanded presentation of the same
  `source_ref`.
- Resolve source-open through source refs and typed target surfaces.

### Phase 5: Publication/Export And Qdrant Projection

- Build publication/export source rows from graph refs.
- Build Qdrant source-entity collection from objectgraph-compatible source reads.
- Switch Qdrant alias only after verification; keep rollback path.

### Phase 6: Deprecation

- Move through `legacy_only -> shadow_write -> graph_read -> enforce`.
- Keep legacy reads until sampled old revisions prove safe.
- Add a static/test gate against new canonical writes to legacy metadata keys:
  `source_entities`, `media_source_refs`, `texture_available_source_entities`,
  and `source_ref_normalization`.

## Verifier Tests

Minimum focused tests before any O3 branch-level acceptance:

- `TestTextureAgentRevisionRegistersMediaSourceEntities` must prove both legacy
  DTO compatibility and graph `choir.source_entity` records.
- `TestTextureAgentRevisionPromotesCoagentPacketSourcesToGraphObjects`.
- `TestPatchTextureWritesSourceRefEdgesPinnedToRevisionAndSourceVersion`.
- `TestPatchTextureSourceRefFailureDoesNotAdvanceDocumentHead`.
- `TestTextureRevisionReadsLegacySourceEntitiesWhenGraphRefsMissing`.
- `TestTextureRevisionDoesNotPersistAvailableSourceEntitiesMetadataForNewWrites`
  in graph-read/enforce modes.
- `TestSourceEntityVersionUpgradeDoesNotMutateHistoricalRefs`.
- `TestMarkSourceUnusedRecordsRationale`.
- `TestToolbarOnlyStyleSourceIsNotSilentlyDropped`.
- `TestSourceRefsAreNativeObjectsNotMarkdownLinks`.
- `TestBackfillLegacyTextureRevisionSourceEntitiesCreatesGraphObjects`.
- `TestTextureSourceGraphModeShadowWriteKeepsLegacyReads`.
- `TestTextureSourceGraphModeEnforceRejectsUngraphableSourceRef`.

Staging/product proof, if behavior-changing code lands to main, must use product
paths such as `/api/prompt-bar`, `/api/texture/*`, `/api/trace/*`, and
`/api/run-acceptances/*` where relevant. Do not use browser-public internal or
test-only routes to seed success.

## Rollback Plan

Runtime rollout must be feature-mode controlled:

```text
legacy_only  -> old behavior only
shadow_write -> write graph objects, read legacy
graph_read   -> read graph first, fallback legacy
enforce      -> require graph-backed source refs for new source-bearing writes
```

Rollback from `shadow_write` or `graph_read` returns reads to legacy revision
JSON and leaves graph objects as inert evidence. Rollback from `enforce`
requires a route/config rollback plus explicit refs for the last safe Git/Dolt
state. Qdrant rollback is alias switch back to the previous verified collection;
Qdrant data remains rebuildable and disposable.

## Independent Review State

O3 design review thread `019f02a7-11d9-7573-885c-d91b7cffe8be` returned
`revise_before_continue` against the prior root design. Required corrections:

- restore useful preserved O0 design content into root;
- remove `source_embed`;
- model tri-state citation explicitly;
- update objectgraph/Qdrant integration for O1/O2 reality;
- define the Texture transaction/versioning boundary before implementation.

This revision applies those corrections as the next candidate for O3 review.

## Settlement

This design is ready to seed implementation only after an independent verifier
returns `accept` that it is doctrine-current and sufficient for a bounded O3
implementation worker.
