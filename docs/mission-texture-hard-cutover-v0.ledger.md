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

## 2026-06-16 - Problem Checkpoint: Universal Wire Story Projection Labels

Claim: Universal Wire story projection labels are a bounded live product API and
frontend-launch residue class that can be repaired without touching broader
storage, file suffix, actor-id, Style.vtext, or general metadata migrations.

Move: document the read-only residue inventory, conjecture delta, protected
surfaces, admissible evidence class, rollback path, heresy delta, adjacent
non-goals, and next behavior-slice design before changing runtime/frontend
code.

Expected ΔV: no coarse V decrease; C22 becomes active with the required Problem
Documentation First checkpoint in place.

Actual ΔV: no coarse V decrease. C22 is active and ready for implementation.

Receipts:
- `internal/types/wire.go` still defines JSON fields
  `projection_vtext_docs`, `story_vtext_doc_id`, and `vtext_content`.
- `internal/runtime/universal_wire.go` still emits
  `source-network-vtext-*`, `source-network-vtext-index`, and
  `universal-wire-*-vtext` story/source labels.
- `frontend/src/lib/UniversalWireApp.svelte` still reads
  `story_vtext_doc_id`, creates `gw-vtext-*` related entities, uses
  `target_kind: 'vtext_document'`, and opens `.story.vtext` source paths.
- `frontend/tests/universal-wire-app.spec.js` still stubs current Universal
  Wire payloads with old story ids/source labels/copy.

Open edge: implement the C22 behavior slice, prove focused runtime/frontend
coverage and residue searches locally, then push, monitor CI/deploy, and run a
deployed Universal Wire product proof if reachable without manually seeding
success records.

## 2026-06-16 - Local Repair: Universal Wire Story Projection Labels

Claim: Universal Wire story projections can emit Texture-named fields and
source labels while preserving only deletion-receipted frontend fallback for
old staged payloads.

Move: rename `types.WireStory` projection JSON fields to
`projection_texture_docs`, `story_texture_doc_id`, and `texture_content`;
rename Universal Wire story/source labels to Texture; update platform story
verification and Texture read-owner helper naming; update Universal Wire
frontend launch context to prefer `story_texture_doc_id`, emit
`texture_document` targets and `gw-texture-*` related entities; update focused
runtime/frontend/staging acceptance tests.

Expected ΔV: support C22 locally; no coarse V decrease until CI, deploy, and
staging product proof are recorded.

Actual ΔV: C22 is supported for local Universal Wire projection scope. V
remains 2.

Protected surfaces: browser-public Universal Wire stories API, story projection
JSON contract, platform story verification, Universal Wire frontend story open
and related Texture launch context, focused tests, and staging acceptance spec.

Admissible evidence class: focused runtime tests, runtime shards, frontend
build, residue searches, CI, staging deploy identity, and deployed Universal
Wire product proof.

Rollback path: restore old Universal Wire story JSON fields/source labels and
frontend consumers if staging story indexing, publication verification, app
rendering, or story launch regresses.

Heresy delta: repaired locally for current Universal Wire story projection
emitters and frontend launch context. Remaining `.vtext`, `vtext:` actor/ref,
Style.vtext, `related_vtexts`, storage, and platform table names remain
discovered residue outside this slice.

Receipts:
- Focused runtime test passed:
  `nix develop -c go test ./internal/runtime -run 'TestHandleUniversalWireStories|TestResolveUniversalWireTextureReadOwner|TestNormalizeWireArticleSourceServiceProse|TestWireAutonomousPublishTranscludesEditionAndDebounces|TestWirePlatformPublishFailsClosedWithoutEditionWhenPlatformdFails' -count=1`.
- Runtime shard coverage passed:
  `nix develop -c scripts/go-test-runtime-shards`.
- Frontend build passed:
  `npm --prefix frontend run build`, with existing Universal Wire warnings for
  unused `currentUser` and `.wire-state` selectors.
- Current-code residue search for old story projection emitters now finds only
  explicit fallback/negative assertions in `UniversalWireApp.svelte`,
  `universal-wire-staging-acceptance.spec.js`, and
  `universal_wire_test.go`.

Open edge: push the behavior commit, monitor CI/deploy, verify staging commit
identity, and run deployed Universal Wire acceptance proof with available auth
state.

## 2026-06-16 - Staging Evidence Checkpoint: Empty Universal Wire Edition

Claim: the first deployed Universal Wire proof attempt exposed an acceptance
oracle gap rather than enough evidence to reject the C22 runtime/frontend
change.

Move: record CI/deploy identity and the failed deployed proof before changing
the staging acceptance spec.

Expected ΔV: no coarse V decrease; C22 moves from local-only support to
deployed source-label/app-surface evidence with deployed story-field proof
still open.

Actual ΔV: no coarse V decrease. The acceptance oracle gap is documented.

Receipts:
- CI run `27593330137` passed for commit
  `9f332529d209e82df86056176ffac2d31d2c5df1`.
- Deploy job `81578635355` succeeded.
- Docs Truth Check run `27593330130` passed; FlakeHub publish run
  `27593330160` passed.
- `https://choir.news/health` reported proxy and sandbox commit
  `9f332529d209e82df86056176ffac2d31d2c5df1`, deployed at
  `2026-06-16T04:05:57Z`.
- Deployed Playwright proof attempt
  `GO_CHOIR_RUN_UNIVERSAL_WIRE_STAGING=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/universal-wire-staging-acceptance.spec.js`
  failed because the API returned `source="universal-wire-edition-texture"`
  with `stories.length === 0`, while the spec assumed an edition source always
  contains at least one story.

Open edge: repair the staging acceptance spec to prove Texture source labels
and empty-state app behavior without claiming story payload fields when staging
has no Universal Wire edition stories; rerun deployed proof.

## 2026-06-16 - Deployed Evidence: Universal Wire Texture Source Labels

Claim: deployed Choir proves Universal Wire's current source-label and
app-empty-state surface under Texture naming, while deployed story-field proof
is unavailable because staging has an empty Universal Wire edition.

Move: update the staging acceptance spec so empty editions prove source labels
and app empty state without claiming story payload fields; refresh staging auth
state; rerun deployed Playwright proof against commit
`9f332529d209e82df86056176ffac2d31d2c5df1`.

Expected ΔV: support C22 for deployed source-label/app-empty-state scope; leave
deployed story-field proof as a named open edge.

Actual ΔV: C22 is supported for deployed source-label/app-empty-state scope.
V remains 2 because deployed story payload fields, broader metadata/storage
residue, and protocol v0 remain open.

Receipts:
- Refreshed auth state from `frontend/` with
  `node scripts/setup-auth-state.mjs --baseUrl https://choir.news`, yielding
  user `qa-1781583037734-7tuzeq@example.com`.
- Deployed proof passed:
  `GO_CHOIR_RUN_UNIVERSAL_WIRE_STAGING=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/universal-wire-staging-acceptance.spec.js`.
- Product observations: stories API source label was not
  `universal-wire-vtext-index` or `universal-wire-edition-vtext`; edition
  metadata existed at `universal-wire/Wire.vtext`; signed-in Universal Wire app
  rendered without SourceMaxx or Global Wire preview copy; empty edition state
  rendered no story cards and showed the empty state.

Open edge: prove deployed `story_texture_doc_id`,
`projection_texture_docs`, and `texture_content` when staging has a Universal
Wire story payload reachable through product paths.

## 2026-06-16 - Problem Checkpoint: Related Texture Metadata Keys

Claim: related-transclusion metadata and frontend app context names are a
bounded live Texture surface that can be repaired separately from storage table
names, `.vtext` file suffixes/source paths, durable `vtext:` actor ids,
`canonical_vtext_source_path`, and protocol v0.

Move: document the read-only residue inventory, conjecture delta, protected
surfaces, admissible evidence class, rollback path, heresy delta, adjacent
non-goals, and next behavior-slice design before changing frontend metadata
serialization or render helpers.

Expected ΔV: no coarse V decrease; C23 becomes active with the required Problem
Documentation First checkpoint in place.

Actual ΔV: no coarse V decrease. C23 is active and ready for implementation.

Protected surfaces: Texture editor revision metadata serialization,
Texture document rendering of related transclusion refs, related Texture open
dispatch, Universal Wire story-to-Texture app context, focused frontend tests,
and markdown serializer/renderer helper exports.

Admissible evidence class: focused frontend tests for source entities,
markdown lineage/editor behavior, Universal Wire related launch context,
frontend build, current-code retired-name residue searches, CI, and staging
deploy/browser proof if the slice changes deployed product behavior enough to
claim staging support.

Rollback path: restore `related_vtexts`, `relatedVTexts`, `vtext_document`, and
old helper exports as the primary write path if editor metadata persistence,
inline related transclusion rendering, pinned revision open behavior, or
Universal Wire story launch regresses. Keep legacy read fallback during the
repair so persisted revisions remain readable.

Heresy delta: discovered in read-only inventory, not yet repaired. The slice
will repair current writer/context/helper names while preserving explicitly
deletion-receipted legacy metadata reads.

Receipts:
- `frontend/src/lib/VTextEditor.svelte` reads `metadata.related_vtexts`,
  writes `metadata.related_vtexts`, consumes `appContext.relatedVTexts`, and
  calls `parseVTextRelatedRef` / `vtextEntityPinnedRevisionID`.
- `frontend/src/lib/UniversalWireApp.svelte` emits `relatedVTexts` in Texture
  launch context while already using `target_kind: 'texture_document'`.
- `frontend/src/lib/vtext-source-renderer.ts` exports
  `parseVTextRelatedRef`, `vtextRelatedMarkdownTarget`,
  `vtextEntityPinnedRevisionID`, `findVTextEntity`, and
  `renderInlineVTextRef`; it still parses markdown `vtext:` refs as the
  current inline compatibility syntax.
- `frontend/src/lib/vtext-markdown-renderer.ts` names the renderer option
  `relatedVTexts`.
- Focused search:
  `rg -n "related_vtexts|relatedVTexts|vtext_document|canonical_vtext_source_path|related VText|VText entity|findVTextEntity|renderInlineVTextRef|parseVTextRelatedRef|vtextEntity|data-texture-related|target_kind.*vtext" internal frontend/src frontend/tests -g '!frontend/dist/**'`.

Open edge: implement the C23 behavior slice with Texture-named writer/context
paths and helper exports; retain explicit legacy read fallback; run focused
frontend coverage, build, and residue searches.

## 2026-06-16 - Local Repair: Related Texture Metadata Keys

Claim: current related-transclusion writers and helper APIs can move to Texture
names without breaking already-authored legacy related metadata or markdown
refs.

Move: rename frontend related-transclusion helper exports to
`parseTextureRelatedRef`, `textureRelatedMarkdownTarget`,
`textureEntityPinnedRevisionID`, `findTextureEntity`, and
`renderInlineTextureRef`; make the editor prefer `metadata.related_textures`
and app context `relatedTextures`; make Universal Wire launch Texture with
`relatedTextures`; serialize related refs as `texture:`; keep explicit legacy
read/parser fallback for `related_vtexts`, `relatedVTexts`, and `vtext:`.

Expected ΔV: support C23 locally; no coarse V decrease until CI, deploy, and
deployed product evidence or a precise staging blocker are recorded.

Actual ΔV: C23 is supported for local frontend scope. V remains 2.

Protected surfaces: Texture editor revision metadata serialization, inline
related Texture rendering, related Texture open dispatch, Universal Wire story
launch context, markdown serializer/parser behavior, and focused frontend tests.

Admissible evidence class: focused related-transclusion Playwright tests,
frontend build, residue searches, CI, staging deploy identity, and deployed
Texture related-transclusion proof or a recorded staging-data blocker.

Rollback path: revert the C23 frontend changes to restore old primary
`related_vtexts` / `relatedVTexts` / `vtext:` write paths if revision
serialization, inline rendering, or related Texture open behavior regresses.
Persisted revisions stay readable because the repair keeps legacy fallbacks.

Heresy delta: repaired locally for current frontend writer/context/helper
names; legacy related metadata and markdown syntax remain deletion-receipted
read compatibility, not current write targets.

Receipts:
- Focused related-transclusion tests passed:
  `npm --prefix frontend run e2e -- --project=chromium tests/vtext-source-entities.spec.js -g "related Texture|legacy vtext"`.
- Frontend build passed:
  `npm --prefix frontend run build`, with existing Universal Wire warnings for
  unused `currentUser` and `.wire-state` selectors.
- Helper-name residue search found no current-code hits:
  `rg -n "parseVTextRelatedRef|vtextRelatedMarkdownTarget|vtextEntityPinnedRevisionID|findVTextEntity|renderInlineVTextRef" frontend/src frontend/tests -g '!frontend/dist/**'`.
- Legacy related-name search now finds only explicit read/parser fallbacks,
  legacy-compat tests, and unrelated auth-intent compatibility:
  `rg -n "related_vtexts|relatedVTexts|vtext_document|\\(vtext:" frontend/src frontend/tests -g '!frontend/dist/**'`.
- Current Texture-name search shows `related_textures`, `relatedTextures`,
  `texture_document`, and `texture:` on the current writer/context path:
  `rg -n "related_textures|relatedTextures|texture_document|\\(texture:" frontend/src frontend/tests -g '!frontend/dist/**'`.

Observer note: the broad `vtext-source-entities` file run without a preview
server failed unrelated browser-backed tests with `localhost:4173` connection
refusals and exposed existing stale source-contract expectations for
`appId: vtext` where current code returns `texture`. Those are adjacent source
contract/app-launcher residue, not evidence against C23.

Open edge: push the behavior commit, monitor CI/deploy, verify staging identity,
and attempt deployed related-transclusion proof or record the smallest blocker.

## 2026-06-16 - Deployed Evidence: Related Texture Metadata Keys

Claim: deployed Choir proves the C23 current related-Texture metadata path:
`related_textures` metadata, `texture_document` target kind, `texture:` inline
refs, pinned related revision rendering, and newer-version indication.

Move: push the C23 checkpoint and behavior commits, monitor CI/deploy, verify
staging build identity, then run a temporary deployed Playwright proof that
creates child and parent Texture documents through public authenticated
`/api/texture` APIs, opens the parent through the deployed Texture UI, and
asserts the related transclusion DOM.

Expected ΔV: support C23 for deployed product scope; no coarse V decrease
because broader storage/file/actor/source-contract residue and protocol v0
remain open.

Actual ΔV: C23 is supported for deployed related-transclusion scope. V remains
2.

Receipts:
- Docs checkpoint commit:
  `2bdeed0c docs: checkpoint related texture metadata keys`.
- Behavior commit:
  `201d2e5d74c68476c9a930fb32165abc6d1c7175 frontend: rename related texture metadata keys`.
- CI run `27593972972` passed, including frontend build, Go vet/build,
  non-runtime tests, integration smoke, runtime shards 0-3, TLA, Docs Truth
  Check, and deploy job `81580494742`.
- Separate Docs Truth Check run `27593973013` passed; FlakeHub publish run
  `27593972981` passed.
- `https://choir.news/health` reported proxy and sandbox commit
  `201d2e5d74c68476c9a930fb32165abc6d1c7175`, deployed at
  `2026-06-16T04:24:18Z`.
- Temporary deployed proof passed:
  `GO_CHOIR_RUN_TEXTURE_RELATED_STAGING=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-related-staging.tmp.spec.js`.
  The proof created a child Texture with two revisions, created a parent
  Texture revision whose metadata used `related_textures` and
  `target_kind: "texture_document"`, opened the parent via `?app=texture`, and
  observed `data-texture-related-ref`, pinned revision id, current revision id,
  `data-texture-related-has-newer-version="true"`, `Version pin`, `Newer
  version available`, and the pinned snapshot text.

Open edge: choose the next residue class. The strongest current candidates are
stale source-contract/app-launcher expectations exposed by the broad
`vtext-source-entities` run, durable metadata keys such as
`canonical_vtext_source_path`, storage/file suffixes, durable actor ids, and
the deployed Universal Wire story-field proof when staging has a story payload.

## 2026-06-16 - Problem Checkpoint: Source Contract Texture Open Surface

Claim: publication source-open contract naming is a bounded current product
surface that can be repaired separately from storage tables, public route
compatibility, durable actor ids, generic platform publication-version identity,
and broader prompt-bar route proofs.

Move: document the read-only residue inventory, conjecture delta, protected
surfaces, admissible evidence class, rollback path, heresy delta, adjacent
non-goals, and next behavior-slice design before changing the shared
source-contract schema or generated frontend contract.

Expected ΔV: no coarse V decrease; C24 becomes active with the required Problem
Documentation First checkpoint in place.

Actual ΔV: no coarse V decrease. C24 is active and ready for implementation.

Protected surfaces: shared `internal/sourcecontract` open-surface schema and
normalization, generated frontend source contract, frontend source-open plans
for published Texture sources, platform publication source entity open-surface
defaults, focused Go/frontend tests, and source-contract matrix expectations.

Admissible evidence class: source-contract Go tests, focused platform
publication source tests, generated frontend contract check, focused frontend
source-open-plan tests, frontend build, residue searches, CI, and staging
deploy identity if runtime behavior changes reach deployed product paths.

Rollback path: restore canonical `vtext` open-surface constants/schema and
frontend expectations if published-publication source opening, source contract
generation, or platform publication metadata regresses. Retain `vtext` /
`published_vtext` as aliases during repair so existing source metadata remains
readable.

Heresy delta: discovered in read-only inventory, not yet repaired. The slice
will repair current contract writer/planner names while preserving explicit
legacy aliases for old source metadata.

Receipts:
- `frontend/src/lib/source-contract.ts` currently dispatches
  `publication_version` and `published_vtext_span` to `appId: "texture"` but
  `openSurface: SOURCE_OPEN_SURFACES.vtext` and `mode: "published_vtext"`.
- `internal/sourcecontract/source_contract_schema.json` canonizes the
  publication open surface as `vtext` with aliases `published_vtext`,
  `publication_version`, and `published_vtext_span`.
- `internal/sourcecontract/open_surface.go` exposes `OpenSurfaceVText =
  "vtext"`.
- `internal/sourcecontract/testdata/source_contract_matrix.json` still expects
  `appId: "vtext"` for the frontend publication-version open plan, contradicting
  current code's `appId: "texture"`.
- `frontend/tests/vtext-source-entities.spec.js` still expects
  `sourceOpenPlan({ targetKind: "publication_version" })` to use
  `appId: "vtext"`.
- `frontend/src/lib/vtext-publication-context.ts` and
  `frontend/src/lib/vtext-source-renderer.ts` still write/read
  `published_vtext_span` for published Texture source entities.

Open edge: implement C24 by making current source-contract publication open
surface names Texture-first, keep legacy aliases/read compatibility, regenerate
frontend contract output, and run focused Go/frontend coverage plus residue
searches.

## 2026-06-16 - Local Repair: Source Contract Texture Open Surface

Claim: current source-contract publication source-open naming can move to
Texture while retaining legacy `vtext` / `published_vtext*` input
compatibility.

Move: change the shared source-contract schema canonical publication open
surface from `vtext` to `texture`; replace Go `OpenSurfaceVText` with
`OpenSurfaceTexture`; regenerate the frontend contract artifact; update
frontend source-open plans to return `openSurface: "texture"` and
`mode: "published_texture"`; update current publication source entity writers
and platform proposal citation edges to emit `published_texture_span`; keep
legacy `vtext` and `published_vtext*` forms as aliases/read compatibility.

Expected ΔV: support C24 locally; no coarse V decrease until CI/deploy and any
needed deployed product evidence are recorded.

Actual ΔV: C24 is supported for local shared-contract/frontend/platform scope.
V remains 2.

Protected surfaces: shared source-contract normalization, generated frontend
source contract, frontend source-open planning, platform publication source
entity open-surface defaults, publication proposal source-kind writes, and
focused tests.

Admissible evidence class: source-contract Go tests, focused platform/proxy Go
tests, focused frontend source-contract tests, frontend build/generated
contract check, residue searches, CI, and staging identity or deployed
publication source-open proof if required.

Rollback path: restore canonical `vtext` open-surface names and
`published_vtext_span` writer defaults if publication source opening, source
contract generation, or platform source metadata regresses. Legacy inputs
remain accepted during the repair.

Heresy delta: repaired locally for current source-contract writer/planner names;
legacy open-surface tokens remain deletion-receipted aliases.

Receipts:
- Source-contract Go tests passed:
  `nix develop -c go test ./internal/sourcecontract`.
- Focused platform publication/source tests passed:
  `nix develop -c go test ./internal/platform -run 'Test.*Source|Test.*Publication|Test.*Publish|Test.*Proposal' -count=1`.
- Focused proxy platform/public tests passed:
  `nix develop -c go test ./internal/proxy -run 'Test.*Platform|Test.*Publication|Test.*Public' -count=1`.
- Focused frontend source-contract tests passed:
  `npm --prefix frontend run e2e -- --project=chromium tests/vtext-source-entities.spec.js -g "frontend source contract|source open plans"`.
- Frontend build and generated-contract check passed:
  `npm --prefix frontend run build`.
- Current writer/planner residue search found no accidental old current
  contract names:
  `rg -n "OpenSurfaceVText|SOURCE_OPEN_SURFACES\\.vtext|published_vtext_derivative|mode: 'published_vtext'|\\\"mode\\\": \\\"published_vtext\\\"|appId: 'vtext'|\\\"appId\\\": \\\"vtext\\\"|openSurface: 'vtext'|\\\"openSurface\\\": \\\"vtext\\\"" internal frontend/src frontend/tests -g '!frontend/dist/**'`.
- Legacy-token search now finds only explicit aliases/read compatibility and
  the separate auth-intent compatibility bridge:
  `rg -n "published_vtext|published_vtext_span" internal frontend/src frontend/tests -g '!frontend/dist/**'`.

Open edge: push C24, monitor CI/deploy, verify staging identity, and determine
whether a deployed publication source-open browser proof is needed for this
contract slice.

## 2026-06-16 - Deployed Evidence: Source Contract Texture Open Surface

Claim: C24 is supported for deployed product API scope, not only local
contract/test scope.

Move: push the C24 source-contract behavior commit, monitor CI/deploy, verify
staging identity, then run a temporary deployed Playwright proof that creates a
child Texture publication, creates a parent Texture publication whose source
entity targets the child publication version with raw
`open_surface: "publication-version"`, and asserts the deployed resolver/export
metadata normalize that source entity to `open_surface: "texture"`.

Expected ΔV: support C24 for deployed product scope; no coarse V decrease
because storage/file suffixes, durable actor ids, `/pub/vtext` compatibility,
generic `publication_version` platform identity, and protocol v0 remain open.

Actual ΔV: C24 is supported for deployed source-contract publication
open-surface scope. V remains 2.

Receipts:
- Docs checkpoint commit:
  `f635add1 docs: checkpoint source contract texture open surface`.
- Behavior commit:
  `e15a1f5f2c9a7b60689f61bb2349f1139045d724 runtime: rename source contract texture open surface`.
- CI run `27594327821` passed.
- Deploy job `81581543204` passed.
- Docs Truth Check run `27594327818` passed.
- FlakeHub publish run `27594327814` passed.
- `https://choir.news/health` reported proxy and sandbox commit
  `e15a1f5f2c9a7b60689f61bb2349f1139045d724`, deployed at
  `2026-06-16T04:34:34Z`.
- Temporary deployed proof passed:
  `GO_CHOIR_RUN_TEXTURE_SOURCE_CONTRACT_STAGING=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-source-contract-staging.tmp.spec.js`.
  The proof published a child Texture under `/pub/texture/...`, published a
  parent Texture with a `published_texture` source entity targeting the child
  `publication_version_id` and raw `open_surface: "publication-version"`, then
  verified `/api/platform/publications/resolve` and
  `/api/platform/publications/export?format=md` returned
  `open_surface: "texture"` with the expected transclusion snapshot.

Open edge: choose the next residue class. The strongest candidates are durable
metadata keys such as `canonical_vtext_source_path`, storage/file suffixes,
durable `vtext:` actor ids, `/pub/vtext` public-route compatibility, and the
deployed Universal Wire story-field proof once staging can provide an edition
story payload through product paths.

## 2026-06-16 - Problem Checkpoint: Canonical Texture Source Path Metadata

Claim: `canonical_vtext_source_path` is a bounded current-writer metadata
residue that can be repaired separately from `.vtext` shortcut files, storage
tables, durable `vtext:` actor ids, Style.vtext language, and `/pub/vtext`
public route compatibility.

Move: run a read-only inventory for next residue candidates, compare local and
subagent classifications, choose the canonical source-path metadata writer as
the next bounded C25 slice, and document the conjecture delta, protected
surfaces, evidence class, rollback path, heresy delta, receipts, and non-goals
before changing runtime writers.

Expected ΔV: no coarse V decrease; C25 becomes active with the required Problem
Documentation First checkpoint in place.

Actual ΔV: no coarse V decrease. C25 is active and ready for implementation.

Protected surfaces: user revision creation, appagent `patch_texture` revision
creation, durable metadata carry-forward, file-open projection alias creation,
Markdown/source-lineage import metadata, focused runtime tests, and frontend
markdown-lineage tests.

Admissible evidence class: focused runtime tests covering file-open user
revision carry-forward, structure stabilization, and appagent patch revisions;
focused frontend markdown-lineage tests; current-writer residue search; CI and
staging identity if runtime behavior changes land.

Rollback path: restore `canonical_vtext_source_path` as the emitted durable
metadata key and remove Texture-name promotion if file-open, revision history,
import lineage, or appagent patch revisions lose source-path lineage.

Heresy delta: discovered in read-only inventory, not yet repaired. The slice
will repair current revision writer/carry-forward names while preserving
explicit legacy read/carry-forward compatibility.

Receipts:
- `internal/runtime/vtext.go` writes `canonical_vtext_source_path` on user
  revision creation.
- `internal/runtime/tools_vtext.go` writes `canonical_vtext_source_path` on
  appagent-authored `patch_texture` revisions.
- `internal/runtime/runtime.go` includes `canonical_vtext_source_path` in
  `durableMetadataKeys`.
- `internal/runtime/vtext_structure.go` carries durable keys forward without
  alias promotion.
- `internal/runtime/vtext_test.go` and
  `frontend/tests/vtext-markdown-lineage.spec.js` assert the retired key as a
  current expectation.
- Independent read-only inventories classified `/pub/vtext` as deliberate live
  public route compatibility and actor/storage/file suffixes as broader red
  surfaces, leaving this metadata key as the cleanest bounded current-writer
  repair.

Open edge: implement C25 with `canonical_texture_source_path` writes and legacy
`canonical_vtext_source_path` promotion into the Texture-named key, then run
focused runtime/frontend verification and residue searches.

## 2026-06-16 - Local Repair: Canonical Texture Source Path Metadata

Claim: current Texture revision writers can emit
`canonical_texture_source_path` while preserving legacy
`canonical_vtext_source_path` read/carry-forward compatibility.

Move: add Texture and legacy metadata-key constants; replace current user and
appagent revision writer keys with `canonical_texture_source_path`; update
durable metadata carry-forward to promote legacy parent/run
`canonical_vtext_source_path` into the Texture-named key without carrying the
legacy key forward; update focused runtime and frontend metadata assertions.

Expected ΔV: support C25 locally; no coarse V decrease until CI/deploy and
deployed metadata proof are recorded.

Actual ΔV: C25 is supported for local runtime/frontend test scope. V remains 2.

Protected surfaces: user revision creation, appagent `patch_texture` revision
creation, appagent run metadata seeding, durable metadata carry-forward,
file-open projection lineage, Markdown/source-lineage import metadata, and
focused runtime/frontend tests.

Admissible evidence class: focused comprehensive runtime tests, frontend build,
current-writer residue search, CI/deploy identity, and deployed product-path
metadata proof after push.

Rollback path: restore `canonical_vtext_source_path` as the emitted durable
metadata key and remove Texture-name promotion if source-path lineage is lost
or if legacy revision compatibility regresses.

Heresy delta: repaired locally for current source-path metadata writers;
legacy `canonical_vtext_source_path` remains explicit read/carry-forward
compatibility.

Receipts:
- Focused comprehensive runtime tests passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestBuildAppagentRevisionMetadataPreservesDurableKeys|TestVTextPlainTextImportCarriesMigrationMetadataToFirstDurableRevision|TestVTextImportedMarkdownRevisionUsesVTextProjectionAndPreservesCollapsedTable|TestVTextAppagentEditCanonicalizesAliasedMarkdownTitle' -count=1`.
- Frontend build and generated-contract check passed:
  `npm --prefix frontend run build`.
- Current-code legacy key search found only the legacy compatibility constant
  and negative frontend assertions:
  `rg -n "canonical_vtext_source_path" internal/runtime frontend/src frontend/tests -g '!frontend/dist/**'`.
- Current-code Texture key search found the new runtime constant and frontend
  expectations:
  `rg -n "canonical_texture_source_path" internal/runtime frontend/src frontend/tests -g '!frontend/dist/**'`.

Open edge: push C25, monitor CI/deploy, verify staging identity, and run a
deployed product-path metadata proof against staging.

## 2026-06-16 - Deployed Evidence: Canonical Texture Source Path Metadata

Claim: C25 is supported for deployed product-path metadata scope, not only
local runtime/frontend test scope.

Move: monitor the C25 behavior commit through CI and staging deploy, verify
staging health reports the pushed SHA, then run a temporary deployed Playwright
proof that opens/imports a text file through `/api/texture/files/open`, creates
a first durable Texture revision, and asserts the revision metadata contains
`canonical_texture_source_path` and not `canonical_vtext_source_path`.

Expected ΔV: support C25 for deployed product scope; no coarse V decrease
because `.vtext` file suffixes, storage names, durable `vtext:` actor ids,
`/pub/vtext` route compatibility, Universal Wire deployed story-field proof,
and protocol v0 remain open.

Actual ΔV: C25 is supported for deployed canonical source-path metadata scope.
V remains 2.

Receipts:
- Problem checkpoint commit:
  `f06e1d686f47c3838796aa171e3bd7f335a1dd33 docs: checkpoint canonical texture source path metadata`.
- Behavior commit:
  `b5cbadcd90d0f21a51ecb016229e119c697a21dd runtime: rename canonical texture source path metadata`.
- CI run `27595056664` passed.
- Deploy job `81583736049` passed.
- Docs Truth Check run `27595056666` passed.
- FlakeHub publish run `27595056663` passed.
- `https://choir.news/health` reported proxy and sandbox commit
  `b5cbadcd90d0f21a51ecb016229e119c697a21dd`, deployed at
  `2026-06-16T04:55:40Z`.
- Temporary deployed proof passed:
  `GO_CHOIR_RUN_TEXTURE_SOURCE_PATH_METADATA_STAGING=1 BASE_URL=https://choir.news CHOIR_DEPLOYED_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-source-path-metadata-staging.tmp.spec.js`.
  The proof opened/imported a text file through `/api/texture/files/open`,
  created a first durable Texture revision through
  `/api/texture/documents/{doc_id}/revisions`, and observed
  `metadata.canonical_texture_source_path` ending in `.vtext`, no
  `metadata.canonical_vtext_source_path`, plus the expected text/plain
  import/migration manifests.

Open edge: choose the next residue class. The strongest candidates are broader
`.vtext` file/alias suffix design, durable `vtext:` actor ids, storage table
names, `/pub/vtext` public route compatibility policy, and the deployed
Universal Wire story-field proof once staging can provide an edition story
payload through product paths.

## 2026-06-16 - Problem Checkpoint: Publication Fallback Texture Labels

Claim: C26 is an admissible bounded residue class. Current publication
fallback/default writers still mint the retired ontology in owner-visible and
exported surfaces, but the repair can avoid broad route identity, storage,
`PublishVText` API symbol, and exported CSS-class migration.

Move: read-only inventory over platform publication writers/readers and
frontend publication acceptance expectations, then document the problem before
behavior changes.

Expected ΔV: no coarse V decrease. This checkpoint buys the admissible first
commit for a behavior-changing C26 slice and should support a later behavior
repair covering default publication titles, proposal titles, DOCX metadata,
export filename basenames, and published-reader accessibility expectations.

Actual ΔV: documentation checkpoint only; C26 remains active.

Receipts:
- `rg -n "Published VText|published-vtext|Untitled VText|VText proposal|Published Texture|published-texture|aria-label=.*Published|Published Vtext" internal/platform internal/proxy frontend/src frontend/tests -g '!frontend/dist/**'`
  found scoped current fallback/default residues in platform publication code
  and one stale frontend publication-reader test expectation.
- Problem checkpoint added to
  `docs/mission-texture-hard-cutover-v0.md` with conjecture delta, protected
  surfaces, admissible evidence class, rollback path, heresy delta, and next
  behavior slice design.

Open edge: commit and push the docs checkpoint, monitor the report-only docs
truth checker, then implement C26 behavior changes.

## 2026-06-16 - Local Repair: Publication Fallback Texture Labels

Claim: C26 is supported for local platform/build scope. Current publication
fallback/default writers can mint Texture-named labels and fallback filenames
without changing live `/pub/vtext/...` public-route compatibility, broad
`PublishVText` Go API symbols, storage names, or exported HTML/CSS class names.

Move: replace scoped fallback/default strings with shared Texture-named
platform constants, update the published-reader accessibility expectation, and
add focused platform tests for pure fallback helpers plus persisted default
publication/proposal titles.

Expected ΔV: no coarse V decrease until CI/deploy/staging proof lands, but the
active C26 code residue should become local-supported.

Actual ΔV: C26 is local-supported. Deployed proof remains open.

Protected surfaces: platform publication default titles, proposal default
titles, publication document construction, DOCX core metadata, export filename
basenames, published-reader accessibility assertions, focused platform tests,
and frontend build.

Admissible evidence class: local full touched-package test plus frontend build;
browser proof is not claimed locally because no local server was running.

Rollback path: restore the previous V-name default strings and test
expectations if CI, deployed route minting, publication reads, proposals,
export filenames, DOCX metadata, or published-reader accessibility regress.

Heresy delta: repaired locally for current publication fallback/default writers;
legacy public route identity, broad Go API names, storage names, and exported
CSS class names remain separately classified residue.

Receipts:
- Docs checkpoint commit
  `52f67a0893ad09fd5f5933067dede245fc3a946f` pushed to `origin/main`; Docs
  Truth Check run `27595386118` passed.
- Focused platform tests passed:
  `nix develop -c go test ./internal/platform -run 'TestPublicationFallbackDefaultsUseTextureLabels|TestPublicationPersistedDefaultTitlesUseTextureLabels|TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes|TestPublishVTextCreatesImmutablePublicRecords' -count=1`.
- Full touched package passed:
  `nix develop -c go test ./internal/platform -count=1`.
- Frontend build passed:
  `npm --prefix frontend run build`.
- Scoped residue search found only the new negative test assertion:
  `rg -n "Published VText|published-vtext|Untitled VText|VText proposal|Published Vtext" internal/platform frontend/src frontend/tests -g '!frontend/dist/**'`.
- Local Playwright publication spec was attempted but failed before page load
  because no server was listening on `http://localhost:4173`; it is not
  evidence for or against the changed assertion.

Open edge: commit and push the behavior, monitor CI/deploy/staging identity,
then run deployed publication/read/export proof against `https://choir.news`.

## 2026-06-16 - Deployed Evidence: Publication Fallback Texture Labels

Claim: C26 is supported for deployed reachable product scope plus CI-covered
platform fallback defaults. The empty-title platform fallback writer paths are
not directly reachable through the browser-public publication API because
Texture document creation requires a title and proxy publication forwards that
document title to platformd.

Move: monitor the C26 behavior commit through CI and Node B deploy, verify
staging health reports the pushed SHA, then run a temporary deployed Playwright
proof that creates a Texture document, publishes it through
`/api/platform/texture/publications`, resolves/exports the publication, opens
the public reader, and asserts Texture route/export/aria labels with no
V-name residue in the reachable surfaces.

Expected ΔV: support C26 for deployed product scope; no coarse V decrease
because `.vtext` file suffixes, storage names, durable `vtext:` actor ids,
`/pub/vtext` route compatibility, exported HTML/CSS class names, Universal
Wire deployed story-field proof, and protocol v0 remain open.

Actual ΔV: C26 is deployed-supported for reachable product scope and
CI-supported for the platform-only empty fallback defaults. V remains 2.

Receipts:
- Problem checkpoint commit:
  `52f67a0893ad09fd5f5933067dede245fc3a946f docs: checkpoint publication fallback texture labels`.
- Behavior commit:
  `0b5d293afbca61f3c1e467e5b7d910a59d02cca0 platform: rename publication fallback labels to texture`.
- CI run `27595560138` passed, including runtime shards, non-runtime package
  tests, vet/build, TLA+, Docs Truth Check, and deploy gate.
- Deploy job `81585218930` passed.
- Docs Truth Check run `27595560180` passed.
- FlakeHub publish run `27595560149` passed.
- `https://choir.news/health` reported proxy and sandbox commit
  `0b5d293afbca61f3c1e467e5b7d910a59d02cca0`, deployed at
  `2026-06-16T05:10:09Z`.
- Temporary deployed proof passed:
  `CHOIR_DEPLOYED_BASE_URL=https://choir.news BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-publication-fallback-staging.tmp.spec.js`.
  The proof created a Texture document, published it through
  `/api/platform/texture/publications`, observed a `/pub/texture/...` route
  without `/pub/vtext/`, resolved the publication, exported TXT and DOCX
  filenames without `vtext`, opened the public reader, and observed
  `aria-label="Published Texture document"` with no old accessibility label.
- The temporary spec was deleted after the proof.

Open edge: choose the next residue class. The strongest candidates are broader
`.vtext` file/alias suffix design, durable `vtext:` actor ids, storage table
names, `/pub/vtext` public route compatibility policy, exported HTML/CSS class
names, and the deployed Universal Wire story-field proof once staging can
provide an edition story payload through product paths.

## 2026-06-16 - Problem Checkpoint: Exported HTML Texture Class Names

Claim: C27 is an admissible bounded residue class. Platform HTML publication
exports are current generated artifacts that still emit retired-name CSS
classes/ids, but the repair can avoid live editor CSS, storage/file suffixes,
durable actor ids, public route compatibility, and broad Go API symbols.

Move: read-only inventory over platform HTML export rendering, embedded profile
CSS, and focused platform tests; document the problem before behavior changes;
compact the oversized Parallax State so the current handoff is state-shaped
instead of a ledger mirror.

Expected ΔV: no coarse V decrease. This checkpoint creates the admissible first
commit for the C27 behavior slice and should make the next move unambiguous.

Actual ΔV: documentation checkpoint only; C27 remains active.

Receipts:
- `internal/platform/export_html.go` currently emits `vtext-publication`,
  `vtext-table`, `vtext-source-ref`, `vtext-sources`, and
  `vtext-sources-heading` in generated HTML and CSS.
- `internal/platform/service_test.go` asserts the old HTML class contract.
- Problem checkpoint added to
  `docs/mission-texture-hard-cutover-v0.md` with conjecture delta, protected
  surfaces, admissible evidence class, rollback path, heresy delta, and next
  behavior slice design.
- The Parallax State in `docs/mission-texture-hard-cutover-v0.md` was compacted
  to current claims/open edges with C27 as the next move.

Open edge: commit and push the docs checkpoint, monitor Docs Truth Check, then
implement C27 behavior changes.

## 2026-06-16 - Local Repair: Exported HTML Texture Class Names

Claim: C27 is supported for local platform test scope. New platform HTML
publication exports can emit Texture-named classes/ids without changing live
editor CSS classes, source manifests, publication routes, JSON-LD, profile
metadata, storage, actor ids, file suffixes, or broad Go API symbols.

Move: rename generated export HTML classes/ids and embedded profile CSS from
`vtext-*` to `texture-*`; update focused platform tests to assert
Texture-class presence and old export-class absence.

Expected ΔV: no coarse V decrease until CI/deploy/staging proof lands; the
active C27 code residue should become local-supported.

Actual ΔV: C27 is local-supported. Deployed proof remains open.

Protected surfaces: platform HTML export rendering, embedded export CSS,
source citation anchors, source-list accessibility ids, and focused platform
tests.

Admissible evidence class: local focused and full `internal/platform` tests,
doccheck, and scoped residue search. Browser/product proof is not claimed until
staging deploy.

Rollback path: restore previous V-name export classes/ids and test
expectations if CI or staging proves generated HTML layout, source anchors,
source lists, or profile styling regressed.

Heresy delta: repaired locally for new exported HTML artifact classes/ids; live
editor CSS classes and broad storage/actor/file/public-route residue remain
separate.

Receipts:
- Docs checkpoint commit `0936099068e8c90c0d07c57a775a718561356881` pushed to
  `origin/main`; Docs Truth Check run `27595845648` passed.
- Focused platform tests passed:
  `nix develop -c go test ./internal/platform -run 'TestPublishVTextCreatesImmutablePublicRecords|TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes' -count=1`.
- Full touched package passed:
  `nix develop -c go test ./internal/platform -count=1`.
- Report-only doccheck passed:
  `scripts/doccheck --report /tmp/choir-doccheck-c27-local.md --json /tmp/choir-doccheck-c27-local.json`.
- Scoped old export class search found only negative assertions:
  `rg -n "vtext-publication|vtext-source-ref|vtext-table|vtext-sources|vtext-sources-heading" internal/platform -g '!frontend/dist/**'`.

Open edge: commit and push the behavior, monitor CI/deploy/staging identity,
then run deployed product-path HTML export proof against `https://choir.news`.

## 2026-06-16 - Deployed Evidence: Exported HTML Texture Class Names

Claim: C27 is supported for deployed product-path HTML export scope. New
platform HTML publication exports emit Texture-named generated artifact
classes/ids and do not emit the scoped retired export classes.

Move: monitor the C27 behavior commit through CI and Node B deploy, verify
staging health reports the pushed SHA, then run a temporary deployed Playwright
proof that creates a Texture with a table and source citation, publishes it
through `/api/platform/texture/publications`, exports HTML, and asserts
Texture-named article/table/source/source-list classes plus old-class absence.

Expected ΔV: support C27 for deployed product scope; no coarse V decrease
because `.vtext` file suffixes, storage names, durable `vtext:` actor ids,
`/pub/vtext` route compatibility, live editor CSS class residue, Universal Wire
deployed story-field proof, and protocol v0 remain open.

Actual ΔV: C27 is deployed-supported. V remains 2.

Receipts:
- Problem checkpoint commit:
  `0936099068e8c90c0d07c57a775a718561356881 docs: checkpoint exported html texture classes`.
- Behavior commit:
  `8cca9ccedabd6323fb57644a53b1835e2eb46329 platform: rename publication html export classes to texture`.
- CI run `27595966664` passed, including runtime shards, non-runtime package
  tests, vet/build, TLA+, Docs Truth Check, and deploy gate.
- Deploy job `81586443933` passed.
- Docs Truth Check run `27595966703` passed.
- FlakeHub publish run `27595966668` passed.
- `https://choir.news/health` reported proxy and sandbox commit
  `8cca9ccedabd6323fb57644a53b1835e2eb46329`, deployed at
  `2026-06-16T05:21:50Z`.
- Temporary deployed proof passed:
  `CHOIR_DEPLOYED_BASE_URL=https://choir.news BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-html-export-classes-staging.tmp.spec.js`.
  The proof created a Texture document containing a table and source citation,
  published it through `/api/platform/texture/publications`, exported HTML, and
  observed `texture-publication`, `texture-table`, `texture-source-ref`,
  `texture-sources`, and `texture-sources-heading` in generated HTML/CSS with
  no `vtext-publication`, `vtext-table`, `vtext-source-ref`, `vtext-sources`,
  or `vtext-sources-heading`.
- The temporary spec was deleted after the proof.

Open edge: choose the next residue class. Strongest remaining candidates are
broader `.vtext` file/alias suffix design, durable `vtext:` actor ids, storage
table names, `/pub/vtext` public route compatibility policy, live editor CSS
class residue, and Universal Wire deployed story-field proof once staging can
provide an edition story payload through product paths.

## 2026-06-16 - Problem Checkpoint: Live Editor Texture Source Classes

Claim: C28 is an admissible bounded residue class. The live Texture renderer
and source journal flow still emit/stylize retired-name source classes in
current product DOM, but the repair can avoid frontend file/module names,
storage/file suffixes, durable actor ids, public route compatibility, and broad
Go API symbols.

Move: read-only inventory over source-ref rendering, Markdown serialization,
source journal flow CSS/DOM construction, VTextEditor source-ref styling, and
focused source-flow tests; document the problem before behavior changes.

Expected ΔV: no coarse V decrease. This checkpoint creates the admissible first
commit for the C28 behavior slice and should make the next move unambiguous.

Actual ΔV: documentation checkpoint only; C28 remains active.

Receipts:
- `frontend/src/lib/vtext-source-renderer.ts` emits `vtext-source-ref*`,
  `vtext-transclusion-*`, `vtext-source-facts`, and `vtext-source-open`
  classes in live rendered source refs.
- `frontend/src/lib/VTextEditor.svelte` styles `.vtext-source-ref*` live
  source refs and popovers.
- `frontend/src/lib/vtext-source-flow.ts` and
  `frontend/src/lib/vtext-source-flow.css` create/style `vtext-source-journal-*`,
  `vtext-source-flow-close`, `vtext-source-open`, and
  `--vtext-source-flow-*`.
- `frontend/tests/vtext-source-entities.spec.js` still inspects some old class
  names for source-flow geometry and old-card absence.
- Problem checkpoint added to
  `docs/mission-texture-hard-cutover-v0.md` with conjecture delta, protected
  surfaces, admissible evidence class, rollback path, heresy delta, and next
  behavior slice design.

Open edge: commit and push the docs checkpoint, monitor Docs Truth Check, then
implement C28 behavior changes.

## 2026-06-16 - Local Repair: Live Editor Texture Source Classes

Claim: C28 repairs the current live editor DOM/CSS class vocabulary for source
refs and source journal flows without widening into frontend file/module names,
storage/file suffixes, durable actor ids, Go publication symbols, or public
route compatibility.

Move: mechanically rename scoped live source-ref/source-flow classes and CSS
custom properties from retired-name forms to Texture forms, repair the accidental
module import-path rename, then run frontend build plus scoped retired-name
search.

Expected ΔV: support C28 locally and make the deployed proof the only remaining
obligation for this slice; no coarse V decrease until CI/deploy/staging proof
passes.

Actual ΔV: C28 is local-supported. V remains 2.

Receipts:
- Docs checkpoint commit:
  `b61659e1163eb662b945c6f0a0150ca469dee791 docs: checkpoint live editor texture source classes`.
- Docs Truth Check run `27596230390` passed for the checkpoint.
- `frontend/src/lib/vtext-source-renderer.ts`,
  `frontend/src/lib/vtext-markdown-serializer.ts`,
  `frontend/src/lib/VTextEditor.svelte`,
  `frontend/src/lib/vtext-source-flow.ts`,
  `frontend/src/lib/vtext-source-flow.css`, and
  `frontend/tests/vtext-source-entities.spec.js` now use
  `texture-source-ref*`, `texture-source-journal-*`,
  `texture-source-flow-close`, `texture-source-open`, and
  `--texture-source-flow-*` for the scoped live editor/source-flow surface.
- `npm --prefix frontend run build` passed.
- Scoped retired-class search returned no hits:
  `rg -n "vtext-source-ref|vtext-source-journal|vtext-source-open|vtext-source-flow-close|--vtext-source-flow" frontend/src/lib frontend/tests/vtext-source-entities.spec.js`.
- Scoped Texture-class search shows the replacement classes in renderer,
  serializer, editor CSS, source-flow CSS/DOM builder, and focused tests:
  `rg -n "texture-source-ref|texture-source-journal|texture-source-open|texture-source-flow-close|--texture-source-flow" frontend/src/lib frontend/tests/vtext-source-entities.spec.js`.

Open edge: commit and push the behavior slice, monitor CI/deploy, verify staging
identity, and run deployed browser/product proof that live source refs and
source journal flows emit Texture classes without the scoped retired classes.

## 2026-06-16 - Deployed Proof: Live Editor Texture Source Classes

Claim: C28 is supported for deployed product scope. Live Texture source refs and
source journal flows now use Texture-named classes in the deployed editor, and
the scoped retired live source classes are absent from the proof surface.

Move: push the C28 behavior commit, monitor CI and Node B deploy, verify staging
health identity, run a temporary deployed Playwright proof that creates a
Texture document through `/api/texture/documents`, opens the deployed Texture
app, clicks a live source ref, and asserts Texture source-ref/source-flow
classes plus old-class absence.

Expected ΔV: support C28 for deployed product scope. No coarse V decrease
because `.vtext` file suffixes, storage names, durable `vtext:` actor ids,
`/pub/vtext` route compatibility, Universal Wire deployed story-field proof,
and protocol v0 remain open.

Actual ΔV: C28 is deployed-supported. V remains 2.

Receipts:
- Behavior commit:
  `7e9d90dc72d964b86881c29c21a7e7a216355d38 frontend: rename live source classes to texture`.
- CI run `27596392703` passed, including runtime shards, non-runtime package
  tests, vet/build, TLA+, Docs Truth Check, frontend build, and deploy gate.
- Deploy job `81587679005` passed.
- Docs Truth Check run `27596392711` passed.
- FlakeHub publish run `27596392712` passed.
- `https://choir.news/health` reported proxy and sandbox commit
  `7e9d90dc72d964b86881c29c21a7e7a216355d38`, deployed at
  `2026-06-16T05:33:26Z`.
- Temporary deployed proof passed:
  `CHOIR_DEPLOYED_BASE_URL=https://choir.news BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-live-source-classes-staging.tmp.spec.js`.
  The proof created a Texture document through `/api/texture/documents`, added a
  revision with a source entity, opened the deployed Texture app through its
  visible app affordance, clicked the source citation, and observed
  `texture-source-ref`, `texture-source-ref-label`,
  `texture-source-ref-popover`, `texture-source-journal-flow`,
  `texture-source-journal-lines`, `texture-source-journal-line`,
  `texture-source-journal-note`, `texture-source-open`, and
  `texture-source-flow-close` with no `vtext-source-ref*`,
  `vtext-source-journal-*`, `vtext-source-open`, or `vtext-source-flow-close`
  on the proof surface.
- The first temporary proof attempt timed out before the target surface because
  the deployed desktop no longer exposed the old `data-desktop-icon-id="vtext"`
  selector in that session; the passing proof used the visible `Texture` app
  affordance instead. The temporary spec and `frontend/test-results` scratch
  output were deleted after proof.

Open edge: choose the next residue class. Strongest remaining candidates are
broader `.vtext` file/alias suffix design, durable `vtext:` actor ids, storage
table names, `/pub/vtext` public route compatibility policy, and Universal Wire
deployed story-field proof once staging can provide an edition story payload
through product paths.

## 2026-06-16 - Problem Checkpoint: Public Legacy Publication Routes

Claim: C29 is an admissible bounded residue class. Current frontend/browser
route recognition and source-reader fixtures still treat `/pub/vtext/...` as
current public route spelling, but current publication minting already uses
`/pub/texture/...`; backend support for stored legacy public route rows should
remain explicit compatibility residue until a later storage migration decides
whether to rewrite or delete those rows.

Move: read-only inventory over frontend public route recognition, desktop route
normalization, source-reader/publication tests, platform route normalization,
and proxy resolve/export tests; document the problem before behavior changes.

Expected ΔV: no coarse V decrease. This checkpoint creates the admissible first
commit for the C29 behavior slice and should make the next move unambiguous.

Actual ΔV: documentation checkpoint only; C29 remains active.

Receipts:
- `frontend/src/App.svelte` recognizes both `/pub/texture/...` and
  `/pub/vtext/...` for public first-load route detection.
- `frontend/src/lib/Desktop.svelte` normalizes both `/pub/texture/...` and
  `/pub/vtext/...` when opening public publication routes.
- `frontend/tests/vtext-source-entities.spec.js` contains `/pub/vtext/...`
  source-reader/publication fixtures.
- `internal/platform/service.go` still defines `legacyPublicVTextPrefix =
  "/pub/vtext/"` and normalizes trailing slashes for stored legacy route rows.
- `internal/platform/service_test.go` manually inserts a legacy `/pub/vtext/...`
  route row and asserts backend bundle resolution still works.
- `internal/proxy/platform_public_test.go` verifies unresolved
  `/pub/vtext/private` resolve/export requests return 404 through the proxy,
  proving the proxy forwards old spelling rather than rejecting it at the
  boundary.
- Problem checkpoint added to
  `docs/mission-texture-hard-cutover-v0.md` with conjecture delta, protected
  surfaces, admissible evidence class, rollback path, heresy delta, and next
  behavior slice design.

Open edge: commit and push the docs checkpoint, monitor Docs Truth Check, then
implement the C29 frontend/browser route recognition and fixture repair while
leaving backend stored-route migration as a separate explicit edge.

## 2026-06-16 - Local Repair: Public Legacy Publication Routes

Claim: C29 repairs current frontend/browser public route vocabulary without
claiming backend storage migration. New/current product surfaces should use
`/pub/texture/...`; stored `/pub/vtext/...` public route rows remain a tagged
backend compatibility shim until a later migration rewrites or deletes them.

Move: remove `/pub/vtext/...` from frontend first-load public route recognition
and desktop public route normalization, update source-reader/publication
fixtures to `/pub/texture/...`, tag the backend legacy prefix as cutover
compatibility residue, and run focused local verification.

Expected ΔV: support C29 locally and make deployed proof the only remaining
obligation for this slice; no coarse V decrease until CI/deploy/staging proof
passes.

Actual ΔV: C29 is local-supported. V remains 2.

Receipts:
- Docs checkpoint commit:
  `4aa9bf294047fa1e2ff5a124d4392755a414a5c9 docs: checkpoint public legacy texture routes`.
- Docs Truth Check run `27596756722` passed for the checkpoint.
- `frontend/src/App.svelte` now recognizes only `/pub/texture/...` for public
  Texture first-load routes.
- `frontend/src/lib/Desktop.svelte` now normalizes only `/pub/texture/...` as a
  public Texture route.
- `frontend/tests/vtext-source-entities.spec.js` publication/source-reader
  fixture route paths now use `/pub/texture/...`.
- `internal/platform/service.go` keeps `legacyPublicVTextPrefix` but tags it as
  `texture-cutover-allow` compatibility residue pending public route storage
  migration.
- Scoped current-frontend search returned no hits:
  `rg -n "pub/vtext|/pub/vtext" frontend/src frontend/tests/vtext-source-entities.spec.js --glob '!frontend/dist/**'`.
- Wider platform/proxy/frontend route search shows only the documented backend
  legacy shim/tests:
  `internal/platform/service.go`, `internal/platform/service_test.go`, and
  `internal/proxy/platform_public_test.go`.
- `npm --prefix frontend run build` passed.
- `nix develop -c go test ./internal/proxy -count=1` passed.
- `nix develop -c go test ./internal/platform -run 'TestPublishVTextCreatesImmutablePublicRecords' -count=1` passed.

Open edge: commit and push the behavior slice, monitor CI/deploy, verify staging
identity, and run deployed product proof that a newly published Texture mints
and loads through `/pub/texture/...` while current frontend surfaces do not
carry `/pub/vtext/...`.

## 2026-06-16 - Deployed Proof: Public Legacy Publication Routes

Claim: C29 is supported for deployed product scope. New/current public
publication surfaces mint and load through `/pub/texture/...`, current frontend
route recognition no longer treats `/pub/vtext/...` as a public reader route,
and backend `/pub/vtext/...` support is explicitly scoped as stored-route
compatibility residue.

Move: push the C29 behavior commit, monitor CI and Node B deploy, verify staging
health identity, run a temporary deployed Playwright proof that creates a
Texture, publishes it through `/api/platform/texture/publications`, opens the
published `/pub/texture/...` route, and checks a same-slug `/pub/vtext/...`
route is not rendered as a public reader.

Expected ΔV: support C29 for deployed product scope. No coarse V decrease
because `.vtext` file suffixes, storage names, durable `vtext:` actor ids,
Universal Wire deployed story-field proof, backend public-route storage
migration, and protocol v0 remain open.

Actual ΔV: C29 is deployed-supported. V remains 2.

Receipts:
- Behavior commit:
  `6e84f0e1756e626abff88617690199e2879994bb frontend: stop recognizing legacy vtext public routes`.
- CI run `27596903040` passed, including runtime shards, non-runtime package
  tests, vet/build, TLA+, Docs Truth Check, frontend build, and deploy gate.
- Deploy job `81589137652` passed.
- Docs Truth Check run `27596903042` passed.
- FlakeHub publish run `27596903056` passed.
- `https://choir.news/health` reported proxy and sandbox commit
  `6e84f0e1756e626abff88617690199e2879994bb`, deployed at
  `2026-06-16T05:46:42Z`.
- Temporary deployed proof passed:
  `CHOIR_DEPLOYED_BASE_URL=https://choir.news BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-public-route-staging.tmp.spec.js`.
  The proof created a Texture document through `/api/texture/documents`,
  published it through `/api/platform/texture/publications`, verified the
  returned `route_path` matched `/pub/texture/...` and not `/pub/vtext/...`,
  resolved the route through `/api/platform/publications/resolve`, opened the
  public reader at the `/pub/texture/...` route, found no `/pub/vtext/...` links
  in the reader, then opened the same slug under `/pub/vtext/...` and observed
  no `[data-texture-published-reader]`, with the authenticated desktop visible
  instead.
- The temporary spec and `frontend/test-results` scratch output were deleted
  after proof.

Open edge: choose the next residue class. Strongest remaining candidates are
broader `.vtext` file/alias suffix design, durable `vtext:` actor ids, storage
table names, and Universal Wire deployed story-field proof once staging can
provide an edition story payload through product paths.

## 2026-06-16 - Problem Checkpoint: Universal Wire Style Texture Suffixes

Claim: C30 is an admissible bounded residue class. Current Universal Wire and
coagent style-source prompt/default surfaces still introduce `Style.vtext`
labels and `.style.vtext` source paths, but this can be repaired without
touching canonical `.vtext` import/open behavior, storage aliases, file-browser
shortcuts, durable actor ids, or metadata compatibility keys.

Move: read-only inventory over coagent prompt construction, Universal Wire
defaults and generated-content cleanup filters, runtime tool profiles, processor
prompt defaults, and focused tests; document the problem before behavior
changes.

Expected ΔV: no coarse V decrease. This checkpoint creates the admissible first
commit for the C30 behavior slice and should make the next move unambiguous.

Actual ΔV: documentation checkpoint only; C30 remains active.

Receipts:
- `internal/runtime/tools_coagent.go` emits `## Style.vtext Source`, `Selected
  Style.vtext source context`, `Style.vtext` reader-facing exclusion rules,
  default style source titles such as `Style.vtext: Universal Wire`, default
  source paths such as `styles/universal-wire.style.vtext`, and style-selection
  rationales ending in `Style.vtext`.
- `internal/runtime/universal_wire.go` supplies default title
  `Style.vtext: Universal Wire` and filters generated `Style.vtext Source`
  headings.
- `internal/runtime/tool_profiles.go` and
  `internal/runtime/prompt_defaults/processor.md` still tell agents to pass
  `Style.vtext` needs.
- Runtime tests in
  `internal/runtime/{runtime,universal_wire,agent_tools}_test.go` assert
  `Style.vtext` prompt content and metadata.
- Problem checkpoint added to
  `docs/mission-texture-hard-cutover-v0.md` with conjecture delta, protected
  surfaces, admissible evidence class, rollback path, heresy delta, and next
  behavior slice design.

Open edge: commit and push the docs checkpoint, monitor Docs Truth Check, then
implement C30 behavior changes: current style-source labels/paths and prompt
contracts move to `Style.texture` / `.style.texture`; legacy `Style.vtext`
cleanup recognition stays explicitly scoped.

## 2026-06-16 - Local Repair: Universal Wire Style Texture Suffixes

Claim: C30 can repair current Universal Wire style-source prompt/default
surfaces by moving `Style.vtext` / `.style.vtext` to `Style.texture` /
`.style.texture`, while preserving legacy generated-content cleanup for old
`Style.vtext Source` headings and leaving canonical file/storage migration out
of scope.

Move: construct the bounded repair after the documentation-first checkpoint.
Updated coagent seed/revision prompts, default Wire style-source titles and
paths, runtime profile/default prompt text, focused runtime tests, Universal
Wire story cleanup, and the Universal Wire UI test expectation.

Expected ΔV: no coarse V decrease until CI/deploy/deployed proof lands; local
evidence should support committing the behavior slice.

Actual ΔV: C30 moved from active to local-supported. Mission V remains 2
because CI/deploy and deployed acceptance are still pending, and the broader
Universal Wire deployed story-field proof remains open.

Receipts:

- Problem checkpoint commit
  `a59b86f2acffb669a851c44c75b03a5db7b6c514` landed first; Docs Truth Check
  run `27597206898` passed.
- `internal/runtime/tools_coagent.go` now emits `## Style.texture Source`,
  `Selected Style.texture source context`, default titles such as
  `Style.texture: Universal Wire`, and source paths such as
  `styles/universal-wire.style.texture`.
- `internal/runtime/tool_profiles.go` and
  `internal/runtime/prompt_defaults/processor.md` now instruct agents to pass
  relevant `Style.texture` needs.
- `internal/runtime/universal_wire.go` now defaults selected style title to
  `Style.texture: Universal Wire` and strips both current `Style.texture
  Source` and legacy `Style.vtext Source` generated headings.
- `nix develop -c go test ./internal/runtime -run
  'TestHandleUniversalWireStories|TestWireArticle|TestCoagent|TestProcessor|Test.*Style|TestVTextPrompt|TestAgentTools|TestSystemPromptForUniversalWireVTextRunsRequiresArticleHead'
  -count=1` passed.
- `nix develop -c scripts/go-test-runtime-shards` passed all runtime shards.
- `npm --prefix frontend run build` passed with only pre-existing Universal
  Wire warnings about the unused `currentUser` export and unused `.wire-state`
  selectors.
- `PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 npm --prefix frontend run e2e --
  --project=chromium tests/universal-wire-app.spec.js -g 'deletes detritus
  source chronology and bespoke style controls'` passed against local Vite,
  asserting both retired `Style.vtext` and current internal `Style.texture`
  labels stay out of the visible Universal Wire UI.
- Scoped search for `Style.vtext` / `style.vtext` in the touched runtime and
  Universal Wire test surfaces found only `internal/runtime/universal_wire.go`
  legacy cleanup filters and `internal/runtime/universal_wire_test.go` negative
  fixture/assertion coverage.

Open edge: commit and push the behavior repair, monitor CI/deploy, verify
staging identity, and record deployed evidence or the precise product-proof
blocker for this prompt/default slice. Do not claim canonical `.vtext`
file/storage migration or Universal Wire deployed story-field proof from this
local repair.

## 2026-06-16 - Deployed Evidence: Universal Wire Style Texture Suffixes

Claim: C30 is deployed-supported at its bounded scope: current Universal Wire
style-source prompt/default surfaces now introduce `Style.texture` /
`.style.texture`; deployed UI proof keeps style-source labels out of reader
surfaces; legacy `Style.vtext Source` recognition remains only cleanup.

Move: push C30 behavior, accept the read-only prover finding that the UI test
needed to guard both old and current style labels, land the follow-up test guard,
force a staging deploy after the follow-up docs/test commit skipped deploy
impact, then verify staging identity and deployed UI behavior.

Expected ΔV: no coarse V decrease until deployment identity and deployed proof
are recorded; C30 should then be closed without claiming broader file/storage or
story-field proof.

Actual ΔV: C30 moved from local-supported to deployed-supported. Mission V
remains 2 because storage/file/durable actor/export/stored-route residue,
Universal Wire deployed story-field proof, and protocol v0 remain open.

Receipts:

- Behavior commit `9b77112902eaa3f7ab308e7ff976c5f3fcb5f13a` pushed C30
  runtime prompt/default and cleanup changes.
- Read-only prover `c30_diff_review` found one test gap: the Universal Wire UI
  test should assert both retired `Style.vtext` and current internal
  `Style.texture` labels absent. Follow-up commit
  `d05cbc5556227ec9c3b5826a101128725532e882` added that guard and updated
  mission evidence.
- Local re-checks after the follow-up:
  `PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 npm --prefix frontend run e2e --
  --project=chromium tests/universal-wire-app.spec.js -g 'deletes detritus
  source chronology and bespoke style controls'` passed, and
  `scripts/doccheck --report /tmp/choir-doccheck-c30-review-fix.md --json
  /tmp/choir-doccheck-c30-review-fix.json` passed in report-only mode.
- Push CI run `27597833570` for
  `d05cbc5556227ec9c3b5826a101128725532e882` passed. The preceding behavior
  push CI run `27597769875` was cancelled by the follow-up push before deploy.
- Manual CI run `27597934917` was dispatched with
  `force_staging_deploy=true`; all gates passed and deploy job `81592293236`
  succeeded.
- `https://choir.news/health` reported proxy and sandbox commit
  `d05cbc5556227ec9c3b5826a101128725532e882`, deployed at
  `2026-06-16T06:12:17Z`.
- Deployed proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  --project=chromium tests/universal-wire-app.spec.js -g 'deletes detritus
  source chronology and bespoke style controls'`.

Open edge: choose the next residue class. C30 does not repair canonical
`.vtext` file import/open behavior, storage schema, workspace paths, metadata
compatibility keys, durable `vtext:` actor ids, stored document titles, backend
stored-route rows, or Universal Wire story-field proof.

## 2026-06-16 - Problem Checkpoint: C31 Publication Helper Symbols

Claim: the next highest-value bounded residue class is publication/export
helper and API symbol naming. Current product routes and publication records
are Texture-shaped, but code symbols still say `PublishVText`,
`publishVText`, `publishVTextRequest`, `sandboxVTextDocument`, and
`sandboxVTextRevision` on the active publication boundary.

Move: probe by read-only inventory and document the problem before any behavior
change. Expected ΔV: no coarse V decrease yet; this checkpoint should make the
C31 repair admissible without crossing storage/public-route/actor migrations.

Actual ΔV: documentation checkpoint only; C31 remains active.

Receipts:

- Read-only subagent `actor_route_export_residue_probe` classified stored
  `/pub/vtext/...` route rows and durable `vtext:` actor ids as migration
  surfaces, but identified publication helper/API symbols as an
  orange/yellow slice that can avoid storage migration.
- Read-only subagent `storage_file_residue_probe` classified durable storage
  names, `.vtext` shortcut files, and durable actor identity as migration
  surfaces.
- Local inspection confirmed current hits in `internal/platform/types.go`,
  `internal/platform/service.go`, `internal/platform/handlers.go`,
  `internal/wirepublish`, `internal/proxy/platform_publish.go`,
  `internal/runtime/wire_platform_publish.go`, `frontend/src/lib/vtext.js`,
  and `frontend/src/lib/VTextEditor.svelte`.

Open edge: run doccheck and commit this checkpoint, then rename the C31
publication/export helper/API symbols while preserving JSON fields, HTTP
routes, storage schema, stored route compatibility, `.vtext` file suffixes,
and durable `vtext:` actor ids.

## 2026-06-16 - Local Evidence: C31 Publication Helper Symbols

Claim: C31 can repair active publication/export helper and API symbols without
changing publication routes, JSON fields, storage schema, or stored public route
compatibility.

Move: construct by renaming platform, proxy, wirepublish, runtime, and frontend
publication helper symbols to Texture names. Expected ΔV: no coarse V decrease
until CI/deploy and deployed publication proof; local-supported status should
remove the active helper-symbol residue from the export/storage coarse bucket.

Actual ΔV: C31 moved from documented to local-supported. Mission V remains 2
until deployment and staging proof are recorded.

Receipts:

- Checkpoint commit `268db43c234f57fdea6e65870b11568805706e7c` was pushed
  first, and Docs Truth Check run `27598505265` passed.
- `internal/platform` now uses `PublishTextureRequest`,
  `PublishTextureResponse`, `Service.PublishTexture`, and
  `HandleInternalPublishTexture`.
- `internal/proxy` now uses `HandleTexturePublication`,
  `publishTextureRequest`, and `sandboxTextureDocument` /
  `sandboxTextureRevision` helper structs.
- `internal/wirepublish`, `internal/runtime`, and the Texture editor publish
  callsite now use Texture-named publication helpers.
- `nix develop -c go test ./internal/platform ./internal/proxy
  ./internal/wirepublish ./internal/runtime -run
  'TestInternalPublishRequiresInternalCallerAndBundleResolve|TestRegisteredTextureRoutesExcludeLegacyVTextPlatformPrefix|TestPublishTextureCreatesImmutablePublicRecords|TestPublicationFallbackDefaultsUseTextureLabels|TestPublicationPersistedDefaultTitlesUseTextureLabels|TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes|TestHandleTexturePublication|TestHandleInternalWirePlatformPublishPostsToPlatformd|TestWirePlatform|TestWirePublication|TestPostPlatformPublication|TestBuildAutonomousPublishRequest'
  -count=1` passed.
- `npm --prefix frontend run build` passed with only pre-existing Universal
  Wire warnings.
- Scoped C31 residue search found no targeted helper/API hits for
  `PublishVText`, `publishVText`, `publishVTextRequest`,
  `HandleInternalPublishVText`, `HandleVTextPublication`,
  `HandlePublicVText`, `sandboxVTextDocument`, `sandboxVTextRevision`,
  `failed to publish vtext`, or `publish vtext` in the touched publication
  surfaces.

Open edge: commit and push the behavior repair, monitor CI/deploy, verify
staging identity, and run deployed publication proof. Do not claim storage,
file suffix, durable actor-id, stored public-route-row, or protocol-v0 repair.

## 2026-06-16 - Deployed Evidence: C31 Publication Helper Symbols

Claim: C31 is deployed-supported at its bounded scope: active publication/export
helper and API symbols use Texture names while publication routes, JSON fields,
storage schema, stored public-route compatibility, and durable actor ids stay
unchanged.

Move: push C31 behavior, monitor CI/deploy, confirm staging identity, and run a
deployed publication proof. Expected ΔV: close the C31 helper-symbol sub-edge
without decreasing coarse mission V until storage/file/actor/stored-route
residue, Universal Wire story-field proof, and protocol v0 are resolved.

Actual ΔV: C31 moved from local-supported to deployed-supported. Mission V
remains 2.

Receipts:

- Behavior commit `90746bccead98b839c1c8cc3fa5c537a80ce66fe` pushed C31.
- CI run `27598740366` passed all gates and deploy job `81594789846` passed.
- `https://choir.news/health` reported proxy and sandbox commit
  `90746bccead98b839c1c8cc3fa5c537a80ce66fe`, deployed at
  `2026-06-16T06:31:08Z`.
- Deployed proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  --project=chromium tests/vtext-source-service-publication.spec.js -g
  'publishes source-service source entities as expandable transclusions and
  canonical exports'`.

Open edge: choose the next residue class. C31 does not repair storage tables,
`.vtext` file suffixes, durable `vtext:` actor ids, stored `/pub/vtext/...`
route rows, Universal Wire deployed story-field payload proof, or protocol v0.

## 2026-06-16 - Problem Checkpoint: C32 Texture File Suffix Defaults

Claim: the next highest-value bounded residue class is file-manifest suffix
behavior. Current new manifest allocation, imported-file document titles,
alias priority, File Browser shortcut recognition, and Universal Wire story
open paths still teach `.vtext` as the canonical file suffix.

Move: probe the remaining Universal Wire proof-only edge, then document the
C32 file-suffix problem before changing persistent file/alias behavior.
Expected ΔV: no coarse V decrease yet; this checkpoint should make the C32
repair admissible while explicitly excluding table/database, durable actor-id,
and stored public-route migrations.

Actual ΔV: documentation checkpoint only; C32 remains active.

Receipts:

- Deployed Universal Wire staging acceptance passed:
  `GO_CHOIR_RUN_UNIVERSAL_WIRE_STAGING=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/universal-wire-staging-acceptance.spec.js`.
- Direct authenticated product API inspection of `/api/universal-wire/stories`
  returned `source: universal-wire-edition-texture`, edition source path
  `universal-wire/Wire.vtext`, many included document ids, and `story_count:
  0`; therefore the deployed story-field proof still has no story payload to
  inspect.
- Local source inspection found current file-suffix defaults in
  `internal/runtime/vtext_import.go`, alias priority in
  `internal/store/vtext.go`, File Browser shortcut recognition in
  `frontend/src/lib/FileBrowser.svelte`, and Universal Wire story source open
  paths in `frontend/src/lib/UniversalWireApp.svelte`.

Open edge: run doccheck and commit this checkpoint, then move new/current
manifest defaults and shortcut recognition to `.texture` while keeping legacy
`.vtext` aliases readable and leaving table/database names, durable actor ids,
stored `/pub/vtext/...` rows, and protocol v0 out of scope.

## 2026-06-16 - Local Evidence: C32 Texture File Suffix Defaults

Claim: C32 is locally-supported at its bounded scope. New/current Texture file
manifestations default to `.texture` for import titles, manifest allocation,
manifest shortcut kind, canonical source-path metadata, alias priority, File
Browser shortcut recognition, and Universal Wire story-open source paths, while
legacy `.vtext` shortcuts remain readable.

Move: construct the bounded file-suffix repair after the Problem Documentation
First checkpoint. Expected ΔV: no coarse V decrease until deployed proof; close
the local C32 sub-edge and leave storage/actor/stored-route/protocol residue
explicit.

Actual ΔV: local C32 sub-edge closed; mission V remains 2 pending
CI/deploy/staging product proof.

Receipts:

- `internal/runtime/vtext_import.go` now allocates `.texture` manifests,
  titles imports as `.texture`, emits `kind:"texture"` for current shortcut
  files, accepts legacy `.vtext` shortcut files, and keeps legacy `.vtext`
  shortcut files at `kind:"vtext"`.
- `internal/store/vtext.go` now prefers `.texture` aliases before `.vtext`
  aliases and other aliases.
- `frontend/src/lib/FileBrowser.svelte` recognizes `.texture` shortcut/text
  files while preserving `.vtext` recognition.
- `frontend/src/lib/UniversalWireApp.svelte` opens story source paths as
  `.story.texture`.
- Focused tests passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'APICreateRevisionCanonicalizesAliasedImportedDocumentTitle|OpenFileResolvesCanonicalAlias|PlainTextImportCarriesMigrationMetadataToFirstDurableRevision|ImportedMarkdownRevisionUsesVTextProjectionAndPreservesCollapsedTable|ImportMarkdownLineageCreatesRevisionHistory|OpenFilePreservesDocxAndPDFOriginalArtifacts|EnsureManifestCreatesAliasAndFile|ShortcutFileKindPreservesLegacyVTextCompatibility|EnsureManifestReusesExistingAlias|AppagentEditCanonicalizesAliasedMarkdownTitle'
  -count=1`.
- Focused store alias test passed:
  `nix develop -c go test ./internal/store -run
  'TestVTextDocumentAliasSourcePathPrefersCanonicalShortcut' -count=1`.
- Wider checks passed:
  `nix develop -c scripts/go-test-runtime-shards`,
  `nix develop -c go test ./internal/store -count=1`, and
  `npm --prefix frontend run build` (with pre-existing Universal Wire Svelte
  warnings).
- Scoped residue search over the touched C32 surfaces found only intended
  legacy shortcut compatibility, projection/schema vocabulary, and explicitly
  excluded storage workspace/table residue.

Open edge: commit/push C32, monitor CI/deploy, verify staging identity, and run
deployed product proof for imported Markdown/plain text plus manifest
`.texture` behavior. C32 does not repair storage workspace/table names, durable
`vtext:` actor ids, stored `/pub/vtext/...` rows, Universal Wire edition
`Wire.vtext`, Universal Wire deployed story-field payload proof, or protocol v0.

## 2026-06-16 - Review Correction: C32 Frontend Shortcut Coverage

Claim: independent diff review found C32 frontend gaps before final deployed
proof, and the follow-up repair keeps the same bounded file-suffix surface.

Move: prover shift plus correction. Expected ΔV: no coarse V decrease; remove
review-discovered local frontend gaps before re-running CI/deploy.

Actual ΔV: C32 remains locally-supported, with review gaps repaired; mission V
remains 2 pending staging product proof.

Receipts:

- Independent reviewer `c32_diff_review` found stale `.vtext` shortcut handling
  in `frontend/src/lib/VTextEditor.svelte`, stale `.vtext` manifest expectations
  in `frontend/tests/desktop-shell-core.spec.js`, and missing Universal Wire
  story-open coverage.
- `frontend/src/lib/VTextEditor.svelte` now recognizes both `.texture` and
  legacy `.vtext` shortcut paths, matching File Browser/backend recognition.
- `frontend/tests/desktop-shell-core.spec.js` now expects prompt-created
  manifests to use `.texture` and `kind:"texture"`.
- `frontend/tests/universal-wire-app.spec.js` now clicks a mocked Universal Wire
  story, opens its Texture document, and fails if `.story.texture` triggers an
  unexpected manifest ensure.
- `npm --prefix frontend run build` passed again with the same pre-existing
  Universal Wire Svelte warnings.

Open edge: commit/push the review correction, monitor the superseding CI/deploy,
verify staging identity, and run deployed product proof.

## 2026-06-16 - Deployed Evidence: C32 Texture File Suffix Defaults

Claim: C32 is deployed-supported at its bounded scope. New/current Texture file
manifestations default to `.texture` for import titles, manifest allocation,
manifest shortcut kind, canonical source-path metadata, alias priority, File
Browser/Texture editor shortcut recognition, desktop-shell manifest
expectations, and Universal Wire story-open source paths, while legacy `.vtext`
shortcuts remain readable.

Move: probe the pushed C32 behavior on staging after CI/deploy. Expected ΔV:
close the deployed C32 sub-edge without decreasing the coarse mission V.

Actual ΔV: deployed C32 sub-edge closed; mission V remains 2 because storage
schema/workspace names, durable `vtext:` actor ids, stored `/pub/vtext/...`
route rows, Universal Wire deployed story-field proof, and protocol v0 remain.

Receipts:

- Behavior commits:
  `abc2f89c8f0cb7a37ea99cf50a84dc9386cc1ad4` (`runtime: default texture
  manifests to texture suffix`) and
  `ae2ada4a4b51f9c2671113e9c07dc7c3e5417050` (`frontend: recognize texture
  shortcut manifests`).
- CI run `27600056369` for `ae2ada4a4b51f9c2671113e9c07dc7c3e5417050`
  passed. Deploy job `81598902993` (`Deploy to Staging (Node B)`) passed.
- `curl -fsS https://choir.news/health` reported proxy and sandbox at commit
  `ae2ada4a4b51f9c2671113e9c07dc7c3e5417050`, deployed at
  `2026-06-16T07:00:48Z`.
- Initial reusable staging command:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/vtext-markdown-lineage.spec.js -g 'Imported Markdown advances|Imported plain text advances'`.
  Result: plain-text `.texture` proof passed; Markdown proof completed the API
  assertions for `.texture` title, canonical source metadata, manifest path,
  and `.md` export, then timed out on old desktop/window selectors
  (`data-desktop-icon-id="vtext"` / `data-window-app-id="vtext"`). A fresh auth
  rerun reproduced the selector drift. This is acceptance-harness debt, not a
  C32 product failure.
- Custom deployed browser/product proof used the current product path:
  `PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_AUTH_STATE=/tmp/choir-c32-acceptance-auth-desk.json node --input-type=module`
  from `frontend/`, with the script opening Desk via the bottom bar, launching
  `[data-desk-app-id="texture"]`, and opening the recent Texture. Result:
  passed. Receipt values:
  `doc_id=a70e9601-f49d-428f-aa6e-637d82b9d9e8`,
  `v0_title=imported-md-vtext-1781593985312.texture`,
  `v1_canonical_texture_source_path=imported-md-vtext-1781593985312-texture.texture`,
  `manifest_source_path=imported-md-vtext-1781593985312-texture.texture`,
  `markdown_export_filename=imported-md-vtext-1781593985312.md`,
  `browser_opened_recent_texture_title=imported-md-vtext-1781593985312.texture`,
  and `browser_rendered_version=v1`.

Open edge: decide whether to repair the reusable Playwright helper/spec to use
current Desk/Texture selectors as a yellow test-harness slice, or leave the
custom proof receipt as sufficient for C32 and move next to the remaining
storage/durable actor/stored-route residue. Universal Wire deployed story-field
proof still needs a real staging story payload or product creation path; do not
claim it from route-mocked frontend coverage.

## 2026-06-16 - Harness Evidence: C33 Reusable Texture Acceptance Selectors

Claim: C33 is supported as a yellow proof-surface repair. The reusable
Markdown/plain-text Texture lineage staging proof no longer depends on retired
`vtext` desktop/window selectors and can launch Texture through the current
product shell.

Move: construct the smallest test-harness repair after the C32 deployed proof
identified selector drift. Expected ΔV: close the acceptance-harness drift
sub-edge without decreasing the coarse mission V.

Actual ΔV: acceptance-harness drift sub-edge closed; mission V remains 2.

Receipts:

- `frontend/tests/vtext-markdown-lineage.spec.js` now uses
  `openRecentTextureDocument` and `launchTextureApp` helpers. The launcher
  tries floating desktop icons, left rail buttons, and the compact Desk app
  switcher; the window locator accepts canonical `texture` and legacy `vtext`
  app ids during migration.
- Previously failing deployed command now passed with a fresh auth state:
  `CHOIR_AUTH_STATE=/tmp/choir-c32-harness-auth.json PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/vtext-markdown-lineage.spec.js -g 'Imported Markdown advances|Imported plain text advances'`.
  Result: 2 passed in 15.7s. This re-proved the Markdown `.texture` title,
  canonical source metadata, manifest path, `.md` export, recent Texture open,
  and plain-text `.texture` migration through the reusable spec.
- `npm --prefix frontend run build` passed with the pre-existing Universal Wire
  Svelte warnings.
- `scripts/doccheck --report /tmp/choir-doccheck-c33-harness-final.md --json
  /tmp/choir-doccheck-c33-harness-final.json` passed report-only with 212 docs and
  1118 warnings.

Open edge: C33 does not repair product runtime behavior, storage schema names,
durable `vtext:` actor ids, stored `/pub/vtext/...` rows, Universal Wire
deployed story-field proof, or protocol v0. Next move should attack one of
those remaining product/protocol edges.

## 2026-06-16 - Landing Evidence: C33 Reusable Texture Acceptance Selectors

Claim: C33 is landed as a yellow test/proof-surface repair.

Move: push the harness repair and monitor repository checks. Expected ΔV:
record landing evidence without changing coarse mission V.

Actual ΔV: landing evidence recorded; mission V remains 2.

Receipts:

- Commit `376ac6d9c5439fd7c08c52fa628dc5f341820b97`
  (`test: launch texture acceptance through current shell`) pushed to
  `origin/main`.
- GitHub CI run `27601085720` passed.
- Docs Truth Check run `27601085740` passed.
- FlakeHub publish run `27601085759` passed.
- `Deploy to Staging (Node B)` was skipped by deploy-impact detection because
  the commit changed only tests/docs and no deployed artifact.

Open edge: same as C33 harness evidence above; the next productive move is a
product/protocol edge, not more harness repair unless another proof fails.

## 2026-06-16 - Problem Checkpoint: C34 Storage And Durable Identity Residue

Claim: the next Texture hard-cutover edge is persistent identity residue, not
another surface label. No runtime repair is claimed in this move.

Move: read-only inventory plus Problem Documentation First checkpoint. Expected
ΔV: no repair decrease; convert the storage/durable actor/stored-route residue
from a broad obligation into a typed problem record with admissible evidence and
rollback requirements.

Actual ΔV: coarse V remains 2. C34 is documented as the next red-surface
candidate slice.

Receipts:

- Read `docs/computer-ontology.md` before inspecting persistent-state residue.
- `rg "vtext_|vtext_documents|vtext_revisions|vtext_document_aliases|vtext_agent_mutations|vtext_controller_checkpoints|vtext_decisions|database=vtext|CREATE DATABASE IF NOT EXISTS vtext|\\.vtext|go-choir-vtext" internal/store internal/runtime -n`
  found the storage cluster in `internal/store/vtext.go`,
  `internal/store/dolt_maintenance.go`, runtime metadata, and focused tests.
- `rg "vtext:|AgentProfileVText|role=vtext|spawn_agent.*vtext|ToAgentID.*vtext|AgentID.*vtext" internal/runtime internal/store internal/types -n`
  found durable actor/profile/addressing residue around `AgentProfileVText`,
  `vtext_agent_revision`, and `vtext:<doc_id>`.
- `rg '"/pub/vtext|/pub/vtext|platformd_route_path|published_vtext|published_texture|/pub/texture|route_path' internal frontend/src frontend/tests -n --glob '!frontend/dist/**'`
  showed new publication minting on `/pub/texture/...` and explicit stored
  `/pub/vtext/...` row compatibility in platform tests.
- `curl -fsS 'https://choir.news/api/platform/publications/resolve?route=%2Fpub%2Fvtext%2Fprivate'`
  returned HTTP 404; therefore this pass discovered code/test support for
  legacy route rows but did not prove an active staging legacy row.
- Added `2026-06-16 - C34 Problem Checkpoint: Storage And Durable Identity
  Residue` to the paradoc with conjecture delta, protected surfaces,
  admissible evidence, rollback path, and heresy delta.

Open edge: no behavior change yet. The next admissible move is a typed C34
behavior design or first narrow migration/compatibility slice. It must preserve
existing computers, old actor/update lookups, and any stored public routes while
making the current write identity Texture.

## 2026-06-16 - Local Repair: C34a Texture Workspace Identity

Claim: the filesystem workspace identity subobligation can move to Texture
without table/database migration by using `.texture` for new/current stores and
falling back to `.vtext` only when an existing legacy workspace is present.

Move: construct the bounded storage-open slice. Expected ΔV: repair one C34
subobligation while keeping coarse V=2 because durable actor ids, table names,
stored legacy route rows, Universal Wire edition refs, deployed story-field
proof, and protocol v0 remain.

Actual ΔV: filesystem workspace identity repaired locally; coarse V remains 2.

Receipts:

- `internal/store/vtext.go` now derives current workspaces as `.texture` /
  `go-choir-texture`, records explicit legacy `.vtext` /
  `go-choir-vtext` derivation, and resolves legacy only when no current
  workspace exists.
- `internal/store/dolt_maintenance.go` now uses the same resolver for Dolt GC.
- `internal/runtime/store_open_test.go` now clones `.texture` test workspaces.
- `nix develop -c go test ./internal/store -run 'TestOpen(UsesTextureWorkspacePathForNewStores|FallsBackToLegacyVTextWorkspace|CreatesDatabase)|TestVTextInitWorkspace' -count=1`
  passed.
- `nix develop -c go test ./internal/runtime -run 'Test.*Store|TestDesktopState' -count=1`
  passed.
- `nix develop -c go test ./internal/store -count=1` passed.
- `nix develop -c scripts/go-test-runtime-shards` passed.
- `scripts/doccheck --report /tmp/choir-doccheck-c34a-workspace.md --json /tmp/choir-doccheck-c34a-workspace.json`
  passed report-only with 212 docs and 1117 warnings.

Open edge: C34a still needs commit/push, CI/deploy identity, and deployed
acceptance proof. It intentionally leaves `database=vtext`, `vtext_*` tables,
durable `vtext:<doc_id>` actor ids, `AgentProfileVText`, stored
`/pub/vtext/...` rows, Universal Wire `Wire.vtext`, and protocol v0 for later
typed slices.

## 2026-06-16 - Landing Evidence: C34a Texture Workspace Identity

Claim: C34a is deployed-supported for filesystem workspace identity.

Move: push the behavior commit, monitor CI/deploy, verify staging identity, and
run deployed product proof. Expected ΔV: record platform landing evidence for
the C34a subobligation; coarse V remains 2.

Actual ΔV: C34a landed and deployed; coarse V remains 2.

Receipts:

- Commit `8e68553e23330e110eacf7f298f7471e101c7c15`
  (`store: default workspaces to texture identity`) pushed to `origin/main`.
- CI run `27602041868` passed.
- Docs Truth Check run `27602041894` passed.
- FlakeHub publish run `27602041885` passed.
- Deploy job `81605380928` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `8e68553e23330e110eacf7f298f7471e101c7c15`, deployed at
  `2026-06-16T07:41:44Z`.
- Deployed acceptance command
  `CHOIR_AUTH_STATE=/tmp/choir-c34a-workspace-auth.json PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/vtext-markdown-lineage.spec.js -g 'Imported Markdown advances|Imported plain text advances'`
  passed 2 tests in 14.7s.

Open edge: this is not a table/database/actor-id/storage-route migration.
Remaining C34 edges are `database=vtext`, `vtext_*` tables, durable
`vtext:<doc_id>` actor ids, `AgentProfileVText`, stored `/pub/vtext/...` rows,
Universal Wire `Wire.vtext`, and protocol v0. Next move should choose one
typed durable identity edge with compatibility and rollback evidence.

## 2026-06-16 - Problem Checkpoint: C35 Durable Actor/Profile Identity Residue

Claim: the next Texture hard-cutover edge is actor/profile identity, not another
filesystem or UI label. No runtime repair is claimed in this move.

Move: read-only inventory plus Problem Documentation First checkpoint. Expected
ΔV: no repair decrease; convert durable actor/profile residue into a typed
problem record with compatibility and rollback requirements.

Actual ΔV: coarse V remains 2. C35 is documented as the next red-surface
candidate slice.

Receipts:

- The previous invariant path named by the operating contract,
  `docs/vtext-agentic-invariants-2026-06-13.md`, is absent; `rg --files docs |
  rg 'vtext.*invariant|agentic.*invariant|vtext-agentic'` found
  `docs/texture-agentic-invariants-2026-06-13.md`, which was read before this
  checkpoint.
- `rg -n "AgentProfileVText|role=vtext|profile=vtext|requested_app\".*vtext|requested_app.*AgentProfileVText|vtext_agent_revision|vtext:<|agent_id\":\"vtext|agent_id.*vtext:" internal/runtime internal/store internal/types frontend/tests internal/runtime/prompt_defaults -g '!frontend/dist/**' | wc -l`
  found 431 current actor/profile residue hits.
- The same search touched 54 files, including runtime profile/tool code, prompt
  defaults, model policy, workflow verifier, agent revision submission, coagent
  routing, persistence/API tests, and deployed frontend Trace assertions.
- Focused code inventory showed new revision runs still write
  `type="vtext_agent_revision"`, `agent_profile="vtext"`,
  `agent_role="vtext"`, and `agent_id="vtext:<doc_id>"`; coagent handoffs and
  verifier contracts also match `vtext:<doc_id>`.
- Added `2026-06-16 - C35 Problem Checkpoint: Durable Actor/Profile Identity
  Residue` to the paradoc with conjecture delta, protected surfaces,
  admissible evidence, rollback path, and heresy delta.

Open edge: no behavior change yet. The first admissible C35 behavior slice
should centralize Texture actor/profile compatibility, keep old `vtext` runs and
deliveries readable, and make one current write path emit `texture` identity
with focused old-read/new-write proof. Do not fold `vtext_agent_revision` task
type or model-policy key migration into that slice unless tests prove they must
move together.

## 2026-06-16 - Local Repair: C35 Texture Actor/Profile Identity

Claim: the first actor/profile identity slice is locally supported for new
Texture writes and legacy delivery reads. It is not deployed-supported yet.

Move: construct centralized Texture actor/profile compatibility helpers, update
new/current Texture appagent write paths to emit `texture` identity, and adjust
old-read delivery/verifier/model-policy boundaries. Expected ΔV: repair the
first C35 behavior slice locally; coarse V remains 2 until CI/deploy/staging
proof.

Actual ΔV: first C35 actor/profile write slice repaired locally. Coarse V
remains 2 pending commit/push, CI/deploy, and staging proof.

Receipts:

- Added `AgentProfileTexture`, current `texture:<doc_id>` helpers, legacy
  `vtext:<doc_id>` helpers, dual-match parsing, and `texture` -> internal
  VText/model-policy compatibility.
- Updated conductor -> Texture initial appagent rows, explicit decision actor
  ids, submitted Texture agent revision run metadata, processor/reconciler
  handoff appagent rows/results, and `request_super_execution` requester roles
  to use current Texture identity.
- Kept old-read compatibility in researcher delivery fallbacks, worker-update
  wake/checkpoint/mark-delivered paths, test worker-update API targeting,
  resident-loop reconciliation, and workflow verifier update routing.
- Focused test command
  `nix develop -c go test ./internal/runtime -run 'TestTextureActorIdentityCompatibility|TestTextureModelPolicyRoleUsesLegacySelectionKey|TestConductorVTextRouteRecordsExplicitDecisionFromStoredPrompt|TestPromptBar|TestProcessor.*VText|TestProcessorMixedPerItemDecisionsCompleteRequestOnceStoryRouteExists|TestHandleInternalRunSubmissionAdmitsProcessorAfterStoryRouteRequestResolutionCompletes' -count=1`
  passed.
- Wider focused command
  `nix develop -c go test ./internal/runtime -run 'TestConductorSpawnAgentCreatesVTextDocumentAndRevisionRun|TestProcessorAndReconcilerProfilesDelegateToVTextOnly|TestResearcherUpdateCoagent|Test.*UpdateCoagent.*VText|Test.*WorkerUpdate|Test.*VTextWorkflow|Test.*VText.*Worker|Test.*VText.*Coagent|Test.*VText.*Revision' -count=1`
  passed.
- Initial `nix develop -c scripts/go-test-runtime-shards` passed shards 0/4
  and 1/4, then shard 2/4 exposed one stale raw-profile expectation. After
  changing `request_super_execution` requester role to `texture`, the exact
  reproducer
  `nix develop -c go test ./internal/runtime -run '^TestHandlePromptBarExplicitSuperExecutionStartsWithVTextThenRequestsSuper$' -count=1 -v`
  passed.
- Post-fix shard commands
  `nix develop -c env SHARD_INDEX=2 TOTAL_SHARDS=4 scripts/go-test-runtime-shards`
  and
  `nix develop -c env SHARD_INDEX=3 TOTAL_SHARDS=4 scripts/go-test-runtime-shards`
  passed.
- The clean post-fix `nix develop -c scripts/go-test-runtime-shards` run
  passed all four runtime shards.
- `scripts/doccheck --report /tmp/choir-doccheck-c35-actor-profile-final.md
  --json /tmp/choir-doccheck-c35-actor-profile-final.json` passed
  report-only after the final evidence update with 212 docs and 1,118
  warnings.
- Scoped production search
  `rg -n "\"vtext:\" \+|strings\.HasPrefix\([^\n]*\"vtext:\"|AgentID:\s+\"vtext:\"|agent_id\":\s*\"vtext:\"" internal/runtime -g '!**/*_test.go'`
  returned no hits.

Open edge: commit/push/CI/deploy/staging proof. This slice intentionally leaves
`vtext_agent_revision`, prompt/tool `role=vtext` affordances, frontend Trace
assertions, model-policy key naming, database/table symbols, and stored legacy
route rows for separate documented repair slices.

## 2026-06-16 - Landing Evidence: C35 Texture Actor/Profile Identity

Claim: C35 is deployed-supported for the first actor/profile identity slice:
new/current Texture actor writes are current-name while legacy delivery reads
remain compatible.

Move: push the behavior commit, monitor CI/deploy, verify staging identity, and
run deployed product proof. Expected ΔV: record platform landing evidence for
C35; coarse V remains 2 because broader storage/route/protocol residue remains.

Actual ΔV: C35 landed and deployed; coarse V remains 2.

Receipts:

- Commit `32b7d98a4e096e9d0399afc841f46de2981e80cb`
  (`runtime: write texture actor identity`) pushed to `origin/main`.
- CI run `27604293193` passed.
- Docs Truth Check run `27604293140` passed.
- FlakeHub publish run `27604293345` passed.
- Deploy job `81612751708` passed.
- `https://choir.news/health` reported deployed commit
  `32b7d98a4e096e9d0399afc841f46de2981e80cb`, deployed at
  `2026-06-16T08:24:29Z`.
- Targeted staging Playwright/API proof recorded
  `/tmp/choir-c35-actor-identity.json`,
  `/tmp/choir-c35-actor-identity-poll.json`, and screenshot
  `/tmp/choir-c35-actor-identity.png`.
- Prompt-bar submission `b0265135-6544-4ae3-9c97-8a3207fd5daa` created Texture
  document `02d689f0-1e7f-457f-928c-3ffd08065147`; Trace showed conductor then
  `texture:02d689f0-1e7f-457f-928c-3ffd08065147` with `profile="texture"` and
  `role="texture"`, no legacy `vtext` actor, no super-before-Texture route, and
  final trace state `completed`.
- The deployed document had user revision
  `269fed4f-c099-462e-89bf-675ac1dc4612` and appagent revision
  `18a07fc2-996e-439d-9f8a-73fa7a8018bc`.
- Staging regression command
  `CHOIR_AUTH_STATE=/tmp/choir-c35-lineage-auth.json PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/vtext-markdown-lineage.spec.js -g 'Imported Markdown advances|Imported plain text advances'`
  passed 2 tests.

Open edge: the deployed prompt submission decision still reports
`app: "vtext"` even though the run/Trace actor is Texture. This is discovered
C35 residue and requires its own Problem Documentation First checkpoint before a
behavior fix. Remaining non-C35 edges include `vtext_agent_revision`,
prompt/tool `role=vtext` affordances, frontend Trace assertions, model-policy
key naming, database/table symbols, stored legacy route rows, deployed
Universal Wire story-field proof, and protocol v0.

## 2026-06-16 - Problem Checkpoint: Prompt Decision App Payload Residue

Claim: the next small repair target is browser-public prompt decision app
payload naming, not durable actor identity. No runtime repair is claimed in this
move.

Move: document the staging-discovered `decision.app: "vtext"` residue and
source inventory before a behavior fix. Expected ΔV: no repair decrease;
convert a discovered residual heresy into a typed problem with compatibility and
rollback requirements.

Actual ΔV: coarse V remains 2. The prompt decision payload slice is documented
as the next candidate repair.

Receipts:

- `/tmp/choir-c35-actor-identity.json` recorded deployed prompt-bar submission
  `b0265135-6544-4ae3-9c97-8a3207fd5daa` returning `decision.app: "vtext"`
  while Trace used `texture:02d689f0-1e7f-457f-928c-3ffd08065147` with
  `profile="texture"` and `role="texture"`.
- `/tmp/choir-c35-actor-identity-poll.json` recorded the same trajectory
  completing with an appagent revision and no legacy Trace actor.
- Source inventory found `conductorRequestedApp`, conductor decision
  normalization, stored prompt recovery, provider fallback decisions, workflow
  verifier checks, runtime tests, and deployed/frontend specs still expecting
  the old prompt decision app id.

Open edge: implement the small payload slice only: new/current prompt decisions
return/store `texture`; legacy `vtext` decisions remain accepted; do not fold
task type, tool profile, model-policy, table, or route-row migration into this
slice.

## 2026-06-16 - Local Repair: C36 Prompt Decision App Payload

Claim: current prompt-bar Texture decisions should report the current app id in
browser-public status while accepting legacy `vtext` decisions during the
compatibility window.

Move: update prompt-bar defaults, immediate conductor decisions, provider
fallback decisions, conductor decision normalization, workflow verification, and
frontend deployed assertions so new/current decisions return/store `texture`
and legacy `vtext` routes remain readable. Expected ΔV: local repair of one
small C35 residue; coarse V remains 2 until deployed proof and broader storage,
task, model-policy, route-row, and protocol residues are settled.

Actual ΔV: locally repaired only. C36 is pending commit, push, CI, deploy,
staging identity, and deployed prompt-bar product proof.

Receipts:

- `nix develop -c go test ./internal/runtime -run 'TestTextureActorIdentityCompatibility|TestTextureModelPolicyRoleUsesLegacySelectionKey|TestPromptBar|TestConductorVTextRouteRecordsExplicitDecisionFromStoredPrompt|Test.*VTextWorkflow' -count=1`
  passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestHandlePromptBarCreatesServerOwnedConductorRun|TestConductorTaskNormalizesStructuredRouteResult|TestConductorDecisionNormalizesToastAfterMaterializedVTextRoute|TestConductorPromptBarStructuredDecisionMaterializesVTextRoute|TestConductorPromptBarVTextRouteFallsBackToSeedPromptContent|TestHandleRunSubmissionPreservesMetadata' -count=1`
  passed.
- Runtime shards 0/4, 1/4, 2/4, and 3/4 passed via
  `nix develop -c env SHARD_INDEX=<n> TOTAL_SHARDS=4 scripts/go-test-runtime-shards`.
- `npm --prefix frontend run build` passed with existing Svelte unused
  export/selector and chunk-size warnings.
- `git diff --check` passed.

Open edge: commit and push the repair, monitor CI/deploy, verify staging build
identity, and run deployed proof that `/api/prompt-bar/submissions/{id}`
returns `decision.app:"texture"` while Trace still shows conductor before the
Texture actor and no legacy `vtext:<doc_id>` actor.

## 2026-06-16 - Landing Evidence: C36 Prompt Decision App Payload

Claim: C36 is deployed-supported for browser-public prompt decision app payload
identity.

Move: push the C36 behavior commit, monitor CI/deploy, verify staging identity,
and run a focused deployed prompt-bar/Trace proof. Expected ΔV: repair the
prompt decision payload residue without changing the coarse V=2 remaining
storage/task/model-policy/route/protocol obligations.

Actual ΔV: C36 landed and deployed; coarse V remains 2.

Receipts:

- Commit `7a9042323a676879afe93f1e6ed226eb3f74e82b`
  (`runtime: return texture prompt decisions`) pushed to `origin/main`.
- CI run `27605982668` passed.
- Docs Truth Check run `27605982675` passed.
- FlakeHub publish run `27605982682` passed.
- Deploy job `81618326388` passed.
- `https://choir.news/health` reported proxy and sandbox commit
  `7a9042323a676879afe93f1e6ed226eb3f74e82b`, deployed at
  `2026-06-16T08:54:47Z`.
- Targeted deployed Playwright/API proof recorded
  `/tmp/choir-c36-prompt-decision.json`,
  `/tmp/choir-c36-prompt-decision-poll.json`, and screenshot
  `/tmp/choir-c36-prompt-decision.png`.
- Prompt submission `f6de90dc-c21b-4531-8e5b-ef594a237713` completed with
  `decision.app: "texture"` for Texture document
  `80f1dd5b-0571-4bc6-bc92-675aa29e062f`.
- Trace showed conductor before
  `texture:80f1dd5b-0571-4bc6-bc92-675aa29e062f`, profile/role `texture`,
  no `vtext:<doc_id>` actor, and no `profile="vtext"` / `role="vtext"` actor.
- The deployed document had user instruction revision
  `f2d4b27a-fbce-4dea-9c46-46488b699aa7` and appagent revision
  `8599c1cf-e04f-40f2-92d0-0755e09db3f0` with metadata source
  `patch_texture`.

Open edge: task type, tool profile wording, model-policy key naming,
database/table symbols, content import app hints, stored legacy route rows,
Universal Wire edition refs, deployed Universal Wire story-field proof, and
protocol v0 remain outside C36.

## 2026-06-16 - Problem Checkpoint: Content App-Hint Payload Residue

Claim: the next repair target is current text-like content app-hint payload
naming, not task type, storage, actor identity, or route-row migration. No
runtime repair is claimed in this move.

Move: document the source-discovered `app_hint:"vtext"` residue before a
behavior fix. Expected ΔV: no repair decrease; convert another V=2 residual
heresy into a typed problem with compatibility and rollback requirements.

Actual ΔV: coarse V remains 2. The content app-hint payload slice is documented
as the next candidate repair.

Receipts:

- `internal/runtime/content.go` maps DOCX, Markdown, and plain text through
  `appHintForMedia` to `vtext`; prompt-bar bare URL routing copies that value
  into `requested_app`, `content_app_hint`, `decision.app`, and
  `decision.app_hint`.
- `internal/runtime/content_extract.go` emits DOCX extraction
  `AppHint: "vtext"`.
- `internal/runtime/vtext_lineage.go` emits Markdown lineage snapshot content
  items with `AppHint: "vtext"`.
- `internal/runtime/content.go` emits YouTube derived transcript content items
  with `AppHint: "vtext"`.
- `internal/runtime/vtext_test.go` and
  `frontend/tests/vtext-markdown-lineage.spec.js` still assert or create
  current text-like content items with old app hints.
- `normalizeAppHint` already accepts both `texture` and `vtext`, so old stored
  content remains readable during the repair.

Open edge: implement only the app-hint payload slice: new/current text-like
content projections emit `texture`; legacy `vtext` hints remain accepted; do
not fold task type, tool profile wording, model-policy keys, table/database
symbols, durable actor ids, or stored route-row migration into this slice.

## 2026-06-16 - Local Repair: C37 Content App-Hint Payload

Claim: C37 is locally supported for current text-like content app-hint payload
identity, but not yet deployed-supported.

Move: construct the documented content app-hint payload repair. Expected ΔV:
repair the app-hint residue locally while leaving coarse V=2 until commit,
CI/deploy identity, and staging product proof land.

Actual ΔV: local repair evidence exists; coarse V remains 2.

Receipts:

- DOCX, Markdown, plain text, Markdown lineage snapshot, and derived transcript
  current emissions now use `AgentProfileTexture`.
- Runtime/frontend tests now assert or create current text-like content items
  with `app_hint:"texture"`.
- Scoped current-emission search
  `rg -n 'app_hint.*vtext|AppHint.*vtext|AppHint:\s+"vtext"|return "vtext"|appHint: "vtext"|app_hint: '\''vtext'\''' internal/runtime frontend/tests frontend/src -g '!frontend/dist/**'`
  returned no hits.
- Focused runtime packet
  `nix develop -c go test ./internal/runtime -run 'TestVTextOpenFileResolvesCanonicalAlias|TestVTextImportMarkdownLineageCreatesRevisionHistory|TestVTextImportMarkdownLineageUsesExistingContentItems|TestVTextOpenFilePreservesDocxAndPDFOriginalArtifacts|TestResearcherReadContentItemReturnsPrivateSourceArtifact|TestImportYouTubeURLContent|TestHandlePromptBar|TestConductorTaskNormalizesStructuredRouteResult' -count=1`
  passed.
- Content/extraction packet
  `nix develop -c go test ./internal/runtime -run 'TestContent|TestExtract|TestFetchYouTubeTranscript' -count=1`
  passed.
- Sequential runtime shard suite
  `nix develop -c scripts/go-test-runtime-shards` passed after Go cache cleanup.
  An accidental parallel shard attempt failed at link time with
  `no space left on device` and was discarded as non-evidence.
- Fresh combined focused runtime packet
  `nix develop -c go test ./internal/runtime -run 'TestVTextOpenFileResolvesCanonicalAlias|TestVTextImportMarkdownLineageCreatesRevisionHistory|TestVTextImportMarkdownLineageUsesExistingContentItems|TestVTextOpenFilePreservesDocxAndPDFOriginalArtifacts|TestResearcherReadContentItemReturnsPrivateSourceArtifact|TestImportYouTubeURLContent|TestHandlePromptBar|TestConductorTaskNormalizesStructuredRouteResult|TestContent|TestExtract|TestFetchYouTubeTranscript' -count=1`
  passed.
- `npm --prefix frontend run build` passed with the pre-existing Universal Wire
  warnings for unused `currentUser` and `.wire-state` selectors.
- `git diff --check` passed.
- `scripts/doccheck --report /tmp/choir-doccheck-c37-content-app-hint-local.md --json /tmp/choir-doccheck-c37-content-app-hint-local.json`
  passed in report-only mode: 212 docs, 1117 warnings.

Open edge: commit/push, monitor CI/deploy, verify staging identity, and run
deployed content app-hint product proof. Task type, tool profile wording,
model-policy keys, table/database symbols, durable actor ids, stored route rows,
Universal Wire edition refs, and protocol v0 remain outside C37.

## 2026-06-16 - Deployed Repair: C37 Content App-Hint Payload

Claim: C37 is deployed-supported for current text-like content app-hint payload
identity.

Move: monitor CI/deploy, verify staging commit identity, and run deployed
browser/API product proof. Expected ΔV: deploy-support the app-hint payload
repair without changing coarse V=2.

Actual ΔV: C37 deployed-supported; coarse V remains 2.

Receipts:

- Commit `79768c1c13bfe5d83039ee7d50df90cab37b2218`
  (`runtime: emit texture content app hints`) pushed to `origin/main`.
- CI run `27607329387` passed.
- Docs Truth Check run `27607329663` passed.
- FlakeHub publish run `27607329354` passed.
- Deploy job `81622826865` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `79768c1c13bfe5d83039ee7d50df90cab37b2218`, deployed at
  `2026-06-16T09:18:43Z`.
- Deployed browser/API proof recorded
  `/tmp/choir-c37-content-app-hint-1781601738637.json` and screenshot
  `/tmp/choir-c37-content-app-hint-1781601738637.png`.
- The proof registered
  `texture-content-hint-proof-1781601738637@example.com`, opened Markdown
  source path `proofs/content-app-hint-1781601738637.md` through
  `/api/texture/files/open`, and created Texture document
  `46a95437-6c31-4a1a-bc92-65439cd4359d`.
- Original content item `a3ee73bf-d31a-46df-9c8c-31746000b4aa` returned
  `media_type:"text/markdown"` and `app_hint:"texture"`.
- A public authenticated `/api/content/items` create for `text/plain` returned
  `app_hint:"texture"`.
- Prompt-bar submission `542fa507-8676-402d-ae50-399be0c619e8` for
  `https://example.com/content-app-hint-1781601738637.md` completed with
  `decision.app:"texture"`, `decision.app_hint:"texture"`,
  `decision.media_type:"text/markdown"`, and the same source URL.
- Browser navigation to the Texture document observed one
  `data-window-app-id="texture"` window and zero legacy `vtext` windows.
- The proof observed zero forbidden product-path requests to `/internal/*`,
  `/api/agent/*`, `/api/test/*`, `/api/prompts`, or `/api/events`.

Open edge: next pass should document the task/profile/model-policy payload
residue before behavior changes. Table/database symbols, durable actor ids,
stored route rows, Universal Wire edition refs, deployed Universal Wire
story-field proof, and protocol v0 remain outside C37.

## 2026-06-16 - Problem Checkpoint: Task/Profile/Model-Policy Payload Residue

Claim: after C35-C37, task/profile/model-policy payload residue is the next
bounded repair target. No runtime repair is claimed in this move.

Move: read-only inventory and Problem Documentation First checkpoint. Expected
ΔV: no repair decrease; convert the next payload residue into a typed problem
with compatibility, rollback, and proof requirements.

Actual ΔV: coarse V remains 2. The next behavior slice is scoped to
`vtext_agent_revision`, prompt/tool role wording, and model-policy key naming,
excluding table/database, durable stored actor-id, stored route-row, Universal
Wire edition, and protocol work.

Receipts:

- `rg -n "vtext_agent_revision" internal/runtime internal/wirepublish frontend/tests internal/types -g '!frontend/dist/**' | wc -l`
  found 57 current task-type hits.
- Current task-type source hits include
  `internal/runtime/vtext_agent_revision.go`, `internal/runtime/runtime.go`,
  `internal/runtime/tools_vtext.go`, `internal/runtime/tool_profiles.go`,
  `internal/runtime/runtime_persistence.go`,
  `internal/runtime/vtext_workflow_verifier.go`,
  `internal/runtime/universal_wire.go`, and
  `internal/wirepublish/eligibility.go`.
- `rg -n "AgentProfileVText|\brole=vtext\b|\bprofile=vtext\b|\"role\"\s*:\s*\"vtext\"|agent_profile\"\s*:\s*\"vtext\"|agent_role\"\s*:\s*\"vtext\"|requested_app\".*vtext|requested_app.*AgentProfileVText" internal/runtime frontend/tests internal/types -g '!frontend/dist/**' | wc -l`
  found 325 scoped profile/role/requested-app hits.
- `internal/runtime/tool_profiles.go` defines `AgentProfileVText = "vtext"`,
  infers `vtext_agent_revision` as `AgentProfileVText`, defaults some requested
  apps to `AgentProfileVText`, and still tells conductor to prefer
  `spawn_agent with role=vtext`.
- Current prompt defaults in `internal/runtime/prompt_defaults/processor.md`,
  `reconciler.md`, `super.md`, `co-super.md`, and `core.md` still teach VText /
  `role=vtext` wording for current model-visible instructions.
- Model-policy residue centers on `[roles.vtext]` and `AgentProfileVText` in
  `internal/runtime/model_policy.go`, `internal/runtime/model_policy_test.go`,
  `internal/runtime/texture_identity_test.go`, and
  `docs/mission-runtime-model-context-substrate-v0.md`.

Open edge: implement current Texture task/profile/model-policy names with
legacy read/fallback compatibility, then prove with focused runtime/model-policy
tests, runtime shards, CI/deploy identity, and deployed prompt-bar/Trace proof.

## 2026-06-16 - Local Repair: C38 Task/Profile/Model-Policy Payloads

Claim: C38 locally repairs current task/profile/model-policy payload naming
without removing legacy compatibility.

Move: implement current Texture task-type emission, current visible
`role=texture` tool/prompt affordances, current generated `[roles.texture]`
model-policy defaults, and current wire eligibility while preserving legacy
`vtext_agent_revision`, `role=vtext`, and `[roles.vtext]` read/fallback paths.
Expected ΔV: move the next payload residue from documented problem to
local-supported repair, with deployed support still open.

Actual ΔV: C38 is local-supported; coarse V remains 2 until commit/CI/deploy
and deployed prompt-bar/Trace proof complete.

Receipts:

- Added centralized task-type compatibility around current
  `texture_agent_revision` and legacy `vtext_agent_revision`.
- New/current revision-run metadata, synthetic Universal Wire normalization
  records, runtime task-type inference, Texture write-tool authorization,
  persistence mutation selection, workflow verification, and wire publication
  eligibility now recognize current Texture revision task records.
- Conductor, processor, and reconciler current spawn affordances now present
  `role=texture`; tests prove current Texture handoffs return
  `texture:<doc_id>`, `profile:"texture"`, `role:"texture"`, and
  `type:"texture_agent_revision"`.
- A conductor repeat-spawn fixture still uses legacy `role:"vtext"` and dedupes
  to the already-materialized Texture route, proving legacy role input remains
  readable during the migration.
- Generated model-policy defaults now emit `[roles.texture]`; tests prove the
  generated policy omits current `[roles.vtext]` while legacy `[roles.vtext]`
  policies still resolve for Texture selection.
- Focused runtime packet
  `nix develop -c go test ./internal/runtime -run 'TestTextureActorIdentityCompatibility|TestTextureAgentRevisionTaskTypeCompatibility|TestTextureModelPolicyRoleUsesLegacySelectionKey|TestGeneratedModelPolicyUsesTextureRoleKey|TestAgentToolProfiles|TestConductorCanSpawnTextureAndTextureCanSpawnResearcher|TestWireProcessorCanSpawnVTextArticleRevision|TestVTextAgentRevisionRealLLMMetadata' -count=1`
  passed.
- `nix develop -c go test ./internal/wirepublish -count=1` passed.
- Widened touched-package packet
  `nix develop -c go test ./internal/runtime ./internal/wirepublish -run 'Test.*Texture|Test.*VText|Test.*ModelPolicy|Test.*Prompt|Test.*Agent|Test.*Wire|Test.*Eligibility' -count=1`
  passed.
- `nix develop -c scripts/go-test-runtime-shards` passed all four sequential
  runtime shards.
- `git diff --check` passed.
- Scoped current-emission search
  `rg -n '"vtext_agent_revision"|role=vtext|\[roles\.vtext\]|spawn_agent with role=vtext|VText owns|VText requests|VText agent' internal/runtime internal/wirepublish -g '!**/*_test.go'`
  now returns only explicit compatibility/fallback anchors:
  `legacyVTextAgentRevisionTaskType = "vtext_agent_revision"` in runtime and
  wirepublish, plus the legacy generated model-policy fallback
  `[roles.vtext]`.

Open edge: commit/push, monitor CI and deploy, verify staging identity, and run
deployed prompt-bar -> conductor -> Texture product proof using public
product/Trace evidence only. Table/database symbols, durable stored actor ids,
stored route rows, Universal Wire edition refs, deployed Universal Wire
story-field proof, and protocol v0 remain outside C38.

## 2026-06-16 - Deployed Repair: C38 Task/Profile/Model-Policy Payloads

Claim: C38 is deployed-supported for current task/profile/model-policy payload
identity.

Move: push the C38 behavior/docs repair, monitor CI/deploy, verify staging
commit identity, run deployed prompt-bar -> conductor -> Texture product proof,
and synthesize a durable run acceptance record from the prompt trajectory.
Expected ΔV: deploy-support this payload slice without changing coarse V=2.

Actual ΔV: C38 deployed-supported; coarse V remains 2 because table/database
symbols, stored legacy routes, Trace event-kind residue, Universal Wire edition
refs, deployed Universal Wire story-field proof, and protocol v0 remain.

Receipts:

- Commit `1a75be52d3f143b26b4cabec215f3a195d51d0dc`
  (`runtime: emit texture task profile payloads`) pushed to `origin/main`.
- CI run `27609193827` passed, including Go vet/build, non-runtime tests,
  integration-tagged smoke, TLA+ model check, Docs Truth Check, all four
  internal/runtime shards, and deploy-impact detection.
- Deploy job `81629249726` passed.
- Separate Docs Truth Check run `27609193826` passed.
- FlakeHub publish run `27609193838` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `1a75be52d3f143b26b4cabec215f3a195d51d0dc`, deployed at
  `2026-06-16T09:53:00Z`.
- Deployed browser/API proof recorded
  `/tmp/choir-c38-task-profile-proof-1781603776347.json` and screenshot
  `/tmp/choir-c38-task-profile-proof-1781603776347.png`.
- The proof registered
  `playwright-state-1781603776987-d8ddg7@example.com`, submitted prompt-bar
  submission `cdab307f-5edb-4d9f-b8ee-85bb6ee551c6`, and created Texture doc
  `30ad9f55-8f35-4168-bb46-882d6c370028`.
- The prompt-bar submission completed with `decision.action:"open_app"`,
  `decision.app:"texture"`, `decision.doc_id:"30ad9f55-8f35-4168-bb46-882d6c370028"`,
  and initial loop `215bbf71-4627-4d99-98bc-eb08fc921d49`.
- Browser DOM proof observed one `data-window-app-id="texture"` window, zero
  legacy `vtext` windows, and a visible Texture editor.
- Public Trace trajectory
  `cdab307f-5edb-4d9f-b8ee-85bb6ee551c6` showed a conductor agent followed by
  `texture:30ad9f55-8f35-4168-bb46-882d6c370028` with
  `profile:"texture"` and `role:"texture"`, edge
  `conductor:* -> texture:*`, `agent_count:2`, `delegation_count:1`, and no
  super-before-Texture route (`firstSuper=-1`, `firstTexture=717` in the saved
  proof text scan).
- The proof observed zero forbidden requests to `/internal/*`, `/api/agent/*`,
  `/api/test/*`, `/api/prompts`, or `/api/events`.
- Run acceptance synthesis via public
  `/api/run-acceptances/synthesize` created
  `runacc-35ec0ac8e8596bbc8416` for trajectory
  `cdab307f-5edb-4d9f-b8ee-85bb6ee551c6`, with
  `acceptance_level:"staging-smoke-level"`, `state:"blocked"`, and passed
  checkpoints `submitted` and `vtext_opened`. The blocked state is expected for
  this slice because the evidence does not claim worker/package/promotion
  acceptance.

Open edge: Trace event kinds still include historical `vtext.*` names, and
table/database symbols, durable stored actor ids, stored route rows, Universal
Wire edition refs, deployed Universal Wire story-field proof, and protocol v0
remain outside C38.

## 2026-06-16 - Problem Checkpoint: Trace Evidence Naming Residue

Claim: after C38, Trace and run-acceptance evidence naming is the next bounded
Texture cutover target. No runtime repair is claimed in this move.

Move: read-only inventory and Problem Documentation First checkpoint. Expected
ΔV: no repair decrease; convert the next evidence-surface residue into a typed
problem with compatibility, rollback, and proof requirements.

Actual ΔV: coarse V remains 2. The next behavior slice is scoped to current
Trace event kinds/summaries and run-acceptance checkpoint/evidence wording,
excluding storage/table symbols, durable stored actor ids, stored route rows,
Universal Wire edition refs, deployed Universal Wire story-field proof, and
protocol v0.

Receipts:

- `rg -n "EventVText|vtext\\.agent_revision|vtext\\.document_revision|vtext\\.decision|vtext_opened|conductor decision opened vtext|vtext document revision exists|prompt/VText|VText/super|VText opened" internal/types internal/runtime -g '!**/*_test.go' | wc -l`
  found 40 current non-test hits.
- `rg -n "EventVText|vtext\\.agent_revision|vtext\\.document_revision|vtext\\.decision|vtext_opened|conductor decision opened vtext|vtext document revision exists|prompt/VText|VText/super|VText opened" internal/types/*_test.go internal/runtime/*_test.go frontend/tests -g '!frontend/dist/**' | wc -l`
  found 29 test/frontend-test hits.
- Current event constants in `internal/types/task.go` still emit
  `vtext.agent_revision.started`, `vtext.agent_revision.progress`,
  `vtext.agent_revision.completed`, `vtext.agent_revision.failed`,
  `vtext.document_revision.created`, and `vtext.decision.recorded`.
- Current event emission in `internal/runtime/vtext_agent_revision.go`,
  `internal/runtime/runtime.go`, and `internal/runtime/tools_vtext.go` still
  routes through `EventVText*` constants; helper/function names may remain
  internal compatibility residue, but emitted values should move to Texture.
- Trace summaries/tone in `internal/runtime/api_trace.go` still render
  "vtext revision started", `vtext <phase>`, "vtext revision completed",
  "vtext revision failed", and "vtext decision ..." for current evidence.
- Run acceptance synthesis in `internal/runtime/run_acceptance.go` still writes
  checkpoint kind `vtext_opened`, evidence labels "conductor decision opened
  vtext document" / "vtext document revision exists for trajectory", and
  invariant/verifier text framed as prompt/VText/super.

Compatibility and proof requirements:

- New/current event rows should use Texture event-kind strings and Texture
  trace summaries.
- Existing stored `vtext.*` event rows must remain readable in Trace and must
  still satisfy run-acceptance synthesis.
- New/current run-acceptance records should use a Texture-opened checkpoint or
  equivalent current label while legacy `vtext_opened` records remain readable
  for level/state/invariant derivation.
- Deployed proof should use public product routes only and show a prompt-bar
  trajectory with conductor -> Texture Trace evidence and a run acceptance
  record whose current checkpoint label no longer introduces new V-name
  evidence.

Protected surfaces and rollback:

- Mutation class for the future repair is red because Trace/evidence and
  run-acceptance records are protected surfaces.
- Admissible evidence: focused Trace/run-acceptance tests, runtime shards,
  docs check, CI/deploy identity, staging health, deployed prompt-bar/Trace
  proof, run-acceptance synthesis, and scoped retired-name search.
- Rollback path: revert the single future behavior commit; do not rewrite
  existing event rows or acceptance records.
- Heresy delta: discovered current evidence-surface V-name residue; repair is
  not yet claimed.

Open edge: implement C39 with current Texture event/checkpoint emission plus
legacy read compatibility.

## 2026-06-16 - C39 Local Repair: Texture Trace Evidence Names

Claim: current Trace/run-acceptance evidence can move to Texture naming without
breaking legacy stored `vtext.*` event rows or old `vtext_opened` acceptance
records.

Move: behavior construct and local proof. Expected ΔV: repair the C39 local
implementation obligation, while coarse V remains 2 until CI/deploy/staging
acceptance proves it in production.

Actual ΔV: C39 is locally supported and ready to land; coarse V remains 2.
Current event constants emit `texture.agent_revision.*`,
`texture.document_revision.created`, and `texture.decision.recorded`. Trace
summaries/tone and document stream projection read both current Texture events
and legacy stored V-name events. Run-acceptance synthesis now writes
`texture_opened` and Texture evidence wording while legacy `vtext_opened`
records remain valid for acceptance level and invariant derivation. Active
runtime prompts/errors/verifier strings touched by this slice now say Texture.

Receipts:

- `nix develop -c go test ./internal/types -run 'TestTextureAgentRevisionEventKinds|TestLegacyVTextAgentRevisionEventKindsRemainReadable' -count=1` passed.
- `nix develop -c go test ./internal/types -count=1` passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestRunAcceptance(Synthesize|Legacy)' -count=1` passed.
- `nix develop -c go test ./internal/runtime -run 'Test(HandleTrace|Trace|BuildTrace|RecordVTextDecision|VTextDiagnosis|DefaultVTextPrompt|RecordVTextDecisionToolDescription|ExplicitNoWorkerDecision|InitialVTextDecision|HandlePromptBar|ConductorVText|ConductorDecision|ConductorPromptBar)' -count=1` passed.
- `nix develop -c scripts/go-test-runtime-shards` passed all four sequential runtime shards.
- `git diff --check` passed.
- `scripts/doccheck --report /tmp/choir-doccheck-c39-texture-evidence.md --json /tmp/choir-doccheck-c39-texture-evidence.json` passed report-only with 212 docs and 1115 warnings.
- Scoped non-test C39 residue search recorded `/tmp/choir-c39-nontest-residue.txt` with 31 hits, all explicit legacy compatibility or the accepted legacy decision-note parser branch.
- Scoped test C39 residue search recorded `/tmp/choir-c39-test-residue.txt` with 11 hits, all legacy-read tests.
- Current Texture evidence search recorded `/tmp/choir-c39-current-texture-hits.txt` with 76 hits.

Open edge: commit/push, monitor CI/deploy, verify staging identity, then run a
deployed prompt-bar -> conductor -> Texture proof showing current Trace event
kinds/summaries and synthesized run acceptance `texture_opened` evidence through
public product routes.

## 2026-06-16 - C39 Landing: Deployed Texture Trace Evidence Names

Claim: C39 is deployed-supported for current Trace/run-acceptance evidence
naming. The scope remains event/checkpoint/evidence projection only; storage
symbols, stored route rows, Universal Wire story-field proof, and protocol v0
remain outside this slice.

Move: land, deploy, and run staging product-path proof. Expected ΔV: close the
C39 deployed-support obligation without changing coarse V=2.

Actual ΔV: C39 deployed-supported; coarse V remains 2.

Receipts:

- Behavior commit `bffad4d6013aafb8359e07baeab2d89b3a789a1b`
  (`runtime: emit texture trace evidence names`) pushed to `origin/main`.
- CI run `27610879275` passed, including Go vet/build, non-runtime tests,
  integration-tagged smoke, TLA+ model check, Docs Truth Check, all four
  internal/runtime shards, and staging deploy.
- Deploy job `81634946340` passed.
- Separate Docs Truth Check run `27610878863` passed.
- FlakeHub publish run `27610878890` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `bffad4d6013aafb8359e07baeab2d89b3a789a1b`, deployed at
  `2026-06-16T10:24:05Z`.
- Deployed browser/API proof recorded
  `/tmp/choir-c39-texture-trace-proof-1781605847169.json` and screenshot
  `/tmp/choir-c39-texture-trace-proof-1781605847169.png`.
- The proof registered/reused auth state for
  `playwright-state-1781605508444-kodf7u@example.com`, submitted prompt-bar
  submission `65dc8b95-05e1-4407-85ce-21218aadce3a`, and created Texture doc
  `41e480f4-da5b-4467-bd4e-5cf325960d75`.
- Prompt decision was `action:"open_app"`, `app:"texture"`,
  `doc_id:"41e480f4-da5b-4467-bd4e-5cf325960d75"`, and initial loop
  `d82bb148-e7f9-4301-a951-e5032db01012`.
- Browser DOM proof observed two `data-window-app-id="texture"` windows, zero
  legacy `vtext` windows, and two Texture app surfaces.
- Public Trace trajectory
  `65dc8b95-05e1-4407-85ce-21218aadce3a` showed conductor ->
  `texture:41e480f4-da5b-4467-bd4e-5cf325960d75`, `agent_count:2`,
  `first_texture_index:1`, and `first_super_index:-1`.
- Trace logs exposed current event kinds
  `texture.document_revision.created`, `texture.agent_revision.started`, and
  `texture.agent_revision.progress`, with zero legacy `vtext.*` event kinds and
  no `vtext revision` / `vtext decision` summary wording.
- The proof observed zero forbidden browser-public internal requests to
  `/internal/*`, `/api/agent/*`, `/api/prompts`, `/api/test/*`, or
  `/api/events`.
- Run acceptance synthesis via public
  `/api/run-acceptances/synthesize` created
  `runacc-49da5125339dded1c5b1` with
  `acceptance_level:"staging-smoke-level"`, `state:"blocked"`, and checkpoints
  `submitted` and `texture_opened`. The blocked state is expected for this
  slice because it does not claim worker/package/promotion acceptance.

Open edge: next high-ΔV move is storage/table and durable actor/stored-route
residue, unless the deployed Universal Wire story-field proof becomes cheaper
through product-path story creation. Protocol v0 remains last.

## 2026-06-16 - Problem Checkpoint: Storage And Persistent Route Residue

Claim: after C39, the highest-ΔV remaining Texture cutover problem is protected
persistent-state residue: Dolt/app table names, platform publication storage,
durable actor compatibility, stored public route rows, and Universal Wire
edition/transclusion refs. No runtime repair is claimed in this move.

Move: read-only inventory and Problem Documentation First checkpoint. Expected
ΔV: no repair decrease; convert the storage/persistent-state residue into a
typed problem with compatibility, migration, proof, and rollback requirements.

Actual ΔV: coarse V remains 2. The next behavior slice must be narrower than
"rename all storage": either platform/public route-row migration evidence or a
current Texture schema alias/migration layer for user Texture tables.

Receipts:

- Read `docs/computer-ontology.md` before touching the storage/persistent-state
  surface. The relevant rule is that Dolt/app state, artifact/provenance graph,
  and route identity are separate ledgers; shared changes must become typed
  artifacts, and route switches need rollback evidence.
- `rg -n "CREATE TABLE IF NOT EXISTS (vtext|platform_vtext)|database=vtext|legacyVTextWorkspace|go-choir-vtext|legacyPublicVTextPrefix|/pub/vtext|vtext:|AgentProfileVText|vtext_agent_revision|Wire\\.vtext|universalWireEditionSourcePath|platform_vtext" internal -g '!**/*_test.go' | tee /tmp/choir-c40-storage-nontest-inventory.txt | wc -l`
  found 97 non-test hits.
- The non-test inventory clusters by file included:
  `internal/store/vtext.go` (11), `internal/platform/store.go` (8),
  `internal/runtime/runtime.go` (8), `internal/runtime/tool_profiles.go` (13),
  `internal/runtime/vtext_workflow_verifier.go` (6),
  `internal/runtime/universal_wire.go` (6), `internal/runtime/model_policy.go`
  (5), and `internal/platform/service.go` (4).
- Current protected storage symbols include `vtext_documents`,
  `vtext_revisions`, `vtext_document_aliases`, `vtext_agent_mutations`,
  `vtext_controller_checkpoints`, `vtext_decisions`, `database=vtext`,
  `platform_vtext_documents`, and `platform_vtext_revisions`.
- Current compatibility residues include legacy `.vtext` / `go-choir-vtext`
  workspace fallback, durable `vtext:<doc_id>` actor matching, stored
  `/pub/vtext/...` route-prefix readability, `universal-wire/Wire.vtext`, and
  `vtext:` transclusion refs.
- `rg -n "CREATE TABLE IF NOT EXISTS (vtext|platform_vtext)|database=vtext|legacyVTextWorkspace|go-choir-vtext|legacyPublicVTextPrefix|/pub/vtext|vtext:|AgentProfileVText|vtext_agent_revision|Wire\\.vtext|universalWireEditionSourcePath|platform_vtext" internal/**/*_test.go frontend/tests -g '!frontend/dist/**' | tee /tmp/choir-c40-storage-test-inventory.txt | wc -l`
  found 423 test/frontend-test hits.
- `rg -n "texture-cutover-allow|legacy.*vtext|LegacyVText|legacyVText|legacy_vtext|published_vtext|private_vtext|/pub/vtext|vtext_opened|LegacyEventVText" internal docs frontend -g '!frontend/dist/**' | tee /tmp/choir-c40-allowance-candidates.txt | wc -l`
  found 551 allowance/legacy-candidate hits.

Compatibility and proof requirements:

- Existing user Texture documents, revisions, aliases, decisions, and mutation
  records must remain readable.
- Existing platform publications and stored legacy route rows must remain
  resolvable until an idempotent migration rewrites or explicitly shims them.
- Durable legacy actor ids and stored Trace/run records must remain readable;
  new current actor ids should continue to be Texture.
- Any storage migration must be typed, idempotent, separately testable, and
  reversible by commit rollback plus no destructive data rewrite without an
  explicit migration rollback path.
- Product proof must use public product APIs and staging identity, not manual
  database seeding or browser-public internal routes.

Protected surfaces and rollback:

- Mutation class for the future repair is red: Dolt/app state, platform
  publication state, route identity, durable actor compatibility, and
  Universal Wire edition/transclusion state are protected.
- Admissible evidence: focused store/platform/runtime tests, runtime shards
  when runtime compatibility changes, doccheck, scoped residue search, CI,
  staging deploy identity, and deployed public-route/API proof for the chosen
  slice.
- Rollback path depends on the chosen sub-slice. Source-only alias/shim changes
  can be reverted. Any data migration needs a before/after route/table receipt
  and explicit down/compatibility behavior before it can land.
- Heresy delta: discovered persistent-state V-name residue; repair is not yet
  claimed.

Open edge: select the first bounded C40 repair. Prefer platform/public route-row
or user Texture table alias/migration work over a broad all-at-once storage
rename. Keep Universal Wire story-field proof and protocol v0 separate.

## 2026-06-16 - C40a Platform Texture Sync Boundary

Claim: the first safe storage-adjacent repair is to make the current platform
Texture sync/read boundary Texture-named in code and emitted evidence, while
leaving persisted `platform_vtext_*` table names as explicit compatibility
substrate until a separate typed migration exists.

Move: construct. Expected ΔV: reduce current platform storage-boundary naming
residue without claiming a table or route-row migration.

Actual ΔV: current platform `/internal/platform/texture/*` routes now land in
Texture-named request/response types, handler methods, service/store methods,
proxy async-sync helper, publication metadata enrichment helper, logs/errors,
and Dolt commit messages. Coarse V remains 2 because table/database names,
stored legacy routes, durable actor compatibility, Universal Wire edition refs,
deployed Universal Wire story-field proof, and protocol v0 remain.

Receipts:

- Edited `internal/platform/types.go`, `internal/platform/store.go`,
  `internal/platform/service.go`, `internal/platform/handlers.go`,
  `internal/proxy/wire_platform_publish.go`,
  `internal/proxy/platform_publish.go`, and focused platform tests.
- `nix develop -c go test ./internal/platform` passed.
- `nix develop -c go test ./internal/proxy -run 'TestPlatform|TestWire|TestHandlePlatform|TestPublication'`
  passed.
- `nix develop -c go test ./internal/platform ./internal/proxy` passed.
- `git diff --check` passed.
- `scripts/doccheck --report /tmp/choir-doccheck-c40a-platform-texture-sync.md --json /tmp/choir-doccheck-c40a-platform-texture-sync.json`
  passed with 212 docs and 1112 report-only warnings.
- The scoped C40 non-test storage inventory now finds 95 hits, recorded at
  `/tmp/choir-c40a-storage-nontest-inventory.txt`; C40 found 97 before this
  boundary repair.
- Scoped boundary search for `SyncVText`, `PlatformVText`,
  `GetPlatformVText`, `ListPlatformVText`, `syncVText`, `enrichVText`,
  `sync vtext`, `platform sync vtext`, `propose vtext`, and related failure
  wording found no hits in `internal/platform` or the touched proxy paths; the
  remaining `HandlePlatformVTextRead` proxy surface is separate browser-public
  read compatibility residue.

Protected surfaces and rollback:

- Mutation class: red, because platform artifact storage/read boundaries and
  Dolt commit messages are protected even though this pass does not rewrite
  data.
- Protected surfaces touched: platform Texture sync/read APIs, platform
  publication metadata enrichment, proxy asynchronous platform sync, and
  persisted `platform_vtext_*` row readability.
- Rollback path: source revert only; no data migration was introduced.
- Heresy delta: repaired current platform sync/read boundary naming; left table
  and route-row residue discovered/open.

Open edge: land C40a through CI/deploy/staging identity and product proof. Then
choose the next storage repair: typed table alias/migration layer or
idempotent public-route-row migration/alias proof.

## 2026-06-16 - C40a Landing Proof

Claim: C40a is deployed-supported at the platform Texture sync/read boundary
scope. It is not a table-name, stored-route-row, durable actor, Universal Wire,
or protocol repair.

Move: settle C40a scope with CI, staging deploy identity, and deployed public
product-path proof. Expected ΔV: promote C40a from local support to deployed
support; coarse V remains 2.

Actual ΔV: C40a deployed-supported. Coarse V remains 2.

Receipts:

- Behavior commit `fd57e00c4a854008a8d5a681d80c9ec4b077b8e6`
  (`platform: rename texture sync boundary`) pushed to `origin/main`.
- CI run `27612192131` passed, including Go vet/build, non-runtime tests,
  integration-tagged smoke, Docs Truth Check, TLA+ model check, all four
  runtime shards, and staging deploy.
- Deploy job `81639495038` passed.
- Separate Docs Truth Check run `27612192088` passed.
- FlakeHub publish run `27612191932` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `fd57e00c4a854008a8d5a681d80c9ec4b077b8e6`, deployed at
  `2026-06-16T10:50:04Z`; receipt stored at `/tmp/choir-c40a-health.json`.
- Deployed product proof:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/vtext-source-service-publication.spec.js -g "publishes source-service source entities"`
  passed. The proof created a Texture, published it through
  `/api/platform/texture/publications`, resolved/exported the current
  `/pub/texture/...` publication route, opened the published reader, and
  exercised source/transclusion/publication metadata through browser-public
  product APIs.

Protected surfaces and rollback:

- Mutation class: red; protected surfaces touched are platform Texture
  sync/read APIs, platform publication metadata enrichment, proxy asynchronous
  platform sync, and persisted platform Texture row readability.
- Rollback path: revert commit
  `fd57e00c4a854008a8d5a681d80c9ec4b077b8e6`. No data migration was introduced.
- Heresy delta: repaired current platform boundary naming; table/database
  names, public stored route rows, durable actor compatibility, Universal Wire
  edition refs, and protocol v0 remain open.

Open edge: next storage repair should choose either a typed table
alias/migration layer for user/platform Texture rows or an idempotent
public-route-row migration/alias proof.

## 2026-06-16 - C40b Local Construct: Public Route Alias Migration

Claim: the next bounded storage repair should eliminate the need for stored
legacy `/pub/vtext/...` publication rows as the only route identity for those
publications, without deleting the legacy rows or weakening legacy readability.

Move: probe, then construct. Expected ΔV: repair stored-route-row residue by
creating current Texture aliases with rollback refs; leave table/database and
durable actor residue for later.

Probe result: a direct Dolt SQL probe in `/tmp` showed simple
`texture_documents` views over `vtext_documents` are not writable in the needed
shape (`expected insert destination to be resolved or unresolved table`). That
means the user/platform table alias layer needs dual-write/backfill design
rather than a trivial writable-view shim, so this pass shifted to the
route-row migration.

Actual ΔV: C40b is locally supported, awaiting landing proof. Platform
`Store.Bootstrap` now runs `MigrateLegacyPublicVTextRoutes`, which idempotently
creates `/pub/texture/...` aliases for stored `/pub/vtext/...` publication
routes when the current alias is missing, preserves the legacy route row, and
records rollback refs for generated aliases. Coarse V remains 2 until
CI/deploy/staging proof lands.

Receipts:

- Edited `internal/platform/store.go` and `internal/platform/service.go`.
- Added `TestMigrateLegacyPublicVTextRoutesCreatesTextureAlias` in
  `internal/platform/service_test.go`.
- `nix develop -c go test ./internal/platform -run 'TestMigrateLegacyPublicVTextRoutesCreatesTextureAlias|TestPublishTextureCreatesImmutablePublicRecords|TestPublicationPublicSurfacesEnforceVisibilityPolicy'`
  passed.
- `nix develop -c go test ./internal/platform` passed.
- Scoped route search now shows the remaining `/pub/vtext` mentions in
  `internal/platform` are the explicit migration, compatibility prefix, and
  route normalization shim.
- Scoped C40 non-test storage inventory recorded
  `/tmp/choir-c40b-storage-nontest-inventory.txt` with 98 hits. The count rises
  because this slice adds explicit migration SQL for `/pub/vtext/%`; this is
  classified as migration residue, not current product naming.

Protected surfaces and rollback:

- Mutation class: red, because route identity and platform publication storage
  are protected surfaces.
- Protected surfaces touched: `public_routes`, `rollback_refs`, platform store
  bootstrap, publication route normalization, and public publication
  resolution/export behavior for migrated aliases.
- Rollback path: source revert before deploy; after deploy, generated alias
  rows have rollback refs of kind `disable_route` and legacy rows remain
  readable. No destructive data rewrite was introduced.
- Heresy delta: repaired current alias absence for legacy route rows; table
  names, durable actor compatibility, Universal Wire edition refs, and protocol
  v0 remain open.

Open edge: land C40b through CI/deploy/staging identity and deployed public
route proof. Then return to table/database alias design or durable actor/profile
residue.

## 2026-06-16 - C40b Landing Proof: Public Route Alias Migration

Claim: C40b is deployed-supported for stored public route-row alias migration.
The claim is scoped to existing `/pub/vtext/...` publication route rows gaining
current `/pub/texture/...` aliases without deleting legacy rows. It does not
claim table/database, durable actor, Universal Wire edition-ref, or protocol
repair.

Move: settle C40b scope with CI, staging deploy identity, public product API
proof, and direct browser proof. Expected ΔV: promote C40b from local support to
deployed support; coarse V remains 2.

Actual ΔV: C40b deployed-supported. Coarse V remains 2 because table/database
names, durable actor compatibility, Universal Wire edition refs, deployed
Universal Wire story-field proof, and protocol v0 remain.

Receipts:

- Behavior commit `af6e4e349d50f78059ced803148884ebbcb8017e`
  (`platform: migrate legacy public texture routes`) pushed to `origin/main`.
- CI run `27613190873` passed, including Go vet/build, non-runtime tests,
  integration-tagged smoke, Docs Truth Check, TLA+ model check, all four
  runtime shards, and staging deploy.
- Deploy job `81642797177` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `af6e4e349d50f78059ced803148884ebbcb8017e`, deployed at
  `2026-06-16T11:09:12Z`; receipt stored at `/tmp/choir-c40b-health.json`.
- Public API proof through browser-public product routes showed legacy route
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6` and alias
  `/pub/texture/choir-private-legal-cloud-proposal-vtext-pub270a62fb6` both
  resolve to publication `pub-270a62fb-62e6-4509-9779-c0b9b32d2c71` and version
  `pubver-fe47bb49-edd1-4390-b0e8-454b81833619`; public Markdown export works
  through the Texture alias. Receipts:
  `/tmp/choir-c40b-legacy-resolve.json`,
  `/tmp/choir-c40b-alias-resolve.json`, and
  `/tmp/choir-c40b-alias-export.json`.
- Direct browser proof opened
  `https://choir.news/pub/texture/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`
  with one `data-app-id="texture"` window, zero `data-app-id="vtext"` windows,
  one published-reader surface, visible proposal text, and zero forbidden
  product-path requests to `/internal/*`, `/api/agent/*`, `/api/test/*`,
  `/api/prompts`, or `/api/events`. Receipt:
  `/tmp/choir-c40b-route-alias-proof/evidence.json`; screenshot:
  `/tmp/choir-c40b-route-alias-proof/alias-reader.png`.
- Broader deployed publication proof
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/vtext-source-service-publication.spec.js -g "publishes source-service source entities"`
  passed, covering current `/api/platform/texture/publications` publication,
  source/transclusion metadata, canonical exports, and published reader paths.

Protected surfaces and rollback:

- Mutation class: red; protected surfaces touched are `public_routes`,
  `rollback_refs`, platform store bootstrap, publication route normalization,
  and public publication resolution/export behavior for migrated aliases.
- Rollback path: revert commit
  `af6e4e349d50f78059ced803148884ebbcb8017e`; generated alias rows also carry
  rollback refs of kind `disable_route`, and legacy rows remain readable. No
  destructive data rewrite was introduced.
- Heresy delta: repaired current alias absence for legacy public route rows;
  table/database names, durable actor compatibility, Universal Wire edition
  refs, and protocol v0 remain open.

Open edge: choose the next storage repair. The strongest current candidates are
a typed table alias/migration layer for user/platform Texture rows or a durable
actor/profile residue slice. Universal Wire deployed story-field proof and
protocol v0 stay separate.

## 2026-06-16 - C41 Problem Checkpoint: Texture Database Identity Residue

Claim: the next storage repair should be the user-store Dolt database identity
boundary, not a broad all-storage rename. The narrower conjecture is that new
Texture workspaces can open a current `texture` database while existing
`vtext` databases remain readable; table names, platform table names, durable
actor ids, and Universal Wire refs need separate typed migrations.

Move: probe/read-only inventory and design. Expected ΔV: buy observer evidence
and select the next bounded red-surface slice; no runtime repair claimed.

Actual ΔV: selected C41 as a bounded repair candidate. Coarse V remains 2 until
behavior, CI/deploy, and staging proof land.

Receipts:

- Read `docs/computer-ontology.md`: the relevant split is that Dolt/app state,
  artifact/provenance graph, and route identity are separate ledgers. Shared
  storage changes must become typed artifacts; route identity is not the same
  surface as table/database identity.
- `rg -n "CREATE TABLE IF NOT EXISTS (vtext|platform_vtext)|vtext_documents|vtext_revisions|vtext_document_aliases|vtext_agent_mutations|vtext_controller_checkpoints|vtext_decisions|platform_vtext_documents|platform_vtext_revisions|database=vtext|legacyVTextWorkspace|go-choir-vtext|legacyPublicVTextPrefix|/pub/vtext|vtext:|AgentProfileVText|vtext_agent_revision|Wire\\.vtext|universalWireEditionSourcePath|platform_vtext" internal frontend docs -g '!frontend/dist/**' | tee /tmp/choir-c41-storage-actor-inventory.txt | wc -l`
  found 1,150 hits.
- Scoped non-test internal inventory
  `/tmp/choir-c41-internal-nontest.txt` found 230 hits; the largest clusters
  were `internal/store/vtext.go` (114), `internal/runtime/tool_profiles.go`
  (13), `internal/runtime/vtext_controller.go` (11),
  `internal/runtime/universal_wire.go` (11), `internal/runtime/runtime.go`
  (10), and `internal/platform/store.go` (8).
- Scoped test/frontend-test inventory `/tmp/choir-c41-test-inventory.txt`
  found 465 hits; the largest clusters were
  `internal/runtime/agent_tools_test.go` (103),
  `internal/runtime/vtext_test.go` (101), and
  `internal/runtime/email_appagent_tools_test.go` (32).
- User-store database/table hits include `database=vtext`, `vtext_documents`,
  `vtext_revisions`, `vtext_document_aliases`, `vtext_agent_mutations`,
  `vtext_controller_checkpoints`, and `vtext_decisions`.
- Platform table hits are narrower: `platform_vtext_documents` and
  `platform_vtext_revisions` in `internal/platform/store.go`.
- Actor/profile hits are a separate runtime surface:
  `AgentProfileVText`, `vtext_agent_revision`, legacy `vtext:<doc_id>`
  routing, and `[roles.vtext]` fallback policy.

Next behavior slice:

- create/open `database=texture` for fresh user-store Texture workspaces;
- detect and preserve existing `database=vtext` workspaces when no current
  `texture` database exists;
- leave `vtext_*` table names, platform `platform_vtext_*` tables, durable
  actor/profile ids, Universal Wire `vtext:` refs, and public route
  compatibility unchanged;
- prove with focused store tests for fresh current database identity and legacy
  database readability, then full store package, doccheck, CI/deploy, staging
  identity, and deployed product proof.

Protected surfaces and rollback:

- Future behavior mutation class: red, because this touches user-computer
  Dolt/app state identity and embedded store bootstrap.
- Protected surfaces: user Texture document/revision/alias persistence,
  embedded Dolt workspace opening, legacy workspace/database readability,
  Dolt maintenance connections, and document-store tests.
- Rollback path: source revert restores new workspace opening to
  `database=vtext`; the slice must not drop or rename tables and must keep
  existing `database=vtext` readable, so there is no destructive data rollback.
- Heresy delta: discovered and bounded database-identity residue; no repair
  claimed yet.

Open edge: implement C41 user-store database identity. If the Dolt adapter
cannot reliably detect database existence without opening the database, revise
the slice before construction rather than silently folding in table migration.

## 2026-06-16 - C41 Local Construct: Texture Database Identity

Claim: new user-store Texture workspaces can open a current `texture` Dolt
database while existing `vtext` databases remain readable, without renaming
tables or moving rows.

Move: construct. Expected ΔV: repair the user-store database identity slice;
leave table names, platform tables, durable actor/profile ids, Universal Wire
refs, deployed proof, and protocol v0 open.

Actual ΔV: C41 is locally supported. Fresh workspaces now create/open
`database=texture`; existing workspaces with only `database=vtext` are detected
and opened as legacy; Dolt GC resolves the same current-or-legacy database
before running. Coarse V remains 2 until CI/deploy/staging proof lands and
because other storage/actor/protocol obligations remain.

Receipts:

- Edited `internal/store/vtext.go` to add current/legacy database constants,
  root-connection database detection, fresh `texture` database creation, and
  legacy `vtext` fallback.
- Edited `internal/store/dolt_maintenance.go` so `DOLT_GC()` uses the same
  current-or-legacy database resolver and no-ops when no Texture database
  exists.
- Added `TestOpenVTextWorkspaceUsesTextureDatabaseForFreshWorkspace` and
  `TestOpenVTextWorkspaceReadsLegacyVTextDatabase` in
  `internal/store/vtext_test.go`.
- `nix develop -c go test ./internal/store -run 'TestOpenVTextWorkspaceUsesTextureDatabaseForFreshWorkspace|TestOpenVTextWorkspaceReadsLegacyVTextDatabase|TestVTextCreateDocument|TestUnifiedDoltWorkspace'`
  passed.
- `nix develop -c go test ./internal/store` passed.
- `nix develop -c scripts/go-test-runtime-shards` passed.
- `scripts/doccheck --report /tmp/choir-doccheck-c41-db-behavior.md --json
  /tmp/choir-doccheck-c41-db-behavior.json` passed in report-only mode with
  212 docs and 1,112 warnings.
- `git diff --check` passed.
- `rg -n "database=vtext" internal/store -g '!**/*_test.go'` returned no hits.

Protected surfaces and rollback:

- Mutation class: red, because this changes user-computer Dolt/app state
  identity and embedded store bootstrap.
- Protected surfaces touched: user Texture document/revision/alias
  persistence, embedded Dolt workspace opening, legacy workspace/database
  readability, Dolt maintenance connections, and runtime startup through
  `store.Open`.
- Rollback path: source revert restores fresh workspace opening to
  `database=vtext`. No table rename/drop or row movement was introduced, and
  existing legacy `vtext` databases remain readable before and after rollback.
- Heresy delta: repaired locally for new user-store database identity; table
  names, platform tables, durable actor/profile ids, Universal Wire refs, and
  protocol v0 remain open.

Open edge: land C41 through CI/deploy/staging identity and deployed Texture
product proof. The deployed proof should exercise a real Texture write/read
path rather than only health, because this slice touches runtime store
bootstrap.

## 2026-06-16 - C41 Landing Proof: Texture Database Identity

Claim: C41 is deployed-supported for the user-store Dolt database identity
slice. The claim is scoped to fresh/current workspaces opening
`database=texture`, legacy `database=vtext` workspaces remaining readable, and
Dolt GC selecting the same current-or-legacy database. It does not claim
`vtext_*` table-name migration, platform table migration, durable actor/profile
repair, Universal Wire ref repair, or protocol v0.

Move: settle C41 scope with CI, staging deploy identity, and deployed Texture
product proof. Expected ΔV: promote C41 from local support to deployed support;
coarse V remains 2.

Actual ΔV: C41 deployed-supported. Coarse V remains 2 because table names,
platform table residue, durable actor/profile residue, Universal Wire
story-field proof, and protocol v0 remain.

Receipts:

- Behavior commit `fc166e4fbe1a93122cd6fb57e5c408d3cc864ff3`
  (`store: use texture dolt database for new workspaces`) pushed to
  `origin/main`.
- CI run `27614905254` passed, including Go vet/build, non-runtime tests,
  integration-tagged smoke, Docs Truth Check, TLA+ model check, all four
  runtime shards, and staging deploy.
- Deploy job `81648551589` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `fc166e4fbe1a93122cd6fb57e5c408d3cc864ff3`, deployed at
  `2026-06-16T11:42:32Z`; receipt stored at `/tmp/choir-c41-health.json`.
- Deployed product proof
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/vtext-source-service-publication.spec.js -g "publishes source-service source entities"`
  passed. The proof exercises Texture document creation/write/read,
  publication through `/api/platform/texture/publications`, source/transclusion
  metadata, canonical exports, and published reader paths against staging after
  the database identity cut.

Protected surfaces and rollback:

- Mutation class: red; protected surfaces touched are user Texture
  document/revision/alias persistence, embedded Dolt workspace opening, legacy
  workspace/database readability, Dolt maintenance connections, and runtime
  startup through `store.Open`.
- Rollback path: revert commit
  `fc166e4fbe1a93122cd6fb57e5c408d3cc864ff3`. No table rename/drop or row
  movement was introduced, and existing legacy `vtext` databases remain
  readable before and after rollback.
- Heresy delta: repaired deployed new user-store database identity; table
  names, platform tables, durable actor/profile ids, Universal Wire refs, and
  protocol v0 remain open.

Open edge: choose the next bounded remaining slice: table-name migration design,
platform `platform_vtext_*` table residue, durable actor/profile compatibility,
or deployed Universal Wire story-field proof. Do not start protocol v0 until the
working surface is fully proven.

## 2026-06-16 - C42 Problem Checkpoint: Platform Table Identity Residue

Claim: the next storage repair should be the platform Texture table identity
boundary, not a broad user-store table rename. The narrower conjecture is that
current platform Texture document sync/read can use `platform_texture_*` tables
while legacy `platform_vtext_*` rows are copied forward idempotently and left in
place for rollback/read compatibility.

Move: probe/read-only inventory and design. Expected ΔV: buy observer evidence
and select a bounded red-surface slice; no runtime repair claimed.

Actual ΔV: selected C42 as the next bounded repair candidate. Coarse V remains
2 until behavior, CI/deploy, and staging proof land.

Receipts:

- `internal/platform/store.go` creates only `platform_vtext_documents` and
  `platform_vtext_revisions` for platform Texture document storage.
- `UpsertTextureDocument`, `UpsertTextureRevision`, `GetTextureDocument`,
  `ListTextureRevisions`, and `GetTextureRevision` directly target those legacy
  table names.
- `rg -n "platform_vtext|platform_texture" internal/platform internal/proxy internal/runtime internal/wirepublish -g '!**/*_test.go'`
  found platform table residue only in `internal/platform/store.go`.
- `docs/computer-ontology.md` keeps Dolt/app state separate from route identity
  and artifact/provenance graph state; this slice is table identity only, not
  route-row, actor-id, or transclusion migration.

Next behavior slice:

- create `platform_texture_documents` and `platform_texture_revisions` at
  bootstrap;
- copy legacy `platform_vtext_*` rows into current tables idempotently before
  current reads/writes use current table names;
- keep legacy tables in place and stop current code from writing new platform
  document rows to them;
- prove current writes, legacy-read migration, and non-test current-code
  residue search.

Protected surfaces and rollback:

- Future behavior mutation class: red, because this touches platform Dolt/app
  state for document sync/read behavior.
- Protected surfaces: platform Texture document sync, revision sync/read,
  platform store bootstrap, existing platform Dolt rows, and rollback to older
  binaries that still expect `platform_vtext_*`.
- Rollback path: source revert restores current reads/writes to
  `platform_vtext_*`. The planned migration must not drop or rename legacy
  tables, so rollback can continue reading legacy rows.
- Heresy delta: discovered and bounded platform table-name residue; no repair
  claimed yet.

## 2026-06-16 - C42 Local Construct: Platform Table Identity

Claim: current platform Texture document sync/read can use
`platform_texture_documents` and `platform_texture_revisions` while legacy
`platform_vtext_*` rows remain available and are copied forward idempotently.

Move: construct. Expected ΔV: repair the platform table identity slice locally;
leave user-store table names, durable actor ids, stored route rows, Universal
Wire refs/proof, deployed proof, and protocol v0 open.

Actual ΔV: C42 is locally supported. Current platform store bootstrap creates
`platform_texture_*`, copies legacy `platform_vtext_*` rows into current tables
with `INSERT IGNORE`, and current reads/writes use the current table names.
Coarse V remains 2 until CI/deploy/staging proof lands and because other
storage/actor/Wire/protocol obligations remain.

Receipts:

- Edited `internal/platform/store.go` to add current platform Texture tables,
  add `MigrateLegacyPlatformVTextTables`, call it during bootstrap before route
  alias migration, and move Texture store methods to `platform_texture_*`.
- Added `TestPlatformTextureStoreWritesCurrentTables` and
  `TestPlatformTextureStoreMigratesLegacyTablesAtBootstrap` in
  `internal/platform/service_test.go`.
- `nix develop -c go test ./internal/platform -run 'TestPlatformTextureStoreWritesCurrentTables|TestPlatformTextureStoreMigratesLegacyTablesAtBootstrap|TestSyncTextureDocument|TestGetTextureDocument|TestInternalPublishRequiresInternalCallerAndBundleResolve'`
  passed.
- `nix develop -c go test ./internal/platform` passed.
- `nix develop -c go test ./internal/proxy -run 'TestPlatformPublicationResolveIsPublicAndInternalOnly|TestPlatformPublicationResolveAndExportPropagateNotFound|TestHandleVTextPublication|TestPlatformTexture'`
  passed.
- `nix develop -c go test ./internal/platform ./internal/proxy` passed.
- `scripts/doccheck --report /tmp/choir-doccheck-c42-platform-table-behavior.md --json /tmp/choir-doccheck-c42-platform-table-behavior.json`
  passed in report-only mode with 212 docs and 1,112 warnings.
- `git diff --check` passed.
- `rg -n "platform_vtext_documents|platform_vtext_revisions" internal/platform internal/proxy internal/runtime internal/wirepublish -g '!**/*_test.go'`
  now finds only retained legacy schema and migration copy-forward reads in
  `internal/platform/store.go`.

Protected surfaces and rollback:

- Mutation class: red, because this changes platform Dolt/app state identity
  for Texture document sync/read behavior.
- Protected surfaces touched: platform Texture document sync, revision
  sync/read, platform store bootstrap, existing platform Dolt rows, publication
  document routes that read platform Texture rows, and rollback to older
  binaries that still expect `platform_vtext_*`.
- Rollback path: source revert restores current code reads/writes to
  `platform_vtext_*`. Existing legacy rows remain in place. Rows created only
  in `platform_texture_*` after this cutover would need a reverse copy into
  `platform_vtext_*` before running an older binary that must see them; this
  slice intentionally stops dual-writing current rows to legacy tables.
- Heresy delta: repaired locally for platform table identity; user-store
  `vtext_*` tables, durable actor ids, stored route rows, Universal Wire refs,
  deployed proof, and protocol v0 remain open.

Open edge: land C42 through CI/deploy/staging identity and deployed product
proof. The deployed proof should exercise platform Texture sync/read or
publication document behavior, not only health, because this slice changes
platform store bootstrap and table identity.

## 2026-06-16 - C42 Landing Proof: Platform Table Identity

Claim: C42 is deployed-supported for the platform table identity slice. The
claim is scoped to platform bootstrap creating `platform_texture_*`, legacy
`platform_vtext_*` rows copying forward idempotently, and current platform
Texture reads/writes targeting current tables. It does not claim user-store
`vtext_*` table migration, durable actor-id migration, stored route-row
deletion, Universal Wire story-field settlement, or protocol v0.

Move: settle C42 scope with CI, staging deploy identity, and deployed
browser-public platform-backed read proof. Expected ΔV: promote C42 from local
support to deployed support; coarse V remains 2.

Actual ΔV: C42 deployed-supported. Coarse V remains 2 because user-store
table-name residue, durable actor/profile compatibility, Universal Wire
story-field proof, and protocol v0 remain.

Receipts:

- Behavior commit `c749e31b21da04575a8477872eb65ac6d881d8b2`
  (`platform: use texture tables for platform documents`) pushed to
  `origin/main`.
- CI run `27616117172` passed, including Go vet/build, non-runtime tests,
  integration-tagged smoke, Docs Truth Check, TLA+ model check, all four
  runtime shards, and staging deploy.
- Separate Docs Truth Check run `27616117130` and FlakeHub publish run
  `27616117230` passed for the same commit.
- Deploy job `81652699154` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `c749e31b21da04575a8477872eb65ac6d881d8b2`, deployed at
  `2026-06-16T12:05:27Z`; receipt stored at `/tmp/choir-c42-health.json`.
- Deployed platform-backed read proof
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/c42-platform-texture-read.tmp.spec.js`
  passed before the temporary spec was deleted. It used a fresh authenticated
  browser session and browser-public `/api/texture` routes with
  `read_owner=universal-wire-platform`; missing platform document and revision
  reads returned controlled `404` responses instead of platform store or missing
  table errors.
- Deployed Universal Wire staging acceptance
  `GO_CHOIR_RUN_UNIVERSAL_WIRE_STAGING=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/universal-wire-staging-acceptance.spec.js`
  passed, but staging still showed zero Universal Wire articles, so it remains
  empty-state coverage rather than the open story-field proof.

Protected surfaces and rollback:

- Mutation class: red; protected surfaces touched are platform Texture document
  sync, revision sync/read, platform store bootstrap, existing platform Dolt
  rows, browser-public platform-backed Texture read projection, and rollback to
  older binaries that still expect `platform_vtext_*`.
- Rollback path: revert commit
  `c749e31b21da04575a8477872eb65ac6d881d8b2`. Existing legacy rows remain in
  place. Rows created only in `platform_texture_*` after this cutover would
  need a reverse copy into `platform_vtext_*` before running an older binary
  that must see them; current code intentionally stops dual-writing legacy
  tables.
- Heresy delta: repaired deployed platform table identity; user-store `vtext_*`
  tables, durable actor/profile ids, stored public route-row compatibility,
  Universal Wire story-field proof, and protocol v0 remain open.

Open edge: choose the next bounded remaining slice: user-store `vtext_*` table
migration design, durable actor/profile compatibility, deployed Universal Wire
story-field proof, or protocol v0 after working-surface proof.
