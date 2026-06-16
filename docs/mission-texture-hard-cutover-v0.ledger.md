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
