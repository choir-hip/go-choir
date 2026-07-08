# O4 Universal Wire Source Identity Carry-Forward Checkpoint - 2026-06-26

## Work Item

`O4-phase3-universal-wire-source-ref-citations`

## Mutation Class

Orange for the planned Universal Wire API/read DTO behavior. This checkpoint is
the required documentation step before the behavior change.

## Conjecture Delta

O4 Phase 2 proved that `/api/universal-wire/stories` can fall back to
graph-backed `choir.web_capture` objects when no Universal Wire Texture edition
story is available. O4 Phase 3 tests the narrower conjecture that those fallback
cards can carry explicit graph/source-open identity for the next Source
Viewer/Web Lens UI proof without pretending the card is a Texture publication,
sourcecycled ingestion result, or native body `source_ref` citation.

## Protected Surfaces

- `/api/universal-wire/stories` empty-state honesty and accepted Texture-edition
  priority from O4 Phase 2.
- O3 source entity/source_ref truth, tri-state citation doctrine, and legacy
  compatibility.
- Objectgraph stable identity/content hash/version semantics from O1/O4 Phase 1.
- Sourcecycled/source-service boundaries.
- Source Viewer/reader artifacts as the default durable source-opening path,
  with Web Lens only for explicit live/original inspection.

No Texture canonical writes, Trace/evidence, auth/session, vmctl,
gateway/provider, candidate, promotion, deployment, publication/export, or run
acceptance surfaces are in scope.

## Evidence

Local code inspection on branch head `03ca986d` found:

- `types.WireSourceItem` exposes `id`, `content_id`, source-service ids, and
  `canonical_url`, but it does not expose graph object kind, canonical id,
  version id, content hash, source kind, target kind, or open-surface policy.
- `wireStoryFromWebCaptureObject` puts the capture `CanonicalID` in
  `Manifest.Lead[0].ID` and the URL in `canonical_url`, but consumers cannot
  distinguish this from a generic source handle or derive the durable graph
  version/content identity.
- There is no `choir.source_ref` object attached to a graph-backed
  `choir.web_capture` fallback card. Minting one in this route would be a false
  citation claim because no Texture body `source_ref` node exists for the
  capture projection.

## Belief State

The smallest useful Phase 3 move is additive DTO enrichment on the graph-backed
fallback manifest item: carry the `choir.web_capture` object kind, canonical id,
version id, content hash, source/target kind, default Source Viewer open
surface, explicit Web Lens alternate, and reader-snapshot readiness. This keeps
the card honest as a capture projection while creating an admissible handle for
the next UI/opening proof.

## Remaining Error Field

After this slice, Universal Wire graph-backed capture cards may expose source
identity sufficient for a frontend opening proof, but the following remains out
of scope:

```text
sourcecycled/web fetch
-> choir.web_capture object write from the real ingestion path
-> processor/story Texture creation with body_doc source_ref citations
-> Source Viewer/Web Lens UI action on Universal Wire cards
-> publication/export/staging acceptance
```

This checkpoint does not claim native Texture `source_ref` rendering for
Universal Wire capture cards, sourcecycled ingestion, Texture publication,
browser rendering, staging, deployment, auth/session, provider calls, promotion,
or rollback proof.

## Rollback

Drop/revert this checkpoint and the O4 Phase 3 implementation commit(s). O0-O4
Phase 2 accepted commits remain out of scope.
