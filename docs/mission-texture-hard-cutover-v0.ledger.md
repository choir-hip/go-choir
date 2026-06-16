# Mission: Texture Hard Cutover v0 Ledger

## 2026-06-15 - Mission Creation

Claim: promoting Texture as the artifact control-plane ontology must be treated
as a hard cutover mission, not a passive docs rename.

Move: construct the initial paradoc and split the former single explanatory
draft into a short current orientation doc plus a historical/background doc.

Expected ΔV: establish the mission source program without decreasing execution
variant; V remains 10 because implementation and proof have not begun.

Actual ΔV: 0 execution obligations discharged; mission became resumable.

Receipts:
- `docs/why-texture-2026-06-15.md`
- `docs/why-texture-background-2026-06-15.md`
- `docs/mission-texture-hard-cutover-v0.md`

Open edge: the checker rule, repo-wide inventory, runtime rename, product proof,
transclusion proof, and protocol canonization remain future moves.

## 2026-06-15 - Problem Checkpoint: Retired-Name Inventory

Claim: the old V-name is system-wide current ontology residue, not a small
implementation alias, so the first admissible move is documentation and
checker-design evidence before behavior changes.

Move: probe plus documentation checkpoint. Ran read-only inventory commands and
captured the checker warning/allowlist design in the paradoc.

Expected ΔV: -1 by discharging the inventory/design obligation without touching
runtime behavior.

Actual ΔV: -1. V moves from 10 to 9. Runtime, prompts, frontend, tests, APIs,
schema, and tool affordances remain unmodified.

Receipts:
- `rg -l -i 'vtext|\.vtext|VText|VTEXT'` over the worktree: retired-name
  content in 172 docs files, 82 runtime Go files, 35 frontend source files,
  33 frontend tests, 9 store files, 9 runtime prompt files, 6 type files,
  4 command files, 2 spec files, and both root contracts.
- Retired-name path components: 44 docs paths, 22 runtime Go paths,
  18 frontend source paths, 16 frontend test paths, 2 store paths, 1 type path,
  1 runtime prompt path, and 1 command path.
- Selected affordance line counts: `/api/vtext` 505, `data-vtext` 604,
  `edit_vtext` 390, `request_super_execution` 122, V-name profile references
  417, `.vtext` 942, `vtext_` 658.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json`: report-only complete, 212 docs, 803 warnings.

Open edge: the report-only checker rule is designed but not implemented; the
checkpoint still needs to be committed before runtime changes.

## 2026-06-15 - Report-Only Checker Rule

Claim: the docs checker can expose Texture retired-name drift now without
turning docs-only CI into a fail-closed gate before the baseline is reduced.

Move: construct H5 in `cmd/doccheck` as a file-level warning over docs, Go,
frontend, prompt, script, workflow, and spec surfaces. Added allowlist handling
for the Texture historical background doc, manifest-classified historical or
evidence docs, explicitly labeled historical/migration mission evidence,
`cmd/doccheck` detector implementation/tests, and temporary code lines marked
`texture-cutover-allow:` with a deletion receipt.

Expected ΔV: -1 by landing report-only checker coverage while preserving the
Problem Documentation First checkpoint boundary before runtime changes.

Actual ΔV: -1. V moves from 9 to 8. Runtime behavior and product affordances
remain unchanged.

Receipts:
- `go test ./cmd/doccheck`: pass.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,155 warnings.
- `/tmp/choir-doccheck.json` warning counts: H1=724, H3=19, H4=3, H5=352,
  R3=57.
- H5 file-level warning distribution: AGENTS.md=1, README.md=1, cmd=5,
  docs=136, frontend=66, internal=142, specs=1.

Open edge: H5 is warning-only; high-read docs, prompts, UI, tests, runtime
symbols, routes, storage names, and tool affordances still need the actual
Texture cutover.

## 2026-06-15 - High-Read Operating Contract Reconciliation

Claim: future agents should read Texture, not the retired name, in the operating
contract and domain invariant before runtime changes begin.

Move: construct a bounded docs slice. Renamed
`docs/vtext-agentic-invariants-2026-06-13.md` to
`docs/texture-agentic-invariants-2026-06-13.md`; reworded AGENTS.md, Choir
Doctrine, docs README, doc authority manifest, mission graph references, and
the invariant doc toward Texture for current operating prose.

Expected ΔV: -1 if high-read doctrine and operating docs cleared the retired
name.

Actual ΔV: 0 against the coarse mission variant. The operating contract no
longer has an H5 old-name warning, and H5 decreased from 352 to 349, but
`docs/README.md`, `docs/choir-doctrine.md`, and `docs/mission-graph.yaml` still
carry H5 warnings for still-existing old filenames and detector symbols. This
is useful progress, not discharge of the high-read-doc obligation.

Receipts:
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,148 warnings.
- `/tmp/choir-doccheck.json` warning counts after the slice: H1=724, H3=15,
  H4=3, H5=349, R3=57.

Open edge: continue the docs/index sweep or move runtime filenames/symbols so
doctrine evidence paths and detector symbols can converge without lying about
current code.

## 2026-06-15 - Local Product-Facing Texture Route And Tool Affordance

Claim: the cutover can move the product-facing route, frontend API client,
registered Texture writer tools, prompt defaults, and acceptance fixtures to
Texture without adding a runtime semantic workflow gate or deleting the old
route before proof.

Move: construct a bounded red-surface runtime slice. Added `/api/texture`
document routes that normalize to the existing internal document handlers,
kept `/api/vtext` as an explicitly temporary compatibility shim, allowed
`/api/texture` through the product API tool allowlist, switched the frontend
Texture client and browser test API calls to `/api/texture`, renamed registered
tool affordances to `edit_texture` and `record_texture_decision`, and updated
prompt defaults/tests to expect Texture tool/source metadata.

Expected ΔV: -1 by discharging the local prompt/tool/product-route affordance
obligation while leaving internal symbol, storage, UI label, staging, and
protocol obligations open.

Actual ΔV: -1. V moves from 8 to 7. The old route and internal `vtext` symbols
remain as migration residue and are not settlement-compatible.

Receipts:
- `nix develop -c go test ./internal/runtime -run 'TestDefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence|TestRecordVTextDecisionToolDescriptionKeepsDecisionsOffDocument|TestRecordVTextDecision|TestAgentToolProfiles|TestToolRegistry|TestProductAPIRequestToolUsesRunOwnerForAllowedProductRoute'`: pass.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run TestHandleVTextDocumentsRootUsesTextureRoutes`: pass.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,146 warnings.
- `/tmp/choir-doccheck.json` warning counts after the slice: H1=724, H3=15,
  H4=3, H5=347, R3=57.
- `rg -n "edit_vtext|record_vtext_decision" internal/runtime frontend/src/lib/vtext.js frontend/tests/vtext-markdown-lineage.spec.js frontend/tests/vtext-real-workflow-demo.spec.js`: no matches.
- `git diff --check`: pass.

Open edge: no staging deploy or browser product proof has run for this slice;
the compatibility shim, internal symbol/file/storage names, UI labels/data
attributes, common-vs-exceptional edit split, transclusion proof, and protocol
v0 remain open.

## 2026-06-15 - CI Failure Checkpoint: Wire Publish Eligibility

Claim: the first pushed Texture route/tool slice did not yet preserve Universal
Wire autonomous publication compatibility under the new revision metadata
source.

Move: document the CI-discovered problem before committing the repair. GitHub
Actions run `27581617910` for commit
`8d8ee883f6e6d11d8e42fef1077ab14c75e8e26d` failed before staging deploy.
Runtime shard 2 failed
`TestWireAutonomousPublishTranscludesEditionAndDebounces` because the edition
content stayed `"# Wire\n\nUniversal Wire edition."` instead of transcluding
`doc-publish-slice`. Runtime shard 3 failed
`TestWirePlatformPublishFailsClosedWithoutEditionWhenPlatformdFails` because
only the story-resolution work item remained open, not the expected
story-resolution plus in-flight publication pair.

Expected ΔV: 0. This is a Problem Documentation First checkpoint, not a repair.

Actual ΔV: 0. V remains 7. The failure shows the product-facing rename reached
revision metadata consumers outside the focused test packet.

Evidence / root-cause hypothesis:
- `internal/wirepublish.EligibleForAutonomousPublish` still gated revisions on
  the retired edit-source metadata value, while new Texture writes and fixtures
  now use `source=edit_texture`.
- Universal Wire read projection also needs to continue recognizing legacy
  stored metadata during the cutover window; deleting that compatibility would
  hide pre-cutover articles from reader/publish paths.

Open edge: repair should accept `edit_texture` as the current source, keep the
retired source only as deletion-receipted legacy metadata compatibility, and
prove both `internal/wirepublish` and the failed runtime publication tests.

## 2026-06-15 - Wire Publish Eligibility Compatibility Repair

Claim: Universal Wire autonomous publication should accept the current
`edit_texture` revision metadata source while preserving legacy stored
edit-source metadata during the cutover window.

Move: repair `internal/wirepublish` eligibility and Universal Wire read
projection predicates to treat `edit_texture` as current and retain the retired
source only behind deletion-receipted compatibility constants. Updated
`internal/wirepublish` tests so current fixtures use Texture metadata and one
explicit test proves legacy metadata remains eligible.

Expected ΔV: 0 against the coarse mission variant; this repairs a discovered
CI regression inside the product-facing route/tool slice rather than
discharging a new settlement obligation.

Actual ΔV: 0. V remains 7 until pushed CI and staging proof pass.

Receipts:
- `nix develop -c go test ./internal/wirepublish`: pass.
- `nix develop -c go test ./internal/runtime -run 'TestWireAutonomousPublishTranscludesEditionAndDebounces|TestWirePlatformPublishFailsClosedWithoutEditionWhenPlatformdFails|TestHandleUniversalWireStoriesUsesVisibleSourceEntitiesForSourceNetworkManifest|TestHandleUniversalWireStoriesSkipsTranscludedUnpublishedPlatformVTexts|TestNormalizeWireArticleSourceServiceProseRewritesBareLabels'`: pass.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,146 warnings.
- `/tmp/choir-doccheck.json` warning counts after the repair: H1=724, H3=15,
  H4=3, H5=347, R3=57.
- `git diff --check`: pass.

Open edge: the repair has not yet passed pushed CI, deployed to staging, or
received browser/product-path acceptance proof.

## 2026-06-15 - Staging Product Proof For Texture First Revision

Claim: the product-facing Texture route/tool slice is deployed and preserves
the core prompt-bar -> conductor -> Texture first-revision loop under the new
`/api/texture` and `edit_texture` affordances.

Move: push the docs checkpoint and repair commits, monitor CI/deploy, verify
staging identity, and run a temporary authenticated Playwright proof against
`https://choir.news`. The scratch spec was removed after the run; evidence was
written outside the repo.

Expected ΔV: -1 by discharging the deployed prompt-bar first-revision proof
for the current product-facing Texture slice.

Actual ΔV: -1. V moves from 7 to 6. This proves the first-revision loop for a
fully supplied prompt; it does not discharge the broader UI/internal symbol
cutover, transclusion proof, compatibility-shim deletion, or protocol v0.

Receipts:
- pushed commit:
  `be76501c5eba0bbb65ceb132d597f57a281affb9`
  (`runtime: accept texture wire publish metadata`), after docs checkpoint
  `d7b7e8e0` and runtime route/tool slice `8d8ee883`.
- CI run `27581914180`: success. Runtime shards 0-3, non-runtime tests,
  integration-tagged smoke, Go vet/build, Docs Truth Check job, TLA+ model
  check, final Go gate, and Node B staging deploy job passed.
- Docs Truth Check run `27581913968`: success.
- FlakeHub publish run `27581913969`: success.
- Staging health: `https://choir.news/health` reported proxy and sandbox
  commit `be76501c5eba0bbb65ceb132d597f57a281affb9`, deployed at
  `2026-06-15T23:00:02Z`.
- Staging acceptance command:
  `PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000 npm --prefix frontend run e2e -- tests/texture-cutover-staging.tmp.spec.js`
  passed, 1 test, 8.8s.
- Staging evidence artifact:
  `/tmp/texture-cutover-staging-proof-1781564582210.json`.
- Product evidence ids: prompt-bar submission
  `68bd8e67-3bb9-4aa8-bbbd-151d6df698d8`; Texture document
  `a73a2f12-9e95-45fd-a75e-6ab50ab2ec80`; user revision
  `4932cab1-0f56-487b-911a-24d4fa72c32f`; initial Texture loop
  `6cd68262-7c42-4d26-98ed-427ba4a3533e`; appagent revision
  `bde617a9-5630-42ce-9395-d5480197d85e`.
- Product observations: `/api/texture/documents/{id}`,
  `/api/texture/documents/{id}/revisions`,
  `/api/texture/documents/{id}/history`, and
  `/api/texture/documents/{id}/diagnosis?limit=10&include_content=false`
  returned authenticated product-path evidence; the appagent revision carried
  `metadata.source=edit_texture`; history included the appagent revision; the
  diagnosis/source surface returned the target document.
- Trace ordering: agents were conductor
  `conductor:f8052051-08d1-4d3a-a519-d6694ab3ad0e` first at
  `2026-06-15T23:03:14Z`, then Texture
  `vtext:a73a2f12-9e95-45fd-a75e-6ab50ab2ec80` at
  `2026-06-15T23:03:19Z`. No super agent appeared before Texture; no super
  agent appeared in this proof trajectory.
- Rollback ref: revert runtime commits `8d8ee883` and `be76501c` (plus
  docs-only checkpoint `d7b7e8e0` if reverting the mission record) to return to
  pre-runtime-cutover commit `53af096a`.

Open edge: continue reducing retired-name residue, rename UI/internal symbols,
prove pinned transclusion/newer-version behavior, delete compatibility shims
with receipts, and write Texture Protocol v0 only after the working surface is
settled.

## 2026-06-15 - Local Texture Transclusion Pin Slice

Claim: related Texture transclusions should carry an immutable version pin by
default, preserve that pin through editor serialization, open the pinned
revision when selected, and show when the related Texture has a newer head.

Move: implement the smallest frontend product slice at the existing
transclusion boundary: parse `vtext:<doc_id>@<revision_id>` refs, default a
document-only ref to the related Texture metadata's transclusion revision,
render pin/current-version attributes plus a visible newer-version note,
serialize refs back with the pinned revision, and pass `initialRevisionId`
when launching the related Texture.

Expected ΔV: 0 until pushed CI/deploy and staging product proof pass. This is
the construct portion of the transclusion obligation, not its deployed proof.

Actual ΔV: 0. V remains 6 because the slice has only local focused proof and
frontend build proof.

Receipts:
- `npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js --grep "related VText"`: pass, 2 tests.
- `npm --prefix frontend run build`: pass; existing Svelte warnings remained
  in `UniversalWireApp.svelte`.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,146 warnings.
- `/tmp/choir-doccheck.json` warning counts: H1=724, H3=15, H4=3, H5=347,
  R3=57.
- `git diff --check`: pass.

Open edge: push the slice, monitor CI and Node B staging deploy identity, then
run a staging browser/product proof that a related Texture transclusion renders
with a pinned revision and a newer-version indicator.

## 2026-06-15 - Staging Proof For Texture Transclusion Pins

Claim: the transclusion pinned-ref/newer-version slice is deployed and proves
the product invariant that related Texture transclusions pin a version by
default and show when a newer version exists.

Move: push commit `0cb42bb95efe8f92cc2d6ba921af19a62ee282e4`, monitor CI and
Node B deploy, verify staging build identity, then run a temporary authenticated
Playwright product proof against `https://choir.news`. The scratch spec was
deleted after the run; evidence was written outside the repo.

Expected ΔV: -1 by discharging the transclusion pinned-ref/newer-version proof.

Actual ΔV: -1. V moves from 6 to 5. This proof does not discharge old UI/file
symbol residue, compatibility-shim deletion, edit-affordance naming, or
Texture Protocol v0.

Receipts:
- pushed commit:
  `0cb42bb95efe8f92cc2d6ba921af19a62ee282e4`
  (`frontend: pin texture transclusion refs`).
- CI run `27582557591`: success. Runtime shards 0-3, non-runtime tests,
  integration-tagged smoke, Go vet/build, frontend build, Docs Truth Check job,
  TLA+ model check, final Go gate, and Node B staging deploy job passed.
- Docs Truth Check run `27582557606`: success.
- FlakeHub publish run `27582557566`: success.
- Staging health: `https://choir.news/health` reported proxy and sandbox
  commit `0cb42bb95efe8f92cc2d6ba921af19a62ee282e4`, deployed at
  `2026-06-15T23:15:09Z`.
- Staging acceptance command:
  `PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000 npm --prefix frontend run e2e -- tests/texture-transclusion-staging.tmp.spec.js`
  passed, 1 test, 13.7s.
- Staging evidence artifact:
  `/tmp/texture-transclusion-staging-proof-1781565407707.json`.
- Product evidence ids: parent Texture document
  `8bee5667-f59e-4081-a1bb-63a9094487d9`; parent revision
  `b081bc14-812c-43ac-9e0c-048cb50b095c`; child Texture document
  `7e6c3873-9716-496c-b4d0-dbab537f50f9`; pinned child revision
  `c554c39c-15c6-4b44-a733-f58d48f8029c` at version 0; current child head
  `71d79821-5420-498c-ab8d-eb0367e4b4ac` at version 1.
- Product observations: the parent Texture was opened through the desktop UI;
  its related Texture ref rendered `data-vtext-related-revision-id`,
  `data-vtext-related-version-number`,
  `data-vtext-related-current-revision-id`,
  `data-vtext-related-current-version-number`, and
  `data-vtext-related-has-newer-version="true"`; the inline transclusion showed
  the pinned child revision text and the newer-version note; clicking the
  related ref opened the child Texture at the pinned revision rather than the
  newer head.
- Rollback ref: revert commit `0cb42bb9` to return to document-only related
  Texture refs and unpinned related-document launches.

Open edge: continue reducing retired-name residue, rename UI/internal symbols,
prove edit-affordance common-vs-exceptional naming, delete compatibility shims
with receipts, and write Texture Protocol v0 only after the working surface is
settled.

## 2026-06-15 - Local Visible Texture UI Label Slice

Claim: visible product affordances can switch to Texture now while internal
`vtext` app ids, selectors, storage keys, route shims, and compatibility API
names remain explicitly deletion-receipted residue for later slices.

Move: update the app registry name/description, desktop launch/status/toast
copy, Texture editor recent/auth/error/status copy, published fallback copy,
Universal Wire launch/empty-state copy, Web Lens and Files import affordances,
source decision labels, related-Texture renderer fallbacks, signed-out preview
copy, and focused browser tests that assert the desktop app label.

Expected ΔV: 0 until pushed CI/deploy and staging browser proof pass. This is
the construct portion of the UI-label obligation, not its deployed proof.

Actual ΔV: 0. V remains 5 because the slice has only local build/doccheck
proof; local Playwright desktop tests could not run without the local Vite and
service stack on `localhost:4173`.

Receipts:
- `npm --prefix frontend run build`: pass; existing Svelte warnings remained
  in `UniversalWireApp.svelte`.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,146 warnings.
- `npm --prefix frontend run e2e -- tests/desktop-shell-core.spec.js --grep "Texture appears|Texture recent|Texture opens near"`: not product evidence; failed because `http://localhost:4173/` refused connection.
- `npm --prefix frontend run e2e -- tests/trace-settings-registry.spec.js`: not product evidence; failed because `http://localhost:4173/` refused connection.
- `git diff --check`: pass before the mission-record edit; rerun required
  before commit.

Open edge: push the slice, monitor CI and Node B staging deploy identity, then
run staging browser proof of visible Texture labels and import affordances.

## 2026-06-15 - Staging Proof For Visible Texture UI Labels

Claim: the visible product UI now presents the artifact surface as Texture
without requiring internal `vtext` ids, selectors, storage keys, or compatibility
routes to be deleted in the same slice.

Move: push commit `78bbcd7ec24d65ab7e17c111ce23ca7731b89003`, monitor CI and
Node B deploy, verify staging build identity, then run a temporary authenticated
Playwright product proof against `https://choir.news`. The scratch spec was
deleted after the run; evidence was written outside the repo.

Expected ΔV: -1 by discharging the visible UI label/browser-proof obligation.

Actual ΔV: -1. V moves from 5 to 4. This proof does not discharge internal
symbol residue, compatibility-shim deletion, edit-affordance naming, final
retired-name receipts, or Texture Protocol v0.

Receipts:
- pushed commit:
  `78bbcd7ec24d65ab7e17c111ce23ca7731b89003`
  (`frontend: show texture in artifact UI`).
- CI run `27583078805`: success. Runtime shards 0-3, non-runtime tests,
  integration-tagged smoke, Go vet/build, frontend build, Docs Truth Check job,
  TLA+ model check, final Go gate, and Node B staging deploy job passed.
- Docs Truth Check run `27583078889`: success.
- FlakeHub publish run `27583078824`: success.
- Staging health: `https://choir.news/health` reported proxy and sandbox
  commit `78bbcd7ec24d65ab7e17c111ce23ca7731b89003`, deployed at
  `2026-06-15T23:27:38Z`.
- Staging acceptance command:
  `PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000 npm --prefix frontend run e2e -- tests/texture-ui-label-staging.tmp.spec.js`
  passed, 1 test, 14.8s.
- Staging evidence artifact:
  `/tmp/texture-ui-label-staging-proof-1781566172780.json`.
- Product observations: desktop artifact icon label `Texture`; artifact window
  title `Texture`; recent landing eyebrow `TEXTURE`; Files import button
  `Texture` with title `Open texture-ui-label-1781566172780.pdf in Texture`;
  Web Lens snapshot import button `Open in Texture`; Web Lens source URL
  `https://example.com/texture-ui-label-1781566172780`.
- Rollback ref: revert commit `78bbcd7e` to return visible product labels to
  the previous V-name surface.

Open edge: continue reducing retired-name residue, cut over internal symbols
and compatibility shims with deletion receipts, prove edit-affordance
common-vs-exceptional naming, and write Texture Protocol v0 only after the
working surface is settled.

## 2026-06-15 - Local Texture Write Tool Split

Claim: the canonical Texture writer should see the common mutation as
`patch_texture` and reserve `rewrite_texture` for exceptional whole-document
recovery, while `edit_texture` remains only as a temporary compatibility alias
with deletion receipts.

Move: add `patch_texture` and `rewrite_texture` registered tools, force the
initial Texture tool choice to `patch_texture`, require rationale for
`rewrite_texture`, keep duplicate side-effect protection across all Texture
write tools, and update prompt defaults, workflow verification, wire
eligibility, metadata, fixtures, and focused tests to recognize the split.

Expected ΔV: 0 until pushed CI/deploy and staging product proof pass. The local
construct supports the common-vs-exceptional naming conjecture but does not
settle it.

Actual ΔV: 0. V remains 4 because the new tool surface is locally tested but
not deployed or proven through staging product evidence.

Receipts:
- `nix develop -c go test ./internal/wirepublish`: pass.
- `nix develop -c go test ./internal/runtime -run 'TestInitialVTextToolChoiceUsesExactTools|TestInstallDefaultAgentToolsProfiles|TestVTextEditRevisionMetadataRecordsOperationEvidence|TestMaterializeVTextToolEditRequiresRationaleForLongRewrite|TestVTextAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestInitialVTextRunWritesFirstAppagentRevisionThroughEdit|TestPromptBarInitialDecisionThenEdit|TestHandlePromptBarInitialDecisionThenEdit|TestVTextPromptBarIntakeTreatsSeedAsInstructionsNotCanonicalProse|TestVTextPromptFocusesLongDirectUserEdits|TestSystemPromptForRun|TestBuildAppagentRevisionMetadata|TestRecordVTextDecision'`: pass.
- `nix develop -c go test ./internal/runtime -run 'TestVTextWorkflowVerifier|TestVerifyVText|TestWorkflowVerifier|TestUniversalWire|TestWire|TestProcessor|TestReconciler|TestBuildCoagentVTextRevisionPrompt'`: pass.
- `nix develop -c go test ./internal/runtime -run 'TestVTextAgentRevisionMutationCompletedOnlyOnce|TestEditVTextInitialWorkingRevisionDoesNotSmuggleRequiredContinuation|TestEditVTextExplicitResearcherDoesNotForceSpawnContinuation|TestEditVTextExplicitResearcherDoesNotForceSpawnAfterSuperBase|TestEditVTextExplicitResearcherFromBaseRevisionContentSurvivesWorkerPrompt|TestEditVTextExplicitResearcherFromSeedPromptSurvivesRequestIntent|TestEditVTextExplicitResearcherDoesNotDuplicateExistingResearcher'`: pass, no matching tests.
- `nix develop -c go test ./internal/runtime -run TestExecuteToolsSkipsDuplicateVTextEditsInSameTurn`: pass after updating the duplicate Texture-write notice assertion.
- `nix develop -c env SHARD_INDEX=0 TOTAL_SHARDS=4 scripts/go-test-runtime-shards`: pass. A prior full-shard run passed shards 1-3 and exposed the stale shard-0 duplicate-notice assertion fixed above.
- `npm --prefix frontend run build`: pass, with existing Svelte warnings in
  `UniversalWireApp.svelte`.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,146 warnings.
- `git diff --check`: pass.

Open edge: commit and push the construct, monitor CI/deploy, then prove on
staging that prompt-bar -> conductor -> Texture first revision uses
`patch_texture` metadata and Trace rather than the compatibility alias.

## 2026-06-15 - Staging Proof For Texture Write Tool Split

Claim: deployed Texture first-revision creation now uses the common
`patch_texture` affordance instead of the compatibility `edit_texture` alias,
while `rewrite_texture` remains available only for exceptional full-document
recovery with rationale.

Move: push commit `7d697e477c9e0c81c30629267743e231395e812c`, monitor CI and
Node B deploy, verify staging build identity, then run a temporary
authenticated Playwright product proof against `https://choir.news`. The
scratch spec was deleted after the run; evidence was written outside the repo.

Expected ΔV: -1 by discharging the common-vs-exceptional edit-affordance proof
for the deployed common path.

Actual ΔV: -1. V moves from 4 to 3. This proof does not discharge high-read
docs, internal symbol residue, compatibility-shim deletion, final retired-name
receipts, or Texture Protocol v0.

Receipts:
- pushed commit:
  `7d697e477c9e0c81c30629267743e231395e812c`
  (`runtime: split texture write tools`).
- CI run `27584278584`: success. Runtime shards 0-3, non-runtime tests,
  integration-tagged smoke, Go vet/build, Docs Truth Check job, TLA+ model
  check, final Go gate, and Node B staging deploy job passed.
- Docs Truth Check run `27584278581`: success.
- FlakeHub publish run `27584278590`: success.
- Staging health: `https://choir.news/health` reported proxy and sandbox
  commit `7d697e477c9e0c81c30629267743e231395e812c`, deployed at
  `2026-06-15T23:56:42Z`.
- Staging acceptance command:
  `PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000 npm --prefix frontend run e2e -- tests/texture-write-tool-staging.tmp.spec.js`
  passed, 1 test, 23.1s.
- Staging evidence artifact:
  `/tmp/texture-write-tool-staging-proof-1781567962356.json`.
- Product evidence ids: submission
  `4c2d66f3-f090-4a6d-aa85-d90f92fd62da`; Texture document
  `a8259528-5a53-4e89-ac7c-e76d8a8cc59a`; user/base revision
  `2567a5d3-d976-44e1-ba5d-08ca163ac665`; appagent revision
  `eb11c5c0-985c-4b5d-ab15-cc43481c7241`; Texture loop
  `9ab5792e-430c-4685-914f-61482ad9a4b0`.
- Product observations: the visible prompt bar created a Texture document for
  marker `TEXTURE_WRITE_TOOL_1781567953855`; the appagent revision content
  preserved the marker; revision metadata recorded `source=patch_texture`,
  `texture_edit_tool=patch_texture`, and `vtext_edit_operation=apply_edits`;
  Trace roles included `conductor` and `vtext`; the successful
  `patch_texture` tool result stored the appagent revision; successful
  `edit_texture` result count was 0.
- Run acceptance synthesis:
  `POST /api/run-acceptances/synthesize` returned
  `runacc-89335b43c1f6e244362f` for trajectory
  `4c2d66f3-f090-4a6d-aa85-d90f92fd62da`, loop
  `9ab5792e-430c-4685-914f-61482ad9a4b0`, at
  `staging-smoke-level/blocked`. Product-path invariants
  `product_path_observed`, `worker_mutation_bounded`,
  `promotion_not_overclaimed`, and `checkpoint_causal_order` passed; the record
  remained blocked on stronger export/continuation-level contracts that this
  prompt/Texture proof did not attempt.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,146 warnings.
- `git diff --check`: pass.
- Rollback ref: revert commit `7d697e47` to return the Texture writer to the
  single compatibility `edit_texture` affordance.

Open edge: continue high-read documentation reconciliation, internal
symbol/storage/data-attribute cleanup, compatibility-shim deletion with
receipts, final retired-name search, and Texture Protocol v0.

## 2026-06-16 - High-Read Texture Docs Reconciliation

Claim: the current high-read docs can teach Texture as the artifact
control-plane ontology while preserving old-name occurrences only as explicitly
classified historical mission paths, internal detector symbols, or compatibility
route deletion targets.

Move: rewrite README, docs index, current architecture, runtime invariants, and
mission portfolio prose from the retired ontology to Texture; label unavoidable
old mission graph ids/paths, doctrine detector paths/symbols, docs-index
historical paths, and public `/pub/vtext` route references with line-local
deletion receipts; update this paradoc state.

Expected ΔV: -1 by discharging the high-read doctrine/index docs obligation.

Actual ΔV: -1. V moves from 3 to 2. This does not discharge internal
runtime/storage/file/UI data-attribute symbols, compatibility-shim deletion,
final retired-name receipts, or Texture Protocol v0.

Receipts:
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,132 warnings;
  warning counts `H1=718`, `H3=15`, `H4=3`, `H5=339`, `R3=57`.
- High-read H5 subset query over `README.md`, `docs/README.md`,
  `docs/choir-doctrine.md`, `docs/current-architecture.md`,
  `docs/runtime-invariants.md`, `docs/mission-portfolio-2026-06-11.md`,
  `docs/mission-graph.yaml`, and this paradoc returned no rows.
- `rg -n -i "vtext|/api/vtext|\\.vtext|edit_vtext|data-vtext|vtext_"`
  over the same high-read set now shows only line-labeled historical
  docs/mission paths, internal detector symbols, or compatibility route
  deletion targets.

Open edge: cut over internal runtime/storage/file/UI data-attribute names and
compatibility shims toward Texture; protocol v0 remains intentionally unwritten
until the working surface is settled.

## 2026-06-16 - Frontend Selector And Probe Cutover

Claim: the frontend DOM/test selector surface can stop teaching the old
artifact name without changing visible product copy, canonical write semantics,
or backend compatibility behavior.

Move: mechanically rename frontend `data-vtext-*` attributes and matching
Playwright selectors to `data-texture-*`, and move frontend tests/probes from
`/api/vtext` to `/api/texture`. Leave backend route shims, app ids, filenames,
metadata keys, and storage symbols for later backend/internal passes.

Expected ΔV: 0 against the coarse mission variant, but a bounded descent on the
internal-symbol sub-surface: remove frontend old-name DOM selectors and
frontend `/api/vtext` probes.

Actual ΔV: 0 against V=2. The UI data-attribute/probe slice is discharged, H5
falls from 339 to 335, and the remaining internal-symbol obligation is narrower
but still open.

Receipts:
- pushed commit:
  `ef0d33d039a0e1ac0216a4a0bacd41bcae61664b`
  (`frontend: cut texture selectors to current naming`).
- `rg -n "data-vtext|/api/vtext" frontend/src frontend/tests`: no matches.
- `npm --prefix frontend run build`: pass, with the pre-existing
  `UniversalWireApp.svelte` unused export/CSS warnings.
- Static preview DOM probe against `http://127.0.0.1:5173`: rendered
  `data-texture-editor`, `data-texture-toolbar`, and
  `data-texture-editor-area`; rendered zero `data-vtext-editor`,
  `data-vtext-toolbar`, or `data-vtext-editor-area`; page HTML contained no
  `/api/vtext` string.
- Attempted focused Playwright test
  `PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 npm --prefix frontend run e2e -- tests/prompt-surface-registry.spec.js -g "logged-out shell uses PromptSurface"`
  failed before reaching the renamed Texture selectors because static preview
  rendered one `[data-window-tray-item]` while the existing test expects three.
  Treat this as a local fixture mismatch, not selector-cutover evidence.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,128 warnings;
  warning counts `H1=718`, `H3=15`, `H4=3`, `H5=335`, `R3=57`.
- `git diff --check`: pass.
- CI run `27585451872`: success. Go vet/build, runtime shards 0-3,
  non-runtime tests, integration-tagged smoke, TLA+ model check, Docs Truth
  job, frontend build, final Go gate, and Node B staging deploy job passed.
- Separate Docs Truth Check run `27585451874`: success.
- FlakeHub publish run `27585451873`: success.
- Staging health: `https://choir.news/health` reported proxy and sandbox
  commit `ef0d33d039a0e1ac0216a4a0bacd41bcae61664b`, deployed at
  `2026-06-16T00:25:41Z`.
- Deployed DOM probe against `https://choir.news`: rendered
  `data-texture-editor`, `data-texture-toolbar`, and
  `data-texture-editor-area`; rendered zero `data-vtext-editor`,
  `data-vtext-toolbar`, or `data-vtext-editor-area`; page HTML contained no
  `/api/vtext` string.
- Rollback ref: revert commit `ef0d33d0` to restore frontend `data-vtext-*`
  selectors and `/api/vtext` frontend test probes.

Open edge: backend/runtime route shims, app ids, filenames, storage symbols,
metadata keys, platform/internal publication names, and final protocol v0
remain open.

## 2026-06-16 - Browser-Public Route Shim Deletion

Claim: Choir can delete the browser-public `/api/vtext` compatibility route and
the matching `product_api_request` allowlist entry while preserving the current
Texture API route behavior under `/api/texture`.

Move: remove public `/api/vtext/documents`, `/api/vtext/*`, and
`product_api_request` allowlist registration for `/api/vtext/`; keep
`/api/texture/documents` and `/api/texture/*` registered; add registered-mux
tests proving Texture create/read behavior and 404s for the retired public
route; add tool tests proving `product_api_request` refuses the retired route;
repair one Universal Wire registered-server test to use `/api/texture`.

Expected ΔV: 0 against the coarse mission variant, with a bounded descent on
the route-shim sub-surface. The browser-public route compatibility path should
be gone, but internal normalization, storage, app ids, file names, metadata,
platform/internal publication symbols, and Texture Protocol v0 remain open.

Actual ΔV: 0 against V=2. The public route shim and product API tool allowlist
shim are discharged. CI exposed one stale registered-server test path, which
was repaired before forced staging deploy. Authenticated staging storage CRUD
was not reproven because available Playwright auth states had expired; deployed
proof covers route topology and auth-gate behavior, with authenticated behavior
covered by local/CI registered-mux tests.

Receipts:
- pushed behavior commit:
  `fddc1be439837006a7b6abb7c71829a58ad48d36`
  (`runtime: remove legacy vtext public route`).
- pushed test-repair commit:
  `f704403dbffcbe8f7d488905e4cea0d14e121315`
  (`test: use texture route for wire read proof`).
- Focused local runtime tests passed:
  `nix develop -c go test ./internal/runtime -run 'TestHandleVTextDocumentsRootUsesTextureRoutes|TestRegisteredTextureRoutesExcludeLegacyVTextPrefix|TestRegisteredPublicRoutesExcludeLegacyRuntimeAPIs|TestProductAPIRequestToolUsesRunOwnerForAllowedProductRoute|TestProductAPIRequestToolRefusesInternalAndNonSuperCalls|TestProductAPIRequestToolRefusesLegacyVTextRoute'`.
- Focused post-repair local runtime tests passed:
  `nix develop -c go test ./internal/runtime -run 'TestResolveUniversalWireVTextReadOwnerAllowsEditionTranscludedPlatformDoc|TestRegisteredTextureRoutesExcludeLegacyVTextPrefix|TestProductAPIRequestToolRefusesLegacyVTextRoute'`.
- Local runtime shard 2 passed:
  `nix develop -c env SHARD_INDEX=2 TOTAL_SHARDS=4 scripts/go-test-runtime-shards`.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,128 warnings;
  warning counts remained `H1=718`, `H3=15`, `H4=3`, `H5=335`, `R3=57`.
- Post-evidence docs check for this ledger update:
  `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json` completed report-only with 212 docs and 1,129
  warnings: `H1=718`, `H3=15`, `H4=3`, `H5=336`, `R3=57`. The one additional
  H5 warning is this historical route-deletion evidence.
- `git diff --check`: pass.
- Residue search:
  `rg -n 's\\.HandleFunc\\("/api/vtext|Temporary compatibility shim during the Texture route cutover|"/api/vtext/"' internal/runtime/api.go internal/runtime/tools_product_api.go internal/runtime/*test.go`
  returned only internal `normalizeTextureAPIPath` mapping from
  `/api/texture/` to `/api/vtext/`.
- Residue search:
  `rg -n 'registeredRuntimeRequest\\([^\\n]+/api/vtext' internal/runtime -g '*_test.go'`
  returned no matches after the test repair.
- CI run `27585762825` for `fddc1be4` failed runtime shard 2 because
  `TestResolveUniversalWireVTextReadOwnerAllowsEditionTranscludedPlatformDoc`
  still called the retired registered `/api/vtext` route.
- CI run `27585924913` for `f704403d` succeeded across Go vet/build,
  non-runtime tests, runtime shards 0-3, integration smoke, TLA+, Docs Truth,
  and aggregate gate. Build Frontend and Deploy were skipped because the
  second push was test-only relative to the failed behavior commit.
- Forced staging deploy CI run `27586013632` with `force_staging_deploy=true`
  succeeded, including frontend build, aggregate gate, and Deploy to Staging
  job `81556557472`.
- Deploy job `81556557472` checked out
  `f704403dbffcbe8f7d488905e4cea0d14e121315`, completed the NixOS switch,
  refreshed active computers `vm-universal-wire-platform` and
  `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19`, and reported staging health OK.
- Staging health at `https://choir.news/health` reported proxy and sandbox
  commit `f704403dbffcbe8f7d488905e4cea0d14e121315`, deployed at
  `2026-06-16T00:40:30Z`.
- Deployed same-origin Playwright route probe with the existing expired
  `choir-news` auth state returned:
  `/api/session` -> 401 JSON `{"error":"authentication required"}`;
  `/api/texture/documents` -> 401 JSON `{"error":"authentication required"}`;
  `/api/vtext/documents` -> 404 plain text `404 page not found`;
  `/api/vtext/diff` -> 404 plain text `404 page not found`.
- Rollback ref: revert commits `f704403d` and `fddc1be4` to restore the
  registered browser-public `/api/vtext` route and product API tool allowlist
  compatibility path.

Open edge: internal `normalizeTextureAPIPath`, app ids, filenames, storage
symbols, metadata keys, platform/internal publication names, `edit_texture`
compatibility alias, and final Texture Protocol v0 remain open.

## 2026-06-16 - Registered Router Normalization Cutover

Claim: after the public `/api/vtext` route deletion, the registered Texture
router and direct handler tests can stop internally normalizing Texture paths
through `/api/vtext` without changing Texture API behavior.

Move: remove `normalizeTextureAPIPath`, route `HandleVTextRouter` directly on
`/api/texture`, make document/revision ID extraction require `/api/texture`,
update Texture API route comments, and mechanically move direct VText API tests
from `/api/vtext` to `/api/texture`. Preserve explicit legacy-route refusal
tests for registered routes and `product_api_request`.

Expected ΔV: 0 against the coarse V=2, with a bounded descent on the internal
registered-router normalization sub-surface.

Actual ΔV: 0 against V=2. The registered-router/extractor old-route
normalization slice is discharged and deployed. Storage tables, file names,
app ids, metadata keys, platform/internal publication symbols,
role/type/function names, the `edit_texture` compatibility alias, and Texture
Protocol v0 remain open.

Receipts:
- pushed behavior commit:
  `247e28415bb7b5a656b9d83072288403666c9c8a`
  (`runtime: dispatch texture routes without legacy normalization`).
- Focused local runtime tests passed:
  `nix develop -c go test ./internal/runtime -run 'TestRegisteredTextureRoutesExcludeLegacyVTextPrefix|TestRegisteredPublicRoutesExcludeLegacyRuntimeAPIs|TestProductAPIRequestToolRefusesLegacyVTextRoute|TestHandleVTextDocumentsRootUsesTextureRoutes|TestVTextAPI(GetDocument|CreateRevisionUserEdit|GetHistory|GetDiff)|TestVTextAPIAuthGating'`.
- Local runtime shard script passed:
  `nix develop -c scripts/go-test-runtime-shards`. The visible stream showed
  shard 0/4, 1/4, and 2/4 passing before completion.
- Explicit local runtime shard 3 passed:
  `nix develop -c env SHARD_INDEX=3 TOTAL_SHARDS=4 scripts/go-test-runtime-shards`.
- Residue search:
  `rg -n 'normalizeTextureAPIPath|/api/vtext|vtext endpoint not found|legacyVText.*PathPrefix' internal/runtime/api.go internal/runtime/vtext.go internal/runtime/vtext_agent_revision.go internal/runtime/vtext_diagnosis.go internal/runtime/*_test.go`
  returned only explicit `/api/vtext` refusal tests in `api_test.go` and
  `tools_product_api_test.go`.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,129 warnings;
  warning counts `H1=718`, `H3=15`, `H4=3`, `H5=336`, `R3=57`.
- `git diff --check`: pass.
- CI run `27587124142` for `247e2841` succeeded across Go vet/build,
  non-runtime tests, runtime shards 0-3, integration smoke, TLA+, Docs Truth,
  aggregate gate, and Node B staging deploy job `81560043847`.
- Staging health at `https://choir.news/health` reported proxy and sandbox
  commit `247e28415bb7b5a656b9d83072288403666c9c8a`, deployed at
  `2026-06-16T01:10:47Z`.
- Unauthenticated curl route probe against staging returned the uniform auth
  gate for all three API paths:
  `/api/texture/documents` -> 401 JSON `{"error":"authentication required"}`;
  `/api/vtext/documents` -> 401 JSON `{"error":"authentication required"}`;
  `/api/vtext/diff` -> 401 JSON `{"error":"authentication required"}`.
  Caller-supplied `X-Authenticated-User` and `X-Authenticated-Email` headers
  were ignored and returned the same 401s, confirming the public proxy did not
  permit trusted-header spoofing.
- Authenticated same-origin Chrome route proof was attempted from an existing
  `choir.news` tab, but the browser automation evaluation sandbox exposed no
  `fetch` or `XMLHttpRequest`, and direct API URL navigation was blocked with
  `net::ERR_BLOCKED_BY_CLIENT`. Therefore deployed authenticated legacy-route
  404 behavior for this internal dispatch slice remains covered by local/CI
  registered-router tests rather than a browser status-code probe.
- Rollback ref: revert commit `247e2841` to restore the internal
  `normalizeTextureAPIPath` compatibility mapping and direct handler test
  paths.

Open edge: app ids, filenames, storage symbols, metadata keys,
platform/internal publication names, `edit_texture` compatibility alias, and
final Texture Protocol v0 remain open.

## 2026-06-16 - Platform Publication Route Residue Checkpoint

Claim: after the main Texture API route cutover, platform publication control
routes remain a distinct old-name route family that can be cut over without
renaming live public article URLs.

Move: read-only route inventory and Problem Documentation First checkpoint
before touching proxy, platformd, Wire autonomous publication, or frontend
publish behavior.

Expected ΔV: 0 against V=2, but it selects the next bounded descent on the
platform/internal publication-symbol sub-surface.

Actual ΔV: 0. The problem is documented and the next slice is scoped: hard-cut
control routes to `/texture` naming, keep `/pub/vtext/...` as live route
identity until a redirect/rollback policy exists.

Conjecture delta: publication control routes can stop teaching the retired
artifact name while preserving existing public route identity.

Protected surfaces: public proxy API routing, platformd internal routing, Wire
autonomous publication, publication read/sync paths, and deployment routing.

Admissible evidence class: focused proxy/platform/runtime route tests,
frontend build or focused caller tests if touched, CI, Node B deploy identity,
and staging route probes for old/new control routes.

Rollback path: revert the future behavior commit to restore the old platform
publication control routes.

Heresy delta: discovered old-name publication control route residue; no repair
claimed yet.

Receipts:
- Read-only search:
  `rg -n 'api/platform/vtext|internal/platform/(publications/vtext|vtext)|internal/wire/platform/publications/vtext|/pub/vtext|HandleVTextPublication|HandlePlatformVTextRead|isPlatformVTextReadRequest|PublishVText|SyncVText|PlatformVText' internal/proxy internal/platform internal/wirepublish internal/runtime frontend/src/lib/vtext.js frontend/src/App.svelte frontend/src/lib/Desktop.svelte`
  showed the old-name platform publish route, internal wire publish route,
  platformd publish/sync/read routes, and `/pub/vtext/...` public reader URL
  shape.

Open edge: implement and land the platform publication control-route cutover,
then continue storage/app-id/file/metadata naming and protocol v0.

## 2026-06-16 - Platform Publication Control Route Cutover

Claim: platform/proxy/internal publication control routes can stop teaching the
retired artifact name while preserving existing `/pub/vtext/...` published
reader URLs as live route identity for a separate migration plan.

Move: rename the public platform publish caller and proxy dispatch to
`/api/platform/texture/publications`; rename the internal Wire publish route to
`/internal/wire/platform/publications/texture`; rename platformd publish, sync,
document-read, and revision-read routes to `/internal/platform/texture...`;
switch publication private reads to `/api/texture`; add explicit old-route
absence checks.

Expected ΔV: 0 against coarse V=2, with bounded descent on the
platform/internal publication-symbol sub-surface.

Actual ΔV: 0 against V=2. The platform publication control-route sub-surface is
discharged and deployed. Storage tables, file names, app ids, metadata keys,
`/pub/vtext/...` public route identity, `edit_texture` compatibility alias, and
Texture Protocol v0 remain open.

Conjecture delta: supported for deployed route-control scope that publication
control routes can move to Texture naming without renaming live public article
URLs.

Protected surfaces: public proxy API routing, platformd internal routing, Wire
autonomous publication, publication read/sync paths, frontend publish caller,
and deployment routing.

Admissible evidence class reached: local route-focused tests, full
touched-package tests, frontend build, runtime shard evidence, report-only
doccheck, residue search, diff check, CI, Node B deploy identity, and staging
route probe.

Receipts:
- Focused local route tests passed:
  `nix develop -c go test ./internal/proxy ./internal/platform ./internal/wirepublish ./internal/runtime -run 'TestHandleVTextPublicationReadsPrivateRevisionAndPostsProjection|TestHandleVTextPublicationRejectsMalformedPolicy|TestHandleVTextPublicationPublishesPublicURLSourceSnapshots|TestHandleVTextPublicationRecordsURLSnapshotImportFailureState|TestHandleVTextPublicationDoesNotPublishPrivateSourceSnapshots|TestHandleInternalWirePlatformPublishPostsToPlatformd|TestIsPlatformVTextReadRequest|TestProtectedAPIResolveTarget_VTextReadsNotRoutedThroughSandbox|TestInternalPublishRequiresInternalCallerAndBundleResolve|TestRegisteredTextureRoutesExcludeLegacyVTextPlatformPrefix|TestHandleUniversalWireStoriesDoesNotIndexUntranscludedPlatformVTexts|TestHandleUniversalWireStoriesSkipsTranscludedUnpublishedPlatformVTexts|TestResolveUniversalWireVTextReadOwnerAllowsEditionTranscludedPlatformDoc|TestPublishWireArticleToPlatform|TestPostPlatformPublication'`.
- Full touched-package tests passed:
  `nix develop -c go test ./internal/proxy ./internal/platform ./internal/wirepublish`.
- Local runtime shard script passed:
  `nix develop -c scripts/go-test-runtime-shards`.
- Frontend production build passed:
  `npm --prefix frontend run build`; pre-existing `UniversalWireApp.svelte`
  unused export/CSS selector warnings and the chunk-size warning remained.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json`: report-only complete, 212 docs, 1,129 warnings.
- Residue search:
  `rg -n "api/platform/vtext|platformPath\\('/vtext|internal/platform/(publications/vtext|vtext)|internal/wire/platform/publications/vtext|/api/vtext" frontend/src/lib/vtext.js internal/proxy internal/platform internal/wirepublish internal/runtime/wire_platform_publish.go internal/runtime/universal_wire.go`
  returned only explicit deletion receipts or negative tests:
  the proxy 404 branch for `/api/platform/vtext/publications`, legacy publish
  route 404 test, old platformd registered-route absence tests, and old
  `/api/vtext` platform-read negative cases.
- `git diff --check`: pass.
- Pushed behavior commit:
  `019e7a9d78f94e78da91ae2ddc6200dd7dee0184`
  (`runtime: cut platform publication routes to texture`).
- CI run `27587958358` passed across Go vet/build, non-runtime tests, runtime
  shards 0-3, integration smoke, TLA+, Docs Truth, frontend build, aggregate
  gate, and Node B staging deploy job `81562610983`.
- Staging health at `https://choir.news/health` reported proxy and sandbox
  commit `019e7a9d78f94e78da91ae2ddc6200dd7dee0184`, deployed at
  `2026-06-16T01:33:28Z`.
- Unauthenticated staging route probe returned:
  `GET /api/platform/texture/publications -> 405 {"error":"method not allowed"}`;
  `POST /api/platform/texture/publications -> 401 {"error":"authentication required"}`;
  `GET /api/platform/vtext/publications -> 404 {"error":"not found"}`;
  `POST /api/platform/vtext/publications -> 404 {"error":"not found"}`.

Rollback path: revert the future behavior commit to restore the old platform
publication control routes and private publication reads.

Heresy delta: repaired for deployed route-control scope; no storage/public-route
or protocol repair claimed.

Open edge: continue storage/app-id/file/metadata naming, `/pub/vtext/...` route
identity migration policy, `edit_texture` compatibility alias deletion, and
protocol v0.

## 2026-06-16 - App Identity And Storage Symbol Residue Checkpoint

Claim: after public route, visible label, selector, and platform publication
cutovers, app identity is the next bounded residue target, while storage tables,
workspace/file suffixes, and metadata symbols are too broad to rename in the
same move without a separate migration plan.

Move: read-only inventory and Problem Documentation First checkpoint before
touching frontend app registry, persisted desktop app ids, source-open plans,
auth intents, storage schema, or metadata keys.

Expected ΔV: 0 against V=2, but it selects the next bounded descent on the
app-identity sub-surface.

Actual ΔV: 0. The problem is documented and the next slice is scoped: canonical
new app launches should use `texture`, legacy persisted/URL `vtext` app ids
should normalize to Texture, and deeper storage/table/file/metadata names stay
out of the slice.

Conjecture delta: canonical app identity can move to Texture without stranding
existing persisted desktop windows, if legacy app ids are normalized at the
desktop-state and app-launch boundaries.

Protected surfaces: app registry, desktop window persistence/restore,
source-open app selection, auth intent replay, public preview windows, frontend
routing, and deployment routing.

Admissible evidence class: focused frontend build/tests and Go desktop-state
tests if backend normalization is touched; CI; Node B deploy identity; staging
browser/DOM proof that new Texture app surfaces use `data-app-id="texture"` and
legacy `app=vtext` or saved state still opens Texture.

Rollback path: revert the future behavior commit to restore canonical `vtext`
app ids and remove any normalization shim.

Heresy delta: discovered app identity and storage symbol residue; no repair
claimed yet.

Receipts:
- Path inventory excluding `frontend/dist`:
  `rg --files | rg '(^|/)[^/]*(vtext|VText|VTEXT)[^/]*$|\\.vtext' | rg -v '^frontend/dist/' | wc -l`
  returned 103.
- App id search:
  `rg -n "appId: 'vtext'|app_id.*\\\"vtext\\\"|AppID: +\\\"vtext\\\"|id: 'vtext'|appId === 'vtext'|app_id=\\\"vtext\\\"" frontend/src frontend/tests internal -g '*.go' -g '*.ts' -g '*.js' -g '*.svelte' | rg -v '^frontend/dist/' | wc -l`
  returned 38.
- Storage symbol search:
  `rg -n 'vtext_documents|vtext_revisions|vtext_document_aliases|vtext_agent_mutations|vtext_controller_checkpoints|vtext_decisions|CREATE DATABASE IF NOT EXISTS vtext|database=vtext|\\.vtext|go-choir-vtext' internal cmd frontend/src frontend/tests specs docs -g '!docs/why-texture-background-2026-06-15.md' -g '!docs/mission-texture-hard-cutover-v0.ledger.md' | rg -v '^frontend/dist/' | wc -l`
  returned 1,009.
- Metadata/tool search:
  `rg -n -e 'edit_vtext' -e 'vtext_ref' -e 'vtext_doc' -e 'vtext_revision' -e 'source_vtext' -e 'platformd_route_path' -e 'related_vtext' -e 'transcluded_vtext' -e 'vtext_' internal frontend/src frontend/tests cmd specs docs -g '!docs/why-texture-background-2026-06-15.md' -g '!docs/mission-texture-hard-cutover-v0.ledger.md' | rg -v '^frontend/dist/' | wc -l`
  returned 791.
- Selected app-id files:
  `frontend/src/lib/apps/registry.ts`, `frontend/src/App.svelte`,
  `frontend/src/lib/Desktop.svelte`, `frontend/src/lib/UniversalWireApp.svelte`,
  `frontend/src/lib/source-contract.ts`, `frontend/src/lib/VTextEditor.svelte`,
  `internal/store/desktop_test.go`, `internal/runtime/desktop_test.go`, and
  `internal/store/store_test.go`.

Open edge: implement and land the app-id cutover, then return to storage
schema/workspace/file suffixes, metadata keys, `/pub/vtext/...` route identity,
`edit_texture` compatibility alias deletion, and protocol v0.

## 2026-06-16 - Local Texture App Identity Repair

Claim: the bounded app-identity slice can move to canonical `texture` without
renaming storage tables, workspace/file suffixes, metadata keys, or auth intent
kinds in the same move.

Move: change the frontend app registry id to `texture`; route app launch,
auth replay, source-open, public preview, Universal Wire, related Texture, and
desktop-store paths through that canonical id; normalize deletion-receipted
legacy `vtext` app ids at frontend launch/restore and runtime desktop-state API
boundaries.

Expected ΔV: support C15 for the app-identity sub-surface, with no global V
decrease claimed until the remaining residue classes are selected.

Actual ΔV: C15 is supported for deployed app-identity scope. The global mission
state remains open because storage schema/workspace/file suffixes, metadata
keys, `/pub/vtext/...` route identity, `edit_texture` compatibility alias
deletion, public preview Trace fixture agent ids, and protocol v0 remain open.

Conjecture delta: canonical app identity can move to Texture without stranding
existing persisted desktop windows, if legacy app ids are normalized at the
desktop-state and app-launch boundaries.

Protected surfaces: app registry, desktop window persistence/restore,
source-open app selection, auth intent replay, public preview windows, frontend
routing, and runtime desktop-state get/save.

Admissible evidence class: focused frontend build and Go desktop-state tests;
runtime shard suite; CI; Node B deploy identity; staging browser/DOM proof that
new Texture app surfaces use `data-app-id="texture"` and legacy app ids still
open Texture.

Rollback path: revert the behavior commit to restore canonical `vtext` app ids
and remove the normalization shims.

Heresy delta: repaired for deployed app identity; no storage/table/file/metadata
symbol repair claimed.

Receipts:
- `npm --prefix frontend run build` passed. Vite emitted existing warnings for
  unused `currentUser` and `.wire-state` selectors in `UniversalWireApp.svelte`.
- `nix develop -c go test -tags comprehensive -v ./internal/runtime -run '^TestDesktopState'`
  passed, including `TestDesktopStateSanitizesLegacyTextureAppID`.
- `nix develop -c scripts/go-test-runtime-shards` passed all four runtime
  shards.
- App-id residue search:
  `rg -n "appId: 'vtext'|appId: \"vtext\"|id: 'vtext'|id: \"vtext\"|openApp\\('vtext'|getAppIcon\\('vtext'|public-preview-vtext|data-app-id=\"vtext\"" frontend/src internal -g '!frontend/dist'`
  returned only public preview Trace fixture agent ids.
- Commit `f27c00154f4eb1025075cc6eb6b76383324dd5f1` passed CI run
  `27588733421`; deploy job `81564942700` succeeded.
- Separate `Docs Truth Check` run `27588733442` and FlakeHub publish run
  `27588733436` completed successfully for the same commit.
- Staging health at `https://choir.news/health` reported proxy and sandbox
  commit `f27c00154f4eb1025075cc6eb6b76383324dd5f1`, deployed at
  `2026-06-16T01:55:03Z`.
- Staging Playwright DOM proof for `https://choir.news/` returned:
  `textureWindows=1`, `legacyWindows=0`, `textureIcons=1`, `legacyIcons=0`,
  `previewWindowIds=["public-preview-texture"]`.
- Staging Playwright DOM proof for
  `https://choir.news/?app=vtext&doc=legacy-proof-doc&title=Legacy%20Texture`
  returned: `textureWindows=1`, `legacyWindows=0`,
  `bodyMentionsTexture=true`, `bodyMentionsVText=false`.

Open edge: select the next bounded residue class among storage
schema/workspace/file suffixes, metadata keys, `/pub/vtext/...` route identity,
`edit_texture` compatibility alias deletion, public preview Trace fixture agent
ids, and protocol v0.

## 2026-06-16 - Public Preview Trace Fixture Residue Checkpoint

Claim: the next smallest residue class is not a storage or runtime agent-id
migration; it is an unused public-preview Trace fixture that keeps the old
Texture actor id alive in signed-out fixture data.

Move: read-only inventory and Problem Documentation First checkpoint before
touching `frontend/src/lib/public-preview-data.ts`.

Expected ΔV: 0 global, but selects a bounded residue class with low blast
radius and high clarity.

Actual ΔV: 0. The problem is documented and the next slice is scoped: delete
the unused fixture exports rather than rename them.

Conjecture delta: deleting unused Trace preview fixture data better advances
Texture ontology than renaming its stale actor ids, because Trace is
evidence/topology and not a public product surface.

Protected surfaces: signed-out preview data, public desktop preview bundle, and
frontend build.

Admissible evidence class: frontend build and residue searches proving the
fixture exports have no consumers and no public-preview Trace `vtext` actor id
remains.

Rollback path: restore the fixture definitions if a real consumer is found.

Heresy delta: discovered unused Trace fixture residue; no repair claimed yet.

Receipts:
- `rg -n "agent_id: 'vtext'|to_agent_id: 'vtext'|from_agent_id: 'vtext'"`
  on `frontend/src/lib/public-preview-data.ts` found seven fixture hits.
- `rg -n "previewTraceSnapshot|previewTraceTrajectories" . -g '!frontend/dist' -g '!node_modules'`
  found only the fixture definitions themselves, with no consumers.
- The fixture acceptance text frames "Trace layout" as public preview data,
  conflicting with the current doctrine guardrail that Trace is evidence and
  topology, not a normal public surface.

Open edge: delete the unused fixture exports, verify frontend build and residue
searches, then select the next storage/API/protocol residue class.

## 2026-06-16 - Local Public Preview Trace Fixture Deletion

Claim: because `previewTraceTrajectories` and `previewTraceSnapshot` are unused
exports, deleting them is a better repair than renaming their actor ids.

Move: delete the unused public-preview Trace fixture exports from
`frontend/src/lib/public-preview-data.ts` while leaving the live
`previewVTextDocument` signed-out Texture preview intact.

Expected ΔV: support C16 locally; no global V decrease until CI/deploy evidence
is recorded.

Actual ΔV: pending deploy. Local build and residue searches support the repair.

Conjecture delta: deleting dead Trace fixture data avoids preserving Trace as a
public preview surface while removing stale Texture actor ids.

Protected surfaces: signed-out preview data module and frontend build.

Admissible evidence class: frontend build, residue searches, CI, and deploy
identity if the frontend source change triggers staging.

Rollback path: restore the deleted fixture exports if a real consumer is found.

Heresy delta: repaired locally for unused public-preview Trace fixture residue;
no durable runtime agent-id or storage-symbol repair claimed.

Receipts:
- `npm --prefix frontend run build` passed. Vite reported existing Universal
  Wire warnings for unused `currentUser` and `.wire-state` selectors.
- `rg -n "previewTraceSnapshot|previewTraceTrajectories|preview-trace|Trace layout|agent_id: 'vtext'|to_agent_id: 'vtext'|from_agent_id: 'vtext'" frontend/src/lib/public-preview-data.ts frontend/src -g '!frontend/dist'`
  returned no hits.

Open edge: push the fixture-deletion commit, monitor CI/deploy if triggered,
record evidence, then select the next storage/API/protocol residue class.

## 2026-06-16 - Deployed Public Preview Trace Fixture Deletion

Claim: the unused public-preview Trace fixture deletion is now supported at the
deployed slice, not merely local build/search scope.

Move: monitor CI/deploy for commit
`3037e1f92971e7324a8bb8c3e356474e4eee2cc6`, verify staging build identity,
and run a staging Playwright DOM proof of the signed-out Texture preview.

Expected ΔV: support C16 at deployed evidence class; no global V decrease
because the coarse remaining obligations are storage/file/metadata residue,
`/pub/vtext/...` route identity, `edit_texture` alias deletion, and protocol
v0.

Actual ΔV: C16 moved from locally supported to deployed supported. Global V
remains 2.

Conjecture delta: deleting the dead fixture preserves the doctrine boundary
that Trace is evidence/topology, not a public preview product surface, while
leaving the live signed-out Texture preview intact.

Protected surfaces: signed-out preview data module, frontend build, staging
deployment identity, and public desktop preview.

Admissible evidence class: CI, deploy identity, health build identity, and
staging browser DOM proof.

Rollback path: revert commit `3037e1f92971e7324a8bb8c3e356474e4eee2cc6` to
restore the fixture exports if a real consumer is found.

Heresy delta: repaired for deployed unused public-preview Trace fixture
residue; no durable runtime agent-id or storage-symbol repair claimed.

Receipts:
- CI run `27589138319` passed for
  `3037e1f92971e7324a8bb8c3e356474e4eee2cc6`; deploy job `81566163866`
  succeeded.
- Separate `Docs Truth Check` run `27589138321` and FlakeHub publish run
  `27589138328` completed successfully for the same commit.
- `https://choir.news/health` reported proxy and sandbox commit
  `3037e1f92971e7324a8bb8c3e356474e4eee2cc6`, deployed at
  `2026-06-16T02:06:07Z`.
- Staging Playwright DOM proof for `https://choir.news/` returned:
  `textureWindows=1`, `legacyWindows=0`, `textureIcons=1`,
  `legacyIcons=0`, `bodyMentionsTraceLayout=false`,
  `bodyMentionsPreviewTrace=false`, and `bodyMentionsVTextActor=false`.

Open edge: select the next bounded residue class among storage
schema/workspace/file suffixes, metadata keys, `/pub/vtext/...` route identity,
and `edit_texture` compatibility alias deletion. Protocol v0 remains last.

## 2026-06-16 - `edit_texture` Alias Deletion Checkpoint

Claim: `edit_texture` is a removable compatibility tool alias, but persisted
revision metadata with `source=edit_texture` or `source=edit_vtext` is a
different compatibility surface that should not be deleted in the same move.

Move: read Texture agentic invariants, inventory `edit_texture` in current code
and tests, and document the next behavior slice before touching runtime.

Expected ΔV: 0 global; C17 becomes active and the next runtime slice is scoped.

Actual ΔV: 0. Problem Documentation First checkpoint landed in docs only.

Conjecture delta: deleting the model-visible compatibility alias advances the
Texture tool ontology, but preserving explicit legacy metadata reads avoids
turning an affordance cleanup into a publication-history break.

Protected surfaces: Texture tool registry, canonical write metadata, tool loop
terminal handling, duplicate write protection, Universal Wire publication
eligibility, and autonomous publication read policy.

Admissible evidence class for the later behavior slice: focused runtime tests
covering tool-profile exposure, duplicate write protection, Texture revision
metadata, terminal tool handling, wire publication eligibility, CI, staging
deploy identity, and a deployed prompt-bar/Trace proof showing no successful
`edit_texture` tool result.

Rollback path for the later behavior slice: restore the `edit_texture`
registered tool and terminal/duplicate handling if deployed Texture writers
cannot use `patch_texture`/`rewrite_texture`.

Heresy delta: discovered alias/metadata coupling risk; no repair claimed yet.

Receipts:
- `docs/texture-agentic-invariants-2026-06-13.md` confirms Texture tool/write
  changes are protected and must not force semantic next-role workflow.
- `rg -n "edit_texture" internal/runtime internal/wirepublish internal/proxy cmd frontend/tests frontend/src -g '!frontend/dist/**'`
  found current non-doc hits only in `internal/runtime` and
  `internal/wirepublish`: 118 runtime hits and 7 wire-publish hits across 15
  code/test files.
- `internal/runtime/tools_vtext.go` registers
  `newEditTextureCompatibilityTool(rt)` and classifies `edit_texture` as a
  Texture write tool.
- `internal/runtime/tools.go` and `internal/runtime/runtime.go` still include
  `edit_texture` in sequential/duplicate/terminal write-tool handling.
- `internal/wirepublish/eligibility.go` and
  `internal/runtime/universal_wire.go` still accept legacy metadata sources
  `edit_texture` and `edit_vtext` for publication eligibility/read behavior.
- Test inventory found 112 `edit_texture` hits in current tests, so the repair
  must update tests intentionally rather than remove one registration and trust
  compile failures to find the semantic boundary.

Open edge: implement the runtime alias-deletion slice while retaining explicit
legacy metadata compatibility, then prove with focused runtime tests before CI
and staging.

## 2026-06-16 - Local `edit_texture` Alias Deletion

Claim: the model-visible `edit_texture` compatibility alias can be deleted
without deleting persisted legacy publication metadata compatibility.

Move: remove the registered `edit_texture` Texture tool, remove it from
terminal/duplicate/sequential write-tool handling, change untagged new-write
metadata fallback to `patch_texture`, update tests to use `patch_texture` or
`rewrite_texture`, and keep explicit `source=edit_texture` /
`source=edit_vtext` read compatibility.

Expected ΔV: support C17 locally; no global V decrease until CI/deploy and
staging prompt-bar evidence are recorded.

Actual ΔV: C17 is locally supported pending CI/deploy.

Conjecture delta: the common Texture write surface is now `patch_texture` plus
exceptional `rewrite_texture`; legacy source metadata compatibility remains a
read-side migration concern rather than a live tool affordance.

Protected surfaces: Texture tool registry, canonical write metadata fallback,
tool-loop terminal successes, duplicate write protection, Universal Wire
publication eligibility/read compatibility, and Texture appagent tests.

Admissible evidence class: focused runtime tests, wirepublish package tests,
runtime shards, CI, staging deploy identity, and deployed prompt-bar/Trace
proof showing current write tools and no successful `edit_texture` result.

Rollback path: restore the `edit_texture` registered tool, write-tool
classification, terminal success entry, duplicate-write handling entry, and
`edit_texture` metadata fallback.

Heresy delta: repaired locally for the model-visible compatibility alias;
legacy revision metadata compatibility remains discovered migration residue.

Receipts:
- `nix develop -c go test ./internal/runtime -run 'TestInstallDefaultAgentToolsProfiles|TestExecuteToolsSkipsDuplicateVTextEditsInSameTurn|TestVTextAppagentEditCanonicalizesAliasedMarkdownTitle|TestVTextAgentRevisionMutationCompletedOnlyOnce|TestEditVTextInitialWorkingRevisionDoesNotSmuggleRequiredContinuation|TestEditVTextExplicitResearcherDoesNotForceSpawnContinuation|TestEditVTextExplicitResearcherDoesNotForceSpawnAfterSuperBase|TestEditVTextExplicitResearcherFromBaseRevisionContentSurvivesWorkerPrompt|TestEditVTextExplicitResearcherFromSeedPromptSurvivesRequestIntent|TestEditVTextExplicitResearcherDoesNotDuplicateExistingResearcher|TestVTextTool|TestEmailAppagent'`
  passed.
- `nix develop -c go test ./internal/wirepublish` passed.
- `nix develop -c scripts/go-test-runtime-shards` passed all four runtime
  shards.
- Live-alias residue search
  `rg -n "newEditTextureCompatibilityTool|Name:\s+\"edit_texture\"|decode edit_texture args|executeTextureEditTool\(ctx, \"edit_texture\"|WithTerminalToolSuccesses\([^)]*edit_texture|case \"patch_texture\", \"rewrite_texture\", \"edit_texture\"|sourceTool = \"edit_texture\"" internal/runtime internal/wirepublish --glob '!frontend/dist/**'`
  returned no hits.
- Broad current-code search
  `rg -n "edit_texture" internal/runtime internal/wirepublish --glob '!frontend/dist/**'`
  now finds only explicit forbidden-tool assertions and legacy
  `source=edit_texture` metadata compatibility tests/read predicates.

Open edge: push the runtime repair, monitor CI/deploy, then run deployed
prompt-bar/Trace proof that Texture first revision uses current write tools and
does not produce a successful `edit_texture` result.

## 2026-06-16 - Deployed `edit_texture` Alias Deletion

Claim: the model-visible `edit_texture` compatibility alias can stay deleted
in the deployed product path while persisted legacy revision metadata remains
read-compatible.

Move: push commit `c6db0df57bd06a22e392fd89eb0f4ee1f4c1bcc1`, monitor CI and
deploy, verify staging build identity, and run a deployed prompt-bar/Trace
proof through public authenticated product APIs.

Expected ΔV: support C17 for deployed scope; no coarse V decrease because
storage/file/metadata/public-route/protocol residue remains.

Actual ΔV: C17 is supported for deployed alias-deletion scope. V remains 2.

Conjecture delta: current Texture writers can use `patch_texture` without the
retired compatibility alias; no super-before-Texture path is needed for the
first revision.

Protected surfaces: Texture tool registry, canonical Texture write metadata,
tool-loop terminal successes, duplicate Texture write protection, prompt-bar
route materialization, Trace evidence, and staging deployment identity.

Admissible evidence class: CI, deploy job, staging health identity, and
deployed browser product proof that submits through prompt-bar, observes
Texture head metadata, reads Trace over `/api/trace/*`, and finds no
`edit_texture` tool result.

Rollback path: restore the `edit_texture` registered tool, write-tool
classification, terminal success entry, duplicate-write handling entry, and
`edit_texture` metadata fallback if deployed Texture writers fail without the
alias.

Heresy delta: repaired for deployed model-visible alias exposure; legacy
`source=edit_texture` and `source=edit_vtext` metadata compatibility remains
discovered migration residue.

Receipts:
- CI run `27589732107` passed for commit
  `c6db0df57bd06a22e392fd89eb0f4ee1f4c1bcc1`.
- Deploy job `81567905099` succeeded.
- `https://choir.news/health` reported proxy and sandbox commit
  `c6db0df57bd06a22e392fd89eb0f4ee1f4c1bcc1`, deployed at
  `2026-06-16T02:22:51Z`.
- Deployed Playwright product proof registered a fresh user, submitted
  prompt-bar request `d2a0ccf4-276f-43f2-be6b-f6da43fdaf15`, and received a
  conductor -> Texture decision for document
  `d4e62340-bd4c-4644-9fd6-fb28a2b85d30`.
- Texture head revision `f5fee46f-4178-4dc2-aee3-fe127525cd9b` had
  `metadata.source=patch_texture` and content
  "Current write tool: patch_texture. Do not call any retired compatibility
  alias."
- Trace for trajectory `d2a0ccf4-276f-43f2-be6b-f6da43fdaf15` contained
  conductor and Texture agents only, 28 moments, two `patch_texture returned`
  tool-result moments, four non-error `patch_texture` tool events, zero
  `rewrite_texture` hits, zero `edit_texture` hits, and zero `super` hits.
- UI proof found one Texture window, zero legacy `vtext` windows, visible
  `patch_texture`, no visible `edit_texture`, no "Writing first draft"
  placeholder, and no forbidden browser requests to `/internal/*`,
  `/api/agent/*`, `/api/test/*`, `/api/prompts`, or `/api/events`.

Open edge: select the next bounded residue class among storage
schema/workspace/file suffixes, metadata keys, `/pub/vtext/...` route identity,
and protocol v0.

## 2026-06-16 - Public Publication Route Identity Checkpoint

Claim: `/pub/vtext/...` is live public link state, not merely a handler name,
so the next behavior move must distinguish new Texture route minting from
legacy public-link preservation.

Move: read-only inventory of publication route generation, public-reader
frontend entry, and route expectations in current tests; document the next
behavior slice before code changes.

Expected ΔV: 0 global; C18 becomes active and the public route-identity slice
is scoped.

Actual ΔV: 0. Problem Documentation First checkpoint landed in docs only.

Conjecture delta: new publication URLs can teach Texture without breaking
existing `/pub/vtext/...` rows if the system mints `/pub/texture/...` for new
publications and keeps old route rows resolvable/readable.

Protected surfaces for the later behavior slice: platform route generation,
public route lookup/export, frontend direct public reader entry, published
Texture window deduplication, and product publication tests.

Admissible evidence class for the later behavior slice: focused platform,
proxy, and frontend publication tests, CI, staging deploy identity, a deployed
publication proof that a new route is `/pub/texture/...`, and a legacy route
proof or fixture that `/pub/vtext/...` remains accepted.

Rollback path for the later behavior slice: restore `/pub/vtext/...` route
minting and frontend prefix recognition if new `/pub/texture/...` routes fail
public reader or publication export behavior.

Heresy delta: discovered route-identity compatibility risk; no behavior repair
claimed yet.

Receipts:
- `internal/platform/service.go` still defines `publicVTextPrefix =
  "/pub/vtext/"`, uses it to construct new `routePath` values, trims it to
  derive publication slugs, and only applies trailing-slash normalization for
  that prefix.
- `frontend/src/App.svelte` only recognizes direct public reader entry for
  paths that start with `/pub/vtext/`.
- `frontend/src/lib/Desktop.svelte` only normalizes and deduplicates public
  reader paths with the `/pub/vtext/` prefix.
- `frontend/tests/file-browser.spec.js` and
  `frontend/tests/vtext-source-service-publication.spec.js` still expect newly
  published routes to match `^/pub/vtext/`.
- Platform/proxy public publication tests still fixture resolve/export routes
  under `/pub/vtext/...`, confirming legacy route preservation needs explicit
  coverage.

Open edge: implement the behavior slice after this checkpoint: mint
`/pub/texture/...` for new publications, preserve `/pub/vtext/...` reads, and
prove both locally before CI/staging.

## 2026-06-16 - Local Public Publication Route Identity Repair

Claim: new public publication links can mint `/pub/texture/...` while existing
`/pub/vtext/...` link state remains accepted as explicit legacy route state.

Move: change platform publication route generation to the Texture prefix,
preserve legacy route normalization for resolve/export, update proxy and
frontend public-reader expectations, and move browser publication tests to the
current Texture publish control endpoint.

Expected ΔV: support C18 locally; no global V decrease until CI, deploy, and
staging publication proof are recorded.

Actual ΔV: C18 is locally supported pending CI/deploy.

Conjecture delta: public route identity can teach Texture at the point of new
publication without rewriting or redirecting old public route rows.

Protected surfaces: platform route generation, public route lookup/export,
frontend direct public reader entry, published Texture window deduplication,
proxy publication public URL projection, and product publication tests.

Admissible evidence class: focused platform/proxy tests, frontend build,
route-residue search, CI, staging deploy identity, deployed proof that new
publication routes are `/pub/texture/...`, and deployed proof that a legacy
`/pub/vtext/...` route still resolves/exports/opens.

Rollback path: restore `/pub/vtext/...` route minting, remove
`/pub/texture/...` public-reader prefix recognition, and revert route
expectations if staging publication/read/export proof fails.

Heresy delta: repaired locally for new public route minting; legacy
`/pub/vtext/...` public links remain discovered compatibility state pending
deployed proof and any later redirect/migration policy.

Receipts:
- `nix develop -c go test ./internal/platform -run 'TestPublishVTextCreatesImmutablePublicRecords|TestInternalPublishRequiresInternalCallerAndBundleResolve'`
  passed.
- `nix develop -c go test ./internal/proxy -run 'TestPlatformPublicationResolveIsPublicAndInternalOnly|TestPlatformPublicationResolveAndExportPropagateNotFound|TestHandleVTextPublication'`
  passed.
- `nix develop -c go test ./internal/platform ./internal/proxy` passed.
- `npm --prefix frontend run build` passed with pre-existing Universal Wire
  warnings for unused `currentUser` and `.wire-state` selectors.
- Route residue search found only explicit legacy route support, route tests or
  fixtures, and frontend dual-prefix acceptance.
- Local Playwright proof was blocked by pre-existing local platformd Dolt state
  under `/tmp/go-choir-m2/platform-dolt`; the foreground service session was
  stopped and local service health checks returned down.

Open edge: push the repair, monitor CI/deploy, then prove on staging that new
publications mint `/pub/texture/...`, public reader and export APIs work, and
legacy `/pub/vtext/...` public routes remain accepted.

## 2026-06-16 - Deployed Public Publication Route Identity Repair

Claim: deployed Choir can mint new public publication URLs under
`/pub/texture/...` while preserving existing `/pub/vtext/...` public link state
for resolve, export, and direct public reader entry.

Move: push commit `65502a706ef1adba7fc2d1ed5428e3f709f9d2d0`, monitor CI and
deploy, verify staging build identity, and run a deployed Playwright product
proof for new and legacy public routes.

Expected ΔV: support C18 for deployed scope; no coarse V decrease because
storage/file suffixes, metadata keys, actor IDs/app route labels, and protocol
v0 residue remain.

Actual ΔV: C18 is supported for deployed public-route scope. V remains 2.

Conjecture delta: new public-route identity can teach Texture without a
database rewrite or redirect, as long as legacy route rows remain accepted.

Protected surfaces: platform route generation, public route lookup/export,
frontend direct public reader entry, published Texture window deduplication,
proxy publication public URL projection, staging deployment identity, and
browser-public product path.

Admissible evidence class: CI, deploy job, staging health identity, and
deployed browser product proof that creates a public publication, observes a
`/pub/texture/...` route, resolves/exports/opens it, and resolves/exports/opens
an existing `/pub/vtext/...` route.

Rollback path: restore `/pub/vtext/...` route minting, remove
`/pub/texture/...` public-reader prefix recognition, and revert route
expectations if later deployed public reader or export regressions appear.

Heresy delta: repaired for deployed new public route minting. Existing
`/pub/vtext/...` public routes remain deliberate legacy compatibility state,
not a current new-publication minting path.

Receipts:
- CI run `27590698503` passed for commit
  `65502a706ef1adba7fc2d1ed5428e3f709f9d2d0`.
- Deploy job `81570766605` succeeded.
- Docs Truth Check run `27590698536` passed, and FlakeHub publish run
  `27590698504` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `65502a706ef1adba7fc2d1ed5428e3f709f9d2d0`, deployed at
  `2026-06-16T02:50:42Z`.
- Deployed Playwright product proof registered
  `texture-public-route-proof-1781578657650-ce9lel@example.com`, created
  document `79579ae6-f620-4194-9a0a-afabee56a1fd`, created revision
  `e673f6f3-3c80-4577-9699-be146f996283`, and published publication
  `pub-19a8e51e-732d-498e-814c-fe18aa37568a` /
  version `pubver-4f361ae5-30e0-4ed6-b9a8-6dd1edb9c2ef`.
- New route
  `/pub/texture/texture-public-route-proof-1781578657650-pub19a8e51e7`
  resolved with trailing slash normalization, exported Markdown with proof
  content, appeared in retrieval search for `1781578657650`, and opened in one
  published Texture reader window.
- Legacy route
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6` resolved
  with trailing slash normalization, exported Markdown, and opened in one
  published Texture reader window.
- Forbidden browser-public request count was zero for `/internal/*`,
  `/api/agent/*`, `/api/test/*`, `/api/prompts`, and `/api/events`.
- Evidence artifact:
  `/tmp/choir-texture-route-proof-1781578657650.json`; screenshots:
  `/tmp/choir-texture-route-proof/new-texture-route-1781578657650.png` and
  `/tmp/choir-texture-route-proof/legacy-vtext-route-1781578657650.png`.

Open edge: select the next bounded residue class among storage
schema/workspace/file suffixes, metadata keys, actor IDs/app route labels, and
protocol v0.

## 2026-06-16 - Problem Checkpoint: Texture Auth Intent Label Residue

Claim: frontend auth-required intent kinds are a bounded product-facing
old-name residue class, separate from durable runtime actor ids, provenance
metadata, verifier predicates, and storage symbols.

Move: read-only inventory of Texture auth intent dispatch/replay labels,
registry auth requirements, legacy app URL compatibility, and adjacent
metadata/runtime residues; document the next behavior slice before code
changes.

Expected ΔV: 0 global; C19 becomes active and the auth-intent label slice is
scoped.

Actual ΔV: 0. Problem Documentation First checkpoint landed in docs only.

Conjecture delta: new Texture auth intents can teach the promoted ontology at
the product overlay/replay layer without touching durable actor routes such as
`vtext:<doc_id>` or storage tables such as `vtext_documents`.

Protected surfaces for the later behavior slice: Texture app registry auth
requirements, Texture editor auth-required dispatches, auth overlay copy,
post-auth app replay, legacy intent replay, and legacy `?app=vtext&doc=...`
URL compatibility.

Admissible evidence class for the later behavior slice: frontend build,
targeted frontend tests for signed-out auth overlay and legacy app URL
compatibility, CI, staging deploy identity, and a deployed browser proof that a
signed-out Texture action opens an auth overlay with a Texture-named intent
while legacy `app=vtext` still opens Texture.

Rollback path for the later behavior slice: restore old intent strings in the
Texture editor dispatches, registry auth requirements, and App replay logic if
auth overlay replay or legacy app URL compatibility regresses.

Heresy delta: discovered auth-intent label residue; no behavior repair claimed
yet.

Receipts:
- `frontend/src/lib/apps/registry.ts` still declares Texture auth requirements
  as `save_vtext`, `revise_vtext`, and `publish_vtext`.
- `frontend/src/lib/VTextEditor.svelte` still dispatches auth intents
  `save_vtext`, `publish_vtext`, `vtext_diagnosis`,
  `vtext_source_repair`, `vtext_source_artifact`, and
  `published_vtext_edit` while using `appId: 'texture'`.
- `frontend/src/App.svelte` still renders/replays `save_vtext`,
  `publish_vtext`, `published_vtext_edit`, and `private_vtext_document`
  intent kinds.
- `frontend/src/App.svelte` still accepts `?app=vtext&doc=...`; this is
  intentional legacy URL compatibility and not current app identity.
- Adjacent hits such as `created_from: 'vtext_source_artifact_ui'`,
  `source: vtext_source_artifact_attachment`, `publish_vtext_revision`,
  `choir.platform.publish_vtext.v0`, and `vtext:<doc_id>` are metadata,
  provenance, verifier, or runtime actor-route residues that require separate
  migration design.

Open edge: implement the behavior slice after this checkpoint: emit
Texture-named frontend auth intents, preserve legacy replay and legacy app URL
compatibility, then prove locally and on staging.

## 2026-06-16 - Local Repair: Texture Auth Intent Labels

Claim: new frontend Texture actions can emit Texture-named auth intent kinds
while the auth overlay/replay boundary still accepts old in-memory intent names
and legacy `?app=vtext&doc=...` URL compatibility.

Move: rename Texture editor auth-required dispatches and registry mutable-intent
requirements to Texture names, add a legacy intent normalization map in
`App.svelte`, expose the pending auth intent kind as a nonvisual overlay test
attribute, and add a signed-out Texture publish overlay proof.

Expected ΔV: support C19 locally; no coarse V decrease until CI, deploy, and
staging proof are recorded.

Actual ΔV: C19 is supported for local auth-intent scope. V remains 2.

Conjecture delta: frontend auth overlay labels can teach Texture without
touching durable `vtext:<doc_id>` actor ids, storage tables, publication
predicates, or source/provenance metadata.

Protected surfaces: Texture app registry auth requirements, Texture editor
auth-required dispatches, auth overlay copy and replay, legacy intent replay,
legacy app URL compatibility, and signed-out public preview Texture actions.

Admissible evidence class: frontend build, focused signed-out Playwright proof,
producer-residue search, CI, staging deploy identity, and deployed browser
proof for signed-out Texture auth overlay plus legacy app URL compatibility.

Rollback path: restore old intent strings in editor dispatches, registry
requirements, and App replay/message handling if auth replay or legacy app URL
compatibility regresses.

Heresy delta: repaired locally for new frontend auth intent labels; durable
actor ids, storage symbols, and source/provenance metadata remain discovered
residue.

Receipts:
- `npm --prefix frontend run build` passed, with the existing Universal Wire
  warnings for unused `currentUser` and `.wire-state` selectors.
- `npm --prefix frontend run e2e -- --project=chromium
  tests/auth-entry-ui.spec.js --grep "signed-out Texture publish"` passed
  against an explicit Vite preview server.
- The broader `auth-entry-ui.spec.js` run was attempted first and failed before
  app execution because no local server was listening on `localhost:4173`.
- Producer residue search for old auth intent names across `frontend/src` and
  `frontend/tests` now finds only the explicit legacy normalization map in
  `frontend/src/App.svelte` plus the out-of-scope
  `created_from: 'vtext_source_artifact_ui'` provenance marker.

Open edge: push, monitor CI/deploy, verify staging identity, then run deployed
browser proof that a signed-out Texture action exposes a Texture-named auth
intent while legacy `?app=vtext&doc=...` still opens Texture.

## 2026-06-16 - Deployed Repair: Texture Auth Intent Labels

Claim: deployed Choir can expose Texture-named auth intent state for signed-out
Texture actions while preserving deletion-receipted legacy `app=vtext` document
deep-link compatibility.

Move: verify pushed commit `2f13598d37be2807f8cefe9258300a1a798a081c`, monitor
CI/deploy, confirm staging health identity, and run a deployed Playwright
browser proof for signed-out auth overlay and authenticated legacy URL replay.

Expected ΔV: support C19 for deployed scope; no coarse V decrease because
storage/file suffixes, metadata keys, durable actor IDs, and protocol v0 remain.

Actual ΔV: C19 is supported for deployed auth-intent scope. V remains 2.

Conjecture delta: deployed frontend auth intent naming can teach Texture at the
auth overlay boundary without touching durable actor ids, storage tables,
publication predicates, or source/provenance metadata.

Protected surfaces: deployed frontend auth overlay state, Texture editor
publish action, legacy `?app=vtext&doc=...` URL replay, authenticated Texture
document open path, and browser-public route hygiene.

Admissible evidence class: CI, deploy job, staging health identity, and
deployed browser proof.

Rollback path: restore old intent strings in editor dispatches, registry
requirements, and App replay/message handling if later auth replay or legacy
app URL compatibility regresses.

Heresy delta: repaired for deployed frontend auth intent labels; durable actor
ids, storage symbols, and source/provenance metadata remain discovered residue.

Receipts:
- CI run `27591417530` passed for commit
  `2f13598d37be2807f8cefe9258300a1a798a081c`; deploy job `81572916777`
  succeeded.
- Docs Truth Check run `27591417528` passed; FlakeHub publish run
  `27591417545` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `2f13598d37be2807f8cefe9258300a1a798a081c`, deployed at
  `2026-06-16T03:10:59Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-auth-intent-deployed.tmp.spec.js`
  passed both staging proof tests before the temporary spec was deleted.
- Signed-out proof observed `data-auth-intent-kind="publish_texture"`, visible
  "Publish this Texture" auth copy, zero `[data-app-id="vtext"]` windows, and
  zero forbidden browser-public requests to `/internal/*`, `/api/agent/*`,
  `/api/test/*`, `/api/prompts`, or `/api/events`.
- Authenticated legacy URL proof registered a staging user with virtual
  passkey, created a Texture document and revision through `/api/texture`,
  navigated to `?app=vtext&doc=...`, and observed one canonical
  `[data-app-id="texture"]` window, zero `[data-app-id="vtext"]` windows,
  rendered proof content, and a consumed URL with no `app=vtext` query.
- Screenshots:
  `/tmp/choir-texture-auth-intent-1781579569646.png` and
  `/tmp/choir-texture-auth-legacy-url-1781579569646.png`.

Open edge: select the next bounded residue class among storage
schema/workspace/file suffixes, metadata keys, durable actor IDs, remaining
app-route labels, and protocol v0.

## 2026-06-16 - Problem Checkpoint: Source Repair Metadata Label Residue

Claim: source repair and source artifact provenance labels are a bounded
metadata residue class, separate from storage tables, durable actor ids,
platform publication predicates, app-package evidence fields, and transclusion
metadata keys.

Move: read-only inventory of source repair/artifact metadata emitters,
frontend source artifact creation provenance, focused assertions, and adjacent
broader metadata residues; document the behavior slice before code changes.

Expected ΔV: 0 global; C20 becomes active and the source repair metadata label
slice is scoped.

Actual ΔV: 0. Problem Documentation First checkpoint landed in docs only.

Conjecture delta: new source repair and source artifact provenance labels can
teach Texture without touching source entity structs, source routes,
`.vtext` alias/storage fields, durable actor ids, or platform publication
attestations.

Protected surfaces for the later behavior slice: source gap repair revision
metadata, source artifact attachment revision metadata, frontend source content
item creation provenance, source repair tests, source artifact attachment
tests, and markdown-lineage browser tests.

Admissible evidence class for the later behavior slice: focused runtime source
repair tests, focused frontend markdown-lineage/source repair test, frontend
build, residue search, CI, staging deploy identity, and deployed product proof
if the behavior is reachable through staging UI/API.

Rollback path for the later behavior slice: restore the old emitted
`vtext_source_*` metadata values and test expectations if source repair,
source artifact attachment, or downstream metadata readers regress.

Heresy delta: discovered source repair metadata label residue; no behavior
repair claimed yet.

Receipts:
- `internal/runtime/vtext_source_repairs.go` still emits
  `source="vtext_source_gap_repair"` and
  `source="vtext_source_artifact_attachment"`.
- `frontend/src/lib/vtext-source-actions.ts` still emits
  `created_from: 'vtext_source_artifact_ui'`.
- `internal/runtime/vtext_test.go` still asserts the old source repair and
  source artifact attachment metadata values.
- `frontend/tests/vtext-markdown-lineage.spec.js` still asserts
  `repaired.metadata?.source === 'vtext_source_gap_repair'`.
- Adjacent hits including `canonical_vtext_source_path`, `related_vtexts`,
  `story_vtext_doc_id`, `vtext_doc_id`, `vtext_revision_id`,
  `private_vtext_revision`, `publish_vtext_revision`, and
  `choir.platform.publish_vtext.v0` are broader migration surfaces kept out of
  this slice.

Open edge: implement the behavior slice after this checkpoint: emit
Texture-named source repair/artifact provenance values, update focused tests,
and prove locally before CI/staging.

## 2026-06-16 - Local Repair: Source Repair Metadata Labels

Claim: new source repair and source artifact paths can emit Texture-named
provenance values without changing source entity structures, source routes,
storage tables, `.vtext` alias behavior, durable actor ids, or platform
publication attestations.

Move: rename emitted source repair/artifact metadata values to
`texture_source_gap_repair`, `texture_source_artifact_attachment`, and
`texture_source_artifact_ui`; update focused runtime and frontend expectations;
run focused checks and residue search.

Expected ΔV: support C20 locally; no coarse V decrease until CI, deploy, and
staging proof are recorded.

Actual ΔV: C20 is supported for local source-metadata scope. V remains 2.

Conjecture delta: the source repair metadata namespace can teach Texture at the
new-emission boundary while leaving broader metadata/storage/actor/platform
surfaces untouched.

Protected surfaces: source gap repair revision metadata, source artifact
attachment revision metadata, frontend source content item creation provenance,
source repair tests, source artifact attachment tests, and markdown-lineage
browser tests.

Admissible evidence class: focused runtime source repair tests, frontend build,
residue search, CI, staging deploy identity, and deployed source repair product
proof.

Rollback path: restore the old emitted `vtext_source_*` metadata values and
test expectations if source repair, source artifact attachment, or downstream
metadata readers regress.

Heresy delta: repaired locally for new source repair/artifact metadata labels;
broader metadata, storage, actor-id, app-package, and platform publication
provenance residue remains discovered and out of scope.

Receipts:
- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVTextSourceGapRepairCreatesRevision|TestVTextSourceArtifactAttachmentCreatesMetadataOnlyRevision'
  -count=1` passed.
- `npm --prefix frontend run build` passed with the existing Universal Wire
  warnings for unused `currentUser` and `.wire-state` selectors.
- Live residue search for
  `vtext_source_gap_repair|vtext_source_artifact_attachment|vtext_source_artifact_ui`
  across `internal`, `frontend/src`, and `frontend/tests` returned no hits.
- Texture-name search found only intended emitters/assertions for
  `texture_source_gap_repair`, `texture_source_artifact_attachment`, and
  `texture_source_artifact_ui`.
- Local Playwright attempt for
  `tests/vtext-markdown-lineage.spec.js --grep "Migrated source gaps"` failed
  before app execution because no local server was listening on
  `localhost:4173`.

Open edge: push, monitor CI/deploy, verify staging identity, then run deployed
product proof for source gap repair metadata.

## 2026-06-16 - Deployed Repair: Source Repair Metadata Labels

Claim: deployed Choir can emit Texture-named provenance metadata for new source
gap repairs and source artifact paths without changing source entity
structures, source routes, storage tables, `.vtext` alias behavior, durable
actor ids, or platform publication attestations.

Move: push commit `39d0c2ba125c81d59b34002685a9ce19ec98eda0`, monitor CI and
deploy, verify staging build identity, and run a deployed Playwright product
proof that creates a Markdown-lineage Texture document, repairs a citation gap,
opens the Texture document in the desktop UI, and observes the new metadata
label.

Expected ΔV: support C20 for deployed scope; no coarse V decrease because
storage/file suffixes, broader metadata keys, durable actor ids, app-package
evidence fields, and protocol v0 remain.

Actual ΔV: C20 is supported for deployed source-metadata scope. V remains 2.

Conjecture delta: source repair metadata can teach Texture at the
new-emission boundary while preserving the broader stateful migration surfaces
for separately designed slices.

Protected surfaces: deployed source gap repair revision metadata, deployed
Texture document/revision APIs, Texture desktop document opening,
browser-public route hygiene, staging deployment identity, and focused
runtime/frontend tests.

Admissible evidence class: full CI, deploy job, staging health identity,
focused local tests, residue search, and deployed browser/product proof.

Rollback path: restore the old emitted `vtext_source_*` metadata values and
test expectations if later source repair, source artifact attachment, or
downstream metadata readers regress.

Heresy delta: repaired for deployed new source repair/artifact metadata labels.
Adjacent metadata keys, storage symbols, app-package evidence fields, durable
actor ids, and platform publication provenance remain discovered residue.

Receipts:
- CI run `27591835245` passed for commit
  `39d0c2ba125c81d59b34002685a9ce19ec98eda0`; deploy job `81574215697`
  succeeded.
- Docs Truth Check run `27591835237` passed; FlakeHub publish run
  `27591835231` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `39d0c2ba125c81d59b34002685a9ce19ec98eda0`, deployed at
  `2026-06-16T03:22:47Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-source-metadata-deployed.tmp.spec.js`
  passed before the temporary spec was deleted.
- The proof used public product APIs
  `/api/texture/markdown-lineage/import`,
  `/api/texture/documents/{doc}/source-repairs`, and
  `/api/texture/documents/{doc}/revisions`; no browser-public internal or
  test-only routes were used.
- Evidence artifact:
  `/tmp/choir-texture-source-metadata-1781580461671.json`; screenshot:
  `/tmp/choir-texture-source-metadata-1781580461671.png`.
- Product evidence ids: staging user
  `playwright-state-1781580336142-whrv71@example.com`; Texture document
  `8161aac2-4710-46a9-a3a3-2e2f7193b797`; base revision
  `f5ae5dd5-7455-4cfd-8e88-009d923fd4bd`; repaired revision
  `4e0ec188-10a3-4b1a-b4fd-dbcaaf71f0ea`.
- Product observations: repaired revision metadata source was
  `texture_source_gap_repair`, not `vtext_source_gap_repair`; repaired content
  linked the citation to the source entity; the Texture desktop app opened the
  proof document under canonical `texture` app identity; rendered citation
  transclusion showed the source label and excerpt; forbidden browser-public
  request count was zero for `/internal/*`, `/api/agent/*`, `/api/test/*`,
  `/api/prompts`, and `/api/events`.

Open edge: select the next bounded residue class among metadata keys,
storage/file suffixes, durable actor ids, app-package evidence fields,
remaining app-route labels, and protocol v0.

## 2026-06-16 - Problem Checkpoint: App Package And Platform Provenance Label Residue

Claim: AppChangePackage human-proof refs and platform publication provenance
labels are a protected evidence/provenance residue class, separate from
Universal Wire story projection fields, general Texture metadata keys, storage
tables, file suffixes, and durable actor ids.

Move: read-only inventory of package tool schema fields, review-evidence
classification, vsuper prompt defaults, app-promotion tests, platform
publication provenance writes, and public bundle read redaction; document the
behavior slice before code changes.

Expected ΔV: 0 global; C21 becomes active and the AppChangePackage/platform
provenance slice is scoped.

Actual ΔV: 0. Problem Documentation First checkpoint landed in docs only.

Conjecture delta: new app-package and platform provenance evidence can teach
Texture without touching storage schema, durable actor ids, Universal Wire
story projection metadata, or `.vtext` file aliases.

Protected surfaces for the later behavior slice: AppChangePackage tool schema,
package provenance refs, review-evidence human-proof classification, vsuper
prompt defaults, platform publication provenance entities/activities/verifier
predicates, public bundle citation redaction, and focused runtime/platform/
frontend fixtures.

Admissible evidence class for the later behavior slice: focused
app-promotion/shipper tests, platform publication tests, touched frontend
fixture tests, residue search, CI, staging deploy identity, and deployed
product/API proof when reachable without manually seeding success records.

Rollback path for the later behavior slice: restore old emitted package
provenance field names and platform provenance predicates if review evidence,
publication, bundle reads, or downstream adoption proof regresses.

Heresy delta: discovered app-package/platform provenance label residue; no
behavior repair claimed yet.

Receipts:
- `internal/runtime/tools_shipper.go` still exposes
  `vtext_doc_id` and `vtext_revision_id` in `publish_app_change_package`
  args/schema/provenance output and describes the human proof narrative as
  VText.
- `internal/runtime/api_app_promotion.go` still recognizes narrative refs by
  `vtext` keys/values and emits missing-proof copy `narrative VText`.
- `internal/runtime/prompt_defaults/vsuper.md` still asks for a causal VText
  narrative plus `vtext_doc_id` and `vtext_revision_id`.
- `internal/runtime/agent_tools_test.go`,
  `internal/runtime/app_promotion_test.go`, and
  `frontend/tests/web-surface-rationalization.spec.js` still assert or stub
  the old package evidence fields.
- `internal/platform/service.go` still writes
  `private_vtext_revision`, `publish_vtext_revision`,
  `choir-private:vtext/...`, and
  `choir.platform.publish_vtext.v0`.
- `internal/platform/service_publication_read.go` still rewrites legacy
  private revision citations so public bundles do not leak private ids.

Open edge: implement the behavior slice after this checkpoint: emit Texture
package evidence fields and platform provenance labels, preserve only explicit
legacy read compatibility where needed, and prove locally before CI/staging.

## 2026-06-16 - Local Repair: App Package And Platform Provenance Labels

Claim: new AppChangePackage human-proof refs and platform publication
provenance can emit Texture-named evidence labels while preserving explicit
legacy read compatibility for existing package provenance and platform rows.

Move: rename new package proof refs to `texture_doc_id` and
`texture_revision_id`; update vsuper prompt defaults and review-evidence copy
to Texture; emit platform provenance as `private_texture_revision`,
`choir-private:texture/...`, `publish_texture_revision`, and
`choir.platform.publish_texture.v0`; update focused runtime/platform/frontend
fixtures; keep deletion-receipted legacy readers.

Expected ΔV: support C21 locally; no coarse V decrease until CI, deploy, and
staging proof are recorded.

Actual ΔV: C21 is supported for local package/provenance scope. V remains 2.

Conjecture delta: package review evidence and platform publication provenance
can teach Texture at the evidence contract boundary without touching Universal
Wire story projection fields, general Texture metadata keys, durable actor ids,
storage tables, or file suffixes.

Protected surfaces: AppChangePackage tool schema and provenance refs,
review-evidence human-proof classification, vsuper prompt defaults, platform
publication provenance/citation/verifier rows, public bundle citation redaction,
runtime/platform tests, and frontend review-evidence fixtures.

Admissible evidence class: focused runtime/platform tests, frontend build,
doccheck, diff check, residue search, CI, staging deploy identity, and deployed
product/API proof.

Rollback path: restore old package provenance field names and platform
publication provenance predicates if review evidence, publication, bundle reads,
or downstream adoption proof regresses.

Heresy delta: repaired locally for new AppChangePackage and platform
publication provenance labels. Legacy package provenance refs and legacy
platform rows remain deletion-receipted read compatibility; Universal Wire
story projection fields, general Texture metadata keys, durable actor ids,
storage symbols, and file suffixes remain discovered residue.

Receipts:
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestPublishAppChangePackageToolPublishesWithoutGitHubPush|TestAppChangePackageReviewEvidenceRequiresNarrativeAndMediaForHumanReview' -count=1`
  passed.
- `nix develop -c go test ./internal/platform -run 'TestPublishVTextCreatesImmutablePublicRecords|TestInternalPublishRequiresInternalCallerAndBundleResolve' -count=1`
  passed, including direct row assertions for current Texture provenance
  labels and public-bundle no-leak checks.
- `npm --prefix frontend run build` passed with the existing Universal Wire
  warnings for unused `currentUser` and `.wire-state` selectors.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json` completed report-only with 212 docs and 1,129
  warnings.
- `git diff --check` passed.
- Old-label residue search across the touched runtime/platform/frontend-test
  files now finds only explicit legacy compatibility/read assertions:
  `private_vtext_revision` redaction support, a no-leak assertion, and a
  legacy package-provenance fixture.
- Texture-label search finds the new emitted/proven values:
  `texture_doc_id`, `texture_revision_id`, `private_texture_revision`,
  `choir-private:texture/...`, `publish_texture_revision`, and
  `choir.platform.publish_texture.v0`.
- Focused frontend Playwright attempt against staging failed before exercising
  package evidence behavior because the test still opens retired
  `apps-changes` launcher selectors while the current app registry exposes the
  surface as `features`.

Open edge: push, monitor CI/deploy, verify staging identity, and run deployed
product/API proof for AppChangePackage review evidence or platform publication
provenance without manually seeding success records.

## 2026-06-16 - Deployed Repair: App Package And Platform Provenance Labels

Claim: deployed Choir can create AppChangePackages with Texture-named
human-proof refs and return human-reviewable package review evidence without
emitting retired package evidence field names.

Move: push commit `24bff527b56e8f76e1ba3066dd5c71d52543120e`, monitor CI and
deploy, verify staging build identity, and run a deployed Playwright product
proof that creates an AppChangePackage with `texture_doc_id` /
`texture_revision_id`, reads package detail and review evidence, and checks
browser-public route hygiene.

Expected ΔV: support C21 for deployed scope; no coarse V decrease because
Universal Wire story projection fields, general Texture metadata keys, durable
actor ids, storage/file suffixes, stale frontend app-launcher test labels, and
protocol v0 remain.

Actual ΔV: C21 is supported for deployed package/provenance scope. V remains 2.

Conjecture delta: package review evidence and platform publication provenance
can teach Texture at the evidence contract boundary while legacy package
provenance refs and legacy platform rows remain explicit read compatibility.

Protected surfaces: deployed AppChangePackage create/detail/review-evidence
APIs, package provenance refs, review-evidence human-proof classification,
platform publication provenance/citation/verifier rows, public bundle citation
redaction, staging deploy identity, and browser-public route hygiene.

Admissible evidence class: CI, deploy job, staging health identity, deployed
browser/product proof, local focused tests, residue searches, and doccheck.

Rollback path: restore old package provenance field names and platform
publication provenance predicates if review evidence, publication, bundle reads,
or downstream adoption proof regresses.

Heresy delta: repaired for deployed new AppChangePackage and platform
publication provenance labels. Legacy package provenance refs and legacy
platform rows remain deletion-receipted read compatibility; Universal Wire
story projection fields, general Texture metadata keys, durable actor ids,
storage symbols, file suffixes, and stale app-launcher test labels remain
discovered residue.

Receipts:
- CI run `27592592351` passed for commit
  `24bff527b56e8f76e1ba3066dd5c71d52543120e`; deploy job `81576474144`
  succeeded.
- Docs Truth Check run `27592592337` passed; FlakeHub publish run
  `27592592343` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `24bff527b56e8f76e1ba3066dd5c71d52543120e`, deployed at
  `2026-06-16T03:44:38Z`.
- `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-package-provenance-deployed.tmp.spec.js`
  passed before the temporary spec was deleted.
- The proof used public authenticated product APIs
  `POST /api/app-change-packages`,
  `GET /api/app-change-packages/{id}`, and
  `GET /api/app-change-packages/{id}/review-evidence`; no browser-public
  internal or test-only routes were used.
- Evidence artifact:
  `/tmp/choir-texture-package-provenance-1781581617265.json`; screenshot:
  `/tmp/choir-texture-package-provenance-1781581617265.png`.
- Product evidence ids: staging user
  `playwright-state-1781581607161-v10dlq@example.com`; package
  `pkg-texture-provenance-1781581617265`; Texture proof document ref
  `doc-texture-package-proof-1781581617265`; Texture proof revision ref
  `rev-texture-package-proof-1781581617265`.
- Product observations: created package and package detail emitted
  `texture_doc_id` and `texture_revision_id`, not `vtext_doc_id` or
  `vtext_revision_id`; review evidence returned
  `human_proof.state="human_reviewable"` with Texture doc/revision narrative
  refs; review evidence contained no `VText` copy; forbidden browser-public
  request count was zero for `/internal/*`, `/api/agent/*`, `/api/test/*`,
  `/api/prompts`, and `/api/events`.

Open edge: select the next bounded residue class among Universal Wire story
projection metadata, general Texture metadata keys, durable actor ids,
storage/file suffixes, stale frontend app-launcher test labels, and protocol v0.
