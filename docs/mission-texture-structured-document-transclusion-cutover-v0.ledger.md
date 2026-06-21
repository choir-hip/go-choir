# Ledger: Texture Structured Document And Transclusion Cutover v0

## 2026-06-21 - Pass 0 - Paradoc Creation

Claim: a dedicated Parallax mission is needed for the structured-document and
source/transclusion hard cutover because the existing Texture hard-cutover
mission primarily tracks ontology/name residue, while this work changes the
canonical document substrate.

Move: construct. Created the paradoc with prior-art references, owner direction,
target model, model-facing edit API, multimedia requirements, domain ramp,
Parallax State, and suggested goal string.

Expected delta V: initialize mission variant at 10 open obligations.

Actual delta V: initialized; no execution obligations discharged.

Receipt: `docs/mission-texture-structured-document-transclusion-cutover-v0.md`.

Open edge: D0 still must decide raw ProseMirror vs Tiptap, exact schema/storage,
and deletion targets before runtime mutation.

## 2026-06-21 - Pass 1 - Thread Review Default

Claim: D0 should run in a fresh Codex thread because the first architecture
decision benefits from a clear-context observer and the Parallax skill now
defaults nontrivial prover/review shifts to Codex thread tools when available.

Move: construct. Updated the paradoc evidence packet, next move, and suggested
goal string so the new thread performs the D0 architecture/review pass and
returns a verdict before the implementing thread proceeds.

Expected delta V: 0 on product obligations; improves the observer for the
schema decision obligation.

Actual delta V: 0. D0 remains open.

Receipt: `docs/mission-texture-structured-document-transclusion-cutover-v0.md`.

Open edge: create the Codex thread and record its id/verdict.

## 2026-06-21 - Pass 2 - D0 Review Thread Spawned

Claim: The D0 architecture/review pass now has a fresh Codex thread handle, so
the implementing thread can proceed by reading the independent verdict instead
of reusing its own context.

Move: construct. Created Codex thread
`019eeb34-2f8a-7883-b4e2-a64e0588aad2` in the `/Users/wiz/go-choir` local
project with the paradoc Suggested Goal String and D0 review instructions.

Expected delta V: 0 on product obligations; converts the observer setup edge
from "create thread" to "consume verdict."

Actual delta V: 0. D0 remains open until the review thread reports its verdict.

Receipt: Codex thread `019eeb34-2f8a-7883-b4e2-a64e0588aad2`.

Open edge: read or receive the review verdict, update Parallax State, then
decide whether implementation can begin or the schema conjecture must be
revised.

## 2026-06-21 - Pass 3 - D0 Architecture Review Verdict

Claim: D0 can settle the schema direction without runtime mutation by inspecting
current Texture write/render/source/publication paths and deciding which old
substrates must be deleted before implementation proceeds.

Move: probe + settle. Read the paradoc, ledger, `AGENTS.md`, Texture invariants,
Texture store/API/tool/frontend/source/publication code, and official
ProseMirror/Tiptap prior-art docs. Recorded the D0 architecture decision in the
paradoc.

Expected delta V: -1 for schema decision/source target/edit API direction; no
runtime obligations discharged.

Actual delta V: -1. Current V=9. Verdict `revise_before_continue`: proceed only
after a Problem Documentation First checkpoint, then implement the structured
schema/write-path cutover. Runtime behavior unchanged.

Receipt:
`docs/mission-texture-structured-document-transclusion-cutover-v0.md#d0-architecture-decision---2026-06-21`;
code evidence includes `internal/store/texture.go:52`,
`internal/runtime/texture.go:199`, `internal/runtime/tools_texture.go:73`,
`frontend/src/lib/texture-markdown-serializer.ts:10`,
`frontend/src/lib/texture-source-renderer.ts:486`, and
`internal/platform/publication_document.go:276`.

Open edge: D1/D2 implementation must first commit a behavior-problem checkpoint:
canonical Texture writes still accept markdown-ish content strings and sidecar
source metadata, so source entities can exist without being document nodes.

## 2026-06-21 - Pass 4 - Problem Checkpoint Authored

Claim: The first post-D0 commit must be a Problem Documentation First checkpoint
before runtime mutation.

Move: construct. Added `Problem Checkpoint - Canonical Body Is Still
Markdown-ish - 2026-06-21` to the paradoc, naming the behavior problem, D0 code
evidence, conjecture delta, rollback path, and heresy delta. Updated the
Suggested Goal String from "run D0" to "commit checkpoint, then implement D1."

Expected delta V: 0 on product obligations; satisfies the documentation-first
precondition for D1/D2.

Actual delta V: 0. Current V remains 9.

Receipt: `docs/mission-texture-structured-document-transclusion-cutover-v0.md`.

Open edge: commit this checkpoint before any runtime/schema behavior change.

## 2026-06-21 - Pass 5 - D1 Implementation Delegated

Claim: D1 can proceed as an additive implementation slice in an isolated worktree
without changing production Texture write behavior.

Move: construct + shift. Created a Codex worktree implementation thread with the
D1 goal and scoped it to Go structs/validators, in-memory projection/rendering,
and focused tests. The thread must commit its work and report SHA/tests/risks
before integration.

Expected delta V: 0 now; potential -1 when D1 is implemented and reviewed.

Actual delta V: 0. Current V remains 9.

Receipt: pending worktree id `local:d0c0fc69-81ca-4489-83ba-a7146b0c2de8`.

Open edge: wait for the worktree thread result, then spawn independent review
before integrating D1.

## 2026-06-21 - Pass 6 - D1 Internal Structured Doc Spike

Claim: D1 can decrease the mission variant without production write-path risk by
building a Choir-owned, Go-validated in-memory StructuredTextureDoc/SourceEntity
schema and projection package that makes source refs/entities first-class nodes.

Move: construct + probe. Added `internal/texturedoc` with StructuredTextureDoc
v1 and SourceEntity v1 structs, validators for the D0 node/mark vocabulary,
strict source target/selector/display/evidence/open-surface validation using the
existing source contract package where appropriate, legacy source syntax
rejection, source node/entity resolution, and a numbered projection renderer for
`source_ref` and `source_embed`.

Expected delta V: -1 for D1 internal parser/renderer spike; no D2 write-path,
editor, publication, deploy, or old-path deletion obligations discharged.

Actual delta V: -1. Current V=8. D1 is implemented additively and not wired into
production Texture writes.

Receipt: `internal/texturedoc/schema.go`, `internal/texturedoc/projection.go`,
`internal/texturedoc/schema_test.go`.

Evidence: `nix develop -c go test ./internal/sourcecontract ./internal/texturedoc`
passed.

Open edge: independent D1 review, then D2 must choose and document the exact
TextureRevision v2 storage/projection cut before changing canonical Texture
writes.

## 2026-06-21 - Pass 7 - D1 Review P1 Image Region Repair

Claim: The D1 reviewer correctly found that the internal structured-doc witness
did not admit the paradoc-promised `image_region` selector kind, so D1 was not
ready to integrate until the shared enum source and texturedoc validator
accepted image-region source entities.

Move: construct + probe. Added `image_region` to
`internal/sourcecontract/source_contract_schema.json`, Go selector constants,
sourcecontract normalization tests and matrix data, regenerated
`frontend/src/lib/source-contract.generated.ts`, and added texturedoc validation
coverage proving an image source entity with an `image_region` selector is
accepted.

Expected delta V: 0 on mission obligations; repair D1 reviewer P1 without
broadening into D2.

Actual delta V: 0. Current V remains 8. D1 witness is ready for independent
re-review; production Texture writes remain untouched.

Receipt: `internal/sourcecontract/source_contract_schema.json`,
`internal/sourcecontract/selector.go`, `internal/texturedoc/schema.go`,
`internal/texturedoc/schema_test.go`, and
`frontend/src/lib/source-contract.generated.ts`.

Evidence: `nix develop -c go test ./internal/sourcecontract ./internal/texturedoc`
and `git diff --check`.

Open edge: independent D1 re-review, then D2 must choose/document the
TextureRevision v2 storage/projection cut before changing canonical writes.

## 2026-06-21 - Pass 8 - D1 Re-review P1 Source Embed Leaf Repair

Claim: The D1 re-review correctly found that `source_embed` validation accepted
hidden child content/text/marks after checking source identity and display mode,
which could let legacy source text bypass the intended text-node syntax
rejection.

Move: construct + probe. Updated `internal/texturedoc/schema.go` so
`source_embed` must be a leaf block with empty `Content`, empty `Text`, and no
`Marks`. Added regression coverage for hidden child content containing
`{{source:hidden}}`, direct text, and marks on `source_embed`.

Expected delta V: 0 on mission obligations; repair D1 reviewer P1 without
broadening into D2.

Actual delta V: 0. Current V remains 8. D1 witness is ready for independent
re-review; production Texture writes remain untouched.

Receipt: `internal/texturedoc/schema.go`,
`internal/texturedoc/schema_test.go`.

Evidence: `nix develop -c go test ./internal/sourcecontract ./internal/texturedoc`
and `git diff --check`.

Open edge: independent D1 re-review, then D2 must choose/document the
TextureRevision v2 storage/projection cut before changing canonical writes.

## 2026-06-21 - Pass 9 - D1 Final Re-review Accepted And Integrated

Claim: D1 is acceptable as an additive internal witness after the image-region
and source-embed leaf repairs, but it does not yet repair production Texture
canonical write behavior.

Move: probe + construct. Integrated the accepted D1 commit range into `main`
and reran focused tests on the integrated state. The final independent D1
re-review returned `accept`, with no findings, and explicitly authorized
integration through the source-embed leaf repair commit.

Expected delta V: 0; D1 already accounted for the -1 variant decrease, and this
pass only raises the evidence class for the integrated branch state.

Actual delta V: 0. Current V remains 8. D2 is now the active next write-path
cutover obligation.

Receipt: commits `9b2b7dff`, `32a238c2`, and `d730b06f` on `main`;
final D1 review thread `019eeb4f-09ef-7543-8297-b30d9679d9a1`.

Evidence: `nix develop -c go test ./internal/sourcecontract ./internal/texturedoc`
and `git diff --check HEAD~3..HEAD` passed on the integrated state.

Open edge: D2 must choose/document the exact `TextureRevision v2`
storage/projection cut before changing canonical Texture writes.

## 2026-06-21 - Pass 10 - D2 Storage/Projection Cut Chosen

Claim: D2 can mutate the first production write boundary only after the paradoc
names the exact TextureRevision v2 storage/projection cut, protected surfaces,
evidence class, rollback, and heresy delta.

Move: construct. Added `D2 Storage/Projection Cut - 2026-06-21` to the paradoc
and rewrote the Parallax State from "choose the DB cut shape" to "implement the
chosen cut." The chosen slice stores canonical `body_doc_json` and
`source_entities_json`, keeps `content` as a derived body text projection for
D2 compatibility, validates new revisions through `internal/texturedoc`, migrates
existing Dolt workspaces through `bootstrapTexture`, and signs the structured
substrate in a v2 revision hash payload.

Expected delta V: 0 on product obligations; satisfies the red-surface
precondition for D2 runtime mutation.

Actual delta V: 0. Current V remains 8. Runtime behavior unchanged by this pass.

Receipt: `docs/mission-texture-structured-document-transclusion-cutover-v0.md`.

Open edge: implement the D2 write-path cutover in the assigned worktree only,
without bundling frontend editor saves, publication/export, staging/deploy, or
broad old-path deletion.

## 2026-06-21 - Pass 11 - D2 Local Write-Path Cutover

Claim: The first production write boundary can be cut over without bundling
frontend editor saves, publication/export, staging/deploy, or broad old-path
deletion by making `Store.CreateRevision` the uniform structured validation and
projection gate.

Move: construct + probe. Added TextureRevision v2 fields to `types.Revision`,
fresh DDL, existing-workspace `bootstrapTexture` migrations, store-side
canonicalization from plain text to a simple structured doc, explicit
`body_doc`/`source_entities` validation through `internal/texturedoc`, public and
internal revision response fields, and a `rev2` hash payload that signs
`body_doc_json`, `source_entities_json`, derived projection, provenance, and the
parent hash. Added focused store/type/runtime tests for persistence, migration,
hash coverage, API round trip, and legacy syntax rejection.

Expected delta V: -1 for the D2 write-path obligation, provided independent
review accepts that the cut does not smuggle source identity through projection,
citations, or metadata sidecars.

Actual delta V: -1 locally. Current V=7. D2 is ready for independent review but
not mission settlement.

Receipt: `internal/types/texture.go`, `internal/types/texture_revision_hash.go`,
`internal/store/texture.go`, `internal/store/texture_structured_revision.go`,
`internal/runtime/texture.go`, `internal/store/texture_test.go`,
`internal/runtime/texture_structured_revision_test.go`, and
`internal/types/texture_revision_hash_test.go`.

Evidence:
`nix develop -c go test ./internal/types ./internal/texturedoc ./internal/store`;
`nix develop -c go test ./internal/runtime -run TestTextureRevisionAPIAcceptsStructuredBodyAndRejectsLegacySourceSyntax`;
`git diff --check`.

Open edge: independent D2 review, then later D3/D4/D6/D7 cuts for editor
source-ref atom preservation, Texture agent structured operation tools,
publication/export structured projection, broad old-path deletion, staging proof,
and mission settlement.

## 2026-06-21 - Pass 12 - D2 Side-Channel Blocker Decision

Claim: Preliminary D2 review correctly found that the local write-path cutover
was incomplete because source identity could still enter new revisions through
legacy sidecars, including sidecars parallel to otherwise structured fields.

Move: probe + construct. Recorded the blocker and repair decision in the paradoc.
The repair will reject/defer non-empty `citations_json` and metadata source
sidecars (`source_entities`, `media_source_refs`, `source_gaps`,
`source_repair_resolutions`, `source_attachment_manifest`,
`source_ref_normalization`) unconditionally at `Store.CreateRevision`. Source
identity must live only in `BodyDoc` source nodes plus top-level
`SourceEntities`; parallel legacy metadata is rejected even when structured
fields are present. Source repair and source artifact attachment APIs should
return clear invalid-revision errors rather than persisting legacy sidecar-only
source state.

Expected delta V: 0 now; the prior local D2 -1 is reopened until the side-channel
repair and tests land.

Actual delta V: +1 reopen. Current V=8 pending repair. Runtime behavior unchanged
by this documentation pass.

Receipt: `docs/mission-texture-structured-document-transclusion-cutover-v0.md`.

Open edge: implement the side-channel repair in the assigned worktree, add
focused regressions, rerun focused tests plus `git diff --check`, and commit.

## 2026-06-21 - Pass 13 - D2 Accepted And Integrated

Claim: The repaired D2 server write-boundary cut is acceptable when independent
review verifies that source identity cannot enter new revisions through
`content`, `citations_json`, or legacy source metadata sidecars in D2 scope.

Move: probe + construct. Independent D2 re-review returned `accept` with no
findings after the side-channel repair. Root integrated the accepted D2 commit
range as `d54458b5`, `e60f6523`, and `9d50485f`, then reran the focused tests
on the integrated branch.

Expected delta V: -1 for the D2 canonical revision write-path obligation.

Actual delta V: -1. Current V=7. D2 is integrated and accepted, but this is not
mission settlement.

Receipt: commits `d54458b5`, `e60f6523`, and `9d50485f` on `main`; D2
re-review thread `019eeb72-a271-72a2-a18d-7f9cb66c4de4`.

Evidence:
`nix develop -c go test ./internal/types ./internal/texturedoc ./internal/store`;
`nix develop -c go test ./internal/runtime -run TestTextureRevisionAPIAcceptsStructuredBodyAndRejectsLegacySourceSyntax`;
`git diff --check HEAD~3..HEAD`.

Open edge: D3 editor/user path. Human edits must preserve source refs as atom
nodes, support intentional insertion/removal, and render numbered source points
from structured `BodyDoc` / `SourceEntities` without clickable-link source
syntax. Later D4/D5/D6/D7 cuts still need Texture agent operations, multimedia
resolver, publication/export, old-path deletion, staging deploy, and Comet
product proof.

## 2026-06-21 - Pass 14 - D3 Editor/User Path Cut Documented

Claim: D3 can mutate the protected human editor path only after the paradoc
names the behavior problem, exact editor/user cut, protected surfaces,
admissible evidence, rollback path, and heresy delta.

Move: construct. Added `D3 Editor/User Path Cut - 2026-06-21` to the paradoc and
rewrote the Parallax State from "D3 next move" to "implement the bounded D3
editor/user path." The documented problem is that D2 made server writes
structured, but the editor still loads, edits, renders, and saves through
markdown-ish DOM/projection paths that can serialize source refs as clickable
`[label](source:id)` links or drop refs during ordinary text edits. The D3 slice
must read/render `body_doc` plus top-level `source_entities`, save structured
`BodyDoc` source_ref atom nodes, allow intentional source-ref insertion/removal,
and preserve existing source-opening affordances without clickable source-link
syntax.

Expected delta V: 0 on product obligations; satisfies the red-surface
precondition for D3 frontend/editor mutation.

Actual delta V: 0. Current V remains 7. Runtime and frontend behavior unchanged
by this pass.

Receipt:
`docs/mission-texture-structured-document-transclusion-cutover-v0.md`.

Open edge: inspect the current frontend editor/source renderer and API contract,
then implement the smallest D3 editor/user path without bundling Texture agent
operation tools, publication/export, staging/deploy, or broad old-path deletion.

## 2026-06-21 - Pass 15 - D3 Editor/User Path Implementation Candidate

Claim: The bounded D3 editor/user path can preserve structured source identity
without reintroducing clickable source links if the editor renders `body_doc`
source refs as native citation atoms, serializes DOM atoms back to
StructuredTextureDoc `source_ref` nodes, and writes top-level `source_entities`
instead of metadata sidecars.

Move: construct. Added a frontend structured editor document codec, taught the
Texture editor to prefer revision `body_doc` for rendering/loading, serialize
editor DOM back to `body_doc` on input/save, project content from that structured
doc, send top-level `source_entities` through the revision API, and remove new
editor writes of `metadata.source_entities`. Updated source-state/rendering
helpers to prefer top-level revision source entities and understand D1-shaped
`display`, `target.kind`, `target.uri`, and `evidence.open_surface` fields.

Expected delta V: 0 until independent D3 review and root integration; then -1
for the human editor path if accepted.

Actual delta V: 0. Current V remains 7 pending review. This is not mission
settlement and does not cover Texture agent operation tools, publication/export,
multimedia embedding, broad legacy deletion, or staging proof.

Receipt:
`frontend/src/lib/texture-structured-editor-doc.ts`;
`frontend/src/lib/TextureEditor.svelte`;
`frontend/src/lib/texture.js`;
`frontend/src/lib/texture-source-state.ts`;
`frontend/src/lib/texture-source-renderer.ts`;
`frontend/tests/texture-structured-editor-doc.spec.js`.

Evidence:
`cd frontend && npx playwright test tests/texture-structured-editor-doc.spec.js`;
`cd frontend && npm run build`;
`nix develop -c go test ./internal/runtime -run TestTextureRevisionAPIAcceptsStructuredBodyAndRejectsLegacySourceSyntax`;
`git diff --check`.

Open edge: independent D3 review should check that source refs cannot round-trip
as `[label](source:id)` links, that `source_entities` are attached only to
document source nodes, that stale structured saves correctly conflict rather
than silently rebasing, and that remaining legacy read fallbacks are not new
write paths. If accepted, integrate into root and proceed to D4 Texture agent
structured operation tools.

## 2026-06-21 - Pass 16 - D3 Accepted And Integrated

Claim: The D3 editor/user path candidate is acceptable if independent review
finds no path where source refs round-trip back to clickable links or legacy
source sidecars, and root reruns the focused evidence after integration.

Move: probe + construct. Independent D3 review returned `accept` with no
blocking findings. Root integrated the accepted D3 commit, resolving only
paradoc/ledger drift from the D2 accepted-state commit, and will rerun focused
frontend/backend checks on the integrated branch.

Expected delta V: -1 for the human editor path obligation.

Actual delta V: -1. Current V=6. D3 is integrated and accepted, but this is not
mission settlement.

Receipt: commit `3e390b76` integrated on root; D3 review thread
`019eeb8d-7523-79e1-8f3f-342172784ee7`.

Evidence target after integration:
`cd frontend && npx playwright test tests/texture-structured-editor-doc.spec.js`;
`cd frontend && npm run build`;
`nix develop -c go test ./internal/runtime -run TestTextureRevisionAPIAcceptsStructuredBodyAndRejectsLegacySourceSyntax`;
`git diff --check HEAD~1..HEAD`.

Open edge: D4 Texture agent structured operation tools. Before runtime mutation,
record the Problem Documentation First checkpoint naming that Texture agents
still edit canonical text through string rewrite/patch surfaces rather than
validated block/node/source operations. Later cuts still need multimedia
embedding/resolution, publication/export, broad legacy deletion, deployment, and
Comet product proof.

## 2026-06-21 - Pass 17 - D4 Agent Operation Problem Checkpoint

Claim: D4 cannot safely mutate Texture agent tools until the behavior problem is
documented separately from the fix: appagent canonical writes still use string
patch/rewrite surfaces and source metadata sidecar normalization even though D2
made those sidecars invalid as canonical source identity.

Move: construct. Added `D4 Agent Operation Problem Checkpoint - 2026-06-21` to
the paradoc. The documented problem is that `patch_texture` edits
`current.Content` through `find` / `replace` / `append`, `rewrite_texture`
accepts full plain `content`, and `commitTextureToolEdit` still normalizes wire
article source prose into `[label](source:id)` while carrying source identity
through `metadata.source_entities` / `source_ref_normalization`. D4 must replace
that with validated block/node/source operations over the structured body, with
runtime-owned ids/provenance and top-level `SourceEntities`.

Expected delta V: 0 on product obligations; satisfies the red-surface
precondition for D4 runtime/tool mutation.

Actual delta V: 0. Current V remains 6. Runtime behavior unchanged by this pass.

Receipt:
`docs/mission-texture-structured-document-transclusion-cutover-v0.md`.

Open edge: implement the bounded D4 runtime/tool cut with focused tests for
source-ref preservation, explicit source deletion, source ref/embed insertion,
legacy syntax rejection, stale-base protection, and no metadata/citation source
sidecar writes.

## 2026-06-21 - Pass 18 - D4 Accepted And Integrated

Claim: D4 can decrement the mission variant only if the accepted implementation
shows appagent Texture writes now use structured block/node/source operations
and no longer create canonical source identity through string patches, clickable
source links, citations, or metadata sidecars.

Move: prover shift + construct. D4 re-review thread
`019eebae-975a-7430-b4ae-251e0e02025e` returned `accept` with no findings after
focused runtime/store tests. Root integrated the accepted commits as `cdd7232c`
(`Cut Texture agent tools to structured operations`) and `cb690d8c` (`Read
Universal Wire sources from structured Texture refs`), then reran the focused
and shard evidence on the root worktree. The paradoc now records D4 as accepted
and updates the next move to D5 multimedia path documentation/implementation.

Expected delta V: -1 for the Texture agent structured operation path.

Actual delta V: -1. Current V=5. D4 is integrated and accepted; this is not
mission settlement.

Receipts:
`internal/runtime/tools_texture.go`;
`internal/runtime/texture_tool_unit_test.go`;
`internal/runtime/texture_evidence_sources_test.go`;
`internal/runtime/texture_legacy_wire_normalization.go`;
`internal/runtime/universal_wire.go`;
`internal/runtime/universal_wire_test.go`;
`docs/mission-texture-structured-document-transclusion-cutover-v0.md`.

Evidence:
`nix develop -c go test ./internal/runtime -run 'Texture.*(Structured|Tool|Agent|Source)|TestHandleUniversalWireStoriesUsesVisibleSourceEntitiesForSourceNetworkManifest'`;
`nix develop -c go test ./internal/texturedoc ./internal/store`;
`nix develop -c scripts/go-test-runtime-shards`;
`git diff --check HEAD~2..HEAD`;
D4 review evidence: `nix develop -c go test ./internal/runtime -run 'TestHandleUniversalWireStoriesUsesVisibleSourceEntitiesForSourceNetworkManifest|TestTextureToolCommitWritesStructuredRevisionAndRejectsStaleBase|TestTextureToolRejectsLegacyEditsAndSourceSyntax|TestTextureCoagentEvidenceSummarySourceCanPatchWithNativeCitation'`;
D4 review evidence: `nix develop -c go test ./internal/texturedoc ./internal/store -run 'TestValidatorRejectsLegacySourceSyntaxesInText|TestTextureCreateRevisionRejectsLegacySourceSidecars|TestTextureCreateRevisionStoresStructuredBodyAndSourceEntities|TestTextureCreateRevisionDerivesStructuredBodyForPlainText'`.

Open edge: D5 multimedia. Before touching resolver/rendering behavior, record
the Problem Documentation First checkpoint naming the current multimedia gap:
schema-level image/video/audio/PDF/transcript/Texture-span source entities exist,
but product resolver/rendering paths still must prove those targets transclude
through the same `source_ref` / `source_embed` plus top-level `SourceEntity`
model, without renderer-only media cards or clickable links.

## 2026-06-21 - Pass 19 - D5 Multimedia Problem Checkpoint

Claim: D5 cannot safely mutate multimedia resolver/rendering behavior until the
mission records the current behavior problem separately from the fix.

Move: construct. Added `D5 Multimedia Path Problem Checkpoint - 2026-06-21` to
the paradoc. The checkpoint names the gap: schema-level multimedia source
entities exist, but live Texture still discovers image/YouTube URLs from
projection text into `metadata.media_source_refs`, derives source entities from
that sidecar, prompts agents with media refs, and has frontend media-ref
synthesis plus kind-specific media rendering outside a fully proven structured
`source_ref` / `source_embed` plus top-level `SourceEntity` path.

Expected delta V: 0 on product obligations; satisfies the red/orange-surface
precondition for D5 runtime/frontend mutation.

Actual delta V: 0. Current V remains 5. Runtime/frontend behavior unchanged by
this pass.

Receipt:
`docs/mission-texture-structured-document-transclusion-cutover-v0.md`.

Evidence recorded:
`internal/runtime/texture_agent_revision.go:375`;
`internal/runtime/texture_agent_revision.go:377`;
`internal/runtime/texture_agent_revision.go:606`;
`internal/runtime/texture_media_sources.go:52`;
`internal/runtime/texture_media_sources.go:199`;
`frontend/src/lib/texture-source-state.ts:9`;
`frontend/src/lib/texture-source-state.ts:25`;
`frontend/src/lib/texture-source-renderer.ts:286`;
`frontend/src/lib/texture-source-renderer.ts:321`;
`internal/texturedoc/schema.go:393`;
`internal/texturedoc/schema.go:403`.

Open edge: implement the bounded D5 multimedia resolver/rendering cut. Do not
bundle publication/export, deployment, or broad old-path deletion. Expected
evidence: focused Go tests for structured multimedia source validation and no
new `metadata.media_source_refs` writes; focused frontend tests proving
structured multimedia entities render from top-level `SourceEntities` plus
document source nodes without client-side media sidecar synthesis or clickable
source links.

## 2026-06-21 - Pass 20 - D5 Multimedia Implementation Checkpoint

Claim: D5 decreases the variant if new deterministic media discovery and
frontend rendering use the structured Texture source model instead of introducing
or preserving a second media sidecar contract.

Move: construct + attempted observer shift. Runtime media URL discovery now
collates media as Texture source entities and removes `media_source_refs` /
`media_source_research_required` from new agent revision metadata and prompts.
Durable appagent metadata no longer carries those keys forward. The source
contract now names image/video/audio/PDF/transcript/file/source-window open
surfaces, and the texturedoc validator accepts them for structured source
entities. Frontend source state keeps `media_source_refs` synthesis only for
legacy revisions without `body_doc`, and the renderer embeds video/image/audio
and PDF media from source entities without clickable source links.

Expected delta V: -1 for the multimedia resolver/rendering path.

Actual delta V: -1 locally. Current V=4. Independent review was attempted with
two read-only Codex review agents (`/root/d5_multimedia_review` and
`/root/d5_quick_review`), but both stalled without returning findings and were
interrupted. This pass is therefore a local implementation checkpoint, not an
independently accepted D5 settlement.

Receipts:
`internal/runtime/texture_agent_revision.go`;
`internal/runtime/texture_media_sources.go`;
`internal/runtime/runtime.go`;
`internal/runtime/texture_test.go`;
`internal/sourcecontract/source_contract_schema.json`;
`internal/sourcecontract/open_surface.go`;
`internal/sourcecontract/testdata/source_contract_matrix.json`;
`internal/texturedoc/schema.go`;
`internal/texturedoc/schema_test.go`;
`frontend/src/lib/source-contract.ts`;
`frontend/src/lib/source-contract.generated.ts`;
`frontend/src/lib/texture-source-state.ts`;
`frontend/src/lib/texture-source-renderer.ts`;
`frontend/tests/texture-source-entities.spec.js`.

Evidence:
`nix develop -c go test ./internal/sourcecontract ./internal/texturedoc`;
`nix develop -c go test -tags comprehensive ./internal/runtime -run TestTextureAgentRevisionRegistersMediaSourceEntities -count=1`;
`nix develop -c go test -tags comprehensive ./internal/runtime -run TestMarkTextureMediaSourceRefsResearchState -count=1`;
`nix develop -c go test ./internal/sourcecontract ./internal/texturedoc ./internal/store`;
`nix develop -c go test ./internal/runtime -run 'Texture.*Media|SourceEntities|MarkTextureMedia'`;
`nix develop -c scripts/go-test-runtime-shards`;
`node scripts/generate-source-contract.mjs --check` from `frontend/`;
`npx playwright test tests/texture-source-entities.spec.js --grep "frontend source contract|structured revisions|multimedia source entities"` from `frontend/`;
`npm run build` from `frontend/`;
`git diff --check`.

Open edge: D6 publication/export projection. Publication/export/diff/search
must consume structured body/source entity projections without reviving markdown
links or source metadata sidecars. Record independent review when available and
do not treat D5 as staging/product proof until the landing loop runs.

## 2026-06-21 - Pass 21 - D6 Publication/Export Problem Checkpoint

Claim: D6 cannot safely change publication/export until the mission records the
remaining behavior problem separately from the fix.

Move: construct. Added `D6 Publication/Export Problem Checkpoint - 2026-06-21`
to the paradoc. The checkpoint names the gap: publication currently parses
source refs from flattened markdown projection links, reads source entities from
metadata, and records version history entries without structured `body_doc` /
top-level `source_entities`.

Expected delta V: 0 on product obligations; satisfies the red/orange-surface
precondition for D6 publication/export mutation.

Actual delta V: 0. Current V remains 4. Runtime/platform behavior unchanged by
this pass.

Evidence recorded:
`internal/platform/publication_document.go:65`;
`internal/platform/publication_document.go:276`;
`internal/platform/publication_document.go:293`;
`internal/platform/source_metadata.go:94`;
`internal/platform/types.go:218`;
`internal/platform/service_test.go:1210`.

Open edge: implement the D6 publication/export projection cut. Keep legacy
markdown publication parsing only as historical fallback and do not bundle broad
old-path deletion or deployment in the same move.

## 2026-06-21 - Pass 22 - D6 Publication/Export Implementation Accepted

Claim: D6 decreases the variant if publication/export consumes structured
Texture documents and top-level source entities as canonical input while keeping
markdown `source:` parsing only as a historical fallback for artifacts without
structured body data.

Move: construct + prover shift. Added structured publication request fields,
artifact manifest persistence, bundle readback, version history preservation,
wire/proxy publish propagation, top-level SourceEntity metadata normalization,
and structured publication document rendering for `source_ref` / `source_embed`
nodes. The independent D6 review returned `revise_before_continue` with two P1
findings: unlabeled structured `source_ref` could render `<nil>`, and explicit
top-level `source_entities: []` could fall back to stale
`metadata.source_entities`. Both were repaired and the same review thread
returned `accept`.

Expected delta V: -1 for publication/export projection.

Actual delta V: -1. Current V=3. D6 is independently accepted at local/package
scope after P1 repair.

Receipts:
`internal/platform/types.go`;
`internal/platform/publication_structured.go`;
`internal/platform/publication_document.go`;
`internal/platform/source_metadata.go`;
`internal/platform/service.go`;
`internal/platform/service_publication_read.go`;
`internal/platform/version_history.go`;
`internal/platform/service_test.go`;
`internal/wirepublish/types.go`;
`internal/wirepublish/request.go`;
`internal/runtime/wire_platform_publish.go`;
`internal/proxy/wire_platform_publish.go`;
`internal/proxy/platform_publish.go`;
`docs/mission-texture-structured-document-transclusion-cutover-v0.md`;
`docs/mission-texture-structured-document-transclusion-cutover-v0.ledger.md`.

Evidence:
`nix develop -c go test ./internal/platform -run 'TestPublishTextureStructuredBodyDrivesPublicationSources|TestPublishTextureStructuredEmptySourceEntitiesSuppressLegacyMetadata' -count=1`;
`nix develop -c go test ./internal/platform ./internal/wirepublish`;
`nix develop -c go test ./internal/proxy -run 'WirePlatformPublish|PlatformPublish'`;
`nix develop -c go test ./internal/runtime -run 'Wire.*Publish|UniversalWire.*Source|SourceEntities'`;
`git diff --check`;
D6 re-review focused test:
`nix develop -c go test ./internal/platform -run 'TestPublishTextureStructured(BodyDrivesPublicationSources|EmptySourceEntitiesSuppressLegacyMetadata)' -count=1`.

Open edge: D7 deletion and proof preparation. Map remaining old source syntax
readers/writers, classify historical fallback versus deletion target, delete or
hard-reject canonical clickable-link/source-token/citation-sidecar paths, then
run broad tests and the staging landing loop.
