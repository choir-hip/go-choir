# O4 Source Artifact Open Proof Checkpoint - 2026-06-26

## Work Item

`O4-phase9-source-artifact-open-proof`

## Mutation Class

Orange for the planned Universal Wire API/UI source-opening behavior. This
checkpoint is the documentation step before code changes.

## Conjecture Delta

O4 Phases 1-8 proved durable `choir.web_capture` objects, graph fallback reads,
additive source/open identity, Source Viewer/Web Lens routing policy,
sourcecycled graph projection, authenticated public-route local proof,
`captured_from` source-entity provenance carry-forward, and honest empty feed
diagnostics. O4 Phase 9 tests the narrower conjecture that a Universal Wire
graph capture card can open a Source Viewer/reader artifact containing the
durable captured text, while Web Lens remains an explicit live/original action.

## Protected Surfaces

- Universal Wire Texture-edition priority over graph fallback.
- Honest graph fallback semantics: capture cards remain `choir.web_capture`
  projections, not Texture publications or native body `source_ref` citations.
- Source-opening doctrine: Source Viewer/reader artifacts are the default
  durable source-opening path; Web Lens is only explicit live/original
  inspection.
- Objectgraph stable identity, content hash, version, tombstone, and edge
  semantics.
- Source entity/source_ref truth: the fix must not mint `choir.source_ref`
  records, body `source_ref` nodes, publication/export state, sourcecycled
  success, provider/search calls, staging evidence, or run acceptance.

No Texture canonical writes, Trace/evidence mutation, auth/session renewal,
vmctl, gateway/provider/model calls, candidate computers, deployment routing,
promotion/rollback, publication/export, Qdrant, or run acceptance surfaces are
in scope.

## Evidence

Local code inspection on branch head `724772c3` found:

- `wireStoryFromWebCaptureObject` reads a durable `choir.web_capture` body and
  uses it for the Wire card's `texture_content` and `wire-style` projection.
- The lead manifest source item carries graph identity, default
  `open_surface: source`, explicit `live_open_surface: web_lens`, and
  `reader_artifact_state: reader_snapshot_ready`.
- `types.WireSourceItem` has no reader-snapshot or text-content field, so the
  durable captured text is not serialized as part of the openable source item.
- `frontend/src/lib/UniversalWireApp.svelte` converts manifest source items
  into source entities for `sourceEntityLaunchPayload`, but it can only pass
  title, URL, graph identity, and reader status. `ContentViewer.svelte` can
  render `sourceEntity.reader_snapshot.text_content` when present; without that
  text it falls back to title and original URL only.
- The Phase 4 browser proof therefore proves source-opening routing and Source
  Viewer default policy, but not that the opened Source Viewer/reader window
  contains a real durable reader artifact from the graph capture.

## Belief State

The smallest useful Phase 9 repair is additive: carry a bounded reader snapshot
from graph-backed Universal Wire source items into the public story DTO, then
have the UI pass that snapshot into the source entity payload used by the
existing Source Viewer launcher. This should preserve Source Viewer as the
default, preserve explicit Web Lens, and keep graph fallback cards separate
from native Texture `source_ref` citations.

## Remaining Error Field

After this slice, branch-level tests may prove graph capture source-opening to
a Source Viewer/reader artifact with durable captured text. The following
remain out of scope:

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

Drop/revert the O4 Phase 9 checkpoint and implementation commits. O0-O4 Phase 8
accepted commits remain out of scope.
