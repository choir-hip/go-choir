# O4 Native Texture Source Ref Browser Harness Checkpoint - 2026-06-26

## Work Item

`O4-phase10c-native-texture-source-ref-browser-harness`

## Mutation Class

Green for this checkpoint. A later harness repair is likely yellow if it only
changes frontend test setup, dependency install policy, or browser proof
fixtures. It becomes orange only if product runtime/API behavior must change.

## Conjecture Delta

O3 Phases 4-6 proved that Texture revision reads can expose graph-backed
`source_entity_objects` plus `source_refs`, and that the frontend can render and
open native body `source_ref` citations from a graph-only revision DTO without
legacy `source_entities`. O4 Phase 9 proved Universal Wire graph-capture source
items can open Source Viewer reader artifacts with durable reader text, but did
not prove native Texture body citation opening. O4 Phase 10 and 10b attempted
to close that native Texture axis and instead exposed a harness blocker.

The revised conjecture is narrower: before making another native Texture
source-ref proof claim, Choir needs a runnable local browser harness for the
existing `frontend/tests/texture-source-entities.spec.js` source-ref path, or a
replacement product-path proof that does not depend on the failing harness
setup.

## Protected Surfaces

- Source-opening doctrine: Source Viewer/reader artifacts are the default
  durable source-opening path; Web Lens is only explicit live/original
  inspection.
- Texture canonical write safety and document-head advancement invariants.
- Source entity/source_ref identity: tests must not synthesize source entities
  from legacy `metadata.media_source_refs`.
- Legacy `source_entities` compatibility and additive graph wrapper fields.
- Local browser proof honesty: mocked revision DTOs are acceptable only when the
  evidence boundary says they prove frontend/UI consumption, not backend graph
  wrapper production.
- Harness cleanup and worktree hygiene: generated `node_modules`, Playwright
  outputs, Vite logs, service logs, and temporary lock files must not be left as
  unclassified tracked or untracked artifacts.

No publication/export, Qdrant, provider/search, auth/session renewal,
vmctl/candidate computers, deployment routing, promotion/rollback, staging, or
run-acceptance surfaces are in scope for this checkpoint.

## Evidence

O4 Phase 10 worker thread
`019f03ff-f119-75d3-8bf2-ae3f50af3ab4` returned no candidate and no diagnostic
evidence beyond a clean starting worktree.

O4 Phase 10b worker thread
`019f0405-4fea-70f1-b248-5b6ebce70775` inspected the focused native Texture
source-ref browser test and found a real proof-tightening opportunity:

- In `frontend/tests/texture-source-entities.spec.js`, the graph-only Texture
  source-opening test used the same phrase for the inline
  selector/excerpt path and the graph object body path.
- Separating those strings would let assertions distinguish inline citation
  note rendering from Source Viewer stored-reader-artifact rendering.
- The worker made that local test edit, then reverted it because the required
  focused Playwright proof could not run.

Observed harness failures:

- Running the focused Playwright test initially failed because
  `@playwright/test` was absent from the worker worktree.
- After `npm ci`, running the focused Playwright test without the service stack
  failed because no frontend was listening at `http://localhost:4173/`.
- Running Vite preview alone made the frontend available, but auth setup timed
  out because backend/proxy endpoints were unavailable; Vite logged
  `ECONNREFUSED 127.0.0.1:8082` for `/api/shell/bootstrap` and
  `/api/preferences/theme`.
- Full `nix develop -c ./start-services.sh` failed at `corpusd failed`;
  logs showed corpusd could not read
  `/tmp/go-choir-m2/platform-dolt/platform/.dolt/repo_state.json`.
- `CHOIR_ENABLE_CORPUSD=0 nix develop -c ./start-services.sh` reached
  auth/sandbox/proxy startup but failed the frontend phase because the script's
  pnpm install refused ignored build scripts for `esbuild@0.21.5`.
- Manual repair attempts were not committed; worker-local generated artifacts
  were cleaned and service listeners were stopped.

Root/orchestration preserved this as a harness diagnosis only. No worker commit,
verifier, root incorporation, staging proof, or product acceptance was claimed.

## Belief State

The smallest useful next repair is not another broad native Texture proof
worker. It is either:

1. a harness-only repair that makes the existing Texture source-ref Playwright
   tests runnable from a clean worktree with documented dependency/setup
   commands and cleanup, or
2. a different product-path proof that exercises native Texture source-ref
   Source Viewer opening without relying on the failing frontend harness.

The likely test tightening remains valid candidate work only after the harness
is runnable: make the graph object body text distinct from inline selector text,
then assert the Texture citation note shows the excerpt while Source Viewer
shows the graph object body-derived reader artifact text.

## Remaining Error Field

This checkpoint documents a blocker; it does not repair it. The remaining
native Texture source/citation proof chain is:

```text
runnable local Texture source-ref browser harness
-> focused graph-only native source_ref Source Viewer reader-artifact assertion
-> independent verifier acceptance
-> root incorporation
-> deployed/staging source artifact proof if the checklist requires product acceptance
```

The broader News benchmark still also excludes publication/export,
sourcecycled-to-Texture article creation with body citations, Qdrant/source
search projection, provider/search calls, promotion/rollback, and run
acceptance until separately proven.

## Rollback

Drop/revert this checkpoint and the corresponding paradoc/ledger update. O0-O4
Phase 9 accepted commits and the Phase 10/10b no-candidate records remain
historical evidence unless superseded by a later accepted proof.
