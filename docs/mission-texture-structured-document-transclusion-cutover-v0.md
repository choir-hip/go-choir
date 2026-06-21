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
staging acceptance proof. Current value: 1. Accepted D7 slices have removed the
current-contract markdown source-link affordance band, Universal Wire/coagent
source-link normalization, markdown-lineage/source-repair metadata residue,
dead source-repair/source-attachment frontend affordances, publication fallback
markdown source-link upgrading, and runtime use of `metadata.source_entities` as
live source context. The remaining local deletion/classification edge is prompt
contract residue that still teaches old `patch_texture` replace/append JSON.
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
or higher if platform behavior changes.

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
refs. D5 checkpoint
records the specific sidecars/pathways repaired locally: runtime
`media_source_refs`, prompt context that prefers those refs, and frontend
media-ref synthesis/rendering outside top-level structured entities. D5
implementation removes the new-write/new-prompt path for those sidecars and
keeps legacy media-ref synthesis only for revisions without structured
`body_doc`. Markdown `[label](source:id)` parsing remains only as historical
fallback for artifacts without structured body data.

next move: commit this prompt-contract residue checkpoint, then repair
`revision_policy.yaml` and active prompt tests so Texture no longer instructs
agents to call old `replace` / `append` JSON operations after the structured
tool cutover. After independent review, run broad tests and the landing loop.
Do not claim settlement until staging proves source creation, agent/human edit
preservation, multimedia source expansion, publication/export, and
markdown-link/source-token rejection from the deployed product path.

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
  `body_doc` before forwarding to platformd.
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
before forwarding detached source entities to platformd. Regression tests cover
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
