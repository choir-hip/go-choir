# O4 Universal Wire Web Capture Read Checkpoint - 2026-06-26

## Work Item

`O4-phase2-universal-wire-web-capture-read`

## Mutation Class

Orange for the planned Universal Wire API/read behavior and runtime
objectgraph service boundary. This checkpoint is the required documentation
step before behavior changes.

## Conjecture Delta

O4 Phase 1 proved a typed `choir.web_capture.v1` objectgraph object can be
created and persisted. O4 Phase 2 tests the narrower conjecture that Universal
Wire can read those graph-backed captures through `/api/universal-wire/stories`
without claiming the full News/sourcecycled ingestion path.

## Protected Surfaces

- `/api/universal-wire/stories` empty-state honesty: the route must still return
  no stories when neither a Universal Wire edition nor graph-backed web capture
  objects exist.
- Objectgraph stable identity, content hash, typed metadata, and edge semantics.
- O3 source entity/source_ref truth and legacy compatibility.
- Sourcecycled/source service ownership boundaries.

No Texture canonical writes, Trace/evidence, auth/session, vmctl,
gateway/provider, candidate, promotion, deployment, publication/export, or run
acceptance surfaces are in scope.

## Evidence

Local code inspection on branch head `f3272233` found:

- `internal/objectgraph/web_capture.go` defines `Service.CreateWebCapture`,
  typed `WebCaptureMetadata`, and `WebCaptureMetadataFromObject`.
- `internal/runtime/universal_wire.go` serves `/api/universal-wire/stories`
  from the Universal Wire Texture edition alias and transcluded Texture article
  heads only.
- `internal/runtime/runtime.go` has no runtime-owned `objectgraph.Service`.
- The only direct runtime import of `internal/objectgraph` is source-ref
  normalization in Texture tooling; route code cannot currently query durable
  `choir.web_capture` objects through a normal runtime service boundary.

## Belief State

The smallest useful Phase 2 fix is to add a runtime-owned objectgraph service,
then teach the Universal Wire route to use it as a fallback projection when no
Texture edition story is available. The route should project the capture itself,
not pretend a sourcecycled article, Texture publication, or cited source_ref
pipeline exists.

## Remaining Error Field

After this slice, Universal Wire may return graph-backed web capture cards from
a controlled fixture, but the following remains out of scope:

```text
sourcecycled/web fetch
-> choir.web_capture object write from the real ingestion path
-> graph query selecting captures for a production News edition
-> processor/story Texture creation with source_ref citations
-> publication/export/staging acceptance
```

## Rollback

Drop/revert this checkpoint and the O4 Phase 2 implementation commit(s). O0-O4
Phase 1 accepted commits remain out of scope.
