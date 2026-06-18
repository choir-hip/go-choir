# Ledger: Texture As A Versioned Provenanced Artifact v0

## 2026-06-18 — Mission carved out of the durable-thread mission

While starting R0 of `docs/mission-texture-long-running-agent-v0.md` (the durable
deep-research thread mission), a grounding read of the store contradicted the
paradoc's framing and an owner design pass re-scoped the work.

### Grounding correction (what the code actually is)

- `texture_revisions` already separates `content` (markdown), `citations_json`,
  and `metadata_json` columns (`internal/store/texture.go`). Revisions are
  immutable, full-content-per-revision, with `parent_revision_id` +
  `version_number` — i.e. an append-only chain already.
- `content_items` (the retrieved-source substrate) already has `text_content`,
  `content_hash`, `metadata_json`, and a typed `provenance_json` column — a ready
  target for deterministic quote validation.
- So "R0 = rebase markdown into a JSON blob in `content`" was wrong; the document
  is already body-plus-sibling. The Codex-flagged store+frontend+Wire migration
  (P1#10) mostly evaporates because `content` stays markdown.

### Owner design pass (publishing reframe)

Owner: publishing a Texture should publish the **whole version history + metadata**,
not just the latest version. Verified: current publish is **head-only**
(`internal/wirepublish/types.go` `PublishTextureRequest` carries one
`SourceRevisionID` + `Content`). Doctrine already frames Texture as the
"versioned, transclusive artifact control plane"
(`mission-texture-hard-cutover-v0.md:5`), so head-only publish is the deviation.

Decisions (owner):

- **The document IS its versioned history**; head-only publish is wrong. Publish
  carries the chain + per-revision provenance + transclusions.
- **Add a `revision_hash` chain now** (`H(parent_hash, canonical(body+citations+
  provenance))`) — cheap with no users, painful to retrofit; it is the signable
  spine. Signatures themselves stay out of scope.
- **Split into its own paradoc** (`docs/mission-texture-versioned-artifact-v0.md`);
  the long-running-thread mission depends on it.

### Decisions carried over from the thread-mission design pass

- Provenance is **system-attributed**, typed, canonical JSON (no maps →
  deterministic bytes); YAML rejected.
- **Deterministic media ingestion** (YouTube/image embedding + transcript fetch)
  is runtime infrastructure with no model call; researchers do semantic retrieval;
  Texture does no semantic retrieval.
- **Source-type-aware** deterministic citation/quote validation: quote-match
  against stored body for text-bodied sources; id+selector existence for
  media/whole-resource/summary projections; tool-error + retry on failure.
- Delete regex source-scraping (`sourceServiceItemIDsFromText`,
  `contentItemIDsFromWorkerMessage`, body-URL scraping) for typed findings packets.

### 2026-06-18 — D1 landed locally (typed provenance + store column + write path)

Implemented D1 TDD-first. No regressions; additive and backward-compatible.

- **Types**: new `internal/types/texture_provenance.go` — typed `Provenance`
  (`schema_version`, `authoring_model`, `authored_at`, `queries_executed`,
  `sources`) with **no map fields**, plus `CanonicalJSON()` (sorts sources by
  EntityID; preserves query order). Moved the collated source-entity schema
  (`SourceEntity` + sub-structs) into `types` as its canonical home.
- **Runtime**: `texture_media_sources.go` now aliases the runtime
  `textureSourceEntity*` names to the `types` versions (deleted ~60 lines of
  duplicate struct defs). `tools_texture.go` `commitTextureToolEdit` populates
  `rev.Provenance` via system-attributed `buildAppagentRevisionProvenance`
  (model/provider from run metadata, authored_at, sources from the
  runtime-maintained `source_entities`). Best-effort; never blocks a write.
- **Store**: added additive `provenance_json LONGTEXT NOT NULL DEFAULT '{}'`
  column to `texture_revisions` (schema DDL + `ensureTextureColumn` migration);
  threaded it through `CreateRevision` insert, all revision SELECTs, and
  `scanRevision`. `types.Revision` gains a `Provenance json.RawMessage` field.
- **Tests (green)**: `internal/types` provenance canonical determinism +
  order-independence + no-mutate; `internal/store` revision provenance round-trip
  + empty-provenance; `internal/runtime` `buildAppagentRevisionProvenance`
  system-attribution. Full `internal/store` (64s) and `internal/types` packages
  pass; focused runtime suite (source entities / media / edit-commit /
  revision-metadata) passes.

Mutation class orange (additive schema + write-path attribution; no reader
behavior change yet). Not pushed; staging proof deferred to D7 settlement.

### State

D1 landed locally (uncommitted). Next: D2 (per-revision `revision_hash` chain
over canonical bytes; genesis + tamper-detection tests).
