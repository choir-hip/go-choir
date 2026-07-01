# Mission: Texture As A Versioned Provenanced Artifact v0

## Target model (one sentence)

A Texture document **is its full versioned history**: an immutable, hash-chained
sequence of revisions, each carrying system-attributed provenance (authoring
model, timestamps, queries actually executed, collated sources) and transcluded
sources — and **publishing exports the whole chain + per-revision provenance +
transclusions**, not just the head. This is the signable substrate; digital
signatures are out of scope now but the chain is designed so they can be added
without a re-format.

## Why (deeper goal G)

Choir doctrine names Texture "the versioned, transclusive artifact control plane"
(`mission-texture-hard-cutover-v0.md`). The value of a Texture is not a single
answer string; it is the **research lineage** — how the document deepened, which
sources and queries entered at which version, which model wrote each revision.
That history is the chain of custody that makes a published Texture trustworthy
and, later, signable. Flattening to the head revision throws that away. This
mission makes the versioned, provenanced, transcludable document a first-class,
publishable object.

## Problem (what is wrong today)

1. **Publish is head-only.** `internal/wirepublish/types.go`
   `PublishTextureRequest` carries one `SourceRevisionID` + `Content` +
   `Metadata`. The public `/pub` reader shows a single flattened revision; the
   version history and per-version provenance are dropped on publish.
2. **Provenance is untyped and partly fabricated.** Per-revision metadata is a
   loose `metadata_json` map; sources are **regex-scraped** out of researcher
   prose (`sourceServiceItemIDsFromText`, `contentItemIDsFromWorkerMessage`,
   body-URL scraping in `texture_media_sources.go`). Nothing deterministically
   binds a body citation to a real retrieved source, and nothing verifies a cited
   quote against the source body.
3. **Revisions are an append-only chain but not tamper-evident.**
   `texture_revisions` is immutable, full-content-per-revision, with
   `parent_revision_id` + `version_number` (`internal/store/texture.go`), but
   there is no `revision_hash` chaining content+provenance to the parent, so the
   history cannot be verified or signed.

## Owner direction (authoritative constraints)

- **The document is its versioned history.** Head-only publish is wrong. Publish
  carries the whole chain + per-revision metadata + transclusions.
- **Hash the chain now.** Add a per-revision hash chain immediately — cheap with
  no users, painful to retrofit once history exists. It is the signable spine.
- **Provenance is system-attributed, never model-authored.** The model writes
  only `body` + inline citation markers referencing real source ids; the runtime
  fills provenance (model, timestamps, queries executed, collated sources) from
  ground truth. No hallucinated provenance. The user prompt is taken as-is.
- **Deterministic media ingestion.** YouTube/image URLs are retrieved + embedded
  by the runtime with **no model tool call** (incl. deterministic YouTube
  transcript fetch). Researchers do semantic retrieval; Texture does no semantic
  retrieval.
- **Canonical JSON.** Provenance/revision records serialize to canonical JSON (a
  typed struct with no `map[string]any` fields → deterministic bytes; RFC 8785
  JCS is the reference if arbitrary JSON ever enters). Not YAML (no robust
  canonical form). Signatures out of scope but the bytes are chosen to be signable.

## Scope / Domain Ramp

Dependency order; each item earns its keep.

- **D1 Typed per-revision provenance.** Define a typed `Provenance` (no maps):
  `schema_version`, authoring model, created-at, the queries actually executed,
  and the collated `sources` (evolve `textureSourceEntity`, which already has
  entity ids, selectors with `TextQuote`/`ContentHash`, evidence state,
  `provenance.created_by`). Store it in a **dedicated `provenance_json` column on
  `texture_revisions`** (mirrors `content_items.provenance_json`), not stuffed
  into `content`. Populate it **system-attributed** in the write path.
- **D2 Revision hash chain.** Add `revision_hash` =
  `H(parent_revision_hash, canonical(body + citations + provenance))` over the
  canonical bytes. Define the genesis (V0) hash. This makes the whole history
  tamper-evident and is the unit a future signature signs. `content` stays
  markdown; the hash is computed from canonical serialization, not raw column
  bytes.
- **D3 Collated sources schema + delete scraping.** Promote `textureSourceEntity`
  to the canonical collated-sources schema referenced by body citation markers.
  Keep deterministic media ingestion (`registerTextureMediaSourceRefs` ->
  `ImportURLContent`, incl. YouTube transcript fetch) as runtime infrastructure.
  DELETE the regex prose-scraping in favor of typed researcher findings packets.
- **D4 Deterministic citation/quote validation (chain of custody).** Source ids
  are runtime-minted at the retrieval boundary. The same validator runs at the
  researcher->finding and Texture-body->collated-list boundaries. It is
  **source-type-aware**: for sources with a verifiable text body (researcher
  full-text / reader snapshot in `content_items.text_content`/`content_hash`)
  require the cited quote to match a specific stored body version/selector; for
  media / whole-resource / source-service projections / feed summaries validate by
  id+selector existence, not quote. Failures return a tool error (offending refs +
  valid id set) for retry. Hard gate: every citation resolves, and where the
  source has a verifiable body its quote matches. Reverse ("every source cited")
  is soft.
- **D5 Full-history publish.** Evolve the publish path so a published Texture is
  the **chain + per-revision provenance + transclusions**, not the head only:
  `PublishTextureRequest` (and corpusd internal publish, the public `/pub`
  reader, export) carry version history + metadata + transcluded sources. Orange/
  red, touches the platform; design the reader so the latest is prominent while
  the lineage and per-version sources are inspectable.
- **D6 Signatures — OUT OF SCOPE, design-compatible only.** Do not implement
  signing. Only ensure D2's hash chain + D1's canonical bytes are a sufficient
  substrate (sign revision hashes / the head) so signing lands later without a
  re-format.
- **D7 Reconcile readers + proof.** Update the frontend editor/history/diff,
  source renderer, Wire publication, heresy detectors, tests, and doctrine to the
  typed-provenance + hash-chain + history-publish model. Deploy; prove on
  `choir.news` that a published Texture carries its version history with
  per-version provenance and that citations validate (a fabricated citation is
  rejected and retried). Record a RunAcceptanceRecord.

## Invariants / qualities

- **I the artifact is the versioned history**: the document = an immutable
  hash-chained revision sequence; head-only publish is forbidden.
- **I tamper-evident chain**: every revision has `revision_hash` chaining content
  + provenance to its parent; the chain is verifiable and signable (signing later).
- **I system-attributed provenance / no hallucination**: provenance is
  runtime-filled from ground truth; the model authors only body + citation refs;
  the user prompt is authoritative and taken as-is.
- **I source chain of custody**: source ids runtime-minted at retrieval;
  deterministic, source-type-aware citation+quote validation at the researcher and
  Texture boundaries; failures return a tool error for retry; a deterministic
  gate, not a classifier.
- **I retrieval split**: deterministic media ingestion (YouTube/image embedding +
  transcript fetch) is runtime infrastructure with no model call; semantic
  retrieval is the researcher's job; Texture does no semantic retrieval.
- **I canonical JSON**: provenance/revision records are typed (no maps) and
  serialize to deterministic, signable bytes; YAML is at most a human projection.
- **I publish carries history + metadata + transclusions**, never a flattened
  head.
- **I hard cutover**: no dual code path, no data migration (no users); but every
  reader of revision content/metadata is updated in the cutover.

## Open design edges

- **Hash function + canonicalization details.** Pick the hash (e.g., SHA-256) and
  the exact canonical serialization (typed struct field order is deterministic;
  forbid `map[string]any`; define how `content`/`citations` are folded in). RFC
  8785 only matters if arbitrary JSON enters.
- **Media transcript delivery to researchers (deferred).** Inject transcript text
  vs hand a content handle read via `list_content_item_selectors` /
  `read_content_item_selector`, possibly adaptive to the researcher's context
  window. Does not block the schema.
- **Public reader UX for versioned history.** What a published Texture looks like
  (latest prominent + lineage + per-version sources). Product/design pass before
  D5.
- **Quote false-accept hardening.** Binding citation markers to a specific stored
  body version (raw bytes vs cleaned reader markdown differ); the validator must
  reference the content version it checked.

## Budget / authority / mutation class

Execution mutation class **red**. Protected surfaces: Texture canonical writes,
the `texture_revisions` schema (new `provenance_json` + `revision_hash`),
provenance/source bookkeeping, the citation validator, platform publication
(route generation, public `/pub` reader, export), Wire publication, Trace, and
deployment routing. Before touching orange/red surfaces name the conjecture
delta, protected surfaces, admissible evidence, rollback path, and heresy delta.

## Evidence packet (settlement)

Focused tests: typed provenance round-trip + canonical determinism; revision hash
chain verification (incl. tamper detection); source-type-aware citation/quote
validation (true accept on real quote, reject + retry on fabricated, no
false-reject on media/whole-resource); deterministic media ingestion without a
model call; full-history publish payload includes the chain + per-revision
provenance + transclusions. Then `go-test-runtime-shards`; frontend build; CI;
Node B deploy identity; deployed proof on `choir.news` (published Texture carries
versioned history + per-version provenance; fabricated citation rejected);
RunAcceptanceRecord at staging-smoke-level; residual risk note. Rollback = revert
mission commits.

## Suggested goal string

```text
/goal Use Parallax on docs/mission-texture-versioned-artifact-v0.md (read it and its ledger first). Make a Texture document its full versioned history: each revision immutable with a revision_hash chaining content+provenance to its parent, carrying typed system-attributed provenance (schema_version, authoring model, timestamps, queries actually executed, collated sources) in a new provenance_json column on texture_revisions. The model authors only body + citation markers referencing real source ids; the runtime fills provenance from ground truth (no hallucinated provenance; prompt taken as-is). Keep deterministic media ingestion (YouTube/image embedding + transcript fetch, no model call); delete regex source-scraping for typed researcher findings. Add deterministic source-type-aware citation/quote validation (quote-match against content_items stored body for text-bodied sources; id+selector existence for media/whole-resource; tool-error+retry on failure). Evolve publish so a published Texture carries the whole chain + per-revision provenance + transclusions, not the head only. Signatures are OUT OF SCOPE; only keep the hash chain + canonical JSON as a sufficient substrate. Work D1-D7 in order; land what you can safely prove and leave precise file-cited blockers. Verify with focused internal/runtime tests + scripts/go-test-runtime-shards, then commit -> push origin main -> CI -> Node B deploy identity -> deployed proof on https://choir.news; record a RunAcceptanceRecord at staging-smoke-level. Mutation class red. Rollback = revert mission commits.
```

## Lineage

Carved out of `docs/mission-texture-long-running-agent-v0.md` (the durable
deep-research thread mission) on 2026-06-18 when an R0 design pass found the
document-schema/provenance/publish concern is distinct from thread lifecycle. The
thread mission **depends on this one**: the durable thread writes
provenance-bearing, hash-chained revisions into this schema. Inherits Choir
doctrine ("versioned, transclusive artifact control plane",
`mission-texture-hard-cutover-v0.md`) and aligns with the portfolio's
"versioned, mergeable user data" bridge (P3/P4, `mission-portfolio-2026-06-11.md`).

## Settlement requirement

Not met. Settles only with deployed staging proof that a published Texture is its
versioned history (chain + per-revision system-attributed provenance +
transclusions, not head-only), each revision hash-chained and verifiable,
citations deterministically validated (fabricated citation rejected and retried)
with source-type-aware quote checking, deterministic media ingestion with no model
call, the regex source-scraping deleted, readers/verifier/heresy/tests/docs
reconciled, and a RunAcceptanceRecord at staging-smoke-level or higher. Digital
signatures are explicitly out of scope but the hash chain + canonical bytes must
be a sufficient substrate for adding them later.
