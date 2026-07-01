# Mission: Texture Structured Document And Transclusion Cutover v0

## Summary

Texture must hard-cut from markdown-ish body text plus source/media sidecars to
a typed structured document whose citation, source, media, and transclusion
objects are first-class document nodes. The product contract is not "links in
markdown"; it is Texture-native source/transclusion state. Clickable markdown
links, prose "Source:" labels, offset-only citation records, and metadata-only
media cards are all insufficient as canonical source representation.

This mission treats ProseMirror/Tiptap-style structured editing as the working
prior art: a schema-governed document tree, immutable transaction updates,
inline atom nodes for source points, block nodes for embedded/transcluded
sources, and model-facing edit tools that apply validated operations rather than
asking the model to author editor JSON.

## Source Documents And Prior Art

- [texture-agentic-invariants-2026-06-13.md](./texture-agentic-invariants-2026-06-13.md)
- [mission-texture-hard-cutover-v0.md](./mission-texture-hard-cutover-v0.md)
- [mission-texture-versioned-artifact-v0.md](./mission-texture-versioned-artifact-v0.md)
- Historical evidence / retired-name predecessor:
  [mission-vtext-source-entities-multimedia-transclusion-v0.md](./mission-vtext-source-entities-multimedia-transclusion-v0.md) <!-- texture-cutover-allow: historical mission evidence path; deletion receipt: texture-hard-cutover-v0 -->
- [ProseMirror guide](https://prosemirror.net/docs/guide/) for schema-governed
  document trees, immutable state, transactions, inline leaf nodes, and
  position mapping.
- [Tiptap overview](https://tiptap.dev/docs/editor/getting-started/overview)
  and [Tiptap node views](https://tiptap.dev/docs/editor/extensions/custom-extensions/node-views)
  for the extension/node-view layer over ProseMirror.
- [Tiptap Content AI](https://tiptap.dev/docs/content-ai/getting-started/overview)
  for prior art on LLM agents editing rich documents through a server-side API
  instead of emitting raw editor JSON.
- [OpenAI function calling](https://developers.openai.com/api/docs/guides/function-calling)
  and [Anthropic tool use](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview)
  for schema-defined tool calls as the LLM integration boundary.
- [Lexical nodes](https://lexical.dev/docs/concepts/nodes) and
  [Slate elements](https://docs.slatejs.org/api/nodes/element) as comparison
  points for typed editor trees, decorator/void nodes, and rich document APIs.

## Owner Direction

- This is before launch; do a hard cutover. Do not design a compatibility migration
  plan except when a concrete production obligation appears later.
- Remove old ways first. Bring back only what is proven required.
- Texture sources are transcluded. Do not support clickable links as the source
  contract.
- Preserve the existing numbered source-point UX: inline numbered points open to
  transcluded source sections, and those source sections can open their own
  source windows.
- Source/media/transclusion must cover text, web, image, video, audio, PDF,
  transcript spans, publication spans, Texture spans, and future source-service
  artifacts.
- The model should not write canonical JSON. It should use a transparent,
  validated edit API that constructs canonical document nodes.
- Editing a paragraph, whether by a human or Texture agent, must not orphan,
  shift incorrectly, or silently delete citation/source points.

## Problem

Texture currently has several partially overlapping source representations:
inline textual syntax, markdown links, `citations_json`, revision metadata,
`media_source_refs`, publication transclusions, source-card rendering, and
source entity experiments. The recent staging proof showed the failure mode
directly: a source entity could exist while the body still rendered a raw
`{{source:...}}` token, because canonical content and source rendering were not
one schema-governed object.

This is not just a renderer bug. It is a substrate bug. As long as canonical
Texture content is plain text/markdown with sidecar metadata, every writer,
renderer, publication path, source panel, and agent prompt can invent or preserve
a parallel citation syntax. That breaks the Texture contract and makes later
multimedia transclusion harder: a video citation, image embed, transcript span,
and web source should differ by source entity target and display policy, not by
four unrelated body syntaxes.

## Target Model

Texture canonical body becomes a typed document tree with one source/transclusion
mechanism:

```text
TextureRevision
  body_doc: StructuredTextureDoc
  source_entities: SourceEntity[]
  provenance: system-attributed revision provenance
  hash_chain: defined by mission-texture-versioned-artifact-v0

StructuredTextureDoc
  block nodes: paragraph, heading, list, quote, code_block, horizontal_rule,
               source_embed, media_embed if separate display is required
  inline nodes: text, hard_break, source_ref, texture_ref
  marks: emphasis/style marks only; ordinary markdown links are not source refs

source_ref/source_embed
  id: document-node id
  source_entity_id: runtime-minted source entity id
  display_mode: numbered_ref | inline_chip | block_embed | excerpt | player
```

`source_ref` is the numbered inline point. It is an atom/leaf node, so editor
transactions move it as a unit. `source_embed` is the expanded transclusion block.
Both point to the same `SourceEntity` object. The source entity owns target type,
selectors, media metadata, transcript ranges, evidence state, provenance, and
open-surface behavior.

There should not be one canonical syntax per medium. A YouTube video, an image,
an article snapshot, a PDF page range, an audio clip, a transcript segment, and
another Texture span all become source entities with different target/selectors
and renderer node views.

## Model-Facing Edit API

The Texture agent should edit through operations like these, not by producing
raw ProseMirror/Tiptap JSON:

```text
read_texture_outline(doc_id)
read_block(doc_id, block_id)
replace_block_text(doc_id, block_id, text)
rewrite_block_with_refs(doc_id, block_id, text_with_markers, refs)
insert_block(doc_id, after_block_id, block_type, text)
insert_source_ref(doc_id, block_id, anchor_or_marker, source_entity_id)
insert_source_embed(doc_id, after_block_id, source_entity_id, display_mode)
attach_source_entity(doc_id, target, selectors, display, evidence)
move_node(doc_id, node_id, before_or_after_node_id)
delete_node(doc_id, node_id, allow_source_ref_removal=false)
suggest_edit(doc_id, target_node_id, operation, rationale)
apply_suggestion(doc_id, suggestion_id)
```

Tool calls are schema-validated. Runtime constructs document nodes, maps editor
positions through transactions, validates source ids, rejects invalid transforms,
and refuses accidental source deletion except when the tool call explicitly declares
that deletion and the UI/agent context makes it admissible.

## D0 Architecture Decision - 2026-06-21

Verdict: `revise_before_continue`. The hard cutover should proceed, but D1/D2
must first document the behavior problem as a checkpoint and then replace the
canonical body substrate in one direction only. The current system still accepts
plain `content` strings plus `citations_json` and metadata source sidecars
(`internal/store/texture.go:52`, `internal/runtime/texture.go:199`), agent
tools still materialize string patches (`internal/runtime/tools_texture.go:73`),
the editor still renders/serializes markdown-ish DOM (`frontend/src/lib/texture-markdown-serializer.ts:10`),
and publication still parses `[label](source:id)` links by regex
(`internal/platform/publication_document.go:276`). Those are not implementation
details; they are the old canonical substrate.

Decision: use a **raw ProseMirror-compatible canonical document shape owned and
validated by Choir**, not Tiptap-owned canonical JSON. ProseMirror's schema/tree
and transaction concepts are the right model; Tiptap is an optional later editor
adapter for Svelte node views, not the server source of truth. The current
frontend has no ProseMirror/Tiptap dependency, and the red surface is the Go
write path, hash chain, source provenance, publication/export, and agent edit
API. Canonical validation must therefore live in Go structs and validators first.

Canonical revision shape for D1/D2:

```text
TextureRevision v2
  body_doc_json: StructuredTextureDoc v1       # canonical body, hash input
  body_text_projection: string                 # derived/search/export fallback only
  source_entities_json: SourceEntity[] v1      # runtime-minted, hash input
  provenance_json: Provenance v2               # system-attributed, hash input
  revision_hash: H(parent_hash, canonical(body_doc, source_entities, provenance))

StructuredTextureDoc v1
  schema: "choir.texture_doc.v1"
  doc: {type:"doc", attrs:{id}, content:block[]}

block nodes:
  paragraph{id} inline*
  heading{id,level:1..6} inline*
  bullet_list{id} list_item+
  ordered_list{id,start?} list_item+
  list_item{id} block+
  blockquote{id} block+
  code_block{id,language?} text*
  horizontal_rule{id}
  source_embed{id,source_entity_id,display_mode,caption?}

inline nodes:
  text
  hard_break
  source_ref{id,source_entity_id,display_mode:"numbered_ref",label?}

marks:
  strong, emphasis, code. Do not make links a source mechanism. If ordinary
  outbound links are reintroduced, they are non-evidentiary marks and must be
  rejected when they use `source:` or claim source/proof semantics.
```

`media_embed` is not a separate canonical node in v1. Image, video, audio, PDF,
web, transcript, Texture, publication, and Source Viewer/reader artifacts all
use `source_ref` or `source_embed` pointing to a `SourceEntity`; display policy
chooses player, image preview, excerpt, PDF page view, transcript snippet, or
source window. This keeps media differences in target/selectors/display rather
than inventing body syntaxes per medium.

Source entity schema for D1/D2:

```text
SourceEntity v1
  source_entity_id: string        # runtime minted
  target: SourceTarget
  selectors: SourceSelector[]
  display: SourceDisplay
  evidence: SourceEvidence
  provenance: SourceEntityProvenance

SourceTarget.kind enum:
  web_url | source_service_item | content_item | image | video | audio | pdf |
  transcript | texture_span | publication_span | source_viewer_artifact |
  reader_artifact | file_artifact

SourceSelector.kind enum:
  whole_resource | text_quote | text_position | paragraph_heading | page_range |
  timestamp_range | transcript_segment | table_range | table_cell |
  image_region | byte_range | selector_set

SourceDisplay.mode enum:
  numbered_ref | inline_chip | block_embed | excerpt | player | image_preview |
  pdf_pages | transcript | source_window

SourceEvidence.state/open_surface/reader_artifact_state:
  use `internal/sourcecontract/source_contract_schema.json` as the enum source,
  and reject unknown values at write/publication boundaries.
```

Agent edit API for D2/D4:

```text
read_texture_outline(doc_id)
read_block(doc_id, node_id)
replace_text(node_id, text, preserve_source_refs:[node_id])
rewrite_block_with_markers(node_id, text_with_markers, marker_bindings)
insert_block(after_node_id, block)
attach_source_entity(target, selectors, display, evidence)
insert_source_ref(block_id, marker_or_offset, source_entity_id)
insert_source_embed(after_node_id, source_entity_id, display_mode)
move_node(node_id, before_or_after_node_id)
delete_node(node_id, deletion_intent, allow_source_ref_removal:false)
suggest_edit(target_node_id, operation, rationale)
apply_suggestion(suggestion_id)
```

The model never sends editor JSON. It sends typed operations; runtime validates
the operation, constructs nodes, preserves or explicitly deletes source refs,
then stores the next canonical revision.

D1/D2 deletion targets:

- `texture_revisions.content` as canonical body; keep only a derived projection
  or remove after readers switch.
- `citations_json` as canonical citation/source identity.
- `metadata.source_entities` and `metadata.media_source_refs` as canonical
  source sidecars.
- `[label](source:id)`, `[source:id]`, `{{source:...}}`, prose `Source:` handles,
  and unresolved `[1]` citation markers as accepted write syntax.
- Regex source discovery/render paths over body text, including publication
  source parsing from markdown links.
- Agent string patch/rewrite tools as the primary edit surface.
- DOM-to-markdown editor serialization as canonical save path.
- Publication/export history entries that carry only flattened `content` rather
  than `body_doc` plus `source_entities`.

## Problem Checkpoint - Canonical Body Is Still Markdown-ish - 2026-06-21

Mutation class: `green` documentation checkpoint only. No runtime behavior,
schema, API, prompt, editor, publication, or source resolver code changes in
this checkpoint.

Reliable D0 review evidence shows the first behavior problem to fix:

> Texture canonical writes still accept markdown-ish content strings and sidecar
> source metadata, so source entities can exist without being document nodes.

This violates the intended Texture source/transclusion invariant. A source
entity that lives only in `citations_json`, revision metadata, `media_source_refs`,
or publication-parsed markdown links is not the same product object as an inline
or block source node in canonical Texture body state. It can render as a raw
token, be dropped by a string rewrite, or be flattened during publication even
when the source entity itself exists.

Evidence from the D0 review:

- `internal/store/texture.go:52` and `internal/store/texture.go:836` keep
  `content` as the canonical revision body with `citations_json` beside it.
- `internal/runtime/texture.go:199` and `internal/runtime/texture.go:1112`
  still pass string content through Texture write paths.
- `internal/runtime/tools_texture.go:73`, `internal/runtime/tools_texture.go:576`,
  `internal/runtime/tools_texture.go:672`, and
  `internal/runtime/tools_texture.go:758` expose string patch/rewrite edit
  surfaces instead of validated block/node/source operations.
- `frontend/src/lib/texture-markdown-serializer.ts:10`,
  `frontend/src/lib/texture-source-renderer.ts:486`, and
  `frontend/src/lib/TextureEditor.svelte:1782` preserve a markdown-ish
  editor round trip where source refs can serialize as links.
- `internal/platform/publication_document.go:276`,
  `internal/platform/types.go:218`, and `internal/platform/service.go:220`
  keep publication/export tied to flattened content and parsed source links.
- `internal/runtime/texture_media_sources.go:52` and
  `internal/runtime/texture_agent_revision.go:374` still feed media/source
  ingestion into sidecars.

Conjecture delta: D1/D2 must replace the canonical body substrate, not merely
add a renderer guard or prompt instruction. The first behavior slice should
introduce a Go-validated structured document and source entity validator in an
internal parser/renderer spike, then cut the canonical write path to reject old
source syntaxes at write time.

Rollback path for the later behavior slice: revert the structured-body/write-path
commit(s) to restore the current string body and sidecar behavior. This
checkpoint itself is documentation only.

Heresy delta: discovered. No repair claimed until runtime writes, editor round
trip, agent edits, and publication/export all use document-node source refs.

## D2 Storage/Projection Cut - 2026-06-21

Mutation class: `red` once the following runtime slice lands, because it changes
Texture canonical revision writes and the revision hash substrate.

Exact D2 cut:

- Add `texture_revisions.body_doc_json` and
  `texture_revisions.source_entities_json` as canonical TextureRevision v2
  storage. Fresh DDL and `bootstrapTexture` migration must both add the columns
  so existing Dolt workspaces receive the cut.
- Keep existing `texture_revisions.content` as the derived
  `body_text_projection` for D2. Search, diff, blame, existing frontend reads,
  publication/export, and legacy tests may continue reading `content`, but D2
  must not treat it as canonical source identity.
- Keep `citations_json` only as legacy/historical metadata. D2 structured writes
  must not use citations, markdown links, `{{source:...}}`, prose `Source:`
  lines, unresolved `[1]` markers, or metadata-only source sidecars as canonical
  source identity.
- Store-level `CreateRevision` becomes the first production write boundary:
  every new revision must either provide a structured `body_doc_json` plus
  `source_entities_json`, or be converted from plain text into a simple
  structured document with no source entities. The resulting document is
  validated by `internal/texturedoc` before insertion.
- The public revision API may accept `body_doc` / `source_entities` for D2. When
  those fields are present, the server derives `content` from
  `texturedoc.Project`; callers do not get to submit a conflicting projection.
- The tamper-evident revision hash changes to sign parent hash,
  canonical `body_doc_json`, canonical `source_entities_json`, provenance, and
  the derived projection. This is a new v2 hash payload; D2 should not leave
  structured canonical state outside the signable substrate.

Conjecture delta: cutting the store/API write boundary to typed document nodes is
enough to stop new canonical Texture revisions from accreting link-shaped or
token-shaped source identity, while later D3/D4/D6 cuts can move editor saves,
agent operations, and publication/export off the projection.

Protected surfaces touched by the runtime slice: Texture canonical writes,
revision schema/storage, source entities/provenance, revision hash chain, public
Texture revision API, and any runtime path that calls `Store.CreateRevision`.
Not touched in D2: frontend editor save semantics beyond the existing API
request shape, publication/export rendering, source opening, Comet/browser
acceptance, deployment routing, or broad old-path deletion.

Admissible evidence class for D2 handoff: architectural-level local proof from
focused Go tests showing structured body/source entities persist, project, and
round-trip through the API/store; existing workspaces get migrated columns; the
revision hash changes when structured body/source entities change; and legacy
source syntaxes are rejected at the store/API write boundary. This is not mission
settlement and not staging proof.

Rollback path: revert the D2 runtime commit(s). The added Dolt columns can remain
as inert unused columns during rollback because pre-D2 readers ignore them; if a
full storage rollback is later required, a follow-up migration can drop
`body_doc_json` and `source_entities_json` after preserving any D2 revision
evidence.

Heresy delta: repaired only for new server-side Texture revision writes at the
store/API boundary. Heresy remains discovered/open for frontend editor node
preservation, Texture agent structured operation tools, publication/export, old
historical revisions, source-opening behavior, and staging acceptance.

## D2 Review Blocker Repair Decision - 2026-06-21

Preliminary D2 review found a blocker: the first local D2 cut made
`body_doc_json` and `source_entities_json` canonical for ordinary
`Store.CreateRevision` writes, but live paths could still create new revisions
whose source identity lived only in legacy sidecars. Examples include source gap
repair and source artifact attachment writing `metadata.source_entities` while
leaving top-level `SourceEntities` empty, agent/tool normalization writing
`[label](source:ENTITY_ID)` plus metadata source entities, and public/store calls
persisting non-empty `citations_json` even though the `rev2` hash signs only the
structured substrate.

Repair decision: D2 will **reject/defer** these legacy source-bearing write
paths rather than convert them in this slice. `Store.CreateRevision` remains the
single production boundary and must reject new revisions that carry citation or
source identity in `citations_json`, `metadata.source_entities`,
`metadata.media_source_refs`, `metadata.source_gaps`,
`metadata.source_repair_resolutions`, `metadata.source_attachment_manifest`, or
`metadata.source_ref_normalization`. This rejection is unconditional: even a
revision with valid `BodyDoc` and top-level `SourceEntities` must not also carry
parallel legacy source metadata that can diverge from the signed canonical
structured fields. D2 still allows historical reads of old revisions and
non-source operational metadata.

Mutation/protected-surface note: this repair remains `red` and touches the same
protected surfaces as D2: Texture canonical writes, revision schema/storage,
source entity/provenance records, public Texture revision API, revision hash
chain, and runtime paths that call `Store.CreateRevision`. It intentionally does
not repair frontend/editor saves, Texture agent structured edit operations,
source repair conversion, publication/export, staging/deploy, or old-path
deletion.

Admissible evidence: focused store/runtime tests proving non-empty
`citations_json` and metadata source sidecars are rejected at the write boundary,
including when valid structured `BodyDoc` plus top-level `SourceEntities` are
present; the called-out source repair/attachment APIs return clear
invalid-revision errors; structured `BodyDoc` plus top-level `SourceEntities`
still persists when no legacy sidecar is present; and legacy source text syntaxes
remain rejected.

Rollback path: revert the D2 repair commit to restore the previous local D2
behavior while keeping the prior D2 documentation and implementation commits
available for rework.

Heresy delta: repaired for the newly discovered D2 side-channel blocker only
after tests pass; discovery alone is not counted as repair.

## D2 Accepted Integration - 2026-06-21

Independent D2 re-review returned `accept` after the side-channel repair. Root
integrated the accepted commit range as `d54458b5`, `e60f6523`, and `9d50485f`
and reran the focused evidence on the integrated branch.

Accepted D2 state:

- New Texture revisions validate through `internal/texturedoc` before storage.
- `body_doc_json` and top-level `source_entities_json` are the only new
  canonical source identity fields in D2 scope.
- `content` remains a derived projection for compatibility, not a source
  authority.
- Non-empty `citations_json` and non-empty legacy source metadata sidecars are
  rejected at `Store.CreateRevision`.
- Source repair and source artifact attachment paths map invalid legacy writes
  to clear bad-request responses instead of persisting sidecar-only source
  state.

Evidence on root:

```text
nix develop -c go test ./internal/types ./internal/texturedoc ./internal/store
nix develop -c go test ./internal/runtime -run TestTextureRevisionAPIAcceptsStructuredBodyAndRejectsLegacySourceSyntax
git diff --check HEAD~3..HEAD
```

D2 does not claim mission settlement. The editor, Texture agent operation API,
multimedia resolver, publication/export, old-path deletion, staging deploy, and
Comet/browser product proof remain open cuts.

## D4 Agent Operation Problem Checkpoint - 2026-06-21

Problem: D2 and D3 make structured revisions and editor/user saves viable, but
Texture agent mutation tools still expose canonical editing as string
replacement surfaces. `patch_texture` accepts `find` / `replace` / `append`
operations over `current.Content`; `rewrite_texture` accepts a full plain
`content` replacement. The commit path then materializes plain text, normalizes
wire article source prose into `[label](source:id)` syntax, and attempts to
carry source identity through `metadata.source_entities` /
`source_ref_normalization` sidecars. After D2, those sidecars are invalid for
new canonical source writes, so the D4 path is both semantically stale and
operationally brittle.

Exact D4 cut:

- Replace model-facing canonical Texture mutation with validated
  block/node/source operations over the current structured `BodyDoc`, not raw
  canonical JSON and not ad hoc string replacement.
- Supported operation vocabulary for D4 should be small: update text in a block,
  append/insert a block, delete a block or source node, insert `source_ref`,
  insert `source_embed`, and whole-document recovery rewrite only through a
  server-owned conversion/validation path.
- Runtime, not the model, mints node ids, source entity ids, provenance, and any
  source entities derived from existing typed evidence packets.
- D4 must write `BodyDoc` plus top-level `SourceEntities`; it must not write
  `metadata.source_entities`, `media_source_refs`, `source_ref_normalization`,
  markdown source links, or `[source:id]` tokens as canonical source identity.
- The existing `patch_texture` / `rewrite_texture` names may remain as aliases
  only if their schemas no longer permit legacy string/source-sidecar canonical
  writes. Otherwise add new explicit structured tools and remove the old write
  tools from Texture agent capability.

Conjecture delta: D4 tests whether the same canonical structured substrate used
by server writes and the frontend editor can also be the agent editing
substrate. If not, the architecture still has a split brain: human edits
preserve transclusions while agent edits flatten them.

Protected surfaces: Texture agent tools/prompts, `commitTextureToolEdit`,
appagent revision metadata/provenance, source entity carry-forward, mutation
state completion, wire article source handling, and all runtime paths that
create appagent-authored Texture revisions.

Admissible evidence class: focused runtime tests proving structured tool edits
preserve existing `source_ref` nodes, can explicitly delete a source node, can
insert source refs/embeds backed by top-level source entities, reject legacy
source link/token/string sidecar attempts, and keep stale-base protection. D4
does not require staging proof until the later landing/proof cut.

Rollback path: revert D4 runtime/tool commits to return to the D3 accepted
state. The D2 store guard remains the safety floor and will continue rejecting
legacy sidecar writes.

Heresy delta: discovered: appagent write tools still operate on string content
and source metadata sidecars. introduced: none by this documentation checkpoint.
repaired: not yet; D4 implementation must repair it.

## D4 Accepted Integration - 2026-06-21

Independent D4 re-review returned `accept` after the Universal Wire structured
source manifest repair. Root integrated the accepted D4 commits as `cdd7232c`
and `cb690d8c`, then reran focused runtime/store evidence and the normal runtime
shard script on the integrated branch.

Accepted D4 state:

- `patch_texture` now exposes validated structured operations over `BodyDoc`
  rather than model-authored find/replace/append string patches.
- `rewrite_texture` remains only as a recovery path whose plain prose is
  converted and validated by the server into a structured document.
- Appagent-authored Texture revisions write `BodyDoc` plus top-level
  `SourceEntities`, clear legacy source/citation sidecars, and derive projection
  text through the structured document projector.
- Structured operations can update blocks, append/insert blocks, delete nodes,
  preserve or explicitly remove source nodes, and insert `source_ref` /
  `source_embed` nodes backed by top-level source entities.
- Universal Wire source-network manifests read visible structured source refs
  from `BodyDoc` / `SourceEntities` before article inventory sections; legacy
  metadata source reads remain only as historical fallback for old revisions.

Evidence on root:

```text
nix develop -c go test ./internal/runtime -run 'Texture.*(Structured|Tool|Agent|Source)|TestHandleUniversalWireStoriesUsesVisibleSourceEntitiesForSourceNetworkManifest'
nix develop -c go test ./internal/texturedoc ./internal/store
nix develop -c scripts/go-test-runtime-shards
git diff --check HEAD~2..HEAD
```

D4 does not claim mission settlement. Multimedia source rendering/resolution,
publication/export, broad old-path deletion, deployment, and Comet/browser
product proof remain open cuts.

## D5 Multimedia Path Problem Checkpoint - 2026-06-21

Mutation class: `green` documentation checkpoint only. No runtime behavior,
frontend rendering, schema, source resolver, publication, deployment, or product
path changed in this checkpoint.

Reliable D5 inspection shows the next behavior problem:

> Multimedia source identity still has a sidecar and renderer-only path:
> live Texture can discover image/YouTube URLs from projection text into
> `metadata.media_source_refs`, derive source entities from that sidecar, and
> render special media HTML by kind rather than proving image/video/audio/PDF/
> transcript/Texture-span targets through the structured `source_ref` /
> `source_embed` plus top-level `SourceEntity` contract.

Evidence:

- `internal/runtime/texture_agent_revision.go:375` calls
  `registerTextureMediaSourceRefs` on `currentRevision.Content`, and
  `internal/runtime/texture_agent_revision.go:377` writes
  `metadata.media_source_refs`.
- `internal/runtime/texture_media_sources.go:52` discovers media URLs from text
  and `internal/runtime/texture_media_sources.go:199` merges the sidecar into
  legacy source entities.
- `internal/runtime/texture_agent_revision.go:606` still prompts Texture agents
  with "Detected durable media source refs" before ordinary source entities.
- `frontend/src/lib/texture-source-state.ts:9` reads
  `revision.metadata.media_source_refs`, and
  `frontend/src/lib/texture-source-state.ts:25` synthesizes source entities from
  those refs when top-level structured entities are missing.
- `frontend/src/lib/texture-source-renderer.ts:286` converts media refs to
  source entities client-side, while
  `frontend/src/lib/texture-source-renderer.ts:321` renders YouTube/image media
  through kind-specific helper HTML.
- `internal/texturedoc/schema.go:393` accepts media display modes such as
  `player`, `image_preview`, `pdf_pages`, and `transcript`, but
  `internal/texturedoc/schema.go:403` only validates source/web-lens/Texture/
  video/image open surfaces. Natural audio/PDF surfaces are not a complete
  structured contract yet.

Exact D5 cut:

- Stop creating new canonical multimedia identity in
  `metadata.media_source_refs`. New media discoveries must become top-level
  structured `SourceEntity` records referenced by `source_ref` or `source_embed`
  nodes, or remain noncanonical suggestions until attached through a structured
  operation.
- Preserve historical `media_source_refs` read fallback only for legacy
  revisions; do not use it for new D5 writes or appagent prompts.
- Add or normalize source-contract open surfaces needed by multimedia targets
  (`audio`, `pdf`, and any required transcript/file/source-window mapping), then
  validate them through `internal/texturedoc`.
- Render image, video, audio, PDF, transcript, and Texture-span transclusions
  from top-level `SourceEntity.target` / `selectors` / `display` plus document
  `source_ref` or `source_embed` nodes. Rendering may still use specialized
  players/previews, but those players must be node-view/display implementations
  of the one structured source model, not parallel body syntaxes or sidecars.

Protected surfaces for the later runtime/frontend slice: deterministic media
ingestion, Texture agent prompt context, source entity normalization, structured
document/source validation, frontend source-state derivation, frontend source
renderer/media transclusion, and any tests that currently expect
`media_source_refs` as the live source carrier.

Admissible evidence class: focused Go tests proving new media discoveries do
not write `metadata.media_source_refs`, multimedia source entities validate for
image/video/audio/PDF/transcript/Texture-span targets and display modes, and
appagent prompt context reads structured top-level entities. Focused frontend
tests should prove structured source entities render image/video/audio/PDF/
transcript/Texture-span transclusions without client-side `media_source_refs`
synthesis or clickable source links. D5 is not settlement and does not require
publication/export or staging proof until later cuts.

Rollback path: revert the D5 runtime/frontend commits to return to the accepted
D4 state. D2 store guards still reject legacy source sidecars on new canonical
revisions, and D4 appagent write sanitization still strips media/source sidecars
from appagent-authored revisions.

Heresy delta: discovered: multimedia still has a sidecar discovery/prompt/render
path outside structured source nodes. introduced: none by this documentation
checkpoint. repaired: not yet; D5 implementation must repair it.

## Editing And Citation Integrity

Offsets are implementation details, not canonical citation identity. Structured
editor transactions may use positions internally, but durable references are
node ids plus source entity ids. A source point is a node in the paragraph, not a
number stored beside the paragraph.

Rules:

- Human editing moves `source_ref` atoms with surrounding text.
- Human deletion of a source point is an explicit source deletion event, not a
  silent offset drift.
- Agent paragraph rewrites must preserve existing refs with explicit markers
  or declare intentional removal.
- Whole-document rewrites are exceptional and must run through a source-ref
  preservation validator.
- Publication/export derives source lists from document nodes plus
  `source_entities`; it must not scrape markdown links or prose labels.

## Multimedia And Transclusion Requirements

The cutover must support at least these target classes in the same source entity
model:

- web/document snapshot with exact text selectors;
- image source with thumbnail, dimensions, alt/display metadata, and original
  open surface;
- video source with player display, transcript availability, timestamp ranges,
  and clip selectors;
- audio/podcast source with episode metadata, transcript ranges, and clip
  selectors;
- PDF source with page/range selectors and extracted text selectors when
  available;
- Texture/document span source, including private Texture revisions and
  published Texture/publication spans;
- source-service item or future Source Viewer/reader artifact with stable
  resolver identity.

Existing YouTube/image detection and frontend media rendering are useful
implementation evidence, but they are not canonical as-is. They should collapse
into source entities plus structured document nodes.

## Via Negativa

Delete or demote these as canonical mechanisms:

- markdown links as citations or source proof;
- prose "Source:" handles as source proof;
- raw `{{source:...}}` body tokens rendered to users;
- numbered citation offsets stored outside the body tree;
- regex scraping of body/prose to discover source ids;
- separate canonical syntaxes for image, video, web, and Texture references;
- model-authored provenance or source identity;
- source lists that do not correspond to document nodes;
- renderer-only media cards that are not represented in canonical Texture state.

## Domain Ramp

- **D0 Documentation and schema decision.** Settle the structured document
  schema, source node vocabulary, media target vocabulary, and edit API. Record
  deletion targets for old syntaxes.
- **D1 Internal parser/renderer spike.** Convert a small in-memory Texture doc
  with paragraphs, numbered `source_ref`, `source_embed`, YouTube/image/web
  source entities, and source panel rendering. No production write path yet.
- **D2 Canonical revision write path.** Store the structured body as the only
  canonical body for new Texture revisions. Reject raw source tokens and
  markdown-link citation attempts at write time.
- **D3 Editor/user path.** Human editing preserves source refs as atom nodes,
  supports inserting/removing source refs intentionally, and renders numbered
  source points with expandable transclusion.
- **D4 Agent API path.** Texture agent tools perform block edits, source attach,
  source ref insertion, source embed insertion, and source-preserving rewrites.
  Prompts describe obligations and tools, not JSON authoring.
- **D5 Multimedia path.** Image/video/audio/PDF/transcript/Texture-span source
  entities render as inline refs, block embeds, source windows, and source panel
  entries from the same resolver.
- **D6 Publication/export path.** Published Texture exports structured body,
  source entities, transclusions, provenance, and version chain without
  flattening sources into clickable links or markdown source lists.
- **D7 Deletion and proof.** Delete old canonical source syntaxes and tests,
  update docs/checkers, deploy, and prove staging source creation, editing,
  multimedia expansion, publication/export, and accidental source-deletion
  rejection.

## Parallax State

status: working

mission conjecture: If Texture hard-cuts to a ProseMirror/Tiptap-style
structured document with source/media/transclusion nodes and a validated
operation API under the invariants below, then Texture becomes a trustworthy
versioned, transclusive artifact core rather than a markdown document with
fragile sidecars.

deeper goal (G): Texture is Choir's canonical artifact control plane: a document
where writing, research, source identity, multimedia evidence, publication, and
agent collaboration survive revision instead of flattening into prose.

witness/spec (A/S): A deployed Texture v2 body schema, source entity schema,
renderer/editor, agent edit API, multimedia resolver, publication projection,
tests, docs, and deletion receipts that make structured source/transclusion
nodes the only canonical source mechanism.

invariants / qualities / domain ramp (I/Q/D): no clickable-link source contract;
no model-authored canonical JSON; no offset-only durable citations; source refs
are document nodes; source entities are runtime-minted and system-provenanced;
multimedia uses the same source entity model; Texture remains agentic, not a
forced workflow; staging proof is required for behavior settlement. Domain ramp
D0-D7 above.

variant (ranking function) V: 10 open obligations: schema decision, source
entity target vocabulary, edit API, write path, human editor path, agent path,
multimedia resolver, publication/export projection, old-syntax deletion,
staging acceptance proof. Current value: 0 for the structured source/
publication/deploy hard-cut obligations proven in this mission pass. Accepted
D7 slices have removed the
current-contract markdown source-link affordance band, Universal Wire/coagent
source-link normalization, markdown-lineage/source-repair metadata residue,
dead source-repair/source-attachment frontend affordances, publication fallback
markdown source-link upgrading, runtime use of `metadata.source_entities` as
live source context, prompt-contract residue that taught old `patch_texture`
replace/append JSON, the dead legacy text edit helper, and the media-source-ref
fallback path.
The landing loop found and repaired a deploy-source allow-list failure: the Nix
per-service source filter excluded the new `internal/texturedoc` package from
services that import it, so staging could not build the pushed repair. Staging
deploy and deployed product-path Texture source/transclusion proof now pass.
Comet-specific visual proof is blocked: the user's Comet session renders a blank
viewport for both `https://choir.news` and a proven public Texture route, while
Playwright and public HTTP health render/validate the route.
D5 multimedia reduced the variant locally, but its independent review agents
stalled; retain that caveat until staging proof or a later reviewer checks the
accumulated multimedia diff.

budget: Planning budget is one paradoc pass. D1-D4 used bounded passes for
schema/source-contract, store/API write boundary, editor/user preservation, and
agent operations. D5 used a bounded multimedia resolver/rendering cut with
focused tests, but independent review stalled. D6 used a bounded
publication/export cut after a Problem Documentation First checkpoint, repaired
two P1 review findings, and obtained re-review acceptance. Solvency verdict:
proceed to D7 deletion receipts and acceptance-proof preparation as a fresh
pass; do not deploy or claim settlement until old-path deletion, broad tests,
CI/deploy identity, and staging product proof are recorded.

authority / bounds: D1 was authorized as an additive internal schema/parser/
renderer spike only. It does not authorize production Texture write behavior,
database schema, agent tools, frontend editor saves, publication export,
staging/deploy, or old-path deletion. D2 preserved Problem Documentation First
and named the exact write-path cut shape before runtime mutation.

mutation class / protected surfaces: D2 and D3 are red because they touch
Texture canonical writes. Protected surfaces touched by D2 are Texture revision
schema/storage, source entity/provenance records, public Texture revision API,
revision hash chain, and runtime paths that call `Store.CreateRevision`.
Protected surfaces touched by D3 are the frontend Texture editor save/load path,
structured source-ref rendering, and the revision API request contract for
`body_doc` / top-level `source_entities`. Protected surfaces touched by D4 are
Texture agent tool schemas/prompts, `commitTextureToolEdit`, appagent revision
metadata/provenance, source entity carry-forward, mutation completion, and
Universal Wire source manifest derivation. Protected surfaces explicitly left
for later cuts remain Source Viewer/reader integration beyond existing
open-source affordances, deployment routing, and run acceptance involving
Texture. D5 will be red/orange
depending on final slice because it touches deterministic media ingestion,
Texture agent prompt context, structured source validation, and frontend media
transclusion rendering. D6 is red/orange because it touches publication/export
input, artifact manifests, source metadata normalization, version history,
wire/proxy publish request contracts, and publication document rendering.

evidence packet: D0 design receipts plus a clear-context Codex thread review
verdict; D1 focused Go tests for schema/source entity validation, including
`image_region`, source_embed leaf enforcement, numbered source-ref/source-embed
projection, old-syntax rejection, source resolution, and multimedia target
entities; D2 focused store/API tests for structured revision persistence,
existing-workspace column migration, projection derivation, hash inclusion of
`body_doc_json` and `source_entities_json`, rejection of old source syntaxes, and
rejection of citation/metadata source sidecars at the write boundary. D2 repair
evidence passed on root after independent acceptance. D3 evidence passed after
independent acceptance and root integration: structured source-ref rendering,
editor save-serialization preservation, top-level source entity filtering, no
clickable source-link serialization, frontend build, and the D2 runtime API
regression. D4 evidence passed after independent acceptance and root integration:
structured Texture agent operations, source-ref preservation/deletion/insertion,
legacy source syntax rejection, stale-base rejection, no legacy metadata/citation
sidecar appagent writes, structured Universal Wire visible-source manifest
derivation, runtime shards, and `git diff --check`. D5 evidence proves locally
that new media discoveries do not write `metadata.media_source_refs`,
multimedia structured source entities validate and render from top-level
`SourceEntities` plus `source_ref` / `source_embed` nodes, and no clickable
source links or client-side media-sidecar synthesis are required for new
revisions. D6 evidence passed after independent review repair: structured
platform publication/export tests, explicit-empty source entity suppression of
legacy metadata, publication/wirepublish package tests, proxy/runtime focused
publish bridge tests, and `git diff --check`. D7 evidence now includes accepted
prompt/frontend/current-fixture deletion, Universal Wire/coagent normalizer
deletion, and markdown-lineage/source-repair retirement with focused runtime and
tagged processor tests. D7 fourth evidence also removes the dead
markdown-link citation validator, source-gap repair frontend affordance,
source-review payload helper, and stale frontend source-attachment affordance;
browser fixtures now assert that the retired controls are absent while structured
refs still expand, with independent review accepted. Later cuts still need
independent review of the local publication/proxy fixture/source enrichment
slice, classification of the remaining runtime historical/prompt legacy refs,
CI, staging deploy identity, Comet/browser staging proof with numbered source
refs, expanded source window, multimedia source expansion, agent edit preserving
refs, publication/export structured source proof on staging, and attempted
markdown link/source token rejection; RunAcceptanceRecord at staging-smoke-level
or higher if platform behavior changes. Landing evidence for pushed commit
`5c057afc63adf6df8db9a881c92942586c70fa51` currently includes GitHub CI run
`27922543313`, whose only failed job is `Deploy to Staging (Node B)` because
Nix package builds could not resolve
`github.com/yusefmosiah/go-choir/internal/texturedoc` under the filtered source
archive. The local repair evidence for the follow-up fix is filtered-source
inspection of the `packages.x86_64-linux` service source outputs for proxy,
corpusd, sandbox, gateway, and sourcecycled plus `nix flake check --no-build`;
full x86_64 package compilation is deferred to CI because this Mac has no
available `x86_64-linux` builder. CI run `27922834186` completed successfully
for `d77e0457806d6a9de27657b7ffb5f8f3a7862922`; the deploy job built selected
Nix services and guest images, restarted services, refreshed active computers,
and health checks reported proxy/sandbox/corpusd deployed commit
`d77e0457806d6a9de27657b7ffb5f8f3a7862922`. Deployed acceptance used
`https://choir.news` with a virtual passkey account and passed publication,
source expansion, multimedia/no-clickable-source-link, edit-preservation, and
legacy markdown source-link rejection probes.

heresy delta: discovered: Texture currently permits or preserves multiple
source-shaped syntaxes that are not canonical transclusions. introduced: none
by this paradoc. repaired by D2 for new server-side Texture revision writes at
the store/API boundary after side-channel rejection, focused tests, independent
review, and root integration. D3 repairs the owner/editor-authored preservation
path after focused tests, independent review, and root integration. D4 repairs
the appagent-authored Texture mutation path after focused tests, independent
review, and root integration. D5 repairs the newly discovered multimedia
sidecar/rendering path locally: new deterministic media discovery surfaces
Texture source entities, not `metadata.media_source_refs`; structured frontend
revisions no longer synthesize media sidecars; audio/PDF/video/image rendering
uses source entities without clickable source links. Independent D5 review was
attempted twice but both review agents stalled and were interrupted; D5 is
therefore an implementation checkpoint, not an independently accepted
settlement. D6 repairs the publication/export projection path after focused
tests, a P1 independent review, repair, and re-review acceptance. D7 repairs the
prompt/frontend/current-fixture source-link affordance band, the Universal
Wire/coagent source-normalizer path, and the markdown-lineage/source-repair
metadata path after focused tests and independent review. Full repair still
requires independent review of the local publication/proxy source fixture
cleanup, remaining runtime historical/prompt legacy classification, and staging
proof.

position / live conjectures / open edges: C1 supported for D1/D2: use a
Choir-owned ProseMirror-compatible typed document schema validated in Go; do not
make Tiptap canonical. Tiptap remains optional for the later Svelte editor layer.
C2 supported: one `source_ref` / `source_embed` body vocabulary plus typed
`SourceEntity` targets is simpler than per-medium body syntaxes. C3 supported:
model-facing operations must be block/node/source commands, not document JSON.
D1 implemented the first in-memory witness in `internal/texturedoc`: supported
node/mark vocabulary, strict source entity enum validation, legacy source text
syntax rejection, source-node/entity resolution, and projection with numbered
refs/source embeds. D1 review found P1 missing `image_region`; the source
contract enum and texturedoc validator now include it. D1 re-review found P1
missing `source_embed` leaf enforcement; source_embed now rejects hidden content,
text, and marks. Final D1 re-review accepted the additive witness. D2 wired the
first production write boundary and the accepted repair rejects/defer legacy
source-bearing citation/metadata sidecars unconditionally at
`Store.CreateRevision`. D3 wires the frontend editor/user path to render
structured `body_doc` source refs as native atom spans, serialize those atoms
back to `source_ref` nodes, and send top-level `source_entities` instead of
source metadata sidecars. D4 wires Texture agent tools to structured
block/node/source operations and appagent revisions now carry top-level
structured source entities instead of metadata sidecars. E1: human insertion
affordance for new source refs is still thinner than preservation/removal
coverage; D3 proves preservation/removal through editor atom round-trip. E2:
multimedia source entities now have local D5 proof that
image/video/audio/PDF/transcript/file open surfaces
validate through the shared source contract and that image/video/audio/PDF
rendering is driven by source entities rather than media sidecars. Texture-span
publication/export proof remains open. E3: publication/export/diff/search
now has local/package D6 support for the publication/export path: publication
accepts structured `body_doc` and top-level `source_entities`, derives flattened
content from structured projection only as a projection, stores structured fields
in artifact manifests and version history, normalizes publication source
metadata from top-level entities before metadata fallback, carries structured
fields through wire/proxy publish requests, and renders publication/export
source refs from structured nodes. D7 first removed prompt/frontend/current-test
affordances that taught clickable source-link syntax. D7 second deleted
Universal Wire source-link/source-token normalization and stopped coagent seed
Texture revisions from carrying legacy `metadata.source_entities`; coagent
source context now enters the Texture revision run through internal request/run
context until native `patch_texture` source operations attach top-level
structured `source_entities`. D7 third converted markdown-lineage import into a
historical conversion path that stores structured `body_doc`/top-level
`source_entities`, rejects unresolved markers, and retires legacy source repair
and source attachment metadata endpoints with 410 responses. D7 fourth
deletes the dead markdown-link citation validator, removes frontend source-gap
repair candidate scanning and source-review panel/actions, deletes the
source-review payload helper, removes stale frontend source-attachment controls
that could only call the retired endpoint, teaches the frontend renderer to read
structured `text_quote` selector `exact` data, and updates browser fixtures to
assert import-time unresolved-marker rejection plus absence of the retired repair
and attachment UI. D7 fifth locally removes publication fallback support for
turning markdown `source:` links into native publication source refs, rejects
`metadata.source_entities` at platform publication input, enriches proxy/wire
publication from top-level structured source entities instead of metadata
sidecars, preserves reader snapshot/status fields in structured source entities,
and converts publication/proxy/browser publication fixtures to structured source
refs. The landing loop discovered and repaired that `flake.nix` service source
filters still omitted `internal/texturedoc`, so deployed Nix builds failed
before staging identity could advance. Staging deploy identity and deployed
product proof for structured source transclusion now pass; Comet visual proof
remains a separate browser-specific blocker because Comet displays a blank
viewport on the proven route. D5 checkpoint
records the specific sidecars/pathways repaired locally: runtime
`media_source_refs`, prompt context that prefers those refs, and frontend
media-ref synthesis/rendering outside top-level structured entities. D5
implementation removes the new-write/new-prompt path for those sidecars and
keeps legacy media-ref synthesis only for revisions without structured
`body_doc`. Markdown `[label](source:id)` parsing remains only as historical
fallback for artifacts without structured body data.

next move: incorporate independent review of the deploy-source repair and
landing evidence. If accepted, close this mission as settled for deployed
structured Texture source/transclusion; carry the Comet blank-viewport issue as
a separate browser-specific acceptance blocker unless the owner requires Comet
visual proof as part of this mission's settlement.

ledger file: docs/mission-texture-structured-document-transclusion-cutover-v0.ledger.md

version / lineage: v0. Created 2026-06-21 as a successor/specialization of
`mission-texture-hard-cutover-v0`, `mission-texture-versioned-artifact-v0`, and
`mission-vtext-source-entities-multimedia-transclusion-v0`. <!-- texture-cutover-allow: historical mission evidence path; deletion receipt: texture-hard-cutover-v0 -->

learning state: D0 schema/API decision retained here. D1 code witness now lives
in `internal/texturedoc`; D2-D4 prove the structured schema across new revision
writes, editor/user preservation, and appagent mutation. D5 locally proves the
multimedia resolver/rendering cut, with independent review still unavailable due
stalled review agents. D6 proves publication/export at local/package scope after
independent review repair. D7 has accepted deletion receipts for
prompt/frontend current-contract fixtures, Universal Wire/coagent
source-normalization residue, markdown-lineage/source-repair metadata residue,
frontend repair/attachment-affordance deletion, publication/proxy source
fixture cleanup, and runtime source-context key separation. Promote outward only
when remaining prompt-contract deletion receipts, staging proof, and any missing
multimedia independent review close the product contract.

settlement: Not met. Settlement requires deployed staging proof that structured
source/transclusion nodes are the only canonical source path, numbered refs
expand through Texture source entities, image/video/audio/PDF/transcript/Texture
sources render through the same resolver, human and agent edits preserve or
explicitly delete source refs, publication/export carries structured
transclusions, and old markdown/source-token/offset sidecar paths are deleted or
classified as noncanonical historical import only.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-texture-structured-document-transclusion-cutover-v0.md. D1-D4 are integrated and accepted; D5 multimedia sidecar/rendering implementation is locally tested but independent review stalled; D6 publication/export is implemented and independently accepted after P1 repairs; D7 prompt/renderer/current-fixture deletion, Universal Wire/coagent source-normalizer deletion, markdown-lineage/source-repair retirement, and citation-helper/frontend repair-affordance deletion are independently accepted; current V=1. Local D7 publication/proxy source fixture cleanup awaits independent review. If accepted, classify remaining runtime historical/prompt legacy refs, then run broad tests and staging proof. Do not deploy or claim mission settlement unless the paradoc is updated first.
```

## D6 Publication/Export Problem Checkpoint - 2026-06-21

Problem: The publication/export path still treats flattened markdown-ish
projection content as the source of publication structure and source refs. In
particular, `publicationDocumentBlocks` parses `bundle.Artifact.Content` and
`parsePublicationInlines` reconstructs `source_ref` inlines from
`[label](source:id)` links. `buildPublicationSourceMetadata` reads
`metadata.source_entities` rather than top-level revision `SourceEntities`, and
`PublicationVersionHistoryEntry` carries flattened `content`, `citations`, and
`metadata` but no `body_doc` / top-level `source_entities` fields. Tests still
publish source refs by embedding `[...](source:...)` in content plus
`metadata.source_entities`.

Evidence:

- `internal/platform/publication_document.go:65` builds document blocks from
  `bundle.Artifact.Content`.
- `internal/platform/publication_document.go:276` parses markdown links into
  publication inlines.
- `internal/platform/publication_document.go:293` treats `source:` markdown
  hrefs as publication `source_ref` inlines.
- `internal/platform/source_metadata.go:94` reads `metadata.source_entities`.
- `internal/platform/types.go:218` records version history entries as flattened
  `Content`, `Citations`, and `Metadata`.
- `internal/platform/service_test.go:1210` publishes a fixture containing
  `[Federal Reserve rate statement](source:src-entity-fed-rates)`.

Invariant to repair: publication/export must consume structured Texture body
documents and top-level source entities as canonical input. Markdown projection
may remain an export format, but it must not be the source identity parser for
new publications. Published source refs, source manifests, DOCX/HTML/PDF exports,
and version history should preserve or derive from structured `body_doc`
`source_ref` / `source_embed` nodes plus top-level `source_entities`, not
markdown links, citations sidecars, or metadata source sidecars.

Cut target: extend platform publication input/types to accept and persist
structured `body_doc` and top-level `source_entities`; derive publication
source metadata from top-level entities first; render publication documents from
structured nodes when present; carry structured fields through version history
and wire/proxy publish paths. Keep `parsePublicationInlines` source-link parsing
only as a historical fallback for artifacts without structured body data.

## D7 Old-Syntax Deletion/Proof Problem Checkpoint - 2026-06-21

Problem: D1-D6 changed the canonical substrate for new Texture revisions, agent
operations, editor preservation, multimedia source entities, and
publication/export, but old source syntax still has too many live affordances to
call the cutover complete. The remaining residue is not one file; it is a set of
families that can still teach, parse, render, normalize, or test clickable
source links and sidecar source identity as if they were Texture's native
contract.

Residue families found by the D7 map:

- Model/prompt text still instructs agents to write or preserve
  `[label](source:ENTITY_ID)` as canonical inline source syntax:
  `internal/runtime/textureprompts/overlays/run_system.yaml`,
  `internal/runtime/textureprompts/overlays/revision_source_entities_intro.yaml`,
  and `internal/runtime/tools_coagent.go`.
- Legacy normalizers and validators still interpret markdown source links or
  bare `[source:id]` markers as the source grammar:
  `internal/runtime/texture_legacy_wire_normalization.go`,
  `internal/runtime/texture_citation_validation.go`,
  `internal/runtime/texture_lineage.go`, and
  `internal/runtime/universal_wire.go`.
- Frontend rendering still upgrades raw source-link text into transclusion spans
  for any rendered markdown-ish content:
  `frontend/src/lib/texture-source-renderer.ts` and
  `frontend/src/lib/texture-markdown-serializer.ts`.
- Legacy media/source entity sidecars remain as fallback/read paths:
  `frontend/src/lib/texture-source-state.ts`,
  `internal/runtime/texture_media_sources.go`, and
  `internal/platform/source_metadata.go`.
- Publication/proxy/frontend tests still seed or assert `[...](source:...)` plus
  `metadata.source_entities`, so the old contract remains executable in
  fixtures even where new structured tests exist:
  `internal/platform/service_test.go`,
  `internal/proxy/platform_publish_test.go`,
  `frontend/tests/texture-markdown-lineage.spec.js`,
  `frontend/tests/texture-source-entities.spec.js`,
  `frontend/tests/texture-source-service-publication.spec.js`, and
  `frontend/tests/texture-source-ref-live-agent.spec.js`.
- `citations_json` remains a stored Texture column and publication proposal
  field. D2 already rejects it as canonical Texture source identity on new
  Texture revisions, but D7 must classify remaining publication/proposal uses as
  either non-Texture external citation data or deletion targets.

Invariant to repair: no new Texture canonical write, prompt, renderer, or
acceptance fixture may represent native source identity as a clickable markdown
link, raw `source:` token, `{{source:...}}`, metadata source sidecar, media
source sidecar, or offset-only citation record. Historical import/export
fallbacks may remain only when narrowly named, unreachable from new canonical
writes, and covered by tests proving they do not teach or reintroduce the old
contract.

Cut target: delete prompt language that asks the model to author source-link
syntax; remove frontend source-link upgrade for structured revisions; confine
legacy publication/renderer parsing to artifacts without `body_doc`; convert
tests that are meant to prove the current Texture contract to structured
`body_doc` plus top-level `source_entities`; retain only explicitly labelled
legacy fallback tests; and prepare the staging acceptance proof for source
creation, edit preservation/deletion, multimedia expansion, publication/export,
and source-link/token rejection.

### D7 Slice 1 Accepted - Prompt/Renderer/Fixure Deletion

Status: locally implemented and independently accepted on 2026-06-21.

This slice deleted the highest-risk model/user-facing affordances for clickable
source-link syntax without claiming the full D7 residue is gone:

- Texture prompt overlays and coagent Texture routing now instruct agents to use
  structured `patch_texture` source operations and `source_entity_id` handles,
  not `[label](source:...)` links or raw source-token prose.
- Agent revision preservation requirements no longer tell the model to preserve
  a markdown source link exactly; they name the durable `source_entity_id` and
  require a structured `source_ref`/`source_embed`.
- Frontend markdown rendering no longer upgrades raw `[label](source:id)` or
  `[source:id]` text into native Texture source refs. Native structured
  `source_ref` nodes still render through the structured document renderer.
- The unused `texture-markdown-serializer.ts` helper was deleted after review
  found no live imports.
- Current-contract frontend source/entity fixtures were converted from
  markdown-only `content` plus `metadata.source_entities` to structured
  `body_doc` with `source_ref` atoms plus top-level structured
  `source_entities`. The only remaining raw source-link fixture in that spec is
  an explicit regression proving raw source links do not become native refs.

Independent re-review verdict: `accept`. Evidence included `git diff --check`,
focused runtime prompt/source-entity tests, frontend build, and the
browser-independent `texture-source-entities` subset. Full authenticated local
browser proof remained blocked by local auth service lifetime
(`127.0.0.1:8081` stopped listening after `start-services.sh`), so staging/Comet
proof remains required before mission settlement.

Residual D7 deletion targets remain live and must stay classified as
legacy/residue until repaired or explicitly confined:
`internal/runtime/texture_legacy_wire_normalization.go`,
`internal/runtime/texture_lineage.go`, `internal/runtime/universal_wire.go`, and
`internal/runtime/texture_citation_validation.go`.

### D7 Slice 2 Accepted - Universal Wire/Coagent Source Normalizer Deletion

Status: locally implemented and independently accepted on 2026-06-21.

This slice removed the runtime path that minted or discovered Universal Wire
article source identity from markdown/source-token prose:

- `texture_legacy_wire_normalization.go` was deleted, removing the helpers that
  rewrote bare `[source:id]` tokens or `Source Service item ...` prose into
  `[label](source:id)` links.
- Universal Wire visible-source manifest derivation now reads structured
  `BodyDoc` source nodes plus top-level `SourceEntities`; it no longer falls
  back to `metadata.source_entities` plus inline markdown/source-token scanning.
- Universal Wire read normalization is now a no-op for source syntax. It does
  not repair reader-facing source refs from old prose.
- Coagent Texture seed revisions no longer write legacy
  `metadata.source_entities`. Source context for processor/reconciler handoff
  remains available to the Texture run through internal request/run metadata, and
  the first article revision must attach sources through structured
  `patch_texture` `insert_source_ref` / `insert_source_embed` operations.
- The processor/reconciler test now proves seed revisions do not carry
  `metadata.source_entities`, the Texture run still has source context, the
  article revision stores top-level structured `SourceEntities`, and neither
  article content nor metadata retains `source_ref_normalization`.

Independent review verdict: `accept`. Evidence:
`git diff --check`;
`nix develop -c go test ./internal/runtime -run 'TestNormalizeWireArticleRevisionForReadDoesNotMintSourceLinks|TestUniversalWire|TestHandleUniversalWireStoriesUsesVisibleSourceEntitiesForSourceNetworkManifest|TestSystemPrompt|TestTexturePromptPreservesInlineSourceRefs' -count=1`;
`nix develop -c go test -tags comprehensive ./internal/runtime -run '^TestProcessorAndReconcilerProfilesDelegateToTextureOnly$|^TestNormalizeWireArticleRevisionForReadDoesNotMintSourceLinks$' -count=1 -v`;
`nix develop -c go test ./internal/runtime -run 'TestTextureAgentRevision|TestTexturePatch|TestTexturePromptPreservesInlineSourceRefs|TestTextureAgentRevisionRegistersMediaSourceEntities|TestTextureAgentRevisionPromotesResearcherContentRefsToSourceEntities' -count=1`.

Residual D7 deletion targets remain: citation validation helpers that still
parse `[label](source:id)` and historical publication/proxy/frontend fixtures
that are not yet classified as legacy-only.

### D7 Slice 3 Accepted - Markdown Lineage/Source Repair Retirement

Status: locally implemented and independently accepted on 2026-06-21.

This slice converted markdown-lineage import from an old-syntax writer into a
historical conversion path and retired the metadata-sidecar repair endpoints:

- Markdown-lineage import now builds a structured `BodyDoc` containing native
  `source_ref` nodes when it sees a resolved markdown marker or historical
  `[label](source:id)` import input.
- Imported source entities are stored as top-level structured `SourceEntities`
  and filtered to entities actually referenced by `source_ref` nodes.
- Unresolved markdown citation markers now reject the import unless they are
  explicitly resolved to a source entity or marked `no_source_needed`.
- `no_source_needed` removes the marker at import time without leaving
  `source_gaps` or source sidecar metadata.
- Legacy source-gap repair and source-artifact attachment endpoints now return
  `410 Gone`; they cannot create metadata-only source revisions.

Independent re-review verdict: `accept` after repairing two test findings.
Evidence:
`git diff --check`;
`nix develop -c go test -tags comprehensive ./internal/runtime -run 'MarkdownLineage|SourceGapRepair|SourceArtifactAttachment' -count=1`;
`nix develop -c go test ./internal/runtime -run 'TestTextureStructured|TestTexturePatch|TestTextureAgentRevision|TestTexturePromptPreservesInlineSourceRefs|TestNormalizeWireArticleRevisionForReadDoesNotMintSourceLinks' -count=1`.

### D7 Slice 4 Local - Citation Helper/Repair UI Deletion

Status: locally implemented and independently accepted on 2026-06-21.

This slice removes remaining noncanonical source-repair/citation affordances
that were no longer part of the structured write path:

- Deleted the dead markdown-link citation validator and its tests; production
  code no longer had callers, and the helper described `[label](source:id)` as
  native citation syntax.
- Changed diagnosis source-marker counting to prefer structured
  `source_ref`/`source_embed` nodes and fall back only to projected numbered
  refs.
- Removed frontend source-gap candidate scanning, source-review payload
  construction, source-repair API calls, source-review panel/actions, and stale
  source-attachment controls/client helpers that could only call the retired
  endpoint.
- Taught the frontend source renderer to display structured `text_quote`
  selector `exact` data in inline transclusions.
- Updated markdown-lineage browser fixtures so unresolved markers are rejected
  at import time, resolved imports assert projected `[1]` plus top-level
  `source_entities`, and the old repair UI is asserted absent. Updated the
  source-entity browser fixture to assert the retired attachment UI is absent
  while native structured source refs still expand.

Local evidence:
`npm run build` from `frontend/`;
`git diff --check`;
`nix develop -c go test ./internal/runtime -run 'TestTextureDiagnosisReportsCurrentRevisionVersion|TestTextureAgentRevisionRegistersMediaSourceEntities|TestTextureAgentRevisionPromotesResearcherContentRefsToSourceEntities|TestTextureToolRejectsLegacyEditsAndSourceSyntax|TestTexturePromptPreservesInlineSourceRefs|TestPendingUpdateRefsBecomeSourceEntities' -count=1`;
`npx playwright test tests/texture-markdown-lineage.spec.js tests/texture-source-entities.spec.js --grep "Markdown lineage import resolves known citation markers|Markdown lineage import rejects unresolved source markers|Texture source panel omits retired artifact attachment UI" --workers=1` from `frontend/`;
`npx playwright test tests/texture-markdown-lineage.spec.js --grep "Texture Sources panel can cancel diagnosis without exposing source repair" --workers=1` from `frontend/`.

Independent review verdict: accepted. The reviewer found no blocking issues,
confirmed no live frontend imports/calls remain for `texture-source-actions`,
`texture-source-review`, `/source-repairs`, or `/source-attachments`, and
confirmed structured source browsing/transclusion remains present. Remaining
old-source work appears concentrated in publication/proxy legacy fixtures and
explicit negative tests, plus deployed staging proof.

### D7 Slice 5 Local - Publication/Proxy Source Fixture Cleanup

Status: locally implemented and independently accepted on 2026-06-21 after two
P1 repair rounds. Independent review found detached top-level
`source_entities` without `body_doc`; first repair was accepted by independent
re-review. A same-thread repair reviewer then found two additional P1
detached-source windows in proxy enrichment and published version history;
second repair was accepted by two focused re-reviews.

This slice removes publication/proxy current-contract reliance on markdown
source links and Texture metadata source sidecars:

- Publication fallback markdown parsing no longer upgrades `[label](source:id)`
  links into native publication `source_ref` inlines. Native publication source
  refs now come from structured `body_doc` source_ref/source_embed nodes.
- Platform publication rejects `metadata.source_entities` as legacy Texture
  source identity and reads sources from top-level `source_entities`, including
  when `metadata` is empty.
- Platform publication and source metadata normalization reject non-empty
  top-level `source_entities` unless a structured `body_doc` is present, so a
  source entity cannot mint publication source rows without a source_ref or
  source_embed document node.
- Proxy and wire publication enrich reader snapshots/status on top-level
  structured source entities, not on `metadata.source_entities`.
- Proxy publication rejects non-empty head-revision `source_entities` without
  `body_doc` before reader snapshot enrichment can fetch content items or import
  URLs.
- Wire direct-payload publication rejects non-empty `source_entities` without
  `body_doc` before forwarding to corpusd.
- Published version history validates each revision with the same structured
  `body_doc`/`source_entities` rule before the manifest can persist
  source-entity JSON.
- Structured source entities now retain publication-relevant source evidence,
  provenance rights, reader snapshots, reader snapshot status, URL and
  publication-version targets, and `data_vintage` selectors through validation.
- Platform/proxy/browser publication fixtures now create/publish structured
  source refs instead of clickable source links or metadata sidecars.

Local evidence:
`git diff --check`;
`nix develop -c go test ./internal/texturedoc ./internal/platform ./internal/proxy ./internal/wirepublish`;
`npm run build` from `frontend/`;
`rg -n "\\]\\(source:|\\[source:|\\(source:" internal/platform internal/proxy frontend/tests/texture-source-service-publication.spec.js frontend/tests/texture-source-ref-live-agent.spec.js frontend/src --glob '!dist/**'`.

Independent review finding: P1 detached top-level source entities could still be
published without structured document-node validation because `body_doc` was the
only trigger for `normalizePublishTextureStructuredInput`, while
`buildPublicationSourceMetadata` consumed `req.SourceEntities`; the wire
direct-payload path had the same shape. First repair: `source_entities` now
require `body_doc` at platform structured input normalization and source
metadata normalization, and the wire direct payload choke point returns 400
before forwarding detached source entities to corpusd. Regression tests cover
`PublishTexture`, `buildPublicationSourceMetadata`, and the wire direct-payload
path. Independent re-review accepted this repair.

Same-thread repair review finding: P1 detached head-revision `source_entities`
could still be enriched by the proxy before platform rejection, and published
version history could still carry per-revision `source_entities` without
structured node validation. Second repair: proxy publication rejects detached
head source entities before enrichment/history/platform forwarding, gathered
history rejects detached revision source entities before forwarding, and
platform structured normalization validates every history revision before
building the version-history manifest. Regression tests cover pre-enrichment
proxy rejection and platform history rejection.

Independent second-repair reviews accepted the proxy/history repair. Non-blocking
ordering note: regular proxy enriches a valid head revision before validating
full version history; this does not allow detached historical `source_entities`
to trigger enrichment or persist, but a future hardening pass may validate the
history before any head-source enrichment side effect.

Open edge: remaining old-source hits are currently expected to be negative
assertions, markdown-lineage historical import, and runtime legacy-ref
preservation prompts/regexes; those still need explicit final classification
before staging proof.

### D7 Slice 6 Problem Checkpoint - Runtime Source Context Still Uses Metadata Key

Status: problem documented on 2026-06-21 before runtime repair.

Observed behavior: the store and publication paths now reject
`metadata.source_entities` as canonical Texture source identity, but runtime
source-context plumbing still uses `metadata["source_entities"]` as an
intermediate/durable key in several live paths:

- `internal/runtime/runtime.go` keeps `source_entities` in
  `durableMetadataKeys`, so generic appagent metadata carry-forward still treats
  it as durable revision metadata even though `store.CreateRevision` rejects
  that key on canonical writes.
- `internal/runtime/texture_agent_revision.go` collates source entities into
  `metadata["source_entities"]` before building the Texture revision prompt and
  starting the run.
- `internal/runtime/texture_media_sources.go` and
  `internal/runtime/texture_evidence_sources.go` read/write the same metadata
  key while normalizing media/evidence source context.
- `internal/runtime/tools_texture.go` reads `rec.Metadata["source_entities"]`
  as the source pool for structured `patch_texture` operations, while the
  canonical revision it writes stores source entities in top-level
  `Revision.SourceEntities` and strips metadata sidecars.

Problem: this is no longer a canonical write bypass because the store rejects
legacy sidecars and appagent writes sanitize revision metadata. It is still a
contract leak: a key whose name now means "legacy revision sidecar" is also used
as live runtime source context. That makes prompts/tests/docs ambiguous and risks
future agents reintroducing `metadata.source_entities` as a write surface.

Desired repair: split runtime source context from revision metadata identity.
Use an explicitly run-scoped key or typed field for available source context,
derive the prompt/source pool from current revision top-level
`SourceEntities` plus incoming typed evidence/media entities, and keep
`source_entities` out of durable revision metadata carry-forward. Existing
historical import may continue to read old artifacts only as a labelled import
adapter that emits structured `body_doc` / top-level `source_entities`.

Open edge: implement the runtime context-key repair, convert tests that expect
run metadata source pools, and independently review that no canonical revision
write path requires `metadata.source_entities`.

### D7 Slice 6 Local - Runtime Source Context Key Split

Status: locally implemented and independently accepted on 2026-06-21.

Repair:

- Introduced run-scoped `texture_available_source_entities` for transient
  Texture source context used by prompts and `patch_texture` source-pool
  validation.
- Removed `source_entities` from `durableMetadataKeys`, so generic appagent
  revision metadata carry-forward no longer treats source identity as durable
  metadata.
- Stopped `buildAppagentRevisionMetadata` from merging worker-update source
  entities into revision metadata. Appagent revision provenance now reads the
  materialized top-level structured `SourceEntities`, not metadata sidecars.
- `buildAgentRevisionRequest` derives the prompt source inventory from the
  current revision's top-level `SourceEntities` plus run-scoped available source
  context.
- `patch_texture` reads available source context from
  `texture_available_source_entities`; the canonical revision still writes
  `BodyDoc` plus top-level `SourceEntities` and sanitizes source sidecars from
  metadata.
- Runtime conversion helpers now accept D1 structured `SourceEntity` JSON
  (`source_entity_id`, `target.kind`, `target.id`/`uri`) when building prompt
  and tool source pools.

Local evidence:
`nix develop -c go test ./internal/runtime -run 'TestTextureAgentRevisionRegistersMediaSourceEntities|TestTextureAgentRevisionPromotesResearcherContentRefsToSourceEntities|TestPendingUpdateRefsBecomeSourceEntities|TestTexturePromptPreservesInlineSourceRefs|TestHandleUniversalWireStoriesUsesVisibleSourceEntitiesForSourceNetworkManifest|TestUniversalWire|TestBuildStructuredAppagentRevisionProvenance|TestTextureToolRejectsLegacyEditsAndSourceSyntax|TestTextureTool|Test.*SourceEntities|Test.*source_entities|TestMarkTextureMediaSourceRefsResearchState|TestTextureCoagentEvidenceSummarySourceCanPatchWithNativeCitation' -count=1`;
`nix develop -c go test ./internal/store ./internal/texturedoc`;
`git diff --check`;
`rg -n 'metadata\["source_entities"\]|rec\.Metadata\["source_entities"\]|run\.Metadata\["source_entities"\]|textureRun\.Metadata\["source_entities"\]|"source_entities": sourceEntities|source_entities metadata|metadata source_entities' internal/runtime -g '*.go'`.

Independent review verdict: accepted. The reviewer found no blocking issue for
the narrow invariant: live run context reads/writes
`texture_available_source_entities`, appagent revision metadata no longer carries
legacy `metadata["source_entities"]`, prompt context merges top-level revision
`SourceEntities` with run-scoped available entities, and `patch_texture` source
availability starts from current structured sources plus run-scoped available
entities.

Open edge: remaining D7 classification should now focus on negative assertions,
historical markdown-lineage import, prompt warnings that ban markdown source
links, and top-level API fields named `source_entities`.

### D7 Slice 7 Problem Checkpoint - Prompt Contract Still Teaches Old Edit JSON

Status: problem documented on 2026-06-21 before prompt repair.

Observed behavior: the structured `patch_texture` tool schema now accepts
validated document operations such as `update_block_text`, `append_block`,
`insert_block`, `delete_node`, `insert_source_ref`, and `insert_source_embed`.
However, the active revision policy overlay still instructs Texture agents to
call the old string-edit JSON:

- `internal/runtime/textureprompts/overlays/revision_policy.yaml` gives
  examples with `{"op":"replace","find":"exact previous text","replace":"new
  text"}` and `{"op":"append","text":"section text"}`.
- The same overlay still describes `rewrite_texture` as the full replacement
  path, but does not explain that ordinary source-preserving edits should use
  structured block/node/source operations.
- Active prompt tests already assert that the retired "canonical inline source
  link" language is absent, but the positive prompt contract still leaves
  agents with stale operation examples that the current tool rejects.

Problem: this does not create a canonical source-sidecar write bypass, because
`patch_texture` rejects old operation names and the store rejects legacy source
metadata. It is still live optimization pressure toward failed tool calls and
string-edit mental models, especially for source-preserving revisions. A prompt
that tells the model to call invalid old JSON undermines the D4 structured
operation cutover even when the runtime rejects the call.

Desired repair: replace old `replace` / `append` prompt examples with the
structured operation vocabulary, including a clear note that `rewrite_texture`
is exceptional recovery and that source changes use `insert_source_ref` /
`insert_source_embed` backed by existing `source_entity_id` values. Update
active prompt tests to assert the old operation examples are absent and the
structured operation contract is present.

Open edge: implement the prompt repair, run focused prompt/tool tests, and
obtain independent review that no active Texture prompt teaches markdown source
links or old string-edit operation JSON as the canonical edit path.

### D7 Slice 7 Local - Structured Prompt Operation Contract

Status: implemented and independently accepted on 2026-06-21 after one repair
round.

Repair:

- `revision_policy.yaml` no longer teaches `patch_texture` calls with old
  `replace` / `append` operation JSON. It now names the structured operation
  vocabulary: `update_block_text`, `insert_block`, `append_block`,
  `delete_node`, `insert_source_ref`, and `insert_source_embed`.
- `run_system.yaml` describes Texture writes as structured block/node/source
  operations, with `append_block` for first-draft material when no target block
  id is available.
- Revision prompts now include a compact structured document outline when the
  current revision has `body_doc`, exposing block/node/source ids needed by
  `patch_texture` without asking the model to author canonical document JSON.
- Required-initial retry reminders in the tool loop now instruct structured
  `append_block` / `update_block_text`, not old append/replace/find-text edits.
- Focused prompt tests assert that structured operation examples and body-doc
  ids are present, and that old `replace` / `append` examples and retired
  canonical markdown source-link language are absent.
- Comprehensive retry/provider fixtures adjacent to this prompt path now use
  structured `append_block` / `update_block_text` payloads, while explicit
  legacy replace tests assert rejection.

Local evidence:
`nix develop -c go test ./internal/runtime -run 'TestTexturePrompt(FocusesLongDirectUserEdits|UsesStructuredPatchTextureOperationContract|PreservesInlineSourceRefs|InitialRevisionUsesSingleWriterLoop|ForFactualFirstRevisionForbidsUngroundedContent)' -count=1`;
`nix develop -c go test ./internal/runtime -run 'TestTexturePrompt|TestTextureToolRejectsLegacyEditsAndSourceSyntax|TestTextureToolStructured' -count=1`;
`nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestInitialTextureNoOpPatchRetriesIntoUsefulDraft|TestInitialTextureRevisionRejectsNoOpPromptCopy' -count=1`;
`nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestTextureAgentRevisionAppliesStructuredEdit|TestTextureAgentRevisionRejectsMalformedEditTextureToolCall|TestTextureAgentRevisionMutationCompletedOnlyOnce|TestTextureApplyEditsRejectsLegacyReplace|TestInitialTextureNoOpPatchRetriesIntoUsefulDraft|TestInitialTextureRevisionRejectsNoOpPromptCopy' -count=1`;
`rg -n '"op":"replace"|"find":"exact previous text"|"op":"append","text":"section text"|replace_all|Canonical inline Source Entity syntax|Preserve inline source ref exactly' internal/runtime/textureprompts internal/runtime/texture_agent_revision.go internal/runtime/texture_prompt_unit_test.go`;
`git diff --check`.

Independent review finding: P1 `internal/runtime/toolloop.go` still taught old
append/replace/find-text retry guidance after a failed required initial
`patch_texture`; P2 adjacent comprehensive retry fixtures still encoded old
patch operations and the cited comprehensive tests failed. Repair: the active
retry reminder now names structured operations, the retry provider extracts
block ids from the structured prompt outline, useful drafts use `append_block`,
no-op checks use `update_block_text`, and legacy replace tests assert rejection.
Independent re-review accepted the repair.

Open edge: independent review should verify that the outline does not re-create
a model-authored JSON contract, that active prompts no longer teach invalid old
operation names, and that the remaining old syntax strings are negative test
assertions only.

Residual outside this slice: one older comprehensive source/table fixture still
creates a user revision containing unresolved `[1]`, which the D2 store guard
now rejects before the prompt path under test. That is D7 comprehensive
source-fixture cleanup, not prompt-contract residue.

### D7 Slice 8 Local - Dead Legacy Text Edit Helper Deletion

Status: implemented and independently accepted on 2026-06-21.

Repair:

- Removed the dead Go-only `textureTextEdit` type and `applyTextureTextEdit`
  helper from `tools_texture.go`.
- Removed the hidden `editTextureArgs.Edits` field and the associated
  "legacy find/replace" rejection branch. Public `patch_texture` calls already
  enter through the structured `edits` array and reject old operation names
  through `applyStructuredTextureEdit`.
- Updated focused and comprehensive tests so legacy `append` / `replace`
  attempts are represented as bad public structured operations, not as a
  hidden Go-only field.

Local evidence:
`nix develop -c go test ./internal/runtime -run 'TestTextureToolRejectsLegacyEditsAndSourceSyntax|TestTextureToolStructured|TestTexturePrompt' -count=1`;
`nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestTextureApplyEditsRejectsLegacyReplace|TestInitialTextureNoOpPatchRetriesIntoUsefulDraft|TestInitialTextureRevisionRejectsNoOpPromptCopy' -count=1`;
`rg -n 'textureTextEdit|applyTextureTextEdit|\bEdits: \[\]textureTextEdit|legacy find/replace|replace edit|append edit|find text|"op":"replace"|"op":"append"' internal/runtime/tools_texture.go internal/runtime/texture_tool_unit_test.go internal/runtime/texture_test.go internal/runtime/toolloop.go internal/runtime/textureprompts internal/runtime/texture_agent_revision.go internal/runtime/texture_prompt_unit_test.go`;
`git diff --check`.

Open edge: focused review should verify no active code path or prompt still
teaches old text edit operations and that remaining `"op":"replace"` hits are
negative assertions or simulated bad model calls only.

Independent review verdict: accepted. The reviewer confirmed the hidden legacy
`Edits` field and text-edit helper are gone, public `patch_texture` exposes only
structured operation enum values, old `append` / `replace` attempts are rejected
as bad structured ops, active prompt/reminder scans no longer teach old
operation names, and focused normal/comprehensive tests pass.

### D7 Slice 9 Local - Media Source Ref Fallback Deletion

Status: implemented and independently accepted on 2026-06-21.

Repair:

- Frontend `revisionSourceEntities` no longer synthesizes source entities from
  `revision.metadata.media_source_refs` and no longer reads
  `metadata.source_entities`; current Texture revisions must provide top-level
  `source_entities`.
- Removed the frontend `mediaRefToSourceEntity` fallback helper.
- Runtime media registration no longer merges `metadata["media_source_refs"]`
  into the source pool. It starts from current structured source entities and
  deterministic URL/media discovery only.
- Removed the runtime `decodeTextureMediaSourceRefs`,
  `sourceEntitiesFromMediaRefs`, and `markTextureMediaSourceRefsResearchState`
  migration helpers, plus the unused `revision_media_source_refs_intro` prompt
  overlay.
- Tests now assert that legacy media-ref metadata does not synthesize source
  entities.

Local evidence:
`nix develop -c go test ./internal/runtime -run 'TestTextureAgentRevisionRegistersMediaSourceEntities|TestTextureAgentRevisionPromotesResearcherContentRefsToSourceEntities|TestMediaSourceRefToSourceEntityUsesTypedEvidenceStates|TestTextureToolRejectsLegacyEditsAndSourceSyntax' -count=1`;
`cd frontend && npx playwright test tests/texture-source-entities.spec.js --grep 'source evidence states|revisions do not synthesize source entities from legacy media refs|multimedia source entities' --workers=1`;
`rg -n 'media_source_refs|RevisionMediaSourceRefsIntro|revision_media_source_refs_intro|decodeTextureMediaSourceRefs|markTextureMediaSourceRefsResearchState|sourceEntitiesFromMediaRefs|revisionMediaSourceRefs|mediaRefToSourceEntity|metadata\.source_entities|metadata\["source_entities"\]' internal/runtime frontend/src frontend/tests -g '*.go' -g '*.ts' -g '*.svelte' -g '*.js' --glob '!frontend/dist/**'`;
`git diff --check`.

Open edge: focused review should verify remaining `media_source_refs` /
`metadata.source_entities` hits are negative tests, publication metadata tests,
or canonical write guard lists, not source construction paths.

Independent review verdict: accepted. The reviewer confirmed frontend source
derivation now returns publication bundle entities or top-level
`revision.source_entities`, runtime source construction starts from current
top-level revision `SourceEntities` plus deterministic media discovery, the
media-source-ref prompt overlay is gone, and focused Go/frontend tests pass.

### D7 Browser Proof Problem - Published Structured Artifacts Render Flattened Content

Status: discovered on 2026-06-21; must repair before staging.

Problem:

- Local authenticated browser proof against the repo service stack showed that
  public publication bundles can resolve with `artifact.body_doc` and
  `source_entities`, while the published Texture reader still initializes the
  editor from only `bundle.artifact.content`.
- That flattened-content rendering path erases structured `source_ref` nodes on
  public routes, so a publication can store transclusions in platform metadata
  while the reader surface has no native `[data-texture-source-ref]` citation
  atoms to expand.
- This is a D7 blocker because the hard-cut contract requires source identity
  to remain a document node across publication/readback, not merely a sidecar
  available through resolve/export APIs.

Evidence:

- `cd frontend && npx playwright test tests/texture-source-entities.spec.js
  tests/texture-structured-editor-doc.spec.js
  tests/texture-source-service-publication.spec.js --workers=1` with
  `nix develop -c env CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh` running
  passed current source-window/media/transclusion tests but failed publication
  source-service tests because `[data-texture-published-reader]` had no
  `[data-texture-source-ref]`.
- Code inspection shows `loadPublishedContext` in
  `frontend/src/lib/TextureEditor.svelte` assigns
  `editorValue = bundle.artifact?.content || ''` but does not assign
  `editorBodyDoc = bundle.artifact?.body_doc || null`.

Required repair:

- Published Texture read mode must hydrate `editorBodyDoc` from
  `bundle.artifact.body_doc` and render via `renderStructuredTextureDocHTML`
  when present, using publication bundle source entities.
- Publication source metadata should preserve the canonical distinction between
  source kind and target kind for URL-backed sources: source kind
  `web_source`, target kind `url`.

Repair:

- `TextureEditor` now hydrates `editorBodyDoc` from
  `bundle.artifact.body_doc` in published read mode before rendering.
- Platform publication source normalization now defaults URL/web-url targets to
  source kind `web_source` while preserving target kind `url`.
- Browser tests now assert the hard-cut legacy table behavior explicitly:
  content-only markdown table text is a structured paragraph until table nodes
  are deliberately added to the canonical schema.

Local evidence:
`nix develop -c go test ./internal/platform -run 'TestPublishTextureStructuredBodyDrivesPublicationSources|TestPublicationURLTargetDefaultsToWebSourceKind|TestBuildPublicationSourceMetadata|TestPublishTextureRejectsSourceEntitiesWithoutBodyDoc' -count=1`;
`cd frontend && npx playwright test tests/texture-source-service-publication.spec.js --workers=1`;
`cd frontend && npx playwright test tests/texture-source-entities.spec.js --grep 'legacy content-only markdown tables' --workers=1`;
`cd frontend && npx playwright test tests/texture-source-entities.spec.js tests/texture-structured-editor-doc.spec.js tests/texture-source-service-publication.spec.js --workers=1`.

Review state: independent review was attempted three times, but review agents
failed to return callbacks and were interrupted. Treat this repair as locally
verified pending later batch review.

Non-blocking discovered edge:

- The same browser run exposed two old table autosave tests that create
  `content`-only markdown-table revisions and expect markdown tables to be the
  current editable contract. The D1 structured schema has no table nodes yet, so
  those fixtures are legacy-shaped under the hard cutover. Track this as a
  separate schema/design edge: either add structured table nodes deliberately or
  retire/relabel those tests as legacy fallback checks that do not define
  canonical Texture structure.

### D7 Landing Problem Checkpoint - Nix Service Source Filters Omit Structured Doc Package

Status: discovered on 2026-06-21 during the pushed landing loop; must repair
before staging deploy proof.

Problem:

- The pushed behavior repair commit
  `5c057afc63adf6df8db9a881c92942586c70fa51` reached GitHub Actions, but
  staging deploy failed before service activation.
- The deployment Nix build uses per-service `internalDirs` allow-lists in
  `flake.nix`. D1-D7 introduced `internal/texturedoc` imports into platform,
  store, runtime, and publication paths, but the Nix service source filters were
  not updated to include that package.
- Local `go test` sees the full worktree, so this class of failure only appears
  in filtered Nix package builds and deploy jobs.

Evidence:

- GitHub Actions CI run `27922543313`, job `Deploy to Staging (Node B)`, failed
  after checkout of `5c057afc`.
- The proxy package build failed with:
  `internal/platform/publication_document.go:13:2: cannot find module providing package github.com/yusefmosiah/go-choir/internal/texturedoc: import lookup disabled by -mod=vendor`.
- The sandbox package build failed with:
  `internal/store/texture_structured_revision.go:8:2: cannot find module providing package github.com/yusefmosiah/go-choir/internal/texturedoc: import lookup disabled by -mod=vendor`.

Required repair:

- Add `internal/texturedoc` to every Nix Go service source filter whose
  transitive imports can reach structured Texture code: at minimum proxy,
  corpusd, gateway, sourcecycled, and sandbox.
- Prove with focused `nix build` package checks before pushing the repair, then
  rerun CI/deploy and staging acceptance.

Repair:

- `flake.nix` now includes `internal/texturedoc` in the `internalDirs`
  allow-list for proxy, corpusd, gateway, sourcecycled, and sandbox.
- Local proof inspected the evaluated filtered source outputs for
  `packages.x86_64-linux.proxy`, `.corpusd`, `.sandbox`, `.gateway`, and
  `.sourcecycled`; each now contains `internal/texturedoc/schema.go` and
  `internal/texturedoc/projection.go`.
- Local x86_64 package compilation could not run on this Mac because no
  `x86_64-linux` builder was available. CI/deploy remains the compilation and
  activation oracle.

Local evidence:

- `nix eval --raw .#packages.x86_64-linux.<service>.src` plus `find` over
  `internal/texturedoc` for proxy, corpusd, sandbox, gateway, and
  sourcecycled.
- `rg -o 'github.com/yusefmosiah/go-choir/internal/[A-Za-z0-9_/-]+'` over the
  evaluated filtered source outputs confirmed the `internal/texturedoc` import
  family is present in each affected source archive.
- `git diff --check && nix flake check --no-build`.

### D7 Landing Proof - Structured Texture Source Transclusions On Staging

Status: deployed and product-path verified on 2026-06-21/2026-06-22 UTC for
commit `d77e0457806d6a9de27657b7ffb5f8f3a7862922`.

CI/deploy:

- GitHub Actions CI run `27922834186` completed successfully for
  `d77e0457806d6a9de27657b7ffb5f8f3a7862922`.
- Staging deploy job `82619778958` built the selected Nix services, ordinary
  guest image, and Playwright guest image. Health checks reported deployed
  commit `d77e0457806d6a9de27657b7ffb5f8f3a7862922` for proxy, sandbox, and
  corpusd.
- Deploy job health log lines reported proxy build commit, upstream sandbox
  build commit, and corpusd build commit
  `d77e0457806d6a9de27657b7ffb5f8f3a7862922`.

Deployed product-path acceptance:

- `BASE_URL=https://choir.news ... npx playwright test
  tests/texture-source-service-publication.spec.js --workers=1`: 3 passed.
  This creates Texture documents through `/api/texture/*`, publishes through
  `/api/platform/texture/publications`, resolves/exports through platform APIs,
  opens public `/pub/texture/...` routes, expands native
  `[data-texture-source-ref]` citations, opens source reader windows, and checks
  guest access without Browser-as-source gathering.
- `BASE_URL=https://choir.news ... npx playwright test
  tests/texture-source-entities.spec.js --grep
  "raw markdown|structured revisions|multimedia source entities|source selectors|source evidence states|legacy texture"
  --workers=1`: 5 passed. This confirms raw markdown source links do not render
  as native Texture source refs and multimedia source entities render
  transcluded media without clickable source links.
- Direct deployed API probe with virtual-passkey auth created document
  `f1fff3b4-1e5a-4416-89f4-e699489e7289`, wrote revision
  `bf27f1a3-c95f-4f64-98f1-b575303650af`, wrote revision
  `4f989f43-e26a-4384-9eff-e92b178cffa9` changing surrounding prose while
  preserving source entity `src-edit-preserve`, verified the head still carries
  `body_doc` `source_ref` plus top-level `source_entities`, and verified raw
  `[bad](source:detached)` revision input is rejected with HTTP 400.
- The same proof document was published at
  `/pub/texture/structured-edit-preservation-1782089779444-pub47b0efe77`
  (`pub-47b0efe7-7040-40fb-9de7-4c3e6633c9ed`,
  `pubver-ef87a710-f7e8-4bb0-9ec6-8be8547afd4b`). Playwright confirmed the
  public reader has exactly one native `data-texture-source-ref` for
  `src-edit-preserve` and no `href=` or `source:src-edit-preserve` source-link
  syntax.

Comet/browser note:

- The user's Comet session was used as requested, but it rendered a blank
  viewport for both `https://choir.news` and the proven public Texture route
  above. Public HTTP for the app shell, deploy health logs, and Playwright
  product-path proof all succeeded. Treat this as a separate Comet-specific
  visual proof blocker unless mission settlement is defined to require Comet
  rendering specifically.

### D8 Live Research Handoff Problem Checkpoint - Findings Without Native Sources

Status: discovered on 2026-06-22 UTC from live staging evidence in the
`yusefnathanson@me.com` primary computer; must repair before claiming the
research-to-Texture citation path is settled for broad current-events briefs.

Problem:

- The live document `What’s new in world news.texture` reached appagent version
  7 and kept revising, which supports the multi-revision liveness claim.
- Every revision in that document stored a structured `body_doc`, but every
  revision had zero top-level `source_entities`. Therefore the frontend had no
  native `source_ref` / `source_embed` nodes to render as citation points.
- Researcher updates delivered useful narrative findings, but the first five
  `update_coagent` packets carried no typed `refs` or `evidence_ids`. The final
  runtime fallback carried raw URLs and `tool:web_search`, but no
  `source_service_item`, `content_item`, or evidence record handle.
- Runtime source collation intentionally ignores free-form prose and raw URLs;
  it mints native Texture sources only from typed evidence records,
  `content_item` refs, or `source_service_item` refs. That invariant is correct,
  but the live researcher path did not produce enough typed source substrate.
- The same shape applies beyond researchers. `update_coagent` is the typed
  handoff envelope from every non-Texture actor into a Texture-owned artifact:
  researchers hand off source-service/content/evidence records; super/vsuper/
  co-super hand off command output, code diffs, test results, AppChangePackages,
  screenshots, videos, benchmark logs, and other execution artifacts. These
  must be represented as citeable/transcludable source entities rather than
  pasted into prose or left as opaque notes.
- Texture then compensated by writing source-status and process/provenance prose
  into the reader-facing document body, including "evidence pending",
  "research checkpoint", and "Evidence note" sections. That violates the
  user-facing artifact contract: internal research state belongs in updates,
  revision metadata, decisions, or native source transclusions, not as ordinary
  document content.

Evidence:

- Live active computer lookup:
  `yusefnathanson@me.com` maps to user
  `5bd6de97-3b58-408c-bf89-c42c81b083de`; `ownerships.json` maps that owner and
  desktop `primary` to active VM
  `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19` at sandbox URL
  `http://10.200.230.2:8085`.
- Sandbox health for that VM reported deployed commit
  `d77e0457806d6a9de27657b7ffb5f8f3a7862922`.
- `GET /api/texture/documents` with trusted owner headers identified document
  `5d89a835-41a6-49e0-8555-172a574ef317`, current revision
  `7fde6600-bea8-4fe6-9ef9-667be64131e8`, current version `7`, and
  `revision_count` `8`.
- `GET /api/texture/documents/5d89a835-41a6-49e0-8555-172a574ef317/revisions`
  showed versions 1 through 7 all appagent-authored, all with `body_doc`
  present and `source_entities` empty.
- `GET /api/texture/documents/5d89a835-41a6-49e0-8555-172a574ef317/diagnosis`
  showed researcher channel messages at seq 1-6. Seq 1-5 contained findings and
  questions but no typed source refs. Seq 6 was a runtime fallback with raw
  Reuters URLs and `tool:web_search`, not source-service or content-item refs.
- Head revision metadata for version 7 recorded `source: rewrite_texture`,
  `worker_updates_consumed` seq 6, and rationale: "Transform accumulated
  research checkpoints into the reader-facing world-news brief..." while
  `source_entities` remained absent.

Debugging/tooling gap:

- This investigation required reconstructing the account -> owner id -> VM
  ownership -> sandbox URL -> trusted-header Texture API path from scattered
  implementation knowledge.
- The project needs a documented, read-only staging-debugging path and ideally a
  small tool for "given email and optional Texture title, print active computer,
  sandbox health/build identity, recent documents, revisions, source entity
  counts, worker update refs/evidence, and relevant runs." This is now recorded
  in `docs/runbook-staging-live-texture-debugging.md`.

Required repair:

- Preserve the no-prose-scraping invariant. Do not reintroduce parsing of
  researcher narrative text or raw markdown/source links into native sources.
- Treat `update_coagent` as an evidence-bearing source envelope for all
  coagents, not a researcher-only findings message. Its typed fields must be
  enough to mint source entities for reader-facing Texture citations and
  transclusions.
- Make researcher current-events handoff produce typed source substrate before a
  reader-facing finalization pass: prefer `source_search` results with
  `source_service_item:<id>`, imported content items, or `save_evidence` records
  with `metadata.content_id`.
- Extend the source/transclusion contract for execution evidence: command
  outputs, shell sessions, diffs/patch hunks, test runs, AppChangePackages,
  screenshots, videos, and benchmark logs should have explicit target kinds,
  selectors, open surfaces, and rights/provenance. A Texture can then cite a
  factual claim with a research source, cite a code-change claim with a diff
  hunk, cite a verification claim with a command/test output, or embed media
  proof, all through the same `source_ref` / `source_embed` nodes.
- Ensure runtime fallback from research tools can carry citable typed refs when
  the underlying tool output has source-service item ids or imported content
  ids, and report an explicit "no native source substrate" blocker when only raw
  web URLs/snippets exist.
- Tighten Texture prompts so status/provenance/checkpoint prose is not written
  into the canonical reader-facing document when native source entities are
  absent. Texture should either write a concise, honest reader-facing draft
  without pretending it has citations, or request/follow up for citable source
  substrate.
- Add regression coverage for the live shape: broad researcher findings with
  no typed source refs must not produce native citations by prose scraping, and
  a citable researcher packet with typed refs must produce top-level
  `source_entities` plus `source_ref` nodes in the next appagent revision.
- Add cross-role regression coverage: a super/vsuper/co-super `update_coagent`
  packet containing command output refs, diff refs, tests, artifacts, or package
  refs must expose those as source entities that Texture can cite/transclude;
  a packet containing only prose must remain non-citeable and should not be
  rendered as reader-facing metadata.

Repair slice:

- `update_coagent` source collation is now role-neutral for typed source
  substrate. Runtime still refuses prose scraping and raw URL promotion, but it
  now mints native source entities from typed execution refs/evidence in
  `refs`, path-shaped `artifacts`, bare `tests`, and evidence records.
- Added execution target kinds to the structured Texture validator:
  `command_output`, `shell_session`, `diff_hunk`, `patch`, `test_run`,
  `app_change_package`, `screenshot`, `video_artifact`, and `benchmark_log`.
  These target kinds round-trip through runtime-to-structured source conversion
  with durable IDs/file paths and open through existing source/window, file,
  image, or video surfaces.
- Tightened researcher, super, `update_coagent`, and Texture prompts:
  citable packets must include typed source substrate; prose-only findings and
  raw URLs are coordination only; partial worker findings must not be pasted as
  process metadata, source-status notes, or checkpoint labels in reader-facing
  canonical document bodies.
- Regression coverage now includes command-output evidence records, super-style
  `update_coagent` refs/artifacts/tests becoming source entities without
  scraping findings prose, and structured-doc validation/projection for
  execution evidence targets.

Repair evidence:

- `nix develop -c go test ./internal/sourcecontract ./internal/texturedoc`
- `nix develop -c go test ./internal/runtime -run 'TestEvidenceRecordToSourceEntity|TestWorkerUpdateExecutionEvidence|TestPendingUpdateRefs|TestTextureCoagent|TestTexturePrompt|TestSystemPrompt|TestUpdateCoagent' -count=1`
- `git diff --check`

Remaining proof:

- Land this behavior change through CI/deploy, then run a staging product-path
  Texture proof that sends at least one researcher-style typed source packet and
  one super-style execution evidence packet through `update_coagent`, verifies
  the next Texture revision has structured `source_entities`, and verifies the
  visible document uses native source refs/embeds without clickable markdown
  source links or process-metadata body prose.

### D8 staging blocker: Super execution packets are lossy/ambiguous

After deploying the D8 `update_coagent` source-envelope repair, staging proved a
new, narrower blocker in the super execution path.

Problem:

- Texture can call `request_super_execution` and the persistent Super run wakes,
  but the handoff is delivered as a generic coagent update rather than an
  unmistakable execution packet. In the live proof, Texture asked Super to run a
  harmless command and report command evidence through typed `update_coagent`
  refs. Super replied that no pending privileged execution packet was present,
  so no command evidence was produced and Texture could not create a native
  `source_ref` for command output.
- The coagent packet shape injected into runs also omits typed fields that now
  matter for the whole source system: `evidence_ids`, `artifacts`, `tests`,
  `questions`, `proposals`, `capability_requests`, and `notes`. The plain
  rendered `content` still contains those sections, but the machine-readable
  packet only exposes `summary`, `findings`, `refs`, and `content`.

Evidence:

- Deployed commit under test:
  `63f44e07691c7df58ceec9dd078b6e9a8be37322`.
- Active staging computer for `yusefnathanson@me.com` reported sandbox URL
  `http://10.200.233.2:8085` and health/build commit
  `63f44e07691c7df58ceec9dd078b6e9a8be37322`.
- Prompt-bar submission
  `0545d5f3-38b7-4765-acca-8bd80c12d3b7` created Texture document
  `a7260780-92d8-4a60-bce5-849cb2f679c8`.
- Diagnosis for that document showed channel seq 1 from Texture to Super:
  "Run a harmless command that prints exactly D8_TYPED_SOURCE_PROOF. Report back
  to Texture via update_coagent with a typed command_output reference or evidence
  entry..." Seq 2 from Super replied: "No privileged execution packet was
  included in this turn/context..."
- The appagent revision stayed honest: it wrote an interim reader-facing note
  without process metadata and without false citation points, but it had
  `source_entities` count `0` because no command evidence arrived.

Required repair:

- Treat `request_super_execution` deliveries as explicitly typed execution
  packets while still using `update_coagent` as the durable delivery envelope.
  Super should see that the objective itself is the work packet, including
  constraints and expected evidence.
- Carry the full typed `WorkerUpdateRecord` evidence surface in coagent packet
  JSON, not just the rendered prose content. Execution refs, tests, artifacts,
  and capability requests must survive wake/injection as first-class fields.
- Keep the source invariant unchanged: Texture may mint native sources from
  typed evidence substrate, never from scraped prose.

### D9 redesign: make `update_coagent` source-centric

Cognitive transforms used:

- **Object transform:** `update_coagent` is not fundamentally a chat message.
  It is an append-only evidence/source packet addressed to another durable
  actor. The message users and agents read is a projection.
- **Duality:** swap the current object/observer relationship. Today sources are
  optional annotations on a worker update; in the target design, narrative
  claims and directives are annotations on sources, artifacts, and requested
  actions.
- **State machine:** separate observed material, proposed interpretation,
  requested action, and reader-facing document mutation. This prevents the
  impossible state "Texture cites a claim whose source identity exists only in
  prose."
- **API contract:** define what the receiver can trust structurally. A receiver
  should not need to parse the rendered message to know what sources, commands,
  tests, diffs, or media are available for transclusion.
- **Deletion-first:** delete parallel syntaxes rather than supporting every old
  shape. The cutover should have one durable envelope and no legacy compatibility
  layer.

Redesigned primitive:

```text
update_coagent = addressed SourcePacket append
```

The canonical payload should have five top-level sections:

```json
{
  "schema_version": "coagent_source_packet.v1",
  "kind": "evidence_update | execution_request | execution_result | blocker | question | proposal | decision_request",
  "summary": "short human-readable projection",
  "claims": [],
  "sources": [],
  "actions": [],
  "questions": [],
  "notes": []
}
```

Only `summary` and `notes` are prose-only. They are never citeable. Everything
that Texture may cite, embed, or open must be represented in `sources`; every
instruction that asks another actor to do work must be represented in `actions`.

#### `sources`

`sources[]` is the source graph edge format shared by researcher, super,
vsuper, co-super, processor, reconciler, and future actors. Each item is either
an inline evidence object or a reference to a durable evidence object:

```json
{
  "source_id": "optional caller-local id",
  "kind": "web_page | content_item | command_output | shell_session | diff_hunk | patch | test_run | app_change_package | file_artifact | screenshot | video_artifact | benchmark_log | publication | texture_span",
  "target": {
    "uri": "source_service_item:... | content_item:... | command_output:... | file:/... | app_change_package:...",
    "title": "human label",
    "media_type": "text/markdown | text/plain | image/png | video/mp4 | application/vnd.choir.diff"
  },
  "selectors": [
    {"kind": "whole_resource"},
    {"kind": "text_quote", "quote": "..."},
    {"kind": "line_range", "start": 12, "end": 20},
    {"kind": "image_region", "x": 0.1, "y": 0.2, "width": 0.3, "height": 0.4}
  ],
  "evidence": {
    "state": "available | pending | blocked | unavailable",
    "confidence": "low | medium | high",
    "rights_scope": "private_user_source | public_url | generated_artifact | local_computer"
  }
}
```

This compiles directly to Texture `SourceEntity` plus visible `source_ref` or
`source_embed` document nodes. It also covers multimedia and code execution
without adding special markdown syntaxes.

#### `claims`

`claims[]` records the semantic use of source material without making the model
write JSON document nodes:

```json
{
  "claim_id": "claim-local-id",
  "text": "A reader-facing claim or execution result.",
  "source_ids": ["source-local-id"],
  "stance": "supports | qualifies | contradicts | background",
  "recommended_surface": "inline_ref | block_embed | source_panel | decision_log"
}
```

Texture can choose whether and where to write this claim into the canonical
document. The source relationship survives even if Texture rewrites the prose.

#### `actions`

`actions[]` replaces the current ambiguous "assignment prose" problem:

```json
{
  "action_id": "act-local-id",
  "type": "run_command | inspect_file | produce_diff | run_tests | open_browser | request_worker | import_source | revise_texture",
  "objective": "Run a harmless command that prints D8_TYPED_SOURCE_PROOF.",
  "inputs": {
    "command": "printf 'D8_TYPED_SOURCE_PROOF\\n'",
    "cwd": "/Users/wiz/go-choir",
    "mutation": "none"
  },
  "expected_sources": [
    {"kind": "command_output", "required": true},
    {"kind": "test_run", "required": false}
  ],
  "safety": {
    "mutation_class": "green | yellow | orange | red | black",
    "network": "forbidden | allowed | required",
    "file_mutation": "forbidden | allowed | required"
  }
}
```

Super no longer has to infer whether a mailbox update is an execution packet:
`kind=execution_request` with `actions[]` is executable. Its response should be
`kind=execution_result` with `sources[]` for command output, diffs, tests,
packages, screenshots, videos, or blockers.

#### Receiver rules

- Texture may cite or embed only `sources[]`, never `summary`, `notes`, or
  freeform `claims[].text`.
- Super may execute only `actions[]`, never ambiguous prose hidden in `summary`.
- Researcher must return source-service/content/evidence sources for factual
  claims, or return a blocker explaining that the current tool output has no
  native source substrate.
- The rendered channel message is derived from the packet for humans. It is not
  the canonical machine contract.
- Legacy `findings`, `evidence_ids`, `refs`, `artifacts`, and `tests` fields
  are deleted in the hard cutover. Do not accept them as alternate API shapes,
  do not compile them at the boundary, and do not leave downstream readers that
  interpret them. Callers must speak `claims[]`, `sources[]`, and `actions[]`
  directly.

#### Why this resolves the current bugs

- The no-citation world-news failure becomes structurally impossible to mistake:
  a prose-only packet has `sources=[]`, so Texture knows it can write an
  uncited draft or ask for source substrate, but cannot invent citation points.
- The Super command proof failure gets a first-class `actions[]` request and a
  first-class `command_output` source result.
- Multimedia and code artifacts use the same source/entity surface as web
  research, so `source_ref` / `source_embed` remain the only Texture
  transclusion primitives.
- Editing becomes simpler: the model edits prose and places refs by
  `source_id`; the stored document remains ProseMirror-compatible JSON with
  source atoms, not markdown links or offset citations.

Hard cutover plan:

- Replace `WorkerUpdateRecord` with `CoagentSourcePacket` in types, storage,
  tool schemas, wake/injection, diagnosis, and tests. Do not add a compatibility
  parser for the old shape.
- Change `update_coagent` to require `schema_version`, `kind`, and at least one
  of `claims`, `sources`, `actions`, `questions`, or `notes`.
- Remove old tool parameters: `findings`, `evidence_ids`, `evidence`,
  `artifacts`, `refs`, `tests`, `proposals`, and `capability_requests`.
  Equivalent information must appear in `claims`, `sources`, `actions`,
  `questions`, or `notes`.
- Delete old packet JSON fields from `coagentUpdatePacketItem`; wake/injection
  should deliver the source packet verbatim plus a rendered human projection.
- Delete Texture source collation from old worker-update fields; source collation
  reads only `packet.sources`.
- Delete Super execution inference from prose. Super executes only
  `kind=execution_request` packets with `actions[]`.
- Update prompts and tests in the same cutover so no role is instructed to use
  old fields.
- Staging acceptance is allowed to break old prerelease documents and old
  in-flight coagent messages. This is prerelease; correctness of the new
  invariant beats migration continuity.
