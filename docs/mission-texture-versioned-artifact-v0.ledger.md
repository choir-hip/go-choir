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

### State

Design only; no code authored. The paradoc body is the gate. Ramp D1-D7 defined.
Next: owner review of this paradoc, then begin D1 (typed per-revision provenance
in a new `provenance_json` column on `texture_revisions`) TDD-first.
