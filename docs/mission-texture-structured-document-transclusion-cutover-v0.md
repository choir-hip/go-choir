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
staging acceptance proof. Current value: 5. Last delta: -1 for accepted D4
agent path integration: appagent Texture writes now use structured operations
and top-level structured source entities instead of string patch/source-sidecar
canonical writes. D5 multimedia problem checkpoint is documented; it does not
reduce V until implementation and review repair the multimedia path.

budget: Planning budget is one paradoc pass. D1 implementation plus review-fix
budget was isolated additive code with focused tests and accepted independent
review. Solvency verdict: proceed to D2 as a fresh implementation pass; do not
bundle frontend editor saves, publication export, staging/deploy, or broad
old-path deletion unless the paradoc is updated first. D2 used the budget for a
store/API write-boundary cut plus focused tests only. D3 used the budget for a
bounded editor/user path preservation cut plus focused frontend tests and the
D2 API regression; it did not bundle agent tools, publication/export,
multimedia embedding, staging/deploy, or broad old-path deletion. D4 budget is a
bounded runtime/tool implementation pass with focused runtime tests; do not
bundle multimedia resolver, publication/export, deployment, or broad old-path
deletion unless this paradoc is updated first. D4 used that budget for the
bounded runtime/tool cut plus Universal Wire structured source-manifest repair
and did not bundle D5/D6/D7. Next budget is D5 multimedia problem checkpoint
first, then a bounded resolver/rendering cut. The D5 checkpoint is now recorded;
the next pass may touch the protected multimedia resolver/rendering surface.

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
for later cuts remain multimedia/source resolver rendering, publication/export,
Source Viewer/reader integration beyond existing open-source affordances,
deployment routing, and run acceptance involving Texture. D5 will be red/orange
depending on final slice because it touches deterministic media ingestion,
Texture agent prompt context, structured source validation, and frontend media
transclusion rendering.

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
derivation, runtime shards, and `git diff --check`. Later cuts still need
multimedia resolver and renderer proof, publication/export serialization, broad
old-path deletion, CI, staging deploy identity, Comet/browser staging proof with
numbered source refs, expanded source window, multimedia source expansion, agent
edit preserving refs, and attempted markdown link/source token rejection;
RunAcceptanceRecord at staging-smoke-level or higher if platform behavior
changes. D5 evidence target is focused Go/frontend tests proving new media
discoveries do not write `metadata.media_source_refs`, multimedia structured
source entities validate and render from top-level `SourceEntities` plus
`source_ref` / `source_embed` nodes, and no clickable source links or client-side
media-sidecar synthesis are required for new revisions.

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
settlement. Full repair still requires publication/export, old-syntax deletion
receipts, and staging proof.

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
multimedia source entities exist in schema, but resolver/rendering paths still
now have local D5 proof that image/video/audio/PDF/transcript/file open surfaces
validate through the shared source contract and that image/video/audio/PDF
rendering is driven by source entities rather than media sidecars. Texture-span
publication/export proof remains open. E3: publication/export/diff/search
still consume the projection and must not be treated as proof of structured
transclusion behavior. D5 checkpoint records the specific sidecars/pathways to
repair: runtime `media_source_refs`, prompt context that prefers those refs, and
frontend media-ref synthesis/rendering outside top-level structured entities.
D5 implementation removes the new-write/new-prompt path for those sidecars and
keeps legacy media-ref synthesis only for revisions without structured
`body_doc`.

next move: record an independent review receipt if a reviewer becomes available;
otherwise continue to D6 publication/export projection cut. Do not bundle
deployment or broad old-path deletion unless this paradoc is updated first.

ledger file: docs/mission-texture-structured-document-transclusion-cutover-v0.ledger.md

version / lineage: v0. Created 2026-06-21 as a successor/specialization of
`mission-texture-hard-cutover-v0`, `mission-texture-versioned-artifact-v0`, and
`mission-vtext-source-entities-multimedia-transclusion-v0`. <!-- texture-cutover-allow: historical mission evidence path; deletion receipt: texture-hard-cutover-v0 -->

learning state: D0 schema/API decision retained here. D1 code witness now lives
in `internal/texturedoc`; D2-D4 prove the structured schema across new revision
writes, editor/user preservation, and appagent mutation. D5 locally proves the
multimedia resolver/rendering cut, with independent review still unavailable due
stalled review agents. Promote outward only when publication/export, deletion
receipts, staging proof, and any missing independent review close the product
contract.

settlement: Not met. Settlement requires deployed staging proof that structured
source/transclusion nodes are the only canonical source path, numbered refs
expand through Texture source entities, image/video/audio/PDF/transcript/Texture
sources render through the same resolver, human and agent edits preserve or
explicitly delete source refs, publication/export carries structured
transclusions, and old markdown/source-token/offset sidecar paths are deleted or
classified as noncanonical historical import only.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-texture-structured-document-transclusion-cutover-v0.md. D1-D4 are integrated and accepted; D5 multimedia sidecar/rendering implementation is locally tested but independent review stalled; current V=4. Continue with D6 publication/export projection cut so structured source_ref/source_embed + SourceEntity transclusions survive publication/export. Do not deploy, bundle broad old-path deletion, or claim mission settlement unless the paradoc is updated first.
```
