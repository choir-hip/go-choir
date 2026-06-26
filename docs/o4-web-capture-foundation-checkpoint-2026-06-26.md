# O4 Web Capture Foundation Checkpoint - 2026-06-26

## Work Item

`O4-phase1-web-capture-object-foundation`

## Mutation Class

Orange for the planned objectgraph runtime/model slice. This checkpoint is a
documentation step before code changes.

## Conjecture Delta

O3 proved Texture/source-open consumption of graph-backed source wrappers
locally. O4 Phase 1 tests the narrower conjecture that Universal Wire can start
moving toward graph-native News if `choir.web_capture` has a durable typed
object shape in objectgraph before any feed rewrite.

## Protected Surfaces

- Objectgraph stable identity, content hash, and edge semantics.
- Source entity and source_ref truth from O3.
- `/api/universal-wire/stories` empty-state honesty.
- Sourcecycled/source service ownership boundaries.

No Texture canonical writes, Trace/evidence, deployment, auth/session, vmctl,
provider/gateway, candidate, promotion, or rollback surfaces are in scope.

## Evidence

Local code inspection on branch head `68cfb026` found:

- `internal/objectgraph/registry.go` already registers `choir.web_capture`.
- `internal/objectgraph/objectgraph_test.go` only proves generic creation and
  listing for the kind; it does not define the required web-capture metadata
  shape.
- `internal/runtime/universal_wire.go` builds `/api/universal-wire/stories`
  from the Universal Wire Texture edition alias and transcluded Texture article
  heads.
- No runtime Universal Wire path currently queries `choir.web_capture` objects.

## Belief State

The smallest useful O4 Phase 1 move is to keep the existing objectgraph storage
and identity semantics intact, then add a typed `choir.web_capture.v1` metadata
contract and focused tests. A full feed rewrite, sourcecycled ingestion change,
or Universal Wire API behavior change would be a later O4 slice.

## Remaining Error Field

After this slice, Universal Wire will still not return graph-backed web capture
stories. The remaining integration gap is:

```text
sourcecycled/web fetch
-> choir.web_capture object write
-> graph query selecting captures for News/Wire
-> processor/story Texture creation or direct feed projection
-> /api/universal-wire/stories reads durable graph-backed captures/stories
```

This checkpoint does not claim staging evidence, non-empty Universal Wire
stories, sourcecycled ingestion, feed query behavior, publication/export
behavior, Qdrant indexing, or product acceptance.

## Rollback

Drop/revert the O4 Phase 1 checkpoint and implementation commits. O0-O3
accepted commits remain out of scope.

