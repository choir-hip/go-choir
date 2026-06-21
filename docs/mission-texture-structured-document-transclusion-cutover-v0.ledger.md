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
