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

### 2026-06-18 — Recurring "tests pass, deploy fails" diagnosed + fixed (CI harness)

D1 pushed (`e7967d16`): all CI test jobs green, but **Deploy to Staging (Node B)
failed**. Root cause is harness debt, not a D1 regression: the post-deploy
proxy-upstream gate added 2026-06-16 (`ceb12e66`) curls `127.0.0.1:8081` — the
**auth** service, whose `/health` has no `"upstream"` field — so
`grep '"upstream":"ok"'` always fails and every deploy since then went red even
though the proxy (`:8082`) and all services were healthy at the deployed commit.
Live evidence: `choir.news/health` returned `service:proxy`, `upstream:ok`,
`deployed_commit: e7967d16`, HTTP 200; the in-job dump showed every service
healthy at the pushed SHA. Fix (`01ad15e1`): probe `:8082` and assert
`"service":"proxy"` so a future port slip fails loudly. The fix's own CI was
green but the Deploy job was correctly **skipped** (workflow-only change has no
deployable impact), so the corrected gate is validated on the next code push (D2).

### 2026-06-18 — D2 landed locally (revision_hash chain)

- **Types**: `internal/types/texture_revision_hash.go` —
  `ComputeRevisionHash(parentHash, body, citations, provenance)` =
  `sha256` over a fixed-order canonical payload (scheme-versioned `rev1:`),
  empty citations/provenance normalized to `[]`/`{}`. `Revision.RevisionHash`
  field added.
- **Store**: additive `revision_hash VARCHAR(255) NOT NULL DEFAULT ''` column
  (DDL + `ensureTextureColumn` migration); `CreateRevision` fetches the parent's
  hash inside the txn and computes the child hash (genesis chains from ""),
  threaded through insert + all SELECTs + `scanRevision`. Computing in the store
  guarantees every revision is hashed regardless of write path.
- **Tests (green)**: types determinism / empty-normalization / tamper-detection
  (body + provenance) / parent-tamper-propagation; store genesis + chain
  recomputation. Full build clean.

Mutation class orange (additive schema + hash compute; no reader behavior change).

### 2026-06-18 — D2 deployed; deploy gate fix verified

D2 pushed (`f592052e`): full CI green and **Deploy to Staging (Node B) ✓ in 40s**
— the corrected proxy-upstream gate passed. `choir.news/health` serves
`deployed_commit f592052e`. The recurring "tests pass, deploy fails" is resolved.

### 2026-06-18 — D3+D4 landed locally (typed sources authoritative + citation/quote gate)

Owner decisions (design pass): typed findings packet authoritative; **delete**
regex researcher-prose scraping; **merge D3+D4**; **keep** body-ref normalization
(model's own prose → native refs), revisit later.

- **D4 validator (new, pure, TDD)**: `texture_citation_validation.go` —
  `validateTextureCitations(body, entities, sourceBodies)` checks every inline
  `[label](source:ENTITY_ID)` citation resolves to a collated source entity, and
  for `text_quote` selectors that the quote verifiably appears in the retrieved
  source body (whitespace/case-tolerant). Reasons: `unknown_source`,
  `quote_not_in_source`, `missing_source_body`. `collateCitationSourceBodies`
  fetches bodies from owner content items / resolved source-service items.
  Wired into `commitTextureToolEdit` **before** `CreateRevision`: on any issue it
  returns a tool error (`executeTextureEditTool` surfaces it) so the authoring
  model retries — no mutation-state corruption (gate runs pre-create).
- **D3 cutover**: deleted the regex prose-scrapers
  `sourceServiceItemIDsFromText`, `contentItemIDsFromWorkerMessage`,
  `sourceServiceEntitiesFromWorkerMessages`, `sourceEntitiesFromWorkerMessages`,
  `sourceEntityQuoteFromContext`, and 4 dead REs. Researcher-message scraping
  removed from `texture_agent_revision.go`; coagent prose scraping removed from
  `coagentTextureSourceEntities`/`coagentTextureSourceContentIDs` (typed
  `req.SourceItemIDs` + metadata keys remain authoritative). `contentItemRefToSourceEntity`
  now defaults to `whole_resource` (no scraped quote). Kept `enrichSourceServiceEntities`
  (used by `universal_wire.go`), the body-ref normalizer, and
  `sourceServiceItemRefToSourceEntity`.
- **Tests**: new validator tests (unknown/whole_resource/quote-present/
  whitespace-tolerant/quote-absent/missing-body/error-format); removed three
  obsolete scraping tests; added `TestTextureContentItemSourceEntityDefaultsToWholeResource`.
  Full `scripts/go-test-runtime-shards` green; build + vet clean.

Mutation class red (researcher↔Texture source contract + canonical-write gate).

### 2026-06-18 — D3 cleanly completed (typed evidence activates the quote-match gate)

Instead of adding a parallel quote field, reused the **existing typed `evidence`**
researchers already deliver on `update_coagent` (`{kind, source_uri, title,
content, metadata}`, persisted as `EvidenceRecord`, referenced by
`WorkerUpdateRecord.EvidenceIDs`). `evidence.content` is the bounded excerpt.

- `texture_evidence_sources.go`: `evidenceRecordToSourceEntity` mints a
  `text_quote` selector (excerpt = `evidence.content`) when the evidence
  references a retrievable content item (`metadata.content_id`); URL-only or
  excerpt-less evidence becomes `whole_resource`; unaddressable evidence is
  skipped. `evidenceSourceEntitiesFromPendingUpdates` lists pending
  `update_coagent` records addressed to `texture:<doc_id>`, loads their evidence,
  and collates entities.
- `texture_agent_revision.go`: on worker integration, folds those typed
  evidence entities into `metadata["source_entities"]` — the typed replacement
  for the deleted regex researcher-prose scrape.
- `prompt_defaults/researcher.yaml`: replaced the prose-refs instruction
  (`content_id:<id> beside bounded excerpts`, which fed the deleted scraper) with
  the typed-evidence contract: put the verbatim excerpt in `evidence.content` and
  the imported id in `evidence.metadata.content_id`; the excerpt is validated
  verbatim against the stored source, paraphrase goes in findings.

Result: the validator's `quote_not_in_source` branch is now **active** —
proven by `TestEvidenceDerivedEntityFeedsCitationValidator` (excerpt present →
pass; absent → `quote_not_in_source`). Builder unit tests + full
`scripts/go-test-runtime-shards` green; build clean. Mutation class red (no new
store column or tool field — pure reuse of typed evidence).

### State

Deploy gate fixed + verified. D1 (`e7967d16`), D2 (`f592052e`), D3+D4
(`7a2980c8`) deployed green. D3 completion (typed-evidence quote path) implemented
locally. Next: commit + push (validates deploy gate), then D5 (full-history
publish), D7 (readers/verifier reconcile + staging acceptance proof).
