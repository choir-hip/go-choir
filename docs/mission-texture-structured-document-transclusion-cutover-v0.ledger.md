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
