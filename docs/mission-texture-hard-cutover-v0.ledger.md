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
