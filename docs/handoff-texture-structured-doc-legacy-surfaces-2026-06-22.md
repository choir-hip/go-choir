# Texture Structured Document Legacy Surfaces Handoff - 2026-06-22

## Purpose

This is a handoff for the next agent or human reviewer after the source-centric
`update_coagent` cutover, manual QA repair, imported source-text repair, and a
new `d@a.com` account investigation.

The important conclusion is blunt: Texture is still between two document
contracts. The intended contract is a structured, ProseMirror-compatible
`body_doc` with native source nodes and top-level `source_entities`. The actual
runtime still stores and consumes older plain/markdown-ish `content` projections
and still has tool/prompt/frontend paths that can treat markdown text as
document structure. Those older paths are now the main source of user-visible
breakage.

This document does not implement a fix. It records the current state, what was
observed, what is already partly repaired, and the work required to remove the
older paths without losing existing-account data.

Mutation class: green documentation. Protected surfaces described here include
Texture canonical writes, `body_doc`, `content`, source entity materialization,
source text/transclusion display, document stream status, prompt-bar Texture
runs, and existing user computer state.

## Current Repo State

**Updated 2026-06-22 22:00 -- post-merge state.**

Two feature branches were merged into main after this handoff was written:

- Desktop app branch (Wails v3, auth bridge, cloud mode, icon) -- 8 commits,
  touched `cmd/desktop/`, `frontend/`, `internal/auth/`, docs. No runtime or
  texture files changed.
- Maild/apps branch (multi-tenancy, Calendar, Slides, email UX) -- 4 commits,
  touched `cmd/maild/`, `internal/maild/`, `frontend/`. No runtime or texture
  files changed.

Three merge conflicts occurred in `internal/runtime/super_controller.go`,
`internal/runtime/tools_worker_update.go`, and `internal/store/texture_test.go`
because both the desktop branch and main had divergent changes to those files.
All conflicts were resolved keeping main's version (the Codex texture work),
which was correct.

**Main is now at `8bd58daa`** (merge commit). `origin/main` is still at
`8685e299` -- main has not been pushed yet.

Recent relevant commits on main:

- `8685e299 docs: hand off Texture structured document legacy surfaces`
- `ab9d305a docs: record Texture patch structure blocker`
- `5bc13afb runtime: hydrate imported source text`
- `3fc60e1b runtime: preserve coagent source text for transclusion`
- `e86673a4 docs: record imported source text gap`

The source text work means the system is no longer merely title/URL fallback in
all cases. `update_coagent` packet sources and imported content can preserve
bounded source text for transclusion. That is the correct direction. The
remaining failure is not "URL-only sources should show title and URL"; the
desired behavior is that URL-backed sources keep URL identity while displaying
the source text that researchers/importers actually read whenever that text is
available.

Recent relevant commits:

- `e86673a4 docs: record imported source text gap`
- `3fc60e1b runtime: preserve coagent source text for transclusion`
- `5bc13afb runtime: hydrate imported source text`
- `ab9d305a docs: record Texture patch structure blocker` (local only at the
  time of this handoff)

The source text work means the system is no longer merely title/URL fallback in
all cases. `update_coagent` packet sources and imported content can preserve
bounded source text for transclusion. That is the correct direction. The
remaining failure is not "URL-only sources should show title and URL"; the
desired behavior is that URL-backed sources keep URL identity while displaying
the source text that researchers/importers actually read whenever that text is
available.

## New Account Investigation: `d@a.com`

The user created a new account, `d@a.com`, to distinguish fresh-account behavior
from old account residue. Read-only Node B/product inspection mapped it to:

- owner id: `45ea050f-9824-4614-8bf0-02e595136a69`
- active VM: `vm-1f73393c7a5b70a315cac61be0e79a1e`
- VM URL: `http://10.200.235.2:8085`
- test document: `eb3ac586-ad4a-48a4-af9c-da5d50bb5e69`
- title: `whats new in music this week.texture`
- trajectory: `cb5d740a-5f29-4227-b6b6-5b0f5df9a370`
- Texture loop: `7ae64428-de7b-4f2c-b370-b76b7dca93b9`
- current revision: `342a4f9b-1d6e-42c0-a2ea-ced4a498d170`
- current version: v3

Observed backend state:

- trajectory state: `passivated`
- Texture loop state: `passivated`
- passivation reason in run metadata: `idle_deadline`
- actor sleep state: `idle`
- researcher had completed before the final Texture idle passivation
- `agent_revision_pending` was not present in the document list response

Interpretation: the backend was not still actively revising forever. It wrote
v3, then entered the resident actor park/idle wait and passivated. The UI can
still show "Revising..." during park wait, which is misleading. The old "v3
stall" is therefore partly a status semantics problem, but it also exposed a
real document-structure problem in the v3 head.

## Actual v3 Document Failure

The d@a.com v3 head had native source identity, but the structured document was
wrong:

- `source_entities`: 9
- `body_doc` source refs: 9
- `source_embed`: 0
- `heading`: 0
- `paragraph`: 1
- list blocks: 0

The one paragraph began with the source refs and then raw markdown-like text:

```text
[source refs...] # What’s new in music this weekAnchored...
```

The Trace tool call for the v3 patch showed the model using:

- `update_block_text` against a single paragraph block with a complete markdown
  article starting `# What’s new in music this week`
- multiple `insert_source_ref` edits against that same block with `offset: 0`

That precisely explains the manual QA symptoms:

- raw markdown rendered as prose because the canonical `body_doc` was a single
  paragraph containing markdown tokens;
- citation markers bunched at the beginning because the source refs were native
  nodes inserted at offset 0;
- citation markers appeared awkwardly around words because offset-based source
  insertion still allowed unsafe placement in some paths;
- source count and source titles could appear while the document itself remained
  poorly structured.

This is not only a frontend rendering issue. The canonical structured document
for that revision was malformed.

## The `content` Problem

`Revision.Content` is still stored as a markdown-ish/plain-text projection next
to the intended canonical `body_doc`. This is legacy.

The current mixed model is:

- `body_doc`: intended canonical document structure;
- top-level `source_entities`: intended canonical source identity/content
  substrate;
- `content`: persisted string projection used by older prompt, history, search,
  diff/export, and fallback rendering paths.

This creates recurring ambiguity. A change can repair `body_doc` while leaving
`content` as raw markdown, or a path can read `content` and rebuild a weak
`body_doc`. Both are signs that the system has not completed the hard cutover.

Near-term reality: deleting `content` immediately is too broad because many
surfaces still rely on it. Long-term target: `content` should stop being a
canonical writable surface. It should either be deleted from new writes or
treated as a derived cache/projection from `body_doc`, never as source of truth.

## Current Draft Patch In The Worktree

**Updated 2026-06-22 22:00 -- the draft was lost and rewritten from memory.**

The original uncommitted draft was accidentally discarded during the merge
process (`git checkout -- .` was run on the main worktree). Codex rewrote the
changes from memory. The rewritten draft is present as 11 uncommitted modified
files in `/Users/wiz/go-choir`. It implements the following:

- `structuredTextureToolDocFromMarkdown()` in `tools_texture.go` -- parses
  markdown headings/lists/paragraphs into structured `body_doc` blocks instead
  of one plain paragraph (addresses the v3 failure);
- `validateStructuredTextureEditBatch()` in `tools_texture.go` -- rejects
  multiple `insert_source_ref` edits at the same block/offset;
- `textureToolTextLooksLikeMarkdownDocument()` in `tools_texture.go` -- rejects
  `update_block_text` when content looks like a whole markdown document;
- `structuredInlineTextLen()` in `tools_texture.go` -- helper for offset-0 guard;
- `insert_source_ref` offset-0 guard: rejects placing citations before existing
  text in a non-empty block;
- Prompt overlay refinements in `revision_policy.yaml`,
  `revision_source_entities_intro.yaml`, `run_system.yaml` -- source refs belong
  after the supported sentence/clause, `insert_source_embed` is for local
  excerpts only;
- `tools_coagent.go` prompt string updated to match;
- Test updates in `texture_tool_unit_test.go` (+121 lines), `texture_test.go`,
  `texture_prompt_unit_test.go`, `agent_tools_test.go`, `runtime_test.go` --
  tests for the new validation guards and updated prompt string assertions.

**Verified 2026-06-22 22:15 -- the `park_wait` mapping IS present.**

The `park_wait_started` / `park_wait_finished` stream progress event mapping to
`synth_completed` is present in `internal/runtime/texture.go` in the rewritten
draft. The earlier claim that it was lost was incorrect. The test
`TestTextureStreamEventMapsProgressSeparatelyFromStarted` in `texture_test.go`
covers this mapping and passes with the `comprehensive` build tag.

**Known test issues:**

1. A comprehensive tagged idle-passivation test exposed a fixture/loop
   interaction after the stricter patch contract: the fixture provider kept
   writing and did not passivate within the test timeout. This must be resolved
   before the draft patch can be landed. The rewritten `runtime_test.go` only
   updates prompt string assertions -- it does NOT fix the passivation fixture.

2. Four comprehensive tests fail identically with and without the draft patch
   (pre-existing on `8bd58daa`):
   - `TestProcessorAndReconcilerProfilesDelegateToTextureOnly` -- source_entities nil
   - `TestTextureAgentRevisionRegistersMediaSourceEntities` -- source_entities nil
   - `TestTextureAgentRevisionPromotesResearcherContentRefsToSourceEntities` -- source_entities nil
   - `TestSystemPromptForUniversalWireTextureRunsRequiresArticleHead` -- missing "evidence-grounded author" policy in ordinary prompt

   These are not caused by the draft and should not block landing it.

3. Two test assertion capitalization fixes were applied to match the draft's
   yaml overlay changes (lowercase to uppercase after punctuation):
   - `texture_prompt_unit_test.go`: "do not write markdown links" -> "Do not write markdown links"
   - `runtime_test.go`: "source ids only" -> "Source ids only"

**Verification summary (2026-06-22 22:15):**

- `go build ./internal/runtime/...` -- clean
- 12 non-comprehensive texture tool tests -- all PASS
- 10 non-comprehensive texture prompt tests -- all PASS
- `TestTextureStreamEventMapsProgressSeparatelyFromStarted` (comprehensive) -- PASS
- `TestTextureStreamEventMapsTexturePassivationToSynthCompleted` (comprehensive) -- PASS

Do not treat this draft as complete. It is useful evidence for the likely
repair, not a reviewed fix. Verify the rewritten code matches the intent
described here before committing.

## Work Required Before Another Landing

### 1. Finish the immediate patch-boundary repair

Goal: prevent new Texture runs from creating one-paragraph markdown BodyDocs and
offset-0 source piles.

Required behavior:

- `rewrite_texture` may accept a full markdown-ish document, but must store a
  structured `body_doc` with heading/list/paragraph blocks.
- `patch_texture.update_block_text` must be only a single-block operation.
  Whole-document markdown should be rejected with a clear tool error.
- `patch_texture.insert_source_ref` must not place refs at the beginning of a
  non-empty block or stack several refs at the same offset.
- Citation placement should normalize away from word interiors and should be
  anchored after supported claims.
- Prompt overlays and article-source prompt fragments must reinforce the same
  source placement semantics.

Required tests:

- markdown rewrite creates structured heading/list/paragraph nodes;
- `update_block_text` rejects whole-document markdown;
- source ref insertion rejects offset 0 on non-empty blocks;
- duplicate same-offset source refs are rejected;
- existing mid-word normalization still works;
- document stream maps park wait to completion/idle state without masking real
  active revision progress.

### 2. Resolve the actor park/status semantics

The d@a.com run passivated cleanly after idle deadline, but the user saw
"Revising..." while the actor parked. The product should distinguish:

- actively revising / tool/model turn in progress;
- waiting for worker evidence;
- parked resident actor / idle;
- passivated complete.

Do not use "Revising..." for actor park wait. If a resident actor is waiting
for future coagent messages, the user-facing status should communicate idle,
waiting, or complete-with-background-listener semantics.

Acceptance evidence should include both API/runtime state and visible UI state.
Empty `worker_updates_pending` alone is not enough; the old failures could have
empty pending updates while still showing an active revision state.

### 3. Preserve and render source text, not only source identity

The source text work on `origin/main` is the right direction and must be
protected.

Required invariant:

- URL-backed sources keep `web_url` identity;
- if researcher/importer read bounded text, that text must be stored in the
  native source entity/source content payload;
- inline source/transclusion stubs should display useful bounded text when
  available;
- Source Viewer should display fuller available text/snapshot when available;
- title/URL fallback is only an honest absence-of-content state.

Do not regress to synthetic `content_item` IDs for URL-only sources. Do not
regress to clickable markdown links. Do not make "URL-only fallback" the target
behavior when source text exists.

### 4. Cut over `content` from canonical field to derived projection

This is larger than the immediate repair and should be a dedicated hard-cutover
slice.

Inventory and replace all live reads of `Revision.Content` that treat it as
canonical:

- Texture prompt construction/current-head context;
- revision history/diff/blame;
- publication/export;
- search/indexing;
- frontend fallback rendering;
- tests that seed appagent revisions with `content` but no `body_doc`;
- markdown lineage utilities that can turn raw content into weak BodyDocs.

Target:

- new appagent/Texture writes require `body_doc`;
- string content is generated from `body_doc` when a legacy consumer still needs
  text;
- fallback from `content` to `body_doc` is quarantined to explicit user-authored
  plain-text compatibility, if still required;
- product renderers prefer structured `body_doc` and do not parse source
  identity from markdown.

### 5. Existing account cleanup/backfill

Because the product is prerelease, breaking changes are allowed, but existing
accounts must not be ignored. The current failures occurred on both an existing
account and a fresh `d@a.com` account.

Required existing-account work:

- identify revisions whose `body_doc` is a single paragraph containing obvious
  markdown heading/list tokens;
- identify revisions with source refs clustered at offset 0 or the beginning of
  the first block;
- identify revisions with source entities but no source text despite a
  researcher/importer packet carrying text;
- decide whether to backfill, quarantine, or let old revisions remain historical
  while all new revisions repair forward;
- make the UI honest when opening historical malformed revisions.

Do not prove only with clean new accounts. Acceptance should include
`yusefnathanson@me.com` and a fresh account such as `d@a.com`.

### 6. Delete legacy source/update surfaces deliberately

Keep using the existing hard-cutover deletion report for the `update_coagent`
surface, but connect it to this document-level work. Remaining legacy surfaces
include:

- `research_findings` historical residue in docs/data, if any live path remains;
- legacy worker-update fields and tests that expect old packet shapes;
- `metadata.source_entities`;
- `citations_json`;
- markdown source links like `[label](source:id)`;
- source parsing from markdown content;
- old publication proposal/source metadata sidecars;
- frontend fallback rendering that silently upgrades markdown links or ignores
  structured source nodes.

The deletion rule should be: no live compatibility path unless it is explicitly
quarantined as historical read-only display and cannot mint new canonical state.

## Acceptance Target

A successful landing should prove all of the following on deployed
`https://choir.news`:

1. Deployed commit identity matches the intended commit on proxy and sandbox.
2. Fresh prompt-bar Texture on an existing account produces structured
   `body_doc`, top-level `source_entities`, and no `metadata.source_entities`.
3. Fresh prompt-bar Texture on a new account does the same.
4. Headings/lists render as structure, not literal `#`, `##`, `###`, or raw
   list markers in the body.
5. Native citation/source refs are distributed next to supported claims, not
   stacked at the beginning or end and not inserted mid-word.
6. Source Viewer shows source title/URL plus preserved source text when the
   researcher/importer had text. It shows honest fallback only when no text is
   available.
7. The UI does not keep saying "Revising..." after the Texture actor has parked
   or passivated.
8. The run trace shows canonical `coagent_source_packet.v1` packets and no
   legacy update shape.

## Suggested Next Move

Do not start by deleting `content` globally. First finish the smaller
patch-boundary/status repair so new runs stop creating malformed structured
documents. Then run a focused review. After that lands, open a separate hard
cutover for `Revision.Content` and the remaining legacy read/render paths.

**Note: main has not been pushed yet.** The merged branches (desktop + maild)
are in local main but not on origin. Push main before starting new work, or
coordinate with the user on push timing.

Recommended order:

1. ~~Review the rewritten uncommitted draft patch (`git diff` in
   `/Users/wiz/go-choir`). Verify it matches the intent described above. Pay
   attention to whether the `park_wait` stream mapping was lost.~~ **Done
   2026-06-22 22:15.** The draft matches intent. The `park_wait` mapping is
   present in `texture.go`. Two test assertion capitalization fixes applied.
2. ~~Re-implement the `park_wait_started` / `park_wait_finished` to
   `synth_completed` stream mapping if it is missing.~~ **Not needed -- already
   present.**
3. Repair the comprehensive passivation test or adjust the fixture to the new
   tool contract. (The four pre-existing comprehensive failures are not caused
   by the draft and should not block landing.)
4. Commit the runtime repair after focused tests.
5. Push main (includes the merged desktop + maild branches + this fix),
   monitor CI/deploy, verify staging identity.
6. Run deployed acceptance on `yusefnathanson@me.com` and `d@a.com`.
7. Only then start the broader `content` deletion/backfill slice.

## Residual Risk

The largest remaining risk is that agents keep adding structured fixes beside
legacy projections instead of deleting the old authority path. That creates a
system where `body_doc` looks correct in one path, `content` drives another,
and users see whichever representation leaked through. The long-term repair is
not another renderer patch; it is a hard authority cutover where one canonical
document shape exists and every other representation is derived, quarantined,
or deleted.
