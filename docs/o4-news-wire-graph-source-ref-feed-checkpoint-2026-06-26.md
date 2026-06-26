# O4 News/Wire Graph Source-Ref Feed Checkpoint - 2026-06-26

## Work Item

`O4-phase7-news-wire-graph-source-ref-feed`

## Mutation Class

Orange for the planned Universal Wire API/feed behavior. This checkpoint is the
documentation step before behavior changes.

## Conjecture Delta

O4 Phases 1-6 proved durable `choir.web_capture` objects, Universal Wire graph
fallback reads, additive graph/source-open identity, frontend source opening,
sourcecycled graph projection, and authenticated public-route local proof. O4
Phase 7 tests the narrower conjecture that graph-backed Universal Wire cards can
become more source-ref-native by reading objectgraph provenance edges from
`choir.web_capture` to `choir.source_entity`, while still refusing to mint body
`source_ref` citations outside Texture publication/export.

## Protected Surfaces

- Universal Wire Texture-edition priority over graph fallback.
- Honest empty state when no Universal Wire edition story or graph capture
  exists.
- Non-tombstoned `choir.web_capture` graph fallback only.
- Objectgraph stable identity, content hash, version, and edge semantics.
- Source entity/source_ref truth: feed provenance may expose graph
  `choir.source_entity` handles, but native cited `choir.source_ref` body claims
  require Texture body nodes and remain out of scope.
- Source Viewer remains the default durable source-opening surface; Web Lens is
  only explicit live/original inspection.

No Texture canonical writes, Trace/evidence mutation, auth/session renewal,
vmctl, gateway/provider/model calls, candidate computers, deployment routing,
promotion/rollback, publication/export, Qdrant, or run acceptance surfaces are
in scope.

## Evidence

Local code inspection on branch head `6dec06b4` found:

- `internal/cycle.WriteWebCaptureGraphObjects` creates a `choir.source_entity`
  object for each eligible sourcecycled item, creates a `choir.web_capture`, and
  links the capture to the source entity with a `captured_from` edge.
- `internal/runtime.universalWireWebCaptureStories` lists non-tombstoned
  `choir.web_capture` objects and projects them into `/api/universal-wire/stories`.
- `wireStoryFromWebCaptureObject` exposes the capture object itself as the lead
  manifest source with graph/source-open identity, but it does not inspect the
  `captured_from` edge or expose the sourcecycled `choir.source_entity`
  provenance object.
- There is still no native `choir.source_ref` object or Texture body
  `source_ref` node attached to graph fallback cards. Creating one in the
  Universal Wire fallback route would falsely imply a cited Texture article.

## Belief State

The smallest durable Phase 7 repair is to enrich graph fallback cards with
`captured_from` provenance: for each web capture, read live graph edges to
`choir.source_entity`, decode the source entity metadata, and add those handles
to the Wire source manifest as context. This makes the News/Wire feed more
graph/source-ref-native because it carries both the capture object and its
durable source provenance, without overstating the card as a Texture
publication or body citation.

## Remaining Error Field

After this slice, Universal Wire graph fallback can expose capture plus
source-entity provenance in the feed. The following remain out of scope:

```text
processor/story Texture creation with body_doc source_ref citations
-> publication/export source_ref carry-forward
-> staging deployed Universal Wire proof
-> Qdrant/source search projection
-> complete News benchmark acceptance
```

This checkpoint does not claim native Texture `source_ref` rendering for
Universal Wire capture cards, publication/export, staging, deployment,
auth/session renewal, provider calls, promotion, rollback, Qdrant indexing, or
run-acceptance proof.

## Rollback

Drop/revert the O4 Phase 7 checkpoint and implementation commits. O0-O4 Phase 6
accepted commits remain out of scope.
