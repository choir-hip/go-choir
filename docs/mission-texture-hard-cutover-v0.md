# Mission: Texture Hard Cutover v0

## Summary

Texture is the promoted ontology and product language for Choir's versioned,
transclusive artifact control plane. The old V-name is now migration residue,
not current doctrine.

This mission is not a cosmetic rename. It is an ontology cutover. The codebase,
prompts, UI, APIs, tests, docs checker, high-read docs, and product-path proofs
must teach the same object: Texture as the artifact layer that turns autonomous
activity into directed results and compounding learning.

The protocol spec is deliberately not written first. The mission must make the
product path work, delete accidental complexity, prove the minimal surface, and
only then canonize a Texture Protocol v0.

## Source Documents

- [why-texture-2026-06-15.md](./why-texture-2026-06-15.md)
- [why-texture-background-2026-06-15.md](./why-texture-background-2026-06-15.md)
- [choir-doctrine.md](./choir-doctrine.md)
- the M3.4 first-draft regression paradoc linked through
  [mission-graph.yaml](./mission-graph.yaml)
- [mission-portfolio-2026-06-11.md](./mission-portfolio-2026-06-11.md)

## Problem

The system currently carries a split ontology. The product object has outgrown
its old internal name, but code, prompts, docs, tests, route names, tool names,
and acceptance language still teach the old object. That split invites shallow
patches: route fixes that preserve wrong concepts, prompt fixes that encode
workflow decisions, and docs that describe a control plane while the runtime
still names it like an internal text widget.

The current urgent regression is also a warning. A prompt can open the artifact
surface but fail to create the first useful revision. That failure is easier to
miss when acceptance overweights route topology and underweights browser-driven
proof of the actual artifact loop.

## Problem Checkpoint: Retired-Name Inventory

Mutation class: `green` documentation and evidence only. No runtime behavior,
schema, API, prompt default, UI, or test surface changed in this checkpoint.

Read-only search on 2026-06-15 confirms that the old V-name is not isolated
implementation residue. It is still the dominant artifact-control-plane name
across current docs, runtime, frontend, tests, API routes, data attributes,
tool names, prompt defaults, and storage identifiers.

Receipts:

- `rg -l -i 'vtext|\.vtext|VText|VTEXT'` over the worktree found retired-name
  content in 172 docs files, 82 runtime Go files, 35 frontend source files,
  33 frontend tests, 9 store files, 9 runtime prompt files, 6 type files,
  4 command files, 2 spec files, and both root contracts.
- The same inventory found retired-name path components in 44 docs paths,
  22 runtime Go paths, 18 frontend source paths, 16 frontend test paths,
  2 store paths, 1 type path, 1 runtime prompt path, and 1 command path.
- Selected affordance line counts: `/api/vtext` 505, `data-vtext` 604,
  `edit_vtext` 390, `request_super_execution` 122, V-name profile references
  417, `.vtext` 942, and `vtext_` 658.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json` completed in report-only mode with 212 docs and
  803 warnings before any Texture-specific checker rule was added.

Checker warning design:

- Add a report-only Texture retired-name warning family to `cmd/doccheck`
  rather than failing docs-only CI in the same pass.
- Scan current and mixed non-evidence docs plus code/prompt/frontend/test
  surfaces for retired-name terms: `VText`, `vtext`, `.vtext`, `/api/vtext`,
  `data-vtext`, `edit_vtext`, and `vtext_`.
- Treat `docs/why-texture-background-2026-06-15.md` as the standing historical
  background allowlist entry.
- Allow historical mission/evidence occurrences only when the manifest marks
  the file `claim_scope: historical` or `is_evidence: true`, or when a mixed
  mission line explicitly labels the occurrence as historical evidence,
  retired-name evidence, migration residue, or a deletion target.
- Current docs, prompts, UI labels, tests, API affordances, storage-facing names,
  and tool names should warn until renamed or explicitly classified as temporary
  cutover residue with a deletion receipt. Warning silence is not settlement;
  final settlement still requires the retired-name search to show only allowed
  historical/background occurrences.

## Problem Checkpoint: Platform Publication Route Residue

Mutation class: `green` documentation and evidence only. No runtime behavior,
schema, API, prompt default, UI, or test surface changed in this checkpoint.

Read-only search on 2026-06-16 confirms a specific remaining route split after
the main Texture API cutover: publication and platform-document routing still
teach the old artifact name even though `/api/texture` is the active canonical
document API.

Receipts:

- `frontend/src/lib/vtext.js` still documents and calls
  `/api/platform/vtext/publications` for publishing a Texture revision.
- `internal/proxy/handlers.go` still dispatches the public platform publish
  route at `/api/platform/vtext/publications` and the internal wire publish
  route at `/internal/wire/platform/publications/vtext`.
- `internal/proxy/platform_publish.go`,
  `internal/proxy/wire_platform_publish.go`, `internal/wirepublish/client.go`,
  and `internal/runtime/wire_platform_publish.go` still call platformd or proxy
  publication endpoints ending in `/vtext`.
- `internal/platform/handlers.go` still registers platformd internal publish,
  sync, document-read, and revision-read routes under
  `/internal/platform/publications/vtext` and `/internal/platform/vtext/...`.
- `/pub/vtext/...` public publication routes remain the live published URL
  shape and require a separate route migration/redirect policy; do not silently
  rename existing public article URLs in the same slice.

Next behavior slice design:

- hard-cut the platform/proxy/internal publication control routes to
  `/texture` naming without preserving a browser-public or platform-internal
  `/vtext` compatibility route;
- preserve `/pub/vtext/...` published reader URLs until a route identity
  migration plan exists, because existing public links are route state rather
  than merely handler names;
- prove the cutover with focused proxy/platform/runtime tests, CI, staging
  deploy identity, and a deployed route probe that shows the old control route
  absent while the new Texture route reaches its expected auth/method gate.

## Problem Checkpoint: App Identity And Storage Symbol Residue

Mutation class: `green` documentation and evidence only. No runtime behavior,
schema, API, prompt default, UI, test, or persistent state changed in this
checkpoint.

Read-only search on 2026-06-16 confirms that, after public route and visible UI
label cutovers, the retired name still carries several different kinds of state
with different migration risk. They must not be collapsed into one rename.

Receipts:

- Path inventory excluding `frontend/dist` found 103 current source/doc/test
  paths whose filenames still contain the retired name or `.vtext`.
- App identity search found 38 current frontend/runtime/store/test hits for
  `appId: 'vtext'`, `id: 'vtext'`, `AppID: "vtext"`, URL `app=vtext`, or
  preview/Trace agent ids. The canonical app registry still uses `id: 'vtext'`
  while the visible app name is already `Texture`.
- Storage symbol search found 1,009 hits for `vtext_documents`,
  `vtext_revisions`, `vtext_document_aliases`, `vtext_agent_mutations`,
  `vtext_controller_checkpoints`, `vtext_decisions`, `platform_vtext_*`,
  `database=vtext`, `.vtext`, and `go-choir-vtext`.
- Metadata/tool search found 791 hits for symbols such as `edit_vtext`,
  `vtext_ref`, `vtext_doc`, `vtext_revision`, `source_vtext`,
  `platformd_route_path`, `related_vtext`, `transcluded_vtext`, and `vtext_`.
- `frontend/src/lib/apps/registry.ts` exposes the current visible Texture app
  under the old app id; `frontend/src/App.svelte`,
  `frontend/src/lib/Desktop.svelte`, `frontend/src/lib/UniversalWireApp.svelte`,
  `frontend/src/lib/source-contract.ts`, and `frontend/src/lib/VTextEditor.svelte`
  still launch or auth-gate that app with `appId: 'vtext'`.
- `internal/store/desktop_test.go`, `internal/runtime/desktop_test.go`, and
  `internal/store/store_test.go` show persisted desktop/app state can contain
  `app_id='vtext'`.

Next behavior slice design:

- cut the canonical frontend app id from `vtext` to `texture` so new launches,
  desktop icons, app switchers, auth intents, source-open plans, and public
  preview windows teach Texture at the app identity layer;
- normalize the legacy `vtext` app id at the desktop-state boundary so existing
  persisted windows reopen as Texture instead of disappearing after deploy;
- keep auth intent kinds such as `save_vtext` and deeper storage/table/file
  symbols out of this slice unless tests prove they must move together;
- prove the slice with focused frontend build/tests, Go desktop-state tests if
  backend normalization is touched, CI, staging deploy identity, and a staging
  browser/DOM proof that the Texture app renders under `data-app-id="texture"`
  while legacy `app=vtext` URL or saved state still opens the same app.

## Repair: Texture App Identity

Mutation class: `orange`, because this changes frontend app identity, app
launch/replay behavior, desktop persistence/restore normalization, source-open
app selection, and runtime desktop-state API sanitization.

Conjecture delta: new app launches and restored windows can use canonical
`texture` app identity while deletion-receipted legacy `vtext` app ids still
resolve at launch, URL-intent, frontend desktop-store, and runtime desktop API
boundaries.

Protected surfaces: app registry, desktop window persistence/restore,
source-open app selection, auth intent replay, public preview windows, and
runtime desktop-state get/save.

Local evidence on 2026-06-16:

- `npm --prefix frontend run build` passed. Vite reported pre-existing
  Universal Wire warnings for unused `currentUser` export and `.wire-state`
  selectors.
- `nix develop -c go test -tags comprehensive -v ./internal/runtime -run '^TestDesktopState'`
  passed, including `TestDesktopStateSanitizesLegacyTextureAppID`.
- `nix develop -c scripts/go-test-runtime-shards` passed all four runtime
  shards.
- App-id residue search for `appId: 'vtext'`, `id: 'vtext'`, legacy open calls,
  `getAppIcon('vtext')`, `public-preview-vtext`, and `data-app-id="vtext"`
  found only public preview Trace fixture agent ids after excluding
  `frontend/dist`.

Rollback path: revert the behavior commit to restore canonical `vtext` app ids
and remove the frontend/runtime normalization shims.

Deployed evidence on 2026-06-16:

- Commit `f27c00154f4eb1025075cc6eb6b76383324dd5f1` passed CI run
  `27588733421`.
- Deploy job `81564942700` succeeded.
- Staging health at `https://choir.news/health` reported proxy and sandbox
  commit `f27c00154f4eb1025075cc6eb6b76383324dd5f1`, deployed at
  `2026-06-16T01:55:03Z`.
- Staging Playwright DOM proof on `https://choir.news/` found one
  `data-app-id="texture"` window, zero `data-app-id="vtext"` windows, one
  `data-desktop-icon-id="texture"` icon, zero legacy `vtext` desktop icons, and
  restored public preview window id `public-preview-texture`.
- Staging Playwright DOM proof on
  `https://choir.news/?app=vtext&doc=legacy-proof-doc&title=Legacy%20Texture`
  found one Texture window, zero legacy `vtext` windows, visible Texture text,
  and no visible `VText` text.

Heresy delta: repaired for deployed app identity; no storage/table/file/metadata
symbol repair claimed.

Remaining scope: storage schema/workspace/file suffixes, metadata keys,
`/pub/vtext/...` route identity, and protocol v0.

## Problem Checkpoint: Public Preview Trace Fixture Residue

Mutation class: `green` documentation and evidence only. No frontend source,
runtime behavior, schema, API, prompt default, UI, test, or persistent state
changed in this checkpoint.

Read-only search on 2026-06-16 shows that the next small residue class is a
public-preview Trace fixture in `frontend/src/lib/public-preview-data.ts`. It
still names the Texture actor as `agent_id: 'vtext'`, routes preview edges
through `vtext`, and records preview moments against `agent_id: 'vtext'`.
This is distinct from durable runtime agent ids such as `vtext:<doc_id>` and
from storage symbols such as `vtext_revisions`; it is local signed-out fixture
data.

Receipts:

- `rg -n "agent_id: 'vtext'|to_agent_id: 'vtext'|from_agent_id: 'vtext'"`
  on `frontend/src/lib/public-preview-data.ts` found seven fixture hits.
- `rg -n "previewTraceSnapshot|previewTraceTrajectories" . -g '!frontend/dist' -g '!node_modules'`
  found only the fixture definitions themselves, with no consumers.
- The fixture's acceptance text says "Trace layout renders without private
  trajectories", which conflicts with the current doctrine guardrail that Trace
  is evidence/topology, not a normal public product surface.

Next behavior/source slice design:

- delete the unused `previewTraceTrajectories` and `previewTraceSnapshot`
  fixture exports instead of renaming their actor ids, so the mission does not
  preserve a dead Trace product preview;
- keep the live `previewVTextDocument` export for the signed-out Texture app
  preview, leaving its exported symbol name for a later broader frontend file
  and API-name migration;
- prove with frontend build and residue searches that no public-preview Trace
  fixture actor id remains.

## Repair: Public Preview Trace Fixture Deletion

Mutation class: `yellow`, because this deletes unused frontend fixture exports
and changes future optimization/documentation pressure without changing a live
product path.

Conjecture delta: deleting the unused fixture is a cleaner Texture cutover move
than renaming it, because it removes a dead Trace-as-product preview and avoids
creating a new public Trace surface.

Protected surfaces: signed-out preview data module and frontend build.

Local evidence on 2026-06-16:

- `npm --prefix frontend run build` passed. Vite reported the existing
  Universal Wire warnings for unused `currentUser` and `.wire-state` selectors.
- `rg -n "previewTraceSnapshot|previewTraceTrajectories|preview-trace|Trace layout|agent_id: 'vtext'|to_agent_id: 'vtext'|from_agent_id: 'vtext'" frontend/src/lib/public-preview-data.ts frontend/src -g '!frontend/dist'`
  returned no hits.

Deployed evidence on 2026-06-16:

- Commit `3037e1f92971e7324a8bb8c3e356474e4eee2cc6` passed CI run
  `27589138319`; deploy job `81566163866` succeeded.
- Separate `Docs Truth Check` run `27589138321` and FlakeHub publish run
  `27589138328` completed successfully for the same commit.
- Staging health at `https://choir.news/health` reported proxy and sandbox
  commit `3037e1f92971e7324a8bb8c3e356474e4eee2cc6`, deployed at
  `2026-06-16T02:06:07Z`.
- Staging Playwright DOM proof on `https://choir.news/` found one
  `data-app-id="texture"` window, zero `data-app-id="vtext"` windows, one
  `data-desktop-icon-id="texture"` icon, zero legacy `vtext` desktop icons,
  and no visible "Trace layout", `preview-trace`, or public-preview `vtext`
  actor text.

Rollback path: restore the deleted fixture exports if a real consumer is found.

Heresy delta: repaired for deployed unused public-preview Trace fixture residue;
no durable runtime agent-id or storage-symbol repair claimed.

## Problem Checkpoint: `edit_texture` Compatibility Alias

Mutation class: `green` documentation and evidence only. No runtime behavior,
tool registration, prompt default, revision metadata, publication eligibility,
or test surface changed in this checkpoint.

Read-only search on 2026-06-16 shows that `edit_texture` is no longer the
common-path Texture write affordance, but it is still wired into several
separable layers. Removing it as a compatibility alias must not accidentally
remove legacy revision metadata needed for publication reads or turn the tool
loop into a semantic workflow gate.

Receipts:

- `rg -n "edit_texture" internal/runtime internal/wirepublish internal/proxy cmd frontend/tests frontend/src -g '!frontend/dist/**'`
  found current non-doc hits only in `internal/runtime` and
  `internal/wirepublish`: 118 runtime hits and 7 wire-publish hits across 15
  code/test files.
- `internal/runtime/tools_vtext.go` still registers
  `newEditTextureCompatibilityTool(rt)` for Texture and classifies
  `edit_texture` as a Texture write tool in `isTextureWriteToolName`.
- `internal/runtime/tools.go` still treats `edit_texture` as sequential and as
  a duplicate-protected Texture write tool.
- `internal/runtime/runtime.go` still treats `edit_texture` as a terminal
  Texture tool success even though `initialVTextToolChoice` now chooses
  `patch_texture` or `record_texture_decision`.
- `materializeVTextToolEdit` and `addVTextEditRevisionMetadata` still default
  a missing `SourceTool` to `edit_texture`; new `patch_texture` and
  `rewrite_texture` calls set `SourceTool` explicitly, so this is a fallback
  residue rather than the intended new-write path.
- `internal/wirepublish/eligibility.go` and
  `internal/runtime/universal_wire.go` still accept revision metadata
  `source=edit_texture` and legacy `source=edit_vtext` for autonomous wire
  publication eligibility and private publication reads. That is a persisted
  revision metadata compatibility concern, not the same surface as the
  model-visible compatibility tool.
- Test residue is broad: `rg -n "edit_texture" internal/runtime/*_test.go internal/wirepublish/*_test.go internal/proxy/*_test.go frontend/tests -g '!frontend/dist/**'`
  found 112 test hits, including tool-profile exposure tests, duplicate
  Texture write tests, email appagent tests, workflow verifier checks, and
  publication eligibility tests.

Next behavior slice design:

- remove the model-visible `edit_texture` registered tool from the Texture tool
  registry, agent profile expectations, terminal-tool success list, sequential
  side-effect list, and duplicate-write test fixtures;
- change new-write fallback metadata from `edit_texture` to `patch_texture` so
  untagged internal edit paths do not mint new alias metadata;
- keep explicit `source=edit_texture` and `source=edit_vtext` read/eligibility
  compatibility in wire publication and Universal Wire for this slice, with
  tests labeling it as persisted metadata migration residue rather than a live
  tool affordance;
- prove with focused runtime tests that Texture exposes `patch_texture`,
  `rewrite_texture`, and `record_texture_decision` but not `edit_texture`, that
  duplicate write protection still covers `patch_texture`/`rewrite_texture`,
  that no new `edit_texture` tool result is available, and that legacy metadata
  reads remain explicitly supported until a separate migration plan removes
  them.

## Local Repair: `edit_texture` Compatibility Alias Deletion

Mutation class: `red`, because this changes protected Texture tool exposure,
canonical write metadata fallback, tool-loop terminal handling, duplicate write
protection, and Texture writer tests.

Conjecture delta: removing the model-visible `edit_texture` compatibility alias
while preserving explicit legacy revision metadata compatibility should advance
the Texture tool ontology without breaking stored Universal Wire publication
history.

Protected surfaces: Texture tool registry, canonical Texture write metadata,
tool-loop terminal successes, duplicate Texture write protection, Universal
Wire publication eligibility/read compatibility, and Texture appagent tests.

Local evidence on 2026-06-16:

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

Deployed evidence on 2026-06-16:

- Commit `c6db0df57bd06a22e392fd89eb0f4ee1f4c1bcc1` passed CI run
  `27589732107`; deploy job `81567905099` succeeded.
- Staging health at `https://choir.news/health` reported proxy and sandbox
  commit `c6db0df57bd06a22e392fd89eb0f4ee1f4c1bcc1`, deployed at
  `2026-06-16T02:22:51Z`.
- Deployed Playwright product proof registered a fresh staging user, submitted
  prompt-bar request `d2a0ccf4-276f-43f2-be6b-f6da43fdaf15`, and received
  conductor -> Texture decision for document
  `d4e62340-bd4c-4644-9fd6-fb28a2b85d30`.
- The Texture head revision `f5fee46f-4178-4dc2-aee3-fe127525cd9b` had
  `metadata.source=patch_texture` and content
  "Current write tool: patch_texture. Do not call any retired compatibility
  alias."
- Trace for trajectory `d2a0ccf4-276f-43f2-be6b-f6da43fdaf15` contained
  conductor and Texture agents only, 28 moments, two `patch_texture returned`
  tool-result moments, four non-error `patch_texture` tool events, zero
  `rewrite_texture` hits, zero `edit_texture` hits, and zero `super` hits.
- The deployed UI proof found one Texture window, zero legacy `vtext` windows,
  visible `patch_texture` content, no visible `edit_texture`, no
  "Writing first draft" placeholder, and no forbidden browser requests to
  `/internal/*`, `/api/agent/*`, `/api/test/*`, `/api/prompts`, or
  `/api/events`.

Rollback path: restore the `edit_texture` registered tool, write-tool
classification, terminal success entry, duplicate-write handling entry, and
`edit_texture` metadata fallback if deployed Texture writers cannot use
`patch_texture` or `rewrite_texture`.

Heresy delta: repaired for the deployed model-visible `edit_texture`
compatibility alias; legacy `source=edit_texture` and `source=edit_vtext`
metadata compatibility remains discovered migration residue.

## Non-Goals

- Do not write a full protocol cold.
- Do not preserve compatibility aliases as indefinite dual paths.
- Do not implement semantic phrase matching in runtime to make the cutover pass.
- Do not weaken docs-only CI filters.
- Do not resume M3 or source/news work until the core prompt-bar artifact loop
  has product-path proof under the Texture ontology.

## Parallax State

status: open_handoff

mission conjecture: if Choir hard-cuts the artifact control-plane ontology to
Texture across docs, code, prompts, UI, tests, tool names, acceptance, and
checker warnings, while preserving the core prompt-bar -> conductor -> Texture
revision loop under deployed product proof, then the M3 lifecycle portfolio can
resume from a cleaner ontology with less route confusion and fewer hidden
workflow gates.

deeper goal (G): make Texture the stable semantic substrate for directing
autonomous results and compounding learnings, so safe self-development,
source/news articles, style, research, super evidence, and future media
projections all share one artifact-native control plane.

witness/spec (A/S):
- replace current user-facing, agent-facing, code-facing, and docs-facing uses
  of the retired V-name with Texture;
- preserve historical explanation only in
  `docs/why-texture-background-2026-06-15.md` and explicitly historical
  mission evidence;
- repair or preserve the deployed prompt-bar -> conductor -> Texture first
  revision loop;
- split the overloaded edit affordance into a common patch tool and an
  exceptional whole-document recovery rewrite, unless investigation proves a
  smaller surface is clearer;
- add report-only docs checker coverage for retired-name drift and later
  promote it to CI failure after the warning baseline is burned down;
- canonize `docs/texture-protocol-v0.md` only after implementation proof shows
  the minimal protocol surface.

invariants / qualities / domain ramp (I/Q/D):
- I: Texture owns canonical artifact meaning and learning; super owns
  privileged execution.
- I: among agents, one Texture writer writes canonical Texture state; other
  agents produce evidence, proposals, receipts, faults, diffs, source packets,
  and promotion claims.
- I: human direct edits remain canonical revisions.
- I: every Texture version is immutable, addressable, comparable, restorable,
  and forkable.
- I: transclusions pin version refs by default and the UI shows when newer
  versions exist.
- I: runtime protects mechanical invariants, not semantic decision trees.
- I: no indefinite dual path. Compatibility shims, if unavoidable for one
  deploy, must have deletion receipts before settlement.
- Q: names should teach distributional expectations. The common edit tool
  should sound common; the whole-document rewrite tool should sound
  exceptional.
- Q: product proof must use browser/computer-driven interaction on staging, not
  only API probes or local tests.
- D ramp: docs and detector warnings -> focused local tests -> staging deploy
  identity -> browser product proof -> protocol canonization.

variant (ranking function) V: current V=2; last ΔV=0 against the coarse
variant, with platform publication control-route cutover landed and deployed:
1. discharged: old-name inventory across code, docs, prompts, API routes,
   database tables, frontend labels, tests, scripts, and checker manifests is
   documented in the Problem Checkpoint above;
2. discharged: docs checker retired-name warning rule is implemented in
   report-only mode as H5 with the documented allowlist;
3. discharged: high-read doctrine, README/index, current architecture,
   runtime-invariants, mission portfolio, mission graph, and this paradoc have
   been reconciled to Texture or line-labeled as historical/deletion residue;
   `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
   /tmp/choir-doccheck.json` now reports no H5 warnings for that high-read set;
4. current V includes: storage, file, and metadata symbols still use
   the old ontology; frontend `data-vtext-*` attributes, frontend `/api/vtext`
   compatibility-route deletion target test probes, the browser-public
   `/api/vtext` route registration, the product API tool allowlist shim,
   registered-router old-route normalization, direct Texture handler test
   paths, and platform/proxy/internal publication control routes are discharged;
5. discharged: visible UI labels and import affordances are cut over to
   Texture and proven on staging through browser product evidence;
6. discharged: the edit affordance surface has a common `patch_texture` tool
   and an exceptional `rewrite_texture` tool; the model-visible `edit_texture`
   compatibility alias is deleted and staging product proof shows the
   prompt-bar Texture first revision stored through `patch_texture`;
7. discharged for local scope: prompt register and registered tool names now
   use Texture-oriented wording and `patch_texture` / `rewrite_texture` /
   `record_texture_decision` affordances without adding runtime semantic
   decision trees;
8. discharged for the current product-facing slice: deployed prompt-bar ->
   conductor -> Texture first-revision proof passed under `/api/texture` and
   `patch_texture`, with no `edit_texture` or super-before-Texture trace;
9. discharged: transclusion pinned-ref plus newer-version indicator behavior is
   locally focused-test green and proven on staging with browser/product UI
   evidence;
10. current V includes: Texture Protocol v0 is intentionally unwritten until
    the working minimal surface is proven.

budget: one broad red-surface cutover mission before M3 resumes. If the rename
reveals a distinct product regression, split the regression into a child
paradoc only after documenting it here.

authority / bounds: mutation class target is `red`; this document creation is
`green`. Protected surfaces for execution: canonical artifact writes, prompt
bar routing, conductor route materialization, Texture prompts/tools, Trace and
acceptance projection, UI labels, docs checker, deployment routing, and any
database migrations. Apply Problem Documentation First before behavior fixes.

evidence packet:
- retired-name inventory and allowlist;
- docs checker report with new warning family in report-only mode;
- focused tests for route, tool, prompt, and revision behavior;
- local sharded runtime tests when runtime changes land;
- pushed commits with CI run ids;
- Node B staging deploy identity for behavior-changing commits;
- browser/computer-use proof of prompt-bar submission creating a Texture,
  non-empty first appagent revision, history navigation, sources panel, and no
  super-before-Texture route;
- proof of pinned transclusion with newer-version indicator, or an explicit
  blocker if the UI surface is absent;
- final retired-name search showing only allowed historical/background
  occurrences;
- protocol v0 created only after the preceding proof.

heresy delta: discovered: the old ontology is now visible as a system-wide
drift source rather than a harmless implementation name. Introduced: none
accepted. Repaired target: delete dual-path naming, direct-super ingress
ambiguity, workflow-forcing prompts, and overloaded edit affordances where this
mission proves them.

position / live conjectures / open edges:
- C1 active: the hard rename is a vocabulary shift that should change route
  choice and acceptance quality, not just labels.
- C2 supported for deployed common-path scope: a common `patch_texture` tool
  plus an exceptional `rewrite_texture` tool better orients the Texture writer
  than one overloaded edit tool. Staging prompt-bar proof created a Texture
  first revision through `patch_texture` metadata and Trace. The compatibility
  alias deletion receipt remains open.
- C3 supported for report-only scope: the docs checker now carries H5
  retired-name warnings without failing docs-only CI. Current baseline:
  `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json` reports 1,128 total warnings, including 335 H5
  file-level warnings across AGENTS.md, cmd, docs, frontend,
  internal, and specs. Promotion to fail-closed remains future work after the
  baseline burns down.
- C4 active: some old mission docs may be cheaper and clearer to delete or
  leave only in git history than to rewrite under the new ontology.
- C5 active: protocol design before proof risks cathedral-building. The
  protocol should be the last deliverable, distilled from the working minimal
  surface.
- C6 supported for deployed product-route scope: `/api/texture` is registered
  and exercised by focused tests, frontend API callers, and staging
  Playwright product proof. The browser-public `/api/vtext` route and
  `product_api_request` allowlist shim are deleted and deployed; prior staging
  route proof showed `/api/texture/documents` reached the auth gate while
  `/api/vtext/documents` and `/api/vtext/diff` returned plain 404. Remaining
  browser-public route residue is gone. The follow-on registered
  router/extractor dependency on `/api/vtext` is also removed and deployed;
  authenticated legacy-route 404 behavior for that internal dispatch slice is
  covered by registered-router tests because the current browser automation
  session could not issue same-origin API fetches after deploy.
- C7 repaired and CI-green: CI exposed a Universal Wire publication compatibility
  regression. The route/tool slice made new Texture revisions write
  `source=edit_texture`, but the `internal/wirepublish` autonomous publication
  eligibility package still accepted only the retired edit-source metadata.
  Result: runtime shards 2 and 3 failed before staging deploy, with missing
  edition transclusion and missing in-flight publication work item evidence.
  The repair accepts current Texture metadata plus deletion-receipted legacy
  metadata in the wire publish/read predicates; the rerun passed CI and staged.
- C8 supported for deployed transclusion scope: related Texture refs now carry
  pinned revision identity, preserve the pin through editor serialization, open
  the pinned revision, and show a newer-version marker when the related Texture
  head advances. The deployed proof covered a parent Texture ref with pinned
  child revision v0 and current child revision v1 on staging.
- C9 supported for deployed visible-UI scope: visible app labels can switch to Texture while internal app ids,
  selectors, storage keys, and compatibility API names remain deletion-receipted
  residue. Staging proof covered the desktop icon, window title, recent landing,
  Files import button, and Web Lens import button.
- C10 supported for deployed common-path scope: `patch_texture` is the exact
  initial Texture write choice and staging Trace showed no successful
  `edit_texture` result for the proof trajectory. The later alias-deletion
  receipt is now also landed under C17.
- C11 supported for high-read docs scope: README, docs index, doctrine,
  current architecture, runtime invariants, mission portfolio, mission graph,
  and this paradoc now teach Texture as the current artifact control-plane
  ontology. Remaining old-name hits in that set are line-labeled historical
  mission paths, internal detector symbols, or compatibility route deletion
  targets; the high-read H5 subset is empty.
- C12 supported for frontend selector/probe scope: frontend source and tests
  no longer contain `data-vtext` selectors or `/api/vtext` product API probes.
  CI, staging deploy identity, and deployed DOM proof show `data-texture-*`
  selectors render and the old editor/toolbar selectors do not. Remaining
  frontend H5 warnings are app/file names, metadata keys, platform/internal
  publication terms, and historical test names.
- C13 supported for deployed registered-router normalization scope: the Texture
  router now dispatches on `/api/texture` directly, the shared doc/revision ID
  extractors only parse `/api/texture`, direct Texture API tests use
  `/api/texture`, and `/api/vtext` remains only in explicit legacy-route
  refusal tests for this runtime slice. CI run `27587124142` passed and Node B
  staging health reported commit `247e28415bb7b5a656b9d83072288403666c9c8a`.
- C14 supported for deployed route-control scope: platform publication control
  routes now use Texture paths
  (`/api/platform/texture/publications`,
  `/internal/wire/platform/publications/texture`,
  `/internal/platform/publications/texture`, and
  `/internal/platform/texture/...`), and private publication reads use
  `/api/texture`. The retired public control route returns 404, platformd
  registered-route tests reject the old internal prefixes, and `/pub/vtext/...`
  remains separately classified as live public route identity until a redirect
  and rollback policy exists. CI run `27587958358` passed, deploy job
  `81562610983` deployed commit `019e7a9d78f94e78da91ae2ddc6200dd7dee0184`,
  and staging route probes showed the new Texture control route reaches
  method/auth gates while the old control route returns 404.
- C15 supported for deployed app identity scope: app identity and storage
  symbols are distinct residue classes. The canonical app registry now uses
  `id: 'texture'`; frontend app launch/replay/source-open/public-preview paths
  now target Texture; frontend and runtime desktop-state boundaries normalize
  deletion-receipted legacy `vtext` app ids; staging DOM proof shows canonical
  `data-app-id="texture"` and legacy `app=vtext` compatibility. Storage
  table/workspace/file and metadata symbols are much broader and require
  separate migration design.
- C16 supported for deployed public-preview fixture scope: the unused
  public-preview Trace fixture exports were deleted instead of renamed. Frontend
  build passes, residue search no longer finds `previewTraceSnapshot`,
  `previewTraceTrajectories`, `preview-trace`, "Trace layout", or
  public-preview `vtext` actor ids in `frontend/src`, CI/deploy passed for
  commit `3037e1f92971e7324a8bb8c3e356474e4eee2cc6`, and staging DOM proof
  shows the signed-out Texture preview still renders without the deleted Trace
  fixture language.
- C17 supported for deployed alias-deletion scope: the model-visible
  `edit_texture` compatibility alias is removed from Texture tool registration,
  terminal handling, new-write fallback metadata, and duplicate-write fixtures.
  `patch_texture`/`rewrite_texture` remain the live Texture write tools.
  Persisted `source=edit_texture` and `source=edit_vtext` publication metadata
  compatibility remains separate migration residue and is intentionally
  preserved. Focused runtime tests, wirepublish tests, runtime shards,
  live-alias residue search, CI run `27589732107`, deploy job `81567905099`,
  staging identity for commit `c6db0df57bd06a22e392fd89eb0f4ee1f4c1bcc1`, and
  deployed prompt-bar/Trace proof all pass.

next move: select the next bounded residue class among storage
schema/workspace/file suffixes, metadata keys, `/pub/vtext/...` route identity,
and protocol v0. The `edit_texture` alias deletion slice is landed and proven
on staging; keep protocol v0 unwritten until the remaining working-surface
proofs are complete.

ledger file: `docs/mission-texture-hard-cutover-v0.ledger.md`

version / lineage: spawned from M3.4 readiness review and the 2026-06-15
Texture rename discussion. Blocks M3 until either settled or explicitly scoped
as a narrower dependency.

learning state: Texture exists to direct results with autonomy and facilitate
learnings. The rename must preserve that reason, not collapse into branding or
API churn.

settlement: settled only when the repo has no non-allowed retired-name
occurrences, Texture docs and doctrine agree, warning-only checker coverage is
landed, deployed product proof shows the core Texture revision loop, the
transclusion UI rule is proven or blocked with a successor, and a minimal
Texture Protocol v0 is canonized from the working surface.

## Suggested Goal String

```text
Use Parallax on docs/mission-texture-hard-cutover-v0.md. Treat it as the source
program for the Texture hard cutover before M3 resumes. Texture is the promoted
ontology for Choir's versioned, transclusive artifact control plane; the old
V-name is migration residue allowed only in the historical background doc and
explicit historical mission evidence. Current status is open_handoff with V=2.
The read-only retired-name inventory, Problem Documentation First checkpoint,
report-only H5 docs checker, operating-contract/high-read-doc Texture
reconciliation, and a deployed product-facing route/tool/prompt slice plus
deployed transclusion pinned-ref/newer-version proof, visible UI label proof,
and deployed `patch_texture` common-path proof are landed. Continue renaming docs/code/
prompts/UI/tests/tool affordances toward Texture; frontend `data-texture-*`
selectors, frontend `/api/texture` probes, browser-public Texture route
registration, product API allowlist cutover, and registered-router
normalization are landed while deeper backend/internal old-name residue
remains.
Preserve one Texture writer among agents, keep human
direct edits canonical, keep super downstream of Texture for privileged
execution, and avoid runtime semantic decision trees. Do
not canonize a Texture Protocol upfront; make protocol v0 the last deliverable
after the working minimal product surface is proven. Append moves to
docs/mission-texture-hard-cutover-v0.ledger.md and settle only with CI, staging
identity, deployed acceptance, retired-name search receipts, checker report,
and a minimal protocol distilled from proof.
```
