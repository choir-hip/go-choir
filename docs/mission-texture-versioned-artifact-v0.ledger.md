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

### 2026-06-18 — D5 full-history publish payload (history manifest)

Decision (cognitive-transform pass, all lenses converged): persist the full
version history as a **canonical-JSON history manifest embedded in the existing
artifact manifest**, not a new normalized Dolt table. Rationale: the invariant
("publish carries the whole chain + per-rev provenance + transclusions") is a
*containment* truth-condition that a manifest satisfies; normalized tables would
guess a schema before the deferred reader-UX design pass (premature, on the
deploy-flaky Dolt surface); a single canonical manifest carrying the D2 hash
chain is exactly the **signable spine** the mission targets and is independently
verifiable; and the change is additive (head path byte-identical when no history
is supplied). Reader UI rendering stays deferred to D7 as the mission flagged.

Layers (each tested):

- **Sandbox API** (`internal/runtime/texture.go`): `textureRevisionResponse` +
  both record conversions now expose `provenance` and `revision_hash`, so the
  proxy can read the full per-revision spine (previously head-only fields).
- **Platform types** (`internal/platform/types.go`): `PublishTextureRequest.History
  []PublishTextureRevision` (oldest-first chain); response carries
  `version_history_hash` + `version_count`; `PublicationBundle.VersionHistory`
  with `PublicationVersionHistory{schema, revision_count, chain_head_hash,
  manifest_hash, revisions[]}`.
- **Manifest builder** (`internal/platform/version_history.go`):
  `buildVersionHistoryManifest` — deterministic struct marshal, per-entry
  `content_hash`, `chain_head_hash` = head `revision_hash`, manifest hash a pure
  function of the chain; empty chain → no-op (head-only stays unchanged).
- **Publish** (`service.go`): embeds `version_history` + `version_history_hash`
  into the artifact `manifest_json` only when a chain is present.
- **Reader** (`service_publication_read.go`): `publicationVersionHistory` reads
  back the manifest JSON and exposes the chain in the bundle; nil for legacy
  head-only publications.
- **Proxy** (`platform_publish.go`): `gatherTextureRevisionHistory` loads
  `/api/texture/documents/{id}/revisions`, sorts newest→oldest into oldest-first
  causal order, and forwards as `History`.

Tests: `version_history_test.go` (determinism, empty, round-trip),
`version_history_e2e_test.go` (publish-with-history → bundle serves chain +
matching manifest hash + oldest-first; head-only omits history), proxy fakes
extended to serve `/revisions` and assert oldest-first forwarding. Build + vet
clean across runtime/platform/proxy; focused runtime+platform suites green.

Mutation class orange (product publish payload + reader bundle shape; additive,
backward-compatible). Protected surface: publication path / reader bundle.
Rollback: revert the commit; head-only publications and existing manifests are
unaffected (history fields are additive/omitempty).

### 2026-06-19 — D7 acceptance probe discovered a publish-route 404 (problem record)

The first D7 deployed product-path probe surfaced a blocking defect that is
independent of D1-D5: publishing a Texture through the real product router
returns **404 not found**, so no Texture can be published at all via the product
path. This is a problem record; the fix follows in the next commit.

**Evidence.** A new deployed acceptance spec
(`frontend/tests/texture-deployed-versioned-publish.spec.js`, gated on
`GO_CHOIR_RUN_DEPLOYED_VERSIONED_PUBLISH=1`) drove the full path on
`choir.news` at the deployed SHA `6cb2fa4f`: register -> prompt bar ->
multi-revision grounded Texture (V0 prompt + >=2 appagent revisions with 2026
research evidence, each already carrying `revision_hash` + typed `provenance`)
-> publish. Everything up to the publish step succeeded; the publish POST to
`/api/platform/texture/publications` returned
`404 {"error":"not found"}`.

**Root cause (read from `git blame`).** The proxy router
(`internal/proxy/handlers.go` `HandleAPI` switch) has two identical case clauses
for `/api/platform/texture/publications`. The first returns `404`; the second
calls `h.HandleTexturePublication`. Go does not flag duplicate boolean case
expressions (only duplicate constant case values), so this compiles and the
first match wins at runtime — the 404 shadows the real handler. Pre-cutover the
two clauses were *distinct paths*: `/api/platform/vtext/publications` (retired
name -> 404) and `/api/platform/texture/publications` (canonical -> handler).
The hard vtext->Texture ontology cutover (`051623952`, 2026-06-16) rewrote the
retired-name string literal too, collapsing the two cases into one duplicate.
The same rename collapse inverted a tail assertion in
`TestHandleTexturePublicationReadsPrivateRevisionAndPostsProjection`, which now
expected the *canonical* path to 404 (it was meant to assert the retired name
404s). D5's publish tests call `HandleTexturePublication` directly, bypassing
the router, which is why D5 landed green on top of an already-broken route.

**Belief state.** D1-D5 are intact and deployed; the typed-provenance +
hash-chain + history-manifest work is correct (its unit/e2e tests pass and
exercise the handler directly). The defect is in the *router dispatch*, not the
publish/history logic. Publish has been unreachable through the product path
since `051623952` (2026-06-16).

**Remaining error field.** Publish 404 through the router. Fix = delete the
shadowing retired-name 404 clause (the retired name is gone from all live
surfaces by the cutover, so it falls through to the generic 404), drop the
inverted tail assertion, and add a router-level dispatch regression test so the
canonical path is proven to reach the handler. Re-run the deployed spec against
the fixed staging for product-path proof.

### 2026-06-19 — Publish-route 404 fixed; router-dispatch regression test added

Fix commit `736bdc5c` (problem record `f785bdcf`). Deleted the shadowing
retired-name 404 clause in `internal/proxy/handlers.go` (the vtext->Texture
rename had collapsed two distinct cases into one duplicate); the retired name
now falls through to the generic 404. Dropped the inverted tail assertion in
`TestHandleTexturePublicationReadsPrivateRevisionAndPostsProjection`. Added
`TestHandleAPIDispatchesTexturePublication`, which routes through `HandleAPI`
and asserts the canonical publish path reaches the handler (400 on malformed
policy), not a 404 shadow. Full `internal/proxy` package + vet green locally;
CI run `27799524770` success; **Deploy to Staging (Node B) green**;
`choir.news/health` reports `deployed_commit=736bdc5c`. Mutation class orange
(proxy routing), rollback = revert `736bdc5c`.

### 2026-06-19 — D7 acceptance proof PASSED (deployed product path)

`GO_CHOIR_RUN_DEPLOYED_VERSIONED_PUBLISH=1` ran the full product path on
`choir.news` at `736bdc5c` and passed:

- Register (fresh passkey) -> prompt bar -> a **3-revision grounded Texture**
  (V0 = verbatim prompt + 2 appagent revisions consuming researcher evidence;
  each revision carries `revision_hash` + typed `provenance`).
- Publish succeeded: route
  `/pub/texture/create-a-texture-briefing-...-pube60500abe`,
  `version_count=3`, `version_history_hash=9a6fa81d…`.
- Resolve served `version_history` with `schema=choir.platform.version_history.v0`,
  `revision_count=3`, `chain_head_hash=rev1:050dade9…` **== the head revision's
  `revision_hash`**, `manifest_hash=9a6fa81d…` **== the publish response's
  `version_history_hash`**, V0 content preserved verbatim, parent-chain causal
  order holds, head revision provenance present.
- `RunAcceptanceRecord` synthesized from the real trajectory:
  `runacc-a5baefc8def0e2af4436`, **`staging-smoke-level`**, state `blocked`,
  trajectory `53d02fa4-e5c9-4a42-b6c6-632e495e7038`. The `blocked` state is
  honest: a prompt/Texture/publish-only trajectory reaches staging-smoke
  evidence but carries no worker-delegation/export/promotion evidence to clear
  the runtime acceptance state-machine; the spec's own assertions independently
  prove version_history chain integrity.

**Citation-validation dimension.** The D4 source-type-aware citation/quote gate
(`texture_citation_validation.go`) and the typed-evidence source collator
(`texture_evidence_sources.go`) are confirmed present in the deployed SHA
(`880a6aa8` is an ancestor of `736bdc5f`...736bdc5c). Its reject-and-retry
behavior (`quote_not_in_source`/`unknown_source`/`missing_source_body`) is
established by the D3/D4 unit tests (`TestEvidenceDerivedEntityFeedsCitationValidator`
et al.). Forcing a fabricated citation through the live product path is
non-deterministic (researchers supply real excerpts), so the deployed proof
targets version_history serving; the gate's correctness is established at unit
level and by code-presence in the deployed build.

**Residual risks.** (1) `version_number` uses `omitempty`, so the V0 genesis
(version 0) drops the field in the published manifest; causal order is still
encoded in `parent_revision_id` + array order and the `manifest_hash` is over
deterministic marshaled bytes, so signability is unaffected — a minor
data-clarity nit, not a correctness defect. (2) D7 reader UX (frontend
history/diff + source renderer for published version history) is still deferred
(separate track). (3) `continuation-level` is out of scope (transitional H008
residue, re-points at M4 trajectory settlement).

### 2026-06-19 — D7 reader UX Option A landed (version-history disclosure)

The published `/pub/texture/...` reader rendered only the head. Added
`TextureVersionHistory.svelte` — a collapsible "Version history" disclosure
mounted in the published-readonly reader branch (`TextureEditor.svelte`): shows
`revision_count`, manifest + chain-head hashes, a "chain verified" affordance
(chain head == head revision hash), and an oldest-first lineage (version,
author, when, typed-provenance summary, per-revision hash). Read-only; renders
only when `version_history` is present, so head-only publications are
unaffected. This is Option A of
`docs/texture-versioned-reader-ux-options-2026-06-19.md` — the minimal lineage
disclosure and a strict prerequisite for the revision-browser (B) and diff (C)
options, both still deferred pending an owner design pick.

**Deployed proof.** Commit `e859ef27`; CI `27800784979` success (incl. Build
Frontend + Deploy to Staging); staging `deployed_commit=e859ef27`. The deployed
spec navigated to the published route and asserted the panel renders: 3 lineage
rows, chain-verified badge visible. `RunAcceptanceRecord
runacc-12617c0f267b2e67f3b4` at `staging-smoke-level`. Mutation class orange
(frontend publication reader); rollback = revert + redeploy.

### State

D1 (`e7967d16`), D2 (`f592052e`), D3+D4 (`7a2980c8`), D3 completion
(`880a6aa8`), D5 (`6cb2fa4f`) deployed green. Publish-route 404 (found by the
D7 probe) fixed in `736bdc5c` — CI `27799524770` success, staging
`deployed_commit=736bdc5c`. **D7 acceptance proof PASSED** at `736bdc5c`: a
published multi-revision Texture serves its `version_history` chain with a
matching manifest hash and a `chain_head_hash` equal to the head revision hash;
`RunAcceptanceRecord runacc-a5baefc8def0e2af4436` at `staging-smoke-level`.
D6 (signatures) stays out of scope by design. D7 acceptance + doctrine
reconcile done; **reader UX Option A (version-history disclosure) landed**
(`e859ef27`, CI `27800784979`, staging deployed, panel rendered end-to-end —
`runacc-12617c0f267b2e67f3b4`). Settlement for the version-history +
citation-gate + reader-legibility claim is met at staging-smoke-level;
promotion-level awaits AppChangePackage adoption + owner review. Open: reader
UX options B (revision browser) and C (diff + per-revision sources) are
deferred pending an owner design pick (`docs/texture-versioned-reader-ux-options-2026-06-19.md`).
