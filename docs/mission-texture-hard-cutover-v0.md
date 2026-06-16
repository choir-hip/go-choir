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

## Problem Checkpoint: Public Publication Route Identity

Mutation class: `green` documentation and evidence only. No route generation,
database state, frontend routing, API behavior, prompt default, test, or
deployment surface changed in this checkpoint.

Read-only search on 2026-06-16 shows that publication control routes are now
Texture-named, but newly published public reader URLs still mint under
`/pub/vtext/...`. This is live public route identity, not the same surface as
the browser-public `/api/texture` document API or platform/internal publication
control routes already cut over.

Receipts:

- `internal/platform/service.go` still defines `publicVTextPrefix =
  "/pub/vtext/"` and constructs new `routePath` values from that prefix in
  `PublishVText`.
- The same file stores the slug by trimming `publicVTextPrefix` and only
  normalizes trailing slashes for routes with that prefix in
  `normalizePublicationRoutePath`.
- `frontend/src/App.svelte` only recognizes direct public reader entry when
  `window.location.pathname.startsWith('/pub/vtext/')`.
- `frontend/src/lib/Desktop.svelte` only normalizes public reader paths with
  the `/pub/vtext/` prefix before opening a published Texture window or
  deduplicating already-open published windows.
- Product tests still assert newly published route paths match
  `^/pub/vtext/` in `frontend/tests/file-browser.spec.js` and
  `frontend/tests/vtext-source-service-publication.spec.js`; platform and proxy
  tests still fixture public routes under `/pub/vtext/...`.

Next behavior slice design:

- mint new publication reader routes under `/pub/texture/...`;
- continue resolving and exporting existing stored `/pub/vtext/...` rows
  through `/api/platform/publications/resolve` and
  `/api/platform/publications/export`, because those rows are public link state;
- make the frontend public reader recognize both `/pub/texture/...` and
  deletion-receipted legacy `/pub/vtext/...` route paths so existing public
  links keep opening Texture;
- avoid database rewrites or silent external-link redirects in this slice; a
  redirect/migration policy can be a later settlement move after new
  Texture-named URLs are proven;
- prove locally with platform route generation/read tests, proxy public
  resolve/export tests, and frontend publication tests that new routes are
  Texture-named while legacy reader paths are still accepted.

## Local Repair: Public Publication Route Identity

Mutation class: `orange`, because this changes public publication route
generation, frontend public-reader route recognition, proxy/platform route
tests, and publication-product expectations.

Conjecture delta: new public publication links can teach Texture by minting
`/pub/texture/...` while existing `/pub/vtext/...` link state remains readable
through explicit legacy route acceptance.

Protected surfaces: platform publication route generation, public route
lookup/export, frontend direct public reader entry, published Texture window
deduplication, proxy publication public URL projection, and product
publication tests.

Local evidence on 2026-06-16:

- `nix develop -c go test ./internal/platform -run 'TestPublishVTextCreatesImmutablePublicRecords|TestInternalPublishRequiresInternalCallerAndBundleResolve'`
  passed.
- `nix develop -c go test ./internal/proxy -run 'TestPlatformPublicationResolveIsPublicAndInternalOnly|TestPlatformPublicationResolveAndExportPropagateNotFound|TestHandleVTextPublication'`
  passed.
- `nix develop -c go test ./internal/platform ./internal/proxy` passed.
- `npm --prefix frontend run build` passed. Vite reported pre-existing
  Universal Wire warnings for unused `currentUser` and `.wire-state`
  selectors.
- Route residue search
  `rg -n "publicVTextPrefix|/pub/vtext/|\^\\/pub\\/vtext|startsWith\('/pub/vtext/'\)|startsWith\(\"/pub/vtext/\"\)" internal/platform internal/proxy frontend/src frontend/tests --glob '!frontend/dist/**'`
  now finds only the explicit legacy route prefix/helper, legacy route tests or
  fixtures, and frontend dual-prefix acceptance.
- Local Playwright was attempted, but the local service harness could not reach
  platformd because the existing `/tmp/go-choir-m2/platform-dolt` state reported
  missing `.dolt/repo_state.json`. The controlled foreground service session
  was stopped and health checks for local service ports returned down.

Rollback path: restore `/pub/vtext/...` route minting, remove
`/pub/texture/...` public-reader prefix recognition, and revert route
expectations if staging publication/read/export proof fails.

Heresy delta: repaired locally for new public route minting; legacy
`/pub/vtext/...` public links remain explicit compatibility state pending
deployed proof and any later redirect/migration policy.

## Deployed Repair: Public Publication Route Identity

Mutation class: `orange`, deployed behavior evidence for the public route
identity repair.

Conjecture delta: deployed Choir can mint new public publication URLs under
`/pub/texture/...` while preserving existing `/pub/vtext/...` public link state
for resolve, export, and direct public reader entry.

Deployed evidence on 2026-06-16:

- Commit `65502a706ef1adba7fc2d1ed5428e3f709f9d2d0` passed CI run
  `27590698503`; the deploy job `81570766605` succeeded.
- Docs Truth Check run `27590698536` passed, and FlakeHub publish run
  `27590698504` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `65502a706ef1adba7fc2d1ed5428e3f709f9d2d0`, deployed at
  `2026-06-16T02:50:42Z`.
- Deployed Playwright product proof registered fresh user
  `texture-public-route-proof-1781578657650-ce9lel@example.com`, created
  Texture document `79579ae6-f620-4194-9a0a-afabee56a1fd`, created revision
  `e673f6f3-3c80-4577-9699-be146f996283`, and published publication
  `pub-19a8e51e-732d-498e-814c-fe18aa37568a` /
  version `pubver-4f361ae5-30e0-4ed6-b9a8-6dd1edb9c2ef`.
- The new route was
  `/pub/texture/texture-public-route-proof-1781578657650-pub19a8e51e7`.
  Public resolve normalized the route with a trailing slash back to that exact
  path, public Markdown export returned the same route and proof content, and
  retrieval search for `1781578657650` returned the new `/pub/texture/...`
  route.
- Direct browser navigation to the new route opened one Texture window and one
  published reader, displaying proof stamp `1781578657650`.
- Legacy route
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6` resolved
  with trailing slash normalization, exported Markdown, and opened in the
  published Texture reader.
- The browser proof observed zero forbidden product-path requests to
  `/internal/*`, `/api/agent/*`, `/api/test/*`, `/api/prompts`, or
  `/api/events`.

Evidence artifact: `/tmp/choir-texture-route-proof-1781578657650.json`.
Screenshots: `/tmp/choir-texture-route-proof/new-texture-route-1781578657650.png`
and `/tmp/choir-texture-route-proof/legacy-vtext-route-1781578657650.png`.

Rollback path remains: restore `/pub/vtext/...` route minting and remove
`/pub/texture/...` public-reader prefix recognition if later deployed public
reader/export regressions appear.

Heresy delta: repaired for deployed new public route minting. Existing
`/pub/vtext/...` public routes remain deliberate legacy compatibility state,
not a current new-publication minting path.

## Problem Checkpoint: Texture Auth Intent Label Residue

Mutation class: `green` documentation and evidence only. No runtime behavior,
frontend source, schema, API, prompt default, UI, test, or persistent state
changed in this checkpoint.

Read-only search on 2026-06-16 shows a narrow product-facing residue class:
frontend auth-required intent kinds and replay labels still use old-name
tokens even though the canonical app id and visible app label are now Texture.
This is distinct from durable runtime actor ids such as `vtext:<doc_id>`,
source metadata keys such as `vtext_source_artifact_attachment`, and storage
symbols such as `vtext_documents`.

Receipts:

- `frontend/src/lib/apps/registry.ts` still declares Texture auth requirements
  as `save_vtext`, `revise_vtext`, and `publish_vtext`.
- `frontend/src/lib/VTextEditor.svelte` still dispatches auth intents
  `save_vtext`, `publish_vtext`, `vtext_diagnosis`,
  `vtext_source_repair`, `vtext_source_artifact`, and
  `published_vtext_edit` while passing `appId: 'texture'` and
  `appName: 'Texture'`.
- `frontend/src/App.svelte` still renders/replays `save_vtext`,
  `publish_vtext`, `published_vtext_edit`, and `private_vtext_document`
  intent kinds.
- Legacy route compatibility remains in `frontend/src/App.svelte` for
  `?app=vtext&doc=...`, and tests intentionally cover that compatibility.
  That is app-route compatibility, not a current app identity target.
- Nearby hits such as `created_from: 'vtext_source_artifact_ui'`,
  `source: vtext_source_artifact_attachment`, `publish_vtext_revision`,
  `choir.platform.publish_vtext.v0`, and `vtext:<doc_id>` are metadata,
  provenance, verifier, or runtime actor-route residue. They need separate
  migration design and must not be renamed as part of this small frontend
  auth-intent slice.

Next behavior slice design:

- introduce Texture-named frontend auth intent kinds:
  `save_texture`, `revise_texture`, `publish_texture`,
  `texture_diagnosis`, `texture_source_repair`,
  `texture_source_artifact`, `published_texture_edit`, and
  `private_texture_document`;
- keep legacy intent-kind handling in the auth overlay/replay boundary during
  the cutover so already-created in-memory or URL-derived intents do not drop;
- update Texture app registry auth requirements and Texture editor dispatches
  to emit only the new intent kinds;
- keep `?app=vtext&doc=...` legacy URL compatibility and durable
  `vtext:<doc_id>` actor ids out of this slice;
- prove locally with frontend build and targeted frontend tests, then push,
  monitor CI/deploy, verify staging identity, and run a deployed browser proof
  that a signed-out Texture action opens an auth overlay whose pending intent
  is Texture-named while legacy `app=vtext` still opens Texture.

## Local Repair: Texture Auth Intent Labels

Mutation class: `orange`, because this changes frontend auth-required intent
kinds, Texture app registry auth requirements, auth overlay test affordances,
and post-auth replay normalization.

Conjecture delta: new frontend Texture actions can emit Texture-named auth
intent kinds while the auth overlay and replay boundary still accepts
deletion-receipted legacy intent names and legacy `?app=vtext&doc=...` URL
compatibility.

Protected surfaces: Texture app registry auth requirements, Texture editor
auth-required dispatches, auth overlay copy, post-auth app replay, legacy
intent replay, legacy `?app=vtext&doc=...` URL compatibility, and signed-out
public preview Texture actions.

Local evidence on 2026-06-16:

- `npm --prefix frontend run build` passed. Vite reported the existing
  Universal Wire warnings for unused `currentUser` and `.wire-state`
  selectors.
- `npm --prefix frontend run e2e -- --project=chromium
  tests/auth-entry-ui.spec.js --grep "signed-out Texture publish"` passed
  against an explicit local Vite preview server.
- A broader
  `npm --prefix frontend run e2e -- --project=chromium
  tests/auth-entry-ui.spec.js` attempt failed before app execution because no
  local server was listening on `localhost:4173`; this was harness setup, not a
  product assertion.
- Producer residue search
  `rg -n "save_vtext|revise_vtext|publish_vtext|vtext_diagnosis|vtext_source_repair|vtext_source_artifact|published_vtext_edit|private_vtext_document" frontend/src frontend/tests -g '!frontend/dist'`
  now finds only the explicit legacy normalization map in `frontend/src/App.svelte`
  and the out-of-scope provenance marker
  `created_from: 'vtext_source_artifact_ui'`.

Rollback path: restore old intent strings in Texture editor dispatches,
registry auth requirements, and App replay/message handling if auth overlay
replay or legacy app URL compatibility regresses.

Heresy delta: repaired locally for new frontend auth intent labels; durable
actor ids, storage symbols, and source/provenance metadata remain separate
discovered residue.

Open edge: push the repair, monitor CI/deploy, verify staging identity, and run
a deployed browser proof that a signed-out Texture action exposes a
Texture-named auth intent while legacy `?app=vtext&doc=...` still opens
Texture.

## Deployed Repair: Texture Auth Intent Labels

Mutation class: `orange`, deployed behavior evidence for the frontend
auth-intent label repair.

Conjecture delta: deployed Choir can present Texture-named auth-required intent
state for signed-out Texture actions while preserving deletion-receipted legacy
`?app=vtext&doc=...` compatibility for authenticated document deep links.

Deployed evidence on 2026-06-16:

- Commit `2f13598d37be2807f8cefe9258300a1a798a081c` passed CI run
  `27591417530`; the deploy job `81572916777` succeeded.
- Docs Truth Check run `27591417528` passed, and FlakeHub publish run
  `27591417545` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `2f13598d37be2807f8cefe9258300a1a798a081c`, deployed at
  `2026-06-16T03:10:59Z`.
- Deployed Playwright proof
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-auth-intent-deployed.tmp.spec.js`
  passed both browser assertions before the temporary spec was deleted.
- The signed-out proof opened the public Texture preview, used the Texture
  publish action, observed `[data-auth-overlay]` with
  `data-auth-intent-kind="publish_texture"`, observed auth copy containing
  "Publish this Texture", observed zero `[data-app-id="vtext"]` windows, and
  observed zero forbidden browser-public requests to `/internal/*`,
  `/api/agent/*`, `/api/test/*`, `/api/prompts`, or `/api/events`.
- The authenticated legacy URL proof registered a fresh staging user with a
  virtual passkey, created a Texture document through `/api/texture/documents`,
  created a revision through `/api/texture/documents/{doc}/revisions`,
  navigated to `?app=vtext&doc=...`, and observed exactly one canonical
  `[data-app-id="texture"]` window, zero `[data-app-id="vtext"]` windows,
  rendered proof content, and a consumed URL with no `app=vtext` query.

Screenshots: `/tmp/choir-texture-auth-intent-1781579569646.png` and
`/tmp/choir-texture-auth-legacy-url-1781579569646.png`.

Rollback path remains: restore old intent strings in editor dispatches,
registry requirements, and App replay/message handling if later auth replay or
legacy app URL compatibility regresses.

Heresy delta: repaired for deployed frontend auth intent labels. Durable actor
ids, storage symbols, and source/provenance metadata remain separate discovered
residue.

## Problem Checkpoint: Source Repair Metadata Label Residue

Mutation class: `green` documentation and evidence only. No runtime behavior,
frontend source, schema, API, prompt default, UI, test, or persistent state
changed in this checkpoint.

Read-only search on 2026-06-16 shows a narrow source/provenance metadata
residue class: new source repair and source artifact paths can still emit
old-name `vtext_source_*` provenance strings even though the user-visible
surface, product API routes, and auth intents now teach Texture. This is
separate from storage table names, durable actor ids, publication verifier
predicates, and app-package review contract fields.

Receipts:

- `internal/runtime/vtext_source_repairs.go` still writes revision metadata
  `source="vtext_source_gap_repair"` for source gap repairs and
  `source="vtext_source_artifact_attachment"` for source artifact attachment
  revisions.
- `frontend/src/lib/vtext-source-actions.ts` still creates source content item
  metadata with `created_from: 'vtext_source_artifact_ui'`.
- `internal/runtime/vtext_test.go` still asserts the old emitted metadata
  values in source repair and source artifact attachment tests.
- `frontend/tests/vtext-markdown-lineage.spec.js` still asserts
  `repaired.metadata?.source === 'vtext_source_gap_repair'`.
- Adjacent metadata hits such as `canonical_vtext_source_path`,
  `related_vtexts`, `story_vtext_doc_id`, `vtext_doc_id`,
  `vtext_revision_id`, `private_vtext_revision`,
  `publish_vtext_revision`, and `choir.platform.publish_vtext.v0` are broader
  storage, transclusion, app-package review, or platform publication
  provenance surfaces and require separate migration design.

Next behavior slice design:

- emit `texture_source_gap_repair`,
  `texture_source_artifact_attachment`, and
  `texture_source_artifact_ui` for new source repair/artifact paths;
- keep source entity structs, source routes, and `.vtext`/alias/storage fields
  out of this slice;
- preserve no reader compatibility unless investigation finds a live consumer
  of these exact old emitted values; if compatibility is required, make it an
  explicit legacy read predicate with a deletion receipt rather than continuing
  to emit old values;
- prove locally with focused runtime source repair tests, the focused frontend
  markdown-lineage/source repair test, frontend build, and residue searches,
  then push, monitor CI/deploy, verify staging identity, and run a deployed
  proof through the source repair or source artifact product path if the
  behavior reaches staging.

## Local Repair: Source Repair Metadata Labels

Mutation class: `orange`, because this changes new revision metadata emitted by
the source repair and source artifact attachment product paths, plus frontend
source content item provenance metadata.

Conjecture delta: new source repair/artifact metadata can emit Texture-named
provenance values without changing source entity structures, source routes,
storage tables, `.vtext` alias behavior, durable actor ids, or platform
publication attestations.

Protected surfaces: source gap repair revision metadata, source artifact
attachment revision metadata, frontend source content item creation provenance,
source repair tests, source artifact attachment tests, and markdown-lineage
browser tests.

Local evidence on 2026-06-16:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVTextSourceGapRepairCreatesRevision|TestVTextSourceArtifactAttachmentCreatesMetadataOnlyRevision'
  -count=1` passed.
- `npm --prefix frontend run build` passed. Vite reported the existing
  Universal Wire warnings for unused `currentUser` and `.wire-state`
  selectors.
- Live residue search
  `rg -n "vtext_source_gap_repair|vtext_source_artifact_attachment|vtext_source_artifact_ui" internal frontend/src frontend/tests -g '!frontend/dist/**'`
  returned no hits.
- Texture-name search now finds only the intended emitters and focused
  assertions for `texture_source_gap_repair`,
  `texture_source_artifact_attachment`, and
  `texture_source_artifact_ui`.
- Local Playwright attempt
  `npm --prefix frontend run e2e -- --project=chromium
  tests/vtext-markdown-lineage.spec.js --grep "Migrated source gaps"`
  failed before app execution because no local server was listening on
  `localhost:4173`; this is local harness availability, not product behavior
  evidence.

Rollback path: restore the old emitted `vtext_source_*` metadata values and
test expectations if source repair, source artifact attachment, or downstream
metadata readers regress.

Heresy delta: repaired locally for new source repair/artifact metadata labels;
broader metadata, storage, actor-id, app-package, and platform publication
provenance residue remains discovered and out of scope.

Open edge: push the repair, monitor CI/deploy, verify staging identity, and run
a deployed product proof for source gap repair metadata through the
browser/API path.

## Deployed Repair: Source Repair Metadata Labels

Mutation class: `orange`, because this changed new revision metadata emitted
by source repair and source artifact attachment paths, plus frontend source
content item provenance metadata.

Conjecture delta: deployed source repair metadata can teach Texture at the
new-emission boundary while preserving source entity structures, source routes,
storage tables, `.vtext` alias behavior, durable actor ids, and platform
publication attestations for later migration slices.

Protected surfaces: deployed source gap repair revision metadata, deployed
Texture document/revision APIs, Texture desktop document opening, browser-public
route hygiene, staging deployment identity, and focused runtime/frontend tests.

Admissible evidence class: focused local tests, residue search, full CI,
Node B staging deploy identity, and deployed browser/product proof that creates
a source repair through public Texture APIs and observes
`metadata.source="texture_source_gap_repair"`.

Deployed evidence on 2026-06-16:

- Pushed behavior commit:
  `39d0c2ba125c81d59b34002685a9ce19ec98eda0`
  (`runtime: rename texture source metadata labels`), after docs checkpoint
  `9498bae2`.
- CI run `27591835245` passed. Runtime shards 0-3, non-runtime tests,
  integration-tagged smoke, Go vet/build, frontend build, Docs Truth Check job,
  TLA+ model check, final Go gate, and Node B staging deploy job all passed.
- Deploy job `81574215697` succeeded.
- Docs Truth Check run `27591835237` passed; FlakeHub publish run
  `27591835231` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `39d0c2ba125c81d59b34002685a9ce19ec98eda0`, deployed at
  `2026-06-16T03:22:47Z`.
- Deployed Playwright proof
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-source-metadata-deployed.tmp.spec.js`
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
  `texture_source_gap_repair`, not the retired
  `vtext_source_gap_repair`; repaired content linked the citation to the
  source entity; the Texture desktop app opened the proof document under
  canonical `texture` app identity; the rendered citation transclusion showed
  the source label and excerpt; forbidden browser-public request count was
  zero for `/internal/*`, `/api/agent/*`, `/api/test/*`, `/api/prompts`, and
  `/api/events`.

Rollback path: restore the old emitted `vtext_source_*` metadata values and
test expectations if later source repair, source artifact attachment, or
downstream metadata readers regress.

Heresy delta: repaired for deployed new source repair/artifact metadata labels.
Adjacent metadata keys such as `canonical_vtext_source_path`,
`related_vtexts`, app-package `vtext_doc_id` and `vtext_revision_id`,
platform publication provenance, storage symbols, and durable actor ids remain
discovered residue outside this slice.

## Problem Checkpoint: App Package And Platform Provenance Label Residue

Mutation class: `green` documentation and evidence only. No runtime behavior,
frontend source, platform provenance writes, tool schema, prompt default, API,
test, or persistent state changed in this checkpoint.

Read-only search on 2026-06-16 shows a protected evidence/provenance residue
class: AppChangePackage human-proof refs and platform publication provenance
still teach the old artifact ontology even though the app, routes, public
publication URLs, auth intents, and source repair metadata now teach Texture.
This is separate from Universal Wire story projection metadata, general
Texture document metadata keys, storage tables, file suffixes, and durable
actor ids.

Receipts:

- `internal/runtime/tools_shipper.go` still exposes
  `publish_app_change_package` args and schema fields `vtext_doc_id` and
  `vtext_revision_id`, writes the same keys into package provenance refs, and
  describes the human proof narrative as VText.
- `internal/runtime/api_app_promotion.go` still classifies human proof
  narrative refs by keys or values containing `vtext`, and missing-proof copy
  says `narrative VText`.
- `internal/runtime/prompt_defaults/vsuper.md` still instructs candidate
  publishers to produce a causal VText narrative and pass `vtext_doc_id` /
  `vtext_revision_id`.
- `internal/runtime/agent_tools_test.go`,
  `internal/runtime/app_promotion_test.go`, and
  `frontend/tests/web-surface-rationalization.spec.js` still use those old
  app-package evidence field names in current fixtures.
- `internal/platform/service.go` still writes publication provenance and
  verifier records using `private_vtext_revision`,
  `publish_vtext_revision`, `choir-private:vtext/...`, and
  `choir.platform.publish_vtext.v0`.
- `internal/platform/service_publication_read.go` still rewrites
  `private_vtext_revision` citation edges so private revision ids do not leak
  into public bundles.
- Adjacent hits such as `story_vtext_doc_id`, `projection_vtext_docs`,
  `vtext_content`, `source-network-vtext-index`,
  `canonical_vtext_source_path`, `related_vtexts`, durable `vtext:<doc_id>`
  actor ids, storage tables, and `.vtext` file aliases are broader surfaces
  kept out of this slice.

Next behavior slice design:

- emit and document `texture_doc_id` and `texture_revision_id` for new
  AppChangePackage human-proof refs;
- update the human-proof detector and review evidence copy so current
  Texture narrative refs are first-class;
- keep deletion-receipted legacy read compatibility for existing package
  provenance refs only if review-evidence tests prove it is needed;
- emit platform publication provenance as `private_texture_revision`,
  `publish_texture_revision`, `choir-private:texture/...`, and
  `choir.platform.publish_texture.v0`;
- keep public bundle reads from leaking either legacy or current private
  revision ids;
- prove locally with focused runtime app-promotion/shipper tests, platform
  publication tests, frontend fixture tests if touched, residue search for new
  emitters, then push, monitor CI/deploy, verify staging identity, and run a
  deployed product/API proof for AppChangePackage review evidence or platform
  publication provenance if the behavior is reachable on staging without
  manually seeding success records.

## Local Repair: App Package And Platform Provenance Labels

Mutation class: `red`, because this changes protected AppChangePackage
human-proof evidence fields, vsuper package-publishing prompt defaults,
platform publication provenance entities/activities/verifier predicates, and
public bundle private-revision redaction behavior.

Conjecture delta: new package review evidence and platform publication
provenance can teach Texture at the evidence contract boundary while existing
legacy package provenance and legacy platform rows remain readable only behind
deletion-receipted compatibility.

Protected surfaces: AppChangePackage tool schema and provenance refs,
review-evidence human-proof classification, vsuper prompt defaults, platform
publication provenance/citation/verifier rows, public bundle citation
redaction, focused runtime/platform tests, and frontend review-evidence
fixtures.

Local evidence on 2026-06-16:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestPublishAppChangePackageToolPublishesWithoutGitHubPush|TestAppChangePackageReviewEvidenceRequiresNarrativeAndMediaForHumanReview'
  -count=1` passed.
- `nix develop -c go test ./internal/platform -run
  'TestPublishVTextCreatesImmutablePublicRecords|TestInternalPublishRequiresInternalCallerAndBundleResolve'
  -count=1` passed. The focused platform test now asserts stored
  `private_texture_revision`, `choir-private:texture/...`,
  `publish_texture_revision`, and `choir.platform.publish_texture.v0`
  values and still verifies public bundle reads do not leak private revision
  ids.
- `npm --prefix frontend run build` passed. Vite reported the existing
  Universal Wire warnings for unused `currentUser` and `.wire-state`
  selectors.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json` completed report-only with 212 docs and 1,129
  warnings.
- `git diff --check` passed.
- Current-emitter residue search for old package/platform labels across the
  touched runtime/platform/frontend-test files now finds only explicit legacy
  compatibility/read assertions:
  `private_vtext_revision` redaction support in
  `internal/platform/service_publication_read.go`, a no-leak assertion in
  `internal/platform/service_test.go`, and a legacy package-provenance fixture
  in `internal/runtime/app_promotion_test.go`.
- Texture-name search finds the new emitted/proven values:
  `texture_doc_id`, `texture_revision_id`, `private_texture_revision`,
  `choir-private:texture/...`, `publish_texture_revision`, and
  `choir.platform.publish_texture.v0`.
- Focused frontend Playwright attempt
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/web-surface-rationalization.spec.js --grep "package-scoped machine receipts"`
  failed before exercising the package evidence assertions because the test
  still opens the retired `apps-changes` launcher path while the current app
  registry exposes the surface as `features`. This is a stale frontend test
  launcher residue, not AppChangePackage provenance behavior evidence.

Rollback path: restore the old emitted package provenance field names and
platform provenance predicates if AppChangePackage review evidence, platform
publication, public bundle reads, or downstream adoption proof regresses.

Heresy delta: repaired locally for new AppChangePackage and platform
publication provenance labels. Legacy package provenance refs and legacy
platform rows remain deletion-receipted read compatibility until a migration
or deletion receipt removes them. Universal Wire story projection fields,
general Texture metadata keys, durable actor ids, storage symbols, and file
suffixes remain separate discovered residue.

Open edge: push the repair, monitor CI/deploy, verify staging identity, then
run deployed product/API proof for AppChangePackage review evidence or platform
publication provenance without manually seeding success records.

## Deployed Repair: App Package And Platform Provenance Labels

Mutation class: `red`, because this changed protected AppChangePackage
human-proof evidence fields, vsuper package-publishing prompt defaults,
platform publication provenance entities/activities/verifier predicates, and
public bundle private-revision redaction behavior.

Conjecture delta: deployed package review evidence can teach Texture at the
evidence contract boundary while existing legacy package provenance and legacy
platform rows remain readable only behind deletion-receipted compatibility.

Protected surfaces: deployed AppChangePackage create/detail/review-evidence
APIs, package provenance refs, review-evidence human-proof classification,
platform publication provenance/citation/verifier rows, public bundle citation
redaction, staging deploy identity, and browser-public route hygiene.

Deployed evidence on 2026-06-16:

- Pushed behavior commit:
  `24bff527b56e8f76e1ba3066dd5c71d52543120e`
  (`runtime: rename texture package provenance labels`), after docs
  checkpoint `5a7e8a40`.
- CI run `27592592351` passed. Runtime shards 0-3, non-runtime tests,
  integration-tagged smoke, Go vet/build, Docs Truth Check job, TLA+ model
  check, final Go gate, and Node B staging deploy job all passed. The frontend
  build job was skipped by deploy-impact classification because this slice did
  not change deployed frontend source.
- Deploy job `81576474144` succeeded.
- Docs Truth Check run `27592592337` passed; FlakeHub publish run
  `27592592343` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `24bff527b56e8f76e1ba3066dd5c71d52543120e`, deployed at
  `2026-06-16T03:44:38Z`.
- Deployed Playwright proof
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-package-provenance-deployed.tmp.spec.js`
  passed before the temporary spec was deleted.
- The proof used public authenticated product APIs
  `POST /api/app-change-packages`,
  `GET /api/app-change-packages/{id}`, and
  `GET /api/app-change-packages/{id}/review-evidence`; it did not call
  browser-public internal or test-only routes.
- Evidence artifact:
  `/tmp/choir-texture-package-provenance-1781581617265.json`; screenshot:
  `/tmp/choir-texture-package-provenance-1781581617265.png`.
- Product evidence ids: staging user
  `playwright-state-1781581607161-v10dlq@example.com`; package
  `pkg-texture-provenance-1781581617265`; Texture proof document ref
  `doc-texture-package-proof-1781581617265`; Texture proof revision ref
  `rev-texture-package-proof-1781581617265`.
- Product observations: the created package and package detail carried
  `provenance_refs_json.texture_doc_id` and
  `provenance_refs_json.texture_revision_id` with no emitted
  `vtext_doc_id` or `vtext_revision_id`; review evidence returned
  `human_proof.state="human_reviewable"` with narrative refs containing the
  Texture doc and revision ids; review evidence contained no `VText` copy;
  forbidden browser-public request count was zero for `/internal/*`,
  `/api/agent/*`, `/api/test/*`, `/api/prompts`, and `/api/events`.

Rollback path: restore the old emitted package provenance field names and
platform publication provenance predicates if AppChangePackage review evidence,
platform publication, public bundle reads, or downstream adoption proof
regresses.

Heresy delta: repaired for deployed new AppChangePackage and platform
publication provenance labels. Legacy package provenance refs and legacy
platform rows remain deletion-receipted read compatibility until a migration
or deletion receipt removes them. Universal Wire story projection fields,
general Texture metadata keys, durable actor ids, storage symbols, and file
suffixes remain separate discovered residue.

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

## Problem Checkpoint: Universal Wire Story Projection Label Residue

Mutation class: `green` documentation and evidence only. No runtime behavior,
frontend source, API contract, test fixture, storage, file alias, platform
publication, or persistent state changed in this checkpoint.

Read-only search on 2026-06-16 shows that Universal Wire's live story
projection contract still teaches the retired artifact ontology at the product
API and frontend-open boundary. The residue is bounded to the Universal Wire
story projection fields and story/source-state labels; broader `.vtext`
shortcut files, storage table names, durable `vtext:` actor labels, source
metadata keys, and Style.vtext prompt/style-source language remain separate
migration surfaces.

Conjecture delta: new Universal Wire story projections can emit Texture-named
document/content fields and source-state labels while the frontend consumes the
current Texture fields first and, only if needed, carries deletion-receipted
legacy read fallback for existing staged or persisted story payloads.

Protected surfaces: `/api/universal-wire/stories` response JSON, runtime story
publication verification checks, Universal Wire frontend story-open and related
Texture launch context, source-state labels, focused runtime/frontend tests,
and deployed browser product proof for opening a Universal Wire story as
Texture.

Admissible evidence class: focused Universal Wire runtime tests, frontend build
or focused Universal Wire Playwright tests, residue search for the old emitted
story projection fields in current code, CI/deploy identity if behavior changes
land, and deployed staging product proof through public authenticated product
paths.

Rollback path: restore the prior Universal Wire story field emitters and
frontend consumers if story indexing, platform publication verification,
related Texture launches, or Universal Wire frontend rendering regresses.

Heresy delta: discovered: Universal Wire still emits and consumes old-name
story projection labels after route/tool/app/provenance cutovers. Introduced:
none in this checkpoint. Repaired target: new Universal Wire story projection
payloads and frontend story launches should teach Texture without pretending
that storage, file suffix, actor id, or style-source residue is fixed.

Receipts:

- `internal/types/wire.go` still defines `ProjectionVTextDocs` with JSON
  `projection_vtext_docs`, `StoryVTextDoc` with JSON
  `story_vtext_doc_id`, and `VTextContent` with JSON `vtext_content`.
- `internal/runtime/universal_wire.go` still emits story ids and source state
  labels such as `source-network-vtext-*` and
  `source-network-vtext-index`, uses `StoryVTextDoc` for platform publication
  verification, and returns `universal-wire-vtext-index` /
  `universal-wire-edition-vtext` source labels.
- `frontend/src/lib/UniversalWireApp.svelte` still reads
  `story_vtext_doc_id`, creates related entities as `gw-vtext-*`, uses
  `target_kind: 'vtext_document'`, and opens story source paths ending in
  `.story.vtext`.
- `frontend/tests/universal-wire-app.spec.js` still stubs
  `source-network-vtext-*`, `universal-wire-vtext-index`, and visible copy
  saying `VText article`.
- Adjacent residue kept out of this slice includes `internal/store/vtext.go`
  storage tables, `platform_vtext_documents`, `.vtext` shortcut/alias files,
  durable `vtext:<doc_id>` author labels, `related_vtexts` metadata,
  source-renderer `vtext_document` compatibility beyond Universal Wire launch
  context, and Style.vtext selection prompt language.

Next behavior slice design:

- emit `projection_texture_docs`, `story_texture_doc_id`, and
  `texture_content` from Universal Wire stories;
- rename new source labels toward `source-network-texture-*`,
  `source-network-texture-index`, and `universal-wire-*-texture` where they
  are current payload/state labels rather than persisted storage keys;
- update Universal Wire frontend story-open and related launch code to consume
  Texture fields first, with legacy fallback only if tests prove current
  payload compatibility needs it;
- keep `.vtext` shortcut file names, durable `vtext:` actor labels, storage
  tables, and general `related_vtexts` metadata out of scope for this slice;
- prove locally with focused runtime Universal Wire tests, frontend build or
  focused Universal Wire Playwright tests, residue searches for the old story
  projection emitters, then push, monitor CI/deploy, verify staging identity,
  and run deployed Universal Wire product proof if the behavior is reachable
  without manually seeding success records.

## Local Repair: Universal Wire Story Projection Labels

Mutation class: `orange` with red-adjacent evidence boundaries, because this
changes the browser-public `/api/universal-wire/stories` story projection JSON,
runtime story publication verification references, Universal Wire frontend
launch context, deployed staging acceptance expectations, and current
Universal Wire tests. It does not change canonical Texture writes, storage
tables, `.vtext` shortcut files, durable actor ids, or platform publication
route registration.

Conjecture delta: Universal Wire can publish current story projection payloads
with Texture-named document/content fields and source labels while retaining
only frontend legacy-read fallback for old `story_vtext_doc_id` payloads until
staging proves the new payload shape.

Protected surfaces: `/api/universal-wire/stories`, `types.WireStory` JSON,
Universal Wire story indexing, platform publication verification, Texture API
read owner resolution for platform-owned Universal Wire docs, Universal Wire
frontend story launch and related Texture entity construction, and the deployed
Universal Wire staging acceptance spec.

Local evidence on 2026-06-16:

- `types.WireStory` now emits `projection_texture_docs`,
  `story_texture_doc_id`, and `texture_content`; the focused runtime test
  marshals a story and asserts those JSON keys exist while
  `projection_vtext_docs`, `story_vtext_doc_id`, and `vtext_content` do not.
- Universal Wire source labels now emit `universal-wire-texture-index`,
  `universal-wire-edition-texture`, `source-network-texture-*`, and
  `source-network-texture-index`.
- `frontend/src/lib/UniversalWireApp.svelte` now defaults to
  `universal-wire-texture-index`, creates related entities as `gw-texture-*`
  with `target_kind: 'texture_document'`, opens stories through
  `story_texture_doc_id` first, and keeps `story_vtext_doc_id` only as an
  explicit legacy payload fallback. The `.story.vtext` source path and
  `relatedVTexts` app-context property remain broader metadata/file-suffix
  residue outside this slice.
- `frontend/tests/universal-wire-staging-acceptance.spec.js` now treats
  `universal-wire-edition-texture` as the edition payload and asserts
  `story_texture_doc_id` is present while `story_vtext_doc_id` is absent.
- Focused runtime test:
  `nix develop -c go test ./internal/runtime -run 'TestHandleUniversalWireStories|TestResolveUniversalWireTextureReadOwner|TestNormalizeWireArticleSourceServiceProse|TestWireAutonomousPublishTranscludesEditionAndDebounces|TestWirePlatformPublishFailsClosedWithoutEditionWhenPlatformdFails' -count=1`
  passed.
- Runtime shard coverage:
  `nix develop -c scripts/go-test-runtime-shards` passed.
- Frontend build:
  `npm --prefix frontend run build` passed with the existing Universal Wire
  warnings for unused `currentUser` and `.wire-state` selectors.
- Current-code residue search
  `rg -n "ProjectionVTextDocs|StoryVTextDoc|VTextContent|projection_vtext_docs|story_vtext_doc_id|vtext_content|source-network-vtext|universal-wire-vtext-index|universal-wire-edition-vtext" internal frontend/src frontend/tests -g '!frontend/dist/**'`
  now finds only explicit legacy fallback/negative assertions:
  two fallback reads in `UniversalWireApp.svelte`, one staging negative
  assertion, and three runtime JSON absence assertions.

Rollback path: restore the old `WireStory` JSON fields, Universal Wire source
labels, and frontend consumers if staging shows story indexing, platform
publication verification, signed-in Universal Wire rendering, or Texture story
launches regress.

Heresy delta: repaired locally for current Universal Wire story projection
emitters, source-state labels, and frontend launch context. Discovered residue
remaining outside this slice includes `.vtext` aliases/source paths,
`vtext:` edition transclusion syntax, durable `vtext:<doc_id>` author labels,
`vtext_agent_revision` metadata types, Style.vtext style-source language,
general `related_vtexts` metadata, storage tables, and platform table names.

## Staging Evidence Checkpoint: Universal Wire Empty Edition Acceptance Gap

Mutation class: `green` documentation and evidence only. No test, runtime,
frontend, product API, staging state, or persistent data changed in this
checkpoint.

Staging evidence on 2026-06-16 after deploying commit
`9f332529d209e82df86056176ffac2d31d2c5df1` exposed an acceptance-oracle gap:
the deployed Universal Wire stories API returned the new
`universal-wire-edition-texture` source label with an empty `stories` array.
The staging acceptance spec assumed that an edition source always implies at
least one story and failed before it could observe a deployed story payload's
`story_texture_doc_id` field.

Receipts:

- Pushed behavior commit:
  `9f332529d209e82df86056176ffac2d31d2c5df1`
  (`runtime: rename texture wire projection labels`), after docs checkpoint
  `e7a61b9e`.
- CI run `27593330137` passed, including all runtime shards, non-runtime tests,
  integration-tagged smoke, Go vet/build, Docs Truth Check job, TLA+ model
  check, frontend build, final Go gate, and staging deploy.
- Deploy job `81578635355` succeeded.
- Docs Truth Check run `27593330130` passed; FlakeHub publish run
  `27593330160` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `9f332529d209e82df86056176ffac2d31d2c5df1`, deployed at
  `2026-06-16T04:05:57Z`.
- Deployed Playwright proof attempt
  `GO_CHOIR_RUN_UNIVERSAL_WIRE_STAGING=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/universal-wire-staging-acceptance.spec.js`
  failed because `stories.source === "universal-wire-edition-texture"` and
  `stories.stories.length === 0`; the assertion expected length greater than
  zero.

Conjecture delta: the deployed proof must distinguish "Universal Wire source
labels and app surface are cut over" from "staging currently has at least one
edition story payload available to inspect." Empty editions can still prove the
source-label and app-empty-state parts of C22, but they cannot prove deployed
story payload field names until staging has an edition story or a product path
creates one without manually seeding success records.

Rollback path: no runtime rollback is indicated by this evidence alone. If the
empty edition reflects a story-indexing regression rather than normal staging
data shape, restore the prior Universal Wire story projection behavior while
investigating platform publication verification.

Heresy delta: discovered: the Universal Wire deployed acceptance oracle
conflated edition existence with story availability. Introduced: none in this
checkpoint. Repaired target: update the acceptance spec to pass on empty
edition state while still asserting Texture labels and app surface, and leave
deployed story-field proof open until an actual story payload is reachable.

## Deployed Evidence: Universal Wire Texture Source Labels

Mutation class: `yellow` for the acceptance-spec repair and `green` for this
evidence update. The deployed runtime/frontend behavior under proof remains
commit `9f332529d209e82df86056176ffac2d31d2c5df1`.

Conjecture delta: after the acceptance oracle distinguishes empty edition state
from story payload availability, staging can prove the Universal Wire source
label and app-surface parts of C22 without overstating deployed story-field
coverage.

Deployed evidence on 2026-06-16:

- Refreshed staging Playwright auth state with
  `node scripts/setup-auth-state.mjs --baseUrl https://choir.news` from
  `frontend/`; generated user
  `qa-1781583037734-7tuzeq@example.com`.
- Deployed Playwright proof
  `GO_CHOIR_RUN_UNIVERSAL_WIRE_STAGING=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/universal-wire-staging-acceptance.spec.js`
  passed after the spec was corrected to accept empty editions.
- Product observations: `/api/universal-wire/stories` returned a
  `universal-wire-*` source label that was not
  `universal-wire-vtext-index` or `universal-wire-edition-vtext`; the response
  included the Universal Wire edition at `universal-wire/Wire.vtext`; the
  signed-in Universal Wire app rendered without SourceMaxx or Global Wire
  preview copy; because `stories.length === 0`, the app rendered the
  Universal Wire empty state and no story cards.
- Deployed story payload fields remain unproven on staging because no
  Universal Wire edition story payload was available. Local focused tests and
  runtime shards prove the emitted JSON shape for current story payloads.

Rollback path: if a future staging edition story exposes old story payload
fields or the app cannot open a story through `story_texture_doc_id`, restore
the previous Universal Wire story consumer/producer while investigating the
projection contract.

Heresy delta: repaired for deployed Universal Wire source-label and empty-state
app proof. Discovered but unrepaired for deployed scope: staging currently lacks
a Universal Wire story payload to prove `story_texture_doc_id`,
`projection_texture_docs`, and `texture_content` end to end.

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
  first revision through `patch_texture` metadata and Trace; the compatibility
  alias deletion receipt is landed under C17.
- C3 supported for report-only scope: the docs checker now carries H5
  retired-name warnings without failing docs-only CI. Current baseline:
  `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json` reports 1,130 total warnings, including 335 H5
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
- C18 supported for deployed public-route scope: new public publication reader
  URLs now mint under `/pub/texture/...`; existing `/pub/vtext/...` public link
  state remains accepted for resolve/export and frontend reader entry. CI run
  `27590698503`, deploy job `81570766605`, staging identity for commit
  `65502a706ef1adba7fc2d1ed5428e3f709f9d2d0`, and deployed Playwright
  publication/read/export proof all pass.
- C19 supported for deployed auth-intent scope: frontend Texture actions now emit
  Texture-named auth intents, the registry requires Texture-named mutable
  intents, and App replay/message handling accepts deletion-receipted legacy
  intent names. Local build, focused signed-out Texture publish overlay proof,
  CI run `27591417530`, deploy job `81572916777`, staging identity for commit
  `2f13598d37be2807f8cefe9258300a1a798a081c`, and deployed Playwright proof
  for signed-out auth overlay plus legacy `app=vtext` deep link all pass.
- C20 supported for deployed source-metadata scope: new source repair and source
  artifact paths now emit `texture_source_gap_repair`,
  `texture_source_artifact_attachment`, and `texture_source_artifact_ui`.
  Focused comprehensive runtime tests and frontend build pass; live residue
  search finds no old `vtext_source_gap_repair`,
  `vtext_source_artifact_attachment`, or `vtext_source_artifact_ui` hits in
  `internal`, `frontend/src`, or `frontend/tests`. CI run `27591835245`,
  deploy job `81574215697`, staging identity for commit
  `39d0c2ba125c81d59b34002685a9ce19ec98eda0`, and deployed source repair
  browser/API proof all pass. Adjacent fields such as
  `canonical_vtext_source_path`, `related_vtexts`, platform publication
  predicates, app-package `vtext_doc_id`, durable actor ids, and storage
  symbols remain broader migration surfaces.
- C21 supported for deployed package/provenance scope: new AppChangePackage
  human-proof refs now emit `texture_doc_id` and `texture_revision_id`, vsuper
  package prompt defaults ask for Texture narratives, review evidence recognizes
  Texture narrative refs and keeps explicit legacy package-provenance read
  compatibility, and platform publication provenance now writes
  `private_texture_revision`, `choir-private:texture/...`,
  `publish_texture_revision`, and `choir.platform.publish_texture.v0`.
  Focused comprehensive runtime tests, focused platform tests with direct row
  assertions, frontend build, doccheck, and residue searches pass locally.
  CI run `27592592351`, deploy job `81576474144`, staging identity for commit
  `24bff527b56e8f76e1ba3066dd5c71d52543120e`, and deployed
  AppChangePackage review-evidence proof all pass. Universal Wire story
  projection fields, general Texture metadata keys, durable actor ids, storage
  tables, and file suffixes are adjacent residue outside this slice.
- C22 supported for local Universal Wire projection scope and deployed
  source-label/app-empty-state scope: current story
  payloads now emit Texture-named projection/document/content fields and
  Texture source labels, frontend Universal Wire opens stories through
  `story_texture_doc_id` first and emits `texture_document` related launch
  targets, focused runtime tests pass, runtime shards pass, frontend build
  passes, and current-code residue search finds old story projection labels
  only in explicit legacy fallback or absence assertions. CI run
  `27593330137`, deploy job `81578635355`, and staging identity for commit
  `9f332529d209e82df86056176ffac2d31d2c5df1` pass. The first deployed
  Universal Wire proof reached the new `universal-wire-edition-texture` source
  label but found an empty edition. The repaired deployed proof passes for the
  source-label and empty-state app surface; deployed story-field proof remains
  open until staging has an edition story payload or a product path creates one
  without manually seeding success records.
- C23 supported for local Texture related-transclusion metadata/context scope:
  current frontend writers now prefer `related_textures`,
  `relatedTextures`, `texture_document`, `texture:` markdown refs, and
  Texture-named helper exports. The editor and markdown renderer keep explicit
  legacy read/parser fallback for `related_vtexts`, `relatedVTexts`, and
  `vtext:` refs. Focused related-transclusion tests, frontend build, and
  residue searches pass locally. Storage table names, `.vtext` file
  suffixes/source paths, durable `vtext:` actor ids,
  `canonical_vtext_source_path`, source-contract app-open expectations, and
  protocol v0 remain adjacent residue.

next move: commit and push C23, monitor CI/deploy/staging identity, and run a
deployed Texture related-transclusion proof if a bounded product path is
reachable without manually seeding success records. If deployed proof is not
reachable, record the blocker precisely and select the next residue class
among stale source-contract/app-launcher expectations, durable metadata keys,
storage/file suffixes, durable actor ids, deployed Universal Wire story-field
proof, and protocol v0. Keep protocol v0 unwritten until the remaining
working-surface proofs are complete.

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
