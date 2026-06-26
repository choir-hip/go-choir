# O4 Empty Feed Diagnostics Checkpoint - 2026-06-26

## Work Item

`O4-phase8-empty-feed-diagnostics`

## Mutation Class

Orange for the planned Universal Wire API/UI behavior. This checkpoint is the
documentation step before behavior changes.

## Conjecture Delta

O4 Phases 1-7 proved durable `choir.web_capture` objects, Universal Wire graph
fallback reads, additive source/open identity, sourcecycled graph projection,
authenticated public-route local proof, and graph `captured_from`
`choir.source_entity` provenance carry-forward. O4 Phase 8 tests the narrower
conjecture that an empty Universal Wire response can stay empty while explaining
which durable feed substrates were considered and what safe condition prevented
stories from appearing.

## Protected Surfaces

- Universal Wire Texture-edition priority over graph fallback.
- Honest empty state when no Universal Wire edition story or graph capture
  exists.
- Non-tombstoned `choir.web_capture` graph fallback only.
- Objectgraph stable identity, content hash, version, tombstone, and edge
  semantics.
- Source entity/source_ref truth: diagnostics must not mint source refs,
  publication/export state, sourcecycled success, provider/search calls,
  staging evidence, or run acceptance.
- Browser-public route safety: diagnostics must not expose local filesystem
  paths, secrets, raw internal errors, provider details, or internal/test-only
  route names.

No Texture canonical writes, Trace/evidence mutation, auth/session renewal,
vmctl, gateway/provider/model calls, candidate computers, deployment routing,
promotion/rollback, publication/export, Qdrant, or run acceptance surfaces are
in scope.

## Evidence

Local code inspection on branch head `8471418c` found:

- `internal/runtime/universal_wire.go` returns `stories`, `style_sources`,
  `source`, and optional `edition`; an empty response has no structured reason
  for why Texture or graph substrates produced no cards.
- `HandleUniversalWireStories` already preserves Texture-edition priority and
  logs edition or graph errors server-side, but the public response does not
  expose a bounded, safe empty-feed diagnostic for operators or UI users.
- `universalWireWebCaptureStories` lists only non-tombstoned
  `choir.web_capture` objects and silently returns no stories when the graph is
  unavailable, empty, tombstoned-only, or when candidates fail projection.
- `frontend/src/lib/UniversalWireApp.svelte` renders an honest empty state, but
  the UI cannot show which durable substrates were checked because the DTO does
  not carry that information.

## Belief State

The smallest useful Phase 8 repair is an additive empty-only diagnostics
contract on `/api/universal-wire/stories`. The response should remain empty and
old clients should ignore the new field. Safe diagnostics can name substrate
states such as missing `universal-wire/Wire.texture`, no publishable
transcluded Texture stories, no non-tombstoned graph captures, tombstoned graph
captures filtered out, or source provenance unavailable. Diagnostics must not
serialize raw errors or internal paths.

## Remaining Error Field

After this slice, Universal Wire may explain empty local/public-route responses,
but the following remain out of scope:

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

Drop/revert the O4 Phase 8 checkpoint and implementation commits. O0-O4 Phase 7
accepted commits remain out of scope.
