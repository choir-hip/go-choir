# Design: Replace `source_embed` with Expanded/Collapsed `source_ref`

## Status

Draft. Ready for review before implementation.

## Problem

Texture documents currently have two distinct source node types:

- `source_ref`: inline numbered citation point that expands to show source metadata
  and excerpt.
- `source_embed`: block-level source excerpt card rendered as an `<aside>` with a
  title and `<blockquote>`.

This split causes two problems:

1. **Models default to `source_embed` as a citation crutch.** The model can
   choose `insert_source_embed` and get a visible block quote, but that breaks
   the intended inline transclusive citation model. The document fills up with
   source excerpt blocks rather than prose with inline citation points.
2. **Two vocabularies where one suffices.** The same source entity can be cited
   inline or expanded. The expanded state is a presentation choice, not a
   different semantic operation.

## Decision

Remove `source_embed` as a canonical node type and operation. Add an
`expanded` flag to `source_ref` that controls whether the citation point renders
as a collapsed inline marker or an expanded block showing the excerpt. All
source citations become `source_ref` nodes; the expanded/collapsed state is a
per-node display flag.

## New model

```
source_ref {
  id: string
  source_entity_id: string
  display_mode: "numbered_ref" | "expanded_ref"
}
```

- `numbered_ref`: the default collapsed inline citation point. Shows a number
  that expands to reveal the source title and excerpt.
- `expanded_ref`: starts as an expanded block showing the source title and
  excerpt. The reader can collapse it to the numbered citation point.

A `source_ref` node is inline (it lives inside paragraph/heading/list content).
The `display_mode` sets the initial reader state. The reader can toggle any
`source_ref` between expanded and collapsed. When collapsed, every `source_ref`
renders as a numbered citation point.

## Schema changes

### `internal/texturedoc/schema.go`

1. Remove `source_embed` from `validateBlock`.
2. Update `source_ref` validation:
   - `display_mode` must be `"numbered_ref"` or `"expanded_ref"`.
3. Update `validDisplayMode` to allow only `numbered_ref` and `expanded_ref`.
   Remove `block_embed`, `excerpt`, `player`, `image_preview`, `pdf_pages`,
   `transcript`, `source_window`, and `inline_chip`.
4. Remove the `source_embed` branch from `Validate`'s orphan check.

### `internal/types/texture.go`

1. Remove `SourceEmbed` and `BodyDoc` source embed types.
2. Update `TextureRevision` and related structures to remove `source_embed`
   fields.

## Tool changes

### `internal/runtime/tools_texture.go`

1. Update `patch_texture` tool description:
   - Remove `insert_source_embed` from the enum.
   - Remove `insert_source_embed` from the description.
2. Update the JSON schema for `edits`:
   - Remove `insert_source_embed` from the `op` enum.
   - Remove `after_block_id` from the schema. `insert_source_embed` was the only
     operation that used it.
   - Change `display_mode` enum to `["numbered_ref", "expanded_ref"]`.
3. Remove the `source_embed` branch from `materializeTextureToolEdit` and all
   related materialization helpers.
4. Update `validateStructuredTextureEditBatch`:
   - Reject `insert_source_embed`.
   - Handle `expanded_ref` display mode on `insert_source_ref`.

### `mark_source_unused` operation

Add a new `patch_texture` edit operation:

```json
{
  "op": "mark_source_unused",
  "source_entity_id": "src-...",
  "rationale": "Duplicate of the NYT item; no distinct claim supported."
}
```

- The source entity remains in the revision's source entity list.
- The validator does not require a `source_ref` node for marked sources.
- The source appears in the toolbar with an "unused" status.

### `internal/runtime/tools_texture.go` structured edit type

The `textureStructuredEdit` struct has `Offset` and `DisplayMode`. Ensure
`insert_source_ref` supports both `offset` and `display_mode`.

## Renderer changes

### `frontend/src/lib/texture-structured-editor-doc.ts`

1. Remove `source_embed` branch from `renderBlockNode`.
2. Update `source_ref` rendering in `renderInlineNode`:
   - Default `numbered_ref`: render as a numbered inline citation point that
     expands to show the source title and excerpt.
   - Default `expanded_ref`: render as an expanded block showing the title and
     excerpt. The reader can collapse it to the numbered citation point.
3. Update `serializeInlineNodes` to preserve `display_mode` on `source_ref`.
4. Update `renderBlockNode` to project `expanded_ref` nodes as standalone blocks
   when the editor is rendering a read-only view.

### `frontend/src/lib/texture-source-renderer.ts`

1. Update `renderInlineSourceRef` to accept an `expanded` flag.
2. Render the source as a Pretext-based expandable section:
   - Expanded: block with source title, excerpt, and open button.
   - Collapsed: inline numbered citation point with popover.
3. The reader can toggle between states. The toggle does not mutate the
   canonical document; it is a local reader UI state.

### `internal/texturedoc/projection.go`

1. Remove `SourceEmbeds` and `ProjectedSourceEmbed` from `Projection`.
2. Remove the `source_embed` branch from `renderBlock`.
3. Update `source_ref` projection to render `[N]` for the collapsed state. The
   expanded state renders as a block-level marker. Projection is independent of
   the reader's local toggle state.

## Editor changes

### `frontend/src/lib/TextureEditor.svelte`

1. Remove `source_embed` handling from source panel interactions.
2. Render every `source_ref` as a Pretext-based expandable section.
3. Clicking a collapsed `source_ref` expands it inline; clicking an expanded
   `source_ref` collapses it to the numbered point.

### `frontend/src/lib/texture-source-flow.ts`

Replace `source_embed` references in the source journal flow with `source_ref`
`expanded_ref` state.

## Store and persistence changes

### `internal/store/texture_structured_revision.go`

1. Remove `source_embed` handling from structured revision parsing/serialization.
2. Ensure `source_ref` with `display_mode` is persisted.

## Publication and export changes

### `internal/platform/publication_document.go`

1. Remove `source_embed` from publication document derivation.
2. Map `source_ref` expanded state to the publication projection.

### `internal/platform/publication_structured.go`

1. Remove `source_embed` handling.
2. Update `source_ref` projection.

### `internal/platform/export_html.go`

1. Remove `source_embed` HTML rendering.
2. Update `source_ref` HTML rendering for expanded state.

### `internal/platform/source_metadata.go`

1. Remove `source_embed` source metadata handling.

## Core docs update

Core docs must be updated in the same PR to reflect the new model. Code and
docs must not drift.

### `AGENTS.md`

Add the prompt control-flow antipattern principle: prompts provide data and
invariants, not boolean branches that change behavior. Reference the
`docs/prompt-revisions-needed-2026-06-23.md` plan.

### `docs/choir-doctrine.md`

Update source citation doctrine:
- `source_embed` is removed. All citations are `source_ref` with `display_mode`.
- Style textures are source entities in the toolbar, not body citations.
- The `WireTexture` special case is removed. All Texture runs use style
  textures.

### `docs/texture-agentic-invariants-2026-06-13.md`

Update source entity invariants:
- Tri-state source handling: cited, toolbar-only, or marked-unused.
- `expanded_ref` and `numbered_ref` are reader-toggleable display modes.
- `mark_source_unused` is the explicit audit path for immaterial sources.
- Style textures are toolbar-only sources.

### `docs/runtime-invariants.md`

Update if source citation invariants are referenced.

### `docs/platform-os-app-state.md`

Update if source entity vocabulary is referenced.

## Prompt control-flow antipatterns

Prompts should provide data and invariants, not branch on boolean flags. Control
flow in prompts is an antipattern because it makes the model's behavior depend
on hidden runtime conditions rather than on the style texture and the actual
source content.

The following prompt templates contain control flow and should be flattened:

1. `internal/runtime/textureprompts/overlays/run_system.yaml`
   - `{{if .WireTexture}}` / `{{else}}` / `{{end}}` — **remove** (already covered
     above).
2. `internal/runtime/textureprompts/overlays/revision_worker_findings.yaml`
   - `{{if .IntegrateWorkerFindings}}` — flatten to always include the policy:
     when worker packets arrive, incorporate them before spawning more workers.
   - `{{if and .NeedsSuperExecution (not .HasSuperDelivery)}}` — remove. The
     availability of `request_super_execution` is enough; the model decides when
     to use it.
   - `{{if .ActiveWorkerDelegation}}` — remove. Active worker status is context
     the model can read; it does not need a special prompt branch.
3. `internal/runtime/textureprompts/overlays/revision_policy.yaml`
   - `{{if .UserAuthoredRevision}}{{if .OwnerPromptRequestRevision}}` /
     `{{else}}` / `{{end}}` — flatten. The current revision content and the run
     context already tell the model whether this is the first owner prompt or a
     later edit. The policy should be the same for both: consume control text,
     write the next version.
   - `{{if .ExplicitResearcherRequest}}` — remove. The model can detect an
     explicit researcher request from the user text; the prompt should not branch.
   - `{{if .HasGroundedHistory}}` / `{{else}}` / nested `{{if .UserAuthoredRevision}}`
     — flatten. Grounded history is part of the run context; the model sees the
     worker messages and the document head.

Data-only substitutions (e.g., `{{.AgentID}}`, `{{.ChannelID}}`,
`{{.SeedPrompt}}`, `{{.RepoBootstrap}}`) are acceptable because they insert values
rather than switch behavior. They should still be reviewed to ensure they do not
leak into control flow.

### Principle

A prompt should say:

- Here is the style texture to follow.
- Here are the sources available.
- Here are the invariants (cite sources, no model priors as grounded, etc.).
- Here are the tools.

A prompt should **not** say:

- If this is a Wire run, do X; otherwise do Y.
- If this is the first owner prompt, treat it as a request.
- If worker findings are present, incorporate them first.

Those decisions belong in the style texture, the run context, or the tool
availability, not in prompt branches.

## Prompt changes

### `internal/runtime/textureprompts/overlays/revision_source_entities_intro.yaml`

1. Remove all `insert_source_embed` text.
2. Update to say: `insert_source_ref` with `display_mode: expanded_ref` when a
   visible excerpt block is needed next to a claim.

### `internal/runtime/textureprompts/overlays/revision_policy.yaml`

1. Remove `insert_source_embed` from the operation list.
2. Update `insert_source_ref` guidance to include the `display_mode` flag.

### `internal/runtime/textureprompts/overlays/run_system.yaml`

1. Remove `insert_source_embed`.
2. Remove the `{{if .WireTexture}}` / `{{else}}` / `{{end}}` wrapper. Keep the
   `{{else}}` research guidance as unconditional text that every run receives.
3. Remove the Wire-specific text (article-format prohibitions, source embed
   guidance, style texture naming instructions).
4. Add unconditional article-format guidance as a bridge: "Write a coherent
   article with clear information hierarchy. Do not default to Q&A format." This
   is invariant text, not a conditional branch. It stays until the style texture
   source entity mechanism is wired and verified, then moves to
   `styles/default.style.texture`.
5. Update source citation guidance: `insert_source_ref` is the only citation
   operation; use `display_mode: expanded_ref` for block excerpts.

### `internal/runtime/tools_coagent.go`

1. Update `buildCoagentTextureRevisionPrompt` to remove `insert_source_embed`
   references.
2. Update hard requirements to say all citations are `insert_source_ref`; use
   `display_mode: expanded_ref` only when a block excerpt is editorially
   required.

## Style texture changes

Style textures are not raw prompt text. They are Texture documents that become
source entities in the toolbar. They shape the document but are not cited in the
body.

### Invariant: every relevant input is a cited source

A Texture document must cite every input that shaped its content:

- Research results (web pages, search results, evidence records)
- Coagent updates (researcher findings, super outputs)
- Source materials the owner provided
- Related Texture documents that were transcluded

Style textures are also registered as source entities, but they appear in the
source toolbar only. They are not cited in the document body because they are
control inputs, not content sources.

The model does not decide which inputs deserve source status. The runtime mints
source entities for all relevant inputs and registers them on the revision. The
model's job is to place `source_ref` nodes in the body so each material input is
cited next to the prose it supports.

### Style texture as source

When a style texture is selected for a document revision:

1. The style texture document is loaded as a source entity.
2. The source entity target is a `texture_span` pointing to the style texture
   doc ID and a pinned revision ID.
3. The style texture is added to the document's source entity list.
4. The style texture appears in the source toolbar but is **not** cited in the
   document body. The style texture is a control input, not a content source.

The style texture is still a first-class source object, just not a body citation.

### Default style texture

Create `styles/default.style.texture` as a Texture document with sections for:

- Citation shape: `insert_source_ref` after the supported sentence or clause.
- Default display mode: `numbered_ref` (collapsed inline point).
- Expanded display mode: use `expanded_ref` only for long excerpts or media
  that should appear as a block.
- Article format: coherent prose with information hierarchy, no Q&A scaffold.

### Existing style textures

Update the existing Wire style textures to use `expanded_ref` where block source
cards are part of the intended style:

- `styles/universal-wire.style.texture`
- `styles/claim-audit.style.texture`
- `styles/market-brief.style.texture`

### Runtime changes for style texture

1. `internal/runtime/tools_coagent.go`:
   - Instead of copying style texture text into the prompt string, mint a
     source entity for the selected style texture.
   - Add the style texture source entity to the revision's source entity list.
   - Do not prompt the model to cite the style texture in the document body.
2. `internal/runtime/texture_evidence_sources.go` or a new helper:
   - Add `textureSpanSourceEntity` to create a source entity from a Texture
     document ID and revision ID.
3. `internal/runtime/textureprompts/overlays/*.yaml`:
   - Remove inline style text instructions.
   - Tell the model to follow the style texture for voice, structure, and
     citation shape.

### Prompt shape

The Texture prompt should look like:

```
Style sources available to this revision:
- Style.texture: Universal Wire [source_entity_id=style:universal-wire]

Research sources available to this revision:
- ...

Your next patch_texture must write a publishable article revision following
Style.texture: Universal Wire. Cite each material research source with
insert_source_ref nodes in the document body. Use display_mode numbered_ref for
inline citations; use expanded_ref only when a block excerpt is required.

The style texture appears in the source toolbar but is not cited in the body.

If a source is immaterial, use mark_source_unused with a short rationale.
Immaterial sources still appear in the source toolbar but do not need a source_ref
in the body.
```

The style texture is a source entity in the toolbar, not a body citation.

## Source registration changes

### Mint source entities for all retrieved pages

The runtime must register every search result and web page that shapes the
article as a source entity. The user's QA showed 6 sources in the toolbar but
more web pages were retrieved. That violates the "every relevant input is a cited
source" invariant.

Required changes:

1. `internal/runtime/texture_evidence_sources.go`:
   - Mint a source entity for every URL or content ID in a researcher packet.
   - Every retrieved web page that informs the document must appear as a source
     entity.
2. `internal/runtime/coagent_update_packet.go`:
   - Convert all `Packet.Sources` to source entities, including search result
     pages that were not summarized as evidence records.
3. `internal/runtime/texture_media_sources.go`:
   - Register all retrieved web pages as source entities, not only media URLs
     embedded in the document body.

### Source entity validation

The validator in `internal/texturedoc/schema.go` requires every **material**
source entity in the revision to be referenced by a `source_ref` node in the
body.

The model must cite every material source. If the model deems a source
immaterial, it must explicitly mark it as unused via the `mark_source_unused`
operation or equivalent metadata.

### Source filtering

The runtime registers every input that shaped the document as a source entity:

- Style textures
- Research results and web pages retrieved
- Coagent updates
- Owner-provided sources
- Related Texture documents

The model places `source_ref` nodes in the body to cite each material source next
to the prose it supports. Immaterial sources remain in the source entity list and
appear in the toolbar, but the model must explicitly mark them as unused.

### Marking sources immaterial

The model can declare a source immaterial. The declaration must be explicit so
it is auditable and visible to the owner.

Options:

1. **`patch_texture` `mark_source_unused` edit:** Add a new operation
   `mark_source_unused` with a `source_entity_id` and a short rationale. The
   runtime records the immaterial source in the revision metadata but does not
   require a `source_ref` in the body.
2. **Source entity metadata:** Add a `material` boolean field to the source entity
   and require the model to set `material: false` with a rationale when it skips a
   source.

Recommendation: add a `mark_source_unused` operation. It produces a clear audit
record and keeps the source visible in the toolbar.

### Immaterial source invariant

- The toolbar shows all source entities, including immaterial ones.
- The document body shows `source_ref` nodes only for material sources.
- The model cannot silently omit a source. It must either cite it or mark it
  unused.

## Test changes

### Remove `source_embed`

1. `internal/texturedoc/schema_test.go`
   - Remove `source_embed` tests.
   - Add `expanded_ref` tests.
2. `internal/runtime/texture_tool_unit_test.go`
   - Replace `insert_source_embed` tests with `insert_source_ref` expanded
     tests.
3. `internal/runtime/runtime_test.go`
   - Remove `insert_source_embed` from prompt assertions.
   - Add `source_ref` `display_mode` assertions.
4. `internal/runtime/agent_tools_test.go`
   - Remove `insert_source_embed` references.
   - Add `source_ref` citation assertions.
5. `internal/runtime/texture_prompt_unit_test.go`
   - Remove `source_embed` from expected prompt strings.
6. `internal/texturedoc/projection_test.go`
   - Remove `SourceEmbeds` projections.
   - Add expanded `source_ref` projection tests.
7. `frontend/tests/texture-structured-editor-doc.spec.js`
   - Replace `source_embed` tests with `expanded_ref` tests.
8. `frontend/tests/texture-source-entities.spec.js`
   - Remove `source_embed` references.
   - Add expanded citation tests.
   - Add immaterial source tests: toolbar shows the source, body does not.
9. `internal/runtime/texture_tool_unit_test.go`
   - Add `mark_source_unused` tests: validator accepts revisions where marked
     sources have no `source_ref`.

## Migration strategy

Hard cutover. No migration. Old revisions containing `source_embed` nodes are
invalid and fail validation. The product has no users yet, so preserving old
canonical shapes is not required.

- Schema validation rejects `source_embed` nodes.
- `patch_texture` rejects `insert_source_embed` operations.
- All tests and fixtures that rely on `source_embed` must be updated.

## Backward compatibility

- **API:** `patch_texture` calls with `insert_source_embed` are rejected with a
  clear error. This is a breaking change for the tool interface.
- **Documents:** Old revisions with `source_embed` nodes are not supported.
- **Publications:** Existing publications derived from old revisions are not
  supported.
- **Tests:** All tests that assert the presence of `source_embed` in prompts or
  output must be updated.

## Files to touch

```
internal/texturedoc/schema.go
internal/texturedoc/schema_test.go
internal/texturedoc/projection.go
internal/types/texture.go
internal/runtime/tools_texture.go
internal/runtime/texture_tool_unit_test.go
internal/runtime/texture_prompt_unit_test.go
internal/runtime/textureprompts/overlays/revision_source_entities_intro.yaml
internal/runtime/textureprompts/overlays/revision_policy.yaml
internal/runtime/textureprompts/overlays/revision_worker_findings.yaml
internal/runtime/textureprompts/overlays/run_system.yaml
internal/runtime/runtimeprompts/overlays/run_context.yaml
internal/runtime/runtimeprompts/overlays/conductor_run.yaml
internal/runtime/runtimeprompts/overlays/co_super_runtime.yaml
internal/runtime/runtimeprompts/overlays/vsuper_runtime.yaml
internal/runtime/tool_profiles.go
internal/runtime/tools_coagent.go
internal/runtime/coagent_update_packet.go
internal/runtime/agent_tools_test.go
internal/runtime/runtime_test.go
internal/runtime/universal_wire.go
internal/runtime/texture_diagnosis.go
internal/runtime/tools_worker_update.go
internal/store/texture_structured_revision.go
internal/platform/publication_document.go
internal/platform/publication_structured.go
internal/platform/export_html.go
internal/platform/source_metadata.go
internal/proxy/platform_publish.go
internal/proxy/wire_platform_publish.go
frontend/src/lib/texture-structured-editor-doc.ts
frontend/src/lib/texture-source-renderer.ts
frontend/src/lib/TextureEditor.svelte
frontend/src/lib/texture-source-flow.ts
frontend/tests/texture-structured-editor-doc.spec.js
frontend/tests/texture-source-entities.spec.js
AGENTS.md
docs/choir-doctrine.md
docs/texture-agentic-invariants-2026-06-13.md
docs/runtime-invariants.md
docs/platform-os-app-state.md
docs/mission-texture-structured-document-transclusion-cutover-v0.md
styles/default.style.texture (new)
styles/universal-wire.style.texture (update)
styles/claim-audit.style.texture (update)
styles/market-brief.style.texture (update)
```

## Verification plan

1. Unit tests pass for `internal/texturedoc` schema validation.
2. Unit tests pass for `patch_texture` with `insert_source_ref` and
   `display_mode: expanded_ref`.
3. Unit tests pass for prompt generation with no `insert_source_embed`
   references.
4. Frontend tests verify that `source_ref` with `expanded_ref` renders as a
   block with excerpt, and `numbered_ref` renders as inline popover.
5. Staging QA: create a Texture document, request a revision with sources, and
   verify that sources appear as inline citation points, not block quote cards.

## Resolved questions

1. **Media source display:** the style texture decides whether media sources
   (YouTube, images, PDFs) use `expanded_ref` by default or render as inline
   `source_ref` that opens a player. This is a style decision, not a schema
   decision. The default style texture specifies this.
2. **Editor toggle:** the editor allows the reader to toggle a `source_ref`
   between expanded and collapsed directly. This is local reader UI state and
   does not mutate the canonical document. The `display_mode` on the node sets
   the initial state; the reader can change it in their view.
3. **`display_mode` type:** string enum constrained to `"numbered_ref"` and
   `"expanded_ref"`. Not a boolean.

## Related work

- This design removes the `WireTexture` special case in
  `internal/runtime/tool_profiles.go`. Once style textures are the default for
  every Texture run, the Wire pipeline selects the appropriate style texture
  (e.g., `Style.texture: Universal Wire`) and the same runtime path applies to
  all Texture documents. No separate overlay is needed.
- This is part of the larger move to put all document formatting guidance into
  `.style.texture` files rather than runtime prompts.
- Hard cutover is acceptable because the product has no users yet and the old
  `source_embed` shape is not worth preserving.
