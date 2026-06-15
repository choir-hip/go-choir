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
