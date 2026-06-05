# VText Mission Hard Review - 2026-06-05

This is a hard review of the current state of the VText fluid editing, document roundtrip, table preservation, and citation transclusion mission. It covers the mission as a whole, not only the latest code changes.

Review point in time:

- Local branch: `main`
- Local HEAD and `origin/main`: `400b8084048129c3051b1df0af50d059300304a3`
- Staging health: `https://choir.news/health` returned `status: ok`, proxy `upstream: ok`, `vmctl_status: ok`
- Staging deployed commit: `4255dc7efe5407b67bb78075cf477c133958d2f3`, deployed at `2026-06-05T14:26:47Z`
- Later commits after `4255dc7e` are docs/test-only and did not redeploy staging artifacts
- Latest observed CI: run `27022749951` passed for `a2c7c62f55c9ad28c7346954a1ccbdd8d7b24c22`; FlakeHub run `27022749965` passed for the same SHA
- Focused local runtime check: `nix develop -c go test ./internal/runtime -run 'TestVText(Merge|Semantic|Compare|Accept|SourceGap|OpenFile|Imported|ImportMarkdown|EnsureManifest)'` passed

## Findings

### P0 - The mission is not accepted until owner-account proof is completed

The deployed system has meaningful generic and owner-adjacent proof, but it does not yet satisfy the mission's own acceptance bar for the actual owner document. The latest mission state still names the unproven owner requirements: canonical title migration from `.md` to `.vtext`, bounded appendix-table edit survival, source-gap repair through the deployed Sources panel on the owner head, citation expansion into transclusions, source-window opening, and focused prompt-size / `apply_edits` metadata on the owner head.

Evidence:

- `docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md:1449` identifies the first bad owner transition as v74 -> v75.
- `docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md:1495` records owner Comet restore from v74 and the table-preserving owner revise to v81.
- `docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md:1540` records bounded table-edit, source-gap, citation/source-window, and metadata proof as incomplete.
- `docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md:2660` records the current remaining error field.

This is the most important review result. The system is substantially better, but final acceptance still depends on real owner-account UI proof that is currently passkey-gated.

### P1 - Source repair is still a raw JSON repair surface, not an owner-grade workflow

The backend and product path now prove source gap repair on a fixture, and the Sources panel can show source gaps, source entities, candidate markers, and edit evidence. However, the repair action is still driven by a visible `Repair JSON` textarea and an `Apply source repair` button. That is useful for QA and agent operation, but it is not the owner workflow implied by the contract.

Evidence:

- `frontend/src/lib/VTextEditor.svelte:2459` renders the Sources panel with gaps, entities, diagnosis, evidence, and repair JSON.
- `frontend/src/lib/VTextEditor.svelte:2560` exposes `Repair JSON` directly in the user-visible panel.
- `frontend/src/lib/VTextEditor.svelte:1886` sends the repair JSON to `handleApplySourceRepair`.
- `docs/source-external-data-publication.md:287` requires every citation marker to be a transclusion point with a retrievable/openable source.

Recommended direction: keep the proven backend contract, but replace the raw JSON interaction with a typed repair draft model. The UI should present unresolved markers as rows with explicit source candidates, create/attach decisions, and source-opening proof. A raw JSON affordance can remain behind a developer mode if it is still needed for diagnosis.

### P1 - The diagnosis surface is too broad and too raw for the product proof role it now serves

The authenticated diagnosis endpoint helped unblock proof when Comet could read private UI state but normal shell/API calls were unauthenticated. It now returns a very broad raw bundle: owner/document identity, store paths, VText paths, revisions, messages, runs, events, evidence, and content fragments. That is acceptable as an internal diagnostic during a mission, but it is too broad to become the durable owner-facing proof surface.

Evidence:

- `internal/runtime/vtext.go:3479` implements `HandleVTextDiagnosis`.
- The mission records authenticated Comet/raw diagnosis use as the proof path after shell calls returned `401 authentication required`.
- `docs/source-external-data-publication.md:398` expects product proof from normal product flows, not a broad internal dump.

Recommended direction: split the endpoint or response model into a minimal product-safe diagnosis bundle and a separately authorized deep-debug bundle. The product bundle should expose only what the owner needs to understand document structure, source gaps, transclusion status, revision provenance, and edit evidence.

### P1 - Compare/merge remains brittle because the key owner restoration path failed through model-backed compare

The owner root cause was identified without relying on compare/merge, and the visible Restore path successfully repaired the owner head. However, the built-in compare/merge route failed on the actual owner v74 -> v78 path with `model-backed semantic compare failed`. That leaves the version-compare contract only partially satisfied for the hardest document.

Evidence:

- `docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md:1460` records the staging `COMPARE FAILED` owner path.
- `internal/runtime/vtext.go:3067` implements `HandleVTextSemanticCompare`.
- `internal/runtime/vtext.go:3146` implements `HandleVTextMergePreview`.
- `docs/vtext-version-compare-merge-debuggability-spec.md:360` requires clear acceptance behavior for compare, merge preview, and focused provenance.

Recommended direction: compare/merge should have a deterministic structural fallback for large VText revisions, especially when model compare fails. The owner path should degrade into an inspectable structural diff rather than blocking repair.

### P2 - `VTextEditor.svelte` is now too large and owns too many unrelated responsibilities

`frontend/src/lib/VTextEditor.svelte` is 4026 lines and currently owns rendering, Markdown/VText serialization, autosave, current revision management, source entity state, source repair, diagnosis, edit evidence display, publication/export actions, compare/merge UI state, and streaming revise state. The mission additions were reasonable under pressure, but the component is now carrying enough responsibilities that future bug fixes will be riskier than necessary.

Evidence:

- `frontend/src/lib/VTextEditor.svelte:292` starts source entity helpers.
- `frontend/src/lib/VTextEditor.svelte:359` starts edit-evidence helpers.
- `frontend/src/lib/VTextEditor.svelte:1184` handles manifest creation.
- `frontend/src/lib/VTextEditor.svelte:1209` handles revision saving.
- `frontend/src/lib/VTextEditor.svelte:1852` starts source panel actions.
- `frontend/src/lib/VTextEditor.svelte:2200` derives source panel state.
- `frontend/src/lib/VTextEditor.svelte:2459` renders source panel UI.

Recommended direction: extract pure helpers and small state modules before adding more mission behavior. Good first cuts are source entity normalization, source repair draft construction, edit evidence extraction, Markdown/VText render helpers, and persistence actions.

### P2 - The default source repair payload builder can generate misleading repairs

`defaultSourceRepairPayload` maps unresolved markers to existing source entities by array position and can emit empty entity IDs. That helped bootstrap manual repair tests, but it is a weak abstraction because it implies a relationship that may not exist.

Evidence:

- `frontend/src/lib/VTextEditor.svelte:397` builds `attach_existing` repairs by pairing candidates and source entities by index.

Recommended direction: represent repair drafts as typed rows with explicit resolution state: unresolved marker, suggested source candidates, selected entity, create-new-source fields, and validation errors. Do not emit a repair operation until the row has an explicit valid source identity.

### P2 - `writeThroughToFile` looks like legacy noncanonical write-through under the new VText invariant

The mission now asserts that imported `.txt`, `.md`, and other source formats should become canonical `.vtext` on the v0 -> v1 transition, with export back to the original format as a projection. Under that invariant, `writeThroughToFile` appears to be a legacy path. It returns for `.vtext` paths and for any `currentDoc?.doc_id`, which covers normal canonical VText documents.

Evidence:

- `frontend/src/lib/VTextEditor.svelte:1168` implements `writeThroughToFile`.
- `frontend/src/lib/VTextEditor.svelte:1173` skips `.vtext` paths.
- `frontend/src/lib/VTextEditor.svelte:1176` skips paths backed by a current VText document.
- `docs/vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md:14` frames original files as projection/import/export artifacts, not canonical mutable state.

Recommended direction: prove whether any current noncanonical editor context still requires this path. If not, delete it and route all document writes through canonical VText revisions plus explicit export.

### P2 - Table preservation is repaired, but the structural preservation layer is still reactive

The implemented table stabilization is not a glossary-specific special case, and it directly addresses the observed corruption path. Still, it is reactive: it recognizes and repairs collapsed Markdown table structure after model/user text changes rather than preserving an AST-like document structure through the whole render/edit/save/revise path.

Evidence:

- `internal/runtime/vtext.go:2221` implements `stabilizeVTextUserMarkdownStructures`.
- `docs/vtext-version-compare-merge-debuggability-spec.md:249` requires structured edit semantics by default.
- The owner incident showed a Markdown glossary table collapsed into prose-like `TermDefinition` artifact between v74 and v75.

Recommended direction: keep the stabilization as a regression guard, but treat the durable design as structured VText block preservation. Tables, headings, transclusion points, hidden metadata, and source references should survive as first-class structure, not as text patterns recovered after the fact.

### P2 - Mission documentation is useful but too large to be the primary operator state

The mission document is now 2668 lines. It contains the right evidence discipline, but the size makes it hard for a new operator to find the current state, remaining risks, and next action quickly. The mission doc should remain the canonical narrative, but this review should become a checkpoint artifact and the mission doc should gain a compact resumable state section.

Evidence:

- `docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md` is 2668 lines.
- The current remaining error field starts at `docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md:2660`.

Recommended direction: keep evidence append-only, but add a short "Current Checkpoint" near the top that points to detailed evidence sections and this review report.

### P3 - Browser regression coverage is high value but should be made less bulky before it grows again

The new Playwright tests cover important staging/product behavior: imported Markdown canonical `.vtext` identity, Markdown export projection, bounded table autosave, source gap repair, source panel refresh, and source-window opening. The tests are valuable, but the files are getting long and carry repeated setup and fetch helpers.

Evidence:

- `frontend/tests/vtext-markdown-lineage.spec.js` is 633 lines.
- `frontend/tests/vtext-source-entities.spec.js` is 219 lines.
- Recent CI passed for the test-only commits that added this coverage.

Recommended direction: extract shared VText fixture setup and `fetchJSON`/product API helpers into a test helper module. Keep the scenario tests readable and focused on product contracts.

## What Works Now

- Computer Use is available, and authenticated Comet was used for owner staging QA where the UI allowed it.
- The real owner regression was root-caused to the first bad transition v74 -> v75. v70-v74 preserved the rendered appendix table, while v75-v78 collapsed the table into a `TermDefinition` artifact.
- The real owner document was restored from v74 through a visible owner UI Restore control, creating a newer head with the table visible.
- The real owner document survived a focus/edit/save/revise path when the table was untouched. Appendix A still rendered as one HTML table and `TermDefinition` did not reappear.
- Generic staging regression coverage now proves bounded rendered-table cell autosave without corrupting table shape.
- Generic staging regression coverage now proves unresolved source repair can create/attach source entities and open the source window from the Sources panel.
- Generic staging regression coverage now proves imported Markdown advances to canonical `.vtext` identity on v0 -> v1 while retaining Markdown export as projection.
- Focused local runtime coverage for merge, compare, source gap, open-file, imported Markdown, import Markdown, and manifest paths passes under `nix develop`.

## What Is Not Proven Yet

- Actual owner document bounded appendix-table edit survival.
- Actual owner document source-gap repair through the deployed Sources panel.
- Actual owner document citation marker expansion into transclusion points.
- Actual owner document source-window opening from repaired owner citations.
- Actual owner document visible focused prompt-size and `apply_edits` metadata after ordinary revisions.
- Actual owner document title migration from `choir_private_legal_cloud_proposal.md` to canonical `.vtext` on the next owner VText write.
- Complete external source lifecycle from external source fetch/search/read through transclusion and publication proof.
- Owner compare/merge repair path for v74 -> latest. Restore worked, compare/merge failed.
- Passkey-completed owner Comet acceptance. Current private UI proof is blocked by passkey overlay/cancelled ceremony for `yusefnathanson@me.com`.

## Current System State

The current system is in a mixed but understandable state:

- Canonical VText direction is correct: imported `.md` should behave as canonical VText after first revision, and Markdown should be an export projection.
- The table corruption incident is contained for the tested paths, including the real owner untouched-table revise and generic bounded table edit.
- The source/citation path has enough backend and fixture proof to continue, but the owner document remains the acceptance gate.
- The frontend editor has accumulated mission-specific UI, diagnosis, and repair responsibilities that should be simplified before the next broad expansion.
- Staging is healthy and serving `4255dc7e`; repo main is ahead with docs/test-only commits at `400b8084`.

## Simplification Plan

Do this only after preserving the report and without weakening the existing tests.

1. Extract source entity normalization, source gap projection, edit evidence extraction, and source repair draft construction from `VTextEditor.svelte` into pure helpers with focused unit tests.
2. Replace `defaultSourceRepairPayload` with a typed repair draft builder that never emits an operation with an empty or inferred source entity ID.
3. Audit `writeThroughToFile`; delete it if no current canonical VText flow depends on it, or fence it behind an explicit noncanonical legacy path with tests.
4. Keep the backend table stabilization regression guard, but name it as structural preservation scaffolding and add tests that show it is not glossary-specific.
5. Extract repeated Playwright product API setup from VText browser tests into shared helpers.
6. Add a compact current-checkpoint section to the mission document that links to this review and the latest remaining error field.

## Landing Recommendation

Do not mark the mission complete yet. The next acceptance-producing work should be:

1. Unblock owner Comet/passkey UI access or obtain an equivalent authenticated owner product path.
2. Prove owner bounded table edit survival.
3. Prove owner source repair, citation expansion, and source-window opening.
4. Prove owner visible edit evidence for prompt sizes and `apply_edits`.
5. Then run the simplification pass with the current regression tests as guardrails.

