# O4 Sourcecycled Web Capture Ingestion Checkpoint - 2026-06-26

## Work Item

`O4-phase5-sourcecycled-web-capture-ingestion-replacement`

## Mutation Class

Orange for the planned sourcecycled ingestion and objectgraph write behavior.
This checkpoint is the documentation step before code changes.

## Conjecture Delta

O4 Phases 1-4 proved a typed `choir.web_capture` object, Universal Wire graph
fallback reads, graph/source-open identity carry-forward, and frontend source
opening proof. O4 Phase 5 tests the narrower conjecture that the real
sourcecycled fetch cycle can write durable `choir.web_capture` graph objects
through the objectgraph service, so the accepted Universal Wire fallback can
consume ingested captures instead of only test-created objects.

## Protected Surfaces

- Sourcecycled/source-service boundaries: source items remain the ingestion
  artifacts; graph writes are an additive projection.
- Objectgraph stable identity, content hash, typed metadata, and
  `captured_from` edge semantics.
- Universal Wire fallback honesty: sourcecycled captures are still capture
  projections, not Texture publications or native body `source_ref` citations.
- Source Viewer remains the default durable source-opening surface; Web Lens is
  only explicit live/original inspection.

No Texture canonical writes, Trace/evidence, auth/session renewal, vmctl,
gateway/provider/model calls, candidate computers, deployment routing,
promotion/rollback, publication/export, Qdrant, or run acceptance surfaces are
in scope.

## Evidence

Local code inspection on branch head `2d0b171b` found:

- `cmd/sourcecycled/main.go` runs the source fetch cycle, persists fetched
  `sources.Item` rows, emits ingestion events, and queues processor handoffs.
- `internal/cycle.Storage` owns durable sourcecycled source item persistence
  and is reusable without importing runtime.
- `Runtime.ObjectGraph()` opens the objectgraph SQLite DB derived from the
  runtime store path, but sourcecycled is standalone and does not own a
  `Runtime`.
- The existing Universal Wire fallback reads non-tombstoned
  `choir.web_capture` objects for `universal-wire-platform` from the runtime
  objectgraph DB.

## Belief State

The smallest useful Phase 5 move is an additive sourcecycled graph projection:
after source items are persisted, write eligible HTTP(S), non-empty source
items as `choir.web_capture` objects via `objectgraph.Service.CreateWebCapture`.
Create a graph `choir.source_entity` for the source-service item and attach a
`captured_from` edge to keep provenance explicit. Because sourcecycled is a
standalone daemon, the graph write should be enabled by an explicit objectgraph
SQLite path configuration; when that path is the runtime objectgraph DB, the
Universal Wire fallback consumes the same durable captures.

## Remaining Error Field

After this slice, the branch may prove sourcecycled item persistence can also
write graph-backed web captures and that Universal Wire can project those
captures locally. The following remain out of scope:

```text
automatic platform config of the shared objectgraph DB path
-> processor/story Texture creation with native body_doc source_ref citations
-> publication/export/staging acceptance
-> deployed authenticated Universal Wire proof
```

This checkpoint does not claim native Texture `source_ref` rendering for
Universal Wire capture cards, publication/export, staging, deployment,
auth/session renewal, provider calls, promotion, rollback, Qdrant indexing, or
run-acceptance proof.

## Rollback

Drop/revert the O4 Phase 5 checkpoint and implementation commits. O0-O4 Phase 4
accepted commits remain out of scope.
