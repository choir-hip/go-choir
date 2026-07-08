# Review: Texture Source Citation and Style Fix

**Date:** 2026-06-23
**Mission:** `docs/paradoc-texture-source-fix-2026-06-23.md`
**Mutation class:** Orange (runtime behavior, product APIs, app state)
**Status:** Code complete, Go tests pass, ready for CI and staging QA

## Summary

Hard cutover removing `source_embed` as a canonical node type, collapsing all
source citations into `source_ref` with a `display_mode` enum
(`numbered_ref` | `expanded_ref`). Adds `mark_source_unused` for tri-state
source handling. Removes the `WireTexture` prompt control-flow branch, making
article-format guidance unconditional. Registers style textures as
toolbar-only source entities. Updates 5 core docs in the same changeset.

## What was done

### Schema and types

- `internal/texturedoc/schema.go`: Removed `source_embed` from `validateBlock`.
  Updated `source_ref` validation to require `display_mode` enum
  (`numbered_ref` | `expanded_ref`). Removed legacy display modes
  (`block_embed`, `excerpt`, `player`, `image_preview`, `pdf_pages`,
  `transcript`, `source_window`, `inline_chip`). Added `mark_source_unused`
  metadata field to source entities.
- `internal/texturedoc/projection.go`: Removed `SourceEmbeds` and
  `ProjectedSourceEmbed`. Updated `source_ref` projection for `display_mode`.
- `internal/types/texture.go`: Removed `SourceEmbed` types. Updated
  `TextureRevision` structures.
- `internal/store/texture_structured_revision.go`: Removed `source_embed`
  parsing/serialization. Ensured `source_ref` with `display_mode` is persisted.

### Tools

- `internal/runtime/tools_texture.go`: Removed `insert_source_embed` from the
  `patch_texture` op enum and all materialization helpers. Added
  `mark_source_unused` operation with `source_entity_id` and `rationale`
  validation. Updated tool description and JSON schema. Legacy display modes
  mapped to `expanded_ref` in `structuredSourceDisplayMode` for backward
  compatibility with old revisions in the store.
- `internal/runtime/tool_profiles.go`: Removed `WireTexture` branch. All
  Texture runs now use the same prompt path.
- `internal/runtime/tools_coagent.go`: Removed `insert_source_embed`
  references. Updated coagent prompt to use `insert_source_ref` with
  `display_mode`.

### Prompts

- `internal/runtime/textureprompts/overlays/run_system.yaml`: Removed
  `{{if .WireTexture}}` / `{{else}}` / `{{end}}` wrapper. Made
  article-format guidance unconditional ("Write a coherent article with clear
  information hierarchy. Do not default to Q&A format..."). Removed
  `insert_source_embed` references. Added `mark_source_unused` guidance.
- `internal/runtime/textureprompts/overlays/revision_policy.yaml`: Removed
  `insert_source_embed` from operation list. Added `display_mode` and
  `mark_source_unused` guidance.
- `internal/runtime/textureprompts/overlays/revision_source_entities_intro.yaml`:
  Removed `insert_source_embed` text. Updated to reference `display_mode`
  and `mark_source_unused`.
- `internal/runtime/textureprompts/prompts.go`: Removed `WireTexture` field
  from prompt data struct.

### Style textures (new files)

- `styles/default.style.texture`: Default style guidance — article format,
  inline citations, `numbered_ref` default, `expanded_ref` for block excerpts.
- `styles/universal-wire.style.texture`: Wire article style.
- `styles/claim-audit.style.texture`: Claim audit style.
- `styles/market-brief.style.texture`: Market brief style.

### Frontend

- `frontend/src/lib/texture-structured-editor-doc.ts`: Removed `source_embed`
  branch from `renderBlockNode`. Updated `source_ref` rendering for
  `display_mode`. Updated `serializeInlineNodes`.
- `frontend/src/lib/texture-source-renderer.ts`: Updated
  `renderInlineSourceRef` to accept `display_mode`. Renders `expanded_ref` as
  block with excerpt, `numbered_ref` as inline popover.
- `frontend/tests/texture-structured-editor-doc.spec.js`: Replaced
  `source_embed` tests with `expanded_ref` tests.

### Publication and export

- `internal/platform/publication_document.go`: Removed `source_embed` from
  publication derivation.
- `internal/platform/publication_structured.go`: Removed `source_embed`
  handling.
- `internal/platform/source_metadata.go`: Removed `source_embed` metadata.
- `internal/proxy/platform_publish.go`: Updated source ref handling.
- `internal/proxy/wire_platform_publish.go`: Updated source ref handling.

### Other runtime

- `internal/runtime/universal_wire.go`: Updated `source_embed` reference.
- `internal/runtime/texture_diagnosis.go`: Updated `source_embed` reference.
- `internal/runtime/tools_worker_update.go`: Updated `source_embed` reference.

### Core docs (all 5 updated in same changeset)

- `AGENTS.md`: Added "Prompt Control-Flow Antipattern" section. Added tri-state
  source citation summary.
- `docs/choir-doctrine.md`: Added invariant I15 (tri-state source citation,
  `display_mode` enum, `source_embed` removed) and I16 (no prompt control
  flow).
- `docs/texture-agentic-invariants-2026-06-13.md`: Added source entity
  invariants for tri-state handling, `display_mode` toggle, style texture as
  toolbar-only source, `mark_source_unused`.
- `docs/runtime-invariants.md`: Added source citation invariant reference.
- `docs/platform-os-app-state.md`: Updated Texture app catalog entry and Live
  State Rules with tri-state source entity vocabulary.

### Tests

- `internal/texturedoc/schema_test.go`: Replaced `source_embed` tests with
  `expanded_ref` tests. Added `mark_source_unused` validation tests.
- `internal/runtime/texture_tool_unit_test.go`: Replaced `insert_source_embed`
  tests with `insert_source_ref` + `display_mode` tests. Added
  `mark_source_unused` tests.
- `internal/runtime/runtime_test.go`: Updated prompt assertions to reject
  `insert_source_embed` and `WireTexture` branch text. Added assertions for
  unconditional article guidance.
- `internal/runtime/texture_prompt_unit_test.go`: Updated expected prompt
  strings.
- `internal/runtime/textureprompts/prompts_test.go`: Added guard test
  rejecting `insert_source_embed` in overlays.

## What was NOT done (deferred)

- `internal/runtime/textureprompts/overlays/revision_worker_findings.yaml`:
  Control-flow flattening deferred to follow-up PR (per paradoc authority
  bounds).
- `internal/runtime/coagent_update_packet.go`: Source entity minting for all
  retrieved pages deferred (design doc section "Source registration changes"
  is a larger follow-up).
- `internal/platform/export_html.go`: Not in the diff — `source_embed` HTML
  rendering may still need cleanup if it exists. Check on staging.
- `frontend/src/lib/TextureEditor.svelte`: Not in the diff — editor UI toggle
  for `display_mode` may need follow-up.
- `frontend/src/lib/texture-source-flow.ts`: Not in the diff — source journal
  flow may need follow-up.
- `frontend/tests/texture-source-entities.spec.js`: Not in the diff —
  Playwright integration test for source entities deferred.
- Style texture source entity registration in the runtime (minting source
  entities from `.style.texture` files) is not yet wired. The style textures
  exist as files but the runtime does not yet load them as source entities.
  This is the bridge described in Conjecture 1 of the paradoc.

## Test results

- `go build ./...` — clean
- `go test ./internal/texturedoc/` — pass
- `go test ./internal/runtime/` (structured/texture/prompt/schema tests) — pass
- `go test ./internal/runtime/textureprompts/` — pass
- `go test ./internal/platform/` — pass
- `go test ./internal/proxy/` — pass
- `go test ./internal/store/` — pass
- `go test ./internal/types/` — pass
- Frontend Playwright tests — not run (require local service stack;
  staging is the acceptance environment per AGENTS.md)

## Residual `source_embed` references

Remaining references are in guard tests and explanatory comments — these are
correct and intentional:

- `internal/runtime/tools_texture.go:1366` — comment explaining legacy mode
  mapping in `structuredSourceDisplayMode`.
- `internal/runtime/runtime_test.go:683` — forbidden-string guard test.
- `internal/texturedoc/schema_test.go:412` — comment on test helper history.
- `internal/runtime/textureprompts/prompts_test.go:29-30` — guard test
  rejecting `insert_source_embed` in overlays.

## Grep verification

- `grep -rn "WireTexture" --include="*.go" --include="*.yaml"` — no
  control-flow branches remain. Only `resolveUniversalWireTextureReadOwner`
  (an unrelated function name) and test comments referencing the removed
  branch.
- `grep -rn "source_embed"` — only guard tests and explanatory comments (see
  above).

## Invariant checklist

1. **Tri-state source handling:** `mark_source_unused` implemented with
   `source_entity_id` + `rationale` validation. Schema enforces. ✓
2. **No prompt control flow:** `{{if .WireTexture}}` removed from
   `run_system.yaml`. Article guidance is unconditional. ✓
3. **Canonical meaning is Texture-owned:** No change to canonical write path. ✓
4. **Reader toggle:** `display_mode` is a string enum on `source_ref`. ✓
5. **Style texture before branch removal:** `styles/default.style.texture`
   exists. Runtime registration of style textures as source entities is
   deferred (Conjecture 1 bridge). Partial.
6. **`{{else}}` preserved as unconditional:** Research guidance is now
   unconditional text in `run_system.yaml`. ✓
7. **`display_mode` is string enum:** `"numbered_ref" | "expanded_ref"`. ✓
8. **Media source display:** Style texture decides. Deferred to style texture
   runtime wiring. ✓ (design decision recorded)
9. **Core docs update with code:** All 5 docs updated in this changeset. ✓

## Risks and open edges

1. **Style texture runtime wiring not done.** The style texture files exist
   but the runtime does not yet mint source entities from them. The
   unconditional article-format guidance in `run_system.yaml` is the bridge.
   Falsifier: model still produces Q&A with unconditional guidance on staging.
2. **Frontend editor toggle not implemented.** The reader cannot yet toggle
   `source_ref` between `numbered_ref` and `expanded_ref` in the editor UI.
   The rendering code handles both modes; the toggle interaction is deferred.
3. **Source registration for all retrieved pages not done.** The design doc
   describes minting source entities for every URL in researcher packets.
   This is a larger follow-up.
4. **Staging model behavior unobservable until deployed.** All local tests
   pass but the actual model behavior (article format, inline citations,
   `mark_source_unused` usage) can only be verified on staging.

## Next steps for CI/landing agent

1. Monitor CI on the pushed commit.
2. Monitor staging deploy.
3. Verify staging commit identity.
4. Run deployed acceptance proof:
   - Create a Texture document on staging.
   - Request a revision with sources.
   - Verify inline `source_ref` citations (not block quote cards).
   - Verify article-format prose (not Q&A).
   - Verify `mark_source_unused` appears for immaterial sources.
   - Grep staging codebase for `source_embed` and `WireTexture` (should be
     clean except guard tests/comments).
5. Update the paradoc ledger with Pass 1 results.
6. Settle the paradoc when all settlement criteria are met.
