# update_coagent Hard Cutover Legacy Deletion Report - 2026-06-22

## Current System State

The system is between two incompatible contracts. The intended contract is
source-centric: every cross-agent update is a canonical
`coagent_source_packet.v1` with typed `claims`, `sources`, `actions`,
`questions`, and `notes`; Texture cites or embeds only `packet.sources`; Super
executes only validated `kind=execution_request` packets; and Texture documents
are structured ProseMirror-compatible documents whose citation points are native
source nodes, not links or sidecars.

The actual local repo state is partially cut over. Commit
`be52b1946475c9915f25997827ebabddc8beab83` introduced the source-centric
`update_coagent` shape. Commit
`c35502b24f640e7a025804d1e4b0bf026cdaa679` repaired several validation gates:
malformed nested packets are rejected more often, legacy worker-event mirroring
is rejected before dispatch, and persistent Super no longer starts privileged
execution from ordinary `evidence_update` packets. Focused tests for those
repairs passed locally.

That is still not a full hard cutover. The store schema still contains legacy
worker-update columns and the separate `research_findings` family. Texture still
has markdown/source-link residue, plain-text body-doc fallback behavior, and
frontend markdown link rendering. Existing account data can still contain old
rows, old revisions, old queue cursors, old channel-message prose, and old
source-free Texture heads. A deletion owner should therefore treat the current
code as a partially repaired intermediate state, not as a settled foundation.

The live staging system is further behind the local branch. Node B was observed
running deployed commit `63f44e07691c7df58ceec9dd078b6e9a8be37322`, which
predates the local D9 commits. The user-visible failure on `choir.news` for
`yusefnathanson@me.com` therefore remains the acceptance target after deletion:
new prompt-bar Texture work must produce reader-facing structured documents with
native sources on the existing account, not just pass fresh local fixtures.

## Scope

This report inventories remaining legacy surfaces after the source-centric
`update_coagent` cutover work around local commit
`be52b1946475c9915f25997827ebabddc8beab83`, follow-up local repair commit
`c35502b24f640e7a025804d1e4b0bf026cdaa679`, and the remaining hard-cutover
deletion work. It is documentation-only.

Mutation class: green. Protected surfaces inspected: Texture canonical writes,
coagent update delivery, researcher/super update propagation, source
transclusion, publication export, and existing account runtime data.

Hard cutover means no live runtime compatibility for old update shapes. One-time
data migration or quarantine is allowed, but the running product must not keep
accepting, reconstructing, displaying, or relying on old source/update syntaxes.

This deletion is the fix, not cleanup. The visible staging failures (raw
markdown rendered in the document body, no source/citation points, and process
metadata leaking into reader-facing prose) are symptoms of a partial cutover:
the new source-centric `update_coagent` path was added alongside the old
`worker_updates`, `research_findings`, and markdown-source surfaces rather than
replacing them, so a packet, revision, or render call can still take the old
shape and hit an incompatibility the new code did not expect. Codex authored the
D9 cutover work and has been deletion-reticent: it adds the canonical path and
repairs validation gates, but it leaves the old surfaces live. Every retained
old surface is a divergence point where the new contract and the old behavior
can disagree. Hard cutover is therefore required to remove the divergent paths,
not merely to add a new one beside them.

## Post-Repair Review Findings

These findings were observed after reviewing local commit
`c35502b24f640e7a025804d1e4b0bf026cdaa679`. They should be handed to the next
deletion implementer as live blockers for claiming hard-cutover completion.

### P1. Packet source validation still accepts unsupported source and selector kinds

`internal/runtime/tools_worker_update.go:604` validates that `source.kind` is
non-empty and `selectors[].kind` is non-empty, but it does not validate either
against the source contract vocabulary. That leaves a path where canonical
`packet_json` can contain unsupported source kinds or selector kinds.

This matters because `internal/runtime/texture_evidence_sources.go:179` can
reinterpret source targets during Texture collation. For example, a packet with
an unsupported HTTP-backed source kind can be converted into a `content_item`
Texture entity because the URI is HTTP. That creates a mismatch between the
canonical packet's declared type and the Texture source entity actually shown to
the user.

Required update:

- Validate `packet.sources[].kind` against the same explicit source/media/code
  evidence vocabulary used by Texture source entities and the source contract.
- Validate `packet.sources[].selectors[].kind` against the source-contract
  selector vocabulary, including text, whole-resource, time, byte/page, image
  region, and future media selectors as they are formally added.
- Add negative tests for unsupported source kinds and unsupported selector
  kinds, not only missing fields.
- Decide whether unsupported but parseable URIs are rejected or explicitly
  normalized before storage; do not silently reinterpret them downstream.

### P2. Tool schema and Go validation disagree about `target.uri`

`internal/runtime/tools_worker_update.go:58` presents `target.uri` as an
optional property: the JSON schema requires `kind` and `target`, but does not
require `target.uri`. `internal/runtime/tools_worker_update.go:608` then rejects
every source without `target.uri`.

Required update:

- If every source must have a URI, make `target.uri` required in the tool schema
  and prompt examples, including command output, diffs, test runs, screenshots,
  videos, files, app-change packages, and source-service items.
- If some execution/source artifacts can be source-id/title backed without URI,
  loosen Go validation and define the required identity field per source kind.
- Do not leave this as a model-visible optional field with a runtime-only
  rejection. That causes agents to produce packets that the tool description
  appears to allow.

### P2. Non-execution packets addressed to persistent Super remain pending forever

Commit `c35502b24f640e7a025804d1e4b0bf026cdaa679` correctly prevents persistent
Super from starting privileged work on non-`execution_request` packets. The
current behavior leaves those packets in the mailbox backlog. The regression
test at `internal/runtime/update_coagent_source_packet_test.go:169` explicitly
asserts the packet remains pending.

For a hard cutover this is not enough. Existing accounts can already have
non-execution packets addressed to persistent Super. Leaving them in the live
backlog creates permanent queue residue and repeated boot/wake consideration,
even if the new filter prevents execution.

Required update:

- Quarantine, reject, or acknowledge non-execution packets addressed to
  persistent Super into an explicit non-executable receipt path.
- Make the mailbox cursor/delivery state reflect that these packets are settled
  as invalid-for-Super, not pending work.
- Add existing-account migration for already queued Super-addressed packets whose
  `packet.kind` is not `execution_request`.
- Update the regression so it asserts no privileged execution and no live pending
  backlog residue.

## Current Evidence

- Staging is still deployed at `63f44e07691c7df58ceec9dd078b6e9a8be37322`, so
  the visible `choir.news` failure is pre-local-D9 runtime behavior.
- The existing user account under inspection is
  `yusefnathanson@me.com` / owner id `5bd6de97-3b58-408c-bf89-c42c81b083de`.
- The live failing Texture seen during this mission was doc
  `08fa6a2f-d886-412d-b2ac-83fe548a9ac4`, title
  `What's new in ai now.texture`, with current revision
  `dadcc214-de23-4404-b8ac-e17e436e383c`, v3.
- That live document showed three product failures: raw markdown was visible in
  the document body, no citation/source points were visible, and the first
  paragraph was process/status metadata rather than reader-facing content.
- Node B read-only inspection confirmed the deployed checkout is still
  `63f44e07691c7df58ceec9dd078b6e9a8be37322`, and the runtime schema still has
  both `research_findings` and `worker_updates` tables.

### Manual QA re-confirm, 2026-06-22 (owner)

The same three failures were re-confirmed by direct manual QA against the
authenticated `yusefnathanson@me.com` session on `choir.news` on a fresh
prompt-bar submission:

1. Markdown rendering is broken in the document body.
2. No sources appear, even though every researcher update is supposed to carry
   `packet.sources`.
3. The first paragraph is still irrelevant process/status metadata rather than
   reader-facing content.

These are the same product failures recorded for v3 of the live document above,
re-observed on a fresh submission. That re-observation is important: it means
the failures are not stale state from one bad revision. They reproduce on new
prompt-bar input on the current staging deploy.

A fourth, wiring-level symptom was observed during the same QA pass: the
document held at v3 and showed "Revising..." for roughly a minute before the
revision loop stopped without producing a v4. The expected behavior is that a
researcher-bearing prompt-bar submission produces a stream of revisions that
each carry `packet.sources`, advancing past v3. Stalling on v3 with no further
revision indicates a wiring defect in the revision loop, not only a render or
source-collation bug. Candidate wiring defects consistent with this report:

- A researcher/super update that took the old `research_findings` or legacy
  `worker_updates` shape fails the new packet validation and is silently
  dropped instead of being presented to Texture for revision, so the loop
  believes there is nothing to integrate and stops.
- A packet addressed to persistent Super that is not `kind=execution_request`
  sits in the mailbox backlog forever (P2 below), so a revision dependent on
  Super execution never advances.
- The `plainTextStructuredTextureDoc` fallback consumes a revision whose body
  is raw markdown and produces a structured doc whose only content is that
  markdown as text (item 5 below), so downstream revision passes see an
  already-"structured" doc and do not re-process it.

The wiring-stall finding should be treated as a first-class acceptance target
alongside the three render findings: after cutover, a prompt-bar submission on
the existing account must advance past v3 and each revision must carry
`packet.sources`.

## Definition Of Legacy For This Cutover

Legacy means any product path that does one of these:

- accepts old `update_coagent` fields such as `findings`, `evidence_ids`,
  `evidence`, `artifacts`, `refs`, `tests`, `proposals`, or
  `capability_requests`;
- reconstructs a source packet from old fields at read time;
- lets source identity exist as markdown links, citations JSON, metadata
  sidecars, `media_source_refs`, or top-level source entities without
  structured `body_doc` source nodes;
- uses `research_findings` as a live researcher-to-Texture contract instead of
  `update_coagent` source packets;
- renders or stores model-authored markdown as canonical Texture structure when
  the user-facing document should be structured ProseMirror-compatible JSON;
- treats legacy channel-message prose as current source substrate;
- proves behavior only with clean new accounts while old account/runtime stores
  keep legacy rows that product projections still consume.

## Legacy Code To Delete Or Replace

### 1. `worker_updates` stores legacy columns beside `packet_json`

`internal/store/store.go:354` still creates `worker_updates` with `kind`,
`summary`, `packet_json`, and `content` columns. `internal/store/store.go:642`
still migrates `kind`, `summary`, and `packet_json` into old databases.
`internal/store/store.go:2389` still writes `kind`, `summary`, and `content`
alongside `packet_json`.

`internal/store/store.go:2656` is the dangerous compatibility shim:
`scanWorkerUpdate` unmarshals `packet_json`, but if the packet is empty it
reconstructs `SchemaVersion`, `Kind`, and `Summary` from the legacy columns at
`internal/store/store.go:2693`.

Delete plan:

- Introduce a one-time storage migration that either fills canonical
  `packet_json` from deterministic old rows or marks the row as legacy
  audit-only outside the live mailbox path.
- After the migration, remove live reconstruction from `scanWorkerUpdate`.
  Empty or invalid `packet_json` must be an error for live delivery reads.
- Drop or rename legacy columns. If a human projection is still useful, store it
  as explicitly derived `human_projection` and regenerate it from `packet_json`
  during migration; do not preserve old free-text `content` as source truth.
- Update `internal/store/store_test.go:238` so it no longer proves migration
  from old `findings_json`/`evidence_ids_json` worker update tables into a live
  compatible table. Replace it with a hard-cutover migration/quarantine test.

### 2. `research_findings` remains a live schema and trace projection family

`internal/store/store.go:337` creates `research_findings` with
`findings_json`, `evidence_ids_json`, `notes_json`, `questions_json`, and
`content`. The type still exists at `internal/types/evidence.go:24`.
Read/write APIs remain in `internal/store/store.go:1822` and
`internal/store/store.go:2200`. Trace still loads and exposes these rows through
`internal/runtime/api_trace.go:577`.

The cutover says researcher output is a `CoagentSourcePacketPayload` through
`update_coagent`, not a separate researcher-specific dispatch substrate.

Delete plan:

- Stop writing `research_findings` in live researcher flows.
- Remove `RequireResearchFindings` verifier requirements and replace them with
  "requires update_coagent packet with packet.sources" checks.
- Keep old `research_findings` only as immutable historical Trace/audit data, or
  move it into an archive table excluded from Texture source collation.
- Delete product-path tests that assert `research_findings` as a success
  requirement. Replace them with packet/source assertions.
- If Trace needs to display old findings, label them as historical legacy
  findings and never expose them as native Texture citations.

### 3. Test-only research/update endpoints keep old workflow concepts alive

`internal/runtime/api.go:1759` registers `/api/test/texture/research-findings`
and `/api/test/texture/worker-update` when test APIs are enabled. The handlers
start runs and invoke `update_coagent` at `internal/runtime/texture.go:1956` and
`internal/runtime/texture.go:1961`.

The worker-update test endpoint can remain only if it accepts exactly the
canonical packet surface. The research-findings endpoint should be deleted or
renamed because the current name preserves the old mental model.

Delete plan:

- Delete `/api/test/texture/research-findings`.
- Keep at most one local-only `/api/test/texture/coagent-source-packet`
  endpoint that accepts canonical packet fields and no researcher-specific
  names.
- Update tests at `internal/runtime/texture_test.go:5533` and
  `internal/runtime/api_test.go:2709`.

### 4. Markdown source-link syntax is still recognized in prompt/tests

`internal/runtime/texture.go:47` still defines
`textureInlineSourceRefRE` for `[label](source:id)`. Tests still create a
current revision containing `[the clip](source:src-youtube-demo)` at
`internal/runtime/texture_prompt_unit_test.go:449`, then assert that the prompt
mentions "legacy inline source ref" at `internal/runtime/texture_prompt_unit_test.go:463`.

Frontend tests also keep an explicit old-syntax regression at
`frontend/tests/texture-source-entities.spec.js:125`.

Delete plan:

- Remove `textureInlineSourceRefRE` from live runtime code.
- Delete prompt concepts that preserve "legacy inline source ref". Existing old
  source links should be migrated once into structured source_ref nodes or
  quarantined as legacy text; they should not be a live prompt obligation.
- Keep only rejection/quarantine tests for `[label](source:id)`.
- Ensure models never see `[label](source:ENTITY_ID)` as something to preserve.
  They should see structured source node IDs and `source_entity_id` values.

### 5. Plain text fallback creates structured docs that preserve raw markdown

`internal/store/texture_structured_revision.go:130` creates a
`plainTextStructuredTextureDoc` when a revision lacks `body_doc`. This wraps the
entire `content` string as paragraph text. If content contains `#`, `##`,
`**bold**`, numbered markdown, or source-ish prose, those bytes become visible
document text. This matches the staging screenshot where `# What's new in AI
now?`, `## The short version`, and `**...**` were displayed literally.

`frontend/src/lib/TextureEditor.svelte:209` renders structured `body_doc` when
present, otherwise falls back to `renderMarkdownBlocks`. If the backend has
already converted markdown bytes into a plain-text structured doc, the frontend
correctly renders literal markdown because the canonical structure says those
characters are text.

Delete plan:

- Stop creating canonical `body_doc` by wrapping arbitrary markdown/plain text.
- Require Texture write tools to write structured operations/body_doc, or add a
  server-side markdown-to-structured parser as a transitional one-time importer
  only for imports and existing-account migration.
- For appagent revisions, reject canonical writes whose projected document body
  contains markdown control tokens that should have been structure, unless those
  tokens are inside code/preformatted nodes.
- Existing raw-markdown revisions should be migrated into structured nodes or
  left as historical legacy revisions with a new corrected head revision.

### 6. Frontend markdown renderer still supports ordinary clickable links

`frontend/src/lib/texture-source-renderer.ts:481` renders inline markdown, and
`frontend/src/lib/texture-source-renderer.ts:489` upgrades ordinary markdown
HTTP links to clickable anchors. The user contract says Texture sources are
transcluded and clickable links should not appear as a substitute for native
source/citation objects.

Delete plan:

- For Texture document/publication surfaces, stop using the generic markdown
  inline renderer for user-facing body content.
- Delete HTTP-link upgrading in Texture rendering paths. If other apps need it,
  move it out of Texture-specific renderers.
- Preserve only `texture:` transclusion refs if they remain a first-class native
  Texture node/transclusion, not as markdown links.

### 7. Source identity sidecar guards exist, but old paths still need full deletion

Good current guards exist in `internal/store/texture_structured_revision.go:80`,
which rejects `citations_json`, and `internal/store/texture_structured_revision.go:99`,
which rejects legacy metadata source sidecars such as `source_entities` and
`media_source_refs`.

Residual deletion work:

- Delete old tests/fixtures that still create metadata `media_source_refs`,
  except rejection tests.
- Delete publication/export parsing of source markdown if any remains; source
  metadata must derive only from structured `body_doc` source_ref/source_embed
  nodes plus validated top-level `source_entities`.
- Keep top-level `source_entities` only as canonical entity data paired with
  structured document nodes. It is not allowed to stand alone.

### 8. Runtime fallback synthesizers still convert legacy-ish refs into sources

`internal/runtime/researcher_checkpoint_fallback.go:98` synthesizes fallback
`update_coagent` args after research tools return without a packet. It builds
`refs` strings and converts them through `coagentSourcesFromRefs` at
`internal/runtime/researcher_checkpoint_fallback.go:120`.

`internal/runtime/delegate_worker_update_fallback.go:354`,
`internal/runtime/delegate_worker_update_fallback.go:437`, and
`internal/runtime/delegate_worker_update_fallback.go:499` do similar conversion
from old `evidenceIDs`, `refs`, and `artifacts` string lists.

This is acceptable only if these are runtime-created canonical packets with
typed `packet.sources`. It is not acceptable if `coagentSourcesFromRefs` remains
a general compatibility parser for arbitrary old update payloads.

Delete plan:

- Rename these helpers away from `refs` and toward explicit source constructors:
  `coagentSourceFromTraceEvent`, `coagentSourceFromRun`, `coagentSourceFromDiff`,
  `coagentSourceFromCommand`, `coagentSourceFromFileArtifact`.
- Delete generic `coagentSourcesFromRefs` parsing for arbitrary `key:value`
  strings.
- Require fallback-generated packets to include typed source kinds, target URIs,
  selectors, and evidence states directly.

### 9. Super execution gating is repaired for privilege, but not for queue settlement

The D9 review found that persistent Super wake/injection could consume
non-`execution_request` packets. Commit
`c35502b24f640e7a025804d1e4b0bf026cdaa679` added
`persistentSuperExecutableUpdate` at `internal/runtime/super_controller.go:256`
and vmctl mirror validation at `internal/runtime/tools_vmctl.go:1344`.

That repair prevents privileged execution from starting on non-execution
packets, but it does not settle invalid-for-Super packets. They remain pending
mailbox backlog rows, which is a hard-cutover residue for existing accounts.

Delete/settlement plan:

- Keep the privilege gate, but add queue settlement: non-`execution_request`
  packets addressed to persistent Super should be rejected, quarantined, or
  acknowledged into a non-executable receipt path.
- Add regression coverage proving Super ignores or reports non-execution packets
  without starting privileged execution and without leaving live pending backlog
  residue.
- Add existing-account checks for already queued Super-addressed packets that
  are not `execution_request`; migrate or quarantine before deploy so old
  queued rows do not keep waking or appearing as pending after the new code
  lands.

### 10. Texture revision metadata still exposes worker-update process status

`internal/runtime/runtime.go:3130` defines `textureWorkerUpdateMetadata` with
`ContentPreview`. `internal/runtime/runtime.go:3140` writes
`worker_updates_consumed`, `worker_updates_skipped`, and
`worker_updates_pending` into revision metadata.

This metadata should not appear in reader-facing document body, but the live
screenshot shows process/status material entering the document itself. That is
mainly a prompt/tool behavior issue, but metadata preview fields can also tempt
renderers or prompts to leak process text.

Delete plan:

- Keep process state in Trace/controller state, not canonical Texture document
  body.
- If revision metadata retains worker update receipts, store update IDs and
  source packet IDs, not content previews.
- Ensure Texture prompts receive structured packet JSON and source entities, not
  free-text "Recent addressed worker messages" as the primary source substrate.

## Existing Account Cutover Requirements

Hard cutover must update existing accounts and runtime stores, not just new
accounts. The deploy plan needs an explicit data phase over every account-owned
runtime store, including `yusefnathanson@me.com`.

Required read-only audit before migration:

- Count `worker_updates` rows where `packet_json` is empty, `{}`, invalid, or
  missing required packet fields.
- Count `worker_updates` rows that still rely on old columns or old table shape
  (`findings_json`, `evidence_ids_json`, `artifacts_json`, `refs_json`,
  `tests_json`, `proposals_json`).
- Count `research_findings` rows by owner and trajectory.
- Count `texture_revisions` rows where `body_doc_json` is empty and `content`
  contains markdown structure tokens.
- Count `texture_revisions` rows where `source_entities_json` is empty but
  revision metadata/channel history indicates researcher/source activity.
- Count `texture_revisions` rows carrying legacy `citations_json` or metadata
  source sidecars.
- Count queued pending coagent packets addressed to persistent Super whose
  `packet.kind` is not `execution_request`.

Migration/quarantine policy:

- Valid `packet_json` rows stay live after validation and normalization.
- Deterministically convertible old rows may be converted once into canonical
  `coagent_source_packet.v1`, but only when the migration can produce explicit
  source IDs, kinds, targets, and evidence state without model inference.
- Non-convertible old rows become historical/audit-only records. They must not
  remain in the live mailbox backlog, Texture source collation, or Super wake
  path.
- Old `research_findings` are not native source packets. They can be archived
  for Trace display, but existing Texture documents need new source packets or
  new corrected revisions if the product should show citations.
- Old Texture revisions with raw markdown should remain in version history as
  historical revisions or be transformed by a deterministic structured importer.
  In either case, the current head for active documents should be a corrected
  structured revision if the document remains user-visible.
- Immutable event logs and channel messages should not be rewritten. Instead,
  product projections must stop treating legacy message prose as current source
  state.

What may not get updated automatically:

- Historical Trace events and old channel messages, because they are audit
  facts.
- Published snapshots already signed on the old projection, unless the platform
  creates a new publication version or marks the old publication as legacy.
- Old Texture revisions whose source identity cannot be reconstructed
  deterministically. These should not gain fabricated sources.
- Research claims from old rows where only snippets/prose remain and no durable
  source target exists.

What must get updated for existing accounts:

- Live mailbox/backlog tables used by Texture and Super.
- Active Texture document heads for documents that users will continue editing
  or publishing.
- Publication/source tabs and source panels for current documents.
- Controller checkpoints and pending-delivery cursors so old queued rows do not
  get redelivered under the new semantics.
- Acceptance fixtures must include the existing `yusefnathanson@me.com` account
  path, not only synthetic new users.

## Replacement Architecture Target

The live contract after deletion should be:

- `update_coagent` accepts only:
  `schema_version`, `kind`, `summary`, `claims`, `sources`, `actions`,
  `questions`, `notes`, plus addressing fields `agent_id` and `channel_id`.
- The runtime validates nested packet objects in Go, not only via JSON Schema
  metadata.
- Texture source collation reads only `packet.sources`.
- Texture canonical writes persist structured `body_doc` plus validated
  `source_entities`; source references are document nodes, not markdown links.
- Super execution starts only from validated `kind=execution_request` packets
  with executable actions.
- Command outputs, diffs, tests, screenshots, videos, artifacts, and app change
  packages are represented as `packet.sources`.
- User-facing Texture body text never includes checkpoint/status metadata unless
  the user explicitly asks for a process report document.

## Acceptance Criteria

Code/deletion proof:

- `rg "findings_json|evidence_ids_json|artifacts_json|refs_json|tests_json|proposals_json" internal frontend` returns only migration archive code or
  rejection tests.
- `rg "\\]\\(source:|source:ENTITY_ID|metadata\\.source_entities|media_source_refs|citations_json" internal frontend` returns only rejection tests and
  historical migration code.
- `worker_updates` live reads fail on empty/invalid `packet_json`; they do not
  reconstruct packets from `kind`/`summary`.
- No live route named `/api/test/texture/research-findings` remains.
- Runtime/source tests prove `update_coagent` rejects legacy top-level fields and
  invalid nested packet objects.

Existing-account proof:

- On staging after deploy, query the active account runtime store and show zero
  live mailbox rows with invalid/empty packet JSON.
- Open `choir.news` in the authenticated `yusefnathanson@me.com` session and
  create/revise a Texture from the prompt bar.
- Confirm the first revision is reader-facing, not process metadata.
- Confirm researcher updates include `packet.sources` — sources must appear on
  every researcher update, not only on some revisions.
- Confirm the revision loop advances past v3 for a researcher-bearing
  prompt-bar submission; it must not stall on "Revising..." and stop without
  producing a subsequent revision.
- Confirm Texture renders citation points/source transclusions from native
  source nodes by v1/v2, not ordinary clickable links and not a source ledger in
  document prose.
- Confirm markdown headings/bold/lists render as structure or are absent from
  canonical output; raw `#`, `##`, and `**` should not appear as visible prose
  unless intentionally in code/preformatted nodes.
- Confirm persistent Super packets with non-`execution_request` kinds do not
  start privileged execution.

## Recommended Next Move

Wait for the active D9 implementation repair to finish and receive independent
review. Then make the next checkpoint a small documentation/problem commit that
records this hard-cutover deletion inventory in the mission ledger. After that,
implement deletion in this order:

1. Storage migration/quarantine for existing account data.
2. Removal of live compatibility shims from `worker_updates` and
   `research_findings`.
3. Runtime prompt/tool deletion of source-link and raw markdown fallback paths.
4. Frontend/publication deletion of clickable-link and version-history-under-body
   source substitutes.
5. Staging acceptance against the existing `yusefnathanson@me.com` account.
